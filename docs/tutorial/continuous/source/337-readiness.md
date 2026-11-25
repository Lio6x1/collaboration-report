# services/collaboration/src/application/readiness.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/337-readiness.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`Readiness`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`6a338965e823f0d730f755bfd98c0823a95fe39045e12d0b4377a30c2126ad5d`。[参考实现](../../../../services/collaboration/src/application/readiness.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
