import { readFileSync, writeFileSync, existsSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomBytes } from 'node:crypto';
import { createServer } from 'node:net';
import mysql from 'mysql2/promise';
import {
  root,
  runtime,
  stateFile,
  state,
  saveJSON,
  compose,
  ownedContainer,
  command,
  pause,
} from './runtime-lib.mjs';
import { installMigration } from './migration.mjs';
import { localServicePeers } from './service-peers.mjs';
import { writeComposeEnvironment } from './compose-environment.mjs';
import { searchServiceConfig } from './search-config.mjs';
import { retainedSMTP } from './smtp-configuration.mjs';

async function freePort(start) {
  for (let port = start; port < start + 200; port++) {
    const server = createServer();
    try {
      await new Promise((done, fail) => {
        server.once('error', fail);
        server.listen(port, '127.0.0.1', done);
      });
      await new Promise((done) => server.close(done));
      return port;
    } catch {
      try {
        server.close();
      } catch {}
    }
  }
  throw Error('LOCAL_PORTS_UNAVAILABLE');
}
function pki(c) {
  const folder = resolve(runtime, 'pki');
  mkdirSync(folder, { recursive: true });
  if (existsSync(resolve(folder, 'edge.pem'))) return;
  const env = { ...process.env };
  if (!env.OPENSSL_CONF && existsSync('D:/Anaconda/Library/ssl/openssl.cnf'))
    env.OPENSSL_CONF = 'D:/Anaconda/Library/ssl/openssl.cnf';
  const openssl = (...args) =>
    command(process.env.OPENSSL_BIN ?? 'openssl', args, {
      env,
      failureCode: 'CERTIFICATE_GENERATION_FAILED',
    });
  openssl(
    'req',
    '-x509',
    '-newkey',
    'rsa:2048',
    '-nodes',
    '-days',
    '365',
    '-subj',
    '/CN=collaboration-report-development-ca',
    '-keyout',
    resolve(folder, 'ca-key.pem'),
    '-out',
    resolve(folder, 'ca.pem'),
    '-addext',
    'basicConstraints=critical,CA:TRUE',
    '-addext',
    'keyUsage=critical,keyCertSign,cRLSign',
  );
  for (const role of ['account', 'document', 'search', 'collaboration', 'edge']) {
    openssl(
      'req',
      '-new',
      '-newkey',
      'rsa:2048',
      '-nodes',
      '-subj',
      `/CN=${role}`,
      '-keyout',
      resolve(folder, role + '-key.pem'),
      '-out',
      resolve(folder, role + '.csr'),
    );
    const ext = resolve(folder, role + '.ext');
    writeFileSync(
      ext,
      `basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth,clientAuth\nsubjectAltName=DNS:localhost,IP:127.0.0.1,URI:spiffe://collaboration-report/${role}\n`,
    );
    openssl(
      'x509',
      '-req',
      '-in',
      resolve(folder, role + '.csr'),
      '-CA',
      resolve(folder, 'ca.pem'),
      '-CAkey',
      resolve(folder, 'ca-key.pem'),
      '-CAcreateserial',
      '-days',
      '30',
      '-extfile',
      ext,
      '-out',
      resolve(folder, role + '.pem'),
    );
  }
}
function writeServiceConfigs(c, checksum, additionalMigrations) {
  for (const role of ['account', 'document', 'search', 'collaboration']) {
    const folder = resolve(runtime, 'pki'),
      database = {
        host: '127.0.0.1',
        port: c.ports.mysql,
        user: 'collab_' + role,
        password: c.passwords[role],
        database: role + '_db',
        timezone: 'Z',
        supportBigNumbers: true,
        bigNumberStrings: true,
        connectionLimit: 8,
      };
    const config = {
      role,
      httpPort: c.ports[role + 'Http'],
      rpcPort: c.ports[role + 'Rpc'],
      host: '127.0.0.1',
      dsn: `${database.user}:${database.password}@tcp(127.0.0.1:${database.port})/${database.database}?parseTime=true&timeout=2s&readTimeout=3s&writeTimeout=3s&loc=UTC&time_zone=%27%2B00%3A00%27`,
      database,
      certFile: resolve(folder, role + '.pem'),
      keyFile: resolve(folder, role + '-key.pem'),
      caFile: resolve(folder, 'ca.pem'),
      expectedSQLSHA: checksum,
      expectedAdditionalMigrations: additionalMigrations,
      origin: `https://localhost:${c.ports.edge}`,
      root,
      peers: localServicePeers(c.ports),
    };
    if (role === 'account' && c.ports.redis)
      config.redis = {
        host: `127.0.0.1:${c.ports.redis}`,
        user: 'account_login',
        password: c.passwords.redisAccount,
      };
    if (role === 'account' && c.ports.smtp) {
      config.smtp = {
        address: `127.0.0.1:${c.ports.smtp}`,
        from: 'no-reply@example.test',
        security: 'development',
      };
      config.challengeKey = c.passwords.challengeKey;
    }
    if (role === 'search' && c.ports.kafka) config.search = searchServiceConfig(c);
    // 升级迁移会重新生成基础配置；资源地址与导出工具属于既有部署事实，
    // 不能因补迁移静默退回本地存储或丢掉转换配置。
    const previousFile = resolve(runtime, 'services', role + '.json');
    if (role === 'account' && existsSync(previousFile)) {
      const previous = JSON.parse(readFileSync(previousFile));
      if (
        previous.role !== role ||
        resolve(previous.root) !== resolve(root) ||
        previous.rpcPort !== c.ports.accountRpc
      )
        throw Error('ACCOUNT_CONFIG_BINDING_INVALID');
      // 补迁移或重跑bootstrap不应把已经配置的真实SMTP退回开发收件器。
      if (previous.smtp || config.smtp) config.smtp = retainedSMTP(previous.smtp, config.smtp);
    }
    if (role === 'document' && existsSync(previousFile)) {
      const previous = JSON.parse(readFileSync(previousFile));
      if (
        previous.role !== role ||
        resolve(previous.root) !== resolve(root) ||
        previous.rpcPort !== c.ports[role + 'Rpc']
      )
        throw Error('DOCUMENT_CONFIG_BINDING_INVALID');
      if (previous.resources) config.resources = previous.resources;
      if (previous.exports) config.exports = previous.exports;
    }
    saveJSON(resolve(runtime, 'services', role + '.json'), config);
  }
}
async function up() {
  mkdirSync(runtime, { recursive: true });
  let c;
  if (existsSync(stateFile)) c = state();
  else {
    const id = randomBytes(4).toString('hex');
    c = {
      project: 'collabreport' + id,
      owner: 'collaboration-report-dev-' + id,
      ports: {},
      passwords: {},
    };
    let cursor = 20600;
    for (const name of [
      'mysql',
      'accountHttp',
      'accountRpc',
      'documentHttp',
      'documentRpc',
      'searchHttp',
      'searchRpc',
      'collaborationHttp',
      'collaborationRpc',
      'edge',
    ]) {
      c.ports[name] = await freePort(cursor);
      cursor = c.ports[name] + 1;
    }
    for (const name of ['root', 'account', 'document', 'search', 'collaboration'])
      c.passwords[name] = randomBytes(24).toString('hex');
    saveJSON(stateFile, c);
  }
  writeComposeEnvironment(c);
  const existing = compose('ps', '--all', '-q', 'mysql');
  if (existing) ownedContainer('mysql');
  compose('up', '-d', 'mysql');
  const container = ownedContainer('mysql');
  let connection;
  for (let n = 0; n < 120; n++) {
    try {
      connection = await mysql.createConnection({
        host: '127.0.0.1',
        port: c.ports.mysql,
        user: 'root',
        password: c.passwords.root,
        multipleStatements: true,
        connectTimeout: 2000,
      });
      break;
    } catch {
      await pause(500);
    }
  }
  if (!connection) throw Error('MYSQL_STARTUP_TIMEOUT');
  try {
    await connection.query("SET time_zone='+00:00'");
    const migration = await installMigration(connection, resolve(root, 'db/migrations'));
    for (const role of ['account', 'document', 'search', 'collaboration'])
      await connection.query(`CREATE USER IF NOT EXISTS 'collab_${role}'@'%' IDENTIFIED BY ?`, [
        c.passwords[role],
      ]);
    await connection.query(readFileSync(resolve(root, 'db/migrations/service-grants.sql'), 'utf8'));
    pki(c);
    writeServiceConfigs(c, migration.checksum, migration.additionalMigrations);
    const [[facts]] = await connection.query(
      'SELECT @@version AS mysqlVersion,@@innodb_flush_log_at_trx_commit AS durable,@@global.time_zone AS timeZone',
    );
    saveJSON(resolve(root, 'artifacts/batches/01/environment.json'), {
      state: 'READY',
      owner: c.owner,
      project: c.project,
      ports: c.ports,
      container,
      migration,
      facts,
      createdAt: new Date().toISOString(),
    });
    console.log(
      JSON.stringify({
        state: 'READY',
        project: c.project,
        ports: c.ports,
        migration: migration.state,
      }),
    );
  } finally {
    await connection.end();
  }
}
async function stop() {
  ownedContainer('mysql');
  compose('stop', '-t', '10', 'mysql');
  console.log(JSON.stringify({ state: 'STOPPED', volumesRetained: true }));
}
try {
  if (process.argv[2] === 'up') await up();
  else if (process.argv[2] === 'stop') await stop();
  else throw Error('USAGE_DEV_ENVIRONMENT_UP_OR_STOP');
} catch (error) {
  // Never print raw SQL, DSNs, passwords or certificate-generation output.
  console.error(
    JSON.stringify({
      state: 'FAILED',
      code: /^[A-Z0-9_]+$/.test(error.message) ? error.message : 'ENVIRONMENT_BOOTSTRAP_FAILED',
    }),
  );
  process.exitCode = 1;
}
