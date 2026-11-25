# services/collaboration/src/application/structure_preparations.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/345-structure-preparations.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`StructurePreparations`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { structurePreparation } from '../domain/structure_preparation.mjs';
import { cutDigest } from '../domain/main_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import * as artifacts from '../repository/draft_transitions.mjs';
import * as repository from '../repository/structure_workspace.mjs';
import * as previews from '../repository/structure_workspace.mjs';
import * as lineages from '../repository/candidate_artifacts.mjs';
import { draftLineage } from '../domain/draft_protocol.mjs';

export class StructurePreparations {
  async plan(connection, binding, authority) {
    const [[row]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREVIEW_FIXED',
      authority.reference,
    );
    if (!row) throw new RPCFailure(14, 'STRUCTURE_PREPARATION_UNKNOWN');
    return structurePreparation(json(row.payload), authority, binding);
  }

  async prepare(connection, binding, authority, mainRooms) {
    const plan = await this.plan(connection, binding, authority);
    if (canonical(plan.cut) !== canonical(binding.expected_cut))
      throw new RPCFailure(6, 'STRUCTURE_PREVIEW_CUT_CONFLICT');
    const [branchRooms] = await previews.lockBranchRooms(
      connection,
      binding.context.target.document_id,
    );
    const actual = mainRooms.map((r) => ({
      room_id: r.id,
      workspace_epoch: r.epoch,
      durable_seq: r.last_durable_seq,
    }));
    for (const branch of plan.branches) {
      const found = branchRooms.filter((r) => r.unit_id === branch.unit_id);
      const room = found[0];
      // 预览时不存在的房间若后来OPEN，零尾也属于切点变化，不能当作原虚拟基线。
      if (found.length > 1 || !!room !== branch.present) return { state: 'CHANGED', cut: plan.cut };
      if (room) {
        if (
          room.id !== branch.room_id ||
          room.workspace_id !== branch.branch_id ||
          room.epoch !== branch.workspace_epoch ||
          room.state !== 'OPEN'
        )
          return { state: 'CHANGED', cut: plan.cut };
        const [[lineage]] = await lineages.lockRoomLineage(connection, room.id, room.epoch);
        if (
          !lineage ||
          canonical(draftLineage(json(lineage.payload))) !== canonical(draftLineage(branch.lineage))
        )
          return { state: 'CHANGED', cut: plan.cut };
        actual.push({
          room_id: room.id,
          workspace_epoch: room.epoch,
          durable_seq: room.last_durable_seq,
        });
      }
    }
    actual.sort((a, b) => (BigInt(a.room_id) < BigInt(b.room_id) ? -1 : 1));
    const cut = { ...plan.cut, rooms: actual, dependency_digest: cutDigest(actual) };
    if (canonical(cut) !== canonical(plan.cut)) return { state: 'CHANGED', cut };
    for (const branch of plan.branches) {
      const [[prior]] = await repository.branchGate(
        connection,
        branch.room_id,
        branch.workspace_epoch,
      );
      if (prior && json(prior.payload).state !== 'ABORTED')
        throw new RPCFailure(14, 'STRUCTURE_BRANCH_GATE_PENDING');
      await repository.prepareBranchGate(connection, branch, binding);
      if (branch.present)
        await artifacts.blockRoom(connection, branch.room_id, branch.workspace_epoch);
    }
    await artifacts.insertArtifact(
      connection,
      'STRUCTURE_PREPARATION',
      binding.context.operation_id,
      { binding, authority, branches: plan.branches },
    );
    return { state: 'PREPARED', cut };
  }

  async abort(connection, binding) {
    const [[row]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREPARATION',
      binding.context.operation_id,
    );
    if (!row) return;
    const stored = json(row.payload);
    if (canonical(stored.binding) !== canonical(binding))
      throw new RPCFailure(6, 'STRUCTURE_PREPARATION_CONFLICT');
    for (const branch of stored.branches) {
      await repository.finishBranchGate(connection, branch, binding, 'ABORTED');
      if (branch.present)
        await artifacts.openRoom(connection, branch.room_id, branch.workspace_epoch);
    }
  }
}
````

UTF-8/LF全文SHA-256：`546210ef1012f364c81ac31e9fbbbc08182604a643602c2959381d230ef7d687`。[参考实现](../../../../services/collaboration/src/application/structure_preparations.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
