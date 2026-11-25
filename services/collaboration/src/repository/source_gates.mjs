import { query } from './sql.mjs';

// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function ensureSourceGate(connection, scope, epoch) {
  return query(
    connection,
    "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)",
    [scope, epoch],
  );
}

export function lockSourceGate(connection, scope) {
  return query(connection, 'SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE', [
    scope,
  ]);
}

export function reopenSourceGate(connection, epoch, scope) {
  return query(
    connection,
    "UPDATE access_gate SET max_epoch=?,state='OPEN',revision=revision+1 WHERE scope_key=?",
    [epoch, scope],
  );
}

// 调用方先按ASCII顺序去重；多行UPSERT仍逐键取得排他锁，值不改变。
// 随后同事务当前读，减少每个广播读者逐来源两次SQL往返的放大。
export function ensureSourceGates(connection, sources) {
  return query(
    connection,
    'INSERT INTO access_gate(scope_key,max_epoch,state) VALUES ' +
      sources.map(() => "(?,?,'OPEN')").join(',') +
      ' ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)',
    sources.flatMap((source) => [source.scope_key, source.epoch]),
  );
}

export function lockSourceGates(connection, sources) {
  return query(
    connection,
    'SELECT scope_key,max_epoch,state FROM access_gate FORCE INDEX(PRIMARY) WHERE scope_key IN (' +
      sources.map(() => '?').join(',') +
      ') ORDER BY scope_key FOR UPDATE',
    sources.map((source) => source.scope_key),
  );
}
