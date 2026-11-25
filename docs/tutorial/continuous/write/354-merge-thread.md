# 分步编写 `services/collaboration/src/worker/merge_thread.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/354-merge-thread.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在线程内运行纯mergeContent并通过parentPort回传结果。

计算线程不应拥有数据库或权限状态，才能在预算超时后安全终止。

写之前先按这条执行路线理解各段的连接：

1. 读取workerData固定输入
2. mergeContent成功回传result
3. 捕获异常仅返回MERGE_COMPUTE_INVALID

本文件需要守住的边界：

- 不能在此插候选SQL或调用Doc，否则terminate可能留下未知副作用
- 主线程负责输入验证和持久结果

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/worker/merge_thread.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/worker/merge_thread.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/worker/merge_thread.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/worker/merge_thread.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:123 -->
````javascript
import { parentPort, workerData } from 'node:worker_threads';
import { mergeContent } from '../domain/content_merge.mjs';

````

## 第 2 步：statement-164

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

worker只做纯计算，不接触数据库或权限状态；终止计算不会留下未知提交。

本步声明：`try {`。

<!-- write-step: 2 bytes=123:370 -->
````javascript
// worker只做纯计算，不接触数据库或权限状态；终止计算不会留下未知提交。
try {
  parentPort.postMessage({ result: mergeContent(workerData) });
} catch {
  parentPort.postMessage({ error: 'MERGE_COMPUTE_INVALID' });
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/worker/merge_thread.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
