import * as repository from '../repository/persistence_outcome.mjs';
export class PersistenceOutcome {
  constructor(pool) {
    this.pool = pool;
  }
  async update(input) {
    const row = await repository.committedUpdate(this.pool, input);
    let task;
    if (row) {
      const tasks = await repository.confirmationsForReceipt(
        this.pool,
        input.room_id + ':' + input.workspace_epoch + ':' + row.seq,
      );
      if (tasks.length === 1) task = tasks[0];
    }
    const binding = task
      ? typeof task.payload === 'string'
        ? JSON.parse(task.payload)
        : task.payload
      : null;
    // 行已提交但唯一保护绑定不完整仍返回UNKNOWN，不能把不确定证明转换成可释放资源的成功。
    return row && binding?.grant_id
      ? {
          state: 'COMMITTED',
          durable_seq: row.seq,
          update_hash: row.payload_hash.toString('hex'),
          protection_operation_id: task.business_key,
          grant_id: binding.grant_id,
          body_bytes: binding.body_bytes,
          body_hash: binding.body_hash,
        }
      : { state: 'UNKNOWN' };
  }
}
