# 分步编写 `services/collaboration/src/repository/room_persistence.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/320-room-persistence.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

存取UPDATE幂等日志、房间持久序号和FLUSH固定回执。

浏览器重试同一更新和点击保存都必须复用原事实，不能因为网络重传产生新版本或更换依赖。

写之前先按这条执行路线理解各段的连接：

1. findUpdate按room/epoch/client/update查既有seq和hash
2. insertUpdate与advanceDurableSeq在同事务写入实际字节和房间cut
3. FLUSH_BINDING先固定operation的请求指纹
4. findDependencyHash逐条证明依赖已落库，再插入FIXED flush_receipt

本文件需要守住的边界：

- 更新日志和last_durable_seq不能分开提交
- 同UPDATE ID不同hash、同FLUSH ID不同指纹应由用例拒绝

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/room_persistence.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/room_persistence.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/room_persistence.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/room_persistence.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:101 -->
````javascript
import { query } from './sql.mjs';

// 来源 update_log.mjs：原业务声明与函数体保留。

````

## 第 2 步：findUpdate

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

本步声明：`export function findUpdate(connection, room, epoch, client, update) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |
| <code>client</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |
| <code>update</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &#x27;SELECT seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &#x27;SELECT seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=101:472 -->
````javascript
// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function findUpdate(connection, room, epoch, client, update) {
  return query(
    connection,
    'SELECT seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?',
    [room, epoch, client, update],
  );
}

````

## 第 3 步：insertUpdate

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`export function insertUpdate(connection, room, epoch, seq, client, update, payload, hash) `。

**这一段为什么这样写：** 保存能被恢复和重试查询的真实Yjs增量。

1. 写入房间代次、连续seq、客户端二元身份和字节哈希
2. 由外层同连接事务与序号推进一起提交


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | UPDATE用例事务连接<br>在此声明中的实际使用：<code>query(     connection,     &#x27;INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)&#x27;,     &#91;room, epoch, seq, client, update, payload, hash&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 房间ID<br>在此声明中的实际使用：<code>query(     connection,     &#x27;INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)&#x27;,     &#91;room, epoch, seq, client, update, payload, hash&#93;,   )</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 房间代次字符串<br>在此声明中的实际使用：<code>query(     connection,     &#x27;INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)&#x27;,     &#91;room, epoch, seq, client, update, payload, hash&#93;,   )</code> |
| <code>seq</code> | <code>JavaScript 参数</code> | 本次持久序号字符串<br>在此声明中的实际使用：<code>query(     connection,     &#x27;INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)&#x27;,     &#91;room, epoch, seq, client, update, payload, hash&#93;,   )</code> |
| <code>client</code> | <code>JavaScript 参数</code> | 客户端会话ID<br>在此声明中的实际使用：<code>query(     connection,     &#x27;INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)&#x27;,     &#91;room, epoch, seq, client, update, payload, hash&#93;,   )</code> |
| <code>update</code> | <code>JavaScript 参数</code> | 客户端更新ID<br>在此声明中的实际使用：<code>query(     connection,     &#x27;INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)&#x27;,     &#91;room, epoch, seq, client, update, payload, hash&#93;,   )</code> |
| <code>payload</code> | <code>JavaScript 参数</code> | 解码并验证过的Yjs字节<br>在此声明中的实际使用：<code>query(     connection,     &#x27;INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)&#x27;,     &#91;room, epoch, seq, client, update, payload, hash&#93;,   )</code> |
| <code>hash</code> | <code>JavaScript 参数</code> | payload的32字节SHA-256<br>在此声明中的实际使用：<code>query(     connection,     &#x27;INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)&#x27;,     &#91;room, epoch, seq, client, update, payload, hash&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &#x27;INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)&#x27;,     &#91;room, epoch, seq, client, update, payload, hash&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &#x27;INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)&#x27;,     &#91;room, epoch, seq, client, update, payload, hash&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=472:786 -->
````javascript
export function insertUpdate(connection, room, epoch, seq, client, update, payload, hash) {
  return query(
    connection,
    'INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)',
    [room, epoch, seq, client, update, payload, hash],
  );
}

````

## 第 4 步：advanceDurableSeq

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`export function advanceDurableSeq(connection, seq, room, epoch) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;UPDATE room SET last_durable_seq=? WHERE id=? AND epoch=?&#x27;, &#91;     seq,     room,     epoch,   &#93;)</code> |
| <code>seq</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;UPDATE room SET last_durable_seq=? WHERE id=? AND epoch=?&#x27;, &#91;     seq,     room,     epoch,   &#93;)</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;UPDATE room SET last_durable_seq=? WHERE id=? AND epoch=?&#x27;, &#91;     seq,     room,     epoch,   &#93;)</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;UPDATE room SET last_durable_seq=? WHERE id=? AND epoch=?&#x27;, &#91;     seq,     room,     epoch,   &#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(connection, &#x27;UPDATE room SET last_durable_seq=? WHERE id=? AND epoch=?&#x27;, &#91;     seq,     room,     epoch,   &#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(connection, &#x27;UPDATE room SET last_durable_seq=? WHERE id=? AND epoch=?&#x27;, &#91;     seq,     room,     epoch,   &#93;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=786:981 -->
````javascript
export function advanceDurableSeq(connection, seq, room, epoch) {
  return query(connection, 'UPDATE room SET last_durable_seq=? WHERE id=? AND epoch=?', [
    seq,
    room,
    epoch,
  ]);
}

