# 分步编写 `services/collaboration/src/application/structure_installations.mjs`

先按 [C05 · 结构拆并与每个在线编辑器保存握手](../lessons/collaboration/05-structure-online.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/344-structure-installations.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按原预览候选安装新支线房间，并退休受影响旧支线。

Doc返回新清单仍须与Node固定迁移一一对应，防止漏安装或把MAIN正文装成草稿。

写之前先按这条执行路线理解各段的连接：

1. install核preparation与preview摘要，targets数量等于transition.branches
2. 逐新支线比candidate_key、target.draft_body、对应MAIN body、seq0 cut和lineage/provenance
3. 同事务建新room/checkpoint/lineage，核旧gate/room/cut后退役旧room并关闭准入
4. navigation从原source_ranges映射旧块到新unit，retiredRooms提供真实排空列表

本文件需要守住的边界：

- 安装导航只提示入口，新room仍须重新申请权限
- 新检查点与旧房间退休必须原子提交
- 旧room未BLOCKED或尾部变动不能继续安装

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/structure_installations.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/structure_installations.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/structure_installations.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/structure_installations.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:711 -->
````javascript
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { canonical, normalizeContent, schema, toPM } from '../../../../packages/content/index.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { wireContent } from '../domain/capture_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import * as artifacts from '../repository/draft_transitions.mjs';
import * as gates from '../repository/structure_workspace.mjs';
import * as rooms from '../repository/room_access.mjs';
import * as lineages from '../repository/candidate_artifacts.mjs';
import { draftBody, draftLineage } from '../domain/draft_protocol.mjs';

````

## 第 2 步：StructureInstallations

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`export class StructureInstallations `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>snapshots</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |

**本段进入 StructureInstallations.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 StructureInstallations.navigation：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>artifacts.artifact(connection, &#x27;STRUCTURE_PREPARATION&#x27;, operation)</code>；<code>artifacts.artifact(       connection,       &#x27;STRUCTURE_PREVIEW_FIXED&#x27;,       json(prepared.payload).authority.reference,     )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>artifacts.artifact(connection, &#x27;STRUCTURE_PREPARATION&#x27;, operation)</code> |
| <code>transition</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>transition.branches</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>transition.branches.flatMap((branch) =&gt; {       const target = targets.get(branch.candidate_key);       if (!target) throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);       re …</code>（长表达式，按本步完整代码继续） |

**本段进入 StructureInstallations.install：**

**这一段为什么这样写：** 将已提交的结构清单物化为可重启恢复的新工作区。

1. 校验新旧候选完整对应
2. 创建所有新支线检查点
3. 核原准备门闩并退休旧支线
4. 返回新cuts供安装回执


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 主线安装事务连接<br>在此声明中的实际使用：<code>artifacts.artifact(       connection,       &#x27;STRUCTURE_PREPARATION&#x27;,       context.operation_id,     )</code>；<code>artifacts.artifact(       connection,       &#x27;STRUCTURE_PREVIEW_FIXED&#x27;,       prepared.authority.reference,     )</code>；<code>rooms.lockRoom(connection, cut.room_id)</code>；<code>rooms.insertRoom(         connection,         cut.room_id,         context.target.document_id,         branch.unit_id,         &#x27;BRANCH&#x27;,         branch.branch_id,         branch.epoch,       )</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>context</code> | <code>JavaScript 参数</code> | 原MAIN_CONTROL操作context<br>在此声明中的实际使用：<code>context.operation_id</code>；<code>context.target</code> |
| <code>transition</code> | <code>JavaScript 参数</code> | Doc已提交的结构支线迁移事实<br>在此声明中的实际使用：<code>!row &#124;&#124; !transition</code>；<code>transition.branches</code>；<code>transition.preview_cut_ref</code>；<code>transition.preview_digest</code> |
| <code>mainUnits</code> | <code>JavaScript 参数</code> | 同次安装的MAIN单元内容<br>在此声明中的实际使用：<code>mainUnits.find</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>cuts</code> |

**跟着本段代码走一遍：**

- 等待 <code>artifacts.artifact(connection, &#x27;STRUCTURE_PREPARATION&#x27;, operation)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!prepared</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>artifacts.artifact(       connection,       &#x27;STRUCTURE_PREVIEW_FIXED&#x27;,       json(prepared.payload).authority.reference,     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!original</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return transition.branches.flatMap((branch) =&gt; {       const target = targets.get(branch.candidate_key);       if (!target) throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);   …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>!target</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return target.source_ranges.map((range) =&gt; ({ ...range, unit_id: branch.unit_id }));</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>artifacts.artifact(       connection,       &#x27;STRUCTURE_PREPARATION&#x27;,       context.operation_id,     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!row &#124;&#124; !transition &#124;&#124; !Array.isArray(transition.branches)</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>transition.preview_cut_ref !== prepared.authority.reference &#124;&#124;       transition.preview_digest !== prepared.authority.digest</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_INSTALL_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_INSTALL_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>artifacts.artifact(       connection,       &#x27;STRUCTURE_PREVIEW_FIXED&#x27;,       prepared.authority.reference,     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=711:2537 -->
````javascript
export class StructureInstallations {
  constructor(snapshots) {
    this.snapshots = snapshots;
  }

  async navigation(connection, operation, transition) {
    const [[prepared]] = await artifacts.artifact(connection, 'STRUCTURE_PREPARATION', operation);
    if (!prepared) throw new RPCFailure(14, 'STRUCTURE_INSTALL_UNKNOWN');
    const [[original]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREVIEW_FIXED',
      json(prepared.payload).authority.reference,
    );
    if (!original) throw new RPCFailure(14, 'STRUCTURE_INSTALL_UNKNOWN');
    const targets = new Map(
      json(original.payload)
        .migration.changes.flatMap((c) => c.targets)
        .map((t) => [t.key, t]),
    );
    // 稳定块只用于定位新编辑入口；新房间正文仍要重新申请真实身份/权限准入。
    return transition.branches.flatMap((branch) => {
      const target = targets.get(branch.candidate_key);
      if (!target) throw new RPCFailure(14, 'STRUCTURE_INSTALL_UNKNOWN');
      return target.source_ranges.map((range) => ({ ...range, unit_id: branch.unit_id }));
    });
  }

  async install(connection, context, transition, mainUnits) {
    const [[row]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREPARATION',
      context.operation_id,
    );
    if (!row || !transition || !Array.isArray(transition.branches))
      throw new RPCFailure(14, 'STRUCTURE_INSTALL_UNKNOWN');
    const prepared = json(row.payload);
    if (
      transition.preview_cut_ref !== prepared.authority.reference ||
      transition.preview_digest !== prepared.authority.digest
    )
      throw new RPCFailure(6, 'STRUCTURE_INSTALL_CONFLICT');
    const [[original]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREVIEW_FIXED',
      prepared.authority.reference,
    );
````

## 第 3 步：StructureInstallations（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 59 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!original</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_INSTALL_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>targets.size !== transition.branches.length</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_INSTALL_INCOMPLETE&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_INSTALL_INCOMPLETE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!target &#124;&#124;         !main &#124;&#124;         content.document_id !== context.target.document_id &#124;&#124;         content.operation_id !== context.operation_id &#124;&#124;         content.unit_id !== branc …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_INSTALL_CONTENT_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_INSTALL_CONTENT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>rooms.lockRoom(connection, cut.room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>existing</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_INSTALL_ROOM_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_INSTALL_ROOM_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>rooms.insertRoom(         connection,         cut.room_id,         context.target.document_id,         branch.unit_id,         &#x27;BRANCH&#x27;,         branch.branch_id,         branch.epoch,       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=2537:4857 -->
````javascript
    if (!original) throw new RPCFailure(14, 'STRUCTURE_INSTALL_UNKNOWN');
    const fixed = json(original.payload),
      targets = new Map(fixed.migration.changes.flatMap((v) => v.targets).map((v) => [v.key, v]));
    if (targets.size !== transition.branches.length)
      throw new RPCFailure(6, 'STRUCTURE_INSTALL_INCOMPLETE');
    const cuts = [];
    for (const branch of transition.branches) {
      const target = targets.get(branch.candidate_key),
        content = wireContent(branch.content);
      const main = mainUnits.find((v) => v.unit_id === branch.unit_id);
      const cut = JSON.parse(content.cut_json_utf8);
      const body = normalizeContent(JSON.parse(content.body_json_utf8));
      if (
        !target ||
        !main ||
        content.document_id !== context.target.document_id ||
        content.operation_id !== context.operation_id ||
        content.unit_id !== branch.unit_id ||
        content.branch_id !== branch.branch_id ||
        content.workspace_epoch !== branch.epoch ||
        content.lineage_ref !== branch.lineage_id ||
        cut.room_id !== branch.room_id ||
        cut.workspace_epoch !== branch.epoch ||
        cut.durable_seq !== '0' ||
        content.hash_algorithm !== 'SHA-256' ||
        !hash(canonical(body)).equals(content.body_hash) ||
        canonical(body) !== canonical(target.draft_body) ||
        Buffer.from(main.content.body_json_utf8).toString() !== canonical(target.main_body) ||
        canonical(draftBody(branch.lineage_context?.body)) !==
          canonical(draftBody(target.main_body)) ||
        branch.lineage_context?.lineage_id !== branch.lineage_id ||
        branch.lineage_context?.base_version_id !== main.content_version_id ||
        canonical(branch.lineage_context?.provenance) !==
          canonical(JSON.parse(content.provenance_json_utf8))
      )
        throw new RPCFailure(6, 'STRUCTURE_INSTALL_CONTENT_INVALID');
      targets.delete(branch.candidate_key);
      const [[existing]] = await rooms.lockRoom(connection, cut.room_id);
      if (existing) throw new RPCFailure(6, 'STRUCTURE_INSTALL_ROOM_CONFLICT');
      await rooms.insertRoom(
        connection,
        cut.room_id,
        context.target.document_id,
        branch.unit_id,
        'BRANCH',
        branch.branch_id,
        branch.epoch,
      );
````

## 第 4 步：StructureInstallations（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 107 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 等待 <code>this.snapshots.checkpoint(           connection,           { id: cut.room_id, epoch: cut.workspace_epoch },           doc,           &#x27;0&#x27;,         )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>lineages.insertRoomLineage(         connection,         cut.room_id,         cut.workspace_epoch,         draftLineage(branch.lineage_context),       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>gates.branchGate(connection, old.room_id, old.workspace_epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!bound &#124;&#124;         bound.state !== &#x27;PREPARED&#x27; &#124;&#124;         bound.operation_id !== context.operation_id &#124;&#124;         bound.generation !== prepared.binding.barrier_generation</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_GATE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_GATE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>old.present</code>。
- 等待 <code>rooms.lockRoom(connection, old.room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!room &#124;&#124;           room.state !== &#x27;BLOCKED&#x27; &#124;&#124;           room.epoch !== old.workspace_epoch &#124;&#124;           room.last_durable_seq !== old.cut.durable_seq</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_PREPARED_BRANCH_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_PREPARED_BRANCH_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>artifacts.retireRoom(connection, old.room_id, old.workspace_epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>artifacts.closeGrants(connection, old.room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>artifacts.stopDeliveries(connection, old.room_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>gates.finishBranchGate(connection, old, prepared.binding, &#x27;INSTALLED&#x27;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 4 bytes=4857:6557 -->
````javascript
      const doc = prosemirrorJSONToYDoc(schema, toPM(body));
      try {
        await this.snapshots.checkpoint(
          connection,
          { id: cut.room_id, epoch: cut.workspace_epoch },
          doc,
          '0',
        );
      } finally {
        doc.destroy();
      }
      await lineages.insertRoomLineage(
        connection,
        cut.room_id,
        cut.workspace_epoch,
        draftLineage(branch.lineage_context),
      );
      cuts.push(cut);
    }
    // 新检查点与旧房间退役同TX；原尾部不再可写，旧socket真实退出后才释放安装ACK。
    for (const old of prepared.branches) {
      const [[gate]] = await gates.branchGate(connection, old.room_id, old.workspace_epoch);
      const bound = gate && json(gate.payload);
      if (
        !bound ||
        bound.state !== 'PREPARED' ||
        bound.operation_id !== context.operation_id ||
        bound.generation !== prepared.binding.barrier_generation
      )
        throw new RPCFailure(14, 'STRUCTURE_BRANCH_GATE_UNKNOWN');
      if (old.present) {
        const [[room]] = await rooms.lockRoom(connection, old.room_id);
        if (
          !room ||
          room.state !== 'BLOCKED' ||
          room.epoch !== old.workspace_epoch ||
          room.last_durable_seq !== old.cut.durable_seq
        )
          throw new RPCFailure(14, 'STRUCTURE_PREPARED_BRANCH_UNKNOWN');
        await artifacts.retireRoom(connection, old.room_id, old.workspace_epoch);
        await artifacts.closeGrants(connection, old.room_id);
        await artifacts.stopDeliveries(connection, old.room_id);
      }
      await gates.finishBranchGate(connection, old, prepared.binding, 'INSTALLED');
    }
````

## 第 5 步：StructureInstallations（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 152 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 StructureInstallations.retiredRooms：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>artifacts.artifact(connection, &#x27;STRUCTURE_PREPARATION&#x27;, operation)</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>artifacts.artifact(connection, &#x27;STRUCTURE_PREPARATION&#x27;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>row       ? json(row.payload)           .branches.filter((v) =&gt; v.present)           .map((v) =&gt; v.room_id)       : &#91;&#93;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return cuts;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>artifacts.artifact(connection, &#x27;STRUCTURE_PREPARATION&#x27;, operation)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return row       ? json(row.payload)           .branches.filter((v) =&gt; v.present)           .map((v) =&gt; v.room_id)       : &#91;&#93;;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=6557:6856 -->
````javascript
    return cuts;
  }

  async retiredRooms(connection, operation) {
    const [[row]] = await artifacts.artifact(connection, 'STRUCTURE_PREPARATION', operation);
    return row
      ? json(row.payload)
          .branches.filter((v) => v.present)
          .map((v) => v.room_id)
      : [];
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/structure_installations.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/05-structure-online.md)。
