import { existsSync, readFileSync, readdirSync, chmodSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomBytes, createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { isIP } from 'node:net';
import { retainedSMTP } from './smtp-configuration.mjs';
import {
  sourceRoot,
  applicationRoot,
  roles,
  ports,
  privateDirectory,
  privateFile,
  privateJSON,
  readJSON,
  isSecret,
  isCanalSecret,
  argumentsForServer,
  deploymentDirectory,
  publicOrigin,
  validateManifest,
  requirePreparedFiles,
  migrationExpectation,
  serviceConfiguration,
  composeEnvironment,
} from './server-configuration.mjs';

function certificates(directory, manifest) {
  const admin = resolve(directory, 'admin');
  const pki = resolve(directory, 'application/pki');
  const env = { ...process.env };
  if (!env.OPENSSL_CONF && existsSync('D:/Anaconda/Library/ssl/openssl.cnf'))
    env.OPENSSL_CONF = 'D:/Anaconda/Library/ssl/openssl.cnf';
  const openssl = (...args) => {
    const result = spawnSync(process.env.OPENSSL_BIN ?? 'openssl', args, {
      encoding: 'utf8',
      windowsHide: true,
      env,
      timeout: 60000,
    });
    if (result.status !== 0) throw Error('SERVER_CERTIFICATE_GENERATION_FAILED');
  };
  if (!existsSync(resolve(admin, 'ca-key.pem'))) {
    openssl(
      'req',
      '-x509',
      '-newkey',
      'rsa:2048',
      '-nodes',
      '-days',
      '365',
      '-subj',
      '/CN=collaboration-report-server-ca',
      '-keyout',
      resolve(admin, 'ca-key.pem'),
      '-out',
      resolve(admin, 'ca.pem'),
      '-addext',
      'basicConstraints=critical,CA:TRUE',
      '-addext',
      'keyUsage=critical,keyCertSign,cRLSign',
    );
  }
  if (!existsSync(resolve(admin, 'ca.pem'))) throw Error('SERVER_CA_PARTIAL_GENERATION');
  privateFile(resolve(pki, 'ca.pem'), readFileSync(resolve(admin, 'ca.pem')));
  const hostname = new URL(manifest.publicOrigin).hostname.replace(/^\[|\]$/g, '');
  for (const role of [...roles, 'edge']) {
    const certificate = resolve(pki, role + '.pem');
    const key = resolve(pki, role + '-key.pem');
    // 中断重试保留已完成证书；缺配对时拒绝，不能更换已投入运行的身份材料。
    if (existsSync(certificate)) {
      if (!existsSync(key)) throw Error('SERVER_CERTIFICATE_PARTIAL_GENERATION');
      continue;
    }
    const csr = resolve(admin, role + '.csr');
    if (!existsSync(key)) {
      openssl(
        'req',
        '-new',
        '-newkey',
        'rsa:2048',
        '-nodes',
        '-subj',
        `/CN=${role}`,
        '-keyout',
        key,
        '-out',
        csr,
      );
    } else {
      openssl('req', '-new', '-key', key, '-subj', `/CN=${role}`, '-out', csr);
    }
    const sans = ['DNS:localhost', 'IP:127.0.0.1', `URI:spiffe://collaboration-report/${role}`];
    if (role === 'edge' && hostname !== 'localhost' && hostname !== '127.0.0.1')
      sans.push((isIP(hostname) ? 'IP:' : 'DNS:') + hostname);
    const extension = resolve(admin, role + '.ext');
    privateFile(
      extension,
      'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\n' +
        'extendedKeyUsage=serverAuth,clientAuth\nsubjectAltName=' +
        sans.join(',') +
        '\n',
    );
    openssl(
      'x509',
      '-req',
      '-in',
      csr,
      '-CA',
      resolve(admin, 'ca.pem'),
      '-CAkey',
      resolve(admin, 'ca-key.pem'),
      '-CAcreateserial',
      '-days',
      '365',
      '-extfile',
      extension,
      '-out',
      certificate,
    );
  }
  if (process.platform !== 'win32') {
    for (const folder of [admin, pki])
      for (const name of readdirSync(folder)) chmodSync(resolve(folder, name), 0o600);
  }
}

function validateSecrets(secrets, manifest) {
  if (
    secrets.owner !== manifest.owner ||
    secrets.project !== manifest.project ||
    ![...roles, 'root', 'redisAdmin', 'redisAccount', 'challengeKey', 'searchCursorKey'].every(
      (name) => isSecret(secrets[name]),
    ) ||
    !isCanalSecret(secrets.canal)
  )
    throw Error('SERVER_CREDENTIALS_INVALID');
}

function prepare() {
  const options = argumentsForServer(process.argv.slice(2), [
    '--directory',
    '--origin',
    '--https-port',
    '--smtp-address',
    '--smtp-from',
  ]);
  const directory = deploymentDirectory(options['--directory']);
  const manifestFile = resolve(directory, 'manifest.json');
  let manifest;
  if (existsSync(manifestFile)) {
    manifest = validateManifest(readJSON(manifestFile), directory);
    for (const [flag, field] of [
      ['--origin', 'publicOrigin'],
      ['--https-port', 'httpsPort'],
      ['--smtp-address', 'smtpAddress'],
      ['--smtp-from', 'smtpFrom'],
    ]) {
      if (
        options[flag] !== undefined &&
        String(manifest[field]) !==
          (flag === '--origin' ? publicOrigin(options[flag]) : options[flag])
      )
        throw Error('SERVER_EXISTING_CONFIGURATION_CONFLICT');
    }
    if (manifest.state === 'PREPARED') {
      requirePreparedFiles(directory);
      const secrets = readJSON(resolve(directory, 'admin/credentials.json'));
      validateSecrets(secrets, manifest);
      console.log(
        JSON.stringify({
          state: 'ALREADY_PREPARED',
          directory,
          project: manifest.project,
          url: manifest.publicOrigin,
        }),
      );
      return;
    }
  } else {
    if (existsSync(directory) && readdirSync(directory).length)
      throw Error('SERVER_DIRECTORY_NOT_EMPTY');
    privateDirectory(directory);
    const identifier = randomBytes(8).toString('hex');
    const origin = publicOrigin(options['--origin'] ?? 'https://localhost:8443');
    manifest = {
      version: 1,
      deployment: 'linux-compose',
      state: 'PREPARING',
      directory,
      project: 'collabserver' + identifier,
      owner: 'collaboration-report-server-' + identifier,
      publicOrigin: origin,
      httpsPort: Number(options['--https-port'] ?? (new URL(origin).port || 443)),
      image: 'collaboration-report:linux-development',
      developmentSMTP: options['--smtp-address'] === undefined,
      smtpAddress: options['--smtp-address'] ?? `127.0.0.1:${ports.smtp}`,
      smtpFrom: options['--smtp-from'] ?? 'no-reply@example.test',
    };
    validateManifest(manifest, directory);
    if (
      !/^[A-Za-z0-9.[\]:-]+$/.test(manifest.smtpAddress) ||
      !/^[^\s@]+@[^\s@]+$/.test(manifest.smtpFrom)
    )
      throw Error('SERVER_SMTP_INVALID');
    privateJSON(manifestFile, manifest);
  }
  const secretFile = resolve(directory, 'admin/credentials.json');
  let secrets;
  if (existsSync(secretFile)) secrets = readJSON(secretFile);
  else {
    secrets = { owner: manifest.owner, project: manifest.project };
    for (const name of [
      ...roles,
      'root',
      'redisAdmin',
      'redisAccount',
      'challengeKey',
      'searchCursorKey',
    ])
      secrets[name] = randomBytes(32).toString('hex');
    secrets.canal = randomBytes(12).toString('hex');
    privateJSON(secretFile, secrets);
  }
  validateSecrets(secrets, manifest);
  for (const folder of [
    'application/services',
    'application/pki',
    'application/resources/document',
    'application/exports/document',
    'application/logs',
    'admin/redis',
  ])
    privateDirectory(resolve(directory, folder));
  const expectation = migrationExpectation(resolve(sourceRoot, 'db/migrations'));
  for (const role of roles) {
    const file = resolve(directory, 'application/services', role + '.json');
    const configuration = serviceConfiguration(role, manifest, secrets, expectation);
    if (role === 'account' && existsSync(file)) {
      const previous = readJSON(file);
      if (
        previous.role !== role ||
        previous.root !== applicationRoot ||
        previous.rpcPort !== ports.accountRpc
      )
        throw Error('SERVER_ACCOUNT_CONFIGURATION_MISMATCH');
      // 重试prepare沿用用户已经选定的SMTP配置，不因重新生成基础段而回退明文。
      configuration.smtp = retainedSMTP(previous.smtp, configuration.smtp);
    }
    privateJSON(file, configuration);
  }
  privateJSON(resolve(directory, 'application/dev.json'), {
    deployment: manifest.deployment,
    root: applicationRoot,
    owner: manifest.owner,
    project: manifest.project,
    ports,
    publicOrigin: manifest.publicOrigin,
    edgeListenHost: '0.0.0.0',
    developmentSMTP: manifest.developmentSMTP,
  });
  certificates(directory, manifest);
  privateFile(resolve(directory, 'admin/mysql-root-password'), secrets.root + '\n');
  privateFile(
    resolve(directory, 'admin/canal.env'),
    `canal.instance.dbPassword=${secrets.canal}\n`,
  );
  const hash = (value) => createHash('sha256').update(value).digest('hex');
  // 保留现有Redis7.2 AOF重放规则，default仍关闭；业务账号只访问登录与验证码前缀。
  const replay =
    '~acct:login:* ~acct:challenge:* +select +multi +exec +incr +incrby ' +
    '+expire +pexpire +pexpireat +set +hset +hincrby +del +restore';
  privateFile(
    resolve(directory, 'admin/redis/users.acl'),
    `user default reset off resetpass ${replay}\n` +
      `user bootstrap_admin reset on #${hash(secrets.redisAdmin)} ~* +@all\n` +
      `user account_login reset on #${hash(secrets.redisAccount)} ~acct:login:* ~acct:challenge:* ` +
      '+ping +hello +client|setname +eval +incr +expire +get +pttl +set +del +hset +hget +hincrby\n',
  );
  privateFile(
    resolve(directory, 'admin/redis/redis.conf'),
    'bind 0.0.0.0\nprotected-mode yes\naclfile /usr/local/etc/redis/users.acl\n' +
      'maxmemory-policy noeviction\nappendonly yes\nappendfsync always\nsave ""\n',
  );
  privateFile(resolve(directory, 'compose.env'), composeEnvironment(manifest));
  privateJSON(manifestFile, { ...manifest, state: 'PREPARED' });
  requirePreparedFiles(directory);
  console.log(
    JSON.stringify({
      state: 'PREPARED',
      directory,
      project: manifest.project,
      url: manifest.publicOrigin,
      developmentSMTP: manifest.developmentSMTP,
    }),
  );
}

try {
  prepare();
} catch (error) {
  console.error(
    JSON.stringify({
      state: 'FAILED',
      code: /^[A-Z0-9_]+$/.test(error.message) ? error.message : 'SERVER_PREPARATION_FAILED',
    }),
  );
  process.exitCode = 1;
}
