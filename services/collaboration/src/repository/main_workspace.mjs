import { query } from './sql.mjs';

// 来源 main_barriers.mjs：原业务声明与函数体保留。

export async function lockDocument(connection, document) {
  const key = 'main:' + document;
  await query(
    connection,
    "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)",
    [key],
  );
  await query(connection, 'SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE', [key]);
}
export const lockBarrier = (connection, document) =>
  query(connection, 'SELECT * FROM main_barrier WHERE document_id=? FOR UPDATE', [document]);
export const lockMainRooms = (connection, document) =>
  query(
    connection,
    "SELECT * FROM room WHERE document_id=? AND workspace_kind='MAIN' AND state<>'RETIRED' ORDER BY id FOR UPDATE",
    [document],
  );
// 阅读页只检查已持久化的MAIN尾部；不恢复Yjs、不创建快照，也不参与写入锁。
export const mainReadingRevision = (connection, document) =>
  query(
    connection,
    "SELECT id,epoch,last_durable_seq FROM room WHERE document_id=? AND workspace_kind='MAIN' AND state<>'RETIRED' ORDER BY id",
    [document],
  );
export const mainContext = (connection, room, epoch) =>
  query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='ROOM_MAIN_CONTEXT' AND business_key=?",
    [room + ':' + epoch],
  );
export const insertMainContext = (connection, room, epoch, context) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('ROOM_MAIN_CONTEXT',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [room + ':' + epoch, JSON.stringify(context)],
  );
export const prepareBarrier = (connection, document, binding, cut) =>
  query(
    connection,
    "INSERT INTO main_barrier(document_id,operation_id,generation,state,cut_vector,structure_ref,style_ref) VALUES(?,?,?,'PREPARED',?,?,?) ON DUPLICATE KEY UPDATE operation_id=VALUES(operation_id),generation=VALUES(generation),state='PREPARED',cut_vector=VALUES(cut_vector),structure_ref=VALUES(structure_ref),style_ref=VALUES(style_ref),installed_manifest_id=NULL,go_result_digest=NULL",
    [
      document,
      binding.context.operation_id,
      binding.barrier_generation,
      JSON.stringify({ binding, prepared_cut: cut }),
      cut.structure_id,
      cut.style_id,
    ],
  );

// 来源 main_captures.mjs：原业务声明与函数体保留。

export const lockCapture = (connection, key) =>
  query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND business_key=? FOR UPDATE",
    [key],
  );
export const insertCapture = (connection, key, operation, payload) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('MAIN_VIEW_FIXED',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [key, operation, JSON.stringify(payload)],
  );

// 原号前缀使用(kind,business_key)现有索引；只取得cut/context，避免把整篇固定正文再传一次。
export const preparationFacts = (connection, operation) =>
  query(
    connection,
    `SELECT JSON_EXTRACT(payload,'$.binding.context') context,
    JSON_EXTRACT(payload,'$.cut') cut,
    JSON_UNQUOTE(JSON_EXTRACT(payload,'$.binding.expected_manifest_id')) manifest,
    JSON_UNQUOTE(JSON_EXTRACT(payload,'$.binding.expected_control_revision')) control
    FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND business_key>=? AND business_key<?
    AND operation_id=? AND state='SUCCEEDED'`,
    [operation + ':', operation + ';', operation],
  );

// 来源 main_installations.mjs：原业务声明与函数体保留。

export const lockReceipt = (connection, operation) =>
  query(connection, 'SELECT * FROM install_receipt WHERE operation_id=? FOR UPDATE', [operation]);
export const retireMainRooms = (connection, document) =>
  query(
    connection,
    "UPDATE room SET state='RETIRED' WHERE document_id=? AND workspace_kind='MAIN' AND state<>'RETIRED'",
    [document],
  );
export const retiredMainRooms = (connection, document) =>
  query(
    connection,
    "SELECT id FROM room WHERE document_id=? AND workspace_kind='MAIN' AND state='RETIRED' ORDER BY id",
    [document],
  );
export const closeRetiredGrants = (connection, document) =>
  query(
    connection,
    "UPDATE collaboration_grant g JOIN room r ON r.id=g.room_id SET g.state='CLOSED' WHERE r.document_id=? AND r.workspace_kind='MAIN' AND r.state='RETIRED'",
    [document],
  );
export const stopRetiredDeliveries = (connection, document) =>
  query(
    connection,
    "UPDATE delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id JOIN room r ON r.id=g.room_id SET d.state='STOPPING',d.stop_reason='MAIN_EPOCH_CHANGED' WHERE r.document_id=? AND r.workspace_kind='MAIN' AND r.state='RETIRED' AND d.state<>'SETTLED'",
    [document],
  );
export const updateLineage = (connection, room, epoch, context) =>
  query(
    connection,
    "UPDATE durable_task SET payload=? WHERE kind='ROOM_LINEAGE_CONTEXT' AND business_key=?",
    [JSON.stringify(context), room + ':' + epoch],
  );
export const insertReceipt = (connection, input, cuts) =>
  query(
    connection,
    "INSERT INTO install_receipt(operation_id,generation,manifest_id,manifest_digest,installed_epochs,state) VALUES(?,?,?,?,?,'INSTALLED')",
    [
      input.context.operation_id,
      input.barrier_generation,
      input.manifest_id,
      Buffer.from(input.manifest_digest),
      JSON.stringify(cuts),
    ],
  );
export const installedBarrier = (connection, document, operation, digest, manifest) =>
  query(
    connection,
    "UPDATE main_barrier SET state='INSTALLED',installed_manifest_id=?,go_result_digest=? WHERE document_id=? AND operation_id=?",
    [manifest, Buffer.from(digest), document, operation],
  );
export const abortBarrier = (connection, document, operation, generation) =>
  query(
    connection,
    "UPDATE main_barrier SET state='ABORTED' WHERE document_id=? AND operation_id=? AND generation=? AND state<>'INSTALLED'",
    [document, operation, generation],
  );
