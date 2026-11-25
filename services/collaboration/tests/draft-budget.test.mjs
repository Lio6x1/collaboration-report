import test from 'node:test';
import assert from 'node:assert/strict';
import * as draft from '../src/domain/draft_protocol.mjs';

test('draft prepare measures the encoded candidate and MAIN baseline before blocking the room', () => {
  const fixed = {
    preview_cut_ref: 'draft-preview:original',
    input_digest: 'digest',
    draft_cut: {},
    candidate: { body_json_utf8: 'a'.repeat(13 << 20) },
    main: { body_json_utf8: 'b'.repeat(12 << 20) },
  };
  // 两份正文单独都没有超过单份限制，但实际JSON响应已超过32MiB通道的保守预算。
  assert.throws(
    () => draft.draftPreparedResult(fixed),
    (error) => error.details === 'DRAFT_TRANSFER_LIMIT',
  );
  fixed.main.body_json_utf8 = 'small';
  assert.equal(draft.draftPreparedResult(fixed).state, 'PREPARED');
});
