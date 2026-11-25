import assert from 'node:assert/strict';
import test from 'node:test';
import {
  validateTableManifest,
  verifyTableCatalog,
  validateCheckManifest,
  verifyCheckCatalog,
} from '../../scripts/migration-tables.mjs';

const manifest = {
  tables: [
    {
      database: 'search_db',
      name: 'index_application',
      primaryKey: ['physical_index', 'document_id'],
      columns: [
        { name: 'physical_index', type: 'varchar(160)', nullable: false, collation: 'ascii_bin' },
        { name: 'document_id', type: 'bigint unsigned', nullable: false },
        { name: 'authority_digest', type: 'binary(32)', nullable: false },
      ],
    },
  ],
};
function catalog() {
  return {
    columns: manifest.tables[0].columns.map((column, index) => ({
      database_name: 'search_db',
      table_name: 'index_application',
      column_name: column.name,
      position: index + 1,
      column_type: column.type,
      nullable: 'NO',
      collation: column.collation ?? null,
    })),
    keys: ['physical_index', 'document_id'].map((name, index) => ({
      database_name: 'search_db',
      table_name: 'index_application',
      column_name: name,
      position: index + 1,
    })),
  };
}
test('逐物理代次主键及完整无符号字段必须与迁移声明一致', () => {
  validateTableManifest(manifest);
  verifyTableCatalog(catalog(), manifest);
});
test('丢失unsigned、摘要变可空、主键顺序改变或多余字段均不算完整安装', () => {
  for (const alter of [
    (value) => {
      value.columns[1].column_type = 'bigint';
    },
    (value) => {
      value.columns[2].nullable = 'YES';
    },
    (value) => {
      value.keys.reverse();
    },
    (value) => {
      value.columns.push({ ...value.columns[0], position: 4, column_name: 'unexpected' });
    },
  ]) {
    const value = catalog();
    alter(value);
    assert.throws(() => verifyTableCatalog(value, manifest), /MIGRATION_TABLE_MISMATCH/);
  }
});
test('第三版manifest的外部库或标识符注入不能成为DDL核验依据', () => {
  for (const name of ['mysql', 'unknown;DROP']) {
    const value = structuredClone(manifest);
    value.tables[0].database = name;
    assert.throws(() => validateTableManifest(value), /MIGRATION_MANIFEST_INVALID/);
  }
});

const bounds = {
  checks: [
    {
      database: 'document_db',
      table: 'export_budget',
      before: {
        name: 'export_budget_chk_2',
        expression: 'reserved_bytes<=268435456 AND used_bytes<=268435456',
      },
      after: {
        name: 'export_pool_budget_bounds',
        expression: 'reserved_bytes<=2147483648 AND used_bytes<=2147483648',
      },
    },
  ],
};
const boundRow = (phase) => ({
  database_name: 'document_db',
  table_name: 'export_budget',
  check_name: bounds.checks[0][phase].name,
  expression: bounds.checks[0][phase].expression,
  enforced: 'YES',
});

test('约束升级先核对精确旧定义，再核对已启用的新定义', () => {
  validateCheckManifest(bounds);
  verifyCheckCatalog([boundRow('before')], bounds, 'before');
  const row = boundRow('after');
  row.expression = '((`reserved_bytes` <= 2147483648) and (`used_bytes` <= 2147483648))';
  verifyCheckCatalog([row], bounds);
});

test('未登记的新约束、旧约束残留、禁用或改变额度均不能冒充完整安装', () => {
  assert.throws(
    () => verifyCheckCatalog([boundRow('after')], bounds, 'before'),
    /MIGRATION_CHECK_MISMATCH/,
  );
  for (const rows of [
    [],
    [boundRow('before')],
    [boundRow('before'), boundRow('after')],
    [{ ...boundRow('after'), enforced: 'NO' }],
    [{ ...boundRow('after'), expression: 'reserved_bytes<=1 AND used_bytes<=2147483648' }],
    [{ ...boundRow('after'), expression: 'reserved_bytes<=2147483648 OR used_bytes<=2147483648' }],
    // 这是把AND的布尔结果再与大整数比较的恒真式，不能因删除括号后相同就认可。
    [
      {
        ...boundRow('after'),
        expression: '((reserved_bytes<=2147483648 AND used_bytes)<=2147483648)',
      },
    ],
  ])
    assert.throws(() => verifyCheckCatalog(rows, bounds), /MIGRATION_CHECK_MISMATCH/);
});
