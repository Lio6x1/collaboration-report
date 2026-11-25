# services/collaboration/src/application/runtime_metrics.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/342-runtime-metrics.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`RuntimeMetrics`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { monitorEventLoopDelay, performance } from 'node:perf_hooks';

// 只计进程与队列聚合事实，不记录正文、房间号或身份。指标用于单机容量
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

UTF-8/LF全文SHA-256：`9216e7056dada3ce3ea17288d7e254ec23fbd9d1e2e38bd9ed6c29e960d23d5f`。[参考实现](../../../../services/collaboration/src/application/runtime_metrics.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
