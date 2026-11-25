# services/collaboration/src/repository/persistence_outcome.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/315-persistence-outcome.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`committedUpdate`、`confirmationsForReceipt`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
// 私有持久证明仅从真实Node日志及唯一固定确认任务取得；保留原pool.query调用和SQL。
export async function committedUpdate(pool, input) {
  const [rows] = await pool.query(
    'SELECT CAST(seq AS CHAR) AS seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?',
    [input.room_id, input.workspace_epoch, input.client_session_id, input.client_update_id],
  );
  return rows[0];
}
export async function confirmationsForReceipt(pool, receipt) {
  const [rows] = await pool.query(
    "SELECT business_key,payload FROM durable_task WHERE kind='RESOURCE_CONFIRM' AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.node_receipt_ref'))=?",
    [receipt],
  );
  return rows;
}
````

UTF-8/LF全文SHA-256：`2eee9eaf581d138a62c5c26c44c6afcac758ba4da56ba0157fc7d96dfdfd192d`。[参考实现](../../../../services/collaboration/src/repository/persistence_outcome.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
