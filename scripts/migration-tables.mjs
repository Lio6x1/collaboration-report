const schemas = new Set(['account_db', 'document_db', 'collaboration_db', 'search_db']);
const identifier = /^[a-z][a-z0-9_]*$/;
const columnType =
  /^(?:bigint unsigned|int unsigned|varchar\([1-9][0-9]*\)|binary\([1-9][0-9]*\)|mediumblob|datetime\(6\))$/;

export function validateTableManifest(manifest) {
  if (!Array.isArray(manifest.tables) || !manifest.tables.length)
    throw Error('MIGRATION_MANIFEST_INVALID');
  const seen = new Set();
  for (const table of manifest.tables) {
    const key = table.database + '.' + table.name;
    if (
      !schemas.has(table.database) ||
      !identifier.test(table.name) ||
      seen.has(key) ||
      !Array.isArray(table.columns) ||
      !table.columns.length ||
      !Array.isArray(table.primaryKey) ||
      !table.primaryKey.length
    )
      throw Error('MIGRATION_MANIFEST_INVALID');
    seen.add(key);
    const names = new Set();
    for (const column of table.columns) {
      if (
        !identifier.test(column.name) ||
        names.has(column.name) ||
        !columnType.test(column.type) ||
        typeof column.nullable !== 'boolean' ||
        (column.collation !== undefined && column.collation !== 'ascii_bin')
      )
        throw Error('MIGRATION_MANIFEST_INVALID');
      names.add(column.name);
    }
    if (
      new Set(table.primaryKey).size !== table.primaryKey.length ||
      table.primaryKey.some((name) => !names.has(name))
    )
      throw Error('MIGRATION_MANIFEST_INVALID');
  }
}

export async function tableCatalog(connection, manifest) {
  validateTableManifest(manifest);
  const where = manifest.tables.map(() => '(TABLE_SCHEMA=? AND TABLE_NAME=?)').join(' OR ');
  const values = manifest.tables.flatMap((table) => [table.database, table.name]);
  const [columns] = await connection.query(
    `SELECT TABLE_SCHEMA database_name,TABLE_NAME table_name,
    COLUMN_NAME column_name,ORDINAL_POSITION position,LOWER(COLUMN_TYPE) column_type,
    IS_NULLABLE nullable,COLLATION_NAME collation FROM information_schema.COLUMNS WHERE ${where}
    ORDER BY TABLE_SCHEMA,TABLE_NAME,ORDINAL_POSITION`,
    values,
  );
  const [keys] = await connection.query(
    `SELECT TABLE_SCHEMA database_name,TABLE_NAME table_name,
    COLUMN_NAME column_name,SEQ_IN_INDEX position FROM information_schema.STATISTICS
    WHERE INDEX_NAME='PRIMARY' AND (${where}) ORDER BY TABLE_SCHEMA,TABLE_NAME,SEQ_IN_INDEX`,
    values,
  );
  return { columns, keys };
}

export function verifyTableCatalog(catalog, manifest) {
  // 同名表或部分DDL不能冒充完整安装；核对全部列、无符号类型、可空性及复合主键顺序。
  for (const table of manifest.tables) {
    const matches = (row) => row.database_name === table.database && row.table_name === table.name;
    const columns = catalog.columns.filter(matches);
    const keys = catalog.keys.filter(matches);
    if (
      columns.length !== table.columns.length ||
      keys.length !== table.primaryKey.length ||
      columns.some((column, index) => {
        const expected = table.columns[index];
        return (
          Number(column.position) !== index + 1 ||
          column.column_name !== expected.name ||
          column.column_type !== expected.type ||
          column.nullable !== (expected.nullable ? 'YES' : 'NO') ||
          (expected.collation !== undefined && column.collation !== expected.collation)
        );
      }) ||
      keys.some(
        (key, index) =>
          Number(key.position) !== index + 1 || key.column_name !== table.primaryKey[index],
      )
    )
      throw Error('MIGRATION_TABLE_MISMATCH');
  }
  if (
    catalog.columns.length !==
      manifest.tables.reduce((count, table) => count + table.columns.length, 0) ||
    catalog.keys.length !==
      manifest.tables.reduce((count, table) => count + table.primaryKey.length, 0)
  )
    throw Error('MIGRATION_TABLE_MISMATCH');
}

