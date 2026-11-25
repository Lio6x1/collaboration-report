import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import net from 'node:net';
import mysql from 'mysql2/promise';
import { root, runtime, state, pause } from './runtime-lib.mjs';
import { installMigration } from './migration.mjs';
import { canonicalGrant } from './server-grants.mjs';
import {
  applicationRoot,
  roles,
  ports,
  readJSON,
  isSecret,
  isCanalSecret,
  privateJSON,
  migrationExpectation,
} from './server-configuration.mjs';

const stage = (name, facts = {}) => console.log(JSON.stringify({ stage: name, ...facts }));

async function grants(connection, user) {
  const [rows] = await connection.query(`SHOW GRANTS FOR '${user}'@'%'`);
  return rows
    .flatMap((row) => Object.values(row))
    .map(canonicalGrant)
    .filter((grant) => grant !== 'USAGE ON *.*');
}

async function ensureAccount(connection, user, password, expected) {
  const [[row]] = await connection.query(
    'SELECT COUNT(*) n FROM mysql.user WHERE User=? AND Host=?',
    [user, '%'],
  );
  if (Number(row.n) === 0) {
    await connection.query(`CREATE USER '${user}'@'%' IDENTIFIED BY ?`, [password]);
  } else {
    // 重试必须用已保存秘密证明原账号；不能ALTER USER接管未知数据卷里的同名账号。
    const proof = await mysql.createConnection({
      host: 'mysql',
      port: 3306,
      user,
      password,
      connectTimeout: 3000,
    });
    await proof.end();
    const actual = await grants(connection, user);
    if (actual.some((grant) => !expected.has(grant)))
      throw Error('SERVER_ACCOUNT_PRIVILEGES_EXCESS');
  }
}

async function verifyAccount(connection, user, expected) {
  const actual = new Set(await grants(connection, user));
  if (actual.size !== expected.size || [...expected].some((grant) => !actual.has(grant)))
    throw Error('SERVER_ACCOUNT_PRIVILEGES_MISMATCH');
}

async function privateLibraries(user, password, ownRole) {
  const connection = await mysql.createConnection({
    host: 'mysql',
    port: 3306,
    user,
    password,
    connectTimeout: 3000,
  });
  try {
    for (const role of roles) {
      if (role === ownRole) {
        await connection.query(`SELECT version FROM ${role}_db.schema_migration`);
        continue;
      }
      let denied = false;
      try {
        await connection.query(`SELECT version FROM ${role}_db.schema_migration LIMIT 0`);
      } catch (error) {
        denied = error.errno === 1142;
      }
      if (!denied) throw Error('SERVER_PRIVATE_LIBRARY_NOT_DENIED');
    }
    if (ownRole === 'canal') await connection.query('SELECT COUNT(*) n FROM document_db.outbox');
  } finally {
    await connection.end();
  }
}

