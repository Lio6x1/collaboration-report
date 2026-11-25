# 分步编写 `services/collaboration/src/application/draft_previews.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/329-draft-previews.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

固定拉主线、丢弃、逆向和冲突解决的草稿预览。

预览应展示准确的草稿与可靠MAIN状态，不得凭mTLS身份读取任意文档或每次刷新换结果。

写之前先按这条执行路线理解各段的连接：

1. Doc /infra/draft/context只返回已登记原操作的范围
2. 同队列事务核来源/MAIN入口/BRANCH身份及lineage，缺房间时从基线建seq0
3. 采样草稿及同源MAIN可靠尾部，按purpose调用computePull/Reverse/Resolution或丢弃取main
4. 固定draft/main/candidate/conflicts、10分钟expires和input_digest到DRAFT_PREVIEW_FIXED
5. fixed/replay只重放原绑定，不重新恢复房间

本文件需要守住的边界：

- 同unit多个MAIN房间或来源不匹配应MAIN_CONTEXT_UNKNOWN
- 预览过期不能悄悄续期；需新预览
- 非PULL_MAIN固定传输超过24MiB明确失败

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/draft_previews.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/draft_previews.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/draft_previews.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/draft_previews.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:1202 -->
````javascript
import { prosemirrorJSONToYDoc, yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  schema,
  toPM,
  fromPM,
  normalizeContent,
  canonical,
} from '../../../../packages/content/index.mjs';
import * as repository from '../repository/draft_transitions.mjs';
import * as rooms from '../repository/room_access.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';
import * as main from '../repository/main_workspace.mjs';
import { wireContent } from '../domain/capture_protocol.mjs';
import {
  draftPreviewBinding,
  fixedDraft,
  computeDraftPreview,
  draftBody,
  draftLineage,
  draftPreviewCutDigest,
} from '../domain/draft_protocol.mjs';
import { wireCut } from '../domain/main_protocol.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { lockSources } from './admission_gates.mjs';
import { requireMainOpen } from './admission_gates.mjs';
import { computeReverse } from '../domain/reverse_draft.mjs';
import { computeResolution } from '../domain/resolution_draft.mjs';
import { matchesMainWorkspace } from '../domain/main_workspace.mjs';

````

## 第 2 步：DraftPreviews

插入位置：第 1 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`export class DraftPreviews `。

**本段进入 DraftPreviews.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, queue, snapshots, peers })</code> |
| <code>queue</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, queue, snapshots, peers })</code> |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, queue, snapshots, peers })</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, queue, snapshots, peers })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 DraftPreviews.capture：**

**这一段为什么这样写：** 产生之后准备阶段可逐项核验的不可变预览。

