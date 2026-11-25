# 分步编写 `services/collaboration/src/application/submission_captures.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/347-submission-captures.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

从有效FLUSH证据冻结送审cut和lineage，再生成固定正文并绑定Doc结果。

送审期间仍可能编辑；首次cut和基线一旦确定，后续重试不能改为更晚正文。

写之前先按这条执行路线理解各段的连接：

1. capture先定位房间，在共享队列内短事务核来源、FIXED回执、actor/epoch及BRANCH身份
2. FLUSH证明依赖已落库；首次事务固定当时room.last_durable_seq与lineage到CUT_FIXED
3. complete只恢复原cut，构造规范body/hash/resources/provenance并存CAPTURED
4. finalize重查GetOperationResult真实SUCCEEDED才BOUND
5. committedCut按Go成功结果返回既成送审事实，旧用户撤权不会抹去历史

本文件需要守住的边界：

- 送审cut可覆盖FLUSH之后、首次冻结之前已持久更新；不能误写为永远等于flush.covered_cut
- cleaned候选不得重新生成，CUT_FIXED必须保留原lineage

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/submission_captures.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/submission_captures.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/submission_captures.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/submission_captures.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:755 -->
````javascript
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  canonical,
  fromPM,
  normalizeContent,
  resourceIds,
} from '../../../../packages/content/index.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';
import * as rooms from '../repository/room_access.mjs';
import * as flushes from '../repository/room_persistence.mjs';
import {
  captureBinding,
  sameBinding,
  matchCaptureQuery,
  captureProvenance,
  storeContent,
  wireContent,
} from '../domain/capture_protocol.mjs';
import { json, deny, validID, bad } from '../domain/shared_protocol.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { lockSources } from './admission_gates.mjs';

````

## 第 2 步：SubmissionCaptures

插入位置：第 1 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`export class SubmissionCaptures `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>queue</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.queue = queue</code> |
| <code>snapshots</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>peers</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |

**本段进入 SubmissionCaptures.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>queue</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.queue = queue</code> |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 SubmissionCaptures.capture：**

**这一段为什么这样写：** 在不阻止后续长期编辑的前提下固定一次送审。

