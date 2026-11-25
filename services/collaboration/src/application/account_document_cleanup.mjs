import { createHash } from 'node:crypto';
import {
  cleanupRequest,
  cleanupAuthority,
  cleanupReply,
} from '../domain/account_document_cleanup.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { binding, ack, assertSameBinding } from '../domain/fence_protocol.mjs';
import * as repository from '../repository/account_document_cleanup.mjs';
import * as barriers from '../repository/main_workspace.mjs';
import { lockedClosure } from '../repository/fences.mjs';
import { query } from '../repository/sql.mjs';

export class AccountDocumentCleanup {
  constructor(transactions, peers) {
    Object.assign(this, { transactions, peers });
  }

  replay(row, request) {
    if (!row) return null;
    if (row.state !== 'SUCCEEDED' || row.operation_id !== request.operation_id)
      throw new RPCFailure(6, 'ACCOUNT_DOCUMENT_CLEANUP_CONFLICT');
    return cleanupReply(cleanupAuthority(json(row.payload), request));
  }

  async clean(input) {
    const started = performance.now();
    const stage = (name) => {
      if (process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === '1')
        console.error(
          JSON.stringify({
            event: 'account_document_cleanup',
            stage: name,
            elapsed_ms: Math.round(performance.now() - started),
          }),
        );
    };
    const request = cleanupRequest(input);
    // 提交回复丢失后先读取原本地回执；后续Doc留存代次变化不改变已完成原清理。
    const prior = await this.transactions.run(async (c) =>
      this.replay(
        await repository.cleanupReceipt(c, request.document_id, request.operation_id),
        request,
      ),
    );
    if (prior) return prior;
    stage('ORIGINAL_RECEIPT_CHECKED');
    const authority = cleanupAuthority(
      await this.peers.http('/infra/lifecycle/document-close-context', request),
      request,
    );
    stage('DOC_AUTHORITY_CONFIRMED');
    return this.transactions.run(async (c) => {
      // 与捕获/UPDATE相同：来源→MAIN入口→房间。系统安装也取得MAIN入口。
      const [[source]] = await repository.lockCleanupSource(c, authority.scope_key);
      await barriers.lockDocument(c, request.document_id);
      const repeated = this.replay(
        await repository.cleanupReceipt(c, request.document_id, request.operation_id),
        request,
      );
      if (repeated) return repeated;
      await this.requireDrain(c, authority, source);
      stage('LOCAL_DRAIN_CONFIRMED');
      const [[barrier]] = await barriers.lockBarrier(c, request.document_id);
      if (barrier && !['INSTALLED', 'ABORTED'].includes(barrier.state))
        throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_INSTALL_PENDING');
      const [fixed] = await repository.documentArtifacts(c, request.document_id);
      stage('ARTIFACTS_READ');
      for (const row of fixed) {
        const value = json(row.payload);
        if (!['SUCCEEDED', 'REJECTED'].includes(row.state))
          throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_ARTIFACT_UNKNOWN');
        if (
          ['DRAFT_MAIN_GATE', 'STRUCTURE_BRANCH_GATE'].includes(row.kind) &&
          !['INSTALLED', 'ABORTED'].includes(value.state)
        )
          throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_INSTALL_PENDING');
        if (row.kind === 'DRAFT_TRANSITION_PREPARED') {
          const installed = await repository.draftInstallation(c, row.business_key);
          if (
            !installed ||
            installed.state !== 'SUCCEEDED' ||
            installed.operation_id !== row.operation_id ||
            json(installed.payload).query?.target?.document_id !== request.document_id
          )
            throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_DRAFT_PENDING');
        }
      }
      const [rooms] = await repository.lockRooms(c, request.document_id);
      stage('ROOMS_LOCKED');
      for (const room of rooms) {
        const [pending] = await repository.pendingDeliveries(c, room.id);
        if (pending.length) throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_OUTPUT_PENDING');
        const [captures] = await repository.roomCaptures(c, room.id);
        if (captures.some((capture) => capture.state === 'UNKNOWN'))
          throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_CAPTURE_UNKNOWN');
      }
      // Doc固定计划证明原写入/安装已终结；CUT_FIXED不再猜测其用户操作结果。
      // 所有正文清除和成功墓碑在同一TX，网络/COMMIT未知只重送原号。
      for (const room of rooms) await repository.clearRoomBody(c, room.id);
      for (const row of fixed) {
        if (row.kind === 'RESOURCE_CONFIRM') continue; // 保留资源确认审计，Doc另行释放保护。
        if (row.kind === 'MERGE_ATTEMPT_CONTEXT')
          await repository.removeMergeResult(c, row.business_key);
        await repository.removeArtifact(c, row.id);
      }
      await repository.clearBarrierBody(c, request.document_id);
      await repository.recordCleanup(c, authority);
      return cleanupReply(authority);
    });
  }

  async requireDrain(connection, authority, source) {
    if (
      !source ||
      source.state !== 'BLOCKED' ||
      String(source.max_epoch) !== authority.closed_epoch ||
      source.last_revocation_id !== authority.revocation_id
    )
      throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_SOURCE_CHANGED');
    const request = {
      revocation_id: authority.revocation_id,
      closed_scopes: [{ scope_key: authority.scope_key, epoch: authority.closed_epoch }],
      generation: '1',
      target_digest: createHash('sha256')
        .update('AUTHORIZATION_CHANGE\n' + authority.scope_key + '\n' + authority.closed_epoch)
        .digest(),
    };
    const closure = await lockedClosure(connection, authority.revocation_id);
    if (!closure || closure.state !== 'SUCCEEDED')
      throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_DRAIN_PENDING');
    assertSameBinding(closure, request.closed_scopes[0], binding(request));
    const [[proof]] = await query(
      connection,
      "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant='node-runtime' AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'",
      [authority.revocation_id, authority.scope_key, authority.closed_epoch],
    );
    if (!proof || !Buffer.from(proof.result_digest).equals(ack(request).result_digest))
      throw new RPCFailure(14, 'ACCOUNT_DOCUMENT_ACK_CHANGED');
  }
}

// 已完成清理的文档不能被迟到系统安装复活。调用方已经持有同一MAIN入口锁。
export async function requireDocumentBodyPresent(connection, document) {
  if (await repository.cleanupReceipt(connection, document))
    throw new RPCFailure(9, 'ACCOUNT_DOCUMENT_CLEANED');
}
