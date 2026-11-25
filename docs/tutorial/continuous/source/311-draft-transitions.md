# services/collaboration/src/repository/draft_transitions.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/311-draft-transitions.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：完整定义见下方源码。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { query } from './sql.mjs';

export const setMainGate = (connection, document, operation, state, binding) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('DRAFT_MAIN_GATE',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE operation_id=VALUES(operation_id),payload=VALUES(payload)",
    [
      document,
      operation,
      JSON.stringify({ document_id: document, operation_id: operation, state, binding }),
    ],
  );

export const artifact = (connection, kind, key) =>
  query(connection, 'SELECT payload FROM durable_task WHERE kind=? AND business_key=? FOR UPDATE', [
    kind,
    key,
  ]);
export const insertArtifact = (connection, kind, operation, payload) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [kind, operation, operation, JSON.stringify(payload)],
  );
export const unitMainRoom = (connection, document, unit) =>
  query(
    connection,
    "SELECT * FROM room WHERE document_id=? AND unit_id=? AND workspace_kind='MAIN' AND state<>'RETIRED' ORDER BY id FOR UPDATE",
    [document, unit],
  );
export const blockRoom = (connection, room, epoch) =>
  query(
    connection,
    "UPDATE room SET state='BLOCKED',barrier_generation=barrier_generation+1 WHERE id=? AND epoch=? AND state='OPEN'",
    [room, epoch],
  );
export const openRoom = (connection, room, epoch) =>
  query(connection, "UPDATE room SET state='OPEN' WHERE id=? AND epoch=? AND state='BLOCKED'", [
    room,
    epoch,
  ]);
export const retireRoom = (connection, room, epoch) =>
  query(connection, "UPDATE room SET state='RETIRED' WHERE id=? AND epoch=? AND state='BLOCKED'", [
    room,
    epoch,
  ]);
export const closeGrants = (connection, room) =>
  query(
    connection,
    "UPDATE collaboration_grant SET state='CLOSED' WHERE room_id=? AND state<>'CLOSED'",
    [room],
  );
export const stopDeliveries = (connection, room) =>
  query(
    connection,
    "UPDATE delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id SET d.state='STOPPING',d.stop_reason='BRANCH_EPOCH_CHANGED' WHERE g.room_id=? AND d.state<>'SETTLED'",
    [room],
  );
````

UTF-8/LF全文SHA-256：`af983355311767a1e4f0ca1894fd11857b984fdec2ff3fa9038be1769f8ac86d`。[参考实现](../../../../services/collaboration/src/repository/draft_transitions.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
