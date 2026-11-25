# 分步编写 `services/collaboration/src/application/draft_transitions.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/330-draft-transitions.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

消费固定预览，准备旧草稿屏障并按Doc真实终态换代安装或回退。

用户确认与跨服务提交可能相隔很久；必须检查预览后是否发生编辑，并使未知结果可原号恢复。

写之前先按这条执行路线理解各段的连接：

1. prepare先向Doc复核原context；同队列事务核fixed候选/有效期和草稿seq
2. 丢弃/逆向还锁全篇MAIN入口并核可靠MAIN cut，固定PREPARED后BLOCKED旧room
3. confirmation把丢弃预览绑定到独立DISCARD_DRAFT操作/轮次
4. apply锁外查GetOperationResult及/infra/draft/committed，结果未知不动作
5. SUCCEEDED核原候选后建新room epoch+1 seq0、lineage，退休旧room并关闭grant；失败只解除自己的屏障
6. 安装事实提交后真实stopRooms，再返回draft_sync_state READY

本文件需要守住的边界：

- 系统收尾消费已提交事实不依赖原用户仍有权限，但必须核原绑定和真实Go终态
- requireDocumentBodyPresent防止账号/生命周期清理后迟到安装重建正文
- CHANGED/PREVIEW_EXPIRED要求重新预览，不能覆盖最新草稿

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/draft_transitions.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/draft_transitions.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/draft_transitions.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/draft_transitions.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:954 -->
````javascript
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { schema, toPM, canonical } from '../../../../packages/content/index.mjs';
import * as repository from '../repository/draft_transitions.mjs';
import * as rooms from '../repository/room_access.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';
import { storeContent, wireContent } from '../domain/capture_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { validateDraftInstall, draftPreparedResult } from '../domain/draft_protocol.mjs';
import { lockSources } from './admission_gates.mjs';
import * as main from '../repository/main_workspace.mjs';
import { requireMainOpen } from './admission_gates.mjs';
import { deriveDiscardConfirmation, sameReliableMainCut } from '../domain/discard_confirmation.mjs';
import { requireDocumentBodyPresent } from './account_document_cleanup.mjs';

````

## 第 2 步：DraftTransitions

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`export class DraftTransitions `。

**本段进入 DraftTransitions.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, queue, snapshots, peers, outputs })</code> |
| <code>queue</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, queue, snapshots, peers, outputs })</code> |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, queue, snapshots, peers, outputs })</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, queue, snapshots, peers, outputs })</code> |
| <code>outputs</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, queue, snapshots, peers, outputs })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 DraftTransitions.prepare：**

**这一段为什么这样写：** 冻结用户刚确认的草稿位置。

