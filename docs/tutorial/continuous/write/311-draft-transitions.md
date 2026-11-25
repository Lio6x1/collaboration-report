# 分步编写 `services/collaboration/src/repository/draft_transitions.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/311-draft-transitions.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久化草稿预览/准备/安装事实及MAIN入口和room状态转换。

换代次需要在同事务里关闭旧room、grant和delivery，不能只建立新room留下旧写入口。

写之前先按这条执行路线理解各段的连接：

1. artifact按kind/key FOR UPDATE读固定事实，insertArtifact写不可变结果
2. setMainGate保存文档级PREPARED/INSTALLED/ABORTED
3. blockRoom从OPEN到BLOCKED；成功retireRoom，失败openRoom
4. closeGrants关闭旧准入，stopDeliveries标记BRANCH_EPOCH_CHANGED等待真实排空

本文件需要守住的边界：

- STOPPING不是SETTLED，后续必须等待真实输出关闭
- openRoom只解除BLOCKED，不能把RETIRED旧代次复活

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/draft_transitions.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/draft_transitions.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/draft_transitions.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/draft_transitions.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:36 -->
````javascript
import { query } from './sql.mjs';

````

## 第 2 步：setMainGate

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`export const setMainGate = (connection, document, operation, state, binding) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;DRAFT_MAIN_GATE&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICAT …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;DRAFT_MAIN_GATE&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICAT …</code>（长表达式，按本步完整代码继续）；<code>JSON.stringify({ document_id: document, operation_id: operation, state, binding })</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;DRAFT_MAIN_GATE&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICAT …</code>（长表达式，按本步完整代码继续）；<code>JSON.stringify({ document_id: document, operation_id: operation, state, binding })</code> |
| <code>state</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify({ document_id: document, operation_id: operation, state, binding })</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify({ document_id: document, operation_id: operation, state, binding })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;DRAFT_MAIN_GATE&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICAT …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 2 bytes=36:513 -->
````javascript
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

````

## 第 3 步：artifact

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`export const artifact = (connection, kind, key) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT payload FROM durable_task WHERE kind=? AND business_key=? FOR UPDATE&#x27;, &#91;     kind,     key,   &#93;)</code> |
| <code>kind</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT payload FROM durable_task WHERE kind=? AND business_key=? FOR UPDATE&#x27;, &#91;     kind,     key,   &#93;)</code> |
| <code>key</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT payload FROM durable_task WHERE kind=? AND business_key=? FOR UPDATE&#x27;, &#91;     kind,     key,   &#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &#x27;SELECT payload FROM durable_task WHERE kind=? AND business_key=? FOR UPDATE&#x27;, &#91;     kind,     key,   &#93;)</code> |

<!-- write-step: 3 bytes=513:690 -->
````javascript
export const artifact = (connection, kind, key) =>
  query(connection, 'SELECT payload FROM durable_task WHERE kind=? AND business_key=? FOR UPDATE', [
    kind,
    key,
  ]);
````

## 第 4 步：insertArtifact

插入位置：第 3 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`export const insertArtifact = (connection, kind, operation, payload) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;kind, operation, operation, JSON.stringify(payload)&#93;,   )</code> |
| <code>kind</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;kind, operation, operation, JSON.stringify(payload)&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;kind, operation, operation, JSON.stringify(payload)&#93;,   )</code> |
| <code>payload</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;kind, operation, operation, JSON.stringify(payload)&#93;,   )</code> |

<!-- write-step: 4 bytes=690:983 -->
````javascript
export const insertArtifact = (connection, kind, operation, payload) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [kind, operation, operation, JSON.stringify(payload)],
  );
````

## 第 5 步：unitMainRoom

插入位置：第 4 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`export const unitMainRoom = (connection, document, unit) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT * FROM room WHERE document_id=? AND unit_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id FOR UPDATE&quot;,     &#91;document, unit&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT * FROM room WHERE document_id=? AND unit_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id FOR UPDATE&quot;,     &#91;document, unit&#93;,   )</code> |
| <code>unit</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT * FROM room WHERE document_id=? AND unit_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id FOR UPDATE&quot;,     &#91;document, unit&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;SELECT * FROM room WHERE document_id=? AND unit_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id FOR UPDATE&quot;,     &#91;document, unit&#93;,   )</code> |

<!-- write-step: 5 bytes=983:1225 -->
````javascript
export const unitMainRoom = (connection, document, unit) =>
  query(
    connection,
    "SELECT * FROM room WHERE document_id=? AND unit_id=? AND workspace_kind='MAIN' AND state<>'RETIRED' ORDER BY id FOR UPDATE",
    [document, unit],
  );
````

## 第 6 步：blockRoom

