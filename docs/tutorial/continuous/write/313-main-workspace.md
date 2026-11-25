# 分步编写 `services/collaboration/src/repository/main_workspace.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/313-main-workspace.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

管理文档级MAIN互斥、固定全篇捕获、屏障和安装回执。

不同MAIN房间和未来尚未建立房间必须受同一文档锁约束，安装重试又不能覆盖后来的新代次。

写之前先按这条执行路线理解各段的连接：

1. lockDocument取得main:document access_gate锁
2. lockMainRooms按id锁全部活跃MAIN，mainContext绑定物理来源
3. MAIN_VIEW_FIXED保存operation+usage结果；preparationFacts只取原context/cut不搬正文
4. prepareBarrier保存固定cut，安装事务退休旧room/关闭grant/标记输出并插install_receipt
5. installedBarrier或abortBarrier只推进原操作

本文件需要守住的边界：

- mainReadingRevision仅变化摘要输入，不是正文授权
- 安装回执需永久支持同号重放，不应在重试时重建最新MAIN

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/main_workspace.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/main_workspace.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/main_workspace.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/main_workspace.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:36 -->
````javascript
import { query } from './sql.mjs';

````

## 第 2 步：lockDocument

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

来源 main_barriers.mjs：原业务声明与函数体保留。

本步声明：`export async function lockDocument(connection, document) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;key&#93;,   )</code>；<code>query(connection, &#x27;SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;, &#91;key&#93;)</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>&#x27;main:&#x27; + document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;key&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(connection, &#x27;SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;, &#91;key&#93;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=36:475 -->
````javascript
// 来源 main_barriers.mjs：原业务声明与函数体保留。

export async function lockDocument(connection, document) {
  const key = 'main:' + document;
  await query(
    connection,
    "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)",
    [key],
  );
  await query(connection, 'SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE', [key]);
}
````

## 第 3 步：lockBarrier

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`export const lockBarrier = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM main_barrier WHERE document_id=? FOR UPDATE&#x27;, &#91;document&#93;)</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM main_barrier WHERE document_id=? FOR UPDATE&#x27;, &#91;document&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &#x27;SELECT * FROM main_barrier WHERE document_id=? FOR UPDATE&#x27;, &#91;document&#93;)</code> |

<!-- write-step: 3 bytes=475:622 -->
````javascript
export const lockBarrier = (connection, document) =>
  query(connection, 'SELECT * FROM main_barrier WHERE document_id=? FOR UPDATE', [document]);
````

## 第 4 步：lockMainRooms

插入位置：第 3 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`export const lockMainRooms = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT * FROM room WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id FOR UPDATE&quot;,     &#91;document&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT * FROM room WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id FOR UPDATE&quot;,     &#91;document&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;SELECT * FROM room WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id FOR UPDATE&quot;,     &#91;document&#93;,   )</code> |

<!-- write-step: 4 bytes=622:839 -->
````javascript
export const lockMainRooms = (connection, document) =>
  query(
    connection,
    "SELECT * FROM room WHERE document_id=? AND workspace_kind='MAIN' AND state<>'RETIRED' ORDER BY id FOR UPDATE",
    [document],
  );
````

## 第 5 步：mainReadingRevision

插入位置：第 4 步代码之后，紧接追加；完成稿约第 22 行。

阅读页只检查已持久化的MAIN尾部；不恢复Yjs、不创建快照，也不参与写入锁。

本步声明：`export const mainReadingRevision = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT id,epoch,last_durable_seq FROM room WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id&quot;,     &#91;document&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT id,epoch,last_durable_seq FROM room WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id&quot;,     &#91;document&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;SELECT id,epoch,last_durable_seq FROM room WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id&quot;,     &#91;document&#93;,   )</code> |

<!-- write-step: 5 bytes=839:1182 -->
````javascript
// 阅读页只检查已持久化的MAIN尾部；不恢复Yjs、不创建快照，也不参与写入锁。
export const mainReadingRevision = (connection, document) =>
  query(
    connection,
    "SELECT id,epoch,last_durable_seq FROM room WHERE document_id=? AND workspace_kind='MAIN' AND state<>'RETIRED' ORDER BY id",
    [document],
  );
````

## 第 6 步：mainContext

