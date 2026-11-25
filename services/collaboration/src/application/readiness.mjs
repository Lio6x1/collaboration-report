export class Readiness {
  constructor(repository, expectedSQLSHA, expectedAdditionalMigrations = {}) {
    this.repository = repository;
    this.expectedSQLSHA = expectedSQLSHA;
    this.expectedAdditionalMigrations = { ...expectedAdditionalMigrations };
  }
  async check(store) {
    // 版本是INT，BIGINT数量允许字符串；摘要从仓储HEX取得，避免二进制编码歧义。
    const versions = await this.repository.versions();
    const expected = { 1: this.expectedSQLSHA, ...this.expectedAdditionalMigrations };
    // 附加版本必须构成完整连续集合；按原版本身份比较，不信返回行序或数据库自报期望。
    const keys = Object.keys(expected);
    if (
      Object.hasOwn(this.expectedAdditionalMigrations, '1') ||
      versions.length !== keys.length ||
      Object.values(expected).some(
        (checksum) => typeof checksum !== 'string' || !/^[a-f0-9]{64}$/.test(checksum),
      ) ||
      keys.some((_, index) => !Object.hasOwn(expected, String(index + 1))) ||
      versions.some(
        (row) => !Number.isInteger(row.version) || expected[String(row.version)] !== row.checksum,
      ) ||
      new Set(versions.map((row) => row.version)).size !== versions.length
    )
      throw Error('SCHEMA_VERSION_MISMATCH');
    if (Number(await this.repository.pendingTasks()) !== 0) throw Error('RECOVERY_REQUIRED');
    if (Number(await this.repository.pendingGates()) !== 0) throw Error('GATE_RECOVERY_REQUIRED');
    if (Number(await this.repository.unboundDraftMainGates()) !== 0)
      throw Error('DRAFT_MAIN_GATE_BINDING_UNKNOWN');
    if (Number(await this.repository.rooms()) !== 0 && !store.roomsRecovered)
      throw Error('ROOM_RECOVERY_REQUIRED');
  }
}