````

## 第 5 步：findDependencyHash

插入位置：第 4 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`export function findDependencyHash(connection, room, epoch, client, update) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |
| <code>client</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |
| <code>update</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &#x27;SELECT payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &#x27;SELECT payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;room, epoch, client, update&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=981:1321 -->
````javascript
export function findDependencyHash(connection, room, epoch, client, update) {
  return query(
    connection,
    'SELECT payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?',
    [room, epoch, client, update],
  );
}

// 来源 flush_receipts.mjs：原业务声明与函数体保留。

````

## 第 6 步：lockFlushBinding

插入位置：第 5 步代码之后，紧接追加；完成稿约第 41 行。

所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

本步声明：`export function lockFlushBinding(connection, operation) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;FLUSH_BINDING&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;operation&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;FLUSH_BINDING&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;operation&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;FLUSH_BINDING&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;operation&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;FLUSH_BINDING&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;operation&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1321:1637 -->
````javascript
// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function lockFlushBinding(connection, operation) {
  return query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='FLUSH_BINDING' AND business_key=? FOR UPDATE",
    [operation],
  );
}

````

## 第 7 步：insertFlushBinding

插入位置：第 6 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`export function insertFlushBinding(connection, operation, payload) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;FLUSH_BINDING&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;operation, payload&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;FLUSH_BINDING&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;operation, payload&#93;,   )</code> |
| <code>payload</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;FLUSH_BINDING&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;operation, payload&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;FLUSH_BINDING&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;operation, payload&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;FLUSH_BINDING&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;operation, payload&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1637:1903 -->
````javascript
export function insertFlushBinding(connection, operation, payload) {
  return query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('FLUSH_BINDING',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [operation, payload],
  );
}

````

## 第 8 步：findFlushReceipt

插入位置：第 7 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`export function findFlushReceipt(connection, operation) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM flush_receipt WHERE receipt_id=?&#x27;, &#91;operation&#93;)</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM flush_receipt WHERE receipt_id=?&#x27;, &#91;operation&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(connection, &#x27;SELECT * FROM flush_receipt WHERE receipt_id=?&#x27;, &#91;operation&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(connection, &#x27;SELECT * FROM flush_receipt WHERE receipt_id=?&#x27;, &#91;operation&#93;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1903:2055 -->
````javascript
export function findFlushReceipt(connection, operation) {
  return query(connection, 'SELECT * FROM flush_receipt WHERE receipt_id=?', [operation]);
}

````

## 第 9 步：insertFlushReceipt

插入位置：第 8 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`export function insertFlushReceipt(connection, receipt, grant, actor, room, epoch, cut, digest) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,&#x27;FIXED&#x27;)&quot;,     &#91;receipt,  …</code>（长表达式，按本步完整代码继续） |
| <code>receipt</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,&#x27;FIXED&#x27;)&quot;,     &#91;receipt,  …</code>（长表达式，按本步完整代码继续） |
| <code>grant</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,&#x27;FIXED&#x27;)&quot;,     &#91;receipt,  …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,&#x27;FIXED&#x27;)&quot;,     &#91;receipt,  …</code>（长表达式，按本步完整代码继续） |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,&#x27;FIXED&#x27;)&quot;,     &#91;receipt,  …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,&#x27;FIXED&#x27;)&quot;,     &#91;receipt,  …</code>（长表达式，按本步完整代码继续） |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,&#x27;FIXED&#x27;)&quot;,     &#91;receipt,  …</code>（长表达式，按本步完整代码继续） |
| <code>digest</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,&#x27;FIXED&#x27;)&quot;,     &#91;receipt,  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,&#x27;FIXED&#x27;)&quot;,     &#91;receipt,  …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,&#x27;FIXED&#x27;)&quot;,     &#91;re …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2055:2390 -->
````javascript
export function insertFlushReceipt(connection, receipt, grant, actor, room, epoch, cut, digest) {
  return query(
    connection,
    "INSERT INTO flush_receipt(receipt_id,grant_id,actor_id,room_id,epoch,covered_cut,dependency_digest,state) VALUES(?,?,?,?,?,?,?,'FIXED')",
    [receipt, grant, actor, room, epoch, cut, digest],
  );
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/room_persistence.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
