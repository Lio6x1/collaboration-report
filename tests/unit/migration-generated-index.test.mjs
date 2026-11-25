import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { verifyIndexCatalog } from '../../scripts/migration-indexes.mjs';

test('派生清理索引认可MySQL实际转义格式，拒绝改变JSON路径大小写', () => {
  // 真实006安装发现information_schema对表达式中的引号再次转义。
  // 格式可以规范化，绑定路径改变仍须拒绝，不能用同名索引掩盖错误清理来源。
  const manifest = JSON.parse(
    readFileSync(new URL('../../db/migrations/006-manifest.json', import.meta.url)),
  );
  const index = manifest.indexes[0];
  const rows = index.columns.map((column, offset) => ({
    database_name: index.database,
    table_name: index.table,
    index_name: index.name,
    column_name: column,
    position: offset + 1,
    non_unique: 1,
    index_type: 'BTREE',
    prefix_length: null,
    visible: 'YES',
    sort_order: 'A',
  }));
  const generated = index.generatedColumns[0];
  Object.assign(rows[0], {
    column_type: generated.type,
    column_collation: generated.collation,
    column_extra: 'VIRTUAL GENERATED',
    generation_expression: generated.expression
      .replace(/payload/g, '`payload`')
      .replace(/'([^']+)'/g, "_utf8mb4\\'$1\\'"),
  });
  assert.doesNotThrow(() => verifyIndexCatalog(rows, manifest));
  rows[0].generation_expression = rows[0].generation_expression.replace('$.context', '$.Context');
  assert.throws(() => verifyIndexCatalog(rows, manifest), /MIGRATION_INDEX_MISMATCH/);
  rows[0].generation_expression = generated.expression.replace('$.context', '$. context');
  assert.throws(() => verifyIndexCatalog(rows, manifest), /MIGRATION_INDEX_MISMATCH/);
});
