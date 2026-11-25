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
