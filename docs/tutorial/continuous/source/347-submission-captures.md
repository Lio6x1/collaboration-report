# services/collaboration/src/application/submission_captures.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/347-submission-captures.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`SubmissionCaptures`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  canonical,
  fromPM,
  normalizeContent,
  resourceIds,
} from '../../../../packages/content/index.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';
import * as rooms from '../repository/room_access.mjs';
import * as flushes from '../repository/room_persistence.mjs';
import {
  captureBinding,
  sameBinding,
  matchCaptureQuery,
  captureProvenance,
  storeContent,
  wireContent,
} from '../domain/capture_protocol.mjs';
import { json, deny, validID, bad } from '../domain/shared_protocol.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { lockSources } from './admission_gates.mjs';

export class SubmissionCaptures {
  constructor(transactions, queue, snapshots, peers) {
    this.transactions = transactions;
    this.queue = queue;
    this.snapshots = snapshots;
    this.peers = peers;
  }

  async capture(input) {
    const binding = captureBinding(input),
      target = binding.context.target;
    const roomID = await this.transactions.run(async (connection) => {
      const [[prior]] = await captures.findCapture(connection, binding.context.operation_id);
      if (prior) {
        sameBinding(json(prior.candidate_json).binding, binding);
        return prior.room_id;
      }
      const [[receipt]] = await flushes.findFlushReceipt(connection, binding.flush_receipt_id);
      if (!receipt) throw deny('FLUSH_BINDING_INVALID');
      return receipt.room_id;
    });
    return this.queue.run(roomID, async () => {
      // 与UPDATE/FLUSH共用队列。首个短事务先固定cut，第二事务只能从原cut生成正文。
      await this.transactions.run(async (connection) => {
        await lockSources(connection, binding.context.sources);
        const [[prior]] = await captures.lockOperation(connection, binding.context.operation_id);
        if (prior) {
          sameBinding(json(prior.candidate_json).binding, binding);
          return prior.room_id;
        }
        const [[receipt]] = await flushes.findFlushReceipt(connection, binding.flush_receipt_id);
        if (
          !receipt ||
          receipt.state !== 'FIXED' ||
          receipt.actor_id !== binding.context.actor_id ||
          receipt.epoch !== target.workspace_epoch
        )
          throw deny('FLUSH_BINDING_INVALID');
        const [[room]] = await rooms.lockRoom(connection, receipt.room_id);
        if (
          !room ||
          room.state !== 'OPEN' ||
          room.workspace_kind !== 'BRANCH' ||
          room.epoch !== target.workspace_epoch ||
          room.document_id !== target.document_id ||
          room.unit_id !== target.unit_id ||
          room.workspace_id !== target.branch_id
        )
          throw deny('WORKSPACE_CHANGED');
        const covered = json(receipt.covered_cut);
        if (
          covered.room_id !== room.id ||
          covered.workspace_epoch !== room.epoch ||
          BigInt(covered.durable_seq) > BigInt(room.last_durable_seq)
        )
          throw deny('FLUSH_CUT_INVALID');
        const [[stored]] = await captures.lockRoomLineage(connection, room.id, room.epoch);
        if (!stored || json(stored.payload).lineage_id !== binding.lineage_ref)
          throw deny('LINEAGE_CHANGED');
        const cut = {
          room_id: room.id,
          workspace_epoch: room.epoch,
          durable_seq: String(room.last_durable_seq),
        };
        await captures.insertCut(connection, binding, room, cut, json(stored.payload));
        return room.id;
      });
      return this.complete(binding);
    });
  }

  async complete(binding) {
    return this.transactions.run(async (connection) => {
      await lockSources(connection, binding.context.sources);
      const [[row]] = await captures.lockOperation(connection, binding.context.operation_id);
      const candidate = json(row.candidate_json);
      sameBinding(candidate.binding, binding);
      if (candidate.cleaned) throw new RPCFailure(9, 'CAPTURE_CANDIDATE_CLEANED');
      if (['CAPTURED', 'BOUND'].includes(row.state))
        return { state: 'FIXED', content: wireContent(candidate.content) };
      if (row.state !== 'CUT_FIXED') return { state: 'REJECTED', business_code: 'CAPTURE_CLOSED' };
      const [[room]] = await rooms.lockRoom(connection, row.room_id);
      if (!room || room.epoch !== row.epoch || room.state !== 'OPEN')
        throw deny('WORKSPACE_CHANGED');
      // 切点与来源在首次事务一并固定；同epoch后来接纳主线，也不能改变这个候选的base/provenance。
      const lineage = candidate.lineage,
        cut = json(row.cut_ref);
      if (!lineage) throw new RPCFailure(14, 'CAPTURE_LINEAGE_UNKNOWN');
      const doc = await this.snapshots.restore(connection, room, cut.durable_seq);
      try {
        const body = normalizeContent(fromPM(yDocToProsemirrorJSON(doc))),
          bytes = Buffer.from(canonical(body)),
          digest = hash(bytes),
          resources = resourceIds(body);
        const content = {
          ...binding.context.target,
          capture_id: row.operation_id,
          operation_id: row.operation_id,
          cut_json_utf8: Buffer.from(canonical(cut)),
          lineage_ref: row.lineage_ref,
          body_json_utf8: bytes,
          schema_version: 'content-v1',
          canonicalization_version: 'canonical-v1',
          hash_algorithm: 'SHA-256',
          body_hash: digest,
          byte_size: String(bytes.length),
          resource_ids: resources,
          provenance_json_utf8: Buffer.from(
            canonical(captureProvenance(body, lineage, row.operation_id)),
          ),
          provenance_schema: 'provenance-v1',
        };
        await captures.saveCandidate(
          connection,
          row.operation_id,
          { binding, content: storeContent(content) },
          bytes,
          digest,
          resources,
        );
        return { state: 'FIXED', content };
      } finally {
        doc.destroy();
      }
    });
  }

