# services/collaboration/src/repository/source_gates.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/322-source-gates.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`ensureSourceGate`、`lockSourceGate`、`reopenSourceGate`、`ensureSourceGates`、`lockSourceGates`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`a7f226f913e110cb2d2c82bb50336405abf120d946368803640a831851633c39`。[参考实现](../../../../services/collaboration/src/repository/source_gates.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
