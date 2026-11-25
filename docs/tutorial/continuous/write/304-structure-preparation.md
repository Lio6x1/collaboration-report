# 分步编写 `services/collaboration/src/domain/structure_preparation.mjs`

先按 [C05 · 结构拆并与每个在线编辑器保存握手](../lessons/collaboration/05-structure-online.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/304-structure-preparation.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

从已冻结结构预览派生本次准备所需MAIN及受影响支线cut。

准备可以缩小到受影响支线，但不能重新采样或改变预览候选。

写之前先按这条执行路线理解各段的连接：

1. 重算fixed.input_digest并核authority.reference/digest/expires
2. 检查确认actor/document/structure/workspace/sources与预览一致
3. 由migration.source_units选中支线，包括预览时不存在room的虚拟基线
4. 核observed_cut摘要/room唯一性，生成selected cut及branches.present

本文件需要守住的边界：

- 预览过期应STRUCTURE_PREVIEW_EXPIRED
- 不存在room也必须生成支线入口门闩，不能让晚到OPEN绕过准备

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/structure_preparation.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/structure_preparation.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/structure_preparation.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/structure_preparation.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:216 -->
````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { hash } from './shared_protocol.mjs';
import { cutDigest } from './main_protocol.mjs';
import { RPCFailure } from './shared_protocol.mjs';

````

## 第 2 步：structurePreparation

插入位置：第 1 步代码之后，紧接追加；完成稿约第 6 行。

从同一固定预览缩小准备范围，不重采样、不改变候选。没有实体房间的支线也需要持久入口屏障。

本步声明：`export function structurePreparation(fixed, authority, binding, now = Date.now()) `。

**这一段为什么这样写：** 纯规则产生可执行的固定准备计划。

1. 验证原预览完整性与有效期
2. 确定受影响unit及room归属
3. 保留所有MAIN和相关BRANCH cut
4. 返回cut/branches


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>JavaScript 参数</code> | 已持久完整预览<br>在此声明中的实际使用：<code>!fixed &#124;&#124;     !authority</code>；<code>fixed.binding</code>；<code>fixed.observed_cut</code>；<code>fixed.source</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>authority</code> | <code>JavaScript 参数</code> | Doc确认的原预览引用、摘要和期限<br>在此声明中的实际使用：<code>!fixed &#124;&#124;     !authority</code>；<code>authority.digest</code>；<code>authority.reference</code>；<code>authority.expires</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | MAIN_CONTROL准备绑定<br>在此声明中的实际使用：<code>binding?.context</code>；<code>binding.context</code> |
| <code>now = Date.now()</code> | <code>带默认值的参数</code> | 可注入的当前毫秒时间，用于过期判断<br>在此声明中的实际使用：<code>Number(authority.expires) &lt;= now</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     cut: { ...fixed.observed_cut, rooms: selected, dependency_digest: cutDigest(selected) },     branches,   }</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_PREPARATION_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!fixed &#124;&#124;     !authority &#124;&#124;     !binding?.context &#124;&#124;     !fixed.binding?.context &#124;&#124;     !fixed.observed_cut &#124;&#124;     !Array.isArray(fixed.observed_cut.rooms) &#124;&#124;     !Array.isArray(fi …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>hash(canonical(original)).toString(&#x27;base64&#x27;) !== digest &#124;&#124;     authority.digest !== digest &#124;&#124;     authority.reference !== fixed.preview_cut_ref &#124;&#124;     authority.expires !== fixed.expires_unix_ms</code>。
- 检查 <code>!/^&#91;1-9&#93;&#91;0-9&#93;{0,15}$/.test(authority.expires)</code>。
- 检查 <code>Number(authority.expires) &lt;= now</code>。条件成立时执行 <code>throw new RPCFailure(9, &#x27;STRUCTURE_PREVIEW_EXPIRED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(9, &#x27;STRUCTURE_PREVIEW_EXPIRED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>current.target?.purpose !== &#x27;MAIN_CONTROL&#x27; &#124;&#124;     current.actor_id !== prior.actor_id &#124;&#124;     current.target.document_id !== prior.target.document_id &#124;&#124;     current.target.structure …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>!affected.size</code>。
- 检查 <code>!unit?.unit_id &#124;&#124;       !unit.branch_id &#124;&#124;       !unit.room_id &#124;&#124;       !unit.workspace_epoch &#124;&#124;       !unit.lineage &#124;&#124;       drafts.has(unit.unit_id) &#124;&#124;       roomOwners.has(unit.room_id)</code>。

<!-- write-step: 2 bytes=216:2188 -->
````javascript
// 从同一固定预览缩小准备范围，不重采样、不改变候选。没有实体房间的支线也需要持久入口屏障。
export function structurePreparation(fixed, authority, binding, now = Date.now()) {
  const unknown = () => {
    throw new RPCFailure(14, 'STRUCTURE_PREPARATION_UNKNOWN');
  };
  if (
    !fixed ||
    !authority ||
    !binding?.context ||
    !fixed.binding?.context ||
    !fixed.observed_cut ||
    !Array.isArray(fixed.observed_cut.rooms) ||
    !Array.isArray(fixed.source?.drafts) ||
    !Array.isArray(fixed.migration?.changes) ||
    fixed.migration.schema_version !== 'structure-migration-v1'
  )
    unknown();
  const { input_digest: digest, ...original } = fixed;
  if (
    hash(canonical(original)).toString('base64') !== digest ||
    authority.digest !== digest ||
    authority.reference !== fixed.preview_cut_ref ||
    authority.expires !== fixed.expires_unix_ms
  )
    unknown();
  if (!/^[1-9][0-9]{0,15}$/.test(authority.expires)) unknown();
  if (Number(authority.expires) <= now) throw new RPCFailure(9, 'STRUCTURE_PREVIEW_EXPIRED');
  const prior = fixed.binding.context,
    current = binding.context;
  if (
    current.target?.purpose !== 'MAIN_CONTROL' ||
    current.actor_id !== prior.actor_id ||
    current.target.document_id !== prior.target.document_id ||
    current.target.structure_id !== prior.target.structure_id ||
    current.target.workspace_epoch !== prior.target.workspace_epoch ||
    canonical(current.sources) !== canonical(prior.sources)
  )
    unknown();
  const affected = new Set(fixed.migration.changes.flatMap((v) => v.source_units));
  if (!affected.size) unknown();
  const drafts = new Map(),
    roomOwners = new Map();
  for (const unit of fixed.source.drafts) {
    if (
      !unit?.unit_id ||
      !unit.branch_id ||
      !unit.room_id ||
      !unit.workspace_epoch ||
      !unit.lineage ||
      drafts.has(unit.unit_id) ||
      roomOwners.has(unit.room_id)
    )
````

## 第 3 步：structurePreparation（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 58 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>rooms.size !== fixed.observed_cut.rooms.length &#124;&#124;     cutDigest(fixed.observed_cut.rooms) !== fixed.observed_cut.dependency_digest</code>。
- 在本段的内部函数内：检查 <code>!unit</code>。
- 在本段的内部函数内：检查 <code>cut &amp;&amp; cut.workspace_epoch !== unit.workspace_epoch</code>。
- 在本段的内部函数内：此处通过 <code>return { ...unit, present: !!cut, ...(cut ? { cut } : {}) };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return {     cut: { ...fixed.observed_cut, rooms: selected, dependency_digest: cutDigest(selected) },     branches,   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2188:3086 -->
````javascript
      unknown();
    drafts.set(unit.unit_id, unit);
    roomOwners.set(unit.room_id, unit.unit_id);
  }
  const rooms = new Map(fixed.observed_cut.rooms.map((v) => [v.room_id, v]));
  if (
    rooms.size !== fixed.observed_cut.rooms.length ||
    cutDigest(fixed.observed_cut.rooms) !== fixed.observed_cut.dependency_digest
  )
    unknown();
  const branches = [...affected].map((id) => {
    const unit = drafts.get(id);
    if (!unit) unknown();
    const cut = rooms.get(unit.room_id);
    if (cut && cut.workspace_epoch !== unit.workspace_epoch) unknown();
    return { ...unit, present: !!cut, ...(cut ? { cut } : {}) };
  });
  const selected = fixed.observed_cut.rooms.filter(
    (v) => !roomOwners.has(v.room_id) || affected.has(roomOwners.get(v.room_id)),
  );
  return {
    cut: { ...fixed.observed_cut, rooms: selected, dependency_digest: cutDigest(selected) },
    branches,
  };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/structure_preparation.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/05-structure-online.md)。
