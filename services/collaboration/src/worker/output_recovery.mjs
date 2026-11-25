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
