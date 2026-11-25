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
