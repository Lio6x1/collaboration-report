# services/collaboration/src/worker/resource_recovery.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/356-resource-recovery.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`startConfirmationRecovery`、`ResourceConfirmations`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`678574419c19d11fe408534510e5619237e3a9e131fb9e4ab7f6e7a3ecadfdf2`。[参考实现](../../../../services/collaboration/src/worker/resource_recovery.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
