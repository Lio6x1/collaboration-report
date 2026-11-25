import test from 'node:test';
import assert from 'node:assert/strict';
import mysql from 'mysql2/promise';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, runtime, state } from '../../scripts/runtime-lib.mjs';
import { installMigration } from '../../scripts/migration.mjs';
import { indexCatalog, verifyIndexCatalog } from '../../scripts/migration-indexes.mjs';
import { tableCatalog, verifyTableCatalog } from '../../scripts/migration-tables.mjs';
const env = state(),
  databases = ['account_db', 'document_db', 'collaboration_db', 'search_db'];
const admin = () =>
  mysql.createConnection({
    host: '127.0.0.1',
    port: env.ports.mysql,
    user: 'root',
    password: env.passwords.root,
    multipleStatements: true,
  });
test('baseline plus version three has 103 tables, 145 foreign keys and identical repeat-install receipt', async () => {
  const db = await admin();
  try {
    const [[t]] = await db.query(
      'SELECT COUNT(*) AS n FROM information_schema.TABLES WHERE TABLE_SCHEMA IN (?,?,?,?)',
      databases,
    );
    const [[f]] = await db.query(
      "SELECT COUNT(*) AS n FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_TYPE='FOREIGN KEY' AND TABLE_SCHEMA IN (?,?,?,?)",
      databases,
    );
    // 001的100表/144外键保留，003明确追加三张Search表和一个本库原消息外键。
    assert.equal(t.n, 103);
    assert.equal(f.n, 145);
    assert.equal(
      (await installMigration(db, resolve(root, 'db/migrations'))).state,
      'ALREADY_INSTALLED',
    );
  } finally {
    await db.end();
  }
});

test('additional indexes have their exact definition and every private migration receipt', async () => {
  const db = await admin();
  const manifest = JSON.parse(readFileSync(resolve(root, 'db/migrations/002-manifest.json')));
  const third = JSON.parse(readFileSync(resolve(root, 'db/migrations/003-manifest.json')));
  try {
    verifyIndexCatalog(await indexCatalog(db, manifest), manifest);
    verifyTableCatalog(await tableCatalog(db, third), third);
    for (const schema of databases) {
      const [rows] = await db.query(
        `SELECT version,LOWER(HEX(checksum)) checksum FROM ${schema}.schema_migration ORDER BY version`,
      );
      assert.deepEqual(
        rows.map((row) => row.version),
        [1, 2, 3],
      );
      assert.equal(rows[1].checksum, manifest.sha256);
      assert.equal(rows[2].checksum, third.sha256);
    }
  } finally {
    await db.end();
  }
});
for (const role of ['account', 'document', 'collaboration', 'search']) {
  test(`${role} private database rejects cross-schema, migration write and DDL`, async () => {
    const cfg = JSON.parse(readFileSync(resolve(runtime, 'services', role + '.json')));
    // 部署凭据不进应用配置；服务账号只持有所属数据库的逐表权限。
    assert(!JSON.stringify(cfg).includes(env.passwords.root));
    const db = await mysql.createConnection({
      host: '127.0.0.1',
      port: env.ports.mysql,
      user: 'collab_' + role,
      password: env.passwords[role],
      database: role + '_db',
    });
    try {
      await db.query("SET time_zone='+00:00'");
      const [[zone]] = await db.query('SELECT @@session.time_zone AS value');
      assert.equal(zone.value, '+00:00');
      for (const schema of databases.filter((x) => x !== role + '_db'))
        await assert.rejects(db.query(`SELECT * FROM ${schema}.schema_migration`), (e) =>
          [1044, 1142].includes(e.errno),
        );
      await assert.rejects(
        db.query('UPDATE schema_migration SET version=version'),
        (e) => e.errno === 1142,
      );
      await assert.rejects(
        db.query('CREATE TABLE bootstrap_forbidden(id INT)'),
        (e) => e.errno === 1142,
      );
      if (role === 'document')
        await assert.rejects(
          // 用真实表验证不可变版本权限；不存在的占位表也会1142，不能构成该保证。
          db.query('UPDATE content_version SET id=id'),
          (e) => e.errno === 1142,
        );
    } finally {
      await db.end();
    }
  });
}
