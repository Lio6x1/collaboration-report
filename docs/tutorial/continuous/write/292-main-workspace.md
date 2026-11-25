# 分步编写 `services/collaboration/src/domain/main_workspace.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/292-main-workspace.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

判断在线MAIN房间与Doc固定manifest/structure/style/control上下文是否同源。

物理room epoch与管理control_revision不总能随意互换，元数据变化也不能授权读取任意旧代次。

写之前先按这条执行路线理解各段的连接：

1. matchesMainWorkspace核room.epoch=current.control_revision=source.workspace_epoch并匹配manifest/structure/style
2. preparedMainSource只从同操作MAIN_VIEW_FIXED事实中寻找context与expected_cut完全一致的来源

本文件需要守住的边界：

- 不能把小于当前control_revision的任何旧room都视为可用
- 找不到确实捕获过的来源返回null，由上层拒绝准备

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/main_workspace.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/main_workspace.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/main_workspace.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/main_workspace.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:69 -->
````javascript
import { canonical } from '../../../../packages/content/index.mjs';

````

## 第 2 步：positive

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`const positive = (value) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof value === &#x27;string&#x27;</code>；<code>/^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(value)</code>；<code>BigInt(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>typeof value === &#x27;string&#x27; &amp;&amp;   /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(value) &amp;&amp;   BigInt(value) &lt;= 18446744073709551615n</code> |

<!-- write-step: 2 bytes=69:293 -->
````javascript
const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;

// Doc管理修订可以只改变标题/负责人；这不改变物理MAIN房间。
````

## 第 3 步：matchesMainWorkspace

插入位置：第 2 步代码之后，紧接追加；完成稿约第 9 行。

允许该差异的依据是权威manifest的实际workspace_epoch，而不是任意“小于当前control”的房间。

本步声明：`export function matchesMainWorkspace(current, room, source) `。

**这一段为什么这样写：** 证明一个在线房间对应这次权威来源。

1. 校验正整数代次和workspace不超过control
2. 逐一匹配物理代次、清单和样式结构身份


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>current</code> | <code>JavaScript 参数</code> | ROOM_MAIN_CONTEXT保存的来源<br>在此声明中的实际使用：<code>!current &#124;&#124;     !room</code>；<code>current.control_revision</code>；<code>current.manifest_id</code>；<code>current.structure_id</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>room</code> | <code>JavaScript 参数</code> | 数据库room行<br>在此声明中的实际使用：<code>!current &#124;&#124;     !room</code>；<code>room.epoch</code> |
| <code>source</code> | <code>JavaScript 参数</code> | Doc当前或原操作已冻结来源<br>在此声明中的实际使用：<code>!current &#124;&#124;     !room &#124;&#124;     !source</code>；<code>source.workspace_epoch</code>；<code>source.control_revision</code>；<code>source.manifest_id</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>false</code>；<code>current.control_revision === room.epoch &amp;&amp;     room.epoch === source.workspace_epoch &amp;&amp;     current.manifest_id === source.manifest_id &amp;&amp;     current.structure_id === source.struct …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>!current &#124;&#124;     !room &#124;&#124;     !source &#124;&#124;     !positive(room.epoch) &#124;&#124;     !positive(source.workspace_epoch) &#124;&#124;     !positive(source.control_revision) &#124;&#124;     BigInt(source.workspace_epoch) &gt; BigInt(source.control_revision)</code>。条件成立时执行 <code>return false;</code>。当前函数在这里结束。
- 此处通过 <code>return (     current.control_revision === room.epoch &amp;&amp;     room.epoch === source.workspace_epoch &amp;&amp;     current.manifest_id === source.manifest_id &amp;&amp;     current.structure_id ===  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=293:987 -->
````javascript
// 允许该差异的依据是权威manifest的实际workspace_epoch，而不是任意“小于当前control”的房间。
export function matchesMainWorkspace(current, room, source) {
  if (
    !current ||
    !room ||
    !source ||
    !positive(room.epoch) ||
    !positive(source.workspace_epoch) ||
    !positive(source.control_revision) ||
    BigInt(source.workspace_epoch) > BigInt(source.control_revision)
  )
    return false;
  return (
    current.control_revision === room.epoch &&
    room.epoch === source.workspace_epoch &&
    current.manifest_id === source.manifest_id &&
    current.structure_id === source.structure_id &&
    current.style_id === source.style_id
  );
}

````

## 第 4 步：preparedMainSource

插入位置：第 3 步代码之后，紧接追加；完成稿约第 30 行。

只比较独立持久捕获的标量绑定与完整cut；准备请求不能自己给旧代次背书。

本步声明：`export function preparedMainSource(binding, facts) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.expected_cut</code>；<code>binding.context</code> |
| <code>facts</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>facts.find</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>fact     ? {         manifest_id: fact.manifest,         structure_id: expected.structure_id,         style_id: expected.style_id,         control_revision: expected.control_revision,       }     : null</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return fact     ? {         manifest_id: fact.manifest,         structure_id: expected.structure_id,         style_id: expected.style_id,         control_revision: expected.control_revision,       }     : null;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=987:1673 -->
````javascript
// 只比较独立持久捕获的标量绑定与完整cut；准备请求不能自己给旧代次背书。
export function preparedMainSource(binding, facts) {
  const expected = binding.expected_cut;
  const fact = facts.find(
    (candidate) =>
      positive(candidate.manifest) &&
      candidate.control === expected.control_revision &&
      canonical(candidate.context) === canonical(binding.context) &&
      canonical(candidate.cut) === canonical(expected),
  );
  return fact
    ? {
        manifest_id: fact.manifest,
        structure_id: expected.structure_id,
        style_id: expected.style_id,
        control_revision: expected.control_revision,
      }
    : null;
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/main_workspace.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
