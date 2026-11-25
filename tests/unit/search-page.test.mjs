import test from 'node:test';
import assert from 'node:assert/strict';

// 测真实纯规则与状态机，不用源码包含某字符串代替分页行为；传输只作为外部完成顺序屏障。
const rules = await import('../../web/src/features/search/rules.ts');
const { createSearchPager } = await import('../../web/src/features/search/page_state.ts');
const anchors = await import('../../web/src/features/public-reading/anchors.ts');
const query = { text: '正式 😀', limit: 1, space: '' };
const item = (id) => ({ document_id: id, release_id: '2', title: '正式标题', snippet: '正文' });
const page = (id, cursor = null) => ({
  items: [item(id)],
  next_cursor: cursor,
  has_more: !!cursor,
});

// 搜索列表只展示围绕关键词的短摘录，不能把2000字投影原样铺成正文。
test('short excerpts preserve keyword context and Unicode while collapsing blank lines', () => {
  const source = '前文😀\n\n'.repeat(120) + '这里是设定的上下文' + '后文\n'.repeat(160);
  const excerpt = rules.searchExcerpt(source, '设定');
  assert.ok(excerpt.includes('这里是设定的上下文'));
  assert.ok([...excerpt].length <= 242);
  assert.equal(excerpt.includes('\n'), false);
  assert.equal(excerpt.startsWith('…'), true);
  assert.equal(excerpt.endsWith('…'), true);
  assert.equal(/\p{Surrogate}/u.test(excerpt), false);
  assert.equal(rules.searchExcerpt('短正文\n\n结束', '标题命中'), '短正文 结束');
});
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((ok, fail) => {
    resolve = ok;
    reject = fail;
  });
  return { promise, resolve, reject };
};
function harness(loader) {
  let identity = 'session-a';
  let latest;
  const pager = createSearchPager(
    loader,
    () => identity,
    (value) => (latest = value),
  );
  return { pager, state: () => latest, identity: (value) => (identity = value) };
}

test('搜索原词法、Unicode500和uint64范围原样进入URL，cursor仅原opaque值', () => {
  const text = ' 中😀 ';
  const value = rules.searchPath({ text, limit: 100, space: '18446744073709551615' }, 'YWJj');
  const url = new URL(value, 'https://localhost');
  assert.equal(url.pathname, '/v1/search');
  assert.equal(url.searchParams.get('q'), text);
  assert.equal(url.searchParams.get('space_id'), '18446744073709551615');
  assert.equal(url.searchParams.get('limit'), '100');
  assert.equal(url.searchParams.get('cursor'), 'YWJj');
  assert.doesNotThrow(() => rules.searchPath({ ...query, text: '😀'.repeat(500) }, null));
  for (const text of ['', ' \u0085\u3000', '\ud800', '😀'.repeat(501)])
    assert.throws(() => rules.searchPath({ ...query, text }, null));
  for (const space of ['0', '01', '-1', '18446744073709551616'])
    assert.throws(() => rules.searchPath({ ...query, space }, null));
  for (const limit of [0, 101, 1.5, NaN])
    assert.throws(() => rules.searchPath({ ...query, limit }, null));
  for (const cursor of ['YQ==', 'a', 'a b', 'a'.repeat(2049)])
    assert.throws(() => rules.searchPath(query, cursor));
});

test('正式结果链接只携带Doc和chapter身份，不选择旧release，最大ID不丢精度', () => {
  const value = { ...item('18446744073709551615'), chapter_id: '18446744073709551614' };
  assert.equal(
    rules.searchResultPath(value),
    '/public/documents/18446744073709551615#chapter-18446744073709551614',
  );
  assert.equal(rules.searchResultPath(item('8')), '/public/documents/8');
  assert.throws(() => rules.searchResultPath({ ...item('8'), chapter_id: '01' }));
});

test('章节定位落到当前结构首个正文单元，章节移除不回退历史版', () => {
  const entries = [
    { chapter_id: '7', kind: 'CHAPTER' },
    { chapter_id: '7', unit_id: '9', kind: 'INTRODUCTION' },
    { chapter_id: '7', unit_id: '10', kind: 'SECTION' },
    { chapter_id: '8', unit_id: '11', kind: 'CHAPTER' },
  ];
  assert.equal(anchors.chapterFromHash('#chapter-18446744073709551615'), '18446744073709551615');
  assert.equal(anchors.chapterFromHash('#chapter-01'), null);
  assert.equal(anchors.firstChapterUnit(entries, '7'), '9');
  assert.equal(anchors.firstChapterUnit(entries, '99'), null);
  assert.equal(anchors.chapterAnchor('7'), 'chapter-7');
});

