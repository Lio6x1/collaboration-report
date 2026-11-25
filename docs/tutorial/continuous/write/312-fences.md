# 分步编写 `services/collaboration/src/repository/fences.mjs`

先按 [C03 · 撤权门闩、真实输出排空与进程恢复](../lessons/collaboration/03-fence-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/312-fences.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久化撤权计划、BLOCKED来源、关闭grant/回执和最终输出停止ACK。

“禁止新输出”和“旧输出已停止”是两个事实，需要分阶段持久化以恢复中途退出。

写之前先按这条执行路线理解各段的连接：

1. lockedClosure锁撤权号并校验重试
2. blockSource使用GREATEST防止旧epoch覆盖新代
3. closeGrants/closeReceipts仅关闭该scope且epoch<=目标的记录
4. completeClosure写OUTPUT_STOPPED authorization_ack再置SUCCEEDED
5. pendingClosures只取FENCING供启动恢复

本文件需要守住的边界：

- 关闭来源事务必须先提交，才执行真实输出排空
- 不可用TTL或delivery到期直接构造OUTPUT_STOPPED

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/fences.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/fences.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/fences.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/fences.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:127 -->
````javascript
// 准备阶段及旧空启动协议保持一秒查询期限；完整排空后的终态写入保留原无单条期限调用。
````

## 第 2 步：query

插入位置：第 1 步代码之后，紧接追加；完成稿约第 2 行。

所有方法使用调用者提供的同一连接，仓储不另开事务，也不凭TTL生成退出证明。

本步声明：`const query = (connection, sql, args = []) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>connection.query</code> |
| <code>sql</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>connection.query({ sql, timeout: 1000 }, args)</code> |
| <code>args = &#91;&#93;</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>connection.query({ sql, timeout: 1000 }, args)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>connection.query({ sql, timeout: 1000 }, args)</code> |

<!-- write-step: 2 bytes=127:333 -->
````javascript
// 所有方法使用调用者提供的同一连接，仓储不另开事务，也不凭TTL生成退出证明。
const query = (connection, sql, args = []) => connection.query({ sql, timeout: 1000 }, args);
````

## 第 3 步：lockedClosure

插入位置：第 2 步代码之后，紧接追加；完成稿约第 4 行。

本步声明：`export async function lockedClosure(connection, revocation) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE&#x27;,     &#91;revocation&#93;,   )</code> |
| <code>revocation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE&#x27;,     &#91;revocation&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>rows&#91;0&#93;</code> |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     &#x27;SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE&#x27;,     &#91;revocation&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return rows&#91;0&#93;;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=333:597 -->
````javascript
export async function lockedClosure(connection, revocation) {
  const [rows] = await query(
    connection,
    'SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE',
    [revocation],
  );
  return rows[0];
}
````

## 第 4 步：insertClosure

插入位置：第 3 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`export async function insertClosure(connection, revocation, scope, current) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&#93;&#x27;)&quot;,     &#91;revocatio …</code>（长表达式，按本步完整代码继续） |
| <code>revocation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&#93;&#x27;)&quot;,     &#91;revocatio …</code>（长表达式，按本步完整代码继续） |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>scope.scope_key</code>；<code>scope.epoch</code> |
| <code>current</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(current)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     &quot;INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&#93;&#x27;)&quot;,     &#91;revocatio …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 4 bytes=597:929 -->
````javascript
export async function insertClosure(connection, revocation, scope, current) {
  await query(
    connection,
    "INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,'FENCING','[]')",
    [revocation, scope.scope_key, scope.epoch, JSON.stringify(current)],
  );
}
````

## 第 5 步：blockSource

插入位置：第 4 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`export async function blockSource(connection, revocation, scope) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUE …</code>（长表达式，按本步完整代码继续） |
| <code>revocation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUE …</code>（长表达式，按本步完整代码继续） |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>scope.scope_key</code>；<code>scope.epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUE …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 5 bytes=929:1428 -->
````javascript
export async function blockSource(connection, revocation, scope) {
  await query(
    connection,
    "INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state=IF(VALUES(max_epoch)>=max_epoch,'BLOCKED',state),last_revocation_id=IF(VALUES(max_epoch)>=max_epoch,VALUES(last_revocation_id),last_revocation_id),revision=revision+1",
    [scope.scope_key, scope.epoch, revocation],
  );
}
````

## 第 6 步：closeGrants

插入位置：第 5 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`export async function closeGrants(connection, scope) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE collaboration_grant g JOIN grant_source s ON s.grant_id=g.grant_id SET g.state=&#x27;CLOSED&#x27; WHERE s.scope_key=? AND s.epoch&lt;=? AND g.state&lt;&gt;&#x27;CLOSED&#x27;&quot; …</code>（长表达式，按本步完整代码继续） |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>scope.scope_key</code>；<code>scope.epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     &quot;UPDATE collaboration_grant g JOIN grant_source s ON s.grant_id=g.grant_id SET g.state=&#x27;CLOSED&#x27; WHERE s.scope_key=? AND s.epoch&lt;=? AND g.state&lt;&gt;&#x27;CLOSED&#x27;&quot; …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 6 bytes=1428:1716 -->
````javascript
export async function closeGrants(connection, scope) {
  await query(
    connection,
    "UPDATE collaboration_grant g JOIN grant_source s ON s.grant_id=g.grant_id SET g.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=? AND g.state<>'CLOSED'",
    [scope.scope_key, scope.epoch],
  );
}
````

## 第 7 步：closeReceipts

插入位置：第 6 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`export async function closeReceipts(connection, scope) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE flush_receipt f JOIN collaboration_grant g ON g.grant_id=f.grant_id JOIN grant_source s ON s.grant_id=g.grant_id SET f.state=&#x27;CLOSED&#x27; WHERE s.sco …</code>（长表达式，按本步完整代码继续） |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>scope.scope_key</code>；<code>scope.epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     &quot;UPDATE flush_receipt f JOIN collaboration_grant g ON g.grant_id=f.grant_id JOIN grant_source s ON s.grant_id=g.grant_id SET f.state=&#x27;CLOSED&#x27; WHERE s.sco …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 7 bytes=1716:2187 -->
````javascript
export async function closeReceipts(connection, scope) {
  // 旧回执的关闭仍由同事务的真实grant/source决定；不得扫描其他范围的已关闭历史。
  await query(
    connection,
    "UPDATE flush_receipt f JOIN collaboration_grant g ON g.grant_id=f.grant_id JOIN grant_source s ON s.grant_id=g.grant_id SET f.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=? AND g.state='CLOSED' AND f.state<>'CLOSED'",
    [scope.scope_key, scope.epoch],
  );
}
````

## 第 8 步：completeClosure

插入位置：第 7 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`export async function completeClosure(connection, revocation, scope, digest, bootstrap = false) `。

**这一段为什么这样写：** 保存已经完成排空的终态证据。

1. 写node-runtime参与者ACK与digest
2. 将原authorization_change置SUCCEEDED


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 排空完成后的终态事务连接<br>在此声明中的实际使用：<code>query(connection, sql, args)</code>；<code>connection.query</code> |
| <code>revocation</code> | <code>JavaScript 参数</code> | 原撤权操作ID<br>在此声明中的实际使用：<code>execute(     &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;node-runtime&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIMESTAMP …</code>（长表达式，按本步完整代码继续）；<code>execute(&quot;UPDATE authorization_change SET state=&#x27;SUCCEEDED&#x27; WHERE revocation_id=?&quot;, &#91;     revocation,   &#93;)</code> |
| <code>scope</code> | <code>JavaScript 参数</code> | 固定待关scope_key与epoch<br>在此声明中的实际使用：<code>scope.scope_key</code>；<code>scope.epoch</code> |
| <code>digest</code> | <code>JavaScript 参数</code> | 由ack生成的确定性结果摘要<br>在此声明中的实际使用：<code>execute(     &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;node-runtime&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIMESTAMP …</code>（长表达式，按本步完整代码继续） |
| <code>bootstrap = false</code> | <code>带默认值的参数</code> | 仅历史空启动协议使用一秒查询期限 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>execute(     &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;node-runtime&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIMESTAMP …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>execute(&quot;UPDATE authorization_change SET state=&#x27;SUCCEEDED&#x27; WHERE revocation_id=?&quot;, &#91;     revocation,   &#93;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 8 bytes=2187:2846 -->
````javascript
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
````

## 第 9 步：bootstrapBusinessCounts

插入位置：第 8 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`export async function* bootstrapBusinessCounts(connection) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT COUNT(*) AS n FROM &#x27; + table)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>query(connection, &#x27;SELECT COUNT(*) AS n FROM &#x27; + table)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 9 bytes=2846:3204 -->
````javascript
export async function* bootstrapBusinessCounts(connection) {
  // 表名是固定私有库清单，逐表按原顺序读取并立即交还，不能绕过非空拒绝边界。
  for (const table of ['room', 'collaboration_grant', 'delivery_session']) {
    const [[row]] = await query(connection, 'SELECT COUNT(*) AS n FROM ' + table);
    yield row.n;
  }
}
````

## 第 10 步：startBootstrap

插入位置：第 9 步代码之后，紧接追加；完成稿约第 60 行。

本步声明：`export const startBootstrap = (connection) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;START TRANSACTION&#x27;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &#x27;START TRANSACTION&#x27;)</code> |

<!-- write-step: 10 bytes=3204:3290 -->
````javascript
export const startBootstrap = (connection) => query(connection, 'START TRANSACTION');
````

## 第 11 步：commitBootstrap

插入位置：第 10 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`export const commitBootstrap = (connection) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;COMMIT&#x27;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &#x27;COMMIT&#x27;)</code> |

<!-- write-step: 11 bytes=3290:3366 -->
````javascript
export const commitBootstrap = (connection) => query(connection, 'COMMIT');
````

## 第 12 步：rollbackBootstrap

插入位置：第 11 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`export const rollbackBootstrap = (connection) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;ROLLBACK&#x27;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &#x27;ROLLBACK&#x27;)</code> |

<!-- write-step: 12 bytes=3366:3446 -->
````javascript
export const rollbackBootstrap = (connection) => query(connection, 'ROLLBACK');
````

## 第 13 步：pendingClosures

插入位置：第 12 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`export async function pendingClosures(pool) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pool</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>pool.query</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>rows</code> |

**跟着本段代码走一遍：**

- 等待 <code>pool.query(     &quot;SELECT revocation_id,scope_key,epoch,target_permission FROM authorization_change WHERE state=&#x27;FENCING&#x27;&quot;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return rows;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=3446:3659 -->
````javascript
export async function pendingClosures(pool) {
  const [rows] = await pool.query(
    "SELECT revocation_id,scope_key,epoch,target_permission FROM authorization_change WHERE state='FENCING'",
  );
  return rows;
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/fences.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/03-fence-recovery.md)。
