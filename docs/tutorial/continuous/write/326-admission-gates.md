# 分步编写 `services/collaboration/src/application/admission_gates.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/326-admission-gates.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

统一每次准入和每帧读写的来源锁与文档/草稿入口屏障。

晚到票据即使对应room还不存在，也必须被文档级结构/主稿屏障拦住。

写之前先按这条执行路线理解各段的连接：

1. lockSources按ASCII排序来源，拒绝重复、旧epoch或同epoch BLOCKED，只有更高epoch重开
2. requireMainOpen先锁文档，再查DRAFT_MAIN_GATE与main_barrier
3. requireBranchOpen按文档→支线入口检查PREPARED/INSTALLED/ABORTED

本文件需要守住的边界：

- PREPARED表示需要等待，不可降级为OPEN
- 结构INSTALLED的旧支线返回WORKSPACE_CHANGED，不能继续沿旧epoch保存

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/admission_gates.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/admission_gates.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/admission_gates.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/admission_gates.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:480 -->
````javascript
import { bad } from '../domain/shared_protocol.mjs';
import { branchGate } from '../repository/structure_workspace.mjs';
import { deny } from '../domain/shared_protocol.mjs';
import * as drafts from '../repository/draft_transitions.mjs';
import { json } from '../domain/shared_protocol.mjs';
import * as repository from '../repository/main_workspace.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import * as sourceGates from '../repository/source_gates.mjs';

````

## 第 2 步：requireMainOpen

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

来源 main_gate.mjs：原业务声明与函数体保留。

