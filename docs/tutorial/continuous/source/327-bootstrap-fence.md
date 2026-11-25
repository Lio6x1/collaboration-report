# services/collaboration/src/application/bootstrap_fence.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/327-bootstrap-fence.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`bootstrapFence`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { binding, ack, assertSameBinding } from '../domain/fence_protocol.mjs';
import * as repository from '../repository/fences.mjs';

// 只保留历史空启动协议作为具名安全边界；真实服务使用完整排空器，非空库绝不能空ACK。
export async function bootstrapFence(store, req) {
  const current = binding(req),
    scope = req.closed_scopes[0],
    connection = await store.pool.getConnection();
  try {
    await repository.startBootstrap(connection);
    const prior = await repository.lockedClosure(connection, req.revocation_id);
    if (prior) {
      assertSameBinding(prior, scope, current);
      if (prior.state === 'SUCCEEDED') return ack(req);
    }
    for await (const count of repository.bootstrapBusinessCounts(connection))
      if (String(count) !== '0') throw new RPCFailure(9, 'FULL_OUTPUT_DRAIN_REQUIRED');
    if (!prior) await repository.insertClosure(connection, req.revocation_id, scope, current);
    await repository.blockSource(connection, req.revocation_id, scope);
    const result = ack(req);
    await repository.completeClosure(
      connection,
      req.revocation_id,
      scope,
      result.result_digest,
      true,
    );
    await repository.commitBootstrap(connection);
    return result;
  } catch (err) {
    if (err instanceof RPCFailure) throw err;
    throw new RPCFailure(14, 'FENCE_COMMIT_UNKNOWN');
  } finally {
    try {
      await repository.rollbackBootstrap(connection);
    } catch {}
    connection.release();
  }
}
````

UTF-8/LF全文SHA-256：`bb2c11df134660d1a8baa159511e42d88dc84b58b2372ade6f92a2ba1babc7e2`。[参考实现](../../../../services/collaboration/src/application/bootstrap_fence.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
