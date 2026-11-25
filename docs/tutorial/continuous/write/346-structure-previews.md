# 分步编写 `services/collaboration/src/application/structure_previews.mjs`

先按 [C05 · 结构拆并与每个在线编辑器保存握手](../lessons/collaboration/05-structure-online.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/346-structure-previews.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在同一事务采样全篇MAIN和每个支线，持久化结构迁移预览。

MAIN与草稿分别取“最新”会得到不一致切点，无法证明确认时用户看到的范围。

写之前先按这条执行路线理解各段的连接：

1. 向Doc取已登记全篇管理范围
2. 锁来源/MAIN入口，复用mainCaptures.captureCurrent的同连接事务
3. 逐unit核支线room/epoch/lineage，存在则恢复真实尾部，不存在取固定基线
4. 排序联合cut、buildStructureMigration并限制实际base64 JSON 24MiB
5. 写STRUCTURE_PREVIEW_FIXED和10分钟有效期，重试只replay

本文件需要守住的边界：

- Doc范围与本地room集合必须完全对应，多余或重复unit/room都失败
- 只把明确正文冲突映射业务拒绝，范围/谱系未知不能冒充未提交

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/structure_previews.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/structure_previews.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/structure_previews.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/structure_previews.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:1004 -->
````javascript
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { canonical, fromPM, normalizeContent } from '../../../../packages/content/index.mjs';
import { structurePreviewBinding } from '../domain/structure_preview_protocol.mjs';
import { buildStructureMigration } from '../domain/structure_migration.mjs';
import { fixedDraft, draftLineage } from '../domain/draft_protocol.mjs';
import { storeContent } from '../domain/capture_protocol.mjs';
import { storedCut, wireCut, cutDigest } from '../domain/main_protocol.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { lockSources } from './admission_gates.mjs';
import { requireMainOpen } from './admission_gates.mjs';
import * as artifacts from '../repository/draft_transitions.mjs';
import * as repository from '../repository/structure_workspace.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';

````

## 第 2 步：StructurePreviews

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`export class StructurePreviews `。

**本段进入 StructurePreviews.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, snapshots, peers, mainCaptures })</code> |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, snapshots, peers, mainCaptures })</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, snapshots, peers, mainCaptures })</code> |
| <code>mainCaptures</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Object.assign(this, { transactions, snapshots, peers, mainCaptures })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 StructurePreviews.capture：**

**这一段为什么这样写：** 固定结构改动的整篇输入和迁移候选。

