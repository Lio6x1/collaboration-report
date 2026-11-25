import { existsSync, readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomBytes, createHash } from 'node:crypto';
import net from 'node:net';
import {
  state,
  stateFile,
  saveJSON,
  runtime,
  root,
  compose,
  ownedContainer,
  pause,
} from './runtime-lib.mjs';
import { redisCommand } from './redis-client.mjs';
import { localServicePeers } from './service-peers.mjs';
import { writeComposeEnvironment } from './compose-environment.mjs';
import { retainedSMTP } from './smtp-configuration.mjs';
async function run() {
  const env = state();
  // 服务尚未启动时，OS只能看到已监听端口；配置里已经预留的HTTP/RPC
  // 端口也不能再分配给Redis/SMTP。已有冲突须先保留现场并明确恢复。
  const configuredPorts = Object.values(env.ports);
  if (new Set(configuredPorts).size !== configuredPorts.length)
    throw Error('LOCAL_SERVICE_PORT_CONFLICT');
  if (!env.ports.redis) {
    for (let port = 20610; port < 20810; port++) {
      if (Object.values(env.ports).includes(port)) continue;
      const server = net.createServer();
      try {
        await new Promise((ok, fail) => {
          server.once('error', fail);
          server.listen(port, '127.0.0.1', ok);
        });
        await new Promise((ok) => server.close(ok));
        env.ports.redis = port;
        break;
      } catch {
        server.close();
      }
    }
    if (!env.ports.redis) throw Error('REDIS_PORT_UNAVAILABLE');
    env.passwords.redisAccount = randomBytes(24).toString('hex');
    env.passwords.redisAdmin = randomBytes(24).toString('hex');
    saveJSON(stateFile, env);
  }
  const folder = resolve(runtime, 'redis');
  mkdirSync(folder, { recursive: true });
  const hash = (s) => createHash('sha256').update(s).digest('hex');
  // 默认账号关闭；登录账号只有自己的前缀和原子限流需要的命令，没有CONFIG/FLUSHALL。
  // Redis7.2的AOF EXEC重检使用default规则（上游#14541）：保持default关闭、无密码，
  // 仅给重放所需的私有前缀写命令；网络客户端仍必须显式AUTH到受限业务账号。
  const replay =
    '~acct:login:* ~acct:challenge:* +select +multi +exec +incr +incrby +expire +pexpire +pexpireat +set +hset +hincrby +del +restore';
  writeFileSync(
    resolve(folder, 'users.acl'),
    `user default reset off resetpass ${replay}\nuser bootstrap_admin reset on #${hash(env.passwords.redisAdmin)} ~* +@all\nuser account_login reset on #${hash(env.passwords.redisAccount)} ~acct:login:* ~acct:challenge:* +ping +hello +client|setname +eval +incr +expire +get +pttl +set +del +hset +hget +hincrby\n`,
  );
  writeFileSync(
    resolve(folder, 'redis.conf'),
    'bind 0.0.0.0\nprotected-mode yes\naclfile /usr/local/etc/redis/users.acl\nmaxmemory 128mb\nmaxmemory-policy noeviction\nappendonly yes\nappendfsync always\nsave ""\n',
  );
  writeComposeEnvironment(env);
  const existing = compose('ps', '--all', '-q', 'redis');
  if (existing) ownedContainer('redis');
  compose('up', '-d', 'redis');
  ownedContainer('redis');
  let ready = false;
  for (let n = 0; n < 60; n++) {
    try {
      if (
        (await redisCommand(env.ports.redis, 'account_login', env.passwords.redisAccount, [
          'PING',
        ])) === 'PONG'
      ) {
        ready = true;
        break;
      }
    } catch {}
    await pause(250);
  }
  if (!ready) throw Error('REDIS_STARTUP_TIMEOUT');
  if (
    (await redisCommand(env.ports.redis, 'bootstrap_admin', env.passwords.redisAdmin, [
      'ACL',
      'LOAD',
    ])) !== 'OK'
  )
    throw Error('REDIS_ACL_RELOAD_FAILED');
  // 验证码与SMTP仅接入账号服务；辅助收件器不增加业务服务或共享数据库。
  if (!env.ports.smtp) {
    for (let port = 20611; port < 20820; port++) {
      if (Object.values(env.ports).includes(port)) continue;
      const server = net.createServer();
      try {
        await new Promise((ok, fail) => {
          server.once('error', fail);
          server.listen(port, '127.0.0.1', ok);
        });
        await new Promise((ok) => server.close(ok));
        env.ports.smtp = port;
        break;
      } catch {
        server.close();
      }
    }
    if (!env.ports.smtp) throw Error('SMTP_PORT_UNAVAILABLE');
  }
  if (!env.passwords.challengeKey) env.passwords.challengeKey = randomBytes(32).toString('hex');
  saveJSON(stateFile, env);
  for (const role of ['account', 'document', 'search', 'collaboration']) {
    const file = resolve(runtime, 'services', role + '.json'),
      cfg = JSON.parse(readFileSync(file));
    cfg.peers = localServicePeers(env.ports);
    if (role === 'account') {
      cfg.redis = {
        host: `127.0.0.1:${env.ports.redis}`,
        user: 'account_login',
        password: env.passwords.redisAccount,
      };
      cfg.smtp = retainedSMTP(cfg.smtp, {
        address: `127.0.0.1:${env.ports.smtp}`,
        from: 'no-reply@example.test',
        security: 'development',
      });
      cfg.challengeKey = env.passwords.challengeKey;
    }
    saveJSON(file, cfg);
  }
  saveJSON(resolve(root, 'artifacts/batches/02-login/redis.json'), {
    state: 'READY',
    port: env.ports.redis,
    owner: env.owner,
    maxmemoryPolicy: 'noeviction',
    aof: 'always',
  });
  console.log(JSON.stringify({ state: 'READY', service: 'redis', port: env.ports.redis }));
}
run().catch((error) => {
  // 仅公开已知的配置冲突；数据库、证书及驱动原文仍统一隐藏。
  const code =
    error.message === 'LOCAL_SERVICE_PORT_CONFLICT' ? error.message : 'LOGIN_ENVIRONMENT_FAILED';
  console.error(JSON.stringify({ code }));
  process.exitCode = 1;
});
