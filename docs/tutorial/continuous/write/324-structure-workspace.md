# 分步编写 `services/collaboration/src/repository/structure_workspace.mjs`

先按 [C05 · 结构拆并与每个在线编辑器保存握手](../lessons/collaboration/05-structure-online.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/324-structure-workspace.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存按room:epoch定位的结构支线门闩，并锁定文档所有活跃支线。

预览时还没有room行的支线也需要持久屏障，单锁room不足以阻止晚到连接。

写之前先按这条执行路线理解各段的连接：

1. branchGate按复合业务键FOR UPDATE
2. prepareBranchGate保存document/operation/generation及PREPARED
3. finishBranchGate仅在同operation/generation且原PREPARED时改INSTALLED/ABORTED
4. lockBranchRooms按id锁所有未退休BRANCH

本文件需要守住的边界：

- 禁止省略finish的原操作和generation条件，否则旧收尾会解开新屏障
- durable_task行state=SUCCEEDED表示门闩登记成功，payload.state才是门闩阶段

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/structure_workspace.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/structure_workspace.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/structure_workspace.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/structure_workspace.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:36 -->
````javascript
import { query } from './sql.mjs';

````

## 第 2 步：branchGate

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

来源 structure_preparations.mjs：原业务声明与函数体保留。

本步声明：`export const branchGate = (connection, room, epoch) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;STRUCTURE_BRANCH_GATE&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;room + &#x27;:&#x27; + epoch&#93;,   )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27;</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room + &#x27;:&#x27; + epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;SELECT payload FROM durable_task WHERE kind=&#x27;STRUCTURE_BRANCH_GATE&#x27; AND business_key=? FOR UPDATE&quot;,     &#91;room + &#x27;:&#x27; + epoch&#93;,   )</code> |

<!-- write-step: 2 bytes=36:329 -->
````javascript
// 来源 structure_preparations.mjs：原业务声明与函数体保留。

export const branchGate = (connection, room, epoch) =>
  query(
    connection,
    "SELECT payload FROM durable_task WHERE kind='STRUCTURE_BRANCH_GATE' AND business_key=? FOR UPDATE",
    [room + ':' + epoch],
  );
````

## 第 3 步：prepareBranchGate

插入位置：第 2 步代码之后，紧接追加；完成稿约第 11 行。

本步声明：`export const prepareBranchGate = (connection, branch, binding) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;STRUCTURE_BRANCH_GATE&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DU …</code>（长表达式，按本步完整代码继续） |
| <code>branch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>branch.room_id</code>；<code>branch.workspace_epoch</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.context</code>；<code>binding.barrier_generation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;STRUCTURE_BRANCH_GATE&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DU …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 3 bytes=329:980 -->
````javascript
export const prepareBranchGate = (connection, branch, binding) =>
  query(
    connection,
    "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('STRUCTURE_BRANCH_GATE',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE operation_id=VALUES(operation_id),payload=VALUES(payload)",
    [
      branch.room_id + ':' + branch.workspace_epoch,
      binding.context.operation_id,
      JSON.stringify({
        document_id: binding.context.target.document_id,
        operation_id: binding.context.operation_id,
        generation: binding.barrier_generation,
        state: 'PREPARED',
      }),
    ],
  );
````

## 第 4 步：finishBranchGate

插入位置：第 3 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`export const finishBranchGate = (connection, branch, binding, state) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET payload=JSON_SET(payload,&#x27;$.state&#x27;,?) WHERE kind=&#x27;STRUCTURE_BRANCH_GATE&#x27; AND business_key=? AND operation_id=? AND JSON_UNQUOTE( …</code>（长表达式，按本步完整代码继续） |
| <code>branch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>branch.room_id</code>；<code>branch.workspace_epoch</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.context</code>；<code>binding.barrier_generation</code> |
| <code>state</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET payload=JSON_SET(payload,&#x27;$.state&#x27;,?) WHERE kind=&#x27;STRUCTURE_BRANCH_GATE&#x27; AND business_key=? AND operation_id=? AND JSON_UNQUOTE( …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;UPDATE durable_task SET payload=JSON_SET(payload,&#x27;$.state&#x27;,?) WHERE kind=&#x27;STRUCTURE_BRANCH_GATE&#x27; AND business_key=? AND operation_id=? AND JSON_UNQUOTE( …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 4 bytes=980:1492 -->
````javascript
export const finishBranchGate = (connection, branch, binding, state) =>
  query(
    connection,
    "UPDATE durable_task SET payload=JSON_SET(payload,'$.state',?) WHERE kind='STRUCTURE_BRANCH_GATE' AND business_key=? AND operation_id=? AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.generation'))=? AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.state'))='PREPARED'",
    [
      state,
      branch.room_id + ':' + branch.workspace_epoch,
      binding.context.operation_id,
      binding.barrier_generation,
    ],
  );

````

## 第 5 步：lockBranchRooms

插入位置：第 4 步代码之后，紧接追加；完成稿约第 38 行。

来源 structure_previews.mjs：原业务声明与函数体保留。

本步声明：`export const lockBranchRooms = (connection, document) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT * FROM room WHERE document_id=? AND workspace_kind=&#x27;BRANCH&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id FOR UPDATE&quot;,     &#91;document&#93;,   )</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT * FROM room WHERE document_id=? AND workspace_kind=&#x27;BRANCH&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id FOR UPDATE&quot;,     &#91;document&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>query(     connection,     &quot;SELECT * FROM room WHERE document_id=? AND workspace_kind=&#x27;BRANCH&#x27; AND state&lt;&gt;&#x27;RETIRED&#x27; ORDER BY id FOR UPDATE&quot;,     &#91;document&#93;,   )</code> |

<!-- write-step: 5 bytes=1492:1786 -->
````javascript
// 来源 structure_previews.mjs：原业务声明与函数体保留。

export const lockBranchRooms = (connection, document) =>
  query(
    connection,
    "SELECT * FROM room WHERE document_id=? AND workspace_kind='BRANCH' AND state<>'RETIRED' ORDER BY id FOR UPDATE",
    [document],
  );
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/structure_workspace.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/05-structure-online.md)。
