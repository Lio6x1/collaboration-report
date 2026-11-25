import { canonical, resourceIds } from '../../../../packages/content/index.mjs';
import * as repository from '../repository/candidate_artifacts.mjs';
import { validateMergeInput } from '../domain/merge_protocol.mjs';
import { captureProvenance, storeContent, wireContent } from '../domain/capture_protocol.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { lockSources } from './admission_gates.mjs';

export class MergeCandidates {
  constructor(transactions, executor) {
    this.transactions = transactions;
    this.executor = executor;
  }

  async compute(input) {
    const fixed = validateMergeInput(input),
      key = input.context;
    const prior = await this.transactions.run(async (connection) => {
      await lockSources(connection, key.sources);
      const row = await repository.lockAttempt(connection, key.operation_id, input.attempt_id);
      if (row.cleaned) throw new RPCFailure(9, 'MERGE_CANDIDATE_CLEANED');
      if (row.context) {
        if (canonical(json(row.context.payload)) !== canonical(fixed.binding))
          throw new RPCFailure(6, 'MERGE_ATTEMPT_CONFLICT');
      } else await repository.saveContext(connection, row.business, fixed.binding);
      return row.result ? json(row.result.payload) : null;
    });
    if (prior) return this.reply(prior);
    // FIXED上下文登记已完成，任务state=SUCCEEDED表达登记事实。只有RESULT表示计算完成；
    // 此处崩溃由Go的持久MERGE_RECOVER重送相同哈希的三份输入，不需要Node保存三份大正文。
    const result = await this.executor.compute(
      Object.fromEntries(Object.entries(fixed.content).map(([key, value]) => [key, value.body])),
    );
    const stored = {
      state: 'FIXED',
      input_digest: Buffer.from(input.input_digest).toString('base64'),
      conflicts: result.conflicts,
    };
    if (result.body) {
      const bytes = Buffer.from(canonical(result.body)),
        provenance = fixed.content.submitted.provenance;
      const content = {
        ...input.submitted,
        capture_id: key.operation_id + '-merge-' + input.attempt_id,
        operation_id: key.operation_id,
        body_json_utf8: bytes,
        body_hash: hash(bytes),
        byte_size: String(bytes.length),
        resource_ids: resourceIds(result.body),
        provenance_json_utf8: Buffer.from(
          canonical(
            captureProvenance(
              result.body,
              {
                body: fixed.content.base.body,
                lineage_id: input.submitted.lineage_ref,
                base_manifest_id: provenance.base_manifest_id,
                base_version_id: provenance.base_content_version_id,
                provenance,
              },
              key.operation_id,
            ),
          ),
        ),
      };
      stored.candidate = storeContent(content);
    }
    const committed = await this.transactions.run(async (connection) => {
      await lockSources(connection, key.sources);
      const row = await repository.lockAttempt(connection, key.operation_id, input.attempt_id);
      if (row.cleaned) throw new RPCFailure(9, 'MERGE_CANDIDATE_CLEANED');
      if (canonical(json(row.context.payload)) !== canonical(fixed.binding))
        throw new RPCFailure(6, 'MERGE_ATTEMPT_CONFLICT');
      if (row.result) return json(row.result.payload);
      await repository.saveResult(connection, row.business, stored);
      return stored;
    });
    return this.reply(committed);
  }

  reply(stored) {
    return {
      state: stored.state,
      input_digest: Buffer.from(stored.input_digest, 'base64'),
      conflicts: stored.conflicts,
      ...(stored.candidate ? { candidate: wireContent(stored.candidate) } : {}),
    };
  }
}
