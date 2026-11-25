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
