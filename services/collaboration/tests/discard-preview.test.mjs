import test from 'node:test';
import assert from 'node:assert/strict';
import { canonical } from '../../../packages/content/index.mjs';
import {
  draftPreviewBinding,
  computeDraftPreview,
  draftPreviewCutDigest,
} from '../src/domain/draft_protocol.mjs';
import { createHash } from 'node:crypto';

const body = (text) => ({
  type: 'doc',
  content: [
    { type: 'paragraph', attrs: { id: 'discard-original' }, content: [{ type: 'text', text }] },
  ],
});
const input = () => ({
  context: {
    operation_id: 'discard-preview-original',
    actor_id: '1',
    target: {
      document_id: '2',
      unit_id: '3',
      branch_id: '4',
      structure_id: '5',
      workspace_epoch: '6',
      purpose: 'DISCARD_PREVIEW',
    },
    request_fingerprint: Buffer.alloc(32, 1),
    identity_admission_ref: 'original-identity',
    document_admission_ref: 'original-document',
  },
  expected_branch_id: '4',
  expected_structure_id: '5',
  expected_lineage_id: '7',
});

test('discard cut digest follows numeric room order without rewriting old PULL or fixed cuts', () => {
  const cuts = [
    { room_id: '19', workspace_epoch: '1', durable_seq: '2' },
    { room_id: '9', workspace_epoch: '2', durable_seq: '3' },
  ];
  const original = canonical(cuts);
  const digest = (value) => createHash('sha256').update(canonical(value)).digest('base64');
  assert.equal(draftPreviewCutDigest('DISCARD_PREVIEW', cuts), digest([cuts[1], cuts[0]]));
  assert.equal(draftPreviewCutDigest('PULL_MAIN', cuts), digest(cuts));
  assert.equal(canonical(cuts), original);
});

test('discard preview binds exact scope and the same digest over RPC bytes and private HTTP base64', () => {
  const value = input();
  const binding = draftPreviewBinding(value);
  const wire = structuredClone(value);
  wire.context.request_fingerprint = value.context.request_fingerprint.toString('base64');
  assert.deepEqual(draftPreviewBinding(wire), binding);
  for (const key of ['expected_branch_id', 'expected_structure_id', 'expected_lineage_id']) {
    const changed = { ...value, [key]: '0' };
    assert.throws(() => draftPreviewBinding(changed));
  }
  assert.throws(() =>
    draftPreviewBinding({
      ...value,
      context: { ...value.context, request_fingerprint: 'x'.repeat(32) },
    }),
  );
  assert.throws(() =>
    draftPreviewBinding({
      ...value,
      context: { ...value.context, target: { ...value.context.target, purpose: 'DISCARD_DRAFT' } },
    }),
  );
});

test('discard candidate is the exact fixed MAIN projection while original draft and PULL strategy stay independent', () => {
  const base = body('共同基线');
  const draft = body('旧草稿S1');
  const main = body('可靠主线');
  main.content[0].content[0].marks = [{ type: 'font', attrs: { family: 'SimSun' } }];
  const originalDraft = canonical(draft);
  const originalMain = canonical(main);
  const result = computeDraftPreview('DISCARD_PREVIEW', base, draft, main);
  assert.deepEqual(result, { body: body('可靠主线'), conflicts: [] });
  assert.equal(canonical(draft), originalDraft);
  assert.equal(canonical(main), originalMain);
  const pull = computeDraftPreview('PULL_MAIN', base, draft, main);
  assert.equal(pull.body, null);
  assert.ok(pull.conflicts.length > 0);
  assert.throws(() => computeDraftPreview('UNKNOWN', base, draft, main));
});
