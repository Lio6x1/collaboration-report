// 准备阶段及旧空启动协议保持一秒查询期限；完整排空后的终态写入保留原无单条期限调用。
// 所有方法使用调用者提供的同一连接，仓储不另开事务，也不凭TTL生成退出证明。
const query = (connection, sql, args = []) => connection.query({ sql, timeout: 1000 }, args);
export async function lockedClosure(connection, revocation) {
  const [rows] = await query(
    connection,
    'SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE',
    [revocation],
  );
  return rows[0];
}
export async function insertClosure(connection, revocation, scope, current) {
  await query(
    connection,
    "INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,'FENCING','[]')",
    [revocation, scope.scope_key, scope.epoch, JSON.stringify(current)],
  );
}
export async function blockSource(connection, revocation, scope) {
  await query(
    connection,
    "INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state=IF(VALUES(max_epoch)>=max_epoch,'BLOCKED',state),last_revocation_id=IF(VALUES(max_epoch)>=max_epoch,VALUES(last_revocation_id),last_revocation_id),revision=revision+1",
    [scope.scope_key, scope.epoch, revocation],
  );
}
export async function closeGrants(connection, scope) {
  await query(
    connection,
    "UPDATE collaboration_grant g JOIN grant_source s ON s.grant_id=g.grant_id SET g.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=? AND g.state<>'CLOSED'",
    [scope.scope_key, scope.epoch],
  );
}
export async function closeReceipts(connection, scope) {
  // 旧回执的关闭仍由同事务的真实grant/source决定；不得扫描其他范围的已关闭历史。
  await query(
    connection,
    "UPDATE flush_receipt f JOIN collaboration_grant g ON g.grant_id=f.grant_id JOIN grant_source s ON s.grant_id=g.grant_id SET f.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=? AND g.state='CLOSED' AND f.state<>'CLOSED'",
    [scope.scope_key, scope.epoch],
  );
}
export async function completeClosure(connection, revocation, scope, digest, bootstrap = false) {
  const execute = bootstrap
    ? (sql, args) => query(connection, sql, args)
    : (sql, args) => connection.query(sql, args);
  await execute(
    "INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,'node-runtime',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE result_digest=VALUES(result_digest)",
    [revocation, scope.scope_key, scope.epoch, digest],
  );
  await execute("UPDATE authorization_change SET state='SUCCEEDED' WHERE revocation_id=?", [
    revocation,
  ]);
}
export async function* bootstrapBusinessCounts(connection) {
  // 表名是固定私有库清单，逐表按原顺序读取并立即交还，不能绕过非空拒绝边界。
  for (const table of ['room', 'collaboration_grant', 'delivery_session']) {
    const [[row]] = await query(connection, 'SELECT COUNT(*) AS n FROM ' + table);
    yield row.n;
  }
}
export const startBootstrap = (connection) => query(connection, 'START TRANSACTION');
export const commitBootstrap = (connection) => query(connection, 'COMMIT');
export const rollbackBootstrap = (connection) => query(connection, 'ROLLBACK');
export async function pendingClosures(pool) {
  const [rows] = await pool.query(
    "SELECT revocation_id,scope_key,epoch,target_permission FROM authorization_change WHERE state='FENCING'",
  );
  return rows;
}
