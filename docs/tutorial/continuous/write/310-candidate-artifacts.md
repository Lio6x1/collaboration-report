# 分步编写 `services/collaboration/src/repository/candidate_artifacts.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/310-candidate-artifacts.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

存取送审cut/候选、合并尝试和候选清理墓碑。

尚无业务行时也要同号互斥；正文删除后仍需留下已清理事实阻止迟到请求重新生成。

写之前先按这条执行路线理解各段的连接：

1. lockOperation/lockAttempt先用access_gate固定capture或merge业务键再读结果
2. insertCut保存CUT_FIXED和原lineage；saveCandidate转CAPTURED，bindResult绑定Go终态转BOUND
3. MERGE_ATTEMPT_CONTEXT与MERGE_CANDIDATE_RESULT分离登记/计算完成
4. saveCleanup保存CANDIDATE_CLEANED，清正文但保留绑定证据

本文件需要守住的边界：

- 登记任务SUCCEEDED只证明上下文已登记，不等于计算结果存在
- 清理仅删除候选正文，不能删除防重生墓碑

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/candidate_artifacts.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/candidate_artifacts.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/candidate_artifacts.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/candidate_artifacts.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:36 -->
````javascript
import { query } from './sql.mjs';

````

## 第 2 步：lockOperation

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

来源 captures.mjs：原业务声明与函数体保留。

本步声明：`export async function lockOperation(connection, operation) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;key&#93;,   )</code>；<code>query(connection, &#x27;SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;, &#91;key&#93;)</code>；<code>query(connection, &#x27;SELECT * FROM capture_operation WHERE operation_id=? FOR UPDATE&#x27;, &#91;     operation,   &#93;)</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>&#x27;capture:&#x27; + operation</code>；<code>query(connection, &#x27;SELECT * FROM capture_operation WHERE operation_id=? FOR UPDATE&#x27;, &#91;     operation,   &#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>query(connection, &#x27;SELECT * FROM capture_operation WHERE operation_id=? FOR UPDATE&#x27;, &#91;     operation,   &#93;)</code> |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;key&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(connection, &#x27;SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;, &#91;key&#93;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return query(connection, &#x27;SELECT * FROM capture_operation WHERE operation_id=? FOR UPDATE&#x27;, &#91;     operation,   &#93;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=36:593 -->
````javascript
// 来源 captures.mjs：原业务声明与函数体保留。

export async function lockOperation(connection, operation) {
  const key = 'capture:' + operation;
  await query(
    connection,
    "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)",
    [key],
  );
  await query(connection, 'SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE', [key]);
  return query(connection, 'SELECT * FROM capture_operation WHERE operation_id=? FOR UPDATE', [
    operation,
  ]);
}
````

## 第 3 步：findCapture

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`export const findCapture = (connection, operation) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM capture_operation WHERE operation_id=?&#x27;, &#91;operation&#93;)</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM capture_operation WHERE operation_id=?&#x27;, &#91;operation&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &#x27;SELECT * FROM capture_operation WHERE operation_id=?&#x27;, &#91;operation&#93;)</code> |

<!-- write-step: 3 bytes=593:737 -->
````javascript
export const findCapture = (connection, operation) =>
  query(connection, 'SELECT * FROM capture_operation WHERE operation_id=?', [operation]);
````

## 第 4 步：lockRoomLineage

插入位置：第 3 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`export const lockRoomLineage = (connection, room, epoch) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;ROOM_LINEAGE_CONTEXT&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;room + &#x27;:&#x27; + epoch&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27;</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27; + epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;ROOM_LINEAGE_CONTEXT&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;room + &#x27;:&#x27; + epoch&#93;,   )</code> |

<!-- write-step: 4 bytes=737:957 -->
````javascript
export const lockRoomLineage = (connection, room, epoch) =>
  query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='ROOM_LINEAGE_CONTEXT' AND business_key=? FOR UPDATE",
    [room + ':' + epoch],
  );
````

## 第 5 步：insertRoomLineage

插入位置：第 4 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`export const insertRoomLineage = (connection, room, epoch, payload) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;ROOM_LINEAGE_CONTEXT&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;room + &#x27;:&#x27; + epoch, JSON.stringify(payload)&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27;</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27; + epoch</code> |
| <code>payload</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;ROOM_LINEAGE_CONTEXT&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;room + &#x27;:&#x27; + epoch, JSON.stringify(payload)&#93;,   )</code> |

