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
