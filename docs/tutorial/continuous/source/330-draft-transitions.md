# services/collaboration/src/application/draft_transitions.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/330-draft-transitions.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`DraftTransitions`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { schema, toPM, canonical } from '../../../../packages/content/index.mjs';
import * as repository from '../repository/draft_transitions.mjs';
import * as rooms from '../repository/room_access.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';
import { storeContent, wireContent } from '../domain/capture_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { validateDraftInstall, draftPreparedResult } from '../domain/draft_protocol.mjs';
import { lockSources } from './admission_gates.mjs';
import * as main from '../repository/main_workspace.mjs';
import { requireMainOpen } from './admission_gates.mjs';
import { deriveDiscardConfirmation, sameReliableMainCut } from '../domain/discard_confirmation.mjs';
import { requireDocumentBodyPresent } from './account_document_cleanup.mjs';

export class DraftTransitions {
  constructor(transactions, queue, snapshots, peers, outputs) {
    Object.assign(this, { transactions, queue, snapshots, peers, outputs });
  }

  async prepare(input) {
    const operation = input.context?.operation_id;
    const discard = input.context?.target?.purpose === 'DISCARD_DRAFT';
    const controlsMain = discard || input.context?.target?.purpose === 'REVERSE';
    const fixed = discard
      ? await this.confirmation(input)
      : await this.transactions.run(async (c) => {
          const [[row]] = await repository.artifact(c, 'DRAFT_PREVIEW_FIXED', operation);
          if (!row) throw new RPCFailure(14, 'DRAFT_PREVIEW_UNKNOWN');
          const preview = json(row.payload);
          this.binding(preview, input);
          return preview;
        });
    // 原登录会话和当前资格由 Doc 在 SQL 锁外复核；未知结果不进入屏障准备。
    await this.peers.http('/infra/draft/context', fixed.binding);
    const preparedInput = discard ? { ...input, expected_input_digest: fixed.input_digest } : input;
    return this.queue.run(fixed.room_id, () =>
      this.transactions.run(async (c) => {
        await lockSources(c, input.context.sources);
        if (controlsMain) await main.lockDocument(c, input.context.target.document_id);
        const [[prior]] = await repository.artifact(c, 'DRAFT_TRANSITION_PREPARED', operation);
        if (prior) {
          const value = json(prior.payload);
          this.binding(value, preparedInput);
          return value.result;
        }
        if (!fixed.candidate) return { state: 'CONFLICTED', conflicts: fixed.conflicts };
        if (Number(fixed.expires_unix_ms) <= Date.now())
          return { state: 'CHANGED', code: 'PREVIEW_EXPIRED' };
        if (controlsMain) {
          await requireMainOpen(c, input.context.target.document_id);
          const [mainRows] = await repository.unitMainRoom(
            c,
            input.context.target.document_id,
            input.context.target.unit_id,
          );
          if (!sameReliableMainCut(fixed, mainRows))
            return { state: 'CHANGED', code: 'MAIN_CHANGED' };
        }
        const [[room]] = await rooms.lockRoom(c, fixed.room_id);
        if (
          !room ||
          room.state !== 'OPEN' ||
          room.epoch !== fixed.draft_cut.workspace_epoch ||
          room.last_durable_seq !== fixed.draft_cut.durable_seq
        )
          return { state: 'CHANGED', code: 'DRAFT_CHANGED' };
        const result = draftPreparedResult(fixed);
        if (controlsMain)
          await repository.setMainGate(
            c,
            input.context.target.document_id,
            operation,
            'PREPARED',
            fixed.binding,
          );
        await repository.blockRoom(c, room.id, room.epoch);
        await repository.insertArtifact(c, 'DRAFT_TRANSITION_PREPARED', operation, {
          ...fixed,
          result,
        });
        return result;
      }),
    );
  }

