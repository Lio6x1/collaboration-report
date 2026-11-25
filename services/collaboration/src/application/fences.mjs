import { binding, ack, assertSameBinding } from '../domain/fence_protocol.mjs';
import * as repository from '../repository/fences.mjs';

// 撤权分成已提交门闩、真实输出排空、终态三个阶段；ACK永远在socket停止之后持久登记。
export async function fenceRooms(engine, req) {
  const current = binding(req),
    scope = req.closed_scopes[0];
  const done = await engine.transaction(async (connection) => {
    const prior = await repository.lockedClosure(connection, req.revocation_id);
    if (prior) {
      assertSameBinding(prior, scope, current);
      if (prior.state === 'SUCCEEDED') return true;
    } else await repository.insertClosure(connection, req.revocation_id, scope, current);
    await repository.blockSource(connection, req.revocation_id, scope);
    await repository.closeGrants(connection, scope);
    await repository.closeReceipts(connection, scope);
    return false;
  });
  if (done) return ack(req);
  await engine.stopScope(scope);
  const result = ack(req);
  await engine.transaction((connection) =>
    repository.completeClosure(connection, req.revocation_id, scope, result.result_digest),
  );
  return result;
}