// 约束变更只接受无字符串、仅由“列<=整数 AND …”组成的上限表达式。
// 先验证括号只包围完整比较/AND组合，不能把布尔结果再比较的恒真式误认成上限。
function boundExpression(value) {
  const fail = () => {
    throw Error('MIGRATION_CHECK_MISMATCH');
  };
  if (typeof value !== 'string' || value.length > 4096) fail();
  const input = value.replace(/`([a-z][a-z0-9_]*)`/gi, '$1').toLowerCase();
  const tokens = input.match(/[a-z][a-z0-9_]*|[0-9]+|<=|[()]/g) ?? [];
  if (!tokens.length || tokens.join('') !== input.replace(/\s/g, '')) fail();
  let offset = 0;
  function expression(depth) {
    if (depth > 16) fail();
    const clauses = term(depth);
    while (tokens[offset] === 'and') {
      offset++;
      clauses.push(...term(depth));
    }
    return clauses;
  }
  function term(depth) {
    if (tokens[offset] === '(') {
      offset++;
      const clauses = expression(depth + 1);
      if (tokens[offset++] !== ')') fail();
      return clauses;
    }
    const column = tokens[offset++],
      operator = tokens[offset++],
      bound = tokens[offset++];
    if (
      !identifier.test(column ?? '') ||
      column === 'and' ||
      operator !== '<=' ||
      !/^[0-9]+$/.test(bound ?? '')
    )
      fail();
    return [`${column}<=${bound}`];
  }
  const clauses = expression(0);
  if (offset !== tokens.length) fail();
  return clauses.join('and');
}

export function validateCheckManifest(manifest) {
  if (!Array.isArray(manifest.checks) || !manifest.checks.length)
    throw Error('MIGRATION_MANIFEST_INVALID');
  const seen = new Set();
  for (const check of manifest.checks) {
    if (!schemas.has(check.database) || !identifier.test(check.table))
      throw Error('MIGRATION_MANIFEST_INVALID');
    for (const side of [check.before, check.after]) {
      const key = `${check.database}.${check.table}.${side?.name}`;
      if (!identifier.test(side?.name ?? '') || seen.has(key))
        throw Error('MIGRATION_MANIFEST_INVALID');
      seen.add(key);
      boundExpression(side.expression);
    }
  }
}

export async function checkCatalog(connection, manifest) {
  validateCheckManifest(manifest);
  const where = manifest.checks
    .map(() => '(t.TABLE_SCHEMA=? AND t.TABLE_NAME=? AND t.CONSTRAINT_NAME IN (?,?))')
    .join(' OR ');
  const [rows] = await connection.query(
    `SELECT t.TABLE_SCHEMA database_name,t.TABLE_NAME table_name,
    t.CONSTRAINT_NAME check_name,c.CHECK_CLAUSE expression,t.ENFORCED enforced
    FROM information_schema.TABLE_CONSTRAINTS t JOIN information_schema.CHECK_CONSTRAINTS c
    ON c.CONSTRAINT_SCHEMA=t.CONSTRAINT_SCHEMA AND c.CONSTRAINT_NAME=t.CONSTRAINT_NAME
    WHERE t.CONSTRAINT_TYPE='CHECK' AND (${where})`,
    manifest.checks.flatMap((check) => [
      check.database,
      check.table,
      check.before.name,
      check.after.name,
    ]),
  );
  return rows;
}

export function verifyCheckCatalog(rows, manifest, phase = 'after') {
  if (rows.length !== manifest.checks.length) throw Error('MIGRATION_CHECK_MISMATCH');
  for (const check of manifest.checks) {
    const expected = check[phase];
    const actual = rows.filter(
      (row) => row.database_name === check.database && row.table_name === check.table,
    );
    if (
      actual.length !== 1 ||
      actual[0].check_name !== expected.name ||
      actual[0].enforced !== 'YES' ||
      boundExpression(actual[0].expression) !== boundExpression(expected.expression)
    )
      throw Error('MIGRATION_CHECK_MISMATCH');
  }
}