  async get(query) {
    if (
      !validID(query.operation_id) ||
      Buffer.from(query.request_fingerprint ?? []).length !== 32 ||
      !query.target
    )
      throw bad('CAPTURE_QUERY_INVALID');
    const result = await this.transactions.run(async (connection) => {
      const [[row]] = await captures.findCapture(connection, query.operation_id);
      if (!row) return null;
      const binding = json(row.candidate_json).binding;
      matchCaptureQuery(row, binding, query);
      await lockSources(connection, binding.context.sources);
      return { row, binding };
    });
    if (!result) return { state: 'PROCESSING' };
    return this.queue.run(result.row.room_id, () => this.complete(result.binding));
  }

  async finalize(input) {
    const supplied = input.authoritative_result;
    if (!input.context || input.capture_id !== input.context.operation_id || !supplied)
      throw bad('FINALIZE_CAPTURE_INVALID');
    // 系统收尾查真实Go终态；不能把调用参数或网络超时当作可绑定/可删除许可。
    const result = await this.peers.rpc('GetOperationResult', {
      operation_id: input.context.operation_id,
      target: input.context.target,
      request_fingerprint: input.context.request_fingerprint,
    });
    if (result.state !== 'SUCCEEDED') throw new RPCFailure(14, 'CAPTURE_RESULT_UNKNOWN');
    if (
      supplied.state !== result.state ||
      supplied.operation_id !== result.operation_id ||
      Buffer.from(supplied.result_digest).toString('hex') !==
        Buffer.from(result.result_digest).toString('hex') ||
      Buffer.from(supplied.fixed_result_json_utf8).toString('base64') !==
        Buffer.from(result.fixed_result_json_utf8).toString('base64')
    )
      throw new RPCFailure(6, 'CAPTURE_RESULT_CONFLICT');
    await this.transactions.run(async (connection) => {
      const [[row]] = await captures.lockOperation(connection, input.capture_id);
      if (!row) throw deny('CAPTURE_NOT_FOUND');
      matchCaptureQuery(row, json(row.candidate_json).binding, { ...input.context });
      const reference =
        result.operation_id + ':' + Buffer.from(result.result_digest).toString('hex');
      if (row.go_result_ref && row.go_result_ref !== reference)
        throw new RPCFailure(6, 'CAPTURE_RESULT_CONFLICT');
      if (!['CAPTURED', 'BOUND'].includes(row.state)) throw deny('CAPTURE_NOT_FIXED');
      await captures.bindResult(connection, row.operation_id, reference);
    });
    return result;
  }

  async committedCut(query) {
    if (!validID(query.operation_id) || !query.target || query.target.purpose !== 'SUBMISSION')
      throw bad('CAPTURE_QUERY_INVALID');
    const fingerprint = Buffer.from(query.request_fingerprint ?? '', 'base64');
    if (fingerprint.length !== 32) throw bad('CAPTURE_QUERY_INVALID');
    const rpcQuery = { ...query, request_fingerprint: fingerprint };
    // 这是已提交送审的系统事实读取：重查Go固定终态，而不是沿用旧用户的准入许可。
    // 新审核者仍由Go合并用例检查当前权限；旧来源关闭不能抹掉早已成功的切点。
    const authority = await this.peers.rpc('GetOperationResult', rpcQuery);
    if (authority.state !== 'SUCCEEDED' || authority.operation_id !== query.operation_id)
      throw new RPCFailure(14, 'CAPTURE_RESULT_UNKNOWN');
    const fixed = JSON.parse(Buffer.from(authority.fixed_result_json_utf8).toString('utf8'));
    if (!/^[1-9][0-9]*$/.test(fixed.result?.content_version_id ?? ''))
      throw new RPCFailure(14, 'CAPTURE_RESULT_UNKNOWN');
    return this.transactions.run(async (connection) => {
      const [[row]] = await captures.findCapture(connection, query.operation_id);
      if (!row || !['CAPTURED', 'BOUND'].includes(row.state))
        throw new RPCFailure(14, 'CAPTURE_NOT_FIXED');
      const candidate = json(row.candidate_json);
      matchCaptureQuery(row, candidate.binding, rpcQuery);
      if (
        !candidate.content ||
        candidate.content.capture_id !== query.operation_id ||
        candidate.content.body_hash !== row.body_hash.toString('base64')
      )
        throw new RPCFailure(14, 'CAPTURE_FIXED_BINDING_UNKNOWN');
      return {
        operation_id: query.operation_id,
        content_version_id: fixed.result.content_version_id,
        cut_json_utf8: candidate.content.cut_json_utf8,
        body_hash: candidate.content.body_hash,
      };
    });
  }
}
````

UTF-8/LF全文SHA-256：`791d63f8480df564fb316f0effddedd1ecebe091e8fd3356e36204baed181a9b`。[参考实现](../../../../services/collaboration/src/application/submission_captures.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
