# services/collaboration/src/main.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/364-main.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：完整定义见下方源码。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`9e441053c9e4625eb5b39c8f5fbf7c5eed9b6eb7f8034266456acb14d3c1a80b`。[参考实现](../../../../services/collaboration/src/main.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
