# services/collaboration/src/application/structure_installations.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/344-structure-installations.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`StructureInstallations`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { canonical, normalizeContent, schema, toPM } from '../../../../packages/content/index.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { wireContent } from '../domain/capture_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import * as artifacts from '../repository/draft_transitions.mjs';
import * as gates from '../repository/structure_workspace.mjs';
import * as rooms from '../repository/room_access.mjs';
import * as lineages from '../repository/candidate_artifacts.mjs';
import { draftBody, draftLineage } from '../domain/draft_protocol.mjs';

export class StructureInstallations {
  constructor(snapshots) {
    this.snapshots = snapshots;
  }

  async navigation(connection, operation, transition) {
    const [[prepared]] = await artifacts.artifact(connection, 'STRUCTURE_PREPARATION', operation);
    if (!prepared) throw new RPCFailure(14, 'STRUCTURE_INSTALL_UNKNOWN');
    const [[original]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREVIEW_FIXED',
      json(prepared.payload).authority.reference,
    );
    if (!original) throw new RPCFailure(14, 'STRUCTURE_INSTALL_UNKNOWN');
    const targets = new Map(
      json(original.payload)
        .migration.changes.flatMap((c) => c.targets)
        .map((t) => [t.key, t]),
    );
    // 稳定块只用于定位新编辑入口；新房间正文仍要重新申请真实身份/权限准入。
    return transition.branches.flatMap((branch) => {
      const target = targets.get(branch.candidate_key);
      if (!target) throw new RPCFailure(14, 'STRUCTURE_INSTALL_UNKNOWN');
      return target.source_ranges.map((range) => ({ ...range, unit_id: branch.unit_id }));
    });
  }

  async install(connection, context, transition, mainUnits) {
    const [[row]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREPARATION',
      context.operation_id,
    );
    if (!row || !transition || !Array.isArray(transition.branches))
      throw new RPCFailure(14, 'STRUCTURE_INSTALL_UNKNOWN');
    const prepared = json(row.payload);
    if (
      transition.preview_cut_ref !== prepared.authority.reference ||
      transition.preview_digest !== prepared.authority.digest
    )
      throw new RPCFailure(6, 'STRUCTURE_INSTALL_CONFLICT');
    const [[original]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREVIEW_FIXED',
      prepared.authority.reference,
    );
    if (!original) throw new RPCFailure(14, 'STRUCTURE_INSTALL_UNKNOWN');
    const fixed = json(original.payload),
      targets = new Map(fixed.migration.changes.flatMap((v) => v.targets).map((v) => [v.key, v]));
    if (targets.size !== transition.branches.length)
      throw new RPCFailure(6, 'STRUCTURE_INSTALL_INCOMPLETE');
    const cuts = [];
    for (const branch of transition.branches) {
      const target = targets.get(branch.candidate_key),
        content = wireContent(branch.content);
      const main = mainUnits.find((v) => v.unit_id === branch.unit_id);
      const cut = JSON.parse(content.cut_json_utf8);
      const body = normalizeContent(JSON.parse(content.body_json_utf8));
      if (
        !target ||
        !main ||
        content.document_id !== context.target.document_id ||
        content.operation_id !== context.operation_id ||
        content.unit_id !== branch.unit_id ||
        content.branch_id !== branch.branch_id ||
        content.workspace_epoch !== branch.epoch ||
        content.lineage_ref !== branch.lineage_id ||
        cut.room_id !== branch.room_id ||
        cut.workspace_epoch !== branch.epoch ||
        cut.durable_seq !== '0' ||
        content.hash_algorithm !== 'SHA-256' ||
        !hash(canonical(body)).equals(content.body_hash) ||
        canonical(body) !== canonical(target.draft_body) ||
        Buffer.from(main.content.body_json_utf8).toString() !== canonical(target.main_body) ||
        canonical(draftBody(branch.lineage_context?.body)) !==
          canonical(draftBody(target.main_body)) ||
        branch.lineage_context?.lineage_id !== branch.lineage_id ||
        branch.lineage_context?.base_version_id !== main.content_version_id ||
        canonical(branch.lineage_context?.provenance) !==
          canonical(JSON.parse(content.provenance_json_utf8))
      )
        throw new RPCFailure(6, 'STRUCTURE_INSTALL_CONTENT_INVALID');
      targets.delete(branch.candidate_key);
      const [[existing]] = await rooms.lockRoom(connection, cut.room_id);
      if (existing) throw new RPCFailure(6, 'STRUCTURE_INSTALL_ROOM_CONFLICT');
      await rooms.insertRoom(
        connection,
        cut.room_id,
        context.target.document_id,
        branch.unit_id,
        'BRANCH',
        branch.branch_id,
        branch.epoch,
      );
      const doc = prosemirrorJSONToYDoc(schema, toPM(body));
      try {
        await this.snapshots.checkpoint(
          connection,
          { id: cut.room_id, epoch: cut.workspace_epoch },
          doc,
          '0',
        );
      } finally {
        doc.destroy();
      }
      await lineages.insertRoomLineage(
        connection,
        cut.room_id,
        cut.workspace_epoch,
        draftLineage(branch.lineage_context),
      );
      cuts.push(cut);
    }
    // 新检查点与旧房间退役同TX；原尾部不再可写，旧socket真实退出后才释放安装ACK。
    for (const old of prepared.branches) {
      const [[gate]] = await gates.branchGate(connection, old.room_id, old.workspace_epoch);
      const bound = gate && json(gate.payload);
      if (
        !bound ||
        bound.state !== 'PREPARED' ||
        bound.operation_id !== context.operation_id ||
        bound.generation !== prepared.binding.barrier_generation
      )
        throw new RPCFailure(14, 'STRUCTURE_BRANCH_GATE_UNKNOWN');
      if (old.present) {
        const [[room]] = await rooms.lockRoom(connection, old.room_id);
        if (
          !room ||
          room.state !== 'BLOCKED' ||
          room.epoch !== old.workspace_epoch ||
          room.last_durable_seq !== old.cut.durable_seq
        )
          throw new RPCFailure(14, 'STRUCTURE_PREPARED_BRANCH_UNKNOWN');
        await artifacts.retireRoom(connection, old.room_id, old.workspace_epoch);
        await artifacts.closeGrants(connection, old.room_id);
        await artifacts.stopDeliveries(connection, old.room_id);
      }
      await gates.finishBranchGate(connection, old, prepared.binding, 'INSTALLED');
    }
    return cuts;
  }

  async retiredRooms(connection, operation) {
    const [[row]] = await artifacts.artifact(connection, 'STRUCTURE_PREPARATION', operation);
    return row
      ? json(row.payload)
          .branches.filter((v) => v.present)
          .map((v) => v.room_id)
      : [];
  }
}
````

UTF-8/LF全文SHA-256：`ea4af3d45bf336ae5de63740b06216a462023287602ab652b351a28532f5565e`。[参考实现](../../../../services/collaboration/src/application/structure_installations.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
