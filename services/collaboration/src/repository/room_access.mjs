import { query } from './sql.mjs';

// 来源 room_admissions.mjs：原业务声明与函数体保留。

// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function lockRoom(connection, room) {
  return query(connection, 'SELECT * FROM room WHERE id=? FOR UPDATE', [room]);
}

export function insertRoom(connection, room, document, unit, kind, workspace, epoch) {
  return query(
    connection,
    "INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,'OPEN')",
    [room, document, unit, kind, workspace, epoch],
  );
}

export function lockGrant(connection, grant) {
  return query(connection, 'SELECT * FROM collaboration_grant WHERE grant_id=? FOR UPDATE', [
    grant,
  ]);
}

export function insertGrant(
  connection,
  grant,
  admission,
  actor,
  session,
  client,
  room,
  epoch,
  capabilities,
  expires,
) {
  return query(
    connection,
    "INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expires_at) VALUES(?,?,?,?,?,?,?,?,'OPEN',?)",
    [grant, admission, actor, session, client, room, epoch, capabilities, expires],
  );
}

export function insertGrantSource(connection, grant, scope, epoch) {
  return query(connection, 'INSERT INTO grant_source(grant_id,scope_key,epoch) VALUES(?,?,?)', [
    grant,
    scope,
    epoch,
  ]);
}

export function lockLiveGrant(connection, grant) {
  return query(
    connection,
    'SELECT *,expires_at>UTC_TIMESTAMP(6) AS alive FROM collaboration_grant WHERE grant_id=? FOR UPDATE',
    [grant],
  );
}

export function lockWorkspaceRoom(connection, room, epoch) {
  return query(connection, 'SELECT * FROM room WHERE id=? AND epoch=? FOR UPDATE', [room, epoch]);
}

// 来源 delivery_sessions.mjs：原业务声明与函数体保留。

// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function insertDelivery(connection, delivery, grant, instance, handler, expires) {
  return query(
    connection,
    "INSERT INTO delivery_session(delivery_id,grant_id,instance_id,handler_id,state,expires_at) VALUES(?,?,?,?,'OPEN',?)",
    [delivery, grant, instance, handler, expires],
  );
}

export function settleDelivery(connection, reason, delivery, instance) {
  return query(
    connection,
    "UPDATE delivery_session SET state='SETTLED',stop_reason=? WHERE delivery_id=? AND instance_id=?",
    [reason, delivery, instance],
  );
}

export function pendingScopeDeliveries(connection, scope, epoch) {
  return query(
    connection,
    "SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN grant_source s ON s.grant_id=d.grant_id WHERE s.scope_key=? AND s.epoch<=? AND d.state<>'SETTLED'",
    [scope, epoch],
  );
}

export function settleClosedDelivery(connection, delivery) {
  return query(
    connection,
    "UPDATE delivery_session SET state='SETTLED',stop_reason='SOURCE_CLOSED' WHERE delivery_id=?",
    [delivery],
  );
}

export function pendingRoomDeliveries(connection, room) {
  return query(
    connection,
    "SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state<>'SETTLED'",
    [room],
  );
}
