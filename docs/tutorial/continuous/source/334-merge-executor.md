# services/collaboration/src/application/merge_executor.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/334-merge-executor.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`MergeExecutor`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { Worker } from 'node:worker_threads';
import { RPCFailure } from '../domain/shared_protocol.mjs';

// 有限计算线程保护WSS、来源gate和自然到期调度；队列满时由原Go任务重试。
export class MergeExecutor {
  constructor() {
    this.active = 0;
  }

  async compute(input) {
    if (this.active >= 2) throw new RPCFailure(14, 'MERGE_COMPUTE_BUSY');
    this.active++;
    try {
      const worker = new Worker(new URL('../worker/merge_thread.mjs', import.meta.url), {
        workerData: input,
        resourceLimits: { maxOldGenerationSizeMb: 256, stackSizeMb: 4 },
      });
      let timer;
      try {
        return await new Promise((resolve, reject) => {
          worker.once('message', (value) =>
            value.result
              ? resolve(value.result)
              : reject(new RPCFailure(14, 'MERGE_COMPUTE_INVALID')),
          );
          worker.once('error', () => reject(new RPCFailure(14, 'MERGE_WORKER_FAILED')));
          worker.once('exit', (code) => {
            if (code !== 0) reject(new RPCFailure(14, 'MERGE_WORKER_FAILED'));
          });
          timer = setTimeout(
            () =>
              resolve({
                body: null,
                conflicts: [
                  {
                    block_id: '',
                    node_path: '$root',
                    kind: 'MERGE_COMPLEXITY_LIMIT',
                    reason: '固定输入超出自动计算时间预算，请分段处理。',
                  },
                ],
              }),
            1500,
          );
        });
      } finally {
        clearTimeout(timer);
        await worker.terminate();
      }
    } finally {
      this.active--;
    }
  }
}
````

UTF-8/LF全文SHA-256：`0b0feb387bc4c1a85572d659dc083cf91a05a741f3ea95ec13ef8b2d859e7c99`。[参考实现](../../../../services/collaboration/src/application/merge_executor.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
