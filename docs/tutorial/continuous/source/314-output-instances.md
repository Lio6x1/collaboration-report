# services/collaboration/src/repository/output_instances.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/314-output-instances.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`listOutputInstances`、`settleExitedOutputs`、`recordOutputExit`、`insertOutputInstance`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { query } from './sql.mjs';

// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function listOutputInstances(connection) {
  return query(
    connection,
    "SELECT business_key,payload FROM durable_task WHERE kind='NODE_INSTANCE'",
  );
}

export function settleExitedOutputs(connection, instance) {
  return query(
    connection,
    "UPDATE delivery_session SET state='SETTLED',stop_reason='EXIT_CONFIRMED' WHERE instance_id=? AND state<>'SETTLED'",
    [instance],
  );
}

export function recordOutputExit(connection, payload, instance) {
  return query(
    connection,
    "UPDATE durable_task SET payload=? WHERE kind='NODE_INSTANCE' AND business_key=?",
    [payload, instance],
  );
}

export function insertOutputInstance(connection, instance, payload) {
  return query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('NODE_INSTANCE',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [instance, payload],
  );
}
````

UTF-8/LF全文SHA-256：`6e874881d337ab2a6c29330bc1d1671826420b355fd7c9903bcb7fb20c9d3307`。[参考实现](../../../../services/collaboration/src/repository/output_instances.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