test('下一页未知后显式重试原cursor，不新开链或累积未经重新核验的前页', async () => {
  const calls = [];
  let tries = 0;
  const h = harness(async (input, cursor) => {
    calls.push({ input, cursor });
    if (!cursor) return page('1', 'YWJj');
    if (++tries === 1) throw Error('lost response');
    return page('2');
  });
  await h.pager.search(query);
  assert.equal(h.state().page, 1);
  await h.pager.next();
  assert.deepEqual(h.state().items, []);
  assert.equal(h.state().retryable, true);
  await h.pager.retry();
  assert.equal(h.state().page, 2);
  assert.deepEqual(
    h.state().items.map((row) => row.document_id),
    ['2'],
  );
  assert.deepEqual(
    calls.map((call) => call.cursor),
    [null, 'YWJj', 'YWJj'],
  );
  assert.ok(calls.every((call) => JSON.stringify(call.input) === JSON.stringify(query)));
});

test('参数重置与后来的新查询使旧回包失效，迟到错误也不能覆盖新页', async () => {
  const old = deferred();
  const h = harness((input) =>
    input.text === query.text ? old.promise : Promise.resolve(page('9')),
  );
  const first = h.pager.search(query);
  h.pager.reset();
  await h.pager.search({ ...query, text: '另一查询' });
  old.reject(Error('late failure'));
  await first;
  assert.deepEqual(
    h.state().items.map((row) => row.document_id),
    ['9'],
  );
  assert.equal(h.state().error, '');
});

test('实际session身份已变即使同一用户，旧成功回包也不能重新展示', async () => {
  const old = deferred();
  const h = harness(() => old.promise);
  const pending = h.pager.search(query);
  h.identity('session-b');
  old.resolve(page('1', 'YWJj'));
  await pending;
  assert.deepEqual(h.state().items, []);
  assert.equal(h.state().nextCursor, null);
  assert.equal(h.state().loading, false);
  assert.match(h.state().error, /身份.*变化/);
});

test('自然到期、旧代次和页最终事实变化只提供重新搜索，不自动复用或降级匿名', async () => {
  for (const code of ['CURSOR_EXPIRED', 'CURSOR_STALE', 'PAGE_CHANGED_RETRY', 'SESSION_INVALID']) {
    const calls = [];
    const h = harness(async (_input, cursor) => {
      calls.push(cursor);
      if (!cursor) return page('1', 'YWJj');
      throw { code, status: code === 'SESSION_INVALID' ? 401 : 409 };
    });
    await h.pager.search(query);
    await h.pager.next();
    assert.deepEqual(h.state().items, []);
    assert.equal(h.state().nextCursor, null);
    assert.equal(h.state().retryable, false);
    await h.pager.retry();
    assert.deepEqual(calls, [null, 'YWJj']);
  }
});

test('返回有额外total/PIT、重复Doc或摘要越界时不交付部分结果', async () => {
  for (const value of [
    { ...page('1'), total: 100 },
    { items: [item('1'), item('1')], next_cursor: null, has_more: false },
    { ...page('1'), items: [{ ...item('1'), snippet: '😀'.repeat(2001) }] },
    { ...page('1'), has_more: true },
  ]) {
    const h = harness(async () => value);
    await h.pager.search({ ...query, limit: 20 });
    assert.deepEqual(h.state().items, []);
    assert.equal(h.state().nextCursor, null);
    assert.match(h.state().error, /返回.*无效/);
  }
});

test('卸载后未完成响应不再通知视图，输入对象改变也不改原请求', async () => {
  const old = deferred();
  let calls = 0;
  let input;
  const pager = createSearchPager(
    (value) => {
      input = value;
      return old.promise;
    },
    () => 'same',
    () => calls++,
  );
  const original = { ...query };
  const pending = pager.search(original);
  original.text = 'later';
  assert.equal(input.text, query.text);
  pager.dispose();
  const before = calls;
  old.resolve(page('1'));
  await pending;
  assert.equal(calls, before);
});
