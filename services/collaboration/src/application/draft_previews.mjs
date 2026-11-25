import { prosemirrorJSONToYDoc, yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  schema,
  toPM,
  fromPM,
  normalizeContent,
  canonical,
} from '../../../../packages/content/index.mjs';
import * as repository from '../repository/draft_transitions.mjs';
import * as rooms from '../repository/room_access.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';
import * as main from '../repository/main_workspace.mjs';
import { wireContent } from '../domain/capture_protocol.mjs';
import {
  draftPreviewBinding,
  fixedDraft,
  computeDraftPreview,
  draftBody,
  draftLineage,
  draftPreviewCutDigest,
} from '../domain/draft_protocol.mjs';
import { wireCut } from '../domain/main_protocol.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { lockSources } from './admission_gates.mjs';
import { requireMainOpen } from './admission_gates.mjs';
import { computeReverse } from '../domain/reverse_draft.mjs';
import { computeResolution } from '../domain/resolution_draft.mjs';
import { matchesMainWorkspace } from '../domain/main_workspace.mjs';

export class DraftPreviews {
  constructor(transactions, queue, snapshots, peers) {
    Object.assign(this, { transactions, queue, snapshots, peers });
  }

  async capture(input) {
    const binding = draftPreviewBinding(input),
      context = input.context;
    const prior = await this.transactions.run(async (c) => {
      await lockSources(c, context.sources);
      const [[row]] = await repository.artifact(c, 'DRAFT_PREVIEW_FIXED', context.operation_id);
      return row && this.replay(json(row.payload), binding);
    });
    if (prior) return prior;
    // Doc 只提供已登记原号的单元范围；mTLS 身份本身不是读取其他单元正文的许可。
    const source = await this.peers.http('/infra/draft/context', binding);
    if (context.target.purpose !== 'PULL_MAIN' && source.purpose !== context.target.purpose)
      throw new RPCFailure(14, 'DRAFT_PREVIEW_PURPOSE_UNKNOWN');
    return this.queue.run(source.room_id, () =>
      this.transactions.run(async (c) => {
        await lockSources(c, context.sources);
        await requireMainOpen(c, context.target.document_id);
        const [[old]] = await repository.artifact(c, 'DRAFT_PREVIEW_FIXED', context.operation_id);
        if (old) return this.replay(json(old.payload), binding);
        let [[room]] = await rooms.lockRoom(c, source.room_id);
        if (!room) {
          await rooms.insertRoom(
            c,
            source.room_id,
            context.target.document_id,
            context.target.unit_id,
            'BRANCH',
            context.target.branch_id,
            context.target.workspace_epoch,
          );
          room = {
            id: source.room_id,
            document_id: context.target.document_id,
            unit_id: context.target.unit_id,
            workspace_kind: 'BRANCH',
            workspace_id: context.target.branch_id,
            epoch: context.target.workspace_epoch,
            last_durable_seq: '0',
            state: 'OPEN',
          };
          const seed = prosemirrorJSONToYDoc(schema, toPM(source.lineage.body));
          try {
            await this.snapshots.checkpoint(c, room, seed, '0');
          } finally {
            seed.destroy();
          }
          await captures.insertRoomLineage(c, room.id, room.epoch, source.lineage);
        }
        if (
          room.state !== 'OPEN' ||
          room.workspace_kind !== 'BRANCH' ||
          room.workspace_id !== context.target.branch_id ||
          room.epoch !== context.target.workspace_epoch ||
          room.unit_id !== context.target.unit_id ||
          room.document_id !== context.target.document_id
        )
          throw new RPCFailure(9, 'WORKSPACE_CHANGED');
        const [[lineageRow]] = await captures.lockRoomLineage(c, room.id, room.epoch);
        if (
          !lineageRow ||
          canonical(draftLineage(json(lineageRow.payload))) !== canonical(source.lineage)
        )
          throw new RPCFailure(9, 'LINEAGE_CHANGED');
        const cut = {
          room_id: room.id,
          workspace_epoch: room.epoch,
          durable_seq: room.last_durable_seq,
        };
        const draft = await this.body(c, room);
        const [mainRooms] = await repository.unitMainRoom(c, room.document_id, room.unit_id);
        if (mainRooms.length > 1) throw new RPCFailure(14, 'MAIN_CONTEXT_UNKNOWN');
        const mainRoom = mainRooms[0];
        let mainBody = normalizeContent(JSON.parse(wireContent(source.main).body_json_utf8), {
          workspaceKind: 'MAIN',
        });
        let mainCut = [];
        if (mainRoom) {
          const [[mainContext]] = await main.mainContext(c, mainRoom.id, mainRoom.epoch);
          const current = mainContext && json(mainContext.payload);
          if (
            mainRoom.state !== 'OPEN' ||
            !matchesMainWorkspace(current, mainRoom, {
              ...source,
              manifest_id: source.main_manifest_id,
            })
          )
            throw new RPCFailure(14, 'MAIN_CONTEXT_UNKNOWN');
          mainBody = await this.body(c, mainRoom, 'MAIN');
          mainCut = [
            {
              room_id: mainRoom.id,
              workspace_epoch: mainRoom.epoch,
              durable_seq: mainRoom.last_durable_seq,
            },
          ];
        }
        const preview =
          context.target.purpose !== 'PULL_MAIN'
            ? context.operation_id
            : 'draft-preview:' + context.operation_id;
        const merged =
          context.target.purpose === 'REVERSE'
            ? computeReverse(
                JSON.parse(wireContent(source.reverse_before).body_json_utf8),
                JSON.parse(wireContent(source.reverse_after).body_json_utf8),
                draft,
                mainBody,
                source.lineage.body,
              )
            : context.target.purpose === 'CONFLICT_RESOLUTION'
              ? computeResolution(
                  JSON.parse(wireContent(source.resolution_submitted).body_json_utf8),
                  source.resolution_content,
                  draft,
                )
              : computeDraftPreview(context.target.purpose, source.lineage.body, draft, mainBody);
        const formalMain = wireContent(source.main);
        const mainLineage = {
          lineage_id: formalMain.lineage_ref,
          // 可靠尾部以正式旧版本为比较基线；尚未物化的新增块只能来自本次采样。
          body: draftBody(JSON.parse(formalMain.body_json_utf8)),
          provenance: JSON.parse(formalMain.provenance_json_utf8),
          base_manifest_id: source.main_manifest_id,
          base_version_id: source.main_version_id,
        };
        const fixed = {
          binding,
          room_id: room.id,
          draft_cut: cut,
          main_cut: mainCut,
          source,
          draft: fixedDraft(draft, context, cut, source.lineage, preview),
          main: fixedDraft(
            draftBody(mainBody),
            {
              ...context,
              target: {
                ...context.target,
                branch_id: '0',
                workspace_epoch: source.control_revision,
              },
            },
            mainCut[0] ?? {
              room_id: '0',
              workspace_epoch: source.workspace_epoch,
              durable_seq: '0',
            },
            mainLineage,
            preview + ':main',
          ),
          candidate:
            merged.body &&
            fixedDraft(
              merged.body,
              context,
              cut,
              source.lineage,
              context.target.purpose === 'REVERSE' ? preview : preview + ':candidate',
            ),
          conflicts: merged.conflicts,
          preview_cut_ref: preview,
          expires_unix_ms: String(Date.now() + 600000),
        };
        if (
          context.target.purpose !== 'PULL_MAIN' &&
          Buffer.byteLength(JSON.stringify(fixed)) > 24 << 20
        )
          throw new RPCFailure(8, 'DISCARD_TRANSFER_LIMIT');
        fixed.input_digest = hash(canonical(fixed)).toString('base64');
        await repository.insertArtifact(c, 'DRAFT_PREVIEW_FIXED', context.operation_id, fixed);
        return this.replay(fixed, binding);
      }),
    );
  }

