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
