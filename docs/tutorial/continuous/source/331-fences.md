# services/collaboration/src/application/fences.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/331-fences.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`fenceRooms`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`56050c302a881bc40c721796d66e22a9a412b11fb54645d3e5633af208eb0903`。[参考实现](../../../../services/collaboration/src/application/fences.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