<!-- write-step: 5 bytes=957:1247 -->
````javascript
export const insertRoomLineage = (connection, room, epoch, payload) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('ROOM_LINEAGE_CONTEXT',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [room + ':' + epoch, JSON.stringify(payload)],
  );
````

## 第 6 步：insertCut

插入位置：第 5 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`export const insertCut = (connection, binding, room, cut, lineage) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO capture_operation(operation_id,request_fingerprint,actor_id,room_id,epoch,cut_ref,lineage_ref,state,generation,candidate_json,resource_refs) …</code>（长表达式，按本步完整代码继续） |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.context</code>；<code>binding.lineage_ref</code>；<code>binding.guard_generation</code>；<code>JSON.stringify({ binding, lineage })</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room.id</code>；<code>room.epoch</code> |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(cut)</code> |
| <code>lineage</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify({ binding, lineage })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO capture_operation(operation_id,request_fingerprint,actor_id,room_id,epoch,cut_ref,lineage_ref,state,generation,candidate_json,resource_refs) …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 6 bytes=1247:1860 -->
````javascript
export const insertCut = (connection, binding, room, cut, lineage) =>
  query(
    connection,
    "INSERT INTO capture_operation(operation_id,request_fingerprint,actor_id,room_id,epoch,cut_ref,lineage_ref,state,generation,candidate_json,resource_refs) VALUES(?,?,?,?,?,?,?,'CUT_FIXED',?,?,'[]')",
    [
      binding.context.operation_id,
      Buffer.from(binding.context.request_fingerprint, 'base64'),
      binding.context.actor_id,
      room.id,
      room.epoch,
      JSON.stringify(cut),
      binding.lineage_ref,
      binding.guard_generation,
      JSON.stringify({ binding, lineage }),
    ],
  );
````

## 第 7 步：saveCandidate

插入位置：第 6 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`export const saveCandidate = (connection, operation, candidate, bytes, digest, resources) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;CAPTURED&#x27;,candidate_json=?,canonical_body=?,body_hash=?,resource_refs=? WHERE operation_id=? AND state=&#x27;CUT_FIXED&#x27;&quot;, …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;CAPTURED&#x27;,candidate_json=?,canonical_body=?,body_hash=?,resource_refs=? WHERE operation_id=? AND state=&#x27;CUT_FIXED&#x27;&quot;, …</code>（长表达式，按本步完整代码继续） |
| <code>candidate</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(candidate)</code> |
| <code>bytes</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;CAPTURED&#x27;,candidate_json=?,canonical_body=?,body_hash=?,resource_refs=? WHERE operation_id=? AND state=&#x27;CUT_FIXED&#x27;&quot;, …</code>（长表达式，按本步完整代码继续） |
| <code>digest</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;CAPTURED&#x27;,candidate_json=?,canonical_body=?,body_hash=?,resource_refs=? WHERE operation_id=? AND state=&#x27;CUT_FIXED&#x27;&quot;, …</code>（长表达式，按本步完整代码继续） |
| <code>resources</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(resources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;CAPTURED&#x27;,candidate_json=?,canonical_body=?,body_hash=?,resource_refs=? WHERE operation_id=? AND state=&#x27;CUT_FIXED&#x27;&quot;, …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 7 bytes=1860:2227 -->
````javascript
export const saveCandidate = (connection, operation, candidate, bytes, digest, resources) =>
  query(
    connection,
    "UPDATE capture_operation SET state='CAPTURED',candidate_json=?,canonical_body=?,body_hash=?,resource_refs=? WHERE operation_id=? AND state='CUT_FIXED'",
    [JSON.stringify(candidate), bytes, digest, JSON.stringify(resources), operation],
  );
````

## 第 8 步：bindResult

插入位置：第 7 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`export const bindResult = (connection, operation, result) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;BOUND&#x27;,go_result_ref=? WHERE operation_id=? AND state IN (&#x27;CAPTURED&#x27;,&#x27;BOUND&#x27;)&quot;,     &#91;result, operation&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;BOUND&#x27;,go_result_ref=? WHERE operation_id=? AND state IN (&#x27;CAPTURED&#x27;,&#x27;BOUND&#x27;)&quot;,     &#91;result, operation&#93;,   )</code> |
| <code>result</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;BOUND&#x27;,go_result_ref=? WHERE operation_id=? AND state IN (&#x27;CAPTURED&#x27;,&#x27;BOUND&#x27;)&quot;,     &#91;result, operation&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;BOUND&#x27;,go_result_ref=? WHERE operation_id=? AND state IN (&#x27;CAPTURED&#x27;,&#x27;BOUND&#x27;)&quot;,     &#91;result, operation&#93;,   )</code> |

<!-- write-step: 8 bytes=2227:2465 -->
````javascript
export const bindResult = (connection, operation, result) =>
  query(
    connection,
    "UPDATE capture_operation SET state='BOUND',go_result_ref=? WHERE operation_id=? AND state IN ('CAPTURED','BOUND')",
    [result, operation],
  );

````

## 第 9 步：lockAttempt

插入位置：第 8 步代码之后，紧接追加；完成稿约第 60 行。

来源 merge_candidates.mjs：原业务声明与函数体保留。

本步声明：`export async function lockAttempt(connection, operation, attempt) `。

**这一段为什么这样写：** 串行化同一operation/attempt的首次登记、结果与清理。

1. 取得merge复合业务锁
2. 读取固定context/result
3. 同时读取cleaned墓碑


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 用例事务连接<br>在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;key&#93;,   )</code>；<code>query(connection, &#x27;SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;, &#91;key&#93;)</code>；<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MERGE_ATTEMPT_CONTEXT&#x27; AND business_key=?&quot;,     &#91;business&#93;,   )</code>；<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MERGE_CANDIDATE_RESULT&#x27; AND business_key=?&quot;,     &#91;business&#93;,   )</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>JavaScript 参数</code> | 主操作ID<br>在此声明中的实际使用：<code>&#x27;merge:&#x27; + operation</code>；<code>operation + &#x27;:&#x27;</code> |
| <code>attempt</code> | <code>JavaScript 参数</code> | 固定合并尝试ID<br>在此声明中的实际使用：<code>&#x27;merge:&#x27; + operation + &#x27;:&#x27; + attempt</code>；<code>operation + &#x27;:&#x27; + attempt</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>{ context, result, business, cleaned }</code> |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;key&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(connection, &#x27;SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;, &#91;key&#93;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MERGE_ATTEMPT_CONTEXT&#x27; AND business_key=?&quot;,     &#91;business&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MERGE_CANDIDATE_RESULT&#x27; AND business_key=?&quot;,     &#91;business&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>findCleanup(connection, &#x27;merge:&#x27; + business)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return { context, result, business, cleaned };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2465:3431 -->
````javascript
// 来源 merge_candidates.mjs：原业务声明与函数体保留。

export async function lockAttempt(connection, operation, attempt) {
  const key = 'merge:' + operation + ':' + attempt;
  await query(
    connection,
    "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)",
    [key],
  );
  await query(connection, 'SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE', [key]);
  const business = operation + ':' + attempt;
  const [[context]] = await query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='MERGE_ATTEMPT_CONTEXT' AND business_key=?",
    [business],
  );
  const [[result]] = await query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='MERGE_CANDIDATE_RESULT' AND business_key=?",
    [business],
  );
  const cleaned = await findCleanup(connection, 'merge:' + business);
  return { context, result, business, cleaned };
}
````

## 第 10 步：saveContext

插入位置：第 9 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`export const saveContext = (connection, business, binding) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;MERGE_ATTEMPT_CONTEXT&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;business, JSON.stringify(binding)&#93;,   )</code> |
| <code>business</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;MERGE_ATTEMPT_CONTEXT&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;business, JSON.stringify(binding)&#93;,   )</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;MERGE_ATTEMPT_CONTEXT&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;business, JSON.stringify(binding)&#93;,   )</code> |

<!-- write-step: 10 bytes=3431:3703 -->
````javascript
export const saveContext = (connection, business, binding) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('MERGE_ATTEMPT_CONTEXT',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [business, JSON.stringify(binding)],
  );
````

## 第 11 步：saveResult

插入位置：第 10 步代码之后，紧接追加；完成稿约第 90 行。

本步声明：`export const saveResult = (connection, business, result) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;MERGE_CANDIDATE_RESULT&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;business, JSON.stringify(result)&#93;,   )</code> |
| <code>business</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;MERGE_CANDIDATE_RESULT&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;business, JSON.stringify(result)&#93;,   )</code> |
| <code>result</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;MERGE_CANDIDATE_RESULT&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;business, JSON.stringify(result)&#93;,   )</code> |

<!-- write-step: 11 bytes=3703:3974 -->
````javascript
export const saveResult = (connection, business, result) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('MERGE_CANDIDATE_RESULT',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [business, JSON.stringify(result)],
  );

````

## 第 12 步：findCleanup

插入位置：第 11 步代码之后，紧接追加；完成稿约第 97 行。

来源 candidate_cleanup.mjs：原业务声明与函数体保留。

本步声明：`export async function findCleanup(connection, reference) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;CANDIDATE_CLEANED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;,     &#91;reference&#93;,   )</code> |
| <code>reference</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;CANDIDATE_CLEANED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;,     &#91;reference&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>row</code> |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;CANDIDATE_CLEANED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;,     &#91;reference&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return row;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3974:4303 -->
````javascript
// 来源 candidate_cleanup.mjs：原业务声明与函数体保留。

export async function findCleanup(connection, reference) {
  const [[row]] = await query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='CANDIDATE_CLEANED' AND business_key=? AND state='SUCCEEDED'",
    [reference],
  );
  return row;
}

