import { query } from './sql.mjs';

export const draftInstallation = async (connection, operation) => {
  const [[row]] = await query(
    connection,
    "SELECT payload,state,operation_id FROM durable_task WHERE kind='DRAFT_TRANSITION_INSTALLED' AND business_key=?",
    [operation],
  );
  return row;
};

// 单文档回执同时是永久的安装墓碑。已清理房间仍保留身份/切点审计，不再恢复正文。
export const cleanupReceipt = async (connection, document, operation) => {
  const [[row]] = await query(
    connection,
    operation === undefined
      ? "SELECT operation_id,payload,state FROM durable_task WHERE (kind='ACCOUNT_DOCUMENT_CLEANUP' AND business_key=?) OR (kind='PURGE_DOCUMENT_CLEANUP' AND business_key LIKE ?) ORDER BY id LIMIT 1"
      : "SELECT operation_id,payload,state FROM durable_task WHERE (kind='ACCOUNT_DOCUMENT_CLEANUP' AND business_key=? AND operation_id=?) OR (kind='PURGE_DOCUMENT_CLEANUP' AND business_key=?) ORDER BY id LIMIT 1",
    operation === undefined
      ? [document, document + ':%']
      : [document, operation, document + ':' + operation],
  );
  return row;
};

export const lockCleanupSource = (connection, scope) =>
  query(
    connection,
    'SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE',
    [scope],
  );

export const lockRooms = (connection, document) =>
  query(connection, 'SELECT * FROM room WHERE document_id=? ORDER BY id FOR UPDATE', [document]);

export const pendingDeliveries = (connection, room) =>
  query(
    connection,
    "SELECT 1 FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state<>'SETTLED' LIMIT 1",
    [room],
  );

export const roomCaptures = (connection, room) =>
  query(
    connection,
    'SELECT operation_id,state FROM capture_operation WHERE room_id=? ORDER BY operation_id FOR UPDATE',
    [room],
  );

export const clearRoomBody = async (connection, room) => {
  await query(connection, "UPDATE room SET state='RETIRED',checkpoint_id=NULL WHERE id=?", [room]);
  await query(connection, 'DELETE FROM update_log WHERE room_id=?', [room]);
  await query(connection, 'DELETE FROM checkpoint WHERE room_id=?', [room]);
  await query(
    connection,
    "UPDATE capture_operation SET state='DELETED',canonical_body=NULL,candidate_json=NULL,resource_refs=JSON_ARRAY() WHERE room_id=?",
    [room],
  );
  // 这两个artifact的键由room与epoch组成，包括已经退休的旧代次。
  for (const kind of ['ROOM_LINEAGE_CONTEXT', 'ROOM_MAIN_CONTEXT']) {
    await query(connection, 'DELETE FROM durable_task WHERE kind=? AND business_key LIKE ?', [
      kind,
      room + ':%',
    ]);
  }
};

// 各固定artifact只匹配已知的权威文档路径；MERGE_RESULT自身没有context，以其原attempt绑定关联。
// 不扫描/删除未知kind，也不把另一个文档的相同unit或用户当作清理对象。
// operation_id必须随制品读出：已准备转换只有匹配原操作的安装证明才能清理，
// 否则完整的拉取主线/安装流程会被误判为尚未安装而永远不能回收。
export const documentArtifacts = (connection, document) =>
  query(
    connection,
    `
  SELECT id,kind,business_key,operation_id,payload,state FROM durable_task WHERE
  cleanup_document=? AND kind IN ('MAIN_VIEW_FIXED','MERGE_ATTEMPT_CONTEXT','DRAFT_PREVIEW_FIXED','DRAFT_CONFIRMATION_FIXED',
    'DRAFT_TRANSITION_PREPARED','STRUCTURE_PREVIEW_FIXED','STRUCTURE_PREPARATION',
    'DRAFT_MAIN_GATE','STRUCTURE_BRANCH_GATE','RESOURCE_CONFIRM') ORDER BY id`,
    [document],
  );

export const removeArtifact = (connection, id) =>
  query(connection, 'DELETE FROM durable_task WHERE id=?', [id]);
export const removeMergeResult = (connection, key) =>
  query(
    connection,
    "DELETE FROM durable_task WHERE kind='MERGE_CANDIDATE_RESULT' AND business_key=?",
    [key],
  );

export const recordCleanup = (connection, context) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [
      context.cleanup_kind === 'PURGE' ? 'PURGE_DOCUMENT_CLEANUP' : 'ACCOUNT_DOCUMENT_CLEANUP',
      context.cleanup_kind === 'PURGE'
        ? context.document_id + ':' + context.operation_id
        : context.document_id,
      context.operation_id,
      JSON.stringify(context),
    ],
  );

export const clearBarrierBody = (connection, document) =>
  query(
    connection,
    "UPDATE main_barrier SET cut_vector=JSON_OBJECT() WHERE document_id=? AND state IN ('INSTALLED','ABORTED')",
    [document],
  );
