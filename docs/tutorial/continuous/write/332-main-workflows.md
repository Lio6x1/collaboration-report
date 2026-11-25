# 分步编写 `services/collaboration/src/application/main_workflows.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/332-main-workflows.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现全篇MAIN捕获、提交准备屏障、成功安装和失败中止。

MAIN由多个独立room组成；必须把固定清单、可靠尾部和所有旧输出一起处理，避免半篇新旧混用。

写之前先按这条执行路线理解各段的连接：

1. MainCaptures按purpose限定usage，Doc提供固定manifest；同事务锁文档与全部MAIN，用实际Yjs尾部替换相应单元并生成全篇cut/digest
2. MainCommits.prepare核expected_cut与来源；结构变换先等所有在线编辑器握手，再持久PREPARED屏障
3. MainInstallations.reconcile重新查询Go终态和已提交清单，逐项核manifest_digest及units
4. 同事务退休旧MAIN、建新seq0房间/上下文、写install_receipt；MERGE才更新支线lineage，MAIN_CONTROL不得冒充接纳
5. 安装后发送结构导航并真实stopRooms才READY；REJECTED/CANCELLED只中止匹配原操作屏障

本文件需要守住的边界：

- READ_MAIN即使有固定结果也复核短期Doc准入，不能借私有证书无限重放正文
- 24MiB全篇传输上限应明确失败，不能截断units
- 同号安装重试优先读原receipt，不得覆盖后来的MAIN代次
- acceptLineage只换同epoch支线基线，不覆盖其Yjs草稿

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/main_workflows.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/main_workflows.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/main_workflows.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/main_workflows.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:1993 -->
````javascript
import { bad } from '../domain/shared_protocol.mjs';
import * as barriers from '../repository/main_workspace.mjs';
import { canonical } from '../../../../packages/content/index.mjs';
import { captureProvenance } from '../domain/capture_protocol.mjs';
import * as captures from '../repository/main_workspace.mjs';
import { cutDigest } from '../domain/main_protocol.mjs';
import { fixedContentBinding } from '../domain/merge_protocol.mjs';
import { fromPM } from '../../../../packages/content/index.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import * as installations from '../repository/main_workspace.mjs';
import { json } from '../domain/shared_protocol.mjs';
import * as lineages from '../repository/candidate_artifacts.mjs';
import { lockSources } from './admission_gates.mjs';
import { matchesMainWorkspace } from '../domain/main_workspace.mjs';
import { normalizeContent } from '../../../../packages/content/index.mjs';
import { prepareBinding } from '../domain/main_protocol.mjs';
import { preparedMainSource } from '../domain/main_workspace.mjs';
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import * as repository from '../repository/main_workspace.mjs';
import { requireDocumentBodyPresent } from './account_document_cleanup.mjs';
import { requireDraftMainOpen } from './admission_gates.mjs';
import { requireMainOpen } from './admission_gates.mjs';
import { resourceIds } from '../../../../packages/content/index.mjs';
import * as rooms from '../repository/room_access.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { schema } from '../../../../packages/content/index.mjs';
import { storeContent } from '../domain/capture_protocol.mjs';
import { toPM } from '../../../../packages/content/index.mjs';
import { validID } from '../domain/shared_protocol.mjs';
import { wireContent } from '../domain/capture_protocol.mjs';
import { wireCut } from '../domain/main_protocol.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';

````

## 第 2 步：MainCaptures

插入位置：第 1 步代码之后，紧接追加；完成稿约第 34 行。

来源 main_captures.mjs：原业务声明与函数体保留。

本步声明：`export class MainCaptures `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>snapshots</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>peers</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |

