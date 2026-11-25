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
