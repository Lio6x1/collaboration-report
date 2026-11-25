import { query } from './sql.mjs';

// 来源 captures.mjs：原业务声明与函数体保留。

export async function lockOperation(connection, operation) {
  const key = 'capture:' + operation;
  await query(
    connection,
    "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)",
    [key],
  );
  await query(connection, 'SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE', [key]);
  return query(connection, 'SELECT * FROM capture_operation WHERE operation_id=? FOR UPDATE', [
    operation,
  ]);
}
export const findCapture = (connection, operation) =>
  query(connection, 'SELECT * FROM capture_operation WHERE operation_id=?', [operation]);
export const lockRoomLineage = (connection, room, epoch) =>
  query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='ROOM_LINEAGE_CONTEXT' AND business_key=? FOR UPDATE",
    [room + ':' + epoch],
  );
export const insertRoomLineage = (connection, room, epoch, payload) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('ROOM_LINEAGE_CONTEXT',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [room + ':' + epoch, JSON.stringify(payload)],
  );
export const insertCut = (connection, binding, room, cut, lineage) =>
  query(
    connection,
    "INSERT INTO capture_operation(operation_id,request_fingerprint,actor_id,room_id,epoch,cut_ref,lineage_ref,state,generation,candidate_json,resource_refs) VALUES(?,?,?,?,?,?,?,'CUT_FIXED',?,?,'[]')",
    [
      binding.context.operation_id,
      Buffer.from(binding.context.request_fingerprint, 'base64'),
      binding.context.actor_id,
      room.id,
      room.epoch,
      JSON.stringify(cut),
      binding.lineage_ref,
      binding.guard_generation,
      JSON.stringify({ binding, lineage }),
    ],
  );
export const saveCandidate = (connection, operation, candidate, bytes, digest, resources) =>
  query(
    connection,
    "UPDATE capture_operation SET state='CAPTURED',candidate_json=?,canonical_body=?,body_hash=?,resource_refs=? WHERE operation_id=? AND state='CUT_FIXED'",
    [JSON.stringify(candidate), bytes, digest, JSON.stringify(resources), operation],
  );
export const bindResult = (connection, operation, result) =>
  query(
    connection,
    "UPDATE capture_operation SET state='BOUND',go_result_ref=? WHERE operation_id=? AND state IN ('CAPTURED','BOUND')",
    [result, operation],
  );

// 来源 merge_candidates.mjs：原业务声明与函数体保留。

export async function lockAttempt(connection, operation, attempt) {
  const key = 'merge:' + operation + ':' + attempt;
  await query(
    connection,
    "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)",
    [key],
  );
  await query(connection, 'SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE', [key]);
  const business = operation + ':' + attempt;
  const [[context]] = await query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='MERGE_ATTEMPT_CONTEXT' AND business_key=?",
    [business],
  );
  const [[result]] = await query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='MERGE_CANDIDATE_RESULT' AND business_key=?",
    [business],
  );
  const cleaned = await findCleanup(connection, 'merge:' + business);
  return { context, result, business, cleaned };
}
export const saveContext = (connection, business, binding) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('MERGE_ATTEMPT_CONTEXT',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [business, JSON.stringify(binding)],
  );
export const saveResult = (connection, business, result) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('MERGE_CANDIDATE_RESULT',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [business, JSON.stringify(result)],
  );

// 来源 candidate_cleanup.mjs：原业务声明与函数体保留。

export async function findCleanup(connection, reference) {
  const [[row]] = await query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='CANDIDATE_CLEANED' AND business_key=? AND state='SUCCEEDED'",
    [reference],
  );
  return row;
}

export const saveCleanup = (connection, binding, result) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('CANDIDATE_CLEANED',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [binding.candidate_ref, binding.operation_id, JSON.stringify({ binding, result })],
  );

export const cleanCaptureBody = (connection, operation, candidate) =>
  query(
    connection,
    "UPDATE capture_operation SET canonical_body=NULL,candidate_json=?,resource_refs=JSON_ARRAY() WHERE operation_id=? AND state='BOUND'",
    [JSON.stringify(candidate), operation],
  );

export const cleanMergeBody = (connection, business) =>
  query(
    connection,
    "DELETE FROM durable_task WHERE kind='MERGE_CANDIDATE_RESULT' AND business_key=?",
    [business],
  );
