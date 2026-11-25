# 分步编写 `services/collaboration/src/repository/room_access.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/319-room-access.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读写房间、准入grant、来源绑定和真实输出delivery_session。

grant是权限证明，delivery是具体套接字输出；撤权必须能追踪每个仍会发数据的delivery。

写之前先按这条执行路线理解各段的连接：

1. lockRoom/lockWorkspaceRoom锁定房间身份与代次
2. insertGrant与insertGrantSource保存固定准入及所有权限来源
3. insertDelivery绑定grant、instance和handler
4. pendingScopeDeliveries/pendingRoomDeliveries查待排空输出，settleDelivery记录真实停止结果

本文件需要守住的边界：

- expires_at不能替代socket已停止的事实
- 插入grant/source/delivery需要与准入核验使用同一事务

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/room_access.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/room_access.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/room_access.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/room_access.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:106 -->
````javascript
import { query } from './sql.mjs';

// 来源 room_admissions.mjs：原业务声明与函数体保留。

````

## 第 2 步：lockRoom

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

本步声明：`export function lockRoom(connection, room) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM room WHERE id=? FOR UPDATE&#x27;, &#91;room&#93;)</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM room WHERE id=? FOR UPDATE&#x27;, &#91;room&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(connection, &#x27;SELECT * FROM room WHERE id=? FOR UPDATE&#x27;, &#91;room&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(connection, &#x27;SELECT * FROM room WHERE id=? FOR UPDATE&#x27;, &#91;room&#93;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=106:338 -->
````javascript
// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function lockRoom(connection, room) {
  return query(connection, 'SELECT * FROM room WHERE id=? FOR UPDATE', [room]);
}

````

## 第 3 步：insertRoom

插入位置：第 2 步代码之后，紧接追加；完成稿约第 11 行。

本步声明：`export function insertRoom(connection, room, document, unit, kind, workspace, epoch) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;)&quot;,     &#91;room, document, unit, kind, workspace, epoch&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;)&quot;,     &#91;room, document, unit, kind, workspace, epoch&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;)&quot;,     &#91;room, document, unit, kind, workspace, epoch&#93;,   )</code> |
| <code>unit</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;)&quot;,     &#91;room, document, unit, kind, workspace, epoch&#93;,   )</code> |
| <code>kind</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;)&quot;,     &#91;room, document, unit, kind, workspace, epoch&#93;,   )</code> |
| <code>workspace</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;)&quot;,     &#91;room, document, unit, kind, workspace, epoch&#93;,   )</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;)&quot;,     &#91;room, document, unit, kind, workspace, epoch&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;)&quot;,     &#91;room, document, unit, kind, workspace, epoch&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;)&quot;,     &#91;room, document, unit, kind, workspace, epoch&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=338:632 -->
````javascript
export function insertRoom(connection, room, document, unit, kind, workspace, epoch) {
  return query(
    connection,
    "INSERT INTO room(id,document_id,unit_id,workspace_kind,workspace_id,epoch,state) VALUES(?,?,?,?,?,?,'OPEN')",
    [room, document, unit, kind, workspace, epoch],
  );
}

````

## 第 4 步：lockGrant