1. 先查同号结果
2. 在锁外取得Doc权威范围
3. 事务内采样可靠cut与lineage
4. 计算目的特定候选并落库


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 含context和expected_branch/structure/lineage的预览请求<br>在此声明中的实际使用：<code>draftPreviewBinding(input)</code>；<code>input.context</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>prior</code>；<code>this.queue.run(source.room_id, () =&gt;       this.transactions.run(async (c) =&gt; {         await lockSources(c, context.sources);         await requireMainOpen(c, context.target.docum …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 等待 <code>this.transactions.run(async (c) =&gt; {       await lockSources(c, context.sources);       const &#91;&#91;row&#93;&#93; = await repository.artifact(c, &#x27;DRAFT_PREVIEW_FIXED&#x27;, context.operation_id);   …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>lockSources(c, context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.artifact(c, &#x27;DRAFT_PREVIEW_FIXED&#x27;, context.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return row &amp;&amp; this.replay(json(row.payload), binding);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>prior</code>。条件成立时执行 <code>return prior;</code>。当前函数在这里结束。
- 等待 <code>this.peers.http(&#x27;/infra/draft/context&#x27;, binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>context.target.purpose !== &#x27;PULL_MAIN&#x27; &amp;&amp; source.purpose !== context.target.purpose</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;DRAFT_PREVIEW_PURPOSE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;DRAFT_PREVIEW_PURPOSE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return this.queue.run(source.room_id, () =&gt;       this.transactions.run(async (c) =&gt; {         await lockSources(c, context.sources);         await requireMainOpen(c, context.targe …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>lockSources(c, context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>requireMainOpen(c, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.artifact(c, &#x27;DRAFT_PREVIEW_FIXED&#x27;, context.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>old</code>。条件成立时执行 <code>return this.replay(json(old.payload), binding);</code>。当前函数在这里结束。
- 在本段的内部函数内：等待 <code>rooms.lockRoom(c, source.room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!room</code>。
- 在本段的内部函数内：等待 <code>rooms.insertRoom(             c,             source.room_id,             context.target.document_id,             context.target.unit_id,             &#x27;BRANCH&#x27;,             context.t …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=1202:3194 -->
````javascript
export class DraftPreviews {
  constructor(transactions, queue, snapshots, peers) {
    Object.assign(this, { transactions, queue, snapshots, peers });
  }

  async capture(input) {
    const binding = draftPreviewBinding(input),
      context = input.context;
    const prior = await this.transactions.run(async (c) => {
      await lockSources(c, context.sources);
      const [[row]] = await repository.artifact(c, 'DRAFT_PREVIEW_FIXED', context.operation_id);
      return row && this.replay(json(row.payload), binding);
    });
    if (prior) return prior;
    // Doc 只提供已登记原号的单元范围；mTLS 身份本身不是读取其他单元正文的许可。
    const source = await this.peers.http('/infra/draft/context', binding);
    if (context.target.purpose !== 'PULL_MAIN' && source.purpose !== context.target.purpose)
      throw new RPCFailure(14, 'DRAFT_PREVIEW_PURPOSE_UNKNOWN');
    return this.queue.run(source.room_id, () =>
      this.transactions.run(async (c) => {
        await lockSources(c, context.sources);
        await requireMainOpen(c, context.target.document_id);
        const [[old]] = await repository.artifact(c, 'DRAFT_PREVIEW_FIXED', context.operation_id);
        if (old) return this.replay(json(old.payload), binding);
        let [[room]] = await rooms.lockRoom(c, source.room_id);
        if (!room) {
          await rooms.insertRoom(
            c,
            source.room_id,
            context.target.document_id,
            context.target.unit_id,
            'BRANCH',
            context.target.branch_id,
            context.target.workspace_epoch,
          );
          room = {
            id: source.room_id,
            document_id: context.target.document_id,
            unit_id: context.target.unit_id,
            workspace_kind: 'BRANCH',
            workspace_id: context.target.branch_id,
            epoch: context.target.workspace_epoch,
            last_durable_seq: '0',
            state: 'OPEN',
          };
````

## 第 3 步：DraftPreviews（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 77 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>this.snapshots.checkpoint(c, room, seed, &#x27;0&#x27;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>captures.insertRoomLineage(c, room.id, room.epoch, source.lineage)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>room.state !== &#x27;OPEN&#x27; &#124;&#124;           room.workspace_kind !== &#x27;BRANCH&#x27; &#124;&#124;           room.workspace_id !== context.target.branch_id &#124;&#124;           room.epoch !== context.target.workspace …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(9, &#x27;WORKSPACE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(9, &#x27;WORKSPACE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>captures.lockRoomLineage(c, room.id, room.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!lineageRow &#124;&#124;           canonical(draftLineage(json(lineageRow.payload))) !== canonical(source.lineage)</code>。条件成立时执行 <code>throw new RPCFailure(9, &#x27;LINEAGE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(9, &#x27;LINEAGE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>this.body(c, room)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.unitMainRoom(c, room.document_id, room.unit_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>mainRooms.length &gt; 1</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_CONTEXT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;MAIN_CONTEXT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>mainRoom</code>。
- 在本段的内部函数内：等待 <code>main.mainContext(c, mainRoom.id, mainRoom.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>mainRoom.state !== &#x27;OPEN&#x27; &#124;&#124;             !matchesMainWorkspace(current, mainRoom, {               ...source,               manifest_id: source.main_manifest_id,             })</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_CONTEXT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。

<!-- write-step: 3 bytes=3194:5155 -->
````javascript
          const seed = prosemirrorJSONToYDoc(schema, toPM(source.lineage.body));
          try {
            await this.snapshots.checkpoint(c, room, seed, '0');
          } finally {
            seed.destroy();
          }
          await captures.insertRoomLineage(c, room.id, room.epoch, source.lineage);
        }
        if (
          room.state !== 'OPEN' ||
          room.workspace_kind !== 'BRANCH' ||
          room.workspace_id !== context.target.branch_id ||
          room.epoch !== context.target.workspace_epoch ||
          room.unit_id !== context.target.unit_id ||
          room.document_id !== context.target.document_id
        )
          throw new RPCFailure(9, 'WORKSPACE_CHANGED');
        const [[lineageRow]] = await captures.lockRoomLineage(c, room.id, room.epoch);
        if (
          !lineageRow ||
          canonical(draftLineage(json(lineageRow.payload))) !== canonical(source.lineage)
        )
          throw new RPCFailure(9, 'LINEAGE_CHANGED');
        const cut = {
          room_id: room.id,
          workspace_epoch: room.epoch,
          durable_seq: room.last_durable_seq,
        };
        const draft = await this.body(c, room);
        const [mainRooms] = await repository.unitMainRoom(c, room.document_id, room.unit_id);
        if (mainRooms.length > 1) throw new RPCFailure(14, 'MAIN_CONTEXT_UNKNOWN');
        const mainRoom = mainRooms[0];
        let mainBody = normalizeContent(JSON.parse(wireContent(source.main).body_json_utf8), {
          workspaceKind: 'MAIN',
        });
        let mainCut = [];
        if (mainRoom) {
          const [[mainContext]] = await main.mainContext(c, mainRoom.id, mainRoom.epoch);
          const current = mainContext && json(mainContext.payload);
          if (
            mainRoom.state !== 'OPEN' ||
            !matchesMainWorkspace(current, mainRoom, {
              ...source,
              manifest_id: source.main_manifest_id,
            })
          )
````

## 第 4 步：DraftPreviews（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 123 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;MAIN_CONTEXT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>this.body(c, mainRoom, &#x27;MAIN&#x27;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 4 bytes=5155:8014 -->
````javascript
            throw new RPCFailure(14, 'MAIN_CONTEXT_UNKNOWN');
          mainBody = await this.body(c, mainRoom, 'MAIN');
          mainCut = [
            {
              room_id: mainRoom.id,
              workspace_epoch: mainRoom.epoch,
              durable_seq: mainRoom.last_durable_seq,
            },
          ];
        }
        const preview =
          context.target.purpose !== 'PULL_MAIN'
            ? context.operation_id
            : 'draft-preview:' + context.operation_id;
        const merged =
          context.target.purpose === 'REVERSE'
            ? computeReverse(
                JSON.parse(wireContent(source.reverse_before).body_json_utf8),
                JSON.parse(wireContent(source.reverse_after).body_json_utf8),
                draft,
                mainBody,
                source.lineage.body,
              )
            : context.target.purpose === 'CONFLICT_RESOLUTION'
              ? computeResolution(
                  JSON.parse(wireContent(source.resolution_submitted).body_json_utf8),
                  source.resolution_content,
                  draft,
                )
              : computeDraftPreview(context.target.purpose, source.lineage.body, draft, mainBody);
        const formalMain = wireContent(source.main);
        const mainLineage = {
          lineage_id: formalMain.lineage_ref,
          // 可靠尾部以正式旧版本为比较基线；尚未物化的新增块只能来自本次采样。
          body: draftBody(JSON.parse(formalMain.body_json_utf8)),
          provenance: JSON.parse(formalMain.provenance_json_utf8),
          base_manifest_id: source.main_manifest_id,
          base_version_id: source.main_version_id,
        };
        const fixed = {
          binding,
          room_id: room.id,
          draft_cut: cut,
          main_cut: mainCut,
          source,
          draft: fixedDraft(draft, context, cut, source.lineage, preview),
          main: fixedDraft(
            draftBody(mainBody),
            {
              ...context,
              target: {
                ...context.target,
                branch_id: '0',
                workspace_epoch: source.control_revision,
              },
            },
            mainCut[0] ?? {
              room_id: '0',
              workspace_epoch: source.workspace_epoch,
              durable_seq: '0',
            },
            mainLineage,
            preview + ':main',
          ),
          candidate:
            merged.body &&
            fixedDraft(
              merged.body,
              context,
              cut,
              source.lineage,
              context.target.purpose === 'REVERSE' ? preview : preview + ':candidate',
            ),
          conflicts: merged.conflicts,
          preview_cut_ref: preview,
          expires_unix_ms: String(Date.now() + 600000),
        };
````

## 第 5 步：DraftPreviews（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 200 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 DraftPreviews.fixed：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>draftPreviewBinding(input)</code>；<code>input.context</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.transactions.run(async (connection) =&gt; {       await lockSources(connection, input.context.sources);       const &#91;&#91;row&#93;&#93; = await repository.artifact(         connection,       …</code>（长表达式，按本步完整代码继续） |

**本段进入 DraftPreviews.replay：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>fixed.binding</code>；<code>fixed.source</code>；<code>fixed.draft_cut</code>；<code>fixed.main_cut</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>canonical(binding)</code>；<code>binding.context</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{       state: &#x27;FIXED&#x27;,       preview_cut_ref: fixed.preview_cut_ref,       draft: wireContent(fixed.draft),       main_manifest_id: source.main_manifest_id,       observed_cut: wi …</code>（长表达式，按本步完整代码继续） |

**本段进入 DraftPreviews.body：**

**这一段为什么这样写：** 从真实持久Yjs提取指定工作区正文。

1. restore当前cut
2. 转PM JSON再normalizeContent
3. finally销毁doc


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>c</code> | <code>JavaScript 参数</code> | 预览事务连接<br>在此声明中的实际使用：<code>this.snapshots.restore(c, room)</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 已锁且已核身份的room行<br>在此声明中的实际使用：<code>this.snapshots.restore(c, room)</code> |
| <code>kind = &#x27;BRANCH&#x27;</code> | <code>带默认值的参数</code> | MAIN允许正式字体，默认BRANCH禁止<br>在此声明中的实际使用：<code>normalizeContent(fromPM(yDocToProsemirrorJSON(doc)), { workspaceKind: kind })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>normalizeContent(fromPM(yDocToProsemirrorJSON(doc)), { workspaceKind: kind })</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>context.target.purpose !== &#x27;PULL_MAIN&#x27; &amp;&amp;           Buffer.byteLength(JSON.stringify(fixed)) &gt; 24 &lt;&lt; 20</code>。条件成立时执行 <code>throw new RPCFailure(8, &#x27;DISCARD_TRANSFER_LIMIT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(8, &#x27;DISCARD_TRANSFER_LIMIT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.insertArtifact(c, &#x27;DRAFT_PREVIEW_FIXED&#x27;, context.operation_id, fixed)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return this.replay(fixed, binding);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.transactions.run(async (connection) =&gt; {       await lockSources(connection, input.context.sources);       const &#91;&#91;row&#93;&#93; = await repository.artifact(         connection …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>lockSources(connection, input.context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.artifact(         connection,         &#x27;DRAFT_PREVIEW_FIXED&#x27;,         input.context.operation_id,       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!row</code>。条件成立时执行 <code>throw new RPCFailure(5, &#x27;DRAFT_PREVIEW_NOT_FOUND&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(5, &#x27;DRAFT_PREVIEW_NOT_FOUND&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>Buffer.byteLength(JSON.stringify(fixed)) &gt; 24 &lt;&lt; 20</code>。条件成立时执行 <code>throw new RPCFailure(8, &#x27;DISCARD_TRANSFER_LIMIT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(8, &#x27;DISCARD_TRANSFER_LIMIT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return fixed;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>canonical(fixed.binding) !== canonical(binding)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DRAFT_PREVIEW_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;DRAFT_PREVIEW_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {       state: &#x27;FIXED&#x27;,       preview_cut_ref: fixed.preview_cut_ref,       draft: wireContent(fixed.draft),       main_manifest_id: source.main_manifest_id,       observed_ …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=8014:10073 -->
````javascript
        if (
          context.target.purpose !== 'PULL_MAIN' &&
          Buffer.byteLength(JSON.stringify(fixed)) > 24 << 20
        )
          throw new RPCFailure(8, 'DISCARD_TRANSFER_LIMIT');
        fixed.input_digest = hash(canonical(fixed)).toString('base64');
        await repository.insertArtifact(c, 'DRAFT_PREVIEW_FIXED', context.operation_id, fixed);
        return this.replay(fixed, binding);
      }),
    );
  }

  async fixed(input) {
    const binding = draftPreviewBinding(input);
    return this.transactions.run(async (connection) => {
      await lockSources(connection, input.context.sources);
      const [[row]] = await repository.artifact(
        connection,
        'DRAFT_PREVIEW_FIXED',
        input.context.operation_id,
      );
      if (!row) throw new RPCFailure(5, 'DRAFT_PREVIEW_NOT_FOUND');
      const fixed = json(row.payload);
      this.replay(fixed, binding);
      // 系统取得同一个固定事实；不会按当前房间重新生成正文、cut或digest。
      if (Buffer.byteLength(JSON.stringify(fixed)) > 24 << 20)
        throw new RPCFailure(8, 'DISCARD_TRANSFER_LIMIT');
      return fixed;
    });
  }

  replay(fixed, binding) {
    if (canonical(fixed.binding) !== canonical(binding))
      throw new RPCFailure(6, 'DRAFT_PREVIEW_ID_CONFLICT');
    const source = fixed.source,
      cuts = [fixed.draft_cut, ...fixed.main_cut];
    return {
      state: 'FIXED',
      preview_cut_ref: fixed.preview_cut_ref,
      draft: wireContent(fixed.draft),
      main_manifest_id: source.main_manifest_id,
      observed_cut: wireCut({
        document_id: binding.context.target.document_id,
        structure_id: source.structure_id,
        style_id: source.style_id,
        control_revision: source.control_revision,
        rooms: cuts,
        dependency_digest: draftPreviewCutDigest(binding.context.target.purpose, cuts),
      }),
      expires_unix_ms: fixed.expires_unix_ms,
      input_digest: Buffer.from(fixed.input_digest, 'base64'),
    };
  }

  async body(c, room, kind = 'BRANCH') {
````

## 第 6 步：DraftPreviews（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 255 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 等待 <code>this.snapshots.restore(c, room)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return normalizeContent(fromPM(yDocToProsemirrorJSON(doc)), { workspaceKind: kind });</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=10073:10279 -->
````javascript
    const doc = await this.snapshots.restore(c, room);
    try {
      return normalizeContent(fromPM(yDocToProsemirrorJSON(doc)), { workspaceKind: kind });
    } finally {
      doc.destroy();
    }
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/draft_previews.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
