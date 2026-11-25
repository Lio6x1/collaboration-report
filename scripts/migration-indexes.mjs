const schemas = new Set(['account_db', 'document_db', 'collaboration_db', 'search_db']);
const identifier = /^[a-z][a-z0-9_]*$/;

// MySQL会给表达式补反引号、字符集前缀和格式空白。路径本身保留大小写，
// 不能把$.Context与$.context规范成相同清理绑定。
const generatedExpression = (value) =>
  value
    .replace(/\\'/g, "'")
    .replace(/'[^']*'|`([^`]*)`|_utf8mb4(?=')|\s+/g, (token, identifier) =>
      token.startsWith("'") ? token : (identifier ?? ''),
    );

export function validateIndexManifest(manifest) {
  const seen = new Set();
  if (!Array.isArray(manifest.indexes) || !manifest.indexes.length)
    throw Error('MIGRATION_MANIFEST_INVALID');
  for (const index of manifest.indexes) {
    const key = `${index.database}.${index.table}.${index.name}`;
    if (
      !schemas.has(index.database) ||
      !identifier.test(index.table) ||
      !identifier.test(index.name) ||
      !Array.isArray(index.columns) ||
      !index.columns.length ||
      index.columns.some((column) => !identifier.test(column)) ||
      seen.has(key)
    )
      throw Error('MIGRATION_MANIFEST_INVALID');
    seen.add(key);
    for (const column of index.generatedColumns ?? []) {
      if (
        !index.columns.includes(column.name) ||
        column.type !== 'varchar(20)' ||
        column.collation !== 'ascii_bin' ||
        typeof column.expression !== 'string' ||
        !column.expression
      )
        throw Error('MIGRATION_MANIFEST_INVALID');
    }
  }
}

export async function indexCatalog(connection, manifest) {
  validateIndexManifest(manifest);
  const conditions = manifest.indexes.map(
    () => '(s.TABLE_SCHEMA=? AND s.TABLE_NAME=? AND s.INDEX_NAME=?)',
  );
  const values = manifest.indexes.flatMap((index) => [index.database, index.table, index.name]);
  const [rows] = await connection.query(
    `SELECT s.TABLE_SCHEMA database_name,s.TABLE_NAME table_name,s.INDEX_NAME index_name,
    s.COLUMN_NAME column_name,s.SEQ_IN_INDEX position,s.NON_UNIQUE non_unique,s.INDEX_TYPE index_type,
    s.SUB_PART prefix_length,s.IS_VISIBLE visible,s.COLLATION sort_order,
    LOWER(c.COLUMN_TYPE) column_type,c.COLLATION_NAME column_collation,c.EXTRA column_extra,
    c.GENERATION_EXPRESSION generation_expression FROM information_schema.STATISTICS s
    JOIN information_schema.COLUMNS c ON c.TABLE_SCHEMA=s.TABLE_SCHEMA AND c.TABLE_NAME=s.TABLE_NAME AND c.COLUMN_NAME=s.COLUMN_NAME
    WHERE ${conditions.join(' OR ')} ORDER BY s.TABLE_SCHEMA,s.TABLE_NAME,s.INDEX_NAME,s.SEQ_IN_INDEX`,
    values,
  );
  return rows;
}

export function verifyIndexCatalog(rows, manifest) {
  // 同名不等于同定义；列顺序、前缀、唯一性或不可见索引都可能让查询仍走宽扫描。
  for (const index of manifest.indexes) {
    const actual = rows.filter(
      (row) =>
        row.database_name === index.database &&
        row.table_name === index.table &&
        row.index_name === index.name,
    );
    if (
      actual.length !== index.columns.length ||
      actual.some(
        (row, offset) =>
          Number(row.position) !== offset + 1 ||
          row.column_name !== index.columns[offset] ||
          Number(row.non_unique) !== 1 ||
          row.index_type !== 'BTREE' ||
          row.prefix_length !== null ||
          row.visible !== 'YES' ||
          row.sort_order !== 'A',
      )
    )
      throw Error('MIGRATION_INDEX_MISMATCH');
    for (const column of index.generatedColumns ?? []) {
      const row = actual.find((row) => row.column_name === column.name);
      if (
        !row ||
        row.column_type !== column.type ||
        row.column_collation !== column.collation ||
        row.column_extra !== 'VIRTUAL GENERATED' ||
        generatedExpression(row.generation_expression ?? '') !==
          generatedExpression(column.expression)
      )
        throw Error('MIGRATION_INDEX_MISMATCH');
    }
  }
  if (rows.length !== manifest.indexes.reduce((count, index) => count + index.columns.length, 0))
    throw Error('MIGRATION_INDEX_MISMATCH');
}
