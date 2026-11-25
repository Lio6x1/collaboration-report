import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createCursorPage,
  pageCursor,
  acceptPage,
  resetPage,
} from '../../web/src/lib/pagination.ts';

// 捕捉“下一页仍用首页游标”、上一页复用旧结果、分支历史未清理等真实导航错误。
test('cursor navigation reuses only cursor starts and discards superseded later pages', () => {
  const state = createCursorPage();
  assert.equal(pageCursor(state, 0), null);
  acceptPage(state, 0, null, 'second');
  assert.equal(pageCursor(state, 1), 'second');
  acceptPage(state, 1, 'second', 'third');
  assert.equal(state.index, 1);
  assert.equal(pageCursor(state, 0), null);
  acceptPage(state, 0, null, 'new-second');
  assert.equal(pageCursor(state, 1), 'new-second');
  assert.throws(() => pageCursor(state, 2));
});

test('empty visible pages can continue while an exhausted page cannot advance', () => {
  const state = createCursorPage();
  acceptPage(state, 0, null, 'authorized-scan-continues');
  assert.equal(pageCursor(state, 1), 'authorized-scan-continues');
  acceptPage(state, 1, 'authorized-scan-continues', null);
  assert.throws(() => pageCursor(state, 2));
  assert.equal(pageCursor(state, 0), null);
});

test('scope or filter reset removes all previous cursors', () => {
  const state = createCursorPage();
  acceptPage(state, 0, null, 'old-context');
  acceptPage(state, 1, 'old-context', 'old-next');
  resetPage(state);
  assert.equal(state.index, 0);
  assert.equal(state.next, null);
  assert.equal(pageCursor(state, 0), null);
  assert.throws(() => pageCursor(state, 1));
  for (const target of [-1, 0.5, NaN, Infinity]) assert.throws(() => pageCursor(state, target));
});
