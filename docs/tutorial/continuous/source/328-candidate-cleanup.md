# services/collaboration/src/application/candidate_cleanup.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/328-candidate-cleanup.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`cleanupBinding`、`CandidateCleanup`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { bad, validID, json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import * as repository from '../repository/candidate_artifacts.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';
import * as merges from '../repository/candidate_artifacts.mjs';

const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;
export function cleanupBinding(input) {
  const request = input?.binding,
    permit = input?.permit;
  if (
    !validID(request?.operation_id) ||
    !positive(request?.generation) ||
    Buffer.from(request?.candidate_digest ?? []).length !== 32 ||
    !permit ||
    permit.operation_id !== request.operation_id ||
    permit.generation !== request.generation ||
    permit.state !== 'CLEANABLE' ||
    Buffer.from(permit.permit_digest ?? []).length !== 32
  )
    throw bad('CANDIDATE_CLEANUP_INVALID');
  const parts = request.candidate_ref?.split(':') ?? [];
  if (
    !(parts.length === 2 && parts[0] === 'capture' && parts[1] === request.operation_id) &&
    !(
      parts.length === 3 &&
      parts[0] === 'merge' &&
      parts[1] === request.operation_id &&
      parts[2] === request.generation
    )
  )
    throw bad('CANDIDATE_REFERENCE_INVALID');
  return {
    operation_id: request.operation_id,
    generation: request.generation,
    candidate_ref: request.candidate_ref,
    candidate_digest: Buffer.from(request.candidate_digest).toString('base64'),
    permit_digest: Buffer.from(permit.permit_digest).toString('base64'),
  };
}

function replay(row, binding) {
  if (!row) return null;
  const saved = json(row.payload);
  if (canonical(saved.binding) !== canonical(binding))
    throw new RPCFailure(6, 'CANDIDATE_BINDING_CHANGED');
  const result = saved.result;
  if (
    result?.operation_id !== binding.operation_id ||
    result.generation !== binding.generation ||
    result.state !== 'SUCCEEDED' ||
    !result.fixed_result_json_utf8 ||
    !result.result_digest ||
    !hash(Buffer.from(result.fixed_result_json_utf8, 'base64')).equals(
      Buffer.from(result.result_digest, 'base64'),
    )
  )
    throw new RPCFailure(14, 'CANDIDATE_CLEANUP_RECEIPT_UNKNOWN');
  return {
    ...result,
    fixed_result_json_utf8: Buffer.from(result.fixed_result_json_utf8, 'base64'),
    result_digest: Buffer.from(result.result_digest, 'base64'),
  };
}

export class CandidateCleanup {
  constructor(transactions, peers) {
    this.transactions = transactions;
    this.peers = peers;
  }

  async clean(input) {
    const binding = cleanupBinding(input);
    const prior = await this.transactions.run(async (connection) =>
      replay(await repository.findCleanup(connection, binding.candidate_ref), binding),
    );
    if (prior) return prior;
    // 调用参数不授予删除权。锁外重查Doc持久原许可，超时/RETAIN/UNKNOWN均保留副本。
    const permit = await this.peers.rpc('AuthorizeCandidateCleanup', input.binding);
    if (permit.state !== 'CLEANABLE') throw new RPCFailure(14, 'CANDIDATE_CLEANUP_NOT_PROVEN');
    if (
      permit.operation_id !== binding.operation_id ||
      permit.generation !== binding.generation ||
      Buffer.from(permit.permit_digest ?? []).toString('base64') !== binding.permit_digest
    )
      throw new RPCFailure(6, 'CANDIDATE_PERMIT_CHANGED');
    return this.transactions.run(async (connection) => {
      const parts = binding.candidate_ref.split(':');
      // 与Capture/Compute相同的原号gate串行，CPU计算期间不持SQL锁；
      // 清理墓碑和删除同事务，迟到的第二次计算提交必须再次读墓碑。
      const row =
        parts[0] === 'capture'
          ? (await captures.lockOperation(connection, binding.operation_id))[0][0]
          : await merges.lockAttempt(connection, binding.operation_id, parts[2]);
      const completed = replay(
        await repository.findCleanup(connection, binding.candidate_ref),
        binding,
      );
      if (completed) return completed;
      if (parts[0] === 'capture') await this.cleanCapture(connection, row, binding);
      else await this.cleanMerge(connection, row, binding);
      const fixed = Buffer.from(
        canonical({
          operation_id: binding.operation_id,
          state: 'SUCCEEDED',
          cleanup_state: 'CLEANED',
          candidate_ref: binding.candidate_ref,
          generation: binding.generation,
        }),
      );
      const stored = {
        state: 'SUCCEEDED',
        operation_id: binding.operation_id,
        generation: binding.generation,
        terminal_revision: binding.generation,
        fixed_result_json_utf8: fixed.toString('base64'),
        result_digest: hash(fixed).toString('base64'),
      };
      await repository.saveCleanup(connection, binding, stored);
      return replay({ payload: { binding, result: stored } }, binding);
    });
  }

  async cleanCapture(connection, row, binding) {
    if (
      !row ||
      row.state !== 'BOUND' ||
      String(row.generation) !== binding.generation ||
      Buffer.from(row.body_hash ?? []).toString('base64') !== binding.candidate_digest ||
      !row.go_result_ref
    )
      throw new RPCFailure(14, 'CAPTURE_CLEANUP_NOT_PROVEN');
    const candidate = json(row.candidate_json);
    if (
      candidate.binding?.guard_generation !== binding.generation ||
      candidate.content?.body_hash !== binding.candidate_digest ||
      !hash(Buffer.from(candidate.content.body_json_utf8 ?? '', 'base64')).equals(
        Buffer.from(row.body_hash),
      )
    )
      throw new RPCFailure(6, 'CAPTURE_CANDIDATE_CHANGED');
    // 后续合并仍使用原送审cut/hash/lineage元数据；只清冗余正文及完整旧lineage副本。
    // 保留BOUND和go_result_ref，不能把已提交送审变成新的失败业务操作。
    delete candidate.content.body_json_utf8;
    delete candidate.lineage;
    candidate.cleaned = true;
    await repository.cleanCaptureBody(connection, binding.operation_id, candidate);
  }

  async cleanMerge(connection, row, binding) {
    const context = row.context && json(row.context.payload),
      result = row.result && json(row.result.payload);
    if (
      !context ||
      !result?.candidate ||
      context.attempt_id !== binding.generation ||
      context.context?.operation_id !== binding.operation_id ||
      result.candidate.body_hash !== binding.candidate_digest ||
      !hash(Buffer.from(result.candidate.body_json_utf8 ?? '', 'base64')).equals(
        Buffer.from(binding.candidate_digest, 'base64'),
      )
    )
      throw new RPCFailure(14, 'MERGE_CLEANUP_NOT_PROVEN');
    await repository.cleanMergeBody(connection, row.business);
  }
}
````

UTF-8/LF全文SHA-256：`ab0a12f5f7de68ff6eadf01d8e33c3aaf01c6c6a5e11c14d6fa3b6372cdcce9f`。[参考实现](../../../../services/collaboration/src/application/candidate_cleanup.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