插入位置：第 5 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`export const mainContext = (connection, room, epoch) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;ROOM_MAIN_CONTEXT&#x27; AND business_key=?&quot;,     &#91;room + &#x27;:&#x27; + epoch&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27;</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27; + epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;ROOM_MAIN_CONTEXT&#x27; AND business_key=?&quot;,     &#91;room + &#x27;:&#x27; + epoch&#93;,   )</code> |

<!-- write-step: 6 bytes=1182:1384 -->
````javascript
export const mainContext = (connection, room, epoch) =>
  query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='ROOM_MAIN_CONTEXT' AND business_key=?",
    [room + ':' + epoch],
  );
````

## 第 7 步：insertMainContext

插入位置：第 6 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`export const insertMainContext = (connection, room, epoch, context) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;ROOM_MAIN_CONTEXT&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;room + &#x27;:&#x27; + epoch, JSON.stringify(context)&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27;</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27; + epoch</code> |
| <code>context</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(context)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;ROOM_MAIN_CONTEXT&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;room + &#x27;:&#x27; + epoch, JSON.stringify(context)&#93;,   )</code> |

<!-- write-step: 7 bytes=1384:1671 -->
````javascript
export const insertMainContext = (connection, room, epoch, context) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('ROOM_MAIN_CONTEXT',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [room + ':' + epoch, JSON.stringify(context)],
  );
````

## 第 8 步：prepareBarrier

插入位置：第 7 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`export const prepareBarrier = (connection, document, binding, cut) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO main_barrier(document_id,operation_id,generation,state,cut_vector,structure_ref,style_ref) VALUES(?,?,?,&#x27;PREPARED&#x27;,?,?,?) ON DUPLICATE KEY U …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO main_barrier(document_id,operation_id,generation,state,cut_vector,structure_ref,style_ref) VALUES(?,?,?,&#x27;PREPARED&#x27;,?,?,?) ON DUPLICATE KEY U …</code>（长表达式，按本步完整代码继续） |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.context</code>；<code>binding.barrier_generation</code>；<code>JSON.stringify({ binding, prepared_cut: cut })</code> |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify({ binding, prepared_cut: cut })</code>；<code>cut.structure_id</code>；<code>cut.style_id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO main_barrier(document_id,operation_id,generation,state,cut_vector,structure_ref,style_ref) VALUES(?,?,?,&#x27;PREPARED&#x27;,?,?,?) ON DUPLICATE KEY U …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 8 bytes=1671:2358 -->
````javascript
export const prepareBarrier = (connection, document, binding, cut) =>
  query(
    connection,
    "INSERT INTO main_barrier(document_id,operation_id,generation,state,cut_vector,structure_ref,style_ref) VALUES(?,?,?,'PREPARED',?,?,?) ON DUPLICATE KEY UPDATE operation_id=VALUES(operation_id),generation=VALUES(generation),state='PREPARED',cut_vector=VALUES(cut_vector),structure_ref=VALUES(structure_ref),style_ref=VALUES(style_ref),installed_manifest_id=NULL,go_result_digest=NULL",
    [
      document,
      binding.context.operation_id,
      binding.barrier_generation,
      JSON.stringify({ binding, prepared_cut: cut }),
      cut.structure_id,
      cut.style_id,
    ],
  );

````

## 第 9 步：lockCapture

插入位置：第 8 步代码之后，紧接追加；完成稿约第 55 行。

来源 main_captures.mjs：原业务声明与函数体保留。

本步声明：`export const lockCapture = (connection, key) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MAIN_VIEW_FIXED&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;key&#93;,   )</code> |
| <code>key</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MAIN_VIEW_FIXED&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;key&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MAIN_VIEW_FIXED&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;key&#93;,   )</code> |

<!-- write-step: 9 bytes=2358:2614 -->
````javascript
// 来源 main_captures.mjs：原业务声明与函数体保留。

export const lockCapture = (connection, key) =>
  query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND business_key=? FOR UPDATE",
    [key],
  );
````

## 第 10 步：insertCapture

