# services/collaboration/src/repository/room_persistence.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/320-room-persistence.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`findUpdate`、`insertUpdate`、`advanceDurableSeq`、`findDependencyHash`、`lockFlushBinding`、`insertFlushBinding`、`findFlushReceipt`、`insertFlushReceipt`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { query } from './sql.mjs';

// 来源 update_log.mjs：原业务声明与函数体保留。

// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function findUpdate(connection, room, epoch, client, update) {
  return query(
    connection,
    'SELECT seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?',
    [room, epoch, client, update],
  );
}

export function insertUpdate(connection, room, epoch, seq, client, update, payload, hash) {
  return query(
    connection,
    'INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)',
    [room, epoch, seq, client, update, payload, hash],
  );
}

export function advanceDurableSeq(connection, seq, room, epoch) {
  return query(connection, 'UPDATE room SET last_durable_seq=? WHERE id=? AND epoch=?', [
    seq,
    room,
    epoch,
  ]);
}

export function findDependencyHash(connection, room, epoch, client, update) {
  return query(
    connection,
    'SELECT payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?',
    [room, epoch, client, update],
  );
}

// 来源 flush_receipts.mjs：原业务声明与函数体保留。

// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function lockFlushBinding(connection, operation) {
  return query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='FLUSH_BINDING' AND business_key=? FOR UPDATE",
    [operation],
  );
}

export function insertFlushBinding(connection, operation, payload) {
  return query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('FLUSH_BINDING',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [operation, payload],
  );
}

export function findFlushReceipt(connection, operation) {
  return query(connection, 'SELECT * FROM flush_receipt WHERE receipt_id=?', [operation]);
}

export function insertFlushReceipt(connection, receipt, grant, actor, room, epoch, cut, digest) {
  return query(
    connection,
    "INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,'FIXED')",
    [receipt, grant, actor, room, epoch, cut, digest],
  );
}
````

UTF-8/LF全文SHA-256：`69b399cea92b113e4fa022a55d81db88fedd545fbad173517c6a684ccf0be4a1`。[参考实现](../../../../services/collaboration/src/repository/room_persistence.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
