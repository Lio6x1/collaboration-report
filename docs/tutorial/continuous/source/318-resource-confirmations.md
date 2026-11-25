# services/collaboration/src/repository/resource_confirmations.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/318-resource-confirmations.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`queueResourceConfirmation`、`markResourceConfirmed`、`retryResourceConfirmation`、`pendingResourceConfirmations`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { query } from './sql.mjs';

// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function queueResourceConfirmation(connection, operation, payload) {
  return query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('RESOURCE_CONFIRM',?,?,'QUEUED',UTC_TIMESTAMP(6))",
    [operation, payload],
  );
}

export function markResourceConfirmed(connection, operation) {
  return query(
    connection,
    "UPDATE durable_task SET state='SUCCEEDED',last_error=NULL WHERE kind='RESOURCE_CONFIRM' AND business_key=?",
    [operation],
  );
}

export function retryResourceConfirmation(connection, state, error, operation) {
  return query(
    connection,
    "UPDATE durable_task SET state=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),attempts=attempts+1,last_error=? WHERE kind='RESOURCE_CONFIRM' AND business_key=?",
    [state, error, operation],
  );
}

export function pendingResourceConfirmations(connection, id) {
  return query(
    connection,
    "SELECT business_key,payload FROM durable_task WHERE kind='RESOURCE_CONFIRM' AND state<>'SUCCEEDED' AND " +
      (id ? 'business_key=?' : 'retry_at<=UTC_TIMESTAMP(6) ORDER BY retry_at LIMIT 32'),
    id ? [id] : [],
  );
}
````

UTF-8/LF全文SHA-256：`bbcbf28467d1ba36a9d3f0b88a1539ef7183f428888bf4e9448e1e6a95921550`。[参考实现](../../../../services/collaboration/src/repository/resource_confirmations.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
