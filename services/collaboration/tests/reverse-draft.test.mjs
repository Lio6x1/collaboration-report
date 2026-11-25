import test from 'node:test';
import assert from 'node:assert/strict';
import { computeReverse } from '../src/domain/reverse_draft.mjs';

const paragraph = (id, text) => ({
  type: 'paragraph',
  attrs: { id },
  content: [{ type: 'text', text }],
});
const doc = (...content) => ({ type: 'doc', content });

test('compatible reverse retains MAIN additions and later reliable draft S2', () => {
  const before = doc(paragraph('original', '原正文')),
    after = doc(paragraph('original', '已合并正文')),
    main = doc(paragraph('original', '已合并正文'), paragraph('main-add', '后来MAIN新增')),
    draft = doc(
      paragraph('original', '已合并正文'),
      paragraph('main-add', '后来MAIN新增'),
      paragraph('draft-s2', '后来草稿S2'),
    );
  // 这条支线已经拉入MAIN新增，随后才写S2；基线必须反映已拉入的MAIN正文。
  const result = computeReverse(before, after, draft, main, main);
  assert.deepEqual(result.conflicts, []);
  assert.deepEqual(
    result.body,
    doc(
      paragraph('original', '原正文'),
      paragraph('main-add', '后来MAIN新增'),
      paragraph('draft-s2', '后来草稿S2'),
    ),
  );
  assert.equal(after.content[0].content[0].text, '已合并正文');
});

test('overlapping later MAIN or draft input stops reverse without a candidate', () => {
  const before = doc(paragraph('original', '原正文')),
    after = doc(paragraph('original', '已合并正文')),
    changed = doc(paragraph('original', '后来同处修改'));
  for (const [draft, main] of [
    [after, changed],
    [changed, after],
  ]) {
    const result = computeReverse(before, after, draft, main, after);
    assert.equal(result.body, null);
    assert.ok(result.conflicts.length > 0);
  }
});

test('restoring a replaced block and later insertion at the same unanchored boundary remains a conflict', () => {
  const before = {
      type: 'doc',
      content: [{ type: 'paragraph', attrs: { id: 'initial-2695' }, content: [] }],
    },
    after = doc(paragraph('review-paragraph', '已合并正文')),
    draft = doc(
      paragraph('review-paragraph', '已合并正文'),
      paragraph('later-unmerged-s2', '后来草稿S2'),
    ),
    result = computeReverse(before, after, draft, after, after);
  assert.equal(result.body, null);
  assert.ok(result.conflicts.some((conflict) => conflict.kind === 'INSERT_ORDER'));
});

test('reverse keeps compatible MAIN additions that have not been pulled into the draft baseline', () => {
  const before = doc(paragraph('original', '原正文')),
    after = doc(paragraph('original', '已合并正文')),
    main = doc(
      paragraph('original', '已合并正文'),
      paragraph('main-tail', '尚未拉入支线的主线新增'),
    ),
    result = computeReverse(before, after, after, main, after);
  assert.deepEqual(result.conflicts, []);
  assert.deepEqual(
    result.body,
    doc(paragraph('original', '原正文'), paragraph('main-tail', '尚未拉入支线的主线新增')),
  );
});

test('reverse removes the original change from a stale draft even when MAIN already equals the original before body', () => {
  const before = doc(paragraph('original', '原正文')),
    after = doc(paragraph('original', '已合并正文')),
    result = computeReverse(before, after, after, before, before);
  // MAIN已经恢复并不代表旧支线也已恢复；不能把旧S1误当作后来应保留的S2。
  assert.deepEqual(result.conflicts, []);
  assert.deepEqual(result.body, before);
});
