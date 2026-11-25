# 分步编写 `services/collaboration/src/repository/account_document_cleanup.mjs`

先按 [C06 · 固定候选清理、账号清理与转移计量](../lessons/collaboration/06-cleanup-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/309-account-document-cleanup.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按权威文档范围读取清理前置事实，清除正文并保存永久安装墓碑。

保留room身份/切点审计而删除正文后，启动恢复必须知道这是受控清理，迟到安装也必须被阻止。

写之前先按这条执行路线理解各段的连接：

1. cleanupReceipt支持账号清理和PURGE原号回执
2. documentArtifacts依cleanup_document索引限定已知kind
3. clearRoomBody退休room、删update/checkpoint、清候选正文和room上下文
4. 清完成的barrier正文并recordCleanup；RESOURCE_CONFIRM由应用保留审计

本文件需要守住的边界：

- 只清目标document的已知artifact，禁止按相同unit/用户宽泛扫描
- 清理回执必须与正文删除同事务，不能先删再补墓碑

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/account_document_cleanup.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/account_document_cleanup.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/account_document_cleanup.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/account_document_cleanup.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:36 -->
````javascript
import { query } from './sql.mjs';

````

## 第 2 步：draftInstallation

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`export const draftInstallation = async (connection, operation) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload,state,operation_id FROM durable_task WHERE kind=&#x27;DRAFT_TRANSITION_INSTALLED&#x27; AND business_key=?&quot;,     &#91;operation&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload,state,operation_id FROM durable_task WHERE kind=&#x27;DRAFT_TRANSITION_INSTALLED&#x27; AND business_key=?&quot;,     &#91;operation&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>row</code> |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     &quot;SELECT payload,state,operation_id FROM durable_task WHERE kind=&#x27;DRAFT_TRANSITION_INSTALLED&#x27; AND business_key=?&quot;,     &#91;operation&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return row;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=36:309 -->
````javascript
export const draftInstallation = async (connection, operation) => {
  const [[row]] = await query(
    connection,
    "SELECT payload,state,operation_id FROM durable_task WHERE kind='DRAFT_TRANSITION_INSTALLED' AND business_key=?",
    [operation],
  );
  return row;
};

````

## 第 3 步：cleanupReceipt

插入位置：第 2 步代码之后，紧接追加；完成稿约第 12 行。

单文档回执同时是永久的安装墓碑。已清理房间仍保留身份/切点审计，不再恢复正文。

本步声明：`export const cleanupReceipt = async (connection, document, operation) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     operation === undefined       ? &quot;SELECT operation_id,payload,state FROM durable_task WHERE (kind=&#x27;ACCOUNT_DOCUMENT_CLEANUP&#x27; AND business_key=?) OR (kind= …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     operation === undefined       ? &quot;SELECT operation_id,payload,state FROM durable_task WHERE (kind=&#x27;ACCOUNT_DOCUMENT_CLEANUP&#x27; AND business_key=?) OR (kind= …</code>（长表达式，按本步完整代码继续）；<code>document + &#x27;:%&#x27;</code>；<code>document + &#x27;:&#x27;</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>operation === undefined</code>；<code>query(     connection,     operation === undefined       ? &quot;SELECT operation_id,payload,state FROM durable_task WHERE (kind=&#x27;ACCOUNT_DOCUMENT_CLEANUP&#x27; AND business_key=?) OR (kind= …</code>（长表达式，按本步完整代码继续）；<code>document + &#x27;:&#x27; + operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>row</code> |

**跟着本段代码走一遍：**

- 等待 <code>query(     connection,     operation === undefined       ? &quot;SELECT operation_id,payload,state FROM durable_task WHERE (kind=&#x27;ACCOUNT_DOCUMENT_CLEANUP&#x27; AND business_key=?) OR (kind= …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return row;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=309:1139 -->
````javascript
// 单文档回执同时是永久的安装墓碑。已清理房间仍保留身份/切点审计，不再恢复正文。
export const cleanupReceipt = async (connection, document, operation) => {
  const [[row]] = await query(
    connection,
    operation === undefined
      ? "SELECT operation_id,payload,state FROM durable_task WHERE (kind='ACCOUNT_DOCUMENT_CLEANUP' AND business_key=?) OR (kind='PURGE_DOCUMENT_CLEANUP' AND business_key LIKE ?) ORDER BY id LIMIT 1"
      : "SELECT operation_id,payload,state FROM durable_task WHERE (kind='ACCOUNT_DOCUMENT_CLEANUP' AND business_key=? AND operation_id=?) OR (kind='PURGE_DOCUMENT_CLEANUP' AND business_key=?) ORDER BY id LIMIT 1",
    operation === undefined
      ? [document, document + ':%']
      : [document, operation, document + ':' + operation],
  );
  return row;
};

````

## 第 4 步：lockCleanupSource

插入位置：第 3 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`export const lockCleanupSource = (connection, scope) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;,     &#91;scope&#93;,   )</code> |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;,     &#91;scope&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &#x27;SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;,     &#91;scope&#93;,   )</code> |

<!-- write-step: 4 bytes=1139:1334 -->
````javascript
export const lockCleanupSource = (connection, scope) =>
  query(
    connection,
    'SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE',
    [scope],
  );

````

## 第 5 步：lockRooms

插入位置：第 4 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`export const lockRooms = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM room WHERE document_id=? ORDER BY id FOR UPDATE&#x27;, &#91;document&#93;)</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT * FROM room WHERE document_id=? ORDER BY id FOR UPDATE&#x27;, &#91;document&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &#x27;SELECT * FROM room WHERE document_id=? ORDER BY id FOR UPDATE&#x27;, &#91;document&#93;)</code> |

<!-- write-step: 5 bytes=1334:1484 -->
````javascript
export const lockRooms = (connection, document) =>
  query(connection, 'SELECT * FROM room WHERE document_id=? ORDER BY id FOR UPDATE', [document]);

````

## 第 6 步：pendingDeliveries

插入位置：第 5 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`export const pendingDeliveries = (connection, room) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT 1 FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state&lt;&gt;&#x27;SETTLED&#x27; LIMIT 1&quot;,     &#91;room&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT 1 FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state&lt;&gt;&#x27;SETTLED&#x27; LIMIT 1&quot;,     &#91;room&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;SELECT 1 FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state&lt;&gt;&#x27;SETTLED&#x27; LIMIT 1&quot;,     &#91;room&#93;,   )</code> |

<!-- write-step: 6 bytes=1484:1723 -->
````javascript
export const pendingDeliveries = (connection, room) =>
  query(
    connection,
    "SELECT 1 FROM delivery_session d JOIN collaboration_grant g ON g.grant_id=d.grant_id WHERE g.room_id=? AND d.state<>'SETTLED' LIMIT 1",
    [room],
  );

````

## 第 7 步：roomCaptures

插入位置：第 6 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`export const roomCaptures = (connection, room) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT operation_id,state FROM capture_operation WHERE room_id=? ORDER BY operation_id FOR UPDATE&#x27;,     &#91;room&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT operation_id,state FROM capture_operation WHERE room_id=? ORDER BY operation_id FOR UPDATE&#x27;,     &#91;room&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &#x27;SELECT operation_id,state FROM capture_operation WHERE room_id=? ORDER BY operation_id FOR UPDATE&#x27;,     &#91;room&#93;,   )</code> |

<!-- write-step: 7 bytes=1723:1921 -->
````javascript
export const roomCaptures = (connection, room) =>
  query(
    connection,
    'SELECT operation_id,state FROM capture_operation WHERE room_id=? ORDER BY operation_id FOR UPDATE',
    [room],
  );

````

## 第 8 步：clearRoomBody

插入位置：第 7 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`export const clearRoomBody = async (connection, room) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &quot;UPDATE room SET state=&#x27;RETIRED&#x27;,checkpoint_id=NULL WHERE id=?&quot;, &#91;room&#93;)</code>；<code>query(connection, &#x27;DELETE FROM update_log WHERE room_id=?&#x27;, &#91;room&#93;)</code>；<code>query(connection, &#x27;DELETE FROM checkpoint WHERE room_id=?&#x27;, &#91;room&#93;)</code>；<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;DELETED&#x27;,canonical_body=NULL,candidate_json=NULL,resource_refs=JSON_ARRAY() WHERE room_id=?&quot;,     &#91;room&#93;,   )</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &quot;UPDATE room SET state=&#x27;RETIRED&#x27;,checkpoint_id=NULL WHERE id=?&quot;, &#91;room&#93;)</code>；<code>query(connection, &#x27;DELETE FROM update_log WHERE room_id=?&#x27;, &#91;room&#93;)</code>；<code>query(connection, &#x27;DELETE FROM checkpoint WHERE room_id=?&#x27;, &#91;room&#93;)</code>；<code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;DELETED&#x27;,canonical_body=NULL,candidate_json=NULL,resource_refs=JSON_ARRAY() WHERE room_id=?&quot;,     &#91;room&#93;,   )</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>query(connection, &quot;UPDATE room SET state=&#x27;RETIRED&#x27;,checkpoint_id=NULL WHERE id=?&quot;, &#91;room&#93;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(connection, &#x27;DELETE FROM update_log WHERE room_id=?&#x27;, &#91;room&#93;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(connection, &#x27;DELETE FROM checkpoint WHERE room_id=?&#x27;, &#91;room&#93;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(     connection,     &quot;UPDATE capture_operation SET state=&#x27;DELETED&#x27;,canonical_body=NULL,candidate_json=NULL,resource_refs=JSON_ARRAY() WHERE room_id=?&quot;,     &#91;room&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(connection, &#x27;DELETE FROM durable_task WHERE kind=? AND business_key LIKE ?&#x27;, &#91;       kind,       room + &#x27;:%&#x27;,     &#93;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 8 bytes=1921:3046 -->
````javascript
export const clearRoomBody = async (connection, room) => {
  await query(connection, "UPDATE room SET state='RETIRED',checkpoint_id=NULL WHERE id=?", [room]);
  await query(connection, 'DELETE FROM update_log WHERE room_id=?', [room]);
  await query(connection, 'DELETE FROM checkpoint WHERE room_id=?', [room]);
  await query(
    connection,
    "UPDATE capture_operation SET state='DELETED',canonical_body=NULL,candidate_json=NULL,resource_refs=JSON_ARRAY() WHERE room_id=?",
    [room],
  );
  // 这两个artifact的键由room与epoch组成，包括已经退休的旧代次。
  for (const kind of ['ROOM_LINEAGE_CONTEXT', 'ROOM_MAIN_CONTEXT']) {
    await query(connection, 'DELETE FROM durable_task WHERE kind=? AND business_key LIKE ?', [
      kind,
      room + ':%',
    ]);
  }
};

// 各固定artifact只匹配已知的权威文档路径；MERGE_RESULT自身没有context，以其原attempt绑定关联。
// 不扫描/删除未知kind，也不把另一个文档的相同unit或用户当作清理对象。
// operation_id必须随制品读出：已准备转换只有匹配原操作的安装证明才能清理，
````

## 第 9 步：documentArtifacts

插入位置：第 8 步代码之后，紧接追加；完成稿约第 71 行。

否则完整的拉取主线/安装流程会被误判为尚未安装而永远不能回收。

本步声明：`export const documentArtifacts = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     `   SELECT id,kind,business_key,operation_id,payload,state FROM durable_task WHERE   cleanup_document=? AND kind IN (&#x27;MAIN_VIEW_FIXED&#x27;,&#x27;MERGE_ATTEMPT_CON …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     `   SELECT id,kind,business_key,operation_id,payload,state FROM durable_task WHERE   cleanup_document=? AND kind IN (&#x27;MAIN_VIEW_FIXED&#x27;,&#x27;MERGE_ATTEMPT_CON …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     `   SELECT id,kind,business_key,operation_id,payload,state FROM durable_task WHERE   cleanup_document=? AND kind IN (&#x27;MAIN_VIEW_FIXED&#x27;,&#x27;MERGE_ATTEMPT_CON …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 9 bytes=3046:3623 -->
````javascript
// 否则完整的拉取主线/安装流程会被误判为尚未安装而永远不能回收。
export const documentArtifacts = (connection, document) =>
  query(
    connection,
    `
  SELECT id,kind,business_key,operation_id,payload,state FROM durable_task WHERE
  cleanup_document=? AND kind IN ('MAIN_VIEW_FIXED','MERGE_ATTEMPT_CONTEXT','DRAFT_PREVIEW_FIXED','DRAFT_CONFIRMATION_FIXED',
    'DRAFT_TRANSITION_PREPARED','STRUCTURE_PREVIEW_FIXED','STRUCTURE_PREPARATION',
    'DRAFT_MAIN_GATE','STRUCTURE_BRANCH_GATE','RESOURCE_CONFIRM') ORDER BY id`,
    [document],
  );

````

## 第 10 步：removeArtifact

插入位置：第 9 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`export const removeArtifact = (connection, id) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;DELETE FROM durable_task WHERE id=?&#x27;, &#91;id&#93;)</code> |
| <code>id</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;DELETE FROM durable_task WHERE id=?&#x27;, &#91;id&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(connection, &#x27;DELETE FROM durable_task WHERE id=?&#x27;, &#91;id&#93;)</code> |

<!-- write-step: 10 bytes=3623:3739 -->
````javascript
export const removeArtifact = (connection, id) =>
  query(connection, 'DELETE FROM durable_task WHERE id=?', [id]);
````

## 第 11 步：removeMergeResult

插入位置：第 10 步代码之后，紧接追加；完成稿约第 85 行。

本步声明：`export const removeMergeResult = (connection, key) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;DELETE FROM durable_task WHERE kind=&#x27;MERGE_CANDIDATE_RESULT&#x27; AND business_key=?&quot;,     &#91;key&#93;,   )</code> |
| <code>key</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;DELETE FROM durable_task WHERE kind=&#x27;MERGE_CANDIDATE_RESULT&#x27; AND business_key=?&quot;,     &#91;key&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;DELETE FROM durable_task WHERE kind=&#x27;MERGE_CANDIDATE_RESULT&#x27; AND business_key=?&quot;,     &#91;key&#93;,   )</code> |

<!-- write-step: 11 bytes=3739:3922 -->
````javascript
export const removeMergeResult = (connection, key) =>
  query(
    connection,
    "DELETE FROM durable_task WHERE kind='MERGE_CANDIDATE_RESULT' AND business_key=?",
    [key],
  );

````

## 第 12 步：recordCleanup

插入位置：第 11 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`export const recordCleanup = (connection, context) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;       context.clean …</code>（长表达式，按本步完整代码继续） |
| <code>context</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>context.cleanup_kind</code>；<code>context.document_id</code>；<code>context.operation_id</code>；<code>JSON.stringify(context)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;       context.clean …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 12 bytes=3922:4436 -->
````javascript
export const recordCleanup = (connection, context) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [
      context.cleanup_kind === 'PURGE' ? 'PURGE_DOCUMENT_CLEANUP' : 'ACCOUNT_DOCUMENT_CLEANUP',
      context.cleanup_kind === 'PURGE'
        ? context.document_id + ':' + context.operation_id
        : context.document_id,
      context.operation_id,
      JSON.stringify(context),
    ],
  );

````

## 第 13 步：clearBarrierBody

插入位置：第 12 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`export const clearBarrierBody = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE main_barrier SET cut_vector=JSON_OBJECT() WHERE document_id=? AND state IN (&#x27;INSTALLED&#x27;,&#x27;ABORTED&#x27;)&quot;,     &#91;document&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE main_barrier SET cut_vector=JSON_OBJECT() WHERE document_id=? AND state IN (&#x27;INSTALLED&#x27;,&#x27;ABORTED&#x27;)&quot;,     &#91;document&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE main_barrier SET cut_vector=JSON_OBJECT() WHERE document_id=? AND state IN (&#x27;INSTALLED&#x27;,&#x27;ABORTED&#x27;)&quot;,     &#91;document&#93;,   )</code> |

<!-- write-step: 13 bytes=4436:4653 -->
````javascript
export const clearBarrierBody = (connection, document) =>
  query(
    connection,
    "UPDATE main_barrier SET cut_vector=JSON_OBJECT() WHERE document_id=? AND state IN ('INSTALLED','ABORTED')",
    [document],
  );
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/account_document_cleanup.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/06-cleanup-storage.md)。