1. 读取权威范围
2. 同事务捕获MAIN与BRANCH
3. 生成迁移并散列
4. 保存后只返回引用/cut/digest


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | STRUCTURE_PREVIEW全篇准入绑定<br>在此声明中的实际使用：<code>structurePreviewBinding(input)</code>；<code>input.context</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>prior</code>；<code>this.transactions.run(async (c) =&gt; {       await lockSources(c, context.sources);       await requireMainOpen(c, context.target.document_id);       const &#91;&#91;old&#93;&#93; = await artifacts. …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 等待 <code>this.transactions.run(async (c) =&gt; {       await lockSources(c, context.sources);       const &#91;&#91;row&#93;&#93; = await artifacts.artifact(c, &#x27;STRUCTURE_PREVIEW_FIXED&#x27;, context.operation_id) …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>lockSources(c, context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>artifacts.artifact(c, &#x27;STRUCTURE_PREVIEW_FIXED&#x27;, context.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return row &amp;&amp; this.replay(json(row.payload), binding);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>prior</code>。条件成立时执行 <code>return prior;</code>。当前函数在这里结束。
- 等待 <code>this.peers.http(&#x27;/infra/structure/context&#x27;, binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return this.transactions.run(async (c) =&gt; {       await lockSources(c, context.sources);       await requireMainOpen(c, context.target.document_id);       const &#91;&#91;old&#93;&#93; = await art …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>lockSources(c, context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>requireMainOpen(c, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>artifacts.artifact(c, &#x27;STRUCTURE_PREVIEW_FIXED&#x27;, context.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>old</code>。条件成立时执行 <code>return this.replay(json(old.payload), binding);</code>。当前函数在这里结束。
- 在本段的内部函数内：等待 <code>this.mainCaptures.captureCurrent(         c,         expected,         source,         context.operation_id + &#x27;:STRUCTURE_PREVIEW_MAIN&#x27;,         binding,       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!Array.isArray(source.drafts) &#124;&#124; source.drafts.length !== main.units.length</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.lockBranchRooms(c, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>seen.has(unit.unit_id) &#124;&#124; !main.units.some((v) =&gt; v.unit_id === unit.unit_id)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 2 bytes=1004:3131 -->
````javascript
export class StructurePreviews {
  constructor(transactions, snapshots, peers, mainCaptures) {
    Object.assign(this, { transactions, snapshots, peers, mainCaptures });
  }

  async capture(input) {
    const binding = structurePreviewBinding(input),
      context = input.context;
    const prior = await this.transactions.run(async (c) => {
      await lockSources(c, context.sources);
      const [[row]] = await artifacts.artifact(c, 'STRUCTURE_PREVIEW_FIXED', context.operation_id);
      return row && this.replay(json(row.payload), binding);
    });
    if (prior) return prior;
    // Doc只提供已登记原号的完整管理范围。mTLS不能代替当前MANAGE或允许任意房间查询。
    const source = await this.peers.http('/infra/structure/context', binding);
    return this.transactions.run(async (c) => {
      await lockSources(c, context.sources);
      await requireMainOpen(c, context.target.document_id);
      const [[old]] = await artifacts.artifact(c, 'STRUCTURE_PREVIEW_FIXED', context.operation_id);
      if (old) return this.replay(json(old.payload), binding);
      const expected = {
        context,
        expected_manifest_id: source.manifest_id,
        expected_structure_id: context.target.structure_id,
        expected_style_id: source.style_id,
        expected_control_revision: context.target.workspace_epoch,
        usage: 'STRUCTURE_PREVIEW',
      };
      const main = await this.mainCaptures.captureCurrent(
        c,
        expected,
        source,
        context.operation_id + ':STRUCTURE_PREVIEW_MAIN',
        binding,
      );
      if (!Array.isArray(source.drafts) || source.drafts.length !== main.units.length)
        throw new RPCFailure(14, 'STRUCTURE_BRANCH_SCOPE_UNKNOWN');
      const [rooms] = await repository.lockBranchRooms(c, context.target.document_id);
      const seen = new Set(),
        cuts = [...main.cut.rooms],
        drafts = [];
      for (const unit of source.drafts) {
        if (seen.has(unit.unit_id) || !main.units.some((v) => v.unit_id === unit.unit_id))
          throw new RPCFailure(14, 'STRUCTURE_BRANCH_SCOPE_UNKNOWN');
````

## 第 3 步：StructurePreviews（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 62 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>matching.length &gt; 1</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>room</code>。
- 在本段的内部函数内：检查 <code>room.id !== unit.room_id &#124;&#124;             room.workspace_id !== unit.branch_id &#124;&#124;             room.epoch !== unit.workspace_epoch &#124;&#124;             room.state !== &#x27;OPEN&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>captures.lockRoomLineage(c, room.id, room.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!row &#124;&#124; canonical(draftLineage(json(row.payload))) !== canonical(unit.lineage)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_LINEAGE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_LINEAGE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>this.snapshots.restore(c, room, room.last_durable_seq)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=3131:4954 -->
````javascript
        seen.add(unit.unit_id);
        const matching = rooms.filter((r) => r.unit_id === unit.unit_id);
        if (matching.length > 1) throw new RPCFailure(14, 'STRUCTURE_BRANCH_SCOPE_UNKNOWN');
        const room = matching[0];
        let body = normalizeContent(unit.lineage.body),
          cut = { room_id: unit.room_id, workspace_epoch: unit.workspace_epoch, durable_seq: '0' };
        if (room) {
          if (
            room.id !== unit.room_id ||
            room.workspace_id !== unit.branch_id ||
            room.epoch !== unit.workspace_epoch ||
            room.state !== 'OPEN'
          )
            throw new RPCFailure(14, 'STRUCTURE_BRANCH_CHANGED');
          const [[row]] = await captures.lockRoomLineage(c, room.id, room.epoch);
          if (!row || canonical(draftLineage(json(row.payload))) !== canonical(unit.lineage))
            throw new RPCFailure(14, 'STRUCTURE_LINEAGE_CHANGED');
          cut = {
            room_id: room.id,
            workspace_epoch: room.epoch,
            durable_seq: room.last_durable_seq,
          };
          const doc = await this.snapshots.restore(c, room, room.last_durable_seq);
          try {
            body = normalizeContent(fromPM(yDocToProsemirrorJSON(doc)));
          } finally {
            doc.destroy();
          }
          cuts.push(cut);
        }
        const scoped = {
          ...context,
          target: {
            ...context.target,
            unit_id: unit.unit_id,
            branch_id: unit.branch_id,
            workspace_epoch: unit.workspace_epoch,
          },
        };
        drafts.push(
          fixedDraft(
            body,
            scoped,
            cut,
            unit.lineage,
            'structure-preview:' + context.operation_id + ':' + unit.unit_id,
          ),
        );
      }
````

## 第 4 步：StructurePreviews（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 111 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 StructurePreviews.replay：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>fixed.binding</code>；<code>fixed.preview_cut_ref</code>；<code>fixed.source</code>；<code>fixed.observed_cut</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>canonical(binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{       state: &#x27;FIXED&#x27;,       preview_cut_ref: fixed.preview_cut_ref,       main_manifest_id: fixed.source.manifest_id,       observed_cut: wireCut(fixed.observed_cut),       expir …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>rooms.some((r) =&gt; !seen.has(r.unit_id))</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>new Set(cuts.map((r) =&gt; r.room_id)).size !== cuts.length</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_ROOM_SCOPE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_ROOM_SCOPE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>conflicts.has(error.code)</code>。条件成立时执行 <code>throw new RPCFailure(9, error.code);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(9, error.code);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：执行 <code>throw error;</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>Buffer.byteLength(JSON.stringify(fixed)) &gt; 24 &lt;&lt; 20</code>。条件成立时执行 <code>throw new RPCFailure(8, &#x27;STRUCTURE_TRANSFER_LIMIT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(8, &#x27;STRUCTURE_TRANSFER_LIMIT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>artifacts.insertArtifact(c, &#x27;STRUCTURE_PREVIEW_FIXED&#x27;, context.operation_id, fixed)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return this.replay(fixed, binding);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=4954:7037 -->
````javascript
      if (rooms.some((r) => !seen.has(r.unit_id)))
        throw new RPCFailure(14, 'STRUCTURE_BRANCH_SCOPE_UNKNOWN');
      cuts.sort((a, b) =>
        BigInt(a.room_id) < BigInt(b.room_id) ? -1 : BigInt(a.room_id) > BigInt(b.room_id) ? 1 : 0,
      );
      if (new Set(cuts.map((r) => r.room_id)).size !== cuts.length)
        throw new RPCFailure(14, 'STRUCTURE_ROOM_SCOPE_UNKNOWN');
      const cut = { ...storedCut(main.cut), rooms: cuts, dependency_digest: cutDigest(cuts) };
      const fixed = {
        binding,
        source,
        main_units: main.units.map(storeContent),
        drafts,
        observed_cut: cut,
        // 冻结HTTP引用沿用OperationID字符/长度规则；全局唯一原号即可定位固定预览，不加非法冒号前缀。
        preview_cut_ref: context.operation_id,
        expires_unix_ms: String(Date.now() + 600000),
      };
      try {
        fixed.migration = buildStructureMigration(fixed);
      } catch (error) {
        // 只有明确的正文前置条件才拒绝原号；缺失范围/谱系或未知异常不能被当成业务未提交。
        const conflicts = new Set([
          'DRAFT_STRUCTURE_CHANGED',
          'STRUCTURE_DRAFT_CONFLICT',
          'STRUCTURE_LEVEL2_MISSING',
          'STRUCTURE_TITLE_INVALID',
          'STRUCTURE_TITLE_CHANGED',
          'STRUCTURE_MODE_CHANGED',
          'STRUCTURE_RANGE_OVERLAP',
          'STRUCTURE_BLOCK_ID_COLLISION',
          'STRUCTURE_CONTENT_LIMIT',
        ]);
        if (conflicts.has(error.code)) throw new RPCFailure(9, error.code);
        throw error;
      }
      // 按实际base64 JSON限制整体传输，失败在固定事实提交之前，不能返回截断范围。
      if (Buffer.byteLength(JSON.stringify(fixed)) > 24 << 20)
        throw new RPCFailure(8, 'STRUCTURE_TRANSFER_LIMIT');
      fixed.input_digest = hash(canonical(fixed)).toString('base64');
      await artifacts.insertArtifact(c, 'STRUCTURE_PREVIEW_FIXED', context.operation_id, fixed);
      return this.replay(fixed, binding);
    });
  }

  replay(fixed, binding) {
````

## 第 5 步：StructurePreviews（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 157 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 StructurePreviews.fixed：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>structurePreviewBinding(input)</code>；<code>input.context</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.transactions.run(async (c) =&gt; {       await lockSources(c, input.context.sources);       const &#91;&#91;row&#93;&#93; = await artifacts.artifact(         c,         &#x27;STRUCTURE_PREVIEW_FIXED&#x27; …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>canonical(fixed.binding) !== canonical(binding)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_PREVIEW_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_PREVIEW_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {       state: &#x27;FIXED&#x27;,       preview_cut_ref: fixed.preview_cut_ref,       main_manifest_id: fixed.source.manifest_id,       observed_cut: wireCut(fixed.observed_cut),      …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.transactions.run(async (c) =&gt; {       await lockSources(c, input.context.sources);       const &#91;&#91;row&#93;&#93; = await artifacts.artifact(         c,         &#x27;STRUCTURE_PREVIEW …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>lockSources(c, input.context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>artifacts.artifact(         c,         &#x27;STRUCTURE_PREVIEW_FIXED&#x27;,         input.context.operation_id,       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!row</code>。条件成立时执行 <code>throw new RPCFailure(5, &#x27;STRUCTURE_PREVIEW_NOT_FOUND&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(5, &#x27;STRUCTURE_PREVIEW_NOT_FOUND&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return value;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=7037:8073 -->
````javascript
    if (canonical(fixed.binding) !== canonical(binding))
      throw new RPCFailure(6, 'STRUCTURE_PREVIEW_ID_CONFLICT');
    return {
      state: 'FIXED',
      preview_cut_ref: fixed.preview_cut_ref,
      main_manifest_id: fixed.source.manifest_id,
      observed_cut: wireCut(fixed.observed_cut),
      expires_unix_ms: fixed.expires_unix_ms,
      input_digest: Buffer.from(fixed.input_digest, 'base64'),
    };
  }

  async fixed(input) {
    const binding = structurePreviewBinding(input);
    return this.transactions.run(async (c) => {
      await lockSources(c, input.context.sources);
      const [[row]] = await artifacts.artifact(
        c,
        'STRUCTURE_PREVIEW_FIXED',
        input.context.operation_id,
      );
      if (!row) throw new RPCFailure(5, 'STRUCTURE_PREVIEW_NOT_FOUND');
      const value = json(row.payload);
      this.replay(value, binding);
      // 仅供Doc系统取得同一个固定事实并保护资源，不接纳调用者提供的新正文或新切点。
      return value;
    });
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/structure_previews.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/05-structure-online.md)。
