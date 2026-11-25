# services/collaboration/src/repository/readiness.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/317-readiness.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`Readiness`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
// 就绪查询保留原一秒期限及查询顺序；未恢复任务和门闩不能被启动成功日志替代。
export class Readiness {
  constructor(pool, recoveryIndexes = false) {
    this.pool = pool;
    this.recoveryIndexes = recoveryIndexes;
  }
  async versions() {
    const [rows] = await this.pool.query({
      sql: 'SELECT version,LOWER(HEX(checksum)) AS checksum FROM schema_migration',
      timeout: 1000,
    });
    return rows;
  }
  async pendingTasks() {
    const [[row]] = await this.pool.query({
      sql: `SELECT COUNT(*) AS pending FROM durable_task ${this.recoveryIndexes ? 'FORCE INDEX (task_pending)' : ''} WHERE state NOT IN ('SUCCEEDED','REJECTED')`,
      timeout: 1000,
    });
    return row.pending;
  }
  async pendingGates() {
    const [[row]] = await this.pool.query({
      sql: `SELECT COUNT(*) AS pending FROM access_gate g ${this.recoveryIndexes ? 'FORCE INDEX (gate_recovery)' : ''} LEFT JOIN authorization_change c ${this.recoveryIndexes ? 'FORCE INDEX (auth_recovery)' : ''} ON c.revocation_id=g.last_revocation_id AND c.scope_key=g.scope_key AND c.epoch>=g.max_epoch WHERE g.state='BLOCKED' AND (c.state IS NULL OR c.state<>'SUCCEEDED')`,
      timeout: 1000,
    });
    return row.pending;
  }
  async rooms() {
    const [[row]] = await this.pool.query({
      sql: 'SELECT COUNT(*) AS pending FROM room',
      timeout: 1000,
    });
    return row.pending;
  }

  async unboundDraftMainGates() {
    // PREPARED本身可以由Go原责任方恢复；缺原固定确认/准备/终态安装关联才是不可信的启动状态。
    const [[row]] = await this.pool.query({
      sql: `SELECT COUNT(*) AS pending FROM durable_task g
        LEFT JOIN durable_task p ON p.kind='DRAFT_TRANSITION_PREPARED' AND p.business_key=g.operation_id AND p.operation_id=g.operation_id AND p.state='SUCCEEDED'
        LEFT JOIN durable_task f ON f.kind=CASE JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.binding.context.target.purpose')) WHEN 'REVERSE' THEN 'DRAFT_PREVIEW_FIXED' ELSE 'DRAFT_CONFIRMATION_FIXED' END AND f.business_key=g.operation_id AND f.operation_id=g.operation_id AND f.state='SUCCEEDED'
        LEFT JOIN durable_task i ON i.kind='DRAFT_TRANSITION_INSTALLED' AND i.business_key=g.operation_id AND i.operation_id=g.operation_id AND i.state='SUCCEEDED'
        WHERE g.kind='DRAFT_MAIN_GATE' AND (
          p.id IS NULL OR f.id IS NULL OR
          g.state<>'SUCCEEDED' OR
          COALESCE(JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.operation_id')),'')<>g.operation_id OR
          COALESCE(JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.document_id')),'')<>g.business_key OR
          COALESCE(JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.binding.context.target.document_id')),'')<>g.business_key OR
          COALESCE(JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.binding.context.target.purpose')),'') NOT IN ('DISCARD_DRAFT','REVERSE') OR
          COALESCE(JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.state')),'') NOT IN ('PREPARED','INSTALLED','ABORTED') OR
          COALESCE(JSON_EXTRACT(g.payload,'$.binding'),JSON_OBJECT())<>COALESCE(JSON_EXTRACT(p.payload,'$.binding'),JSON_OBJECT()) OR
          COALESCE(JSON_EXTRACT(p.payload,'$.binding'),JSON_OBJECT())<>COALESCE(JSON_EXTRACT(f.payload,'$.binding'),JSON_OBJECT()) OR
          COALESCE(JSON_EXTRACT(p.payload,'$.draft_cut'),JSON_OBJECT())<>COALESCE(JSON_EXTRACT(f.payload,'$.draft_cut'),JSON_OBJECT()) OR
          COALESCE(JSON_EXTRACT(p.payload,'$.main_cut'),JSON_ARRAY())<>COALESCE(JSON_EXTRACT(f.payload,'$.main_cut'),JSON_ARRAY()) OR
          (JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.state')) IN ('INSTALLED','ABORTED') AND i.id IS NULL)
        )`,
      timeout: 1000,
    });
    return row.pending;
  }
}
````

UTF-8/LF全文SHA-256：`903532642a5603ad1d68d5fe6d3c28dbc1ab8c04d26ad671f8be10ae4e18073f`。[参考实现](../../../../services/collaboration/src/repository/readiness.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
