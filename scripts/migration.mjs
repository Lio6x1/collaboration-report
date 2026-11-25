import { indexCatalog, verifyIndexCatalog } from './migration-indexes.mjs';
import {
  tableCatalog,
  verifyTableCatalog,
  checkCatalog,
  verifyCheckCatalog,
} from './migration-tables.mjs';
import { readMigrationChain } from './migration-chain.mjs';
export { validateMigration } from './migration-checksum.mjs';

const schemas = ['account_db', 'document_db', 'collaboration_db', 'search_db'];

function migrationObjects(connection, manifest) {
  if (manifest.checks)
    return {
      read: () => checkCatalog(connection, manifest),
      verify: (catalog) => verifyCheckCatalog(catalog, manifest),
      // ALTER前必须是精确旧约束；已出现新约束却没有回执同样拒绝，不猜DDL提交结果。
      verifyPending: (catalog) => verifyCheckCatalog(catalog, manifest, 'before'),
    };
  const indexed = Boolean(manifest.indexes);
  return {
    read: () => (indexed ? indexCatalog(connection, manifest) : tableCatalog(connection, manifest)),
    verify: (catalog) =>
      indexed ? verifyIndexCatalog(catalog, manifest) : verifyTableCatalog(catalog, manifest),
    verifyPending: (catalog) => {
      if (indexed ? catalog.length : catalog.columns.length || catalog.keys.length)
        throw Error('MIGRATION_PARTIAL_INSTALLATION');
    },
  };
}

export async function installationState(connection, manifest, additions = []) {
  // MySQL的DDL不能整体回滚，半安装必须显式拒绝，不能假装是空库再执行。
  const [rows] = await connection.query(
    'SELECT SCHEMA_NAME AS name FROM information_schema.SCHEMATA WHERE SCHEMA_NAME IN (?,?,?,?)',
    schemas,
  );
  if (!rows.length) return 'ABSENT';
  if (rows.length !== schemas.length) throw Error('MIGRATION_PARTIAL_INSTALLATION');
  const expected = [manifest, ...additions];
  let installed;
  for (const schema of schemas) {
    let versions;
    try {
      [versions] = await connection.query(
        `SELECT version,LOWER(HEX(checksum)) AS checksum FROM ${schema}.schema_migration`,
      );
    } catch {
      throw Error('MIGRATION_PARTIAL_INSTALLATION');
    }
    versions.sort((left, right) => left.version - right.version);
    if (
      !versions.length ||
      versions.length > expected.length ||
      versions.some(
        (row, index) => row.version !== index + 1 || row.checksum !== expected[index]?.sha256,
      )
    )
      throw Error('MIGRATION_VERSION_MISMATCH');
    if (installed !== undefined && installed !== versions.length)
      throw Error('MIGRATION_PARTIAL_INSTALLATION');
    installed = versions.length;
  }
  return installed === expected.length ? 'INSTALLED' : 'BASELINE';
}

export async function installMigration(connection, folder) {
  const [baseline, ...additions] = readMigrationChain(folder);
  const { manifest, bytes } = baseline;
  const manifests = additions.map((step) => step.manifest);
  const receipt = {
    checksum: manifest.sha256,
    additionalMigrations: Object.fromEntries(manifests.map((step) => [step.version, step.sha256])),
  };
  const [[lock]] = await connection.query('SELECT GET_LOCK(?,5) AS acquired', [
    'collaboration_report_migration_v1',
  ]);
  if (lock.acquired !== 1) throw Error('MIGRATION_LOCK_UNAVAILABLE');
  try {
    // 命名锁串行化安装器；相同版本和摘要只返回已有事实，不重复建表。
    const current = await installationState(connection, manifest, manifests);
    if (current === 'ABSENT') {
      try {
        await connection.query(bytes.toString('utf8'));
        for (const schema of schemas)
          await connection.query(
            `INSERT INTO ${schema}.schema_migration(version,checksum) VALUES(1,UNHEX(?))`,
            [manifest.sha256],
          );
      } catch {
        throw Error('MIGRATION_APPLY_FAILED_REQUIRES_INSPECTION');
      }
    }
    const [installed] = await connection.query(
      'SELECT version,LOWER(HEX(checksum)) AS checksum FROM account_db.schema_migration',
    );
    const installedVersions = new Set(installed.map((row) => row.version));
    for (const step of additions) {
      const objects = migrationObjects(connection, step.manifest);
      const catalog = await objects.read();
      if (installedVersions.has(step.manifest.version)) {
        objects.verify(catalog);
        continue;
      }
      // 新版本任一对象已存在却没有四库回执，即为未知半安装，不靠IF EXISTS认可。
      objects.verifyPending(catalog);
      try {
        await connection.query(step.bytes.toString('utf8'));
        objects.verify(await objects.read());
        for (const schema of schemas)
          await connection.query(
            `INSERT INTO ${schema}.schema_migration(version,checksum) VALUES(${step.manifest.version},UNHEX(?))`,
            [step.manifest.sha256],
          );
      } catch (error) {
        if (
          [
            'MIGRATION_INDEX_MISMATCH',
            'MIGRATION_TABLE_MISMATCH',
            'MIGRATION_CHECK_MISMATCH',
          ].includes(error.message)
        )
          throw error;
        throw Error('MIGRATION_APPLY_FAILED_REQUIRES_INSPECTION');
      }
    }
    return {
      state:
        current === 'INSTALLED'
          ? 'ALREADY_INSTALLED'
          : await installationState(connection, manifest, manifests),
      ...receipt,
    };
  } finally {
    try {
      await connection.query('SELECT RELEASE_LOCK(?)', ['collaboration_report_migration_v1']);
    } catch {
      /* Connection loss releases the advisory lock. */
    }
  }
}