1. 复核Doc当前资格
2. 校验预览候选/期限/切点
3. 建立必要的MAIN门闩
4. 将旧room阻塞并存准备结果


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 原操作context、预览引用/input_digest、lineage与轮次期望<br>在此声明中的实际使用：<code>input.context</code>；<code>this.confirmation(input)</code>；<code>this.binding(preview, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.queue.run(fixed.room_id, () =&gt;       this.transactions.run(async (c) =&gt; {         await lockSources(c, input.context.sources);         if (controlsMain) await main.lockDocumen …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 等待 <code>this.confirmation(input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>this.transactions.run(async (c) =&gt; {           const &#91;&#91;row&#93;&#93; = await repository.artifact(c, &#x27;DRAFT_PREVIEW_FIXED&#x27;, operation);           if (!row) throw new RPCFailure(14, &#x27;DRAFT_P …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.artifact(c, &#x27;DRAFT_PREVIEW_FIXED&#x27;, operation)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!row</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_PREVIEW_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;DRAFT_PREVIEW_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return preview;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.peers.http(&#x27;/infra/draft/context&#x27;, fixed.binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return this.queue.run(fixed.room_id, () =&gt;       this.transactions.run(async (c) =&gt; {         await lockSources(c, input.context.sources);         if (controlsMain) await main.lock …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>lockSources(c, input.context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>controlsMain</code>。
- 在本段的内部函数内：等待 <code>main.lockDocument(c, input.context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.artifact(c, &#x27;DRAFT_TRANSITION_PREPARED&#x27;, operation)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>prior</code>。
- 在本段的内部函数内：此处通过 <code>return value.result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>!fixed.candidate</code>。条件成立时执行 <code>return { state: &#x27;CONFLICTED&#x27;, conflicts: fixed.conflicts };</code>。当前函数在这里结束。
- 在本段的内部函数内：检查 <code>Number(fixed.expires_unix_ms) &lt;= Date.now()</code>。条件成立时执行 <code>return { state: &#x27;CHANGED&#x27;, code: &#x27;PREVIEW_EXPIRED&#x27; };</code>。当前函数在这里结束。
- 在本段的内部函数内：检查 <code>controlsMain</code>。
- 在本段的内部函数内：等待 <code>requireMainOpen(c, input.context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.unitMainRoom(             c,             input.context.target.document_id,             input.context.target.unit_id,           )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!sameReliableMainCut(fixed, mainRows)</code>。条件成立时执行 <code>return { state: &#x27;CHANGED&#x27;, code: &#x27;MAIN_CHANGED&#x27; };</code>。当前函数在这里结束。
- 在本段的内部函数内：等待 <code>rooms.lockRoom(c, fixed.room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=954:3190 -->
````javascript
export class DraftTransitions {
  constructor(transactions, queue, snapshots, peers, outputs) {
    Object.assign(this, { transactions, queue, snapshots, peers, outputs });
  }

  async prepare(input) {
    const operation = input.context?.operation_id;
    const discard = input.context?.target?.purpose === 'DISCARD_DRAFT';
    const controlsMain = discard || input.context?.target?.purpose === 'REVERSE';
    const fixed = discard
      ? await this.confirmation(input)
      : await this.transactions.run(async (c) => {
          const [[row]] = await repository.artifact(c, 'DRAFT_PREVIEW_FIXED', operation);
          if (!row) throw new RPCFailure(14, 'DRAFT_PREVIEW_UNKNOWN');
          const preview = json(row.payload);
          this.binding(preview, input);
          return preview;
        });
    // 原登录会话和当前资格由 Doc 在 SQL 锁外复核；未知结果不进入屏障准备。
    await this.peers.http('/infra/draft/context', fixed.binding);
    const preparedInput = discard ? { ...input, expected_input_digest: fixed.input_digest } : input;
    return this.queue.run(fixed.room_id, () =>
      this.transactions.run(async (c) => {
        await lockSources(c, input.context.sources);
        if (controlsMain) await main.lockDocument(c, input.context.target.document_id);
        const [[prior]] = await repository.artifact(c, 'DRAFT_TRANSITION_PREPARED', operation);
        if (prior) {
          const value = json(prior.payload);
          this.binding(value, preparedInput);
          return value.result;
        }
        if (!fixed.candidate) return { state: 'CONFLICTED', conflicts: fixed.conflicts };
        if (Number(fixed.expires_unix_ms) <= Date.now())
          return { state: 'CHANGED', code: 'PREVIEW_EXPIRED' };
        if (controlsMain) {
          await requireMainOpen(c, input.context.target.document_id);
          const [mainRows] = await repository.unitMainRoom(
            c,
            input.context.target.document_id,
            input.context.target.unit_id,
          );
          if (!sameReliableMainCut(fixed, mainRows))
            return { state: 'CHANGED', code: 'MAIN_CHANGED' };
        }
        const [[room]] = await rooms.lockRoom(c, fixed.room_id);
````

## 第 3 步：DraftTransitions（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 61 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 DraftTransitions.confirmation：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.context</code>；<code>input.expected_lineage_id</code>；<code>input.expected_attempt_id</code>；<code>input.expected_round_id</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.transactions.run(async (c) =&gt; {       await lockSources(c, context.sources);       const &#91;&#91;original&#93;&#93; = await repository.artifact(         c,         &#x27;DRAFT_PREVIEW_FIXED&#x27;,    …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>!room &#124;&#124;           room.state !== &#x27;OPEN&#x27; &#124;&#124;           room.epoch !== fixed.draft_cut.workspace_epoch &#124;&#124;           room.last_durable_seq !== fixed.draft_cut.durable_seq</code>。条件成立时执行 <code>return { state: &#x27;CHANGED&#x27;, code: &#x27;DRAFT_CHANGED&#x27; };</code>。当前函数在这里结束。
- 在本段的内部函数内：检查 <code>controlsMain</code>。
- 在本段的内部函数内：等待 <code>repository.setMainGate(             c,             input.context.target.document_id,             operation,             &#x27;PREPARED&#x27;,             fixed.binding,           )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.blockRoom(c, room.id, room.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.insertArtifact(c, &#x27;DRAFT_TRANSITION_PREPARED&#x27;, operation, {           ...fixed,           result,         })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.peers.http(&#x27;/infra/draft/context&#x27;, binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>input.expected_attempt_id !== authority.preview_cut_ref &#124;&#124;       input.expected_round_id !== authority.expected_round_id &#124;&#124;       Buffer.from(         input.expected_input_digest,  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DISCARD_CONFIRMATION_CONFLICT&#x27;);</code>。转入外层异常处理路径。

<!-- write-step: 3 bytes=3190:4867 -->
````javascript
        if (
          !room ||
          room.state !== 'OPEN' ||
          room.epoch !== fixed.draft_cut.workspace_epoch ||
          room.last_durable_seq !== fixed.draft_cut.durable_seq
        )
          return { state: 'CHANGED', code: 'DRAFT_CHANGED' };
        const result = draftPreparedResult(fixed);
        if (controlsMain)
          await repository.setMainGate(
            c,
            input.context.target.document_id,
            operation,
            'PREPARED',
            fixed.binding,
          );
        await repository.blockRoom(c, room.id, room.epoch);
        await repository.insertArtifact(c, 'DRAFT_TRANSITION_PREPARED', operation, {
          ...fixed,
          result,
        });
        return result;
      }),
    );
  }

  async confirmation(input) {
    const context = {
      ...input.context,
      request_fingerprint: Buffer.from(
        input.context.request_fingerprint,
        typeof input.context.request_fingerprint === 'string' ? 'base64' : undefined,
      ).toString('base64'),
    };
    const binding = {
      context,
      expected_branch_id: context.target.branch_id,
      expected_structure_id: context.target.structure_id,
      expected_lineage_id: input.expected_lineage_id,
    };
    const authority = await this.peers.http('/infra/draft/context', binding);
    if (
      input.expected_attempt_id !== authority.preview_cut_ref ||
      input.expected_round_id !== authority.expected_round_id ||
      Buffer.from(
        input.expected_input_digest,
        typeof input.expected_input_digest === 'string' ? 'base64' : undefined,
      ).toString('base64') !== authority.preview_input_digest
    )
````

## 第 4 步：DraftTransitions（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 110 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 DraftTransitions.binding：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>fixed.binding</code>；<code>fixed.preview_cut_ref</code>；<code>fixed.source</code>；<code>fixed.input_digest</code> |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.context</code>；<code>input.expected_attempt_id</code>；<code>input.expected_round_id</code>；<code>input.expected_input_digest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**本段进入 DraftTransitions.apply：**

**这一段为什么这样写：** 用权威终态完成可恢复的房间换代。

1. 查Go原号终态与固定安装源
2. 校验manifest内容及新lineage
3. 同事务安装新代并记回执，或回退原屏障
4. 等待旧socket真实关闭后READY


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | context、expected_attempt_id、expected_input_digest、expected_round_id和固定resolution<br>在此声明中的实际使用：<code>input.context</code>；<code>input.expected_round_id</code>；<code>this.binding(fixed, input)</code>；<code>input.resolution</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>{       ...authority,       current_state_json_utf8: Buffer.from(         JSON.stringify({           draft_sync_state: &#x27;READY&#x27;,           new_room_id: source.new_room_id ?? null,   …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 执行 <code>throw new RPCFailure(6, &#x27;DISCARD_CONFIRMATION_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return this.transactions.run(async (c) =&gt; {       await lockSources(c, context.sources);       const &#91;&#91;original&#93;&#93; = await repository.artifact(         c,         &#x27;DRAFT_PREVIEW_FIX …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>lockSources(c, context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.artifact(         c,         &#x27;DRAFT_PREVIEW_FIXED&#x27;,         authority.preview_cut_ref,       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!original</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DISCARD_PREVIEW_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;DISCARD_PREVIEW_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.artifact(         c,         &#x27;DRAFT_CONFIRMATION_FIXED&#x27;,         context.operation_id,       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>prior</code>。
- 在本段的内部函数内：检查 <code>canonical(json(prior.payload)) !== canonical(fixed)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DISCARD_CONFIRMATION_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;DISCARD_CONFIRMATION_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return json(prior.payload);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>repository.insertArtifact(c, &#x27;DRAFT_CONFIRMATION_FIXED&#x27;, context.operation_id, fixed)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return fixed;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>canonical(context) !== canonical(fixed.binding.context) &#124;&#124;       input.expected_attempt_id !== fixed.preview_cut_ref &#124;&#124;       (context.target.purpose === &#x27;CONFLICT_RESOLUTION&#x27; &amp;&amp;   …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DRAFT_TRANSITION_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;DRAFT_TRANSITION_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 4 bytes=4867:6666 -->
````javascript
      throw new RPCFailure(6, 'DISCARD_CONFIRMATION_CONFLICT');
    return this.transactions.run(async (c) => {
      await lockSources(c, context.sources);
      const [[original]] = await repository.artifact(
        c,
        'DRAFT_PREVIEW_FIXED',
        authority.preview_cut_ref,
      );
      if (!original) throw new RPCFailure(14, 'DISCARD_PREVIEW_UNKNOWN');
      const fixed = deriveDiscardConfirmation(json(original.payload), binding, authority);
      const [[prior]] = await repository.artifact(
        c,
        'DRAFT_CONFIRMATION_FIXED',
        context.operation_id,
      );
      if (prior) {
        if (canonical(json(prior.payload)) !== canonical(fixed))
          throw new RPCFailure(6, 'DISCARD_CONFIRMATION_CONFLICT');
        return json(prior.payload);
      }
      draftPreparedResult(fixed);
      await repository.insertArtifact(c, 'DRAFT_CONFIRMATION_FIXED', context.operation_id, fixed);
      return fixed;
    });
  }

  binding(fixed, input) {
    const context = {
      ...input.context,
      request_fingerprint: Buffer.from(
        input.context?.request_fingerprint ?? '',
        typeof input.context?.request_fingerprint === 'string' ? 'base64' : undefined,
      ).toString('base64'),
    };
    if (
      canonical(context) !== canonical(fixed.binding.context) ||
      input.expected_attempt_id !== fixed.preview_cut_ref ||
      (context.target.purpose === 'CONFLICT_RESOLUTION' &&
        input.expected_round_id !== fixed.source.expected_round_id) ||
      Buffer.from(
        input.expected_input_digest ?? '',
        typeof input.expected_input_digest === 'string' ? 'base64' : undefined,
      ).toString('base64') !== fixed.input_digest
    )
      throw new RPCFailure(6, 'DRAFT_TRANSITION_ID_CONFLICT');
  }

  async apply(input) {
````

## 第 5 步：DraftTransitions（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 158 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!&#91;&#x27;PULL_MAIN&#x27;, &#x27;DISCARD_DRAFT&#x27;, &#x27;REVERSE&#x27;, &#x27;CONFLICT_RESOLUTION&#x27;&#93;.includes(         query.target?.purpose,       ) &#124;&#124;       (!&#91;&#x27;DISCARD_DRAFT&#x27;, &#x27;CONFLICT_RESOLUTION&#x27;&#93;.includes(quer …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(3, &#x27;DRAFT_TRANSITION_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(3, &#x27;DRAFT_TRANSITION_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.peers.rpc(&#x27;GetOperationResult&#x27;, query)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!&#91;&#x27;SUCCEEDED&#x27;, &#x27;REJECTED&#x27;, &#x27;CANCELLED&#x27;&#93;.includes(authority.state)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_RESULT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;DRAFT_RESULT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.peers.http(&#x27;/infra/draft/committed&#x27;, {       ...query,       request_fingerprint: Buffer.from(query.request_fingerprint).toString(&#x27;base64&#x27;),     })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>source.result_digest !== Buffer.from(authority.result_digest).toString(&#x27;base64&#x27;)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_RESULT_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;DRAFT_RESULT_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>(discard &#124;&#124; query.target.purpose === &#x27;CONFLICT_RESOLUTION&#x27;) &amp;&amp;       source.expected_round_id !== input.expected_round_id</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DISCARD_ROUND_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;DISCARD_ROUND_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.queue.run(roomID, () =&gt;       this.transactions.run(async (c) =&gt; {         // 即使只安装支线也与生命周期清理共用入口，避免迟到系统请求重建私有房间。         await main.lockDocument(c, query.target.document_id); …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>main.lockDocument(c, query.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>requireDocumentBodyPresent(c, query.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.artifact(           c,           &#x27;DRAFT_TRANSITION_PREPARED&#x27;,           query.operation_id,         )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>authority.state === &#x27;SUCCEEDED&#x27;</code>。
- 在本段的内部函数内：检查 <code>!prepared</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_BARRIER_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;DRAFT_BARRIER_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 5 bytes=6666:8878 -->
````javascript
    const query = {
      operation_id: input.context?.operation_id,
      target: input.context?.target,
      request_fingerprint: input.context?.request_fingerprint,
    };
    if (
      !['PULL_MAIN', 'DISCARD_DRAFT', 'REVERSE', 'CONFLICT_RESOLUTION'].includes(
        query.target?.purpose,
      ) ||
      (!['DISCARD_DRAFT', 'CONFLICT_RESOLUTION'].includes(query.target.purpose) &&
        input.expected_round_id !== '0')
    )
      throw new RPCFailure(3, 'DRAFT_TRANSITION_INVALID');
    // 安装属于已提交系统事实，不能依赖原用户仍有权限；失败也只能解除自己的旧屏障。
    const authority = await this.peers.rpc('GetOperationResult', query);
    if (!['SUCCEEDED', 'REJECTED', 'CANCELLED'].includes(authority.state))
      throw new RPCFailure(14, 'DRAFT_RESULT_UNKNOWN');
    const source = await this.peers.http('/infra/draft/committed', {
      ...query,
      request_fingerprint: Buffer.from(query.request_fingerprint).toString('base64'),
    });
    if (source.result_digest !== Buffer.from(authority.result_digest).toString('base64'))
      throw new RPCFailure(14, 'DRAFT_RESULT_CHANGED');
    const discard = query.target.purpose === 'DISCARD_DRAFT';
    const controlsMain = discard || query.target.purpose === 'REVERSE';
    if (
      (discard || query.target.purpose === 'CONFLICT_RESOLUTION') &&
      source.expected_round_id !== input.expected_round_id
    )
      throw new RPCFailure(6, 'DISCARD_ROUND_CONFLICT');
    const roomID = source.old_room_id;
    await this.queue.run(roomID, () =>
      this.transactions.run(async (c) => {
        // 即使只安装支线也与生命周期清理共用入口，避免迟到系统请求重建私有房间。
        await main.lockDocument(c, query.target.document_id);
        await requireDocumentBodyPresent(c, query.target.document_id);
        const [[prepared]] = await repository.artifact(
          c,
          'DRAFT_TRANSITION_PREPARED',
          query.operation_id,
        );
        if (authority.state === 'SUCCEEDED') {
          if (!prepared) throw new RPCFailure(14, 'DRAFT_BARRIER_UNKNOWN');
          const fixed = json(prepared.payload);
          this.binding(fixed, input);
````

## 第 6 步：DraftTransitions（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 203 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>canonical(storeContent(input.resolution)) !== canonical(source.content)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DRAFT_INSTALL_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;DRAFT_INSTALL_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.artifact(           c,           &#x27;DRAFT_TRANSITION_INSTALLED&#x27;,           query.operation_id,         )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>finished</code>。
- 在本段的内部函数内：检查 <code>canonical(json(finished.payload).query) !==             canonical({               ...query,               request_fingerprint: Buffer.from(query.request_fingerprint).toString(&#x27;base64&#x27;),             })</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DRAFT_TRANSITION_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;DRAFT_TRANSITION_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>rooms.lockRoom(c, roomID)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>authority.state !== &#x27;SUCCEEDED&#x27;</code>。
- 在本段的内部函数内：检查 <code>prepared</code>。
- 在本段的内部函数内：检查 <code>canonical(context) !== canonical(fixed.binding.context) &#124;&#124;               input.expected_attempt_id !== fixed.preview_cut_ref</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DRAFT_TRANSITION_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;DRAFT_TRANSITION_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>!room &#124;&#124; room.epoch !== query.target.workspace_epoch &#124;&#124; room.state !== &#x27;BLOCKED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_BARRIER_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;DRAFT_BARRIER_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.openRoom(c, room.id, room.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!prepared &#124;&#124; !room &#124;&#124; room.state !== &#x27;BLOCKED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_BARRIER_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;DRAFT_BARRIER_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 6 bytes=8878:10823 -->
````javascript
          validateDraftInstall(fixed, source);
          if (canonical(storeContent(input.resolution)) !== canonical(source.content))
            throw new RPCFailure(6, 'DRAFT_INSTALL_CONFLICT');
        }
        const [[finished]] = await repository.artifact(
          c,
          'DRAFT_TRANSITION_INSTALLED',
          query.operation_id,
        );
        if (finished) {
          if (
            canonical(json(finished.payload).query) !==
            canonical({
              ...query,
              request_fingerprint: Buffer.from(query.request_fingerprint).toString('base64'),
            })
          )
            throw new RPCFailure(6, 'DRAFT_TRANSITION_ID_CONFLICT');
          return;
        }
        const [[room]] = await rooms.lockRoom(c, roomID);
        if (authority.state !== 'SUCCEEDED') {
          if (prepared) {
            const fixed = json(prepared.payload);
            // 已拒绝的系统原号不能依赖失效来源重新取预览；全原上下文和权威终态仍须一致。
            const context = {
              ...input.context,
              request_fingerprint: Buffer.from(input.context.request_fingerprint).toString(
                'base64',
              ),
            };
            if (
              canonical(context) !== canonical(fixed.binding.context) ||
              input.expected_attempt_id !== fixed.preview_cut_ref
            )
              throw new RPCFailure(6, 'DRAFT_TRANSITION_ID_CONFLICT');
            if (!room || room.epoch !== query.target.workspace_epoch || room.state !== 'BLOCKED')
              throw new RPCFailure(14, 'DRAFT_BARRIER_UNKNOWN');
            await repository.openRoom(c, room.id, room.epoch);
          }
        } else {
          if (!prepared || !room || room.state !== 'BLOCKED')
            throw new RPCFailure(14, 'DRAFT_BARRIER_UNKNOWN');
          const fixed = json(prepared.payload);
          this.binding(fixed, input);
````

## 第 7 步：DraftTransitions（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 248 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>room.epoch !== query.target.workspace_epoch &#124;&#124;             room.last_durable_seq !== fixed.draft_cut.durable_seq &#124;&#124;             source.new_epoch !== (BigInt(room.epoch) + 1n).toStr …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DRAFT_INSTALL_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;DRAFT_INSTALL_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>rooms.lockRoom(c, source.new_room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>next</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_NEW_ROOM_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;DRAFT_NEW_ROOM_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>rooms.insertRoom(             c,             source.new_room_id,             query.target.document_id,             query.target.unit_id,             &#x27;BRANCH&#x27;,             query.tar …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>this.snapshots.checkpoint(               c,               { id: source.new_room_id, epoch: source.new_epoch },               doc,               &#x27;0&#x27;,             )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>captures.insertRoomLineage(c, source.new_room_id, source.new_epoch, source.lineage)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.retireRoom(c, room.id, room.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.closeGrants(c, room.id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.stopDeliveries(c, room.id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.insertArtifact(c, &#x27;DRAFT_TRANSITION_INSTALLED&#x27;, query.operation_id, {           query: {             ...query,             request_fingerprint: Buffer.from(query.request …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>controlsMain</code>。

<!-- write-step: 7 bytes=10823:12616 -->
````javascript
          if (
            room.epoch !== query.target.workspace_epoch ||
            room.last_durable_seq !== fixed.draft_cut.durable_seq ||
            source.new_epoch !== (BigInt(room.epoch) + 1n).toString() ||
            canonical(storeContent(input.resolution)) !== canonical(source.content)
          )
            throw new RPCFailure(6, 'DRAFT_INSTALL_CONFLICT');
          const [[next]] = await rooms.lockRoom(c, source.new_room_id);
          if (next) throw new RPCFailure(14, 'DRAFT_NEW_ROOM_UNKNOWN');
          await rooms.insertRoom(
            c,
            source.new_room_id,
            query.target.document_id,
            query.target.unit_id,
            'BRANCH',
            query.target.branch_id,
            source.new_epoch,
          );
          const doc = prosemirrorJSONToYDoc(
            schema,
            toPM(JSON.parse(wireContent(source.content).body_json_utf8)),
          );
          try {
            await this.snapshots.checkpoint(
              c,
              { id: source.new_room_id, epoch: source.new_epoch },
              doc,
              '0',
            );
          } finally {
            doc.destroy();
          }
          await captures.insertRoomLineage(c, source.new_room_id, source.new_epoch, source.lineage);
          await repository.retireRoom(c, room.id, room.epoch);
          await repository.closeGrants(c, room.id);
          await repository.stopDeliveries(c, room.id);
        }
        await repository.insertArtifact(c, 'DRAFT_TRANSITION_INSTALLED', query.operation_id, {
          query: {
            ...query,
            request_fingerprint: Buffer.from(query.request_fingerprint).toString('base64'),
          },
          result_digest: source.result_digest,
        });
        if (controlsMain) {
````

## 第 8 步：DraftTransitions（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 293 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>repository.artifact(c, &#x27;DRAFT_MAIN_GATE&#x27;, query.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>row &amp;&amp; json(row.payload).state === &#x27;PREPARED&#x27;</code>。
- 在本段的内部函数内：检查 <code>json(row.payload).operation_id !== query.operation_id</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_MAIN_GATE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;DRAFT_MAIN_GATE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.setMainGate(               c,               query.target.document_id,               query.operation_id,               authority.state === &#x27;SUCCEEDED&#x27; ? &#x27;INSTALLED&#x27; : &#x27;AB …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>authority.state === &#x27;SUCCEEDED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_MAIN_GATE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;DRAFT_MAIN_GATE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>authority.state === &#x27;SUCCEEDED&#x27;</code>。
- 等待 <code>this.outputs.stopRooms(&#91;roomID&#93;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return {       ...authority,       current_state_json_utf8: Buffer.from(         JSON.stringify({           draft_sync_state: &#x27;READY&#x27;,           new_room_id: source.new_room_id ??  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=12616:13901 -->
````javascript
          const [[row]] = await repository.artifact(c, 'DRAFT_MAIN_GATE', query.target.document_id);
          if (row && json(row.payload).state === 'PREPARED') {
            if (json(row.payload).operation_id !== query.operation_id)
              throw new RPCFailure(14, 'DRAFT_MAIN_GATE_UNKNOWN');
            await repository.setMainGate(
              c,
              query.target.document_id,
              query.operation_id,
              authority.state === 'SUCCEEDED' ? 'INSTALLED' : 'ABORTED',
              json(row.payload).binding,
            );
          } else if (authority.state === 'SUCCEEDED')
            throw new RPCFailure(14, 'DRAFT_MAIN_GATE_UNKNOWN');
        }
      }),
    );
    if (authority.state === 'SUCCEEDED') await this.outputs.stopRooms([roomID]);
    // fixed_result 是原 Go 回执；实际输出关闭后的 current_state 是本次收尾确认。
    return {
      ...authority,
      current_state_json_utf8: Buffer.from(
        JSON.stringify({
          draft_sync_state: 'READY',
          new_room_id: source.new_room_id ?? null,
          new_epoch: source.new_epoch ?? null,
          lineage_id: source.content?.lineage_ref ?? null,
          content_digest: source.content?.body_hash ?? null,
        }),
      ),
    };
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/draft_transitions.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
