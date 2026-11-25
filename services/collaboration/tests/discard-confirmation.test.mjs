import test from 'node:test';
import assert from 'node:assert/strict';
import { canonical } from '../../../packages/content/index.mjs';
import { deriveDiscardConfirmation } from '../src/domain/discard_confirmation.mjs';
import { createHash } from 'node:crypto';

const digest = (value) => createHash('sha256').update(canonical(value)).digest('base64');
const fixture = () => {
  const context = {
    operation_id: 'original-discard-preview',
    actor_id: '1',
    sources: [],
    identity_admission_ref: 'old-identity',
    document_admission_ref: 'old-document',
    request_fingerprint: Buffer.alloc(32, 1).toString('base64'),
    target: {
      document_id: '2',
      unit_id: '3',
      branch_id: '4',
      structure_id: '5',
      workspace_epoch: '6',
      purpose: 'DISCARD_PREVIEW',
    },
  };
  const provenance = {
    source_operation_id: context.operation_id,
    block_sources: [{ origin_capture_ref: context.operation_id }],
    accepted_captures: [{ capture_ref: 'older-accepted' }],
  };
  const content = {
    operation_id: context.operation_id,
    capture_id: context.operation_id,
    body_json_utf8: Buffer.from('{}').toString('base64'),
    provenance_json_utf8: Buffer.from(canonical(provenance)).toString('base64'),
  };
  const preview = {
    binding: {
      context,
      expected_branch_id: '4',
      expected_structure_id: '5',
      expected_lineage_id: '7',
    },
    draft: content,
    main: { ...content, capture_id: context.operation_id + ':main' },
    candidate: { ...content, capture_id: context.operation_id + ':candidate' },
    preview_cut_ref: context.operation_id,
  };
  preview.input_digest = digest(preview);
  const confirmation = {
    ...preview.binding,
    context: {
      ...context,
      operation_id: 'independent-discard-confirm',
      identity_admission_ref: 'new-identity',
      document_admission_ref: 'new-document',
      target: { ...context.target, purpose: 'DISCARD_DRAFT' },
      request_fingerprint: Buffer.alloc(32, 2).toString('base64'),
    },
  };
  const authority = {
    purpose: 'DISCARD_DRAFT',
    preview_cut_ref: preview.preview_cut_ref,
    preview_input_digest: preview.input_digest,
    preview_context: context,
    expected_round_id: '0',
  };
  return { preview, confirmation, authority };
};

test('confirmation derives a new binding without changing original preview bytes or accepted sources', () => {
  const { preview, confirmation, authority } = fixture();
  const before = canonical(preview);
  const fixed = deriveDiscardConfirmation(preview, confirmation, authority);
  assert.equal(canonical(preview), before);
  assert.deepEqual(fixed.binding, confirmation);
  assert.equal(fixed.preview_cut_ref, preview.preview_cut_ref);
  assert.notEqual(fixed.input_digest, preview.input_digest);
  assert.equal(fixed.candidate.operation_id, confirmation.context.operation_id);
  assert.equal(fixed.candidate.body_json_utf8, preview.candidate.body_json_utf8);
  const provenance = JSON.parse(Buffer.from(fixed.candidate.provenance_json_utf8, 'base64'));
  assert.equal(provenance.source_operation_id, confirmation.context.operation_id);
  assert.equal(provenance.block_sources[0].origin_capture_ref, confirmation.context.operation_id);
  assert.deepEqual(provenance.accepted_captures, [{ capture_ref: 'older-accepted' }]);
});

test('confirmation rejects substituted preview identity, digest, purpose, or scope', () => {
  for (const mutate of [
    (x) => (x.authority.preview_input_digest = Buffer.alloc(32).toString('base64')),
    (x) => (x.authority.preview_context.actor_id = '9'),
    (x) => (x.confirmation.context.target.unit_id = '9'),
    (x) => (x.preview.binding.context.target.purpose = 'PULL_MAIN'),
  ]) {
    const fixtureValue = fixture();
    mutate(fixtureValue);
    assert.throws(() =>
      deriveDiscardConfirmation(
        fixtureValue.preview,
        fixtureValue.confirmation,
        fixtureValue.authority,
      ),
    );
  }
});
