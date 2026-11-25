# services/collaboration/src/worker/fence_recovery.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/353-fence-recovery.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`recoverFences`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { pendingClosures } from '../repository/fences.mjs';
import { fenceRooms } from '../application/fences.mjs';

// 启动前消费原持久目标；监听开放前完成所有未知撤权，不能根据当前权限重写旧计划。
export async function recoverFences(pool, engine) {
  for (const pending of await pendingClosures(pool)) {
    const p =
      typeof pending.target_permission === 'string'
        ? JSON.parse(pending.target_permission)
        : pending.target_permission;
    await fenceRooms(engine, {
      revocation_id: pending.revocation_id,
      closed_scopes: [{ scope_key: pending.scope_key, epoch: String(pending.epoch) }],
      generation: p.generation,
      target_digest: Buffer.from(p.bootstrap_target_digest, 'hex'),
    });
  }
}
````

UTF-8/LF全文SHA-256：`1035b97ee76867cf3638db9cbeabb952a3fd65bba58cec0dec9a11ff0cc788ff`。[参考实现](../../../../services/collaboration/src/worker/fence_recovery.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
