# services/collaboration/src/application/persistence_outcome.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/335-persistence-outcome.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`PersistenceOutcome`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`f5cf2308b71ad1013009cefa0713838edf4eb5dfa18a8680574aebfb6d3d1b7d`。[参考实现](../../../../services/collaboration/src/application/persistence_outcome.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
