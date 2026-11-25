import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomBytes } from 'node:crypto';
import { createServer } from 'node:net';
import mysql from 'mysql2/promise';
import {
  state,
  stateFile,
  saveJSON,
  runtime,
  compose,
  ownedContainer,
  docker,
  pause,
} from './runtime-lib.mjs';
import { writeComposeEnvironment } from './compose-environment.mjs';
import { searchServiceConfig } from './search-config.mjs';
import { assertContainerOwner } from './ownership.mjs';

const topic = 'collab-publications-v1';
const stage = (name, facts = {}) => console.log(JSON.stringify({ stage: name, ...facts }));
async function freePort(start) {
  for (let port = start; port < start + 200; port++) {
    const server = createServer();
    try {
      await new Promise((done, failed) => {
        server.once('error', failed);
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
  throw Error('SEARCH_PORTS_UNAVAILABLE');
}
async function poll(check, code) {
  for (let attempt = 0; attempt < 80; attempt++) {
    try {
      if (await check()) return;
    } catch {}
    await pause(500);
  }
  throw Error(code);
}
async function limitedCanal(configuration) {
  const connection = await mysql.createConnection({
    host: '127.0.0.1',
    port: configuration.ports.mysql,
    user: 'collab_canal',
    password: configuration.passwords.canal,
    connectTimeout: 2000,
  });
  try {
    await connection.query('SELECT COUNT(*) n FROM document_db.outbox');
    for (const table of ['document_db.document', 'account_db.user', 'search_db.consumed_event']) {
      let denied = false;
      try {
        await connection.query('SELECT * FROM ' + table + ' LIMIT 0');
      } catch (error) {
        denied = error.errno === 1142;
      }
      if (!denied) throw Error('CANAL_PRIVATE_LIBRARY_NOT_DENIED');
    }
  } finally {
    await connection.end();
  }
}
function verifyExistingVolumes(configuration) {
  const existing = new Set(docker('volume', 'ls', '--format', '{{.Name}}').split(/\r?\n/));
  for (const role of ['kafka-data', 'es-data', 'canal-position']) {
    const name = configuration.project + '_' + role;
    if (!existing.has(name)) continue;
    const labels = JSON.parse(docker('volume', 'inspect', '--format', '{{json .Labels}}', name));
    // 容器可能已退出或重建；初始化目录前还要单独核对持久卷，不能只信容器标签。
    assertContainerOwner(
      {
        owner: labels?.['collaboration.report.owner'],
        project: labels?.['com.docker.compose.project'],
      },
      configuration,
    );
  }
}
async function run() {
  const configuration = state();
  ownedContainer('mysql');
  if (Boolean(configuration.ports.kafka) !== Boolean(configuration.ports.es))
    throw Error('SEARCH_ENVIRONMENT_STATE_INVALID');
  if (!configuration.ports.kafka) {
    const start = Math.max(...Object.values(configuration.ports)) + 1;
    configuration.ports.kafka = await freePort(start);
    configuration.ports.es = await freePort(configuration.ports.kafka + 1);
  }
  // 秘密先持久保存，DDL的未知结果才能用原秘密核验；不覆盖不明账号，不每次轮换密码。
  if (!configuration.passwords.canal)
    configuration.passwords.canal = randomBytes(12).toString('hex');
  if (!configuration.passwords.searchCursorKey)
    configuration.passwords.searchCursorKey = randomBytes(32).toString('hex');
  saveJSON(stateFile, configuration);
  writeComposeEnvironment(configuration);
  const searchFile = resolve(runtime, 'services/search.json');
  const searchConfig = JSON.parse(readFileSync(searchFile));
  searchConfig.search = searchServiceConfig(configuration);
  saveJSON(searchFile, searchConfig);

  const connection = await mysql.createConnection({
    host: '127.0.0.1',
    port: configuration.ports.mysql,
    user: 'root',
    password: configuration.passwords.root,
    connectTimeout: 2000,
  });
  try {
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
    const [[account]] = await connection.query(
      "SELECT COUNT(*) n FROM mysql.user WHERE User='collab_canal' AND Host='%'",
    );
    if (!Number(account.n))
      await connection.query("CREATE USER 'collab_canal'@'%' IDENTIFIED BY ?", [
        configuration.passwords.canal,
      ]);
    else {
      // 原账号存在时只能用已持久原秘密证明归属；验证失败不ALTER USER接管一个未知账号。
      const original = await mysql.createConnection({
        host: '127.0.0.1',
        port: configuration.ports.mysql,
        user: 'collab_canal',
        password: configuration.passwords.canal,
        connectTimeout: 2000,
      });
      await original.end();
    }
    await connection.query(
      "GRANT REPLICATION SLAVE,REPLICATION CLIENT ON *.* TO 'collab_canal'@'%'",
    );
    await connection.query("GRANT SELECT ON document_db.outbox TO 'collab_canal'@'%'");
    const [grants] = await connection.query("SHOW GRANTS FOR 'collab_canal'@'%'");
    if (
      !grants.every((row) =>
        Object.values(row).every(
          (grant) =>
            /^GRANT (?:USAGE|REPLICATION SLAVE|REPLICATION CLIENT|REPLICATION SLAVE, REPLICATION CLIENT|REPLICATION CLIENT, REPLICATION SLAVE) ON \*\.\* TO /.test(
              grant,
            ) || /^GRANT SELECT ON `document_db`\.`outbox` TO /.test(grant),
        ),
      )
    )
      throw Error('CANAL_PRIVILEGES_NOT_LIMITED');
    await limitedCanal(configuration);
    stage('SEARCH_REPLICATION_ACCOUNT_READY', {
      originalMySQLRetained: true,
      privateLibrariesDenied: true,
    });
    for (const role of ['kafka-volume-init', 'kafka', 'es', 'canal']) {
      if (compose('ps', '--all', '-q', role)) ownedContainer(role);
    }
    verifyExistingVolumes(configuration);
    compose('up', '-d', 'kafka', 'es');
    const kafka = ownedContainer('kafka');
    ownedContainer('es');
    await poll(() => {
      docker(
        'exec',
        kafka,
        '/opt/kafka/bin/kafka-topics.sh',
        '--bootstrap-server',
        'localhost:29092',
        '--list',
      );
      return true;
    }, 'KAFKA_STARTUP_TIMEOUT');
    docker(
      'exec',
      kafka,
      '/opt/kafka/bin/kafka-topics.sh',
      '--bootstrap-server',
      'localhost:29092',
      '--create',
      '--if-not-exists',
      '--topic',
      topic,
      '--partitions',
      '3',
      '--replication-factor',
      '1',
    );
    const description = docker(
      'exec',
      kafka,
      '/opt/kafka/bin/kafka-topics.sh',
      '--bootstrap-server',
      'localhost:29092',
      '--describe',
      '--topic',
      topic,
    );
    if (!/PartitionCount:\s*3\b/.test(description)) throw Error('KAFKA_TOPIC_PARTITIONS_CHANGED');
    const version = docker('exec', kafka, '/opt/kafka/bin/kafka-topics.sh', '--version');
    if (!/^3\.7\.0\b/.test(version)) throw Error('KAFKA_VERSION_MISMATCH');
    await poll(async () => {
      const response = await fetch(
        `http://127.0.0.1:${configuration.ports.es}/_cluster/health?wait_for_status=yellow&timeout=1s`,
        { signal: AbortSignal.timeout(1500) },
      );
      const health = await response.json();
      return response.ok && !health.timed_out && ['yellow', 'green'].includes(health.status);
    }, 'ES_STARTUP_TIMEOUT');
    const elastic = await (
      await fetch(`http://127.0.0.1:${configuration.ports.es}`, {
        signal: AbortSignal.timeout(1500),
      })
    ).json();
    if (elastic.version.number !== '8.19.4') throw Error('ES_VERSION_MISMATCH');
    stage('SEARCH_BROKER_INDEX_DEPENDENCIES_READY', {
      kafkaVersion: '3.7.0',
      esVersion: '8.19.4',
      partitions: 3,
    });
    compose('up', '-d', 'canal');
    ownedContainer('canal');
    // Running容器不是复制就绪；原MySQL中必须出现实际受限账号的binlog dump线程。
    await poll(async () => {
      const [[replication]] = await connection.query(
        "SELECT COUNT(*) n FROM performance_schema.threads WHERE PROCESSLIST_COMMAND IN ('Binlog Dump','Binlog Dump GTID') AND PROCESSLIST_USER='collab_canal'",
      );
      return Number(replication.n) === 1;
    }, 'CANAL_REPLICATION_STARTUP_TIMEOUT');
    stage('SEARCH_DEPENDENCIES_READY', {
      ownerVerified: true,
      replicationConnected: true,
      goConsumerImplemented: false,
      applicationIndexReady: false,
    });
  } finally {
    await connection.end();
  }
}
run().catch((error) => {
  stage('SEARCH_DEPENDENCIES_FAILED', {
    code: /^[A-Z0-9_]+$/.test(error.message) ? error.message : 'SEARCH_ENVIRONMENT_FAILED',
  });
  process.exitCode = 1;
});