  async confirmation(input) {
    const context = {
      ...input.context,
      request_fingerprint: Buffer.from(
        input.context.request_fingerprint,
        typeof input.context.request_fingerprint === 'string' ? 'base64' : undefined,
      ).toString('base64'),
    };
    const binding = {
      context,
      expected_branch_id: context.target.branch_id,
      expected_structure_id: context.target.structure_id,
      expected_lineage_id: input.expected_lineage_id,
    };
    const authority = await this.peers.http('/infra/draft/context', binding);
    if (
      input.expected_attempt_id !== authority.preview_cut_ref ||
      input.expected_round_id !== authority.expected_round_id ||
      Buffer.from(
        input.expected_input_digest,
        typeof input.expected_input_digest === 'string' ? 'base64' : undefined,
      ).toString('base64') !== authority.preview_input_digest
    )
      throw new RPCFailure(6, 'DISCARD_CONFIRMATION_CONFLICT');
    return this.transactions.run(async (c) => {
      await lockSources(c, context.sources);
      const [[original]] = await repository.artifact(
        c,
        'DRAFT_PREVIEW_FIXED',
        authority.preview_cut_ref,
      );
      if (!original) throw new RPCFailure(14, 'DISCARD_PREVIEW_UNKNOWN');
      const fixed = deriveDiscardConfirmation(json(original.payload), binding, authority);
      const [[prior]] = await repository.artifact(
        c,
        'DRAFT_CONFIRMATION_FIXED',
        context.operation_id,
      );
      if (prior) {
        if (canonical(json(prior.payload)) !== canonical(fixed))
          throw new RPCFailure(6, 'DISCARD_CONFIRMATION_CONFLICT');
        return json(prior.payload);
      }
      draftPreparedResult(fixed);
      await repository.insertArtifact(c, 'DRAFT_CONFIRMATION_FIXED', context.operation_id, fixed);
      return fixed;
    });
  }

  binding(fixed, input) {
    const context = {
      ...input.context,
      request_fingerprint: Buffer.from(
        input.context?.request_fingerprint ?? '',
        typeof input.context?.request_fingerprint === 'string' ? 'base64' : undefined,
      ).toString('base64'),
    };
    if (
      canonical(context) !== canonical(fixed.binding.context) ||
      input.expected_attempt_id !== fixed.preview_cut_ref ||
      (context.target.purpose === 'CONFLICT_RESOLUTION' &&
        input.expected_round_id !== fixed.source.expected_round_id) ||
      Buffer.from(
        input.expected_input_digest ?? '',
        typeof input.expected_input_digest === 'string' ? 'base64' : undefined,
      ).toString('base64') !== fixed.input_digest
    )
      throw new RPCFailure(6, 'DRAFT_TRANSITION_ID_CONFLICT');
  }

