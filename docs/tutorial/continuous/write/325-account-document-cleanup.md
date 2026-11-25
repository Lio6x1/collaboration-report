# 分步编写 `services/collaboration/src/application/account_document_cleanup.mjs`

先按 [C06 · 固定候选清理、账号清理与转移计量](../lessons/collaboration/06-cleanup-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/325-account-document-cleanup.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

重查Doc清理许可，证明本地撤权排空及所有安装终结后原子清正文。

清理与迟到系统安装并发会把已删除正文重新造出来，因此双方必须共用文档入口和永久墓碑。

写之前先按这条执行路线理解各段的连接：

1. clean先重放本地原号成功回执，锁外取得Doc关闭context
2. 按来源→文档→room锁序，requireDrain核BLOCKED原scope/epoch/revocation及OUTPUT_STOPPED digest
3. 拒绝未结束MAIN/草稿/结构屏障、未知capture或未SETTLED delivery
4. 同事务清room正文、相关固定artifact、完成barrier并写cleanup回执
5. requireDocumentBodyPresent供所有系统安装拒绝清理后重生

本文件需要守住的边界：

- 超时或未知前置事实不得当可删除许可
- RESOURCE_CONFIRM保留审计，资源释放由Doc自己的协议决定

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/account_document_cleanup.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/account_document_cleanup.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/account_document_cleanup.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/account_document_cleanup.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:590 -->
````javascript
import { createHash } from 'node:crypto';
import {
  cleanupRequest,
  cleanupAuthority,
  cleanupReply,
} from '../domain/account_document_cleanup.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { binding, ack, assertSameBinding } from '../domain/fence_protocol.mjs';
import * as repository from '../repository/account_document_cleanup.mjs';
import * as barriers from '../repository/main_workspace.mjs';
import { lockedClosure } from '../repository/fences.mjs';
import { query } from '../repository/sql.mjs';

````

## 第 2 步：AccountDocumentCleanup

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`export class AccountDocumentCleanup `。

**本段进入 AccountDocumentCleanup.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, peers })</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, peers })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 AccountDocumentCleanup.replay：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>row.state</code>；<code>row.operation_id</code>；<code>row.payload</code> |
| <code>request</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>request.operation_id</code>；<code>cleanupAuthority(json(row.payload), request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>null</code>；<code>cleanupReply(cleanupAuthority(json(row.payload), request))</code> |

**本段进入 AccountDocumentCleanup.clean：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>cleanupRequest(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>prior</code>；<code>this.transactions.run(async (c) =&gt; {       // 与捕获/UPDATE相同：来源→MAIN入口→房间。系统安装也取得MAIN入口。       const &#91;&#91;source&#93;&#93; = await repository.lockCleanupSource(c, authority.scope_key);       aw …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>!row</code>。条件成立时执行 <code>return null;</code>。当前函数在这里结束。
- 检查 <code>row.state !== &#x27;SUCCEEDED&#x27; &#124;&#124; row.operation_id !== request.operation_id</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;ACCOUNT_DOCUMENT_CLEANUP_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;ACCOUNT_DOCUMENT_CLEANUP_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return cleanupReply(cleanupAuthority(json(row.payload), request));</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === &#x27;1&#x27;</code>。
- 等待 <code>this.transactions.run(async (c) =&gt;       this.replay(         await repository.cleanupReceipt(c, request.document_id, request.operation_id),         request,       ),     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.cleanupReceipt(c, request.document_id, request.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>prior</code>。条件成立时执行 <code>return prior;</code>。当前函数在这里结束。
- 等待 <code>this.peers.http(&#x27;/infra/lifecycle/document-close-context&#x27;, request)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return this.transactions.run(async (c) =&gt; {       // 与捕获/UPDATE相同：来源→MAIN入口→房间。系统安装也取得MAIN入口。       const &#91;&#91;source&#93;&#93; = await repository.lockCleanupSource(c, authority.scope_key);   …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>repository.lockCleanupSource(c, authority.scope_key)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>barriers.lockDocument(c, request.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.cleanupReceipt(c, request.document_id, request.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=590:2412 -->
````javascript
export class AccountDocumentCleanup {
  constructor(transactions, peers) {
    Object.assign(this, { transactions, peers });
  }

  replay(row, request) {
    if (!row) return null;
    if (row.state !== 'SUCCEEDED' || row.operation_id !== request.operation_id)
      throw new RPCFailure(6, 'ACCOUNT_DOCUMENT_CLEANUP_CONFLICT');
    return cleanupReply(cleanupAuthority(json(row.payload), request));
  }

  async clean(input) {
    const started = performance.now();
    const stage = (name) => {
      if (process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === '1')
        console.error(
          JSON.stringify({
            event: 'account_document_cleanup',
            stage: name,
            elapsed_ms: Math.round(performance.now() - started),
          }),
        );
    };
    const request = cleanupRequest(input);
    // 提交回复丢失后先读取原本地回执；后续Doc留存代次变化不改变已完成原清理。
    const prior = await this.transactions.run(async (c) =>
      this.replay(
        await repository.cleanupReceipt(c, request.document_id, request.operation_id),
        request,
      ),
    );
    if (prior) return prior;
    stage('ORIGINAL_RECEIPT_CHECKED');
    const authority = cleanupAuthority(
      await this.peers.http('/infra/lifecycle/document-close-context', request),
      request,
    );
    stage('DOC_AUTHORITY_CONFIRMED');
    return this.transactions.run(async (c) => {
      // 与捕获/UPDATE相同：来源→MAIN入口→房间。系统安装也取得MAIN入口。
      const [[source]] = await repository.lockCleanupSource(c, authority.scope_key);
      await barriers.lockDocument(c, request.document_id);
      const repeated = this.replay(
        await repository.cleanupReceipt(c, request.document_id, request.operation_id),
        request,
      );
````

## 第 3 步：AccountDocumentCleanup（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 62 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>repeated</code>。条件成立时执行 <code>return repeated;</code>。当前函数在这里结束。
- 在本段的内部函数内：等待 <code>this.requireDrain(c, authority, source)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>barriers.lockBarrier(c, request.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>barrier &amp;&amp; !&#91;&#x27;INSTALLED&#x27;, &#x27;ABORTED&#x27;&#93;.includes(barrier.state)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_INSTALL_PENDING&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_INSTALL_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.documentArtifacts(c, request.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!&#91;&#x27;SUCCEEDED&#x27;, &#x27;REJECTED&#x27;&#93;.includes(row.state)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_ARTIFACT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_ARTIFACT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>&#91;&#x27;DRAFT_MAIN_GATE&#x27;, &#x27;STRUCTURE_BRANCH_GATE&#x27;&#93;.includes(row.kind) &amp;&amp;           !&#91;&#x27;INSTALLED&#x27;, &#x27;ABORTED&#x27;&#93;.includes(value.state)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_INSTALL_PENDING&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_INSTALL_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>row.kind === &#x27;DRAFT_TRANSITION_PREPARED&#x27;</code>。
- 在本段的内部函数内：等待 <code>repository.draftInstallation(c, row.business_key)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!installed &#124;&#124;             installed.state !== &#x27;SUCCEEDED&#x27; &#124;&#124;             installed.operation_id !== row.operation_id &#124;&#124;             json(installed.payload).query?.target?.document_id !== request.document_id</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_DRAFT_PENDING&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_DRAFT_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.lockRooms(c, request.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.pendingDeliveries(c, room.id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>pending.length</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_OUTPUT_PENDING&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_OUTPUT_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.roomCaptures(c, room.id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>captures.some((capture) =&gt; capture.state === &#x27;UNKNOWN&#x27;)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_CAPTURE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_CAPTURE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.clearRoomBody(c, room.id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>row.kind === &#x27;RESOURCE_CONFIRM&#x27;</code>。条件成立时执行 <code>continue;</code>。跳过当前循环本轮剩余语句。
- 在本段的内部函数内：检查 <code>row.kind === &#x27;MERGE_ATTEMPT_CONTEXT&#x27;</code>。
- 在本段的内部函数内：等待 <code>repository.removeMergeResult(c, row.business_key)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.removeArtifact(c, row.id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=2412:4900 -->
````javascript
      if (repeated) return repeated;
      await this.requireDrain(c, authority, source);
      stage('LOCAL_DRAIN_CONFIRMED');
      const [[barrier]] = await barriers.lockBarrier(c, request.document_id);
      if (barrier && !['INSTALLED', 'ABORTED'].includes(barrier.state))
        throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_INSTALL_PENDING');
      const [fixed] = await repository.documentArtifacts(c, request.document_id);
      stage('ARTIFACTS_READ');
      for (const row of fixed) {
        const value = json(row.payload);
        if (!['SUCCEEDED', 'REJECTED'].includes(row.state))
          throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_ARTIFACT_UNKNOWN');
        if (
          ['DRAFT_MAIN_GATE', 'STRUCTURE_BRANCH_GATE'].includes(row.kind) &&
          !['INSTALLED', 'ABORTED'].includes(value.state)
        )
          throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_INSTALL_PENDING');
        if (row.kind === 'DRAFT_TRANSITION_PREPARED') {
          const installed = await repository.draftInstallation(c, row.business_key);
          if (
            !installed ||
            installed.state !== 'SUCCEEDED' ||
            installed.operation_id !== row.operation_id ||
            json(installed.payload).query?.target?.document_id !== request.document_id
          )
            throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_DRAFT_PENDING');
        }
      }
      const [rooms] = await repository.lockRooms(c, request.document_id);
      stage('ROOMS_LOCKED');
      for (const room of rooms) {
        const [pending] = await repository.pendingDeliveries(c, room.id);
        if (pending.length) throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_OUTPUT_PENDING');
        const [captures] = await repository.roomCaptures(c, room.id);
        if (captures.some((capture) => capture.state === 'UNKNOWN'))
          throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_CAPTURE_UNKNOWN');
      }
      // Doc固定计划证明原写入/安装已终结；CUT_FIXED不再猜测其用户操作结果。
      // 所有正文清除和成功墓碑在同一TX，网络/COMMIT未知只重送原号。
      for (const room of rooms) await repository.clearRoomBody(c, room.id);
      for (const row of fixed) {
        if (row.kind === 'RESOURCE_CONFIRM') continue; // 保留资源确认审计，Doc另行释放保护。
        if (row.kind === 'MERGE_ATTEMPT_CONTEXT')
          await repository.removeMergeResult(c, row.business_key);
        await repository.removeArtifact(c, row.id);
      }
````

## 第 4 步：AccountDocumentCleanup（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 108 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 AccountDocumentCleanup.requireDrain：**

**这一段为什么这样写：** 检查已经完成的真实输出停止证明。

1. 核来源门闩与原撤权ID
2. 重建原binding并比历史authorization_change
3. 比较OUTPUT_STOPPED ack摘要


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 清理事务连接<br>在此声明中的实际使用：<code>lockedClosure(connection, authority.revocation_id)</code>；<code>query(       connection,       &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=&#x27;node-runtime&#x27; AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPP …</code>（长表达式，按本步完整代码继续） |
| <code>authority</code> | <code>JavaScript 参数</code> | Doc固定关闭清理计划<br>在此声明中的实际使用：<code>authority.closed_epoch</code>；<code>authority.revocation_id</code>；<code>authority.scope_key</code> |
| <code>source</code> | <code>JavaScript 参数</code> | 已锁access_gate行<br>在此声明中的实际使用：<code>!source &#124;&#124;       source.state !== &#x27;BLOCKED&#x27;</code>；<code>source.state</code>；<code>source.max_epoch</code>；<code>source.last_revocation_id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>repository.clearBarrierBody(c, request.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.recordCleanup(c, authority)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return cleanupReply(authority);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!source &#124;&#124;       source.state !== &#x27;BLOCKED&#x27; &#124;&#124;       String(source.max_epoch) !== authority.closed_epoch &#124;&#124;       source.last_revocation_id !== authority.revocation_id</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_SOURCE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_SOURCE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>lockedClosure(connection, authority.revocation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!closure &#124;&#124; closure.state !== &#x27;SUCCEEDED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_DRAIN_PENDING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_DRAIN_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>query(       connection,       &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=&#x27;node-runtime&#x27; AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPP …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!proof &#124;&#124; !Buffer.from(proof.result_digest).equals(ack(request).result_digest)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_ACK_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;ACCOUNT_DOCUMENT_ACK_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 4 bytes=4900:6452 -->
````javascript
      await repository.clearBarrierBody(c, request.document_id);
      await repository.recordCleanup(c, authority);
      return cleanupReply(authority);
    });
  }

  async requireDrain(connection, authority, source) {
    if (
      !source ||
      source.state !== 'BLOCKED' ||
      String(source.max_epoch) !== authority.closed_epoch ||
      source.last_revocation_id !== authority.revocation_id
    )
      throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_SOURCE_CHANGED');
    const request = {
      revocation_id: authority.revocation_id,
      closed_scopes: [{ scope_key: authority.scope_key, epoch: authority.closed_epoch }],
      generation: '1',
      target_digest: createHash('sha256')
        .update('AUTHORIZATION_CHANGE\n' + authority.scope_key + '\n' + authority.closed_epoch)
        .digest(),
    };
    const closure = await lockedClosure(connection, authority.revocation_id);
    if (!closure || closure.state !== 'SUCCEEDED')
      throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_DRAIN_PENDING');
    assertSameBinding(closure, request.closed_scopes[0], binding(request));
    const [[proof]] = await query(
      connection,
      "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant='node-runtime' AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'",
      [authority.revocation_id, authority.scope_key, authority.closed_epoch],
    );
    if (!proof || !Buffer.from(proof.result_digest).equals(ack(request).result_digest))
      throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_ACK_CHANGED');
  }
}

````

## 第 5 步：requireDocumentBodyPresent

插入位置：第 4 步代码之后，紧接追加；完成稿约第 144 行。

已完成清理的文档不能被迟到系统安装复活。调用方已经持有同一MAIN入口锁。

本步声明：`export async function requireDocumentBodyPresent(connection, document) `。

**这一段为什么这样写：** 阻止迟到安装重建已经清理的正文。

1. 调用方先持MAIN文档锁
2. 任何已存在清理墓碑都拒绝安装


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 安装事务连接<br>在此声明中的实际使用：<code>repository.cleanupReceipt(connection, document)</code> |
| <code>document</code> | <code>JavaScript 参数</code> | 待安装文档ID<br>在此声明中的实际使用：<code>repository.cleanupReceipt(connection, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 检查 <code>await repository.cleanupReceipt(connection, document)</code>。条件成立时执行 <code>throw new RPCFailure(9, &#x27;ACCOUNT_DOCUMENT_CLEANED&#x27;);</code>。转入外层异常处理路径。
- 等待 <code>repository.cleanupReceipt(connection, document)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 执行 <code>throw new RPCFailure(9, &#x27;ACCOUNT_DOCUMENT_CLEANED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 5 bytes=6452:6752 -->
````javascript
// 已完成清理的文档不能被迟到系统安装复活。调用方已经持有同一MAIN入口锁。
export async function requireDocumentBodyPresent(connection, document) {
  if (await repository.cleanupReceipt(connection, document))
    throw new RPCFailure(9, 'ACCOUNT_DOCUMENT_CLEANED');
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/account_document_cleanup.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/06-cleanup-storage.md)。
