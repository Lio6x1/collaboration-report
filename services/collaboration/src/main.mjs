import { run } from './bootstrap/runtime.mjs';

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
