import test from 'node:test';
import assert from 'node:assert/strict';
import { matchesMainWorkspace, preparedMainSource } from '../src/domain/main_workspace.mjs';

const sample = () => ({
  current: { manifest_id: '5', structure_id: '7', style_id: '11', control_revision: '2' },
  room: { epoch: '2' },
  source: {
    manifest_id: '5',
    structure_id: '7',
    style_id: '11',
    control_revision: '4',
    workspace_epoch: '2',
  },
});

// 元数据control领先不意味着room epoch可任意放宽；完整物理来源仍必须与Doc权威来源精确一致。
test('MAIN physical context stays exact when document metadata control advances', () => {
  const s = sample();
  assert.equal(matchesMainWorkspace(s.current, s.room, s.source), true);
  for (const change of [
    (v) => {
      v.current.manifest_id = '6';
    },
    (v) => {
      v.current.structure_id = '8';
    },
    (v) => {
      v.current.style_id = '12';
    },
    (v) => {
      v.current.control_revision = '3';
    },
    (v) => {
      v.room.epoch = '3';
    },
    (v) => {
      v.source.workspace_epoch = '3';
    },
    (v) => {
      v.source.workspace_epoch = undefined;
    },
    (v) => {
      v.source.control_revision = '1';
    },
  ]) {
    const wrong = sample();
    change(wrong);
    assert.equal(matchesMainWorkspace(wrong.current, wrong.room, wrong.source), false);
  }
});

test('MAIN physical epoch preserves complete uint64 and rejects noncanonical or unsafe values', () => {
  for (const epoch of ['18446744073709551615', '18446744073709551614']) {
    const s = sample();
    s.current.control_revision = s.room.epoch = s.source.workspace_epoch = epoch;
    s.source.control_revision = '18446744073709551615';
    assert.equal(matchesMainWorkspace(s.current, s.room, s.source), true);
  }
  for (const epoch of ['0', '02', '+2', '-2', '18446744073709551616', 2, undefined]) {
    const s = sample();
    s.current.control_revision = s.room.epoch = s.source.workspace_epoch = epoch;
    assert.equal(matchesMainWorkspace(s.current, s.room, s.source), false);
  }
});

const preparation = () => {
  const binding = {
    context: {
      operation_id: 'original',
      request_fingerprint: 'fixed-original',
      target: { purpose: 'MERGE' },
    },
    expected_cut: {
      document_id: '3',
      structure_id: '7',
      style_id: '11',
      control_revision: '4',
      rooms: [{ room_id: '13', workspace_epoch: '2', durable_seq: '5' }],
      dependency_digest: 'original-cut',
    },
  };
  return {
    binding,
    facts: [
      {
        context: structuredClone(binding.context),
        cut: structuredClone(binding.expected_cut),
        manifest: '5',
        control: '4',
      },
    ],
  };
};

// 放行旧物理代次前，要有独立保存的同原号、同上下文、同全篇cut捕获；调用方声称一个cut不足以通过。
test('metadata-gap preparation requires independently fixed original capture and exact complete cut', () => {
  const original = preparation();
  assert.deepEqual(preparedMainSource(original.binding, original.facts), {
    manifest_id: '5',
    structure_id: '7',
    style_id: '11',
    control_revision: '4',
  });
  assert.equal(preparedMainSource(original.binding, []), null);
  for (const change of [
    (f) => {
      f.context.operation_id = 'another';
    },
    (f) => {
      f.context.request_fingerprint = 'another';
    },
    (f) => {
      f.cut.rooms[0].workspace_epoch = '3';
    },
    (f) => {
      f.cut.rooms[0].durable_seq = '6';
    },
    (f) => {
      f.cut.style_id = '12';
    },
    (f) => {
      f.manifest = '05';
    },
    (f) => {
      f.control = '3';
    },
  ]) {
    const wrong = preparation();
    change(wrong.facts[0]);
    assert.equal(preparedMainSource(wrong.binding, wrong.facts), null);
  }
});
