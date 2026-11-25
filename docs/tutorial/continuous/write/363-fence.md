# 分步编写 `services/collaboration/src/fence.mjs`

先按 [C03 · 撤权门闩、真实输出排空与进程恢复](../lessons/collaboration/03-fence-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/363-fence.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

维持既有调用面，重导出拆分后的撤权规则与用例。

测试和装配可以继续导入fence.mjs，同时SQL、领域校验和实际排空各自归属清晰。

写之前先按这条执行路线理解各段的连接：

1. 导出RPCFailure及binding/ack
2. 导出完整fenceRooms和受限bootstrapFence

本文件需要守住的边界：

- 这个文件没有自己的撤权逻辑；改协议应修改对应领域/应用源文件
- 正式运行必须装配完整排空引擎，不能因为兼容导出而使用空启动路径

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/fence.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/fence.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/fence.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/fence.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：statement-45

插入位置：空文件第 1 行。

兼容既有测试/服务调用面；规则、SQL及真实排空已经分别归属领域、仓储和应用用例。

本步声明：`export { RPCFailure } from './domain/shared_protocol.mjs';`。

<!-- write-step: 1 bytes=0:178 -->
````javascript
// 兼容既有测试/服务调用面；规则、SQL及真实排空已经分别归属领域、仓储和应用用例。
export { RPCFailure } from './domain/shared_protocol.mjs';
````

## 第 2 步：statement-104

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`export { binding, ack } from './domain/fence_protocol.mjs';`。

<!-- write-step: 2 bytes=178:238 -->
````javascript
export { binding, ack } from './domain/fence_protocol.mjs';
````

## 第 3 步：statement-164

插入位置：第 2 步代码之后，紧接追加；完成稿约第 4 行。

本步声明：`export { fenceRooms } from './application/fences.mjs';`。

<!-- write-step: 3 bytes=238:293 -->
````javascript
export { fenceRooms } from './application/fences.mjs';
````

## 第 4 步：statement-219

插入位置：第 3 步代码之后，紧接追加；完成稿约第 5 行。

本步声明：`export { bootstrapFence } from './application/bootstrap_fence.mjs';`。

<!-- write-step: 4 bytes=293:361 -->
````javascript
export { bootstrapFence } from './application/bootstrap_fence.mjs';
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/fence.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/03-fence-recovery.md)。
