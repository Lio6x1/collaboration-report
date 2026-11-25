import assert from 'node:assert/strict';
import { conflictBody, conflictParagraph } from './conflict-contexts.mjs';
import { post, operation } from './workspace.mjs';

export function resolutionInput(context, block, id = operation('conflict-resolution')) {
  return {
    operation_id: id,
    expected_attempt_id: context.attempt_id,
    expected_input_digest: context.input_digest,
    expected_round_id: context.round_id,
    resolution_content: conflictBody(conflictParagraph(block, '人工修复正文')),
    resolved_conflict_ids: context.conflicts.map((item) => item.id),
  };
}

export async function withdrawConflict(f, fixed) {
  const response = await post(
    `/v1/review-requests/${fixed.request_id}/withdrawals`,
    { operation_id: operation('repair-withdraw'), expected_round_id: fixed.round_id },
    f.user.headers,
  );
  assert.equal(response.status, 200, response.body);
}
