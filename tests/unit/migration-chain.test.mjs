import assert from 'node:assert/strict';
import test from 'node:test';
import { createHash } from 'node:crypto';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve, dirname, basename } from 'node:path';
import { readMigrationChain } from '../../scripts/migration-chain.mjs';

function fixture(t) {
  const folder = mkdtempSync(join(tmpdir(), 'collaboration-migration-chain-'));
  t.after(() => {
    assert.equal(dirname(resolve(folder)), resolve(tmpdir()));
    assert.ok(basename(folder).startsWith('collaboration-migration-chain-'));
    rmSync(folder, { recursive: true, force: true });
  });
  let parent;
  const hashes = [];
  for (let version = 1; version <= 3; version++) {
    const prefix = String(version).padStart(3, '0');
    const sql = `SELECT ${version};\n`;
    const sha256 = createHash('sha256').update(sql).digest('hex');
    const manifest = { version, sha256 };
    if (parent) manifest.parentSHA256 = parent;
    if (version === 2)
      manifest.indexes = [
        {
          database: 'search_db',
          table: 'durable_task',
          name: 'task_pending',
          columns: ['state', 'kind'],
        },
      ];
    if (version === 3)
      manifest.tables = [
        {
          database: 'search_db',
          name: 'example',
          primaryKey: ['id'],
          columns: [{ name: 'id', type: 'bigint unsigned', nullable: false }],
        },
      ];
    writeFileSync(join(folder, prefix + '.up.sql'), sql);
    writeFileSync(
      join(folder, version === 1 ? 'schema-manifest.json' : prefix + '-manifest.json'),
      JSON.stringify(manifest),
    );
    hashes.push(sha256);
    parent = sha256;
  }
  return { folder, hashes };
}
test('迁移链保留每版原字节和父摘要，按连续数值版本加载', (t) => {
  const { folder, hashes } = fixture(t);
  const chain = readMigrationChain(folder);
  assert.deepEqual(
    chain.map((step) => step.manifest.version),
    [1, 2, 3],
  );
  assert.deepEqual(
    chain.map((step) => step.manifest.sha256),
    hashes,
  );
  assert.equal(chain[2].bytes.toString(), 'SELECT 3;\n');
});
test('第三版被修改时在执行任何DDL之前拒绝', (t) => {
  const { folder } = fixture(t);
  writeFileSync(join(folder, '003.up.sql'), 'DROP DATABASE document_db;');
  assert.throws(() => readMigrationChain(folder), /MIGRATION_CHECKSUM_MISMATCH/);
});
test('缺失中间版本不能把第三版当作第二版安装', (t) => {
  const { folder } = fixture(t);
  rmSync(join(folder, '002-manifest.json'));
  assert.throws(() => readMigrationChain(folder), /MIGRATION_VERSION_MISMATCH/);
});
test('第三版父摘要不匹配时拒绝，即使第三版SQL自身摘要正确', (t) => {
  const { folder, hashes } = fixture(t);
  writeFileSync(
    join(folder, '003-manifest.json'),
    JSON.stringify({ version: 3, sha256: hashes[2], parentSHA256: hashes[0] }),
  );
  assert.throws(() => readMigrationChain(folder), /MIGRATION_VERSION_MISMATCH/);
});