````

## 第 13 步：saveCleanup

插入位置：第 12 步代码之后，紧接追加；完成稿约第 108 行。

本步声明：`export const saveCleanup = (connection, binding, result) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;CANDIDATE_CLEANED&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;bi …</code>（长表达式，按本步完整代码继续） |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.candidate_ref</code>；<code>binding.operation_id</code>；<code>JSON.stringify({ binding, result })</code> |
| <code>result</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify({ binding, result })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;CANDIDATE_CLEANED&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;bi …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 13 bytes=4303:4632 -->
````javascript
export const saveCleanup = (connection, binding, result) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('CANDIDATE_CLEANED',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [binding.candidate_ref, binding.operation_id, JSON.stringify({ binding, result })],
  );

````

## 第 14 步：cleanCaptureBody

插入位置：第 13 步代码之后，紧接追加；完成稿约第 115 行。

本步声明：`export const cleanCaptureBody = (connection, operation, candidate) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE capture_operation SET canonical_body=NULL,candidate_json=?,resource_refs=JSON_ARRAY() WHERE operation_id=? AND state=&#x27;BOUND&#x27;&quot;,     &#91;JSON.stringify(candidate), operation&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE capture_operation SET canonical_body=NULL,candidate_json=?,resource_refs=JSON_ARRAY() WHERE operation_id=? AND state=&#x27;BOUND&#x27;&quot;,     &#91;JSON.stringify(candidate), operation&#93;,   )</code> |
| <code>candidate</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(candidate)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE capture_operation SET canonical_body=NULL,candidate_json=?,resource_refs=JSON_ARRAY() WHERE operation_id=? AND state=&#x27;BOUND&#x27;&quot;,     &#91;JSON.stringify(candidate), operation&#93;,   )</code> |

