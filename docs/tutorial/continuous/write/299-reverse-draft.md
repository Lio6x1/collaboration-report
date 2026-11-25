# 分步编写 `services/collaboration/src/domain/reverse_draft.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/299-reverse-draft.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

计算相对于指定历史前后版本的逆向修改并合入草稿。

撤销历史接纳不应直接把整篇退回旧版本，其他后续改动须保留且冲突可见。

写之前先按这条执行路线理解各段的连接：

1. 先在可靠MAIN上以after为基线、before为逆向提案检查可逆性
2. computePull将草稿对齐当前MAIN
3. 再把相同逆向差异合入对齐草稿
4. 任一步冲突立即返回，成功规范为BRANCH

本文件需要守住的边界：

- 不能跳过第一步主线冲突检查
- before/after必须来自已固定的那次历史变更

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/reverse_draft.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/reverse_draft.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/reverse_draft.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/reverse_draft.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:247 -->
````javascript
import { mergeContent } from './content_merge.mjs';
import { draftBody, computePull } from './draft_protocol.mjs';

// 先证明反向在当前MAIN仍兼容，再把同一修改叠加到可靠草稿；两个阶段都保留其它位置的新增。
````

## 第 2 步：computeReverse

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

任一阶段有重叠修改就停止，不能用历史整篇正文替换当前S2。

本步声明：`export function computeReverse(before, after, draft, main, branchBase) `。

**这一段为什么这样写：** 用三方合并表达逆向差异而非整篇替换。

1. 证明逆向在当前MAIN上可应用
2. 对齐支线
3. 在对齐结果应用逆向差异


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>before</code> | <code>JavaScript 参数</code> | 目标历史操作之前正文<br>在此声明中的实际使用：<code>draftBody(before)</code> |
| <code>after</code> | <code>JavaScript 参数</code> | 目标历史操作之后正文<br>在此声明中的实际使用：<code>draftBody(after)</code> |
| <code>draft</code> | <code>JavaScript 参数</code> | 当前草稿正文<br>在此声明中的实际使用：<code>computePull(branchBase, draft, main)</code> |
| <code>main</code> | <code>JavaScript 参数</code> | 可靠主线正文<br>在此声明中的实际使用：<code>draftBody(main)</code>；<code>computePull(branchBase, draft, main)</code> |
| <code>branchBase</code> | <code>JavaScript 参数</code> | 草稿lineage共同基线<br>在此声明中的实际使用：<code>computePull(branchBase, draft, main)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>reversed</code>；<code>aligned</code>；<code>{ ...result, body: result.body &amp;&amp; draftBody(result.body) }</code> |

**跟着本段代码走一遍：**

- 检查 <code>!reversed.body</code>。条件成立时执行 <code>return reversed;</code>。当前函数在这里结束。
- 检查 <code>!aligned.body</code>。条件成立时执行 <code>return aligned;</code>。当前函数在这里结束。
- 此处通过 <code>return { ...result, body: result.body &amp;&amp; draftBody(result.body) };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=247:1244 -->
````javascript
// 任一阶段有重叠修改就停止，不能用历史整篇正文替换当前S2。
export function computeReverse(before, after, draft, main, branchBase) {
  const current = draftBody(main);
  const reversed = mergeContent({
    base: draftBody(after),
    submitted: draftBody(before),
    main: current,
  });
  if (!reversed.body) return reversed;
  // 支线未拉入的MAIN新增不代表用户删除。先按真实支线基线对齐，再叠加同一反向修改。
  const aligned = computePull(branchBase, draft, main);
  if (!aligned.body) return aligned;
  // 仍叠加原合并的反向，而非“当前MAIN到反向MAIN”的差量；MAIN已经恢复时后者为空，
  // 会错误保留支线里的旧S1。原前后正文才能区分应撤销的S1与后来独立的S2。
  const result = mergeContent({
    base: draftBody(after),
    submitted: draftBody(before),
    main: draftBody(aligned.body),
  });
  return { ...result, body: result.body && draftBody(result.body) };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/reverse_draft.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
