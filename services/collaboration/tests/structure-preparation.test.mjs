import test from 'node:test';
import assert from 'node:assert/strict';
import { canonical } from '../../../packages/content/index.mjs';
import { hash } from '../src/domain/shared_protocol.mjs';
import { cutDigest } from '../src/domain/main_protocol.mjs';
import { structurePreparation } from '../src/domain/structure_preparation.mjs';

function fixture() {
  const rooms = [
    { room_id: '11', workspace_epoch: '2', durable_seq: '4' },
    { room_id: '12', workspace_epoch: '2', durable_seq: '5' },
    { room_id: '21', workspace_epoch: '1', durable_seq: '7' },
    { room_id: '22', workspace_epoch: '1', durable_seq: '8' },
  ];
  const fixed = {
    binding: {
      context: {
        actor_id: '7',
        target: { document_id: '3', structure_id: '5', workspace_epoch: '2' },
        sources: [{ scope_key: 'doc-acl:3', epoch: '1' }],
      },
    },
    preview_cut_ref: 'original-preview',
    expires_unix_ms: '9999999999999',
    observed_cut: {
      document_id: '3',
      structure_id: '5',
      style_id: '6',
      control_revision: '2',
      rooms,
      dependency_digest: cutDigest(rooms),
    },
    source: {
      drafts: [
        {
          unit_id: '1',
          branch_id: '31',
          room_id: '21',
          workspace_epoch: '1',
          lineage: { lineage_id: '41' },
        },
        {
          unit_id: '2',
          branch_id: '32',
          room_id: '22',
          workspace_epoch: '1',
          lineage: { lineage_id: '42' },
        },
      ],
    },
    migration: {
      schema_version: 'structure-migration-v1',
      changes: [{ type: 'SPLIT_LEVEL2', source_units: ['1'] }],
    },
  };
  fixed.input_digest = hash(canonical(fixed)).toString('base64');
  const authority = {
    reference: fixed.preview_cut_ref,
    digest: fixed.input_digest,
    expires: fixed.expires_unix_ms,
  };
  const binding = {
    context: {
      actor_id: '7',
      target: {
        document_id: '3',
        structure_id: '5',
        workspace_epoch: '2',
        purpose: 'MAIN_CONTROL',
      },
      sources: fixed.binding.context.sources,
    },
  };
  return { fixed, authority, binding };
}

test('range prepare retains every MAIN cut and only affected real BRANCH cuts', () => {
  const f = fixture(),
    p = structurePreparation(f.fixed, f.authority, f.binding, 1000);
  assert.deepEqual(
    p.cut.rooms.map((v) => v.room_id),
    ['11', '12', '21'],
  );
  assert.equal(p.cut.dependency_digest, cutDigest(p.cut.rooms));
  assert.equal(p.branches.length, 1);
  assert.equal(p.branches[0].present, true);
  assert.deepEqual(
    f.fixed.observed_cut.rooms.map((v) => v.room_id),
    ['11', '12', '21', '22'],
  );
});

test('absent affected branch is still part of the durable gate scope', () => {
  const f = fixture();
  f.fixed.observed_cut.rooms = f.fixed.observed_cut.rooms.filter((v) => v.room_id !== '21');
  f.fixed.observed_cut.dependency_digest = cutDigest(f.fixed.observed_cut.rooms);
  delete f.fixed.input_digest;
  f.fixed.input_digest = hash(canonical(f.fixed)).toString('base64');
  f.authority.digest = f.fixed.input_digest;
  const p = structurePreparation(f.fixed, f.authority, f.binding, 1000);
  assert.equal(p.branches[0].room_id, '21');
  assert.equal(p.branches[0].present, false);
  assert.deepEqual(
    p.cut.rooms.map((v) => v.room_id),
    ['11', '12'],
  );
});

test('wrong original, expired preview, changed source and omitted branch fail closed', () => {
  for (const change of [
    (f) => (f.authority.reference = 'other-preview'),
    (f) => (f.authority.digest = Buffer.alloc(32).toString('base64')),
    (f) => (f.authority.expires = '9999999999998'),
    (f) => (f.binding.context.actor_id = '8'),
    (f) => (f.binding.context.target.document_id = '4'),
    (f) => (f.binding.context.sources = [{ scope_key: 'doc-acl:3', epoch: '2' }]),
    (f) => f.fixed.source.drafts.pop(),
  ]) {
    const f = fixture();
    change(f);
    assert.throws(
      () => structurePreparation(f.fixed, f.authority, f.binding, 1000),
      /STRUCTURE_PREPARATION_UNKNOWN/,
    );
  }
  const f = fixture();
  assert.throws(
    () => structurePreparation(f.fixed, f.authority, f.binding, 10000000000000),
    /STRUCTURE_PREVIEW_EXPIRED/,
  );
});
