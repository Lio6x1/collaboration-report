# services/collaboration/src/worker/merge_thread.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/354-merge-thread.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：完整定义见下方源码。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { parentPort, workerData } from 'node:worker_threads';
import { mergeContent } from '../domain/content_merge.mjs';

// worker只做纯计算，不接触数据库或权限状态；终止计算不会留下未知提交。
try {
  parentPort.postMessage({ result: mergeContent(workerData) });
} catch {
  parentPort.postMessage({ error: 'MERGE_COMPUTE_INVALID' });
}
````

UTF-8/LF全文SHA-256：`8c81eb7922d1fcd46c32442387e2761c7f5de6b174b00cd3734b274cfb29ced0`。[参考实现](../../../../services/collaboration/src/worker/merge_thread.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
