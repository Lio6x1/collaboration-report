import { json } from '../domain/shared_protocol.mjs';
import * as resourceConfirmations from '../repository/resource_confirmations.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';

// 来源 confirmation_scheduler.mjs：原业务声明与函数体保留。
// 同一恢复轮不重叠；未知确认保持pin，由既有worker按固定任务继续，不以静默catch冒充成功。
export function startConfirmationRecovery(engine) {
  let recovering = false;
  const timer = setInterval(() => {
    if (recovering) return;
    recovering = true;
    engine
      .recoverResources()
      .catch(() => {})
      .finally(() => {
        recovering = false;
      });
  }, 1000);
  timer.unref();
  return () => clearInterval(timer);
}

// 来源 resource_confirmations.mjs：原业务声明与函数体保留。

// 调度与重试不虚构保存依据；未知结果保留pin，闭来源的资源等待可证明的生命周期清理。
export class ResourceConfirmations {
  constructor(store, peers) {
    this.store = store;
    this.peers = peers;
  }

  async recover(id) {
    const [tasks] = await resourceConfirmations.pendingResourceConfirmations(this.store.pool, id);
    for (const task of tasks) {
      try {
        const receipt = await this.peers.rpc('ConfirmDraftResources', json(task.payload));
        if (receipt.state !== 'CONFIRMED') throw new RPCFailure(14, 'RESOURCE_CONFIRM_UNKNOWN');
        await resourceConfirmations.markResourceConfirmed(this.store.pool, task.business_key);
      } catch (err) {
        // 已闭来源不会被后台任务重开。临时引用保持pin，闭代次的清理由后续可证明的生命周期协议接管。
        const terminal = err instanceof RPCFailure && [7, 9].includes(err.code);
        await resourceConfirmations.retryResourceConfirmation(
          this.store.pool,
          terminal ? 'SUCCEEDED' : 'RETRY',
          terminal ? 'PINNED_AWAITING_CLEANUP' : 'RESOURCE_CONFIRM_UNKNOWN',
          task.business_key,
        );
      }
    }
  }
}