function redisCommand(user, password, args) {
  // 初始化只需有限RESP2状态核验；整段应答后才认成功，连接失败不能当成ACL已加载。
  const encode = (items) =>
    Buffer.from(
      '*' +
        items.length +
        '\r\n' +
        items
          .map((item) => '$' + Buffer.byteLength(String(item)) + '\r\n' + item + '\r\n')
          .join(''),
    );
  return new Promise((done, reject) => {
    const socket = net.connect(6379, 'redis');
    let buffer = Buffer.alloc(0),
      replies = 0,
      settled = false;
    const finish = (error, value) => {
      if (settled) return;
      settled = true;
      socket.destroy();
      error ? reject(error) : done(value);
    };
    socket.setTimeout(3000, () => finish(Error('SERVER_REDIS_TIMEOUT')));
    socket.once('connect', () =>
      socket.write(Buffer.concat([encode(['AUTH', user, password]), encode(args)])),
    );
    socket.on('data', (bytes) => {
      buffer = Buffer.concat([buffer, bytes]);
      if (buffer.length > 65536) return finish(Error('SERVER_REDIS_REPLY_INVALID'));
      for (;;) {
        const end = buffer.indexOf('\r\n');
        if (end < 0) return;
        const header = buffer.subarray(0, end).toString('utf8');
        let consumed = end + 2;
        let reply;
        // ACL DRYRUN的拒绝说明是RESP bulk string，AUTH/PING是simple string。
        // 按声明长度等完整帧，不能把正常的权限拒绝误判为权限过宽。
        if (header.startsWith('$')) {
          if (!/^\$(0|[1-9][0-9]*)$/.test(header))
            return finish(Error('SERVER_REDIS_REPLY_INVALID'));
          const size = Number(header.slice(1));
          if (size > 65536) return finish(Error('SERVER_REDIS_REPLY_INVALID'));
          if (buffer.length < consumed + size + 2) return;
          if (buffer.subarray(consumed + size, consumed + size + 2).toString() !== '\r\n')
            return finish(Error('SERVER_REDIS_REPLY_INVALID'));
          reply = buffer.subarray(consumed, consumed + size).toString('utf8');
          consumed += size + 2;
        } else if (header.startsWith('+') || header.startsWith('-')) {
          reply = header.slice(1);
        } else return finish(Error('SERVER_REDIS_REPLY_INVALID'));
        buffer = buffer.subarray(consumed);
        if (header.startsWith('-') || reply.includes('has no permissions'))
          return finish(
            Error(
              reply.includes('NOPERM') || reply.includes('has no permissions')
                ? 'SERVER_REDIS_PERMISSION_DENIED'
                : 'SERVER_REDIS_REJECTED',
            ),
          );
        if (++replies === 1 && reply !== 'OK') return finish(Error('SERVER_REDIS_REJECTED'));
        if (replies === 2) return finish(null, reply);
      }
    });
    socket.once('error', () => finish(Error('SERVER_REDIS_UNAVAILABLE')));
    socket.once('close', () => finish(Error('SERVER_REDIS_UNAVAILABLE')));
  });
}

