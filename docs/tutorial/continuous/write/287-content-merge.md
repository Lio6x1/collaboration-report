# 分步编写 `services/collaboration/src/domain/content_merge.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/287-content-merge.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

统一三方正文合并入口，只有零冲突且最终结构合法才返回body。

即使局部编辑都不冲突，组合后的表格几何和正文schema仍可能非法。

写之前先按这条执行路线理解各段的连接：

1. 先按MAIN规则规范base/submitted/main
2. 处理送审未改、两侧同改和主线未改的快捷分支
3. MergeBlocks+MergeValues递归合并
4. 无冲突后再次normalizeContent；失败新增INVALID_MERGE_RESULT
5. 有任意冲突统一返回body:null

本文件需要守住的边界：

- 不能拿内部暂存正文在存在conflicts时安装
- MAIN字体由合并规则保留，支线语义比较不等于可以覆盖正式样式

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/content_merge.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/content_merge.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/content_merge.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/content_merge.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:215 -->
````javascript
import { normalizeContent } from '../../../../packages/content/index.mjs';
import { MergeValues, clone, same, equal, semantic, hasFont } from './merge/values.mjs';
import { MergeBlocks } from './merge/blocks.mjs';

````

## 第 2 步：mergeContent

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

三方计算只消费固定业务正文。任何冲突都不交付可安装候选，调用方不能偷偷选择保底主线。

本步声明：`export function mergeContent({ base, submitted, main }) `。

**这一段为什么这样写：** 提供纯计算的固定输入→候选/冲突输出。

1. 规范三份输入
2. 合并结构与字段
3. 重新校验最终正文
4. 返回{body,conflicts}


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>{ base, submitted, main }</code> | <code>解构对象</code> | 同一内容目标的共同基线、已冻结送审、可靠主线三份正文<br>在此声明中的实际使用：<code>&#91;base, submitted, main&#93; = &#91;base, submitted, main&#93;.map(validate)</code>；<code>&#91;base, submitted, main&#93;.map</code>；<code>same(base, submitted)</code>；<code>same(main, submitted)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ body: clone(main), conflicts: &#91;&#93; }</code>；<code>{ body: validate(semantic(submitted)), conflicts: &#91;&#93; }</code>；<code>{ body: validate(result), conflicts: &#91;&#93; }</code>；<code>{ body: null, conflicts: values.conflicts }</code> |

**跟着本段代码走一遍：**

- 检查 <code>same(base, submitted) &#124;&#124; same(main, submitted)</code>。条件成立时执行 <code>return { body: clone(main), conflicts: &#91;&#93; };</code>。当前函数在这里结束。
- 检查 <code>equal(base, main) &amp;&amp; !hasFont(main)</code>。条件成立时执行 <code>return { body: validate(semantic(submitted)), conflicts: &#91;&#93; };</code>。当前函数在这里结束。
- 检查 <code>!values.conflicts.length</code>。
- 此处通过 <code>return { body: validate(result), conflicts: &#91;&#93; };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return { body: null, conflicts: values.conflicts };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=215:1235 -->
````javascript
// 三方计算只消费固定业务正文。任何冲突都不交付可安装候选，调用方不能偷偷选择保底主线。
export function mergeContent({ base, submitted, main }) {
  const validate = (body) => normalizeContent(body, { workspaceKind: 'MAIN' });
  [base, submitted, main] = [base, submitted, main].map(validate);
  if (same(base, submitted) || same(main, submitted)) return { body: clone(main), conflicts: [] };
  if (equal(base, main) && !hasFont(main))
    return { body: validate(semantic(submitted)), conflicts: [] };
  const values = new MergeValues();
  const result = { type: 'doc', content: new MergeBlocks(values).roots(base, submitted, main) };
  if (!values.conflicts.length) {
    try {
      return { body: validate(result), conflicts: [] };
    } catch {
      values.report(
        'INVALID_MERGE_RESULT',
        '',
        '$root',
        '组合后的正文无法通过结构与表格几何校验。',
      );
    }
  }
  return { body: null, conflicts: values.conflicts };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/content_merge.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