插入位置：第 5 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`export const blockRoom = (connection, room, epoch) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE room SET state=&#x27;BLOCKED&#x27;,barrier_generation=barrier_generation+1 WHERE id=? AND epoch=? AND state=&#x27;OPEN&#x27;&quot;,     &#91;room, epoch&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE room SET state=&#x27;BLOCKED&#x27;,barrier_generation=barrier_generation+1 WHERE id=? AND epoch=? AND state=&#x27;OPEN&#x27;&quot;,     &#91;room, epoch&#93;,   )</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE room SET state=&#x27;BLOCKED&#x27;,barrier_generation=barrier_generation+1 WHERE id=? AND epoch=? AND state=&#x27;OPEN&#x27;&quot;,     &#91;room, epoch&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE room SET state=&#x27;BLOCKED&#x27;,barrier_generation=barrier_generation+1 WHERE id=? AND epoch=? AND state=&#x27;OPEN&#x27;&quot;,     &#91;room, epoch&#93;,   )</code> |

<!-- write-step: 6 bytes=1225:1447 -->
````javascript
export const blockRoom = (connection, room, epoch) =>
  query(
    connection,
    "UPDATE room SET state='BLOCKED',barrier_generation=barrier_generation+1 WHERE id=? AND epoch=? AND state='OPEN'",
    [room, epoch],
  );
````

## 第 7 步：openRoom

插入位置：第 6 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`export const openRoom = (connection, room, epoch) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &quot;UPDATE room SET state=&#x27;OPEN&#x27; WHERE id=? AND epoch=? AND state=&#x27;BLOCKED&#x27;&quot;, &#91;     room,     epoch,   &#93;)</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &quot;UPDATE room SET state=&#x27;OPEN&#x27; WHERE id=? AND epoch=? AND state=&#x27;BLOCKED&#x27;&quot;, &#91;     room,     epoch,   &#93;)</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &quot;UPDATE room SET state=&#x27;OPEN&#x27; WHERE id=? AND epoch=? AND state=&#x27;BLOCKED&#x27;&quot;, &#91;     room,     epoch,   &#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &quot;UPDATE room SET state=&#x27;OPEN&#x27; WHERE id=? AND epoch=? AND state=&#x27;BLOCKED&#x27;&quot;, &#91;     room,     epoch,   &#93;)</code> |

<!-- write-step: 7 bytes=1447:1624 -->
````javascript
export const openRoom = (connection, room, epoch) =>
  query(connection, "UPDATE room SET state='OPEN' WHERE id=? AND epoch=? AND state='BLOCKED'", [
    room,
    epoch,
  ]);
````

## 第 8 步：retireRoom

插入位置：第 7 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`export const retireRoom = (connection, room, epoch) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &quot;UPDATE room SET state=&#x27;RETIRED&#x27; WHERE id=? AND epoch=? AND state=&#x27;BLOCKED&#x27;&quot;, &#91;     room,     epoch,   &#93;)</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &quot;UPDATE room SET state=&#x27;RETIRED&#x27; WHERE id=? AND epoch=? AND state=&#x27;BLOCKED&#x27;&quot;, &#91;     room,     epoch,   &#93;)</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &quot;UPDATE room SET state=&#x27;RETIRED&#x27; WHERE id=? AND epoch=? AND state=&#x27;BLOCKED&#x27;&quot;, &#91;     room,     epoch,   &#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &quot;UPDATE room SET state=&#x27;RETIRED&#x27; WHERE id=? AND epoch=? AND state=&#x27;BLOCKED&#x27;&quot;, &#91;     room,     epoch,   &#93;)</code> |

<!-- write-step: 8 bytes=1624:1806 -->
````javascript
export const retireRoom = (connection, room, epoch) =>
  query(connection, "UPDATE room SET state='RETIRED' WHERE id=? AND epoch=? AND state='BLOCKED'", [
    room,
    epoch,
  ]);
````

## 第 9 步：closeGrants

插入位置：第 8 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`export const closeGrants = (connection, room) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE collaboration_grant SET state=&#x27;CLOSED&#x27; WHERE room_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;,     &#91;room&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE collaboration_grant SET state=&#x27;CLOSED&#x27; WHERE room_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;,     &#91;room&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE collaboration_grant SET state=&#x27;CLOSED&#x27; WHERE room_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;,     &#91;room&#93;,   )</code> |

<!-- write-step: 9 bytes=1806:1986 -->
````javascript
export const closeGrants = (connection, room) =>
  query(
    connection,
    "UPDATE collaboration_grant SET state='CLOSED' WHERE room_id=? AND state<>'CLOSED'",
    [room],
  );
````

## 第 10 步：stopDeliveries

插入位置：第 9 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`export const stopDeliveries = (connection, room) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id SET d.state=&#x27;STOPPING&#x27;,d.stop_reason=&#x27;BRANCH_EPOCH_CHANGED&#x27; WHERE g.room_i …</code>（长表达式，按本步完整代码继续） |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id SET d.state=&#x27;STOPPING&#x27;,d.stop_reason=&#x27;BRANCH_EPOCH_CHANGED&#x27; WHERE g.room_i …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id SET d.state=&#x27;STOPPING&#x27;,d.stop_reason=&#x27;BRANCH_EPOCH_CHANGED&#x27; WHERE g.room_i …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 10 bytes=1986:2266 -->
````javascript
export const stopDeliveries = (connection, room) =>
  query(
    connection,
    "UPDATE delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id SET d.state='STOPPING',d.stop_reason='BRANCH_EPOCH_CHANGED' WHERE g.room_id=? AND d.state<>'SETTLED'",
    [room],
  );
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/draft_transitions.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