本步声明：`export async function requireMainOpen(connection, document) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>repository.lockDocument(connection, document)</code>；<code>requireDraftMainOpen(connection, document)</code>；<code>repository.lockBarrier(connection, document)</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>repository.lockDocument(connection, document)</code>；<code>requireDraftMainOpen(connection, document)</code>；<code>repository.lockBarrier(connection, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>repository.lockDocument(connection, document)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>requireDraftMainOpen(connection, document)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>repository.lockBarrier(connection, document)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>barrier &amp;&amp; !&#91;&#x27;INSTALLED&#x27;, &#x27;ABORTED&#x27;&#93;.includes(barrier.state)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_BARRIER_PENDING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;MAIN_BARRIER_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 2 bytes=480:1027 -->
````javascript
// 来源 main_gate.mjs：原业务声明与函数体保留。

export async function requireMainOpen(connection, document) {
  // 先锁文档级入口，房间尚未存在也不能绕过屏障。调用方仍在原来源/日志事务内。
  await repository.lockDocument(connection, document);
  await requireDraftMainOpen(connection, document);
  const [[barrier]] = await repository.lockBarrier(connection, document);
  if (barrier && !['INSTALLED', 'ABORTED'].includes(barrier.state))
    throw new RPCFailure(14, 'MAIN_BARRIER_PENDING');
}

````

## 第 3 步：requireBranchOpen

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`export async function requireBranchOpen(connection, document, room, epoch) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>repository.lockDocument(connection, document)</code>；<code>branchGate(connection, room, epoch)</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>repository.lockDocument(connection, document)</code>；<code>fixed.document_id !== document</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>branchGate(connection, room, epoch)</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>branchGate(connection, room, epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>undefined</code> |

**跟着本段代码走一遍：**

- 等待 <code>repository.lockDocument(connection, document)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>branchGate(connection, room, epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!gate</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。
- 检查 <code>fixed.document_id !== document &#124;&#124; !&#91;&#x27;PREPARED&#x27;, &#x27;INSTALLED&#x27;, &#x27;ABORTED&#x27;&#93;.includes(fixed.state)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_GATE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_GATE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>fixed.state === &#x27;PREPARED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_GATE_PENDING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_GATE_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>fixed.state === &#x27;INSTALLED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(9, &#x27;WORKSPACE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(9, &#x27;WORKSPACE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 3 bytes=1027:1817 -->
````javascript
export async function requireBranchOpen(connection, document, room, epoch) {
  // 与结构prepare共用来源→文档→支线入口→房间顺序；尚无room行的晚到票据也不能绕过准备。
  await repository.lockDocument(connection, document);
  const [[gate]] = await branchGate(connection, room, epoch);
  if (!gate) return;
  const fixed = json(gate.payload);
  if (fixed.document_id !== document || !['PREPARED', 'INSTALLED', 'ABORTED'].includes(fixed.state))
    throw new RPCFailure(14, 'STRUCTURE_BRANCH_GATE_UNKNOWN');
  if (fixed.state === 'PREPARED') throw new RPCFailure(14, 'STRUCTURE_BRANCH_GATE_PENDING');
  if (fixed.state === 'INSTALLED') throw new RPCFailure(9, 'WORKSPACE_CHANGED');
}

// 来源 draft_main_gate.mjs：原业务声明与函数体保留。

````

## 第 4 步：requireDraftMainOpen

插入位置：第 3 步代码之后，紧接追加；完成稿约第 35 行。

调用方已经取得统一Doc互斥。此持久入口覆盖全篇MAIN，既有和晚到房间都不能越过。

本步声明：`export async function requireDraftMainOpen(connection, document) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>drafts.artifact(connection, &#x27;DRAFT_MAIN_GATE&#x27;, document)</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>drafts.artifact(connection, &#x27;DRAFT_MAIN_GATE&#x27;, document)</code>；<code>gate.document_id !== document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>undefined</code> |

**跟着本段代码走一遍：**

- 等待 <code>drafts.artifact(connection, &#x27;DRAFT_MAIN_GATE&#x27;, document)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!row</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。
- 检查 <code>gate.document_id !== document &#124;&#124; !&#91;&#x27;PREPARED&#x27;, &#x27;INSTALLED&#x27;, &#x27;ABORTED&#x27;&#93;.includes(gate.state)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_MAIN_GATE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;DRAFT_MAIN_GATE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>gate.state === &#x27;PREPARED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_MAIN_GATE_PENDING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;DRAFT_MAIN_GATE_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 4 bytes=1817:2449 -->
````javascript
// 调用方已经取得统一Doc互斥。此持久入口覆盖全篇MAIN，既有和晚到房间都不能越过。
export async function requireDraftMainOpen(connection, document) {
  const [[row]] = await drafts.artifact(connection, 'DRAFT_MAIN_GATE', document);
  if (!row) return;
  const gate = json(row.payload);
  if (gate.document_id !== document || !['PREPARED', 'INSTALLED', 'ABORTED'].includes(gate.state))
    throw new RPCFailure(14, 'DRAFT_MAIN_GATE_UNKNOWN');
  if (gate.state === 'PREPARED') throw new RPCFailure(14, 'DRAFT_MAIN_GATE_PENDING');
}

// 来源 source_access.mjs：原业务声明与函数体保留。

````

## 第 5 步：lockSources

插入位置：第 4 步代码之后，紧接追加；完成稿约第 47 行。

来源统一按ASCII顺序取得排他锁，与关闭事务共享线性化点；只有更高已核验代次可重开。

本步声明：`export async function lockSources(c, sources) `。

**这一段为什么这样写：** 把来源关闭与读写准入放在同一事务行锁边界上。

1. 检查来源数量和去重
2. 批量ensure并按主键锁定
3. 比较BigInt epoch，拒绝旧来源或已关同代
4. 仅更高已核验来源可重开


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>c</code> | <code>JavaScript 参数</code> | 此次业务事务连接<br>在此声明中的实际使用：<code>sourceGates.ensureSourceGates(c, list)</code>；<code>sourceGates.lockSourceGates(c, list)</code>；<code>sourceGates.reopenSourceGate(c, s.epoch, s.scope_key)</code> |
| <code>sources</code> | <code>JavaScript 参数</code> | Doc核验得到的来源列表，不能使用客户端自报权限<br>在此声明中的实际使用：<code>Array.isArray(sources)</code>；<code>sources.length</code>；<code>&#91;...sources&#93;.sort</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 检查 <code>!Array.isArray(sources) &#124;&#124; !sources.length &#124;&#124; sources.length &gt; 512</code>。条件成立时执行 <code>throw deny(&#x27;SOURCES_REQUIRED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw deny(&#x27;SOURCES_REQUIRED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>new Set(list.map((x) =&gt; x.scope_key)).size !== list.length</code>。条件成立时执行 <code>throw bad(&#x27;SOURCE_DUPLICATE&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;SOURCE_DUPLICATE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>sourceGates.ensureSourceGates(c, list)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>sourceGates.lockSourceGates(c, list)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!gate</code>。条件成立时执行 <code>throw deny(&#x27;SOURCE_CLOSED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw deny(&#x27;SOURCE_CLOSED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>BigInt(gate.max_epoch) &gt; BigInt(s.epoch) &#124;&#124;       (String(gate.max_epoch) === s.epoch &amp;&amp; gate.state === &#x27;BLOCKED&#x27;)</code>。条件成立时执行 <code>throw deny(&#x27;SOURCE_CLOSED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw deny(&#x27;SOURCE_CLOSED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>BigInt(gate.max_epoch) &lt; BigInt(s.epoch)</code>。
- 等待 <code>sourceGates.reopenSourceGate(c, s.epoch, s.scope_key)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 5 bytes=2449:3530 -->
````javascript
// 来源统一按ASCII顺序取得排他锁，与关闭事务共享线性化点；只有更高已核验代次可重开。
export async function lockSources(c, sources) {
  if (!Array.isArray(sources) || !sources.length || sources.length > 512)
    throw deny('SOURCES_REQUIRED');
  const list = [...sources].sort((a, b) =>
    Buffer.compare(Buffer.from(a.scope_key), Buffer.from(b.scope_key)),
  );
  if (new Set(list.map((x) => x.scope_key)).size !== list.length) throw bad('SOURCE_DUPLICATE');
  await sourceGates.ensureSourceGates(c, list);
  const [rows] = await sourceGates.lockSourceGates(c, list);
  const gates = new Map(rows.map((gate) => [gate.scope_key, gate]));
  for (const s of list) {
    const gate = gates.get(s.scope_key);
    if (!gate) throw deny('SOURCE_CLOSED');
    if (
      BigInt(gate.max_epoch) > BigInt(s.epoch) ||
      (String(gate.max_epoch) === s.epoch && gate.state === 'BLOCKED')
    )
      throw deny('SOURCE_CLOSED');
    if (BigInt(gate.max_epoch) < BigInt(s.epoch))
      await sourceGates.reopenSourceGate(c, s.epoch, s.scope_key);
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/admission_gates.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