1. 从既有操作或回执确定room
2. 同房间队列内固定cut/lineage
3. 调用complete从固定位置重放


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | context、flush_receipt_id、guard_generation、lineage_ref<br>在此声明中的实际使用：<code>captureBinding(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.queue.run(roomID, async () =&gt; {       // 与UPDATE/FLUSH共用队列。首个短事务先固定cut，第二事务只能从原cut生成正文。       await this.transactions.run(async (connection) =&gt; {         await lockSources(con …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 等待 <code>this.transactions.run(async (connection) =&gt; {       const &#91;&#91;prior&#93;&#93; = await captures.findCapture(connection, binding.context.operation_id);       if (prior) {         sameBinding(j …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>captures.findCapture(connection, binding.context.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>prior</code>。
- 在本段的内部函数内：此处通过 <code>return prior.room_id;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>flushes.findFlushReceipt(connection, binding.flush_receipt_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!receipt</code>。条件成立时执行 <code>throw deny(&#x27;FLUSH_BINDING_INVALID&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;FLUSH_BINDING_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return receipt.room_id;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.queue.run(roomID, async () =&gt; {       // 与UPDATE/FLUSH共用队列。首个短事务先固定cut，第二事务只能从原cut生成正文。       await this.transactions.run(async (connection) =&gt; {         await lockSour …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>this.transactions.run(async (connection) =&gt; {         await lockSources(connection, binding.context.sources);         const &#91;&#91;prior&#93;&#93; = await captures.lockOperation(connection, bin …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>lockSources(connection, binding.context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>captures.lockOperation(connection, binding.context.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>prior</code>。
- 在本段的内部函数内：此处通过 <code>return prior.room_id;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>flushes.findFlushReceipt(connection, binding.flush_receipt_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!receipt &#124;&#124;           receipt.state !== &#x27;FIXED&#x27; &#124;&#124;           receipt.actor_id !== binding.context.actor_id &#124;&#124;           receipt.epoch !== target.workspace_epoch</code>。条件成立时执行 <code>throw deny(&#x27;FLUSH_BINDING_INVALID&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;FLUSH_BINDING_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>rooms.lockRoom(connection, receipt.room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!room &#124;&#124;           room.state !== &#x27;OPEN&#x27; &#124;&#124;           room.workspace_kind !== &#x27;BRANCH&#x27; &#124;&#124;           room.epoch !== target.workspace_epoch &#124;&#124;           room.document_id !== target.d …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw deny(&#x27;WORKSPACE_CHANGED&#x27;);</code>。转入外层异常处理路径。

<!-- write-step: 2 bytes=755:2800 -->
````javascript
export class SubmissionCaptures {
  constructor(transactions, queue, snapshots, peers) {
    this.transactions = transactions;
    this.queue = queue;
    this.snapshots = snapshots;
    this.peers = peers;
  }

  async capture(input) {
    const binding = captureBinding(input),
      target = binding.context.target;
    const roomID = await this.transactions.run(async (connection) => {
      const [[prior]] = await captures.findCapture(connection, binding.context.operation_id);
      if (prior) {
        sameBinding(json(prior.candidate_json).binding, binding);
        return prior.room_id;
      }
      const [[receipt]] = await flushes.findFlushReceipt(connection, binding.flush_receipt_id);
      if (!receipt) throw deny('FLUSH_BINDING_INVALID');
      return receipt.room_id;
    });
    return this.queue.run(roomID, async () => {
      // 与UPDATE/FLUSH共用队列。首个短事务先固定cut，第二事务只能从原cut生成正文。
      await this.transactions.run(async (connection) => {
        await lockSources(connection, binding.context.sources);
        const [[prior]] = await captures.lockOperation(connection, binding.context.operation_id);
        if (prior) {
          sameBinding(json(prior.candidate_json).binding, binding);
          return prior.room_id;
        }
        const [[receipt]] = await flushes.findFlushReceipt(connection, binding.flush_receipt_id);
        if (
          !receipt ||
          receipt.state !== 'FIXED' ||
          receipt.actor_id !== binding.context.actor_id ||
          receipt.epoch !== target.workspace_epoch
        )
          throw deny('FLUSH_BINDING_INVALID');
        const [[room]] = await rooms.lockRoom(connection, receipt.room_id);
        if (
          !room ||
          room.state !== 'OPEN' ||
          room.workspace_kind !== 'BRANCH' ||
          room.epoch !== target.workspace_epoch ||
          room.document_id !== target.document_id ||
          room.unit_id !== target.unit_id ||
          room.workspace_id !== target.branch_id
        )
````

## 第 3 步：SubmissionCaptures（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 72 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 SubmissionCaptures.complete：**

**这一段为什么这样写：** 续接CUT_FIXED，不重新采样最新草稿。

1. 核来源及同号binding
2. 已有CAPTURED/BOUND重放
3. 恢复row.cut_ref并构造固定内容
4. 事务内保存后释放Y.Doc


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 首次captureBinding产生的固定输入<br>在此声明中的实际使用：<code>binding.context</code>；<code>sameBinding(candidate.binding, binding)</code>；<code>captures.saveCandidate(           connection,           row.operation_id,           { binding, content: storeContent(content) },           bytes,           digest,           resources,         )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.transactions.run(async (connection) =&gt; {       await lockSources(connection, binding.context.sources);       const &#91;&#91;row&#93;&#93; = await captures.lockOperation(connection, binding.c …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的内部函数内：执行 <code>throw deny(&#x27;WORKSPACE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>covered.room_id !== room.id &#124;&#124;           covered.workspace_epoch !== room.epoch &#124;&#124;           BigInt(covered.durable_seq) &gt; BigInt(room.last_durable_seq)</code>。条件成立时执行 <code>throw deny(&#x27;FLUSH_CUT_INVALID&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;FLUSH_CUT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>captures.lockRoomLineage(connection, room.id, room.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!stored &#124;&#124; json(stored.payload).lineage_id !== binding.lineage_ref</code>。条件成立时执行 <code>throw deny(&#x27;LINEAGE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;LINEAGE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>captures.insertCut(connection, binding, room, cut, json(stored.payload))</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return room.id;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：此处通过 <code>return this.complete(binding);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.transactions.run(async (connection) =&gt; {       await lockSources(connection, binding.context.sources);       const &#91;&#91;row&#93;&#93; = await captures.lockOperation(connection, bi …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>lockSources(connection, binding.context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>captures.lockOperation(connection, binding.context.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>candidate.cleaned</code>。条件成立时执行 <code>throw new RPCFailure(9, &#x27;CAPTURE_CANDIDATE_CLEANED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(9, &#x27;CAPTURE_CANDIDATE_CLEANED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>&#91;&#x27;CAPTURED&#x27;, &#x27;BOUND&#x27;&#93;.includes(row.state)</code>。条件成立时执行 <code>return { state: &#x27;FIXED&#x27;, content: wireContent(candidate.content) };</code>。当前函数在这里结束。
- 在本段的内部函数内：检查 <code>row.state !== &#x27;CUT_FIXED&#x27;</code>。条件成立时执行 <code>return { state: &#x27;REJECTED&#x27;, business_code: &#x27;CAPTURE_CLOSED&#x27; };</code>。当前函数在这里结束。
- 在本段的内部函数内：等待 <code>rooms.lockRoom(connection, row.room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!room &#124;&#124; room.epoch !== row.epoch &#124;&#124; room.state !== &#x27;OPEN&#x27;</code>。条件成立时执行 <code>throw deny(&#x27;WORKSPACE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;WORKSPACE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>!lineage</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_LINEAGE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_LINEAGE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>this.snapshots.restore(connection, room, cut.durable_seq)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=2800:5063 -->
````javascript
          throw deny('WORKSPACE_CHANGED');
        const covered = json(receipt.covered_cut);
        if (
          covered.room_id !== room.id ||
          covered.workspace_epoch !== room.epoch ||
          BigInt(covered.durable_seq) > BigInt(room.last_durable_seq)
        )
          throw deny('FLUSH_CUT_INVALID');
        const [[stored]] = await captures.lockRoomLineage(connection, room.id, room.epoch);
        if (!stored || json(stored.payload).lineage_id !== binding.lineage_ref)
          throw deny('LINEAGE_CHANGED');
        const cut = {
          room_id: room.id,
          workspace_epoch: room.epoch,
          durable_seq: String(room.last_durable_seq),
        };
        await captures.insertCut(connection, binding, room, cut, json(stored.payload));
        return room.id;
      });
      return this.complete(binding);
    });
  }

  async complete(binding) {
    return this.transactions.run(async (connection) => {
      await lockSources(connection, binding.context.sources);
      const [[row]] = await captures.lockOperation(connection, binding.context.operation_id);
      const candidate = json(row.candidate_json);
      sameBinding(candidate.binding, binding);
      if (candidate.cleaned) throw new RPCFailure(9, 'CAPTURE_CANDIDATE_CLEANED');
      if (['CAPTURED', 'BOUND'].includes(row.state))
        return { state: 'FIXED', content: wireContent(candidate.content) };
      if (row.state !== 'CUT_FIXED') return { state: 'REJECTED', business_code: 'CAPTURE_CLOSED' };
      const [[room]] = await rooms.lockRoom(connection, row.room_id);
      if (!room || room.epoch !== row.epoch || room.state !== 'OPEN')
        throw deny('WORKSPACE_CHANGED');
      // 切点与来源在首次事务一并固定；同epoch后来接纳主线，也不能改变这个候选的base/provenance。
      const lineage = candidate.lineage,
        cut = json(row.cut_ref);
      if (!lineage) throw new RPCFailure(14, 'CAPTURE_LINEAGE_UNKNOWN');
      const doc = await this.snapshots.restore(connection, room, cut.durable_seq);
      try {
        const body = normalizeContent(fromPM(yDocToProsemirrorJSON(doc))),
          bytes = Buffer.from(canonical(body)),
          digest = hash(bytes),
          resources = resourceIds(body);
````

## 第 4 步：SubmissionCaptures（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 118 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 SubmissionCaptures.get：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>query</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query.operation_id</code>；<code>query.request_fingerprint</code>；<code>query.target</code>；<code>matchCaptureQuery(row, binding, query)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>{ state: &#x27;PROCESSING&#x27; }</code>；<code>this.queue.run(result.row.room_id, () =&gt; this.complete(result.binding))</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>captures.saveCandidate(           connection,           row.operation_id,           { binding, content: storeContent(content) },           bytes,           digest,           resources,         )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return { state: &#x27;FIXED&#x27;, content };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!validID(query.operation_id) &#124;&#124;       Buffer.from(query.request_fingerprint ?? &#91;&#93;).length !== 32 &#124;&#124;       !query.target</code>。条件成立时执行 <code>throw bad(&#x27;CAPTURE_QUERY_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;CAPTURE_QUERY_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.transactions.run(async (connection) =&gt; {       const &#91;&#91;row&#93;&#93; = await captures.findCapture(connection, query.operation_id);       if (!row) return null;       const binding = j …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>captures.findCapture(connection, query.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!row</code>。条件成立时执行 <code>return null;</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=5063:6565 -->
````javascript
        const content = {
          ...binding.context.target,
          capture_id: row.operation_id,
          operation_id: row.operation_id,
          cut_json_utf8: Buffer.from(canonical(cut)),
          lineage_ref: row.lineage_ref,
          body_json_utf8: bytes,
          schema_version: 'content-v1',
          canonicalization_version: 'canonical-v1',
          hash_algorithm: 'SHA-256',
          body_hash: digest,
          byte_size: String(bytes.length),
          resource_ids: resources,
          provenance_json_utf8: Buffer.from(
            canonical(captureProvenance(body, lineage, row.operation_id)),
          ),
          provenance_schema: 'provenance-v1',
        };
        await captures.saveCandidate(
          connection,
          row.operation_id,
          { binding, content: storeContent(content) },
          bytes,
          digest,
          resources,
        );
        return { state: 'FIXED', content };
      } finally {
        doc.destroy();
      }
    });
  }

  async get(query) {
    if (
      !validID(query.operation_id) ||
      Buffer.from(query.request_fingerprint ?? []).length !== 32 ||
      !query.target
    )
      throw bad('CAPTURE_QUERY_INVALID');
    const result = await this.transactions.run(async (connection) => {
      const [[row]] = await captures.findCapture(connection, query.operation_id);
      if (!row) return null;
      const binding = json(row.candidate_json).binding;
      matchCaptureQuery(row, binding, query);
````

## 第 5 步：SubmissionCaptures（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 163 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 SubmissionCaptures.finalize：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.authoritative_result</code>；<code>input.context</code>；<code>input.capture_id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>result</code> |

**本段进入 SubmissionCaptures.committedCut：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>query</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query.operation_id</code>；<code>query.target</code>；<code>query.request_fingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.transactions.run(async (connection) =&gt; {       const &#91;&#91;row&#93;&#93; = await captures.findCapture(connection, query.operation_id);       if (!row &#124;&#124; !&#91;&#x27;CAPTURED&#x27;, &#x27;BOUND&#x27;&#93;.includes(ro …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>lockSources(connection, binding.context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return { row, binding };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!result</code>。条件成立时执行 <code>return { state: &#x27;PROCESSING&#x27; };</code>。当前函数在这里结束。
- 此处通过 <code>return this.queue.run(result.row.room_id, () =&gt; this.complete(result.binding));</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!input.context &#124;&#124; input.capture_id !== input.context.operation_id &#124;&#124; !supplied</code>。条件成立时执行 <code>throw bad(&#x27;FINALIZE_CAPTURE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;FINALIZE_CAPTURE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.peers.rpc(&#x27;GetOperationResult&#x27;, {       operation_id: input.context.operation_id,       target: input.context.target,       request_fingerprint: input.context.request_fingerprint,     })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>result.state !== &#x27;SUCCEEDED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_RESULT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_RESULT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>supplied.state !== result.state &#124;&#124;       supplied.operation_id !== result.operation_id &#124;&#124;       Buffer.from(supplied.result_digest).toString(&#x27;hex&#x27;) !==         Buffer.from(result.r …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;CAPTURE_RESULT_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;CAPTURE_RESULT_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.transactions.run(async (connection) =&gt; {       const &#91;&#91;row&#93;&#93; = await captures.lockOperation(connection, input.capture_id);       if (!row) throw deny(&#x27;CAPTURE_NOT_FOUND&#x27;);     …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>captures.lockOperation(connection, input.capture_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!row</code>。条件成立时执行 <code>throw deny(&#x27;CAPTURE_NOT_FOUND&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;CAPTURE_NOT_FOUND&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>row.go_result_ref &amp;&amp; row.go_result_ref !== reference</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;CAPTURE_RESULT_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;CAPTURE_RESULT_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>!&#91;&#x27;CAPTURED&#x27;, &#x27;BOUND&#x27;&#93;.includes(row.state)</code>。条件成立时执行 <code>throw deny(&#x27;CAPTURE_NOT_FIXED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;CAPTURE_NOT_FIXED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>captures.bindResult(connection, row.operation_id, reference)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!validID(query.operation_id) &#124;&#124; !query.target &#124;&#124; query.target.purpose !== &#x27;SUBMISSION&#x27;</code>。条件成立时执行 <code>throw bad(&#x27;CAPTURE_QUERY_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;CAPTURE_QUERY_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 5 bytes=6565:8815 -->
````javascript
      await lockSources(connection, binding.context.sources);
      return { row, binding };
    });
    if (!result) return { state: 'PROCESSING' };
    return this.queue.run(result.row.room_id, () => this.complete(result.binding));
  }

  async finalize(input) {
    const supplied = input.authoritative_result;
    if (!input.context || input.capture_id !== input.context.operation_id || !supplied)
      throw bad('FINALIZE_CAPTURE_INVALID');
    // 系统收尾查真实Go终态；不能把调用参数或网络超时当作可绑定/可删除许可。
    const result = await this.peers.rpc('GetOperationResult', {
      operation_id: input.context.operation_id,
      target: input.context.target,
      request_fingerprint: input.context.request_fingerprint,
    });
    if (result.state !== 'SUCCEEDED') throw new RPCFailure(14, 'CAPTURE_RESULT_UNKNOWN');
    if (
      supplied.state !== result.state ||
      supplied.operation_id !== result.operation_id ||
      Buffer.from(supplied.result_digest).toString('hex') !==
        Buffer.from(result.result_digest).toString('hex') ||
      Buffer.from(supplied.fixed_result_json_utf8).toString('base64') !==
        Buffer.from(result.fixed_result_json_utf8).toString('base64')
    )
      throw new RPCFailure(6, 'CAPTURE_RESULT_CONFLICT');
    await this.transactions.run(async (connection) => {
      const [[row]] = await captures.lockOperation(connection, input.capture_id);
      if (!row) throw deny('CAPTURE_NOT_FOUND');
      matchCaptureQuery(row, json(row.candidate_json).binding, { ...input.context });
      const reference =
        result.operation_id + ':' + Buffer.from(result.result_digest).toString('hex');
      if (row.go_result_ref && row.go_result_ref !== reference)
        throw new RPCFailure(6, 'CAPTURE_RESULT_CONFLICT');
      if (!['CAPTURED', 'BOUND'].includes(row.state)) throw deny('CAPTURE_NOT_FIXED');
      await captures.bindResult(connection, row.operation_id, reference);
    });
    return result;
  }

  async committedCut(query) {
    if (!validID(query.operation_id) || !query.target || query.target.purpose !== 'SUBMISSION')
      throw bad('CAPTURE_QUERY_INVALID');
    const fingerprint = Buffer.from(query.request_fingerprint ?? '', 'base64');
````

## 第 6 步：SubmissionCaptures（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 208 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>fingerprint.length !== 32</code>。条件成立时执行 <code>throw bad(&#x27;CAPTURE_QUERY_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;CAPTURE_QUERY_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.peers.rpc(&#x27;GetOperationResult&#x27;, rpcQuery)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>authority.state !== &#x27;SUCCEEDED&#x27; &#124;&#124; authority.operation_id !== query.operation_id</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_RESULT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_RESULT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!/^&#91;1-9&#93;&#91;0-9&#93;*$/.test(fixed.result?.content_version_id ?? &#x27;&#x27;)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_RESULT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_RESULT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return this.transactions.run(async (connection) =&gt; {       const &#91;&#91;row&#93;&#93; = await captures.findCapture(connection, query.operation_id);       if (!row &#124;&#124; !&#91;&#x27;CAPTURED&#x27;, &#x27;BOUND&#x27;&#93;.incl …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>captures.findCapture(connection, query.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!row &#124;&#124; !&#91;&#x27;CAPTURED&#x27;, &#x27;BOUND&#x27;&#93;.includes(row.state)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_NOT_FIXED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_NOT_FIXED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>!candidate.content &#124;&#124;         candidate.content.capture_id !== query.operation_id &#124;&#124;         candidate.content.body_hash !== row.body_hash.toString(&#x27;base64&#x27;)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_FIXED_BINDING_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_FIXED_BINDING_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return {         operation_id: query.operation_id,         content_version_id: fixed.result.content_version_id,         cut_json_utf8: candidate.content.cut_json_utf8,         body …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=8815:10494 -->
````javascript
    if (fingerprint.length !== 32) throw bad('CAPTURE_QUERY_INVALID');
    const rpcQuery = { ...query, request_fingerprint: fingerprint };
    // 这是已提交送审的系统事实读取：重查Go固定终态，而不是沿用旧用户的准入许可。
    // 新审核者仍由Go合并用例检查当前权限；旧来源关闭不能抹掉早已成功的切点。
    const authority = await this.peers.rpc('GetOperationResult', rpcQuery);
    if (authority.state !== 'SUCCEEDED' || authority.operation_id !== query.operation_id)
      throw new RPCFailure(14, 'CAPTURE_RESULT_UNKNOWN');
    const fixed = JSON.parse(Buffer.from(authority.fixed_result_json_utf8).toString('utf8'));
    if (!/^[1-9][0-9]*$/.test(fixed.result?.content_version_id ?? ''))
      throw new RPCFailure(14, 'CAPTURE_RESULT_UNKNOWN');
    return this.transactions.run(async (connection) => {
      const [[row]] = await captures.findCapture(connection, query.operation_id);
      if (!row || !['CAPTURED', 'BOUND'].includes(row.state))
        throw new RPCFailure(14, 'CAPTURE_NOT_FIXED');
      const candidate = json(row.candidate_json);
      matchCaptureQuery(row, candidate.binding, rpcQuery);
      if (
        !candidate.content ||
        candidate.content.capture_id !== query.operation_id ||
        candidate.content.body_hash !== row.body_hash.toString('base64')
      )
        throw new RPCFailure(14, 'CAPTURE_FIXED_BINDING_UNKNOWN');
      return {
        operation_id: query.operation_id,
        content_version_id: fixed.result.content_version_id,
        cut_json_utf8: candidate.content.cut_json_utf8,
        body_hash: candidate.content.body_hash,
      };
    });
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/submission_captures.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
