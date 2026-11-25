# services/collaboration/src/repository/fences.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/312-fences.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`lockedClosure`、`insertClosure`、`blockSource`、`closeGrants`、`closeReceipts`、`completeClosure`、`pendingClosures`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`43818936462f922bd974ea33a7a8e91a77b4a351668315efbf416d6656c9f5a7`。[参考实现](../../../../services/collaboration/src/repository/fences.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