**本段进入 MainCaptures.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 MainCaptures.revision：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input?.document_id</code>；<code>input.document_id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.transactions.run(async (connection) =&gt; {       const &#91;rooms&#93; = await captures.mainReadingRevision(connection, input.document_id);       return hash(Buffer.from(canonical(rooms))).toString(&#x27;hex&#x27;);     })</code> |

**本段进入 MainCaptures.capture：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.context</code>；<code>input.usage</code>；<code>this.captureCurrent(connection, input, source, key, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>prior</code>；<code>this.transactions.run((connection) =&gt;       this.captureCurrent(connection, input, source, key, binding),     )</code> |

**跟着本段代码走一遍：**

- 检查 <code>typeof input?.document_id !== &#x27;string&#x27; &#124;&#124;       !/^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(input.document_id) &#124;&#124;       BigInt(input.document_id) &gt; 18446744073709551615n</code>。条件成立时执行 <code>throw bad(&#x27;MAIN_REVISION_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MAIN_REVISION_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return this.transactions.run(async (connection) =&gt; {       const &#91;rooms&#93; = await captures.mainReadingRevision(connection, input.document_id);       return hash(Buffer.from(canonical(rooms))).toString(&#x27;hex&#x27;);     });</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>captures.mainReadingRevision(connection, input.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return hash(Buffer.from(canonical(rooms))).toString(&#x27;hex&#x27;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!validID(context?.operation_id) &#124;&#124;       !usagePattern?.test(input.usage) &#124;&#124;       Buffer.from(context.request_fingerprint ?? &#91;&#93;).length !== 32</code>。条件成立时执行 <code>throw bad(&#x27;MAIN_CAPTURE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MAIN_CAPTURE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 2 bytes=1993:3666 -->
````javascript
// 来源 main_captures.mjs：原业务声明与函数体保留。

export class MainCaptures {
  constructor(transactions, snapshots, peers) {
    this.transactions = transactions;
    this.snapshots = snapshots;
    this.peers = peers;
  }

  async revision(input) {
    if (
      typeof input?.document_id !== 'string' ||
      !/^[1-9][0-9]{0,19}$/.test(input.document_id) ||
      BigInt(input.document_id) > 18446744073709551615n
    )
      throw bad('MAIN_REVISION_INVALID');
    // 仅向Doc证书返回不透明摘要。它只是变化提示，不能作为正文读取或编辑准入。
    return this.transactions.run(async (connection) => {
      const [rooms] = await captures.mainReadingRevision(connection, input.document_id);
      return hash(Buffer.from(canonical(rooms))).toString('hex');
    });
  }

  async capture(input) {
    const context = input.context;
    const purpose = context?.target?.purpose;
    const usages = {
      MERGE: /^MERGE_ATTEMPT:[1-9][0-9]{0,19}$/,
      MAIN_CONTROL: /^MAIN_CONTROL:[1-9][0-9]{0,19}$/,
      READ_MAIN: /^MAIN_READ_VIEW:1$/,
      PUBLICATION: /^RELEASE_CREATE:1$/,
    };
    const usagePattern = Object.hasOwn(usages, purpose) ? usages[purpose] : undefined;
    if (
      !validID(context?.operation_id) ||
      !usagePattern?.test(input.usage) ||
      Buffer.from(context.request_fingerprint ?? []).length !== 32
    )
      throw bad('MAIN_CAPTURE_INVALID');
    const key = context.operation_id + ':' + input.usage;
    const binding = {
      ...input,
      context: {
        ...context,
        request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
      },
    };
````

## 第 3 步：MainCaptures（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 81 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 MainCaptures.captureCurrent：**

**这一段为什么这样写：** 在一个事务视图中取得整篇可靠MAIN内容。

1. 核Doc清单与期望版本
2. 核每个room的物理来源及unit唯一性
3. 逐unit恢复可靠尾部并重建provenance
4. 固定全篇cut/units/digest


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 调用方已经开始的事务连接<br>在此声明中的实际使用：<code>lockSources(connection, context.sources)</code>；<code>requireMainOpen(connection, context.target.document_id)</code>；<code>captures.lockCapture(connection, key)</code>；<code>barriers.lockMainRooms(connection, context.target.document_id)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>JavaScript 参数</code> | 带expected_manifest/structure/style/control的固定捕获输入<br>在此声明中的实际使用：<code>input.context</code>；<code>input.expected_manifest_id</code>；<code>input.expected_structure_id</code>；<code>input.expected_style_id</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>source</code> | <code>JavaScript 参数</code> | Doc为原操作提供的正式清单来源<br>在此声明中的实际使用：<code>source.manifest_id</code>；<code>source.structure_id</code>；<code>source.style_id</code>；<code>source.control_revision</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>key</code> | <code>JavaScript 参数</code> | operation_id:usage固定业务键<br>在此声明中的实际使用：<code>captures.lockCapture(connection, key)</code>；<code>key + &#x27;:&#x27;</code>；<code>captures.insertCapture(connection, key, context.operation_id, { ...result, digest })</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 已规范化的完整请求绑定<br>在此声明中的实际使用：<code>this.replay(json(stored.payload), binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.replay(json(stored.payload), binding)</code>；<code>{       state: &#x27;FIXED&#x27;,       cut: wireCut(cut),       units: units.map(wireContent),       result_digest: Buffer.from(digest, &#x27;base64&#x27;),     }</code> |

**跟着本段代码走一遍：**

- 等待 <code>this.transactions.run(async (connection) =&gt; {       await lockSources(connection, context.sources);       const &#91;&#91;stored&#93;&#93; = await captures.lockCapture(connection, key);       retu …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>lockSources(connection, context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>captures.lockCapture(connection, key)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return stored ? this.replay(json(stored.payload), binding) : null;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>prior</code>。
- 检查 <code>purpose === &#x27;READ_MAIN&#x27;</code>。
- 等待 <code>this.peers.http(&#x27;/infra/main/context&#x27;, binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return prior;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.peers.http(&#x27;/infra/main/context&#x27;, binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return this.transactions.run((connection) =&gt;       this.captureCurrent(connection, input, source, key, binding),     );</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>lockSources(connection, context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>requireMainOpen(connection, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>captures.lockCapture(connection, key)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>stored</code>。条件成立时执行 <code>return this.replay(json(stored.payload), binding);</code>。当前函数在这里结束。
- 检查 <code>source.manifest_id !== input.expected_manifest_id &#124;&#124;       source.structure_id !== input.expected_structure_id &#124;&#124;       source.style_id !== input.expected_style_id &#124;&#124;       source. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(9, &#x27;MAIN_SOURCE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(9, &#x27;MAIN_SOURCE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>barriers.lockMainRooms(connection, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>barriers.mainContext(connection, room.id, room.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!matchesMainWorkspace(current, room, source) &#124;&#124; byUnit.has(room.unit_id)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_CONTEXT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;MAIN_CONTEXT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 3 bytes=3666:5998 -->
````javascript
    const prior = await this.transactions.run(async (connection) => {
      await lockSources(connection, context.sources);
      const [[stored]] = await captures.lockCapture(connection, key);
      return stored ? this.replay(json(stored.payload), binding) : null;
    });
    if (prior) {
      // 读取没有后台续期责任：原固定结果存在也须核验原短期Doc准入，不能过期后借mTLS重放正文。
      if (purpose === 'READ_MAIN') await this.peers.http('/infra/main/context', binding);
      return prior;
    }
    // 不持Node SQL锁调用Doc；固定清单只从Doc已登记原操作取得，不能靠调用者填的正文采样。
    const source = await this.peers.http('/infra/main/context', binding);
    return this.transactions.run((connection) =>
      this.captureCurrent(connection, input, source, key, binding),
    );
  }

  // 全篇预览调用方已取得自己的原准入来源；共享同一事务快照，不能另开池破坏MAIN/BRANCH切点的一致性。
  async captureCurrent(connection, input, source, key, binding) {
    const context = input.context;
    await lockSources(connection, context.sources);
    await requireMainOpen(connection, context.target.document_id);
    const [[stored]] = await captures.lockCapture(connection, key);
    if (stored) return this.replay(json(stored.payload), binding);
    if (
      source.manifest_id !== input.expected_manifest_id ||
      source.structure_id !== input.expected_structure_id ||
      source.style_id !== input.expected_style_id ||
      source.control_revision !== input.expected_control_revision ||
      !Array.isArray(source.units)
    )
      throw new RPCFailure(9, 'MAIN_SOURCE_CHANGED');
    const [rooms] = await barriers.lockMainRooms(connection, context.target.document_id);
    const byUnit = new Map();
    for (const room of rooms) {
      const [[row]] = await barriers.mainContext(connection, room.id, room.epoch);
      const current = row && json(row.payload);
      if (!matchesMainWorkspace(current, room, source) || byUnit.has(room.unit_id))
        throw new RPCFailure(14, 'MAIN_CONTEXT_UNKNOWN');
      byUnit.set(room.unit_id, room);
    }
    const cuts = rooms.map((room) => ({
      room_id: room.id,
      workspace_epoch: room.epoch,
      durable_seq: room.last_durable_seq,
    }));
````

## 第 4 步：MainCaptures（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 127 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>seen.has(unit.unit_id)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_MANIFEST_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;MAIN_MANIFEST_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>original.unit_id !== unit.unit_id &#124;&#124;         original.document_id !== context.target.document_id &#124;&#124;         original.structure_id !== source.structure_id &#124;&#124;         !hash(original. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_MANIFEST_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;MAIN_MANIFEST_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>room</code>。
- 等待 <code>this.snapshots.restore(connection, room, room.last_durable_seq)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>bytes.length &gt; 16 &lt;&lt; 20</code>。条件成立时执行 <code>throw new RPCFailure(8, &#x27;MAIN_CONTENT_TOO_LARGE&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(8, &#x27;MAIN_CONTENT_TOO_LARGE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 4 bytes=5998:8474 -->
````javascript
    const cut = {
      document_id: context.target.document_id,
      structure_id: source.structure_id,
      style_id: source.style_id,
      control_revision: source.control_revision,
      rooms: cuts,
      dependency_digest: cutDigest(cuts),
    };
    let transfer = 0;
    const units = [];
    const seen = new Set();
    for (const unit of source.units) {
      if (seen.has(unit.unit_id)) throw new RPCFailure(14, 'MAIN_MANIFEST_INVALID');
      seen.add(unit.unit_id);
      const original = wireContent(unit.content);
      if (
        original.unit_id !== unit.unit_id ||
        original.document_id !== context.target.document_id ||
        original.structure_id !== source.structure_id ||
        !hash(original.body_json_utf8).equals(original.body_hash)
      )
        throw new RPCFailure(14, 'MAIN_MANIFEST_INVALID');
      const baseline = normalizeContent(JSON.parse(original.body_json_utf8), {
        workspaceKind: 'MAIN',
      });
      const room = byUnit.get(unit.unit_id);
      let body = baseline;
      if (room) {
        const doc = await this.snapshots.restore(connection, room, room.last_durable_seq);
        try {
          body = normalizeContent(fromPM(yDocToProsemirrorJSON(doc)), { workspaceKind: 'MAIN' });
        } finally {
          doc.destroy();
        }
      }
      const bytes = Buffer.from(canonical(body));
      if (bytes.length > 16 << 20) throw new RPCFailure(8, 'MAIN_CONTENT_TOO_LARGE');
      const provenance = JSON.parse(original.provenance_json_utf8);
      const content = {
        ...original,
        branch_id: '0',
        capture_id: key + ':' + unit.unit_id,
        operation_id: context.operation_id,
        workspace_epoch: source.control_revision,
        cut_json_utf8: room
          ? Buffer.from(canonical(cuts.find((item) => item.room_id === room.id)))
          : original.cut_json_utf8,
        body_json_utf8: bytes,
        body_hash: hash(bytes),
        byte_size: String(bytes.length),
        resource_ids: resourceIds(body),
        provenance_json_utf8: Buffer.from(
          canonical(
            captureProvenance(
              body,
              {
                body: baseline,
                lineage_id: provenance.lineage_id,
                base_manifest_id: source.manifest_id,
                base_version_id: unit.content_version_id,
                provenance,
              },
              context.operation_id,
            ),
          ),
        ),
      };
````

## 第 5 步：MainCaptures（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 194 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 MainCaptures.replay：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>stored</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>stored.binding</code>；<code>stored.cut</code>；<code>stored.units</code>；<code>stored.digest</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>canonical(binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{       state: &#x27;FIXED&#x27;,       cut: wireCut(stored.cut),       units: stored.units.map(wireContent),       result_digest: Buffer.from(stored.digest, &#x27;base64&#x27;),     }</code> |

**跟着本段代码走一遍：**

- 检查 <code>transfer &gt; 24 &lt;&lt; 20</code>。条件成立时执行 <code>throw new RPCFailure(8, &#x27;MAIN_VIEW_TRANSFER_LIMIT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(8, &#x27;MAIN_VIEW_TRANSFER_LIMIT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>&#91;...byUnit.keys()&#93;.some((unit) =&gt; !seen.has(unit))</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_MANIFEST_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;MAIN_MANIFEST_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>captures.insertCapture(connection, key, context.operation_id, { ...result, digest })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return {       state: &#x27;FIXED&#x27;,       cut: wireCut(cut),       units: units.map(wireContent),       result_digest: Buffer.from(digest, &#x27;base64&#x27;),     };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>canonical(stored.binding) !== canonical(binding)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_CAPTURE_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;MAIN_CAPTURE_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {       state: &#x27;FIXED&#x27;,       cut: wireCut(stored.cut),       units: stored.units.map(wireContent),       result_digest: Buffer.from(stored.digest, &#x27;base64&#x27;),     };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=8474:9675 -->
````javascript
      transfer +=
        bytes.length + content.provenance_json_utf8.length + content.cut_json_utf8.length + 4096;
      // 单次全篇RPC保留32MiB传输余量；超限明确失败，不截断全篇清单或声称覆盖最新主线。
      if (transfer > 24 << 20) throw new RPCFailure(8, 'MAIN_VIEW_TRANSFER_LIMIT');
      units.push(storeContent(content));
    }
    if ([...byUnit.keys()].some((unit) => !seen.has(unit)))
      throw new RPCFailure(14, 'MAIN_MANIFEST_INVALID');
    const result = { binding, cut, units };
    const digest = hash(Buffer.from(canonical({ cut, units }))).toString('base64');
    await captures.insertCapture(connection, key, context.operation_id, { ...result, digest });
    return {
      state: 'FIXED',
      cut: wireCut(cut),
      units: units.map(wireContent),
      result_digest: Buffer.from(digest, 'base64'),
    };
  }

  replay(stored, binding) {
    if (canonical(stored.binding) !== canonical(binding))
      throw new RPCFailure(6, 'MAIN_CAPTURE_ID_CONFLICT');
    return {
      state: 'FIXED',
      cut: wireCut(stored.cut),
      units: stored.units.map(wireContent),
      result_digest: Buffer.from(stored.digest, 'base64'),
    };
  }
}

````

## 第 6 步：MainCommits

插入位置：第 5 步代码之后，紧接追加；完成稿约第 225 行。

来源 main_commits.mjs：原业务声明与函数体保留。

本步声明：`export class MainCommits `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>peers</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |
| <code>structurePreparations</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.structurePreparations = structurePreparations</code> |
| <code>outputs</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.outputs = outputs</code> |

**本段进入 MainCommits.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |
| <code>structurePreparations</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.structurePreparations = structurePreparations</code> |
| <code>outputs</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.outputs = outputs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 MainCommits.prepare：**

**这一段为什么这样写：** 证明将提交的正文仍对应已预览切点。

1. 结构操作先完成浏览器保存准备
2. 锁来源及文档入口
3. 比较可靠MAIN cut与expected_cut
4. 相同才存PREPARED，不同返回CHANGED


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | context、barrier_generation和expected_cut<br>在此声明中的实际使用：<code>prepareBinding(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.reply(binding, &#x27;PROCESSING&#x27;)</code>；<code>reply</code> |

**跟着本段代码走一遍：**

- 等待 <code>this.peers.http(&#x27;/infra/structure/confirmation&#x27;, binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>authority.structure_preview</code>。
- 等待 <code>this.transactions.run((c) =&gt;         this.structurePreparations.plan(c, binding, authority.structure_preview),       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>state === &#x27;WAITING&#x27;</code>。条件成立时执行 <code>return this.reply(binding, &#x27;PROCESSING&#x27;);</code>。当前函数在这里结束。
- 检查 <code>state === &#x27;FAILED&#x27;</code>。
- 执行 <code>throw new RPCFailure(9, &#x27;STRUCTURE_EDITORS_NOT_READY&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.transactions.run(async (connection) =&gt; {         await lockSources(connection, binding.context.sources);         await repository.lockDocument(connection, expected.document_id …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>lockSources(connection, binding.context.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.lockDocument(connection, expected.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>requireDraftMainOpen(connection, expected.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.lockBarrier(connection, expected.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>prior &amp;&amp; !&#91;&#x27;INSTALLED&#x27;, &#x27;ABORTED&#x27;&#93;.includes(prior.state)</code>。
- 在本段的内部函数内：检查 <code>prior.operation_id !== binding.context.operation_id</code>。条件成立时执行 <code>return this.reply(binding, &#x27;PROCESSING&#x27;);</code>。当前函数在这里结束。
- 在本段的内部函数内：检查 <code>prior.generation !== binding.barrier_generation &#124;&#124;             canonical(stored.binding) !== canonical(binding)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_BARRIER_CONFLICT&#x27;);</code>。转入外层异常处理路径。

<!-- write-step: 6 bytes=9675:11772 -->
````javascript
// 来源 main_commits.mjs：原业务声明与函数体保留。

export class MainCommits {
  constructor(transactions, peers, structurePreparations, outputs) {
    this.transactions = transactions;
    this.peers = peers;
    this.structurePreparations = structurePreparations;
    this.outputs = outputs;
  }

  async prepare(input) {
    const binding = prepareBinding(input),
      expected = binding.expected_cut;
    // 权威原确认在SQL锁外查询；Node不接受调用方另传候选或假称预览引用。
    const authority =
      binding.context.target.purpose === 'MAIN_CONTROL'
        ? await this.peers.http('/infra/structure/confirmation', binding)
        : { structure_preview: null };
    if (authority.structure_preview) {
      const plan = await this.transactions.run((c) =>
        this.structurePreparations.plan(c, binding, authority.structure_preview),
      );
      // 等待浏览器不占SQL事务/RPC长等待。持久任务以同一原号续接，不能换预览或采新cut。
      const state = this.outputs.prepareStructure(binding, plan);
      if (state === 'WAITING') return this.reply(binding, 'PROCESSING');
      if (state === 'FAILED') {
        this.outputs.finishStructure(binding);
        throw new RPCFailure(9, 'STRUCTURE_EDITORS_NOT_READY');
      }
    }
    let reply;
    try {
      reply = await this.transactions.run(async (connection) => {
        await lockSources(connection, binding.context.sources);
        await repository.lockDocument(connection, expected.document_id);
        await requireDraftMainOpen(connection, expected.document_id);
        const [[prior]] = await repository.lockBarrier(connection, expected.document_id);
        if (prior && !['INSTALLED', 'ABORTED'].includes(prior.state)) {
          if (prior.operation_id !== binding.context.operation_id)
            return this.reply(binding, 'PROCESSING');
          const stored = json(prior.cut_vector);
          if (
            prior.generation !== binding.barrier_generation ||
            canonical(stored.binding) !== canonical(binding)
          )
````

## 第 7 步：MainCommits（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 270 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;MAIN_BARRIER_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return this.reply(             binding,             prior.state === &#x27;PREPARED&#x27; ? &#x27;PREPARED&#x27; : &#x27;PROCESSING&#x27;,             stored.prepared_cut,           );</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>prior &amp;&amp; BigInt(prior.generation) &gt;= BigInt(binding.barrier_generation)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_BARRIER_GENERATION_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;MAIN_BARRIER_GENERATION_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.lockMainRooms(connection, expected.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.mainContext(connection, room.id, room.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!context</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_CONTEXT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;MAIN_CONTEXT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>current.control_revision !== expected.control_revision</code>。
- 在本段的内部函数内：检查 <code>capturedSource === undefined</code>。
- 在本段的内部函数内：等待 <code>captures.preparationFacts(                 connection,                 binding.context.operation_id,               )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!capturedSource</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_PREPARE_CAPTURE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;MAIN_PREPARE_CAPTURE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>!matchesMainWorkspace(current, room, {                 ...capturedSource,                 workspace_epoch: originalRoom?.workspace_epoch,               })</code>。条件成立时执行 <code>return this.reply(binding, &#x27;CHANGED&#x27;, cut);</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=11772:13855 -->
````javascript
            throw new RPCFailure(6, 'MAIN_BARRIER_CONFLICT');
          return this.reply(
            binding,
            prior.state === 'PREPARED' ? 'PREPARED' : 'PROCESSING',
            stored.prepared_cut,
          );
        }
        if (prior && BigInt(prior.generation) >= BigInt(binding.barrier_generation))
          throw new RPCFailure(6, 'MAIN_BARRIER_GENERATION_CHANGED');
        const [rows] = await repository.lockMainRooms(connection, expected.document_id);
        const rooms = rows.map((room) => ({
          room_id: room.id,
          workspace_epoch: room.epoch,
          durable_seq: room.last_durable_seq,
        }));
        const cut = { ...expected, rooms, dependency_digest: cutDigest(rooms) };
        let capturedSource;
        for (const room of rows) {
          const [[context]] = await repository.mainContext(connection, room.id, room.epoch);
          if (!context) throw new RPCFailure(14, 'MAIN_CONTEXT_UNKNOWN');
          const current = json(context.payload);
          if (current.control_revision !== expected.control_revision) {
            // 元数据没有安装新房间：只允许同原操作真正捕获过的物理代次，不能放宽成任意较小epoch。
            if (capturedSource === undefined) {
              const [facts] = await captures.preparationFacts(
                connection,
                binding.context.operation_id,
              );
              capturedSource = preparedMainSource(
                binding,
                facts.map((fact) => ({
                  ...fact,
                  context: json(fact.context),
                  cut: json(fact.cut),
                })),
              );
            }
            if (!capturedSource) throw new RPCFailure(14, 'MAIN_PREPARE_CAPTURE_UNKNOWN');
            const originalRoom = expected.rooms.find((item) => item.room_id === room.id);
            if (
              !matchesMainWorkspace(current, room, {
                ...capturedSource,
                workspace_epoch: originalRoom?.workspace_epoch,
              })
            )
````

## 第 8 步：MainCommits（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 315 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 MainCommits.reply：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.context</code>；<code>binding.barrier_generation</code> |
| <code>state</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>return {       state,       operation_id: binding.context.operation_id,       barrier_generation: binding.barrier_generation,       ...(cut ? { prepared_cut: wireCut(cut), digest:  …</code>（长表达式，按本步完整代码继续） |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>return {       state,       operation_id: binding.context.operation_id,       barrier_generation: binding.barrier_generation,       ...(cut ? { prepared_cut: wireCut(cut), digest:  …</code>（长表达式，按本步完整代码继续）；<code>wireCut(cut)</code>；<code>canonical(cut)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{       state,       operation_id: binding.context.operation_id,       barrier_generation: binding.barrier_generation,       ...(cut ? { prepared_cut: wireCut(cut), digest: hash(Bu …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的内部函数内：此处通过 <code>return this.reply(binding, &#x27;CHANGED&#x27;, cut);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>current.structure_id !== expected.structure_id &#124;&#124;             current.style_id !== expected.style_id &#124;&#124;             current.control_revision !== room.epoch</code>。条件成立时执行 <code>return this.reply(binding, &#x27;CHANGED&#x27;, cut);</code>。当前函数在这里结束。
- 在本段的内部函数内：检查 <code>authority.structure_preview</code>。
- 在本段的内部函数内：等待 <code>this.structurePreparations.prepare(             connection,             binding,             authority.structure_preview,             rows,           )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>prepared.state === &#x27;PREPARED&#x27;</code>。
- 在本段的内部函数内：等待 <code>repository.prepareBarrier(               connection,               expected.document_id,               binding,               prepared.cut,             )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return this.reply(binding, prepared.state, prepared.cut);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>canonical(cut) !== canonical(expected)</code>。条件成立时执行 <code>return this.reply(binding, &#x27;CHANGED&#x27;, cut);</code>。当前函数在这里结束。
- 在本段的内部函数内：等待 <code>repository.prepareBarrier(connection, expected.document_id, binding, cut)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return this.reply(binding, &#x27;PREPARED&#x27;, cut);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>authority.structure_preview</code>。
- 执行 <code>throw error;</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>authority.structure_preview &amp;&amp; reply.state !== &#x27;PREPARED&#x27;</code>。
- 此处通过 <code>return reply;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return {       state,       operation_id: binding.context.operation_id,       barrier_generation: binding.barrier_generation,       ...(cut ? { prepared_cut: wireCut(cut), digest:  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=13855:15612 -->
````javascript
              return this.reply(binding, 'CHANGED', cut);
          } else if (
            current.structure_id !== expected.structure_id ||
            current.style_id !== expected.style_id ||
            current.control_revision !== room.epoch
          )
            return this.reply(binding, 'CHANGED', cut);
        }
        if (authority.structure_preview) {
          const prepared = await this.structurePreparations.prepare(
            connection,
            binding,
            authority.structure_preview,
            rows,
          );
          if (prepared.state === 'PREPARED')
            await repository.prepareBarrier(
              connection,
              expected.document_id,
              binding,
              prepared.cut,
            );
          return this.reply(binding, prepared.state, prepared.cut);
        }
        if (canonical(cut) !== canonical(expected)) return this.reply(binding, 'CHANGED', cut);
        // 全部已提交MAIN尾部已锁定，新OPEN/UPDATE在同一文档锁后看到此持久屏障；BRANCH不参加。
        await repository.prepareBarrier(connection, expected.document_id, binding, cut);
        return this.reply(binding, 'PREPARED', cut);
      });
    } catch (error) {
      if (authority.structure_preview) this.outputs.finishStructure(binding);
      throw error;
    }
    if (authority.structure_preview && reply.state !== 'PREPARED')
      this.outputs.finishStructure(binding);
    return reply;
  }

  reply(binding, state, cut) {
    return {
      state,
      operation_id: binding.context.operation_id,
      barrier_generation: binding.barrier_generation,
      ...(cut ? { prepared_cut: wireCut(cut), digest: hash(Buffer.from(canonical(cut))) } : {}),
    };
  }
}

````

## 第 9 步：MainInstallations

插入位置：第 8 步代码之后，紧接追加；完成稿约第 363 行。

来源 main_installations.mjs：原业务声明与函数体保留。

本步声明：`export class MainInstallations `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>snapshots</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>peers</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |
| <code>outputs</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.outputs = outputs</code> |
| <code>structurePreparations</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.structurePreparations = structurePreparations</code> |
| <code>structureInstallations</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.structureInstallations = structureInstallations</code> |

**本段进入 MainInstallations.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |
| <code>outputs</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.outputs = outputs</code> |
| <code>structurePreparations</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.structurePreparations = structurePreparations</code> |
| <code>structureInstallations</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.structureInstallations = structureInstallations</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 MainInstallations.reconcile：**

**这一段为什么这样写：** 将Go已经决定的清单安装到Node并排空旧输出。

1. 重新查权威原号终态
2. 比较清单摘要与完整units
3. 同事务换代并保存回执
4. 关闭旧socket后返回READY


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 带authoritative_result、manifest_id/digest、generation与units的安装请求<br>在此声明中的实际使用：<code>input.context</code>；<code>input.authoritative_result</code>；<code>input.barrier_generation</code>；<code>this.abort(input)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.abort(input)</code>；<code>installed</code> |

**跟着本段代码走一遍：**

- 检查 <code>!context?.target &#124;&#124;       !&#91;&#x27;MERGE&#x27;, &#x27;MAIN_CONTROL&#x27;&#93;.includes(context.target.purpose) &#124;&#124;       !supplied &#124;&#124;       !input.barrier_generation</code>。条件成立时执行 <code>throw bad(&#x27;MAIN_RECONCILE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MAIN_RECONCILE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.peers.rpc(&#x27;GetOperationResult&#x27;, {       operation_id: context.operation_id,       target: context.target,       request_fingerprint: context.request_fingerprint,     })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!&#91;&#x27;SUCCEEDED&#x27;, &#x27;REJECTED&#x27;, &#x27;CANCELLED&#x27;&#93;.includes(result.state)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_RESULT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;MAIN_RESULT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>result.operation_id !== supplied.operation_id &#124;&#124;       result.state !== supplied.state &#124;&#124;       !Buffer.from(result.result_digest).equals(Buffer.from(supplied.result_digest)) &#124;&#124;    …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_RESULT_CONFLICT&#x27;);</code>。转入外层异常处理路径。

<!-- write-step: 9 bytes=15612:17173 -->
````javascript
// 来源 main_installations.mjs：原业务声明与函数体保留。

export class MainInstallations {
  constructor(
    transactions,
    snapshots,
    peers,
    outputs,
    structurePreparations,
    structureInstallations,
  ) {
    this.transactions = transactions;
    this.snapshots = snapshots;
    this.peers = peers;
    this.outputs = outputs;
    this.structurePreparations = structurePreparations;
    this.structureInstallations = structureInstallations;
  }

  async reconcile(input) {
    const context = input.context,
      supplied = input.authoritative_result;
    if (
      !context?.target ||
      !['MERGE', 'MAIN_CONTROL'].includes(context.target.purpose) ||
      !supplied ||
      !input.barrier_generation
    )
      throw bad('MAIN_RECONCILE_INVALID');
    // 即使原用户已经撤权，系统也只安装Go原号的既成事实；传入的SUCCEEDED本身没有权威性。
    const result = await this.peers.rpc('GetOperationResult', {
      operation_id: context.operation_id,
      target: context.target,
      request_fingerprint: context.request_fingerprint,
    });
    if (!['SUCCEEDED', 'REJECTED', 'CANCELLED'].includes(result.state))
      throw new RPCFailure(14, 'MAIN_RESULT_UNKNOWN');
    if (
      result.operation_id !== supplied.operation_id ||
      result.state !== supplied.state ||
      !Buffer.from(result.result_digest).equals(Buffer.from(supplied.result_digest)) ||
      !Buffer.from(result.fixed_result_json_utf8).equals(
        Buffer.from(supplied.fixed_result_json_utf8),
      )
    )
````

## 第 10 步：MainInstallations（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 408 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 执行 <code>throw new RPCFailure(6, &#x27;MAIN_RESULT_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>result.state !== &#x27;SUCCEEDED&#x27;</code>。条件成立时执行 <code>return this.abort(input);</code>。当前函数在这里结束。
- 等待 <code>this.peers.http(&#x27;/infra/main/committed&#x27;, {       operation_id: context.operation_id,       target: context.target,       request_fingerprint: Buffer.from(context.request_fingerprint).toString(&#x27;base64&#x27;),     })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>authority.manifest_id !== input.manifest_id &#124;&#124;       authority.generation !== input.barrier_generation &#124;&#124;       !digest.equals(Buffer.from(input.manifest_digest)) &#124;&#124;       !digest. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_MANIFEST_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;MAIN_MANIFEST_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.transactions.run(async (connection) =&gt; {       await barriers.lockDocument(connection, context.target.document_id);       await requireDocumentBodyPresent(connection, context. …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>barriers.lockDocument(connection, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>requireDocumentBodyPresent(connection, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>installations.lockReceipt(connection, context.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>receipt</code>。
- 在本段的内部函数内：检查 <code>receipt.generation !== input.barrier_generation &#124;&#124;           receipt.manifest_id !== input.manifest_id &#124;&#124;           !receipt.manifest_digest.equals(digest)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_INSTALL_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;MAIN_INSTALL_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return this.receiptReply(receipt);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>barriers.lockBarrier(connection, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 10 bytes=17173:19246 -->
````javascript
      throw new RPCFailure(6, 'MAIN_RESULT_CONFLICT');
    if (result.state !== 'SUCCEEDED') return this.abort(input);
    const authority = await this.peers.http('/infra/main/committed', {
      operation_id: context.operation_id,
      target: context.target,
      request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
    });
    const units = authority.units.map((unit) => ({ ...unit, content: wireContent(unit.content) }));
    const digest = hash(
      Buffer.from(
        canonical({
          manifest_id: authority.manifest_id,
          units: units.map((unit) => ({
            unit_id: unit.unit_id,
            content_version_id: unit.content_version_id,
            content: fixedContentBinding(unit.content),
          })),
        }),
      ),
    );
    if (
      authority.manifest_id !== input.manifest_id ||
      authority.generation !== input.barrier_generation ||
      !digest.equals(Buffer.from(input.manifest_digest)) ||
      !digest.equals(Buffer.from(authority.manifest_digest, 'base64')) ||
      canonical(units.map((unit) => ({ ...unit, content: storeContent(unit.content) }))) !==
        canonical(
          (input.units ?? []).map((unit) => ({ ...unit, content: storeContent(unit.content) })),
        )
    )
      throw new RPCFailure(6, 'MAIN_MANIFEST_CONFLICT');
    const installed = await this.transactions.run(async (connection) => {
      await barriers.lockDocument(connection, context.target.document_id);
      await requireDocumentBodyPresent(connection, context.target.document_id);
      const [[receipt]] = await installations.lockReceipt(connection, context.operation_id);
      if (receipt) {
        if (
          receipt.generation !== input.barrier_generation ||
          receipt.manifest_id !== input.manifest_id ||
          !receipt.manifest_digest.equals(digest)
        )
          throw new RPCFailure(6, 'MAIN_INSTALL_CONFLICT');
        return this.receiptReply(receipt);
      }
      const [[barrier]] = await barriers.lockBarrier(connection, context.target.document_id);
````

## 第 11 步：MainInstallations（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 453 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>!barrier &#124;&#124;         barrier.operation_id !== context.operation_id &#124;&#124;         barrier.generation !== input.barrier_generation &#124;&#124;         barrier.state !== &#x27;PREPARED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_BARRIER_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;MAIN_BARRIER_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>barriers.lockMainRooms(connection, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>installations.retireMainRooms(connection, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>installations.closeRetiredGrants(connection, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>installations.stopRetiredDeliveries(connection, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!hash(Buffer.from(canonical(body))).equals(content.body_hash) &#124;&#124;           cut.workspace_epoch !== authority.control_revision &#124;&#124;           cut.durable_seq !== &#x27;0&#x27; &#124;&#124;           cont …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_INSTALL_CONTENT_INVALID&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;MAIN_INSTALL_CONTENT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>rooms.lockRoom(connection, cut.room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>existing</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_INSTALL_ROOM_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;MAIN_INSTALL_ROOM_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>rooms.insertRoom(           connection,           cut.room_id,           context.target.document_id,           unit.unit_id,           &#x27;MAIN&#x27;,           unit.unit_id,           cut.workspace_epoch,         )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>this.snapshots.checkpoint(connection, room, doc, &#x27;0&#x27;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 11 bytes=19246:21223 -->
````javascript
      if (
        !barrier ||
        barrier.operation_id !== context.operation_id ||
        barrier.generation !== input.barrier_generation ||
        barrier.state !== 'PREPARED'
      )
        throw new RPCFailure(14, 'MAIN_BARRIER_UNKNOWN');
      this.matchContext(barrier, context);
      await barriers.lockMainRooms(connection, context.target.document_id);
      await installations.retireMainRooms(connection, context.target.document_id);
      await installations.closeRetiredGrants(connection, context.target.document_id);
      await installations.stopRetiredDeliveries(connection, context.target.document_id);
      const cuts = [];
      for (const unit of units) {
        const content = unit.content,
          cut = JSON.parse(content.cut_json_utf8);
        const body = normalizeContent(JSON.parse(content.body_json_utf8), {
          workspaceKind: 'MAIN',
        });
        if (
          !hash(Buffer.from(canonical(body))).equals(content.body_hash) ||
          cut.workspace_epoch !== authority.control_revision ||
          cut.durable_seq !== '0' ||
          content.unit_id !== unit.unit_id ||
          content.document_id !== context.target.document_id ||
          content.structure_id !== authority.structure_id
        )
          throw new RPCFailure(6, 'MAIN_INSTALL_CONTENT_INVALID');
        const [[existing]] = await rooms.lockRoom(connection, cut.room_id);
        if (existing) throw new RPCFailure(6, 'MAIN_INSTALL_ROOM_CONFLICT');
        await rooms.insertRoom(
          connection,
          cut.room_id,
          context.target.document_id,
          unit.unit_id,
          'MAIN',
          unit.unit_id,
          cut.workspace_epoch,
        );
        const room = { id: cut.room_id, epoch: cut.workspace_epoch };
        const doc = prosemirrorJSONToYDoc(schema, toPM(body));
        try {
          await this.snapshots.checkpoint(connection, room, doc, '0');
        } finally {
          doc.destroy();
        }
````

## 第 12 步：MainInstallations（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 499 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>barriers.insertMainContext(connection, cut.room_id, cut.workspace_epoch, {           structure_id: authority.structure_id,           style_id: authority.style_id,           control …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>context.target.purpose === &#x27;MERGE&#x27;</code>。
- 在本段的内部函数内：等待 <code>this.acceptLineage(connection, authority.lineage)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>authority.lineage != null</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_CONTROL_LINEAGE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;MAIN_CONTROL_LINEAGE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>authority.structure_transition</code>。
- 在本段的内部函数内：检查 <code>context.target.purpose !== &#x27;MAIN_CONTROL&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_INSTALL_PURPOSE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_INSTALL_PURPOSE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>this.structureInstallations.install(             connection,             context,             authority.structure_transition,             units,           )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>installations.insertReceipt(connection, input, cuts)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>installations.installedBarrier(         connection,         context.target.document_id,         context.operation_id,         result.result_digest,         input.manifest_id,       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return {         state: &#x27;READY&#x27;,         operation_id: context.operation_id,         barrier_generation: input.barrier_generation,         manifest_id: input.manifest_id,         i …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.transactions.run(async (connection) =&gt; {       const &#91;rows&#93; = await installations.retiredMainRooms(connection, context.target.document_id);       return &#91;         ...rows.map( …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>installations.retiredMainRooms(connection, context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return &#91;         ...rows.map((room) =&gt; room.id),         ...(await this.structureInstallations.retiredRooms(connection, context.operation_id)),       &#93;;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>this.structureInstallations.retiredRooms(connection, context.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 12 bytes=21223:23238 -->
````javascript
        await barriers.insertMainContext(connection, cut.room_id, cut.workspace_epoch, {
          structure_id: authority.structure_id,
          style_id: authority.style_id,
          control_revision: authority.control_revision,
          manifest_id: authority.manifest_id,
        });
        cuts.push(cut);
      }
      // 管理控制只物化MAIN，不接纳任何支线；MERGE仍必须证明并追加原ACCEPTED_CUT来源。
      if (context.target.purpose === 'MERGE')
        await this.acceptLineage(connection, authority.lineage);
      else if (authority.lineage != null) throw new RPCFailure(6, 'MAIN_CONTROL_LINEAGE_INVALID');
      if (authority.structure_transition) {
        if (context.target.purpose !== 'MAIN_CONTROL')
          throw new RPCFailure(6, 'STRUCTURE_INSTALL_PURPOSE_INVALID');
        cuts.push(
          ...(await this.structureInstallations.install(
            connection,
            context,
            authority.structure_transition,
            units,
          )),
        );
      }
      await installations.insertReceipt(connection, input, cuts);
      await installations.installedBarrier(
        connection,
        context.target.document_id,
        context.operation_id,
        result.result_digest,
        input.manifest_id,
      );
      return {
        state: 'READY',
        operation_id: context.operation_id,
        barrier_generation: input.barrier_generation,
        manifest_id: input.manifest_id,
        installed_epochs: cuts,
        manifest_digest: digest,
      };
    });
    const retired = await this.transactions.run(async (connection) => {
      const [rows] = await installations.retiredMainRooms(connection, context.target.document_id);
      return [
        ...rows.map((room) => room.id),
        ...(await this.structureInstallations.retiredRooms(connection, context.operation_id)),
      ];
    });
    // 发送READY之前等待真实旧socket退出。丢ACK时只重读原receipt，不会再覆盖后来MAIN代次。
````

## 第 13 步：MainInstallations（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 548 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 MainInstallations.acceptLineage：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>rooms.lockRoom(connection, patch.room_id)</code>；<code>lineages.lockRoomLineage(connection, patch.room_id, patch.epoch)</code>；<code>installations.updateLineage(connection, patch.room_id, patch.epoch, patch.context)</code> |
| <code>patch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>patch?.context</code>；<code>patch.room_id</code>；<code>patch.epoch</code>；<code>patch.context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>undefined</code> |

**本段进入 MainInstallations.matchContext：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>barrier</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>barrier.cut_vector</code> |
| <code>context</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>canonical({         ...context,         request_fingerprint: Buffer.from(context.request_fingerprint).toString(&#x27;base64&#x27;),       })</code>；<code>context.request_fingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**跟着本段代码走一遍：**

- 检查 <code>authority.structure_transition</code>。
- 等待 <code>this.transactions.run((c) =&gt;         this.structureInstallations.navigation(           c,           context.operation_id,           authority.structure_transition,         ),       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>this.outputs.installedStructure(context.operation_id, navigation)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>this.outputs.stopRooms(retired)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return installed;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!patch?.context</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_LINEAGE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;MAIN_LINEAGE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>rooms.lockRoom(connection, patch.room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>lineages.lockRoomLineage(connection, patch.room_id, patch.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!room &#124;&#124;       room.workspace_kind !== &#x27;BRANCH&#x27; &#124;&#124;       room.epoch !== patch.epoch &#124;&#124;       room.state !== &#x27;OPEN&#x27; &#124;&#124;       !stored</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_LINEAGE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;MAIN_LINEAGE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>current.lineage_id === patch.context.lineage_id</code>。
- 检查 <code>canonical(current) !== canonical(patch.context)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_LINEAGE_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;MAIN_LINEAGE_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>current.lineage_id !== patch.previous_lineage_id</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MAIN_LINEAGE_PENDING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;MAIN_LINEAGE_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>installations.updateLineage(connection, patch.room_id, patch.epoch, patch.context)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>canonical(binding) !==       canonical({         ...context,         request_fingerprint: Buffer.from(context.request_fingerprint).toString(&#x27;base64&#x27;),       })</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_BARRIER_CONFLICT&#x27;);</code>。转入外层异常处理路径。

<!-- write-step: 13 bytes=23238:25001 -->
````javascript
    if (authority.structure_transition) {
      const navigation = await this.transactions.run((c) =>
        this.structureInstallations.navigation(
          c,
          context.operation_id,
          authority.structure_transition,
        ),
      );
      await this.outputs.installedStructure(context.operation_id, navigation);
    }
    await this.outputs.stopRooms(retired);
    return installed;
  }

  async acceptLineage(connection, patch) {
    if (!patch?.context) throw new RPCFailure(14, 'MAIN_LINEAGE_UNKNOWN');
    const [[room]] = await rooms.lockRoom(connection, patch.room_id);
    const [[stored]] = await lineages.lockRoomLineage(connection, patch.room_id, patch.epoch);
    if (
      !room ||
      room.workspace_kind !== 'BRANCH' ||
      room.epoch !== patch.epoch ||
      room.state !== 'OPEN' ||
      !stored
    )
      throw new RPCFailure(14, 'MAIN_LINEAGE_UNKNOWN');
    const current = json(stored.payload);
    if (current.lineage_id === patch.context.lineage_id) {
      if (canonical(current) !== canonical(patch.context))
        throw new RPCFailure(6, 'MAIN_LINEAGE_CONFLICT');
      return;
    }
    if (current.lineage_id !== patch.previous_lineage_id)
      throw new RPCFailure(14, 'MAIN_LINEAGE_PENDING');
    // 同epoch只换来源基线，不修改Yjs草稿。已CUT_FIXED保存了自己的旧来源，继续保持原固定语义。
    await installations.updateLineage(connection, patch.room_id, patch.epoch, patch.context);
  }

  matchContext(barrier, context) {
    const binding = json(barrier.cut_vector).binding.context;
    if (
      canonical(binding) !==
      canonical({
        ...context,
        request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
      })
    )
````

## 第 14 步：MainInstallations（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 595 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 MainInstallations.abort：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.context</code>；<code>input.barrier_generation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>result</code> |

**本段进入 MainInstallations.receiptReply：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>receipt</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>receipt.operation_id</code>；<code>receipt.generation</code>；<code>receipt.manifest_id</code>；<code>receipt.installed_epochs</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{       state: &#x27;READY&#x27;,       operation_id: receipt.operation_id,       barrier_generation: receipt.generation,       manifest_id: receipt.manifest_id,       installed_epochs: json …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 执行 <code>throw new RPCFailure(6, &#x27;MAIN_BARRIER_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.transactions.run(async (connection) =&gt; {       await barriers.lockDocument(connection, input.context.target.document_id);       const &#91;&#91;barrier&#93;&#93; = await barriers.lockBarrier( …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>barriers.lockDocument(connection, input.context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>barriers.lockBarrier(connection, input.context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>barrier &amp;&amp; barrier.operation_id === input.context.operation_id</code>。
- 在本段的内部函数内：检查 <code>barrier.generation !== input.barrier_generation &#124;&#124; barrier.state === &#x27;INSTALLED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MAIN_BARRIER_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;MAIN_BARRIER_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>this.structurePreparations.abort(connection, json(barrier.cut_vector).binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>installations.abortBarrier(           connection,           input.context.target.document_id,           input.context.operation_id,           input.barrier_generation,         )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return {         state: &#x27;ABORTED&#x27;,         operation_id: input.context.operation_id,         barrier_generation: input.barrier_generation,       };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return {       state: &#x27;READY&#x27;,       operation_id: receipt.operation_id,       barrier_generation: receipt.generation,       manifest_id: receipt.manifest_id,       installed_epoch …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=25001:26467 -->
````javascript
      throw new RPCFailure(6, 'MAIN_BARRIER_CONFLICT');
  }

  async abort(input) {
    const result = await this.transactions.run(async (connection) => {
      await barriers.lockDocument(connection, input.context.target.document_id);
      const [[barrier]] = await barriers.lockBarrier(connection, input.context.target.document_id);
      if (barrier && barrier.operation_id === input.context.operation_id) {
        if (barrier.generation !== input.barrier_generation || barrier.state === 'INSTALLED')
          throw new RPCFailure(6, 'MAIN_BARRIER_CONFLICT');
        this.matchContext(barrier, input.context);
        await this.structurePreparations.abort(connection, json(barrier.cut_vector).binding);
        await installations.abortBarrier(
          connection,
          input.context.target.document_id,
          input.context.operation_id,
          input.barrier_generation,
        );
      }
      return {
        state: 'ABORTED',
        operation_id: input.context.operation_id,
        barrier_generation: input.barrier_generation,
      };
    });
    this.outputs.finishStructure({ context: input.context });
    return result;
  }

  receiptReply(receipt) {
    return {
      state: 'READY',
      operation_id: receipt.operation_id,
      barrier_generation: receipt.generation,
      manifest_id: receipt.manifest_id,
      installed_epochs: json(receipt.installed_epochs),
      manifest_digest: receipt.manifest_digest,
    };
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/main_workflows.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