插入位置：第 9 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`export const insertCapture = (connection, key, operation, payload) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;MAIN_VIEW_FIXED&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;key, …</code>（长表达式，按本步完整代码继续） |
| <code>key</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;MAIN_VIEW_FIXED&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;key, …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;MAIN_VIEW_FIXED&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;key, …</code>（长表达式，按本步完整代码继续） |
| <code>payload</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;MAIN_VIEW_FIXED&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;key, …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 10 bytes=2614:2910 -->
````javascript
export const insertCapture = (connection, key, operation, payload) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('MAIN_VIEW_FIXED',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [key, operation, JSON.stringify(payload)],
  );

````

## 第 11 步：preparationFacts

插入位置：第 10 步代码之后，紧接追加；完成稿约第 70 行。

原号前缀使用(kind,business_key)现有索引；只取得cut/context，避免把整篇固定正文再传一次。

本步声明：`export const preparationFacts = (connection, operation) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     `SELECT JSON_EXTRACT(payload,&#x27;$.binding.context&#x27;) context,     JSON_EXTRACT(payload,&#x27;$.cut&#x27;) cut,     JSON_UNQUOTE(JSON_EXTRACT(payload,&#x27;$.binding.expect …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>operation + &#x27;:&#x27;</code>；<code>operation + &#x27;;&#x27;</code>；<code>query(     connection,     `SELECT JSON_EXTRACT(payload,&#x27;$.binding.context&#x27;) context,     JSON_EXTRACT(payload,&#x27;$.cut&#x27;) cut,     JSON_UNQUOTE(JSON_EXTRACT(payload,&#x27;$.binding.expect …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     `SELECT JSON_EXTRACT(payload,&#x27;$.binding.context&#x27;) context,     JSON_EXTRACT(payload,&#x27;$.cut&#x27;) cut,     JSON_UNQUOTE(JSON_EXTRACT(payload,&#x27;$.binding.expect …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 11 bytes=2910:3580 -->
````javascript
// 原号前缀使用(kind,business_key)现有索引；只取得cut/context，避免把整篇固定正文再传一次。
export const preparationFacts = (connection, operation) =>
  query(
    connection,
    `SELECT JSON_EXTRACT(payload,'$.binding.context') context,
    JSON_EXTRACT(payload,'$.cut') cut,
    JSON_UNQUOTE(JSON_EXTRACT(payload,'$.binding.expected_manifest_id')) manifest,
    JSON_UNQUOTE(JSON_EXTRACT(payload,'$.binding.expected_control_revision')) control
    FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND business_key>=? AND business_key<?
    AND operation_id=? AND state='SUCCEEDED'`,
    [operation + ':', operation + ';', operation],
  );

````

## 第 12 步：lockReceipt

插入位置：第 11 步代码之后，紧接追加；完成稿约第 83 行。

来源 main_installations.mjs：原业务声明与函数体保留。

本步声明：`export const lockReceipt = (connection, operation) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM install_receipt WHERE operation_id=? FOR UPDATE&#x27;, &#91;operation&#93;)</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM install_receipt WHERE operation_id=? FOR UPDATE&#x27;, &#91;operation&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &#x27;SELECT * FROM install_receipt WHERE operation_id=? FOR UPDATE&#x27;, &#91;operation&#93;)</code> |

<!-- write-step: 12 bytes=3580:3806 -->
````javascript
// 来源 main_installations.mjs：原业务声明与函数体保留。

export const lockReceipt = (connection, operation) =>
  query(connection, 'SELECT * FROM install_receipt WHERE operation_id=? FOR UPDATE', [operation]);
````

## 第 13 步：retireMainRooms

插入位置：第 12 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`export const retireMainRooms = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE room SET state=&#x27;RETIRED&#x27; WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27;&quot;,     &#91;document&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE room SET state=&#x27;RETIRED&#x27; WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27;&quot;,     &#91;document&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE room SET state=&#x27;RETIRED&#x27; WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27;&quot;,     &#91;document&#93;,   )</code> |

<!-- write-step: 13 bytes=3806:4015 -->
````javascript
export const retireMainRooms = (connection, document) =>
  query(
    connection,
    "UPDATE room SET state='RETIRED' WHERE document_id=? AND workspace_kind='MAIN' AND state<>'RETIRED'",
    [document],
  );
````

## 第 14 步：retiredMainRooms

插入位置：第 13 步代码之后，紧接追加；完成稿约第 93 行。

本步声明：`export const retiredMainRooms = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT id FROM room WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state=&#x27;RETIRED&#x27; ORDER BY id&quot;,     &#91;document&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT id FROM room WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state=&#x27;RETIRED&#x27; ORDER BY id&quot;,     &#91;document&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;SELECT id FROM room WHERE document_id=? AND workspace_kind=&#x27;MAIN&#x27; AND state=&#x27;RETIRED&#x27; ORDER BY id&quot;,     &#91;document&#93;,   )</code> |

<!-- write-step: 14 bytes=4015:4224 -->
````javascript
export const retiredMainRooms = (connection, document) =>
  query(
    connection,
    "SELECT id FROM room WHERE document_id=? AND workspace_kind='MAIN' AND state='RETIRED' ORDER BY id",
    [document],
  );
````

## 第 15 步：closeRetiredGrants

插入位置：第 14 步代码之后，紧接追加；完成稿约第 99 行。

本步声明：`export const closeRetiredGrants = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE collaboration_grant g JOIN room r ON r.id=g.room_id SET g.state=&#x27;CLOSED&#x27; WHERE r.document_id=? AND r.workspace_kind=&#x27;MAIN&#x27; AND r.state=&#x27;RETIRED&#x27;&quot;,     &#91;document&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE collaboration_grant g JOIN room r ON r.id=g.room_id SET g.state=&#x27;CLOSED&#x27; WHERE r.document_id=? AND r.workspace_kind=&#x27;MAIN&#x27; AND r.state=&#x27;RETIRED&#x27;&quot;,     &#91;document&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE collaboration_grant g JOIN room r ON r.id=g.room_id SET g.state=&#x27;CLOSED&#x27; WHERE r.document_id=? AND r.workspace_kind=&#x27;MAIN&#x27; AND r.state=&#x27;RETIRED&#x27;&quot;,     &#91;document&#93;,   )</code> |

<!-- write-step: 15 bytes=4224:4489 -->
````javascript
export const closeRetiredGrants = (connection, document) =>
  query(
    connection,
    "UPDATE collaboration_grant g JOIN room r ON r.id=g.room_id SET g.state='CLOSED' WHERE r.document_id=? AND r.workspace_kind='MAIN' AND r.state='RETIRED'",
    [document],
  );
````

## 第 16 步：stopRetiredDeliveries

插入位置：第 15 步代码之后，紧接追加；完成稿约第 105 行。

本步声明：`export const stopRetiredDeliveries = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id JOIN room r ON r.id=g.room_id SET d.state=&#x27;STOPPING&#x27;,d.stop_reason=&#x27;MAIN_E …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id JOIN room r ON r.id=g.room_id SET d.state=&#x27;STOPPING&#x27;,d.stop_reason=&#x27;MAIN_E …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id JOIN room r ON r.id=g.room_id SET d.state=&#x27;STOPPING&#x27;,d.stop_reason=&#x27;MAIN_E …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 16 bytes=4489:4866 -->
````javascript
export const stopRetiredDeliveries = (connection, document) =>
  query(
    connection,
    "UPDATE delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id JOIN room r ON r.id=g.room_id SET d.state='STOPPING',d.stop_reason='MAIN_EPOCH_CHANGED' WHERE r.document_id=? AND r.workspace_kind='MAIN' AND r.state='RETIRED' AND d.state<>'SETTLED'",
    [document],
  );
````

## 第 17 步：updateLineage

插入位置：第 16 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`export const updateLineage = (connection, room, epoch, context) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET payload=? WHERE kind=&#x27;ROOM_LINEAGE_CONTEXT&#x27; AND business_key=?&quot;,     &#91;JSON.stringify(context), room + &#x27;:&#x27; + epoch&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27;</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27; + epoch</code> |
| <code>context</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(context)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE durable_task SET payload=? WHERE kind=&#x27;ROOM_LINEAGE_CONTEXT&#x27; AND business_key=?&quot;,     &#91;JSON.stringify(context), room + &#x27;:&#x27; + epoch&#93;,   )</code> |

<!-- write-step: 17 bytes=4866:5108 -->
````javascript
export const updateLineage = (connection, room, epoch, context) =>
  query(
    connection,
    "UPDATE durable_task SET payload=? WHERE kind='ROOM_LINEAGE_CONTEXT' AND business_key=?",
    [JSON.stringify(context), room + ':' + epoch],
  );
````

## 第 18 步：insertReceipt

插入位置：第 17 步代码之后，紧接追加；完成稿约第 117 行。

本步声明：`export const insertReceipt = (connection, input, cuts) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO install_receipt(operation_id,generation,manifest_id,manifest_digest,installed_epochs,state) VALUES(?,?,?,?,?,&#x27;INSTALLED&#x27;)&quot;,     &#91;       inpu …</code>（长表达式，按本步完整代码继续） |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.context</code>；<code>input.barrier_generation</code>；<code>input.manifest_id</code>；<code>input.manifest_digest</code> |
| <code>cuts</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(cuts)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO install_receipt(operation_id,generation,manifest_id,manifest_digest,installed_epochs,state) VALUES(?,?,?,?,?,&#x27;INSTALLED&#x27;)&quot;,     &#91;       inpu …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 18 bytes=5108:5511 -->
````javascript
export const insertReceipt = (connection, input, cuts) =>
  query(
    connection,
    "INSERT INTO install_receipt(operation_id,generation,manifest_id,manifest_digest,installed_epochs,state) VALUES(?,?,?,?,?,'INSTALLED')",
    [
      input.context.operation_id,
      input.barrier_generation,
      input.manifest_id,
      Buffer.from(input.manifest_digest),
      JSON.stringify(cuts),
    ],
  );
````

## 第 19 步：installedBarrier

插入位置：第 18 步代码之后，紧接追加；完成稿约第 129 行。

本步声明：`export const installedBarrier = (connection, document, operation, digest, manifest) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE main_barrier SET state=&#x27;INSTALLED&#x27;,installed_manifest_id=?,go_result_digest=? WHERE document_id=? AND operation_id=?&quot;,     &#91;manifest, Buffer.from(digest), document, operation&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE main_barrier SET state=&#x27;INSTALLED&#x27;,installed_manifest_id=?,go_result_digest=? WHERE document_id=? AND operation_id=?&quot;,     &#91;manifest, Buffer.from(digest), document, operation&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE main_barrier SET state=&#x27;INSTALLED&#x27;,installed_manifest_id=?,go_result_digest=? WHERE document_id=? AND operation_id=?&quot;,     &#91;manifest, Buffer.from(digest), document, operation&#93;,   )</code> |
| <code>digest</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Buffer.from(digest)</code> |
| <code>manifest</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE main_barrier SET state=&#x27;INSTALLED&#x27;,installed_manifest_id=?,go_result_digest=? WHERE document_id=? AND operation_id=?&quot;,     &#91;manifest, Buffer.from(digest), document, operation&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE main_barrier SET state=&#x27;INSTALLED&#x27;,installed_manifest_id=?,go_result_digest=? WHERE document_id=? AND operation_id=?&quot;,     &#91;manifest, Buffer.from(digest), document, operation&#93;,   )</code> |

<!-- write-step: 19 bytes=5511:5817 -->
````javascript
export const installedBarrier = (connection, document, operation, digest, manifest) =>
  query(
    connection,
    "UPDATE main_barrier SET state='INSTALLED',installed_manifest_id=?,go_result_digest=? WHERE document_id=? AND operation_id=?",
    [manifest, Buffer.from(digest), document, operation],
  );
````

## 第 20 步：abortBarrier

插入位置：第 19 步代码之后，紧接追加；完成稿约第 135 行。

本步声明：`export const abortBarrier = (connection, document, operation, generation) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE main_barrier SET state=&#x27;ABORTED&#x27; WHERE document_id=? AND operation_id=? AND generation=? AND state&lt;&gt;&#x27;INSTALLED&#x27;&quot;,     &#91;document, operation, generation&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE main_barrier SET state=&#x27;ABORTED&#x27; WHERE document_id=? AND operation_id=? AND generation=? AND state&lt;&gt;&#x27;INSTALLED&#x27;&quot;,     &#91;document, operation, generation&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE main_barrier SET state=&#x27;ABORTED&#x27; WHERE document_id=? AND operation_id=? AND generation=? AND state&lt;&gt;&#x27;INSTALLED&#x27;&quot;,     &#91;document, operation, generation&#93;,   )</code> |
| <code>generation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE main_barrier SET state=&#x27;ABORTED&#x27; WHERE document_id=? AND operation_id=? AND generation=? AND state&lt;&gt;&#x27;INSTALLED&#x27;&quot;,     &#91;document, operation, generation&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE main_barrier SET state=&#x27;ABORTED&#x27; WHERE document_id=? AND operation_id=? AND generation=? AND state&lt;&gt;&#x27;INSTALLED&#x27;&quot;,     &#91;document, operation, generation&#93;,   )</code> |

<!-- write-step: 20 bytes=5817:6089 -->
````javascript
export const abortBarrier = (connection, document, operation, generation) =>
  query(
    connection,
    "UPDATE main_barrier SET state='ABORTED' WHERE document_id=? AND operation_id=? AND generation=? AND state<>'INSTALLED'",
    [document, operation, generation],
  );
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/main_workspace.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