  async fixed(input) {
    const binding = draftPreviewBinding(input);
    return this.transactions.run(async (connection) => {
      await lockSources(connection, input.context.sources);
      const [[row]] = await repository.artifact(
        connection,
        'DRAFT_PREVIEW_FIXED',
        input.context.operation_id,
      );
      if (!row) throw new RPCFailure(5, 'DRAFT_PREVIEW_NOT_FOUND');
      const fixed = json(row.payload);
      this.replay(fixed, binding);
      // 系统取得同一个固定事实；不会按当前房间重新生成正文、cut或digest。
      if (Buffer.byteLength(JSON.stringify(fixed)) > 24 << 20)
        throw new RPCFailure(8, 'DISCARD_TRANSFER_LIMIT');
      return fixed;
    });
  }

  replay(fixed, binding) {
    if (canonical(fixed.binding) !== canonical(binding))
      throw new RPCFailure(6, 'DRAFT_PREVIEW_ID_CONFLICT');
    const source = fixed.source,
      cuts = [fixed.draft_cut, ...fixed.main_cut];
    return {
      state: 'FIXED',
      preview_cut_ref: fixed.preview_cut_ref,
      draft: wireContent(fixed.draft),
      main_manifest_id: source.main_manifest_id,
      observed_cut: wireCut({
        document_id: binding.context.target.document_id,
        structure_id: source.structure_id,
        style_id: source.style_id,
        control_revision: source.control_revision,
        rooms: cuts,
        dependency_digest: draftPreviewCutDigest(binding.context.target.purpose, cuts),
      }),
      expires_unix_ms: fixed.expires_unix_ms,
      input_digest: Buffer.from(fixed.input_digest, 'base64'),
    };
  }

  async body(c, room, kind = 'BRANCH') {
    const doc = await this.snapshots.restore(c, room);
    try {
      return normalizeContent(fromPM(yDocToProsemirrorJSON(doc)), { workspaceKind: kind });
    } finally {
      doc.destroy();
    }
  }
}
