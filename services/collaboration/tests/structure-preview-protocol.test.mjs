import test from 'node:test';
import assert from 'node:assert/strict';
import { structurePreviewBinding } from '../src/domain/structure_preview_protocol.mjs';

function input() {
  return {
    context: {
      operation_id: 'structure-preview-original',
      actor_id: '7',
      identity_admission_ref: 'original-identity',
      document_admission_ref: 'original-document',
      request_fingerprint: Buffer.alloc(32, 1),
      sources: [{ scope_key: 'doc-acl:3', epoch: '1' }],
      target: {
        document_id: '3',
        unit_id: '0',
        branch_id: '0',
        structure_id: '5',
        workspace_epoch: '2',
        purpose: 'STRUCTURE_PREVIEW',
      },
    },
    expected_branch_id: '0',
    expected_lineage_id: '0',
    expected_structure_id: '5',
  };
}
test('structure preview binds an explicit whole-document target and original fingerprint', () => {
  const good = input();
  assert.equal(
    structurePreviewBinding(good).context.request_fingerprint,
    Buffer.alloc(32, 1).toString('base64'),
  );
  for (const edit of [
    (p) => (p.context.target.unit_id = '9'),
    (p) => (p.context.target.branch_id = '11'),
    (p) => (p.context.target.purpose = 'PULL_MAIN'),
    (p) => (p.expected_structure_id = '6'),
    (p) => (p.expected_branch_id = '11'),
    (p) => (p.expected_lineage_id = '13'),
    (p) => (p.context.request_fingerprint = Buffer.alloc(31)),
    (p) => (p.context.target.workspace_epoch = '02'),
    (p) => (p.context.sources = []),
  ]) {
    const wrong = input();
    edit(wrong);
    assert.throws(() => structurePreviewBinding(wrong), /STRUCTURE_PREVIEW_INVALID/);
  }
});

test('structure preview reads the same exact fingerprint from protobuf JSON and RPC bytes', () => {
  const bytes = input(),
    http = input();
  http.context.request_fingerprint = bytes.context.request_fingerprint.toString('base64');
  assert.deepEqual(structurePreviewBinding(http), structurePreviewBinding(bytes));
  for (const value of [
    'invalid!',
    Buffer.alloc(31).toString('base64'),
    http.context.request_fingerprint + '\n',
  ]) {
    const wrong = input();
    wrong.context.request_fingerprint = value;
    assert.throws(() => structurePreviewBinding(wrong), /STRUCTURE_PREVIEW_INVALID/);
  }
});