  async apply(input) {
    const query = {
      operation_id: input.context?.operation_id,
      target: input.context?.target,
      request_fingerprint: input.context?.request_fingerprint,
    };
    if (
      !['PULL_MAIN', 'DISCARD_DRAFT', 'REVERSE', 'CONFLICT_RESOLUTION'].includes(
        query.target?.purpose,
      ) ||
      (!['DISCARD_DRAFT', 'CONFLICT_RESOLUTION'].includes(query.target.purpose) &&
        input.expected_round_id !== '0')
    )
      throw new RPCFailure(3, 'DRAFT_TRANSITION_INVALID');
    // 安装属于已提交系统事实，不能依赖原用户仍有权限；失败也只能解除自己的旧屏障。
    const authority = await this.peers.rpc('GetOperationResult', query);
    if (!['SUCCEEDED', 'REJECTED', 'CANCELLED'].includes(authority.state))
      throw new RPCFailure(14, 'DRAFT_RESULT_UNKNOWN');
    const source = await this.peers.http('/infra/draft/committed', {
      ...query,
      request_fingerprint: Buffer.from(query.request_fingerprint).toString('base64'),
    });
    if (source.result_digest !== Buffer.from(authority.result_digest).toString('base64'))
      throw new RPCFailure(14, 'DRAFT_RESULT_CHANGED');
    const discard = query.target.purpose === 'DISCARD_DRAFT';
    const controlsMain = discard || query.target.purpose === 'REVERSE';
    if (
      (discard || query.target.purpose === 'CONFLICT_RESOLUTION') &&
      source.expected_round_id !== input.expected_round_id
    )
      throw new RPCFailure(6, 'DISCARD_ROUND_CONFLICT');
    const roomID = source.old_room_id;
    await this.queue.run(roomID, () =>
      this.transactions.run(async (c) => {
        // 即使只安装支线也与生命周期清理共用入口，避免迟到系统请求重建私有房间。
        await main.lockDocument(c, query.target.document_id);
        await requireDocumentBodyPresent(c, query.target.document_id);
        const [[prepared]] = await repository.artifact(
          c,
          'DRAFT_TRANSITION_PREPARED',
          query.operation_id,
        );
        if (authority.state === 'SUCCEEDED') {
          if (!prepared) throw new RPCFailure(14, 'DRAFT_BARRIER_UNKNOWN');
          const fixed = json(prepared.payload);
          this.binding(fixed, input);
          validateDraftInstall(fixed, source);
          if (canonical(storeContent(input.resolution)) !== canonical(source.content))
            throw new RPCFailure(6, 'DRAFT_INSTALL_CONFLICT');
        }
        const [[finished]] = await repository.artifact(
          c,
          'DRAFT_TRANSITION_INSTALLED',
          query.operation_id,
        );
        if (finished) {
          if (
            canonical(json(finished.payload).query) !==
            canonical({
              ...query,
              request_fingerprint: Buffer.from(query.request_fingerprint).toString('base64'),
            })
          )
            throw new RPCFailure(6, 'DRAFT_TRANSITION_ID_CONFLICT');
          return;
        }
        const [[room]] = await rooms.lockRoom(c, roomID);
        if (authority.state !== 'SUCCEEDED') {
          if (prepared) {
            const fixed = json(prepared.payload);
            // 已拒绝的系统原号不能依赖失效来源重新取预览；全原上下文和权威终态仍须一致。
            const context = {
              ...input.context,
              request_fingerprint: Buffer.from(input.context.request_fingerprint).toString(
                'base64',
              ),
            };
            if (
              canonical(context) !== canonical(fixed.binding.context) ||
              input.expected_attempt_id !== fixed.preview_cut_ref
            )
              throw new RPCFailure(6, 'DRAFT_TRANSITION_ID_CONFLICT');
            if (!room || room.epoch !== query.target.workspace_epoch || room.state !== 'BLOCKED')
              throw new RPCFailure(14, 'DRAFT_BARRIER_UNKNOWN');
            await repository.openRoom(c, room.id, room.epoch);
          }
        } else {
          if (!prepared || !room || room.state !== 'BLOCKED')
            throw new RPCFailure(14, 'DRAFT_BARRIER_UNKNOWN');
          const fixed = json(prepared.payload);
          this.binding(fixed, input);
          if (
            room.epoch !== query.target.workspace_epoch ||
            room.last_durable_seq !== fixed.draft_cut.durable_seq ||
            source.new_epoch !== (BigInt(room.epoch) + 1n).toString() ||
            canonical(storeContent(input.resolution)) !== canonical(source.content)
          )
            throw new RPCFailure(6, 'DRAFT_INSTALL_CONFLICT');
          const [[next]] = await rooms.lockRoom(c, source.new_room_id);
          if (next) throw new RPCFailure(14, 'DRAFT_NEW_ROOM_UNKNOWN');
          await rooms.insertRoom(
            c,
            source.new_room_id,
            query.target.document_id,
            query.target.unit_id,
            'BRANCH',
            query.target.branch_id,
            source.new_epoch,
          );
          const doc = prosemirrorJSONToYDoc(
            schema,
            toPM(JSON.parse(wireContent(source.content).body_json_utf8)),
          );
          try {
            await this.snapshots.checkpoint(
              c,
              { id: source.new_room_id, epoch: source.new_epoch },
              doc,
              '0',
            );
          } finally {
            doc.destroy();
          }
          await captures.insertRoomLineage(c, source.new_room_id, source.new_epoch, source.lineage);
          await repository.retireRoom(c, room.id, room.epoch);
          await repository.closeGrants(c, room.id);
          await repository.stopDeliveries(c, room.id);
        }
        await repository.insertArtifact(c, 'DRAFT_TRANSITION_INSTALLED', query.operation_id, {
          query: {
            ...query,
            request_fingerprint: Buffer.from(query.request_fingerprint).toString('base64'),
          },
          result_digest: source.result_digest,
        });
        if (controlsMain) {
          const [[row]] = await repository.artifact(c, 'DRAFT_MAIN_GATE', query.target.document_id);
          if (row && json(row.payload).state === 'PREPARED') {
            if (json(row.payload).operation_id !== query.operation_id)
              throw new RPCFailure(14, 'DRAFT_MAIN_GATE_UNKNOWN');
            await repository.setMainGate(
              c,
              query.target.document_id,
              query.operation_id,
              authority.state === 'SUCCEEDED' ? 'INSTALLED' : 'ABORTED',
              json(row.payload).binding,
            );
          } else if (authority.state === 'SUCCEEDED')
            throw new RPCFailure(14, 'DRAFT_MAIN_GATE_UNKNOWN');
        }
      }),
    );
    if (authority.state === 'SUCCEEDED') await this.outputs.stopRooms([roomID]);
    // fixed_result 是原 Go 回执；实际输出关闭后的 current_state 是本次收尾确认。
    return {
      ...authority,
      current_state_json_utf8: Buffer.from(
        JSON.stringify({
          draft_sync_state: 'READY',
          new_room_id: source.new_room_id ?? null,
          new_epoch: source.new_epoch ?? null,
          lineage_id: source.content?.lineage_ref ?? null,
          content_digest: source.content?.body_hash ?? null,
        }),
      ),
    };
  }
}
````

UTF-8/LF全文SHA-256：`f2fe17a4c7cda3847160fb9880a531fc298748f3a07a99ffe179108050f36eb4`。[参考实现](../../../../services/collaboration/src/application/draft_transitions.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
