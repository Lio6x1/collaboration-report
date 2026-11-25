# 分步编写 `services/collaboration/src/domain/resolution_draft.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/298-resolution-draft.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把审核解决稿作为相对送审稿的修改，三方合入当前支线草稿。

直接用解决稿覆盖草稿会丢掉送审后新增的用户编辑。

写之前先按这条执行路线理解各段的连接：

1. submitted和draft按BRANCH规则校验
2. resolution校验失败映射RESOLUTION_CONTENT_INVALID
3. 以submitted为base、resolution为submitted、draft为main调用mergeContent
4. 成功结果再次规范为BRANCH正文

本文件需要守住的边界：

- 草稿后续修改与解决稿重叠必须返回冲突
- 解决稿不能带MAIN专有font标记

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/resolution_draft.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/resolution_draft.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/resolution_draft.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/resolution_draft.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:273 -->
````javascript
import { normalizeContent } from '../../../../packages/content/index.mjs';
import { mergeContent } from './content_merge.mjs';
import { bad } from './shared_protocol.mjs';

// 修复页面基于旧送审快照，操作只叠加“原submitted→人工结果”的修改。
````

## 第 2 步：computeResolution

插入位置：第 1 步代码之后，紧接追加；完成稿约第 6 行。

最新可靠草稿中的独立S2由三方规则保留；同处竞争返回冲突，不能整篇覆盖当前房间。

本步声明：`export function computeResolution(submitted, resolution, draft) `。

**这一段为什么这样写：** 保留送审之后的草稿尾部编辑。

1. 规范三份输入
2. 计算解决稿相对送审稿的改动
3. 三方合并到当前草稿


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>submitted</code> | <code>JavaScript 参数</code> | 产生审核冲突的固定送审正文<br>在此声明中的实际使用：<code>&#91;submitted, draft&#93;.map</code> |
| <code>resolution</code> | <code>JavaScript 参数</code> | 用户给出的解决正文<br>在此声明中的实际使用：<code>normalizeContent(resolution, { workspaceKind: &#x27;BRANCH&#x27; })</code> |
| <code>draft</code> | <code>JavaScript 参数</code> | 当前可靠草稿正文<br>在此声明中的实际使用：<code>&#91;submitted, draft&#93;.map</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     ...result,     body: result.body &amp;&amp; normalizeContent(result.body, { workspaceKind: &#x27;BRANCH&#x27; }),   }</code> |

**跟着本段代码走一遍：**

- 执行 <code>throw bad(&#x27;RESOLUTION_CONTENT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {     ...result,     body: result.body &amp;&amp; normalizeContent(result.body, { workspaceKind: &#x27;BRANCH&#x27; }),   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=273:1050 -->
````javascript
// 最新可靠草稿中的独立S2由三方规则保留；同处竞争返回冲突，不能整篇覆盖当前房间。
export function computeResolution(submitted, resolution, draft) {
  const [base, current] = [submitted, draft].map((body) =>
    normalizeContent(body, { workspaceKind: 'BRANCH' }),
  );
  let proposed;
  try {
    proposed = normalizeContent(resolution, { workspaceKind: 'BRANCH' });
  } catch {
    // 用户正文的确定错误可以沿原号拒绝；固定submitted/draft坏掉仍保留未知来源错误。
    throw bad('RESOLUTION_CONTENT_INVALID');
  }
  const result = mergeContent({ base, submitted: proposed, main: current });
  return {
    ...result,
    body: result.body && normalizeContent(result.body, { workspaceKind: 'BRANCH' }),
  };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/resolution_draft.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