<!-- write-step: 14 bytes=4632:4916 -->
````javascript
export const cleanCaptureBody = (connection, operation, candidate) =>
  query(
    connection,
    "UPDATE capture_operation SET canonical_body=NULL,candidate_json=?,resource_refs=JSON_ARRAY() WHERE operation_id=? AND state='BOUND'",
    [JSON.stringify(candidate), operation],
  );

````

## 第 15 步：cleanMergeBody

插入位置：第 14 步代码之后，紧接追加；完成稿约第 122 行。

本步声明：`export const cleanMergeBody = (connection, business) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;DELETE FROM durable_task WHERE kind=&#x27;MERGE_CANDIDATE_RESULT&#x27; AND business_key=?&quot;,     &#91;business&#93;,   )</code> |
| <code>business</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;DELETE FROM durable_task WHERE kind=&#x27;MERGE_CANDIDATE_RESULT&#x27; AND business_key=?&quot;,     &#91;business&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;DELETE FROM durable_task WHERE kind=&#x27;MERGE_CANDIDATE_RESULT&#x27; AND business_key=?&quot;,     &#91;business&#93;,   )</code> |

<!-- write-step: 15 bytes=4916:5105 -->
````javascript
export const cleanMergeBody = (connection, business) =>
  query(
    connection,
    "DELETE FROM durable_task WHERE kind='MERGE_CANDIDATE_RESULT' AND business_key=?",
    [business],
  );
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/candidate_artifacts.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
