import { query } from './sql.mjs';

// 来源 structure_preparations.mjs：原业务声明与函数体保留。

export const branchGate = (connection, room, epoch) =>
  query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='STRUCTURE_BRANCH_GATE' AND business_key=? FOR UPDATE",
    [room + ':' + epoch],
  );
export const prepareBranchGate = (connection, branch, binding) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('STRUCTURE_BRANCH_GATE',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE operation_id=VALUES(operation_id),payload=VALUES(payload)",
    [
      branch.room_id + ':' + branch.workspace_epoch,
      binding.context.operation_id,
      JSON.stringify({
        document_id: binding.context.target.document_id,
        operation_id: binding.context.operation_id,
        generation: binding.barrier_generation,
        state: 'PREPARED',
      }),
    ],
  );
export const finishBranchGate = (connection, branch, binding, state) =>
  query(
    connection,
    "UPDATE durable_task SET payload=JSON_SET(payload,'$.state',?) WHERE kind='STRUCTURE_BRANCH_GATE' AND business_key=? AND operation_id=? AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.generation'))=? AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.state'))='PREPARED'",
    [
      state,
      branch.room_id + ':' + branch.workspace_epoch,
      binding.context.operation_id,
      binding.barrier_generation,
    ],
  );

// 来源 structure_previews.mjs：原业务声明与函数体保留。

export const lockBranchRooms = (connection, document) =>
  query(
    connection,
    "SELECT * FROM room WHERE document_id=? AND workspace_kind='BRANCH' AND state<>'RETIRED' ORDER BY id FOR UPDATE",
    [document],
  );
