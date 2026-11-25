# services/collaboration/src/worker/output_recovery.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/355-output-recovery.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`OutputRecovery`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { resolve } from 'node:path';
import * as outputInstances from '../repository/output_instances.mjs';
import { json } from '../domain/shared_protocol.mjs';

// 恢复旧实例输出、全部连续房间日志及资源任务后才允许就绪，不能把旧持久房间当作空启动。
export class OutputRecovery {
  constructor(
    store,
    cfg,
    instance,
    transactions,
    snapshots,
    confirmations,
    processOwner,
    startupStage = () => {},
  ) {
    this.store = store;
    this.cfg = cfg;
    this.instance = instance;
    this.transactions = transactions;
    this.snapshots = snapshots;
    this.confirmations = confirmations;
    this.processOwner = processOwner;
    this.startupStage = startupStage;
    this.recovered = false;
  }

  async initialize() {
    // 超时不是进程退出证据。指定PID、绝对入口和随机启动标记三者共同确定旧输出所有者。
    const [instances] = await outputInstances.listOutputInstances(this.store.pool);
    for (const old of instances) {
      const facts = json(old.payload);
      if (old.business_key === this.instance || facts.exit_proof) continue;
      facts.exit_proof = this.processOwner.exitProof(facts);
      await this.transactions.run(async (c) => {
        await outputInstances.settleExitedOutputs(c, old.business_key);
        await outputInstances.recordOutputExit(c, JSON.stringify(facts), old.business_key);
      });
    }
    const facts = {
      pid: process.pid,
      entry: resolve(this.cfg.root, 'services/collaboration/src/main.mjs'),
      owner: process.env.COLLAB_RUN_OWNER,
    };
    await outputInstances.insertOutputInstance(
      this.store.pool,
      this.instance,
      JSON.stringify(facts),
    );
    this.startupStage('OLD_OUTPUTS_RECOVERED');
    await this.snapshots.verifyRooms();
    this.startupStage('ROOMS_VERIFIED');
    await this.confirmations.recover();
    this.startupStage('CONFIRMATIONS_RECOVERED');
    this.recovered = true;
    this.store.roomsRecovered = true;
  }
}
````

UTF-8/LF全文SHA-256：`1522caccc17de774251ec5cf9caeeb6a3742bf288c481b859cc376b2fd809cc5`。[参考实现](../../../../services/collaboration/src/worker/output_recovery.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
