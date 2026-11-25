# 分步编写 `services/collaboration/src/main.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/364-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保持监督器识别的固定Node入口，捕获顶层启动失败并非零退出。

Windows进程owner探测依赖绝对入口和启动标记，入口移动会破坏退出证明。

写之前先按这条执行路线理解各段的连接：

1. 调用bootstrap.run
2. 失败固定打印SERVICE_START_FAILED
3. 诊断模式只允许有界大写稳定cause_code，否则BOOTSTRAP_UNKNOWN
4. process.exit(1)让监督器观察失败

本文件需要守住的边界：

- 不要输出error.stack或驱动message，其中可能包含SQL/连接配置
- 不能吞启动异常以0退出

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/main.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/main.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/main.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/main.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:48 -->
````javascript
import { run } from './bootstrap/runtime.mjs';

````

## 第 2 步：statement-101

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

保留Windows监督登记的绝对入口及--run-owner参数；仅启动失败稳定码可跨出进程边界。

本步声明：`run().catch((error) => {`。

<!-- write-step: 2 bytes=48:775 -->
````javascript
// 保留Windows监督登记的绝对入口及--run-owner参数；仅启动失败稳定码可跨出进程边界。
run().catch((error) => {
  console.error(JSON.stringify({ code: 'SERVICE_START_FAILED', role: 'collaboration' }));
  // 故障定位只输出有界稳定码；驱动原文/堆栈可能包含SQL、路径和私有连接配置。
  if (process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === '1') {
    const candidate = error?.code ?? error?.message;
    const cause =
      typeof candidate === 'string' && /^[A-Z][A-Z0-9_]{1,95}$/.test(candidate)
        ? candidate
        : 'BOOTSTRAP_UNKNOWN';
    console.error(JSON.stringify({ event: 'collaboration_startup_failed', cause_code: cause }));
  }
  process.exit(1);
});
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/main.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
