# 分步编写 `services/collaboration/src/repository/personal_transfer_storage.mjs`

先按 [C06 · 固定候选清理、账号清理与转移计量](../lessons/collaboration/06-cleanup-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/316-personal-transfer-storage.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在锁定文档房间后按表聚合真实存储字节并保存转移计量回执。

把MiB正文传到Node再计算长度浪费内存；SQL OCTET_LENGTH能按真实存储内容计量。

写之前先按这条执行路线理解各段的连接：

1. storageReceipt锁原operation回执
2. lockStorageSource锁关闭来源
3. storageFacts按id锁全部room，分别GROUP BY room求checkpoint/update/capture字节
4. saveStorageReceipt把原request和result同事务写入

本文件需要守住的边界：

- SUM值可能是十进制字符串，应交BigInt处理
- 所有聚合与room锁必须用同一连接，不能在池中分开查询

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/personal_transfer_storage.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/personal_transfer_storage.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/personal_transfer_storage.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/personal_transfer_storage.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:36 -->
````javascript
import { query } from './sql.mjs';

````

## 第 2 步：kind

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`const kind = 'PERSONAL_TRANSFER_STORAGE_MEASUREMENT';`。

<!-- write-step: 2 bytes=36:90 -->
````javascript
const kind = 'PERSONAL_TRANSFER_STORAGE_MEASUREMENT';
````

## 第 3 步：storageReceipt

插入位置：第 2 步代码之后，紧接追加；完成稿约第 4 行。

本步声明：`export async function storageReceipt(c, operation) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>c</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     c,     &#x27;SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=? FOR UPDATE&#x27;,     &#91;kind, operation, operation&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     c,     &#x27;SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=? FOR UPDATE&#x27;,     &#91;kind, operation, operation&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>rows&#91;0&#93;</code> |

**跟着本段代码走一遍：**

- 等待 <code>query(     c,     &#x27;SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=? FOR UPDATE&#x27;,     &#91;kind, operation, operation&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return rows&#91;0&#93;;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=90:348 -->
````javascript
export async function storageReceipt(c, operation) {
  const [rows] = await query(
    c,
    'SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=? FOR UPDATE',
    [kind, operation, operation],
  );
  return rows[0];
}

````

## 第 4 步：lockStorageSource

插入位置：第 3 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`export async function lockStorageSource(c, scope) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>c</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     c,     &#x27;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;,     &#91;scope&#93;,   )</code> |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     c,     &#x27;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;,     &#91;scope&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>rows&#91;0&#93;</code> |

**跟着本段代码走一遍：**

- 等待 <code>query(     c,     &#x27;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;,     &#91;scope&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return rows&#91;0&#93;;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=348:552 -->
````javascript
export async function lockStorageSource(c, scope) {
  const [rows] = await query(
    c,
    'SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE',
    [scope],
  );
  return rows[0];
}

````

## 第 5 步：storageFacts

插入位置：第 4 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`export async function storageFacts(c, document) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>c</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     c,     &#x27;SELECT id,epoch,last_durable_seq,checkpoint_id,state FROM room WHERE document_id=? ORDER BY id FOR UPDATE&#x27;,     &#91;document&#93;,   )</code>；<code>query(     c,     `SELECT x.room_id,COUNT(*) AS records,COALESCE(SUM(OCTET_LENGTH(x.yjs_state)),0) AS bytes     FROM checkpoint x JOIN room r ON r.id=x.room_id WHERE r.document_id= …</code>（长表达式，按本步完整代码继续）；<code>query(     c,     `SELECT x.room_id,COUNT(*) AS records,MAX(x.seq) AS sequence,COALESCE(SUM(OCTET_LENGTH(x.payload)),0) AS bytes     FROM update_log x JOIN room r ON r.id=x.room_id …</code>（长表达式，按本步完整代码继续）；<code>query(     c,     `SELECT x.room_id,COUNT(*) AS records,COALESCE(SUM(COALESCE(OCTET_LENGTH(x.canonical_body),0)+COALESCE(OCTET_LENGTH(CAST(x.candidate_json AS CHAR)),0)),0) AS byte …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     c,     &#x27;SELECT id,epoch,last_durable_seq,checkpoint_id,state FROM room WHERE document_id=? ORDER BY id FOR UPDATE&#x27;,     &#91;document&#93;,   )</code>；<code>query(     c,     `SELECT x.room_id,COUNT(*) AS records,COALESCE(SUM(OCTET_LENGTH(x.yjs_state)),0) AS bytes     FROM checkpoint x JOIN room r ON r.id=x.room_id WHERE r.document_id= …</code>（长表达式，按本步完整代码继续）；<code>query(     c,     `SELECT x.room_id,COUNT(*) AS records,MAX(x.seq) AS sequence,COALESCE(SUM(OCTET_LENGTH(x.payload)),0) AS bytes     FROM update_log x JOIN room r ON r.id=x.room_id …</code>（长表达式，按本步完整代码继续）；<code>query(     c,     `SELECT x.room_id,COUNT(*) AS records,COALESCE(SUM(COALESCE(OCTET_LENGTH(x.canonical_body),0)+COALESCE(OCTET_LENGTH(CAST(x.candidate_json AS CHAR)),0)),0) AS byte …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>{ rooms, checkpoints, updates, captures }</code> |

**跟着本段代码走一遍：**

- 等待 <code>query(     c,     &#x27;SELECT id,epoch,last_durable_seq,checkpoint_id,state FROM room WHERE document_id=? ORDER BY id FOR UPDATE&#x27;,     &#91;document&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(     c,     `SELECT x.room_id,COUNT(*) AS records,COALESCE(SUM(OCTET_LENGTH(x.yjs_state)),0) AS bytes     FROM checkpoint x JOIN room r ON r.id=x.room_id WHERE r.document_id= …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(     c,     `SELECT x.room_id,COUNT(*) AS records,MAX(x.seq) AS sequence,COALESCE(SUM(OCTET_LENGTH(x.payload)),0) AS bytes     FROM update_log x JOIN room r ON r.id=x.room_id …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>query(     c,     `SELECT x.room_id,COUNT(*) AS records,COALESCE(SUM(COALESCE(OCTET_LENGTH(x.canonical_body),0)+COALESCE(OCTET_LENGTH(CAST(x.candidate_json AS CHAR)),0)),0) AS byte …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return { rooms, checkpoints, updates, captures };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=552:1964 -->
````javascript
export async function storageFacts(c, document) {
  // 来源ACK之后仍锁MAIN入口和全部room，和真实保存/捕获/检查点使用同一顺序。
  // 聚合按表批量查询，避免每个room单独往返，也不把正文搬到Doc或内存中再数长度。
  const [rooms] = await query(
    c,
    'SELECT id,epoch,last_durable_seq,checkpoint_id,state FROM room WHERE document_id=? ORDER BY id FOR UPDATE',
    [document],
  );
  const [checkpoints] = await query(
    c,
    `SELECT x.room_id,COUNT(*) AS records,COALESCE(SUM(OCTET_LENGTH(x.yjs_state)),0) AS bytes
    FROM checkpoint x JOIN room r ON r.id=x.room_id WHERE r.document_id=? GROUP BY x.room_id ORDER BY x.room_id`,
    [document],
  );
  const [updates] = await query(
    c,
    `SELECT x.room_id,COUNT(*) AS records,MAX(x.seq) AS sequence,COALESCE(SUM(OCTET_LENGTH(x.payload)),0) AS bytes
    FROM update_log x JOIN room r ON r.id=x.room_id WHERE r.document_id=? GROUP BY x.room_id ORDER BY x.room_id`,
    [document],
  );
  const [captures] = await query(
    c,
    `SELECT x.room_id,COUNT(*) AS records,COALESCE(SUM(COALESCE(OCTET_LENGTH(x.canonical_body),0)+COALESCE(OCTET_LENGTH(CAST(x.candidate_json AS CHAR)),0)),0) AS bytes
    FROM capture_operation x JOIN room r ON r.id=x.room_id WHERE r.document_id=? GROUP BY x.room_id ORDER BY x.room_id`,
    [document],
  );
  return { rooms, checkpoints, updates, captures };
}

````

## 第 6 步：saveStorageReceipt

插入位置：第 5 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`export async function saveStorageReceipt(c, request, result) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>c</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     c,     `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)     VALUES(?,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))`,     &#91;kind, request.operation_i …</code>（长表达式，按本步完整代码继续） |
| <code>request</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>request.operation_id</code>；<code>JSON.stringify({ request, result })</code> |
| <code>result</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify({ request, result })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>query(     c,     `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)     VALUES(?,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))`,     &#91;kind, request.operation_i …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 6 bytes=1964:2285 -->
````javascript
export async function saveStorageReceipt(c, request, result) {
  await query(
    c,
    `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
    VALUES(?,?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))`,
    [kind, request.operation_id, request.operation_id, JSON.stringify({ request, result })],
  );
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/personal_transfer_storage.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/06-cleanup-storage.md)。
