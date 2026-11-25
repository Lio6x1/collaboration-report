# 分步编写 `services/collaboration/src/application/runtime_metrics.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/342-runtime-metrics.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供不含正文和身份的进程、事件循环与房间队列聚合指标。

单机容量排查需要区分CPU、内存和排队，不应通过读取正文或触发恢复观察性能。

写之前先按这条执行路线理解各段的连接：

1. 构造时开启20ms事件循环延迟监控
2. snapshot读取CPU/内存/事件循环利用率及p95，纳秒转毫秒
3. 附queue.metrics展示pending/active/waiting峰值

本文件需要守住的边界：

- 指标不得改变保存、授权或关闭分支
- 不输出room ID、用户身份或正文

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/runtime_metrics.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/runtime_metrics.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/runtime_metrics.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/runtime_metrics.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:174 -->
````javascript
import { monitorEventLoopDelay, performance } from 'node:perf_hooks';

// 只计进程与队列聚合事实，不记录正文、房间号或身份。指标用于单机容量
````

## 第 2 步：RuntimeMetrics

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

判断，不能改变任何保存/权限/关闭分支；读取也不触发业务SQL或恢复。

本步声明：`export class RuntimeMetrics `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>queue</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.queue = queue</code> |
| <code>delay</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.delay = monitorEventLoopDelay({ resolution: 20 })</code> |
| <code>loopBaseline</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.loopBaseline = performance.eventLoopUtilization()</code> |
| <code>startedAt</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.startedAt = performance.now()</code> |

**本段进入 RuntimeMetrics.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>queue</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.queue = queue</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 RuntimeMetrics.snapshot：**

**这一段为什么这样写：** 读取无业务副作用的运行聚合快照。

1. 读取进程计数器
2. 规范有限数值和时间单位
3. 组合房间队列指标


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{       uptimeMs: Math.round(performance.now() - this.startedAt),       cpuUserMicros: cpu.user,       cpuSystemMicros: cpu.system,       rssBytes: memory.rss,       heapUsedBytes: …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return {       uptimeMs: Math.round(performance.now() - this.startedAt),       cpuUserMicros: cpu.user,       cpuSystemMicros: cpu.system,       rssBytes: memory.rss,       heapUse …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=174:1310 -->
````javascript
// 判断，不能改变任何保存/权限/关闭分支；读取也不触发业务SQL或恢复。
export class RuntimeMetrics {
  constructor(queue) {
    this.queue = queue;
    this.delay = monitorEventLoopDelay({ resolution: 20 });
    this.delay.enable();
    this.loopBaseline = performance.eventLoopUtilization();
    this.startedAt = performance.now();
  }

  snapshot() {
    const cpu = process.cpuUsage();
    const memory = process.memoryUsage();
    const loop = performance.eventLoopUtilization(this.loopBaseline);
    const milliseconds = (value) => (Number.isFinite(value) ? Number((value / 1e6).toFixed(2)) : 0);
    return {
      uptimeMs: Math.round(performance.now() - this.startedAt),
      cpuUserMicros: cpu.user,
      cpuSystemMicros: cpu.system,
      rssBytes: memory.rss,
      heapUsedBytes: memory.heapUsed,
      eventLoopUtilization: Number(loop.utilization.toFixed(4)),
      eventLoopDelayMeanMs: milliseconds(this.delay.mean),
      eventLoopDelayMaxMs: milliseconds(this.delay.max),
      eventLoopDelayP95Ms: milliseconds(this.delay.percentile(95)),
      queue: this.queue.metrics(),
    };
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/runtime_metrics.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
