import { chmodSync, existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { readMigrationChain } from './migration-chain.mjs';
import { localServicePeers } from './service-peers.mjs';
import { smtpSettings } from './smtp-configuration.mjs';

export const sourceRoot = fileURLToPath(new URL('../', import.meta.url));
export const applicationRoot = '/app/';
export const applicationRuntime = '/app/.runtime';
export const roles = ['account', 'document', 'search', 'collaboration'];
export const ports = Object.freeze({
  mysql: 3306,
  redis: 6379,
  kafka: 29092,
  es: 9200,
  accountHttp: 20601,
  accountRpc: 20602,
  documentHttp: 20603,
  documentRpc: 20604,
  searchHttp: 20605,
  searchRpc: 20606,
  collaborationHttp: 20607,
  collaborationRpc: 20608,
  edge: 8443,
  smtp: 20611,
});

export function privateDirectory(directory) {
  mkdirSync(directory, { recursive: true, mode: 0o700 });
  if (process.platform !== 'win32') chmodSync(directory, 0o700);
}

export function privateFile(file, content) {
  privateDirectory(resolve(file, '..'));
  // 原子替换避免中断后把半个JSON当持久身份；新文件从创建起即限制权限。
  const temporary = file + '.pending';
  writeFileSync(temporary, content, { mode: 0o600 });
  if (process.platform !== 'win32') chmodSync(temporary, 0o600);
  renameSync(temporary, file);
}

export function privateJSON(file, value) {
  privateFile(file, JSON.stringify(value, null, 2) + '\n');
}

export const readJSON = (file) => JSON.parse(readFileSync(file, 'utf8'));
export const isSecret = (value) => typeof value === 'string' && /^[a-f0-9]{48,64}$/.test(value);
// 固定Canal驱动将密码写入复制注册包，沿用已验证的24位随机hex，不能用64位通用口令。
export const isCanalSecret = (value) =>
  typeof value === 'string' && /^[a-f0-9]{24,32}$/.test(value);

export function argumentsForServer(args, allowed) {
  const options = {};
  for (let index = 0; index < args.length; index++) {
    const name = args[index];
    if (!allowed.includes(name) || options[name] !== undefined || !args[index + 1])
      throw Error('SERVER_ARGUMENTS_INVALID');
    options[name] = args[++index];
  }
  return options;
}

export function deploymentDirectory(value) {
  const directory = resolve(value ?? resolve(sourceRoot, '.runtime/server-deployment'));
  // 独立子目录可以位于.runtime下，但不能覆盖原Windows运行目录或仓库本身。
  for (const protectedPath of [resolve(sourceRoot), resolve(sourceRoot, '.runtime')]) {
    const normalize = (path) => (process.platform === 'win32' ? path.toLowerCase() : path);
    const target = normalize(directory);
    const protectedTarget = normalize(protectedPath);
    if (target === protectedTarget || protectedTarget.startsWith(target + '/'))
      throw Error('SERVER_DIRECTORY_NOT_ISOLATED');
    if (process.platform === 'win32' && protectedTarget.startsWith(target + '\\'))
      throw Error('SERVER_DIRECTORY_NOT_ISOLATED');
  }
  if (/[\r\n'"$]/.test(directory)) throw Error('SERVER_DIRECTORY_INVALID');
  return directory;
}

export function publicOrigin(value) {
  const parsed = new URL(value);
  if (
    parsed.protocol !== 'https:' ||
    parsed.username ||
    parsed.password ||
    parsed.pathname !== '/' ||
    parsed.search ||
    parsed.hash
  )
    throw Error('SERVER_ORIGIN_INVALID');
  return parsed.origin;
}

export function validateManifest(manifest, directory) {
  if (
    manifest.version !== 1 ||
    manifest.deployment !== 'linux-compose' ||
    !/^collabserver[a-f0-9]{16}$/.test(manifest.project ?? '') ||
    manifest.owner !== 'collaboration-report-server-' + manifest.project.slice(12) ||
    manifest.directory !== directory ||
    manifest.publicOrigin !== publicOrigin(manifest.publicOrigin) ||
    !Number.isInteger(manifest.httpsPort) ||
    manifest.httpsPort < 1 ||
    manifest.httpsPort > 65535 ||
    manifest.image !== 'collaboration-report:linux-development' ||
    typeof manifest.developmentSMTP !== 'boolean' ||
    !/^[A-Za-z0-9.[\]:-]+$/.test(manifest.smtpAddress ?? '') ||
    !/^[^\s@]+@[^\s@]+$/.test(manifest.smtpFrom ?? '') ||
    !['PREPARING', 'PREPARED'].includes(manifest.state)
  )
    throw Error('SERVER_MANIFEST_INVALID');
  return manifest;
}

export function loadDeployment(directory) {
  const manifest = validateManifest(readJSON(resolve(directory, 'manifest.json')), directory);
  if (manifest.state !== 'PREPARED') throw Error('SERVER_PREPARATION_INCOMPLETE');
  const environment = readJSON(resolve(directory, 'application/dev.json'));
  if (
    environment.deployment !== manifest.deployment ||
    environment.root !== applicationRoot ||
    environment.owner !== manifest.owner ||
    environment.project !== manifest.project ||
    environment.publicOrigin !== manifest.publicOrigin ||
    JSON.stringify(environment.ports) !== JSON.stringify(ports) ||
    environment.edgeListenHost !== '0.0.0.0' ||
    environment.developmentSMTP !== manifest.developmentSMTP ||
    environment.passwords !== undefined
  )
    throw Error('SERVER_CONFIGURATION_MISMATCH');
  return { manifest, environment };
}

export function migrationExpectation(folder) {
  const [baseline, ...additional] = readMigrationChain(folder);
  return {
    expectedSQLSHA: baseline.manifest.sha256,
    expectedAdditionalMigrations: Object.fromEntries(
      additional.map(({ manifest }) => [manifest.version, manifest.sha256]),
    ),
  };
}

export function serviceConfiguration(role, manifest, secrets, expectation) {
  const database = {
    host: 'mysql',
    port: 3306,
    user: 'collab_' + role,
    password: secrets[role],
    database: role + '_db',
    timezone: 'Z',
    supportBigNumbers: true,
    bigNumberStrings: true,
    connectionLimit: 8,
  };
  const configuration = {
    role,
    httpPort: ports[role + 'Http'],
    rpcPort: ports[role + 'Rpc'],
    host: '127.0.0.1',
    dsn: `${database.user}:${database.password}@tcp(mysql:3306)/${database.database}?parseTime=true&timeout=2s&readTimeout=3s&writeTimeout=3s&loc=UTC&time_zone=%27%2B00%3A00%27`,
    database,
    certFile: `${applicationRuntime}/pki/${role}.pem`,
    keyFile: `${applicationRuntime}/pki/${role}-key.pem`,
    caFile: `${applicationRuntime}/pki/ca.pem`,
    ...expectation,
    origin: manifest.publicOrigin,
    root: applicationRoot,
    peers: localServicePeers(ports),
  };
  if (role === 'account') {
    configuration.redis = {
      host: 'redis:6379',
      user: 'account_login',
      password: secrets.redisAccount,
    };
    configuration.smtp = {
      address: manifest.smtpAddress,
      from: manifest.smtpFrom,
      security: manifest.developmentSMTP
        ? 'development'
        : manifest.smtpAddress.endsWith(':465')
          ? 'tls'
          : 'starttls',
      ...(manifest.developmentSMTP
        ? {}
        : { credentialsFile: `${applicationRuntime}/private/smtp-credentials.json` }),
    };
    configuration.challengeKey = secrets.challengeKey;
  }
  if (role === 'search') {
    configuration.search = {
      owner: manifest.owner,
      brokers: ['kafka:29092'],
      elasticEndpoint: 'http://es:9200',
      topic: 'collab-publications-v1',
      group: manifest.project + '-search',
      alias: 'collab-publications',
      initialIndex: 'collab-publications-g1',
      cursorKey: secrets.searchCursorKey,
      cursorTTLSeconds: 30,
    };
  }
  if (role === 'document') {
    configuration.resources = {
      root: `${applicationRuntime}/resources/document`,
      spaceBytes: 1 << 30,
      uploadBytes: 32 << 20,
      temporaryBytes: 256 << 20,
      uploadTTLSeconds: 3600,
    };
    configuration.exports = {
      root: `${applicationRuntime}/exports/document`,
      pandoc: '/usr/local/bin/pandoc',
      node: '/usr/local/bin/node',
      browser: '',
      workers: 1,
    };
  }
  return configuration;
}

export function composeEnvironment(manifest) {
  return (
    [
      `COLLAB_PROJECT=${manifest.project}`,
      `COLLAB_OWNER=${manifest.owner}`,
      `COLLAB_DEPLOYMENT_DIRECTORY='${manifest.directory.replaceAll('\\', '/')}'`,
      `COLLAB_HTTPS_PORT=${manifest.httpsPort}`,
      `COLLAB_APPLICATION_IMAGE=${manifest.image}`,
    ].join('\n') + '\n'
  );
}

export function requirePreparedFiles(directory) {
  const loaded = loadDeployment(directory);
  const { manifest } = loaded;
  const secrets = readJSON(resolve(directory, 'admin/credentials.json'));
  if (
    secrets.owner !== manifest.owner ||
    secrets.project !== manifest.project ||
    ![...roles, 'root', 'redisAdmin', 'redisAccount', 'challengeKey', 'searchCursorKey'].every(
      (name) => isSecret(secrets[name]),
    ) ||
    !isCanalSecret(secrets.canal)
  )
    throw Error('SERVER_CREDENTIALS_INVALID');
  if (
    readFileSync(resolve(directory, 'compose.env'), 'utf8') !== composeEnvironment(loaded.manifest)
  )
    throw Error('SERVER_COMPOSE_BINDING_MISMATCH');
  if (
    readFileSync(resolve(directory, 'admin/mysql-root-password'), 'utf8') !== secrets.root + '\n' ||
    readFileSync(resolve(directory, 'admin/canal.env'), 'utf8') !==
      `canal.instance.dbPassword=${secrets.canal}\n`
  )
    throw Error('SERVER_BOOTSTRAP_BINDING_MISMATCH');
  // 宿主先核验准备目录中的身份和网络绑定，不能等启动后才发现Go DSN与Node配置不同。
  // 摘要在initialize用真实迁移回执更新；Doc的OSS/转换运维配置不被prepare覆盖。
  for (const role of roles) {
    const actual = readJSON(resolve(directory, 'application/services', role + '.json'));
    const expected = serviceConfiguration(role, manifest, secrets, {});
    const keys = [
      'role',
      'root',
      'httpPort',
      'rpcPort',
      'host',
      'dsn',
      'database',
      'certFile',
      'keyFile',
      'caFile',
      'origin',
      'peers',
      ...(role === 'account' ? ['redis', 'challengeKey'] : []),
      ...(role === 'search' ? ['search'] : []),
    ];
    if (keys.some((key) => JSON.stringify(actual[key]) !== JSON.stringify(expected[key])))
      throw Error('SERVER_SERVICE_CONFIGURATION_MISMATCH');
    // SMTP可通过独立配置步骤选择TLS/STARTTLS，仍严格核公开字段；
    // 不能把它与默认开发段比较，从而拒绝或覆盖已配置的真实发信通道。
    if (role === 'account') smtpSettings(actual.smtp);
  }
  for (const role of [...roles, 'edge']) {
    for (const suffix of ['.pem', '-key.pem']) {
      if (!existsSync(resolve(directory, 'application/pki', role + suffix)))
        throw Error('SERVER_CERTIFICATE_MISSING');
    }
  }
  return loaded;
}
