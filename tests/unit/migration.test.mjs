import test from 'node:test';
import assert from 'node:assert/strict';
import { validateMigration } from '../../scripts/migration.mjs';
import { installMigration } from '../../scripts/migration.mjs';
import { createHash } from 'node:crypto';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve, dirname, basename } from 'node:path';

test('changed migration bytes are rejected before installation', () => {
  // Dropping checksum validation would allow altered DDL to execute.
  assert.throws(
    () =>
      validateMigration(Buffer.from('SELECT 2;'), {
        sha256: '17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d062b4e06a63527d11a',
      }),
    /MIGRATION_CHECKSUM_MISMATCH/,
  );
});
test('untrusted malformed checksum does not authorize installation', () => {
  assert.throws(
    () => validateMigration(Buffer.from('SELECT 1;'), { sha256: '' }),
    /MIGRATION_CHECKSUM_MISMATCH/,
  );
});

const privateSchemas = ['account_db', 'document_db', 'collaboration_db', 'search_db'];
const sha = (bytes) => createHash('sha256').update(bytes).digest('hex');

function upgradeFixture(
  t,
  { installed = 1, partialIndex = false, rejectDDL = false, wrongColumn = false } = {},
) {
  const folder = mkdtempSync(join(tmpdir(), 'collaboration-migration-'));
  t.after(() => {
    assert.equal(dirname(resolve(folder)), resolve(tmpdir()));
    assert.ok(basename(folder).startsWith('collaboration-migration-'));
    rmSync(folder, { recursive: true, force: true });
  });
  const baseline = 'SELECT 1;\n';
  const upgrade = 'ALTER TABLE document_db.durable_task ADD INDEX task_pending(state,kind);\n';
  const index = {
    database: 'document_db',
    table: 'durable_task',
    name: 'task_pending',
    columns: ['state', 'kind'],
  };
  writeFileSync(join(folder, '001.up.sql'), baseline);
  writeFileSync(
    join(folder, 'schema-manifest.json'),
    JSON.stringify({ version: 1, sha256: sha(baseline) }),
  );
  writeFileSync(join(folder, '002.up.sql'), upgrade);
  writeFileSync(
    join(folder, '002-manifest.json'),
    JSON.stringify({
      version: 2,
      parentSHA256: sha(baseline),
      sha256: sha(upgrade),
      indexes: [index],
    }),
  );
  const records = Object.fromEntries(
    privateSchemas.map((schema) => [schema, [{ version: 1, checksum: sha(baseline) }]]),
  );
  if (installed === 2)
    for (const rows of Object.values(records)) rows.push({ version: 2, checksum: sha(upgrade) });
  let indexed = installed === 2 || partialIndex;
  const ddl = [];
  const connection = {
    async query(sql, values = []) {
      if (sql.includes('information_schema.SCHEMATA'))
        return [privateSchemas.map((name) => ({ name }))];
      if (sql.startsWith('SELECT GET_LOCK')) return [[{ acquired: 1 }]];
      if (sql.startsWith('SELECT RELEASE_LOCK')) return [[{ released: 1 }]];
      const read =
        /^SELECT version,LOWER\(HEX\(checksum\)\) AS checksum FROM (\w+)\.schema_migration/.exec(
          sql,
        );
      if (read) return [records[read[1]].map((row) => ({ ...row }))];
      if (sql.includes('information_schema.STATISTICS'))
        return [
          indexed
            ? index.columns.map((column, i) => ({
                database_name: index.database,
                table_name: index.table,
                index_name: index.name,
                column_name: wrongColumn && i === 1 ? 'payload' : column,
                position: i + 1,
                non_unique: 1,
                index_type: 'BTREE',
                prefix_length: null,
                visible: 'YES',
                sort_order: 'A',
              }))
            : [],
        ];
      if (sql === upgrade) {
        ddl.push(sql);
        if (rejectDDL) throw Error('INJECTED_DDL_FAILURE');
        indexed = true;
        return [{}];
      }
      const insert =
        /^INSERT INTO (\w+)\.schema_migration\(version,checksum\) VALUES\((\d+),UNHEX\(\?\)\)/.exec(
          sql,
        );
      if (insert) {
        records[insert[1]].push({ version: Number(insert[2]), checksum: values[0] });
        return [{}];
      }
      throw Error('UNEXPECTED_MIGRATION_ACTION');
    },
  };
  return { folder, connection, records, ddl, checksum: sha(upgrade) };
}

test('existing baseline appends version two without replaying original DDL', async (t) => {
  // 历史摘要必须保留；新索引不能通过重建001或覆盖旧安装回执交付。
  const f = upgradeFixture(t);
  const result = await installMigration(f.connection, f.folder);
  assert.equal(f.ddl.length, 1);
  assert.deepEqual(result.additionalMigrations, { 2: f.checksum });
  for (const rows of Object.values(f.records))
    assert.deepEqual(
      rows.map((row) => row.version),
      [1, 2],
    );
});

test('complete additional migration repeats without executing any DDL', async (t) => {
  const f = upgradeFixture(t, { installed: 2 });
  assert.equal((await installMigration(f.connection, f.folder)).state, 'ALREADY_INSTALLED');
  assert.equal(f.ddl.length, 0);
});

test('altered additional migration is rejected before any DDL', async (t) => {
  const f = upgradeFixture(t);
  writeFileSync(
    join(f.folder, '002.up.sql'),
    'ALTER TABLE document_db.durable_task DROP COLUMN payload;',
  );
  await assert.rejects(installMigration(f.connection, f.folder), /MIGRATION_CHECKSUM_MISMATCH/);
  assert.equal(f.ddl.length, 0);
});

test('unrecorded partial index installation is refused without guessing a retry', async (t) => {
  const f = upgradeFixture(t, { partialIndex: true });
  await assert.rejects(installMigration(f.connection, f.folder), /MIGRATION_PARTIAL_INSTALLATION/);
  assert.equal(f.ddl.length, 0);
});

test('failed additional DDL never records an installed version', async (t) => {
  const f = upgradeFixture(t, { rejectDDL: true });
  await assert.rejects(
    installMigration(f.connection, f.folder),
    /MIGRATION_APPLY_FAILED_REQUIRES_INSPECTION/,
  );
  for (const rows of Object.values(f.records))
    assert.deepEqual(
      rows.map((row) => row.version),
      [1],
    );
});

test('different additional versions across private databases are refused', async (t) => {
  const f = upgradeFixture(t, { installed: 2 });
  f.records.search_db.pop();
  await assert.rejects(installMigration(f.connection, f.folder), /MIGRATION_PARTIAL_INSTALLATION/);
  assert.equal(f.ddl.length, 0);
});

test('a recorded index with different columns is not treated as installed', async (t) => {
  const f = upgradeFixture(t, { installed: 2, wrongColumn: true });
  await assert.rejects(installMigration(f.connection, f.folder), /MIGRATION_INDEX_MISMATCH/);
  assert.equal(f.ddl.length, 0);
});

test('additional migration cannot bind itself to a different baseline', async (t) => {
  const f = upgradeFixture(t);
  const file = join(f.folder, '002-manifest.json');
  const manifest = JSON.parse((await import('node:fs')).readFileSync(file));
  manifest.parentSHA256 = '0'.repeat(64);
  writeFileSync(file, JSON.stringify(manifest));
  await assert.rejects(installMigration(f.connection, f.folder), /MIGRATION_VERSION_MISMATCH/);
  assert.equal(f.ddl.length, 0);
});
