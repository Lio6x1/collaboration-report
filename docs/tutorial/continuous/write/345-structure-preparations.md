# 分步编写 `services/collaboration/src/application/structure_preparations.mjs`

先按 [C05 · 结构拆并与每个在线编辑器保存握手](../lessons/collaboration/05-structure-online.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/345-structure-preparations.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

核对预览以来room与lineage未变，再持久阻塞受影响支线。

即便新建room仍是seq0，也与预览时“room不存在”不同，不能忽略这次并发准入。

写之前先按这条执行路线理解各段的连接：

1. plan读原STRUCTURE_PREVIEW_FIXED并调用structurePreparation
2. prepare比较expected_cut，检查room存在性/身份/OPEN及lineage
3. 重算实际MAIN+受影响BRANCH cut，相同后为每个branch写gate，已有room置BLOCKED
4. 写STRUCTURE_PREPARATION；abort仅按原binding解开自己gate和room

本文件需要守住的边界：

- 任何room从无到有、seq变化或lineage变化返回CHANGED
- 已有非ABORTED支线gate不可覆盖

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/structure_preparations.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/structure_preparations.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/structure_preparations.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/structure_preparations.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:646 -->
````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { structurePreparation } from '../domain/structure_preparation.mjs';
import { cutDigest } from '../domain/main_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import * as artifacts from '../repository/draft_transitions.mjs';
import * as repository from '../repository/structure_workspace.mjs';
import * as previews from '../repository/structure_workspace.mjs';
import * as lineages from '../repository/candidate_artifacts.mjs';
import { draftLineage } from '../domain/draft_protocol.mjs';

````

## 第 2 步：StructurePreparations

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`export class StructurePreparations `。

**本段进入 StructurePreparations.plan：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>artifacts.artifact(       connection,       &#x27;STRUCTURE_PREVIEW_FIXED&#x27;,       authority.reference,     )</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>structurePreparation(json(row.payload), authority, binding)</code> |
| <code>authority</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>authority.reference</code>；<code>structurePreparation(json(row.payload), authority, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>structurePreparation(json(row.payload), authority, binding)</code> |

**本段进入 StructurePreparations.prepare：**

**这一段为什么这样写：** 把预览后的并发编辑检测转换为持久屏障。

1. 重读原计划
2. 比较存在性和真实切点
3. 逐支线建立PREPARED gate
4. 保存准备事实


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | MAIN准备事务的同一连接<br>在此声明中的实际使用：<code>this.plan(connection, binding, authority)</code>；<code>previews.lockBranchRooms(       connection,       binding.context.target.document_id,     )</code>；<code>lineages.lockRoomLineage(connection, room.id, room.epoch)</code>；<code>repository.branchGate(         connection,         branch.room_id,         branch.workspace_epoch,       )</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>JavaScript 参数</code> | 原MAIN_CONTROL确认请求<br>在此声明中的实际使用：<code>this.plan(connection, binding, authority)</code>；<code>binding.expected_cut</code>；<code>binding.context</code>；<code>repository.prepareBranchGate(connection, branch, binding)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>authority</code> | <code>JavaScript 参数</code> | Doc核验的预览引用摘要<br>在此声明中的实际使用：<code>this.plan(connection, binding, authority)</code>；<code>artifacts.insertArtifact(       connection,       &#x27;STRUCTURE_PREPARATION&#x27;,       binding.context.operation_id,       { binding, authority, branches: plan.branches },     )</code> |
| <code>mainRooms</code> | <code>JavaScript 参数</code> | 已经按统一锁序锁定的MAIN room行<br>在此声明中的实际使用：<code>mainRooms.map</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>{ state: &#x27;CHANGED&#x27;, cut: plan.cut }</code>；<code>{ state: &#x27;CHANGED&#x27;, cut }</code>；<code>{ state: &#x27;PREPARED&#x27;, cut }</code> |

**跟着本段代码走一遍：**

- 等待 <code>artifacts.artifact(       connection,       &#x27;STRUCTURE_PREVIEW_FIXED&#x27;,       authority.reference,     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!row</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_PREPARATION_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_PREPARATION_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return structurePreparation(json(row.payload), authority, binding);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.plan(connection, binding, authority)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>canonical(plan.cut) !== canonical(binding.expected_cut)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_PREVIEW_CUT_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_PREVIEW_CUT_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>previews.lockBranchRooms(       connection,       binding.context.target.document_id,     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>found.length &gt; 1 &#124;&#124; !!room !== branch.present</code>。条件成立时执行 <code>return { state: &#x27;CHANGED&#x27;, cut: plan.cut };</code>。当前函数在这里结束。
- 检查 <code>room</code>。
- 检查 <code>room.id !== branch.room_id &#124;&#124;           room.workspace_id !== branch.branch_id &#124;&#124;           room.epoch !== branch.workspace_epoch &#124;&#124;           room.state !== &#x27;OPEN&#x27;</code>。条件成立时执行 <code>return { state: &#x27;CHANGED&#x27;, cut: plan.cut };</code>。当前函数在这里结束。
- 等待 <code>lineages.lockRoomLineage(connection, room.id, room.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!lineage &#124;&#124;           canonical(draftLineage(json(lineage.payload))) !== canonical(draftLineage(branch.lineage))</code>。条件成立时执行 <code>return { state: &#x27;CHANGED&#x27;, cut: plan.cut };</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=646:2623 -->
````javascript
export class StructurePreparations {
  async plan(connection, binding, authority) {
    const [[row]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREVIEW_FIXED',
      authority.reference,
    );
    if (!row) throw new RPCFailure(14, 'STRUCTURE_PREPARATION_UNKNOWN');
    return structurePreparation(json(row.payload), authority, binding);
  }

  async prepare(connection, binding, authority, mainRooms) {
    const plan = await this.plan(connection, binding, authority);
    if (canonical(plan.cut) !== canonical(binding.expected_cut))
      throw new RPCFailure(6, 'STRUCTURE_PREVIEW_CUT_CONFLICT');
    const [branchRooms] = await previews.lockBranchRooms(
      connection,
      binding.context.target.document_id,
    );
    const actual = mainRooms.map((r) => ({
      room_id: r.id,
      workspace_epoch: r.epoch,
      durable_seq: r.last_durable_seq,
    }));
    for (const branch of plan.branches) {
      const found = branchRooms.filter((r) => r.unit_id === branch.unit_id);
      const room = found[0];
      // 预览时不存在的房间若后来OPEN，零尾也属于切点变化，不能当作原虚拟基线。
      if (found.length > 1 || !!room !== branch.present) return { state: 'CHANGED', cut: plan.cut };
      if (room) {
        if (
          room.id !== branch.room_id ||
          room.workspace_id !== branch.branch_id ||
          room.epoch !== branch.workspace_epoch ||
          room.state !== 'OPEN'
        )
          return { state: 'CHANGED', cut: plan.cut };
        const [[lineage]] = await lineages.lockRoomLineage(connection, room.id, room.epoch);
        if (
          !lineage ||
          canonical(draftLineage(json(lineage.payload))) !== canonical(draftLineage(branch.lineage))
        )
          return { state: 'CHANGED', cut: plan.cut };
        actual.push({
          room_id: room.id,
          workspace_epoch: room.epoch,
          durable_seq: room.last_durable_seq,
        });
      }
    }
````

## 第 3 步：StructurePreparations（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 62 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 StructurePreparations.abort：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>artifacts.artifact(       connection,       &#x27;STRUCTURE_PREPARATION&#x27;,       binding.context.operation_id,     )</code>；<code>repository.finishBranchGate(connection, branch, binding, &#x27;ABORTED&#x27;)</code>；<code>artifacts.openRoom(connection, branch.room_id, branch.workspace_epoch)</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.context</code>；<code>canonical(binding)</code>；<code>repository.finishBranchGate(connection, branch, binding, &#x27;ABORTED&#x27;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>undefined</code> |

**跟着本段代码走一遍：**

- 检查 <code>canonical(cut) !== canonical(plan.cut)</code>。条件成立时执行 <code>return { state: &#x27;CHANGED&#x27;, cut };</code>。当前函数在这里结束。
- 等待 <code>repository.branchGate(         connection,         branch.room_id,         branch.workspace_epoch,       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>prior &amp;&amp; json(prior.payload).state !== &#x27;ABORTED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_GATE_PENDING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_BRANCH_GATE_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>repository.prepareBranchGate(connection, branch, binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>branch.present</code>。
- 等待 <code>artifacts.blockRoom(connection, branch.room_id, branch.workspace_epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>artifacts.insertArtifact(       connection,       &#x27;STRUCTURE_PREPARATION&#x27;,       binding.context.operation_id,       { binding, authority, branches: plan.branches },     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return { state: &#x27;PREPARED&#x27;, cut };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>artifacts.artifact(       connection,       &#x27;STRUCTURE_PREPARATION&#x27;,       binding.context.operation_id,     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!row</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。
- 检查 <code>canonical(stored.binding) !== canonical(binding)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_PREPARATION_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_PREPARATION_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>repository.finishBranchGate(connection, branch, binding, &#x27;ABORTED&#x27;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>branch.present</code>。
- 等待 <code>artifacts.openRoom(connection, branch.room_id, branch.workspace_epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=2623:4199 -->
````javascript
    actual.sort((a, b) => (BigInt(a.room_id) < BigInt(b.room_id) ? -1 : 1));
    const cut = { ...plan.cut, rooms: actual, dependency_digest: cutDigest(actual) };
    if (canonical(cut) !== canonical(plan.cut)) return { state: 'CHANGED', cut };
    for (const branch of plan.branches) {
      const [[prior]] = await repository.branchGate(
        connection,
        branch.room_id,
        branch.workspace_epoch,
      );
      if (prior && json(prior.payload).state !== 'ABORTED')
        throw new RPCFailure(14, 'STRUCTURE_BRANCH_GATE_PENDING');
      await repository.prepareBranchGate(connection, branch, binding);
      if (branch.present)
        await artifacts.blockRoom(connection, branch.room_id, branch.workspace_epoch);
    }
    await artifacts.insertArtifact(
      connection,
      'STRUCTURE_PREPARATION',
      binding.context.operation_id,
      { binding, authority, branches: plan.branches },
    );
    return { state: 'PREPARED', cut };
  }

  async abort(connection, binding) {
    const [[row]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREPARATION',
      binding.context.operation_id,
    );
    if (!row) return;
    const stored = json(row.payload);
    if (canonical(stored.binding) !== canonical(binding))
      throw new RPCFailure(6, 'STRUCTURE_PREPARATION_CONFLICT');
    for (const branch of stored.branches) {
      await repository.finishBranchGate(connection, branch, binding, 'ABORTED');
      if (branch.present)
        await artifacts.openRoom(connection, branch.room_id, branch.workspace_epoch);
    }
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/structure_preparations.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/05-structure-online.md)。
