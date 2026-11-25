import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { canonical, fromPM, normalizeContent } from '../../../../packages/content/index.mjs';
import { structurePreviewBinding } from '../domain/structure_preview_protocol.mjs';
import { buildStructureMigration } from '../domain/structure_migration.mjs';
import { fixedDraft, draftLineage } from '../domain/draft_protocol.mjs';
import { storeContent } from '../domain/capture_protocol.mjs';
import { storedCut, wireCut, cutDigest } from '../domain/main_protocol.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { lockSources } from './admission_gates.mjs';
import { requireMainOpen } from './admission_gates.mjs';
import * as artifacts from '../repository/draft_transitions.mjs';
import * as repository from '../repository/structure_workspace.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';

export class StructurePreviews {
  constructor(transactions, snapshots, peers, mainCaptures) {
    Object.assign(this, { transactions, snapshots, peers, mainCaptures });
  }

  async capture(input) {
    const binding = structurePreviewBinding(input),
      context = input.context;
    const prior = await this.transactions.run(async (c) => {
      await lockSources(c, context.sources);
      const [[row]] = await artifacts.artifact(c, 'STRUCTURE_PREVIEW_FIXED', context.operation_id);
      return row && this.replay(json(row.payload), binding);
    });
    if (prior) return prior;
    // Doc只提供已登记原号的完整管理范围。mTLS不能代替当前MANAGE或允许任意房间查询。
    const source = await this.peers.http('/infra/structure/context', binding);
    return this.transactions.run(async (c) => {
      await lockSources(c, context.sources);
      await requireMainOpen(c, context.target.document_id);
      const [[old]] = await artifacts.artifact(c, 'STRUCTURE_PREVIEW_FIXED', context.operation_id);
      if (old) return this.replay(json(old.payload), binding);
      const expected = {
        context,
        expected_manifest_id: source.manifest_id,
        expected_structure_id: context.target.structure_id,
        expected_style_id: source.style_id,
        expected_control_revision: context.target.workspace_epoch,
        usage: 'STRUCTURE_PREVIEW',
      };
      const main = await this.mainCaptures.captureCurrent(
        c,
        expected,
        source,
        context.operation_id + ':STRUCTURE_PREVIEW_MAIN',
        binding,
      );
      if (!Array.isArray(source.drafts) || source.drafts.length !== main.units.length)
        throw new RPCFailure(14, 'STRUCTURE_BRANCH_SCOPE_UNKNOWN');
      const [rooms] = await repository.lockBranchRooms(c, context.target.document_id);
      const seen = new Set(),
        cuts = [...main.cut.rooms],
        drafts = [];
      for (const unit of source.drafts) {
        if (seen.has(unit.unit_id) || !main.units.some((v) => v.unit_id === unit.unit_id))
          throw new RPCFailure(14, 'STRUCTURE_BRANCH_SCOPE_UNKNOWN');
        seen.add(unit.unit_id);
        const matching = rooms.filter((r) => r.unit_id === unit.unit_id);
        if (matching.length > 1) throw new RPCFailure(14, 'STRUCTURE_BRANCH_SCOPE_UNKNOWN');
        const room = matching[0];
        let body = normalizeContent(unit.lineage.body),
          cut = { room_id: unit.room_id, workspace_epoch: unit.workspace_epoch, durable_seq: '0' };
        if (room) {
          if (
            room.id !== unit.room_id ||
            room.workspace_id !== unit.branch_id ||
            room.epoch !== unit.workspace_epoch ||
            room.state !== 'OPEN'
          )
            throw new RPCFailure(14, 'STRUCTURE_BRANCH_CHANGED');
          const [[row]] = await captures.lockRoomLineage(c, room.id, room.epoch);
          if (!row || canonical(draftLineage(json(row.payload))) !== canonical(unit.lineage))
            throw new RPCFailure(14, 'STRUCTURE_LINEAGE_CHANGED');
          cut = {
            room_id: room.id,
            workspace_epoch: room.epoch,
            durable_seq: room.last_durable_seq,
          };
          const doc = await this.snapshots.restore(c, room, room.last_durable_seq);
          try {
            body = normalizeContent(fromPM(yDocToProsemirrorJSON(doc)));
          } finally {
            doc.destroy();
          }
          cuts.push(cut);
        }
        const scoped = {
          ...context,
          target: {
            ...context.target,
            unit_id: unit.unit_id,
            branch_id: unit.branch_id,
            workspace_epoch: unit.workspace_epoch,
          },
        };
        drafts.push(
          fixedDraft(
            body,
            scoped,
            cut,
            unit.lineage,
            'structure-preview:' + context.operation_id + ':' + unit.unit_id,
          ),
        );
      }
      if (rooms.some((r) => !seen.has(r.unit_id)))
        throw new RPCFailure(14, 'STRUCTURE_BRANCH_SCOPE_UNKNOWN');
      cuts.sort((a, b) =>
        BigInt(a.room_id) < BigInt(b.room_id) ? -1 : BigInt(a.room_id) > BigInt(b.room_id) ? 1 : 0,
      );
      if (new Set(cuts.map((r) => r.room_id)).size !== cuts.length)
        throw new RPCFailure(14, 'STRUCTURE_ROOM_SCOPE_UNKNOWN');
      const cut = { ...storedCut(main.cut), rooms: cuts, dependency_digest: cutDigest(cuts) };
      const fixed = {
        binding,
        source,
        main_units: main.units.map(storeContent),
        drafts,
        observed_cut: cut,
        // 冻结HTTP引用沿用OperationID字符/长度规则；全局唯一原号即可定位固定预览，不加非法冒号前缀。
        preview_cut_ref: context.operation_id,
        expires_unix_ms: String(Date.now() + 600000),
      };
      try {
        fixed.migration = buildStructureMigration(fixed);
      } catch (error) {
        // 只有明确的正文前置条件才拒绝原号；缺失范围/谱系或未知异常不能被当成业务未提交。
        const conflicts = new Set([
          'DRAFT_STRUCTURE_CHANGED',
          'STRUCTURE_DRAFT_CONFLICT',
          'STRUCTURE_LEVEL2_MISSING',
          'STRUCTURE_TITLE_INVALID',
          'STRUCTURE_TITLE_CHANGED',
          'STRUCTURE_MODE_CHANGED',
          'STRUCTURE_RANGE_OVERLAP',
          'STRUCTURE_BLOCK_ID_COLLISION',
          'STRUCTURE_CONTENT_LIMIT',
        ]);
        if (conflicts.has(error.code)) throw new RPCFailure(9, error.code);
        throw error;
      }
      // 按实际base64 JSON限制整体传输，失败在固定事实提交之前，不能返回截断范围。
      if (Buffer.byteLength(JSON.stringify(fixed)) > 24 << 20)
        throw new RPCFailure(8, 'STRUCTURE_TRANSFER_LIMIT');
      fixed.input_digest = hash(canonical(fixed)).toString('base64');
      await artifacts.insertArtifact(c, 'STRUCTURE_PREVIEW_FIXED', context.operation_id, fixed);
      return this.replay(fixed, binding);
    });
  }

  replay(fixed, binding) {
    if (canonical(fixed.binding) !== canonical(binding))
      throw new RPCFailure(6, 'STRUCTURE_PREVIEW_ID_CONFLICT');
    return {
      state: 'FIXED',
      preview_cut_ref: fixed.preview_cut_ref,
      main_manifest_id: fixed.source.manifest_id,
      observed_cut: wireCut(fixed.observed_cut),
      expires_unix_ms: fixed.expires_unix_ms,
      input_digest: Buffer.from(fixed.input_digest, 'base64'),
    };
  }

  async fixed(input) {
    const binding = structurePreviewBinding(input);
    return this.transactions.run(async (c) => {
      await lockSources(c, input.context.sources);
      const [[row]] = await artifacts.artifact(
        c,
        'STRUCTURE_PREVIEW_FIXED',
        input.context.operation_id,
      );
      if (!row) throw new RPCFailure(5, 'STRUCTURE_PREVIEW_NOT_FOUND');
      const value = json(row.payload);
      this.replay(value, binding);
      // 仅供Doc系统取得同一个固定事实并保护资源，不接纳调用者提供的新正文或新切点。
      return value;
    });
  }
}
