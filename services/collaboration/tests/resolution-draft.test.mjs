import test from 'node:test';
import assert from 'node:assert/strict';
import { computeResolution } from '../src/domain/resolution_draft.mjs';

const paragraph = (id, text) => ({
  type: 'paragraph',
  attrs: { id },
  content: [{ type: 'text', text }],
});
const doc = (...content) => ({ type: 'doc', content });

test('manual resolution applies only its original submitted delta and retains later draft S2', () => {
  const submitted = doc(paragraph('original', '原提交S1'));
  const resolution = doc(paragraph('original', '人工修复后的正文'));
  const draft = doc(paragraph('original', '原提交S1'), paragraph('later', '后来可靠S2'));
  const originals = JSON.stringify({ submitted, resolution, draft });
  const result = computeResolution(submitted, resolution, draft);
  assert.deepEqual(result.conflicts, []);
  assert.deepEqual(
    result.body,
    doc(paragraph('original', '人工修复后的正文'), paragraph('later', '后来可靠S2')),
  );
  assert.equal(JSON.stringify({ submitted, resolution, draft }), originals);
});

test('overlapping later draft edit or deletion stops manual resolution without replacing the draft', () => {
  const submitted = doc(paragraph('original', '原提交S1'));
  const resolution = doc(paragraph('original', '人工修复后的正文'));
  for (const draft of [
    doc(paragraph('original', '后来同处修改S2')),
    doc(paragraph('remaining', '删除原块后保留其他合法正文')),
  ]) {
    const result = computeResolution(submitted, resolution, draft);
    assert.equal(result.body, null);
    assert.ok(result.conflicts.length > 0);
  }
});

test('manual repair and compatible later Unicode edit in the same paragraph are both retained', () => {
  const result = computeResolution(
    doc(paragraph('original', '开始🌏中间结束')),
    doc(paragraph('original', '人工🌏中间结束')),
    doc(paragraph('original', '开始🌏中间结束，后来S2')),
  );
  assert.deepEqual(result.conflicts, []);
  assert.deepEqual(result.body, doc(paragraph('original', '人工🌏中间结束，后来S2')));
});

test('manual resolution cannot remove branch-forbidden font marks silently', () => {
  const submitted = doc(paragraph('original', '原提交S1'));
  const resolution = structuredClone(submitted);
  resolution.content[0].content[0].marks = [{ type: 'font', attrs: { family: 'SimSun' } }];
  assert.throws(() => computeResolution(submitted, resolution, submitted), {
    code: 3,
    details: 'RESOLUTION_CONTENT_INVALID',
  });
});

test('invalid proposal table geometry is a definite input rejection while broken fixed sources stay unknown', () => {
  const submitted = doc(paragraph('original', '原提交S1'));
  const invalid = doc({
    type: 'table',
    attrs: { id: 'table' },
    content: [
      {
        type: 'table_row',
        attrs: { id: 'row' },
        content: [
          {
            type: 'table_cell',
            attrs: { id: 'cell', colspan: 1, rowspan: 2, colwidth: null },
            content: [paragraph('inside', '格')],
          },
        ],
      },
    ],
  });
  assert.throws(() => computeResolution(submitted, invalid, submitted), {
    code: 3,
    details: 'RESOLUTION_CONTENT_INVALID',
  });
  // 持久submitted或可靠draft坏掉属于未知来源，不能伪装成用户正文拒绝后开放控制。
  assert.throws(() => computeResolution(invalid, submitted, submitted), {
    code: 'INVALID_CONTENT',
  });
  assert.throws(() => computeResolution(submitted, submitted, invalid), {
    code: 'INVALID_CONTENT',
  });
});