插入位置：第 3 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`export function lockGrant(connection, grant) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM collaboration_grant WHERE grant_id=? FOR UPDATE&#x27;, &#91;     grant,   &#93;)</code> |
| <code>grant</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM collaboration_grant WHERE grant_id=? FOR UPDATE&#x27;, &#91;     grant,   &#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(connection, &#x27;SELECT * FROM collaboration_grant WHERE grant_id=? FOR UPDATE&#x27;, &#91;     grant,   &#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(connection, &#x27;SELECT * FROM collaboration_grant WHERE grant_id=? FOR UPDATE&#x27;, &#91;     grant,   &#93;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=632:793 -->
````javascript
export function lockGrant(connection, grant) {
  return query(connection, 'SELECT * FROM collaboration_grant WHERE grant_id=? FOR UPDATE', [
    grant,
  ]);
}

````

## 第 5 步：insertGrant

插入位置：第 4 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`export function insertGrant(`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |
| <code>grant</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |
| <code>admission</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |
| <code>session</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |
| <code>client</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |
| <code>capabilities</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |
| <code>expires</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state,expire …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;INSERT INTO collaboration_grant(grant_id,document_admission_ref,actor_id,session_ref,client_session_id,room_id,workspace_epoch,capabilities,state …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=793:1258 -->
````javascript
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

````

## 第 6 步：insertGrantSource

插入位置：第 5 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`export function insertGrantSource(connection, grant, scope, epoch) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;INSERT INTO grant_source(grant_id,scope_key,epoch) VALUES(?,?,?)&#x27;, &#91;     grant,     scope,     epoch,   &#93;)</code> |
| <code>grant</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;INSERT INTO grant_source(grant_id,scope_key,epoch) VALUES(?,?,?)&#x27;, &#91;     grant,     scope,     epoch,   &#93;)</code> |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;INSERT INTO grant_source(grant_id,scope_key,epoch) VALUES(?,?,?)&#x27;, &#91;     grant,     scope,     epoch,   &#93;)</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;INSERT INTO grant_source(grant_id,scope_key,epoch) VALUES(?,?,?)&#x27;, &#91;     grant,     scope,     epoch,   &#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(connection, &#x27;INSERT INTO grant_source(grant_id,scope_key,epoch) VALUES(?,?,?)&#x27;, &#91;     grant,     scope,     epoch,   &#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(connection, &#x27;INSERT INTO grant_source(grant_id,scope_key,epoch) VALUES(?,?,?)&#x27;, &#91;     grant,     scope,     epoch,   &#93;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1258:1466 -->
````javascript
export function insertGrantSource(connection, grant, scope, epoch) {
  return query(connection, 'INSERT INTO grant_source(grant_id,scope_key,epoch) VALUES(?,?,?)', [
    grant,
    scope,
    epoch,
  ]);
}

````

## 第 7 步：lockLiveGrant

插入位置：第 6 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`export function lockLiveGrant(connection, grant) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT *,expires_at&gt;UTC_TIMESTAMP(6) AS alive FROM collaboration_grant WHERE grant_id=? FOR UPDATE&#x27;,     &#91;grant&#93;,   )</code> |
| <code>grant</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT *,expires_at&gt;UTC_TIMESTAMP(6) AS alive FROM collaboration_grant WHERE grant_id=? FOR UPDATE&#x27;,     &#91;grant&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &#x27;SELECT *,expires_at&gt;UTC_TIMESTAMP(6) AS alive FROM collaboration_grant WHERE grant_id=? FOR UPDATE&#x27;,     &#91;grant&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &#x27;SELECT *,expires_at&gt;UTC_TIMESTAMP(6) AS alive FROM collaboration_grant WHERE grant_id=? FOR UPDATE&#x27;,     &#91;grant&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1466:1676 -->
````javascript
export function lockLiveGrant(connection, grant) {
  return query(
    connection,
    'SELECT *,expires_at>UTC_TIMESTAMP(6) AS alive FROM collaboration_grant WHERE grant_id=? FOR UPDATE',
    [grant],
  );
}

````

## 第 8 步：lockWorkspaceRoom

插入位置：第 7 步代码之后，紧接追加；完成稿约第 60 行。

本步声明：`export function lockWorkspaceRoom(connection, room, epoch) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM room WHERE id=? AND epoch=? FOR UPDATE&#x27;, &#91;room, epoch&#93;)</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM room WHERE id=? AND epoch=? FOR UPDATE&#x27;, &#91;room, epoch&#93;)</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM room WHERE id=? AND epoch=? FOR UPDATE&#x27;, &#91;room, epoch&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(connection, &#x27;SELECT * FROM room WHERE id=? AND epoch=? FOR UPDATE&#x27;, &#91;room, epoch&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(connection, &#x27;SELECT * FROM room WHERE id=? AND epoch=? FOR UPDATE&#x27;, &#91;room, epoch&#93;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1676:1911 -->
````javascript
export function lockWorkspaceRoom(connection, room, epoch) {
  return query(connection, 'SELECT * FROM room WHERE id=? AND epoch=? FOR UPDATE', [room, epoch]);
}

// 来源 delivery_sessions.mjs：原业务声明与函数体保留。

````

## 第 9 步：insertDelivery

插入位置：第 8 步代码之后，紧接追加；完成稿约第 66 行。

所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

本步声明：`export function insertDelivery(connection, delivery, grant, instance, handler, expires) `。

**这一段为什么这样写：** 为每次实际连接留下可恢复的输出登记。

1. 保存delivery→grant→instance/handler关系
2. 初始状态OPEN，后续排空才SETTLED


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 当前准入事务连接<br>在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO delivery_session(delivery_id,grant_id,instance_id,handler_id,state,expires_at) VALUES(?,?,?,?,&#x27;OPEN&#x27;,?)&quot;,     &#91;delivery, grant, instance, handler, expires&#93;,   )</code> |
| <code>delivery</code> | <code>JavaScript 参数</code> | 本次输出随机ID<br>在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO delivery_session(delivery_id,grant_id,instance_id,handler_id,state,expires_at) VALUES(?,?,?,?,&#x27;OPEN&#x27;,?)&quot;,     &#91;delivery, grant, instance, handler, expires&#93;,   )</code> |
| <code>grant</code> | <code>JavaScript 参数</code> | Doc签发的grant_id<br>在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO delivery_session(delivery_id,grant_id,instance_id,handler_id,state,expires_at) VALUES(?,?,?,?,&#x27;OPEN&#x27;,?)&quot;,     &#91;delivery, grant, instance, handler, expires&#93;,   )</code> |
| <code>instance</code> | <code>JavaScript 参数</code> | 本Node进程实例ID<br>在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO delivery_session(delivery_id,grant_id,instance_id,handler_id,state,expires_at) VALUES(?,?,?,?,&#x27;OPEN&#x27;,?)&quot;,     &#91;delivery, grant, instance, handler, expires&#93;,   )</code> |
| <code>handler</code> | <code>JavaScript 参数</code> | Edge/Node输出链路处理器身份<br>在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO delivery_session(delivery_id,grant_id,instance_id,handler_id,state,expires_at) VALUES(?,?,?,?,&#x27;OPEN&#x27;,?)&quot;,     &#91;delivery, grant, instance, handler, expires&#93;,   )</code> |
| <code>expires</code> | <code>JavaScript 参数</code> | 准入到期时间，不是退出证明<br>在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO delivery_session(delivery_id,grant_id,instance_id,handler_id,state,expires_at) VALUES(?,?,?,?,&#x27;OPEN&#x27;,?)&quot;,     &#91;delivery, grant, instance, handler, expires&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;INSERT INTO delivery_session(delivery_id,grant_id,instance_id,handler_id,state,expires_at) VALUES(?,?,?,?,&#x27;OPEN&#x27;,?)&quot;,     &#91;delivery, grant, instance, handler, expires&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;INSERT INTO delivery_session(delivery_id,grant_id,instance_id,handler_id,state,expires_at) VALUES(?,?,?,?,&#x27;OPEN&#x27;,?)&quot;,     &#91;delivery, grant, instance, handler, expires&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=1911:2319 -->
````javascript
// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function insertDelivery(connection, delivery, grant, instance, handler, expires) {
  return query(
    connection,
    "INSERT INTO delivery_session(delivery_id,grant_id,instance_id,handler_id,state,expires_at) VALUES(?,?,?,?,'OPEN',?)",
    [delivery, grant, instance, handler, expires],
  );
}

````

## 第 10 步：settleDelivery

插入位置：第 9 步代码之后，紧接追加；完成稿约第 76 行。

本步声明：`export function settleDelivery(connection, reason, delivery, instance) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=? WHERE delivery_id=? AND instance_id=?&quot;,     &#91;reason, delivery, instance&#93;,   )</code> |
| <code>reason</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=? WHERE delivery_id=? AND instance_id=?&quot;,     &#91;reason, delivery, instance&#93;,   )</code> |
| <code>delivery</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=? WHERE delivery_id=? AND instance_id=?&quot;,     &#91;reason, delivery, instance&#93;,   )</code> |
| <code>instance</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=? WHERE delivery_id=? AND instance_id=?&quot;,     &#91;reason, delivery, instance&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=? WHERE delivery_id=? AND instance_id=?&quot;,     &#91;reason, delivery, instance&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=? WHERE delivery_id=? AND instance_id=?&quot;,     &#91;reason, delivery, instance&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2319:2569 -->
````javascript
export function settleDelivery(connection, reason, delivery, instance) {
  return query(
    connection,
    "UPDATE delivery_session SET state='SETTLED',stop_reason=? WHERE delivery_id=? AND instance_id=?",
    [reason, delivery, instance],
  );
}

````

## 第 11 步：pendingScopeDeliveries

插入位置：第 10 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`export function pendingScopeDeliveries(connection, scope, epoch) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN grant_source s ON s.grant_id=d.grant_id WHERE s.scope_key=? AND s.epoch&lt;=? AND d.state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;scope, epoch&#93;,   )</code> |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN grant_source s ON s.grant_id=d.grant_id WHERE s.scope_key=? AND s.epoch&lt;=? AND d.state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;scope, epoch&#93;,   )</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN grant_source s ON s.grant_id=d.grant_id WHERE s.scope_key=? AND s.epoch&lt;=? AND d.state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;scope, epoch&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN grant_source s ON s.grant_id=d.grant_id WHERE s.scope_key=? AND s.epoch&lt;=? AND d.state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;scope, epoch&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN grant_source s ON s.grant_id=d.grant_id WHERE s.scope_key=? AND s.epoch&lt;=? AND d. …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=2569:2865 -->
````javascript
export function pendingScopeDeliveries(connection, scope, epoch) {
  return query(
    connection,
    "SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN grant_source s ON s.grant_id=d.grant_id WHERE s.scope_key=? AND s.epoch<=? AND d.state<>'SETTLED'",
    [scope, epoch],
  );
}

````

## 第 12 步：settleClosedDelivery

插入位置：第 11 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`export function settleClosedDelivery(connection, delivery) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;SOURCE_CLOSED&#x27; WHERE delivery_id=?&quot;,     &#91;delivery&#93;,   )</code> |
| <code>delivery</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;SOURCE_CLOSED&#x27; WHERE delivery_id=?&quot;,     &#91;delivery&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;SOURCE_CLOSED&#x27; WHERE delivery_id=?&quot;,     &#91;delivery&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;SOURCE_CLOSED&#x27; WHERE delivery_id=?&quot;,     &#91;delivery&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=2865:3081 -->
````javascript
export function settleClosedDelivery(connection, delivery) {
  return query(
    connection,
    "UPDATE delivery_session SET state='SETTLED',stop_reason='SOURCE_CLOSED' WHERE delivery_id=?",
    [delivery],
  );
}

````

## 第 13 步：pendingRoomDeliveries

插入位置：第 12 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`export function pendingRoomDeliveries(connection, room) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;room&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;room&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;room&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;room&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=3081:3349 -->
````javascript
export function pendingRoomDeliveries(connection, room) {
  return query(
    connection,
    "SELECT d.delivery_id,d.instance_id FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state<>'SETTLED'",
    [room],
  );
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/room_access.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