async function initialize() {
  if (process.platform !== 'linux' || root !== applicationRoot)
    throw Error('SERVER_INITIALIZE_REQUIRES_APPLICATION_IMAGE');
  const environment = state();
  const secrets = readJSON('/run/collab-admin/credentials.json');
  if (
    environment.deployment !== 'linux-compose' ||
    environment.root !== root ||
    environment.owner !== secrets.owner ||
    environment.project !== secrets.project ||
    JSON.stringify(environment.ports) !== JSON.stringify(ports) ||
    ![...roles, 'root', 'redisAdmin', 'redisAccount', 'challengeKey', 'searchCursorKey'].every(
      (name) => isSecret(secrets[name]),
    ) ||
    !isCanalSecret(secrets.canal)
  )
    throw Error('SERVER_CONFIGURATION_MISMATCH');
  const files = roles.map((role) => resolve(runtime, 'services', role + '.json'));
  const configurations = files.map(readJSON);
  const expectation = migrationExpectation(resolve(root, 'db/migrations'));
  for (const [index, role] of roles.entries()) {
    const config = configurations[index];
    if (
      config.role !== role ||
      config.root !== root ||
      config.origin !== environment.publicOrigin ||
      config.database?.host !== 'mysql' ||
      config.database?.port !== 3306 ||
      config.database?.user !== 'collab_' + role ||
      config.database?.database !== role + '_db' ||
      config.database?.password !== secrets[role] ||
      config.dsn !==
        `collab_${role}:${secrets[role]}@tcp(mysql:3306)/${role}_db?parseTime=true&timeout=2s&readTimeout=3s&writeTimeout=3s&loc=UTC&time_zone=%27%2B00%3A00%27` ||
      config.httpPort !== ports[role + 'Http'] ||
      config.rpcPort !== ports[role + 'Rpc']
    )
      throw Error('SERVER_SERVICE_CONFIGURATION_MISMATCH');
  }
  const account = configurations[0];
  const search = configurations[2];
  if (
    account.redis?.host !== 'redis:6379' ||
    account.redis?.user !== 'account_login' ||
    account.redis?.password !== secrets.redisAccount ||
    account.challengeKey !== secrets.challengeKey ||
    search.search?.owner !== environment.owner ||
    search.search?.group !== environment.project + '-search' ||
    search.search?.topic !== 'collab-publications-v1' ||
    search.search?.cursorKey !== secrets.searchCursorKey ||
    JSON.stringify(search.search?.brokers) !== '["kafka:29092"]' ||
    search.search?.elasticEndpoint !== 'http://es:9200'
  )
    throw Error('SERVER_SERVICE_CONFIGURATION_MISMATCH');
  const receiptFile = resolve(runtime, 'initialized.json');
  if (existsSync(receiptFile)) {
    const previous = readJSON(receiptFile);
    if (previous.owner !== environment.owner || previous.project !== environment.project)
      throw Error('SERVER_INITIALIZATION_OWNER_MISMATCH');
  }
  const connection = await mysql.createConnection({
    host: 'mysql',
    port: 3306,
    user: 'root',
    password: secrets.root,
    multipleStatements: true,
    connectTimeout: 5000,
  });
  let locked = false;
  try {
    const [[lock]] = await connection.query('SELECT GET_LOCK(?,10) acquired', [
      'collab_server_init_' + environment.project,
    ]);
    if (lock.acquired !== 1) throw Error('SERVER_INITIALIZATION_LOCK_UNAVAILABLE');
    locked = true;
    await connection.query("SET time_zone='+00:00'");
    const [[variables]] = await connection.query(
      'SELECT @@log_bin enabled,@@binlog_format format,@@binlog_row_image image,@@server_id server',
    );
    if (
      !variables.enabled ||
      variables.format !== 'ROW' ||
      variables.image !== 'FULL' ||
      !variables.server
    )
      throw Error('MYSQL_REPLICATION_SETTINGS_REQUIRED');
    const migration = await installMigration(connection, resolve(root, 'db/migrations'));
    if (
      migration.checksum !== expectation.expectedSQLSHA ||
      JSON.stringify(migration.additionalMigrations) !==
        JSON.stringify(expectation.expectedAdditionalMigrations)
    )
      throw Error('SERVER_MIGRATION_RECEIPT_MISMATCH');
    stage('SERVER_MIGRATIONS_INSTALLED', { migration: migration.state });
    const sql = readFileSync(resolve(root, 'db/migrations/service-grants.sql'), 'utf8').replace(
      /^\s*--.*$/gm,
      '',
    );
    const statements = sql
      .split(';')
      .map((value) => value.trim())
      .filter(Boolean);
    for (const role of roles) {
      const user = 'collab_' + role;
      const own = statements.filter((statement) => statement.includes(`TO '${user}'@'%'`));
      if (!own.length || own.some((statement) => !statement.includes(`ON ${role}_db.`)))
        throw Error('SERVER_SERVICE_GRANTS_INVALID');
      const expected = new Set(own.map(canonicalGrant));
      await ensureAccount(connection, user, secrets[role], expected);
      for (const statement of own) await connection.query(statement);
      await verifyAccount(connection, user, expected);
      await privateLibraries(user, secrets[role], role);
    }
    const canalGrants = [
      "GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'collab_canal'@'%'",
      "GRANT SELECT ON document_db.outbox TO 'collab_canal'@'%'",
    ];
    const expectedCanal = new Set(canalGrants.map(canonicalGrant));
    await ensureAccount(connection, 'collab_canal', secrets.canal, expectedCanal);
    for (const statement of canalGrants) await connection.query(statement);
    await verifyAccount(connection, 'collab_canal', expectedCanal);
    await privateLibraries('collab_canal', secrets.canal, 'canal');
    stage('SERVER_PRIVATE_ACCOUNTS_VERIFIED', {
      privateLibrariesDenied: true,
      canalOutboxOnly: true,
    });
    if (
      (await redisCommand('bootstrap_admin', secrets.redisAdmin, ['ACL', 'LOAD'])) !== 'OK' ||
      (await redisCommand('account_login', secrets.redisAccount, ['PING'])) !== 'PONG'
    )
      throw Error('SERVER_REDIS_ACL_NOT_READY');
    for (const command of [['CONFIG', 'GET', '*'], ['FLUSHALL']]) {
      let denied = false;
      // DRYRUN由管理员检查业务ACL，绝不实际执行FLUSHALL这个反例。
      try {
        await redisCommand('bootstrap_admin', secrets.redisAdmin, [
          'ACL',
          'DRYRUN',
          'account_login',
          ...command,
        ]);
      } catch (error) {
        denied = error.message === 'SERVER_REDIS_PERMISSION_DENIED';
      }
      if (!denied) throw Error('SERVER_REDIS_PRIVILEGES_EXCESS');
    }
    stage('SERVER_REDIS_ACL_VERIFIED');
    // 只有真实迁移回执确认后才更新期望摘要。资源/导出及原cursor/group配置原样保留。
    for (const [index, file] of files.entries())
      privateJSON(file, { ...configurations[index], ...expectation });
    privateJSON(receiptFile, {
      state: 'DEPENDENCIES_INITIALIZED',
      owner: environment.owner,
      project: environment.project,
      migration: expectation,
      updatedAt: new Date().toISOString(),
    });
    // index/alias、物理日志身份与恢复水位由现有Go Search bootstrap核验；不能手推checkpoint。
    stage('DEPENDENCIES_INITIALIZED', {
      applicationReady: false,
      searchBootstrap: 'APPLICATION_STARTUP',
    });
  } finally {
    if (locked)
      await connection.query('SELECT RELEASE_LOCK(?)', [
        'collab_server_init_' + environment.project,
      ]);
    await connection.end();
  }
}

async function replicationReady() {
  if (process.platform !== 'linux' || root !== applicationRoot)
    throw Error('SERVER_INITIALIZE_REQUIRES_APPLICATION_IMAGE');
  const environment = state();
  const secrets = readJSON('/run/collab-admin/credentials.json');
  if (
    environment.deployment !== 'linux-compose' ||
    environment.owner !== secrets.owner ||
    environment.project !== secrets.project ||
    !isSecret(secrets.root) ||
    !isCanalSecret(secrets.canal)
  )
    throw Error('SERVER_CONFIGURATION_MISMATCH');
  const connection = await mysql.createConnection({
    host: 'mysql',
    port: 3306,
    user: 'root',
    password: secrets.root,
    connectTimeout: 5000,
  });
  try {
    // Canal外层shell/Java存活不能证明复制已建立；应用接收发布前核实际原账号dump线程。
    const deadline = Date.now() + 180000;
    while (Date.now() < deadline) {
      const [[replication]] = await connection.query(
        "SELECT COUNT(*) n FROM performance_schema.threads WHERE PROCESSLIST_COMMAND IN ('Binlog Dump','Binlog Dump GTID') AND PROCESSLIST_USER='collab_canal'",
      );
      if (Number(replication.n) === 1) {
        stage('CANAL_REPLICATION_READY', { replicationConnected: true });
        return;
      }
      await pause(1000);
    }
    throw Error('CANAL_REPLICATION_STARTUP_TIMEOUT');
  } finally {
    await connection.end();
  }
}

const arguments_ = process.argv.slice(2);
const operation =
  arguments_.length === 0
    ? initialize
    : arguments_.length === 1 && arguments_[0] === '--replication-ready'
      ? replicationReady
      : async () => {
          throw Error('SERVER_INITIALIZATION_ARGUMENTS_INVALID');
        };
operation().catch((error) => {
  // 驱动错误可能带SQL/口令；只输出稳定阶段和错误码。
  stage('SERVER_INITIALIZATION_FAILED', {
    code: /^[A-Z0-9_]+$/.test(error.message) ? error.message : 'SERVER_INITIALIZATION_FAILED',
  });
  process.exitCode = 1;
});
