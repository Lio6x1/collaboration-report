import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';
import ts from 'typescript';

const require = createRequire(import.meta.url);
const read = (path) => readFileSync(new URL('../../web/src/' + path, import.meta.url), 'utf8');
function moduleURL(source) {
  return (
    'data:text/javascript;base64,' +
    Buffer.from(
      ts.transpileModule(source, {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
      }).outputText,
    ).toString('base64')
  );
}
async function history(feature) {
  const vueURL = pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href;
  const vue = await import(vueURL);
  // 真正运行响应式组合函数；组件卸载钩子映射到测试scope，不需启动用户实例。
  const lifecycle = moduleURL(
    `export * from '${vueURL}'; import { onScopeDispose } from '${vueURL}'; export const onBeforeUnmount = onScopeDispose; export const onMounted = () => {};`,
  );
  const httpURL =
    moduleURL(read('lib/http.ts').replace("from 'vue'", `from '${vueURL}'`)) + '#' + Math.random();
  const pagination = moduleURL(read('lib/pagination.ts'));
  const adapt = (source) =>
    source
      .replaceAll("from 'vue'", `from '${lifecycle}'`)
      .replaceAll("from '../../lib/http'", `from '${httpURL}'`)
      .replaceAll("from '../../lib/pagination'", `from '${pagination}'`);
  let source = adapt(
    read(
      `features/${feature}/use${{ trash: 'Trash', transfers: 'PersonalTransfers', publication: 'Publication', review: 'Reviews' }[feature]}.ts`,
    ),
  );
  if (['publication', 'review'].includes(feature)) {
    const api = moduleURL(adapt(read(`features/${feature}/api.ts`)));
    source = source.replace("from './api'", `from '${api}'`);
  }
  if (feature === 'review')
    source = source.replace(
      "from './reviewDiff'",
      `from '${moduleURL(read('features/review/reviewDiff.ts'))}'`,
    );
  const exports = await import(moduleURL(source) + '#' + Math.random());
  const scope = vue.effectScope();
  return {
    ...exports,
    vue,
    http: await import(httpURL),
    start: (run) => scope.run(run),
    stop: () => scope.stop(),
  };
}
const response = (value, status = 200) =>
  new Response(JSON.stringify(value), { status, headers: { 'content-type': 'application/json' } });
const page = (items, next_cursor = null) => ({ items, next_cursor, has_more: !!next_cursor });
const items = (prefix, start = 1, count = 20) =>
  Array.from({ length: count }, (_, i) => ({ id: prefix + (start + i) }));
const settle = () => new Promise(setImmediate);

// 将load改回追加会让本页变成40条；上一页必须再请求服务端，不能恢复旧授权结果。
test('trash replaces twenty rows per page and rechecks previous-page authorization', async () => {
  const h = await history('trash');
  const reads = [];
  let firstReads = 0;
  globalThis.fetch = async (path) => {
    reads.push(path);
    if (path.includes('cursor=next')) return response(page(items('trash', 21)));
    firstReads++;
    return response(page(firstReads === 1 ? items('trash') : [{ id: 'still-authorized' }], 'next'));
  };
  const state = h.start(() =>
    h.useTrash(
      () => 'space',
      () => {},
    ),
  );
  await settle();
  await state.load(1);
  assert.equal(state.documents.value.length, 20);
  assert.equal(state.documents.value[0].id, 'trash21');
  assert.equal(state.pagination.index, 1);
  await state.load(0);
  assert.deepEqual(
    state.documents.value.map((x) => x.id),
    ['still-authorized'],
  );
  assert.equal(reads.length, 3);
  assert.ok(reads.every((path) => path.includes('limit=20')));
  h.stop();
});

test('late trash pages cannot restore the previous space after a scope change', async () => {
  const h = await history('trash');
  const space = h.vue.ref('old');
  let release;
  globalThis.fetch = async (path) =>
    path.includes('cursor=next')
      ? new Promise((done) => {
          release = () => done(response(page([{ id: 'old-secret' }])));
        })
      : response(page([{ id: path.includes('/old/') ? 'old' : 'new' }], 'next'));
  const state = h.start(() =>
    h.useTrash(
      () => space.value,
      () => {},
    ),
  );
  await settle();
  const pending = state.load(1);
  space.value = 'new';
  await settle();
  release();
  await pending;
  assert.deepEqual(
    state.documents.value.map((x) => x.id),
    ['new'],
  );
  assert.equal(state.pagination.index, 0);
  h.stop();
});

test('personal transfer history replaces its page while the incoming target picker remains on demand', async () => {
  const h = await history('transfers');
  const reads = [];
  globalThis.fetch = async (path) => {
    reads.push(path);
    if (path.includes('/personal-transfers'))
      return response(
        path.includes('cursor=next') ? page(items('offer', 21)) : page(items('offer'), 'next'),
      );
    if (path.includes('/me/spaces'))
      return response(page([{ id: 'personal', kind: 'PERSONAL', lifecycle: 'ACTIVE' }]));
    if (path.endsWith('/quota')) return response({ revision: 'quota1' });
    return response(page([{ id: 'kb', lifecycle: 'ACTIVE' }], 'more-targets'));
  };
  const state = h.start(() => h.usePersonalTransfers());
  await settle();
  await state.load(1);
  assert.equal(state.offers.value.length, 20);
  assert.equal(state.offers.value[0].id, 'offer21');
  assert.equal(state.pagination.index, 1);
  assert.equal(state.knowledgeBase.value, 'kb');
  assert.equal(state.knowledgeBaseCursor.value, 'more-targets');
  assert.equal(reads.filter((path) => path.includes('/knowledge-bases')).length, 1);
  h.stop();
});

test('publication pages preserve the fixed preview and the original write choice', async () => {
  const h = await history('publication');
  const doc = { id: 'doc', control_revision: '7', publication_epoch: '8' };
  globalThis.fetch = async (path) =>
    path.endsWith('/old')
      ? response({ release_id: 'old', document_id: 'doc' })
      : response(
          path.includes('cursor=next')
            ? page(items('release', 21))
            : page([{ id: 'old', created_at: '2026-10-01T00:00:00Z' }], 'next'),
        );
  const state = h.start(() =>
    h.usePublication(
      () => doc,
      async () => {},
      async () => {},
    ),
  );
  await state.refresh();
  await state.preview(state.releases.value[0]);
  state.choose('SWITCH', state.releases.value[0]);
  await state.loadPage(1);
  assert.equal(state.releases.value.length, 20);
  assert.equal(state.releases.value[0].id, 'release21');
  assert.equal(state.fixed.value.release_id, 'old');
  assert.equal(state.fixedRelease.value.id, 'old');
  assert.equal(state.choice.value.document.publication_epoch, '8');
  await state.refresh();
  assert.equal(state.fixed.value.release_id, 'old');
  h.stop();
});

test('publication ignores an old preview after the document changes', async () => {
  const h = await history('publication');
  const doc = h.vue.ref({ id: 'old' });
  let release;
  globalThis.fetch = async (path) =>
    path.endsWith('/release')
      ? new Promise((done) => {
          release = () => done(response({ release_id: 'release', document_id: 'old' }));
        })
      : response(page([{ id: path.includes('/old/') ? 'old-release' : 'new-release' }]));
  const state = h.start(() =>
    h.usePublication(
      () => doc.value,
      async () => {},
      async () => {},
    ),
  );
  await state.refresh();
  const pending = state.preview({ id: 'release' });
  doc.value = { id: 'new' };
  await settle();
  release();
  await pending;
  assert.equal(state.fixed.value, null);
  assert.deepEqual(
    state.releases.value.map((x) => x.id),
    ['new-release'],
  );
  h.stop();
});

const round = (id, request = 'request1', submitter = 'other') => ({
  id,
  request_id: request,
  round_no: 1,
  submitter_id: submitter,
  content_version_id: 'content-' + id,
  base_version_id: 'base-' + id,
  content: { id: 'content-' + id, document_id: 'doc', unit_id: 'unit' },
  structure: { entries: [] },
});
const request = (id = 'request1') => ({
  id,
  document_id: 'doc',
  unit_id: 'unit',
  current_round_id: 'round1',
  current_round: round('round1', id),
  actions: ['REQUEST_CHANGES'],
  status: 'REVIEWING',
});
function reviewFetch(reads, extra = () => undefined) {
  return async (path) => {
    reads.push(path);
    const override = extra(path);
    if (override !== undefined) return override;
    if (path.includes('/review-requests?limit=100'))
      return response(
        page([
          request(),
          ...items('request', 21).map((value) => ({ ...request(value.id), status: 'MERGED' })),
        ]),
      );
    if (path.includes('/review-requests?'))
      return response(
        path.includes('cursor=request-next')
          ? page(items('request', 21))
          : page([request()], 'request-next'),
      );
    if (path.includes('/rounds?'))
      return response(
        path.includes('cursor=round-next')
          ? page([round('round2')])
          : page([round('round1')], 'round-next'),
      );
    if (path.includes('/change-requests?'))
      return response(
        path.includes('cursor=change-next')
          ? page(items('change', 21))
          : page(items('change'), 'change-next'),
      );
    if (path.includes('/content-versions/'))
      return response({ id: path.split('/').at(-1), document_id: 'doc', unit_id: 'unit' });
    return response(request(path.split('/').at(-1)));
  };
}

test('review request pages preserve selected detail and both kinds of local draft', async () => {
  const h = await history('review');
  const reads = [];
  globalThis.fetch = reviewFetch(reads);
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
    ),
  );
  await state.load();
  state.reason.value = '未提交的审核理由';
  state.resolution.value.change1 = '未提交的解除理由';
  assert.equal(typeof state.loadRequests, 'function', '审核请求必须暴露替换页面的导航');
  await state.loadRequests(1);
  assert.equal(state.requests.value.length, 20);
  assert.equal(state.requests.value[0].id, 'request21');
  assert.equal(state.request.value.id, 'request1');
  assert.equal(state.fixed.value.id, 'round1');
  assert.equal(state.reason.value, '未提交的审核理由');
  assert.equal(state.resolution.value.change1, '未提交的解除理由');
  assert.equal(reads.filter((path) => path.includes('/review-requests?limit=20')).length, 2);
  assert.ok(
    reads
      .filter((path) => path.includes('/change-requests?'))
      .every((path) => path.includes('limit=20')),
  );
  h.stop();
});

test('review round pages preserve the fixed selection and evaluate submission eligibility across all pages', async () => {
  const h = await history('review');
  const reads = [];
  globalThis.fetch = reviewFetch(reads, (path) =>
    path.includes('cursor=round-next')
      ? response(page([round('round2', 'request1', 'me')]))
      : undefined,
  );
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
    ),
  );
  await state.load();
  assert.ok(
    reads.some((path) => path.includes('cursor=round-next')),
    '资格判断必须独立核完后页',
  );
  assert.equal(state.canResolve({ author_id: 'me' }), false, '自己参与过后页送审不能解除要求');
  assert.equal(typeof state.loadRounds, 'function');
  await state.loadRounds(1);
  assert.deepEqual(
    state.displayedRounds.value.map((x) => x.id),
    ['round2'],
  );
  assert.equal(state.fixed.value.id, 'round1');
  assert.equal(state.selectedRound.value, 'round1');
  h.stop();
});

test('change-request pages retain drafts and cannot redefine the whole request blocking count', async () => {
  const h = await history('review');
  globalThis.fetch = reviewFetch([]);
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => true,
    ),
  );
  await state.load();
  state.request.value.blocking_change_count = 27;
  state.resolution.value.change1 = '跨页草稿';
  assert.equal(typeof state.loadChanges, 'function');
  await state.loadChanges(1);
  assert.equal(state.changes.value.length, 20);
  assert.equal(state.changes.value[0].id, 'change21');
  assert.equal(state.resolution.value.change1, '跨页草稿');
  assert.equal(state.request.value.blocking_change_count, 27);
  h.stop();
});

test('a late review request page cannot reappear after the session generation changes', async () => {
  const h = await history('review');
  const fallback = reviewFetch([]);
  let release;
  let changed = false;
  globalThis.fetch = async (path) => {
    if (path.includes('cursor=request-next'))
      return new Promise((done) => {
        release = () => done(response(page([{ id: 'old-session-secret' }])));
      });
    if (changed && path.includes('/review-requests?')) return response(page([request('new')]));
    return fallback(path);
  };
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
    ),
  );
  await state.load();
  state.reason.value = '旧身份草稿';
  const pending = state.loadRequests(1);
  changed = true;
  h.http.rotateSessionContext();
  await settle();
  release();
  await pending;
  assert.deepEqual(
    state.requests.value.map((value) => value.id),
    ['new'],
  );
  assert.equal(state.requestPagination.index, 0);
  assert.equal(state.request.value.id, 'new');
  assert.equal(state.reason.value, '');
  h.stop();
});

test('a late change-request page cannot overwrite a different selected review request', async () => {
  const h = await history('review');
  let release;
  globalThis.fetch = reviewFetch([], (path) => {
    if (path.includes('request1/change-requests?') && path.includes('cursor=change-next'))
      return new Promise((done) => {
        release = () => done(response(page([{ id: 'old-change' }])));
      });
    if (path.includes('request2/change-requests?')) return response(page([{ id: 'new-change' }]));
    if (path.includes('request2/rounds?')) return response(page([round('round1', 'request2')]));
  });
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
    ),
  );
  await state.load();
  const pending = state.loadChanges(1);
  await state.open('request2');
  release();
  await pending;
  assert.equal(state.request.value.id, 'request2');
  assert.deepEqual(
    state.changes.value.map((value) => value.id),
    ['new-change'],
  );
  assert.equal(state.changePagination.index, 0);
  assert.equal(state.loading.value, false);
  h.stop();
});

test('a repeated eligibility cursor fails closed without an unbounded scan', async () => {
  const h = await history('review');
  let pages = 0;
  globalThis.fetch = reviewFetch([], (path) => {
    if (!path.includes('/rounds?')) return;
    pages++;
    assert.ok(pages <= 2);
    return response(page([round('round' + pages)], 'repeated'));
  });
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
    ),
  );
  await state.load();
  assert.equal(state.canResolve({ author_id: 'me' }), false);
  assert.match(state.readingError.value, /分页.*资格/);
  assert.equal(pages, 2);
  h.stop();
});

test('review paging keeps the unknown operation and retries its original input while preserving a newer draft', async () => {
  const h = await history('review');
  const fallback = reviewFetch([]);
  const writes = [];
  let recovering = false;
  globalThis.fetch = async (path, options) => {
    if (options.method !== 'POST') return fallback(path);
    const input = JSON.parse(options.body);
    writes.push(input);
    if (!recovering) throw new TypeError('lost original response');
    return response({ operation_id: input.operation_id, state: 'SUCCEEDED' });
  };
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
    ),
  );
  await state.load();
  state.reason.value = '原提交理由';
  await state.requestChanges();
  const operation = state.action.pending.value;
  const retry = state.action.retry.value;
  assert.ok(operation);
  assert.equal(state.reason.value, '原提交理由');
  await state.loadRequests(1);
  assert.equal(state.action.pending.value, operation);
  assert.equal(state.action.retry.value, retry);
  await state.refresh();
  assert.equal(state.requestPagination.index, 0);
  assert.equal(state.action.pending.value, operation);
  assert.equal(state.action.retry.value, retry);
  state.reason.value = '后来编辑的本机草稿';
  recovering = true;
  await retry();
  assert.equal(writes.length, 2);
  assert.deepEqual(writes[0], writes[1]);
  assert.equal(writes[1].reason, '原提交理由');
  assert.equal(state.reason.value, '后来编辑的本机草稿');
  assert.equal(state.action.pending.value, '');
  h.stop();
});

test('refresh after archiving a request selects the new active request without adding it to another page', async () => {
  const h = await history('review');
  let archived = false;
  globalThis.fetch = reviewFetch([], (path) => {
    if (!archived) return;
    if (path.includes('/review-requests?'))
      return response(
        page([{ ...request(), archived_at: '2026-10-13T00:00:00Z' }, request('request2')]),
      );
    if (path.endsWith('/request1'))
      return response({ ...request(), archived_at: '2026-10-13T00:00:00Z' });
    if (path.includes('request2/rounds?')) return response(page([round('round1', 'request2')]));
  });
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
    ),
  );
  await state.load();
  assert.equal(state.request.value.id, 'request1');
  archived = true;
  await state.load();
  assert.equal(state.request.value.id, 'request2');
  assert.equal(state.requests.value.length, 2);
  h.stop();
});

test('publication paging leaves an unknown switch bound to its original release and revision', async () => {
  const h = await history('publication');
  const doc = h.vue.ref({ id: 'doc', publication_epoch: '8', control_revision: '7' });
  const writes = [];
  let recovering = false;
  globalThis.fetch = async (path, options) => {
    if (options.method === 'POST') {
      const input = JSON.parse(options.body);
      writes.push(input);
      if (!recovering) throw new TypeError('lost original response');
      return response({ operation_id: input.operation_id, state: 'SUCCEEDED' });
    }
    return response(
      path.includes('cursor=next')
        ? page(items('release', 21))
        : page([{ id: 'original-release' }], 'next'),
    );
  };
  const state = h.start(() =>
    h.usePublication(
      () => doc.value,
      async () => {},
      async () => {},
    ),
  );
  await state.refresh();
  state.choose('SWITCH', state.releases.value[0]);
  await state.confirm();
  const operation = state.action.pending.value;
  const retry = state.action.retry.value;
  assert.ok(operation);
  await state.loadPage(1);
  await state.refresh();
  assert.equal(state.pagination.index, 0);
  doc.value = { ...doc.value, publication_epoch: '99' };
  assert.equal(state.action.pending.value, operation);
  recovering = true;
  await retry();
  assert.deepEqual(writes[0], writes[1]);
  assert.equal(writes[1].release_id, 'original-release');
  assert.equal(writes[1].expected_publication_revision, '8');
  assert.equal(state.action.pending.value, '');
  h.stop();
});

for (const feature of ['trash', 'transfers', 'publication']) {
  test(`${feature} manual refresh recovers an expired third page by discarding its cursor chain`, async () => {
    const h = await history(feature);
    let expired = false;
    const reads = [];
    globalThis.fetch = async (path) => {
      if (path.includes('/me/spaces'))
        return response(page([{ id: 'personal', kind: 'PERSONAL', lifecycle: 'ACTIVE' }]));
      if (path.includes('/knowledge-bases'))
        return response(page([{ id: 'kb', lifecycle: 'ACTIVE' }]));
      if (path.endsWith('/quota')) return response({ revision: 'quota1' });
      reads.push(path);
      const cursor = new URL(path, 'https://history.test').searchParams.get('cursor');
      if (expired && cursor) return response({ code: 'CURSOR_INVALID' }, 400);
      return response(
        cursor === 'third'
          ? page(items('entry', 41))
          : cursor === 'second'
            ? page(items('entry', 21), 'third')
            : page(items('entry'), 'second'),
      );
    };
    const state = h.start(() =>
      feature === 'trash'
        ? h.useTrash(
            () => 'space',
            () => {},
          )
        : feature === 'transfers'
          ? h.usePersonalTransfers()
          : h.usePublication(
              () => ({ id: 'doc' }),
              async () => {},
              async () => {},
            ),
    );
    await settle();
    const navigate = (index) =>
      feature === 'publication' ? (state.loadPage ?? state.refresh)(index) : state.load(index);
    const values =
      feature === 'trash'
        ? state.documents
        : feature === 'transfers'
          ? state.offers
          : state.releases;
    await navigate(1);
    await navigate(2);
    assert.equal(state.pagination.index, 2);
    if (feature === 'publication') {
      state.fixed.value = { release_id: 'entry1' };
      state.fixedRelease.value = { id: 'entry1' };
      state.choose('SWITCH', { id: 'entry1' });
    }
    expired = true;
    await navigate(2);
    assert.ok(state.readingError.value);
    assert.equal(typeof state.refresh, 'function', '页面需要独立的手动刷新恢复入口');
    await state.refresh();
    assert.equal(state.pagination.index, 0);
    assert.deepEqual(state.pagination.cursors, [null]);
    assert.equal(state.readingError.value, '');
    assert.equal(values.value[0].id, 'entry1');
    assert.equal(new URL(reads.at(-1), 'https://history.test').searchParams.has('cursor'), false);
    if (feature === 'publication') {
      assert.equal(state.fixed.value.release_id, 'entry1');
      assert.equal(state.choice.value.release.id, 'entry1');
    }
    h.stop();
  });
}

test('review manual refreshes recover expired request round and change pages without discarding the fixed draft', async () => {
  const h = await history('review');
  let expired = false;
  globalThis.fetch = reviewFetch([], (path) => {
    const type = path.includes('/review-requests?')
      ? 'request'
      : path.includes('/rounds?')
        ? 'round'
        : path.includes('/change-requests?')
          ? 'change'
          : '';
    if (!type) return;
    const cursor = new URL(path, 'https://history.test').searchParams.get('cursor');
    if (expired && cursor && !cursor.startsWith('fresh-'))
      return response({ code: 'CURSOR_INVALID' }, 400);
    const prefix = expired ? 'fresh-' : '';
    const values =
      type === 'request'
        ? [request()]
        : type === 'round'
          ? [round(cursor ? (cursor.endsWith('third') ? 'round3' : 'round2') : 'round1')]
          : [{ id: cursor || 'change1' }];
    return response(
      page(
        values,
        cursor?.endsWith('third') ? null : prefix + type + (cursor ? '-third' : '-next'),
      ),
    );
  });
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
      () => 'request1',
    ),
  );
  await state.load();
  await state.loadRequests(1);
  await state.loadRequests(2);
  await state.loadRounds(1);
  await state.loadRounds(2);
  await state.loadChanges(1);
  await state.loadChanges(2);
  state.reason.value = '保留审核理由';
  state.resolution.value.change1 = '保留解除理由';
  expired = true;
  await state.loadRequests(2);
  await state.loadRounds(2);
  await state.loadChanges(2);
  assert.ok(state.readingError.value);
  for (const name of ['refresh', 'refreshRounds', 'refreshChanges'])
    assert.equal(typeof state[name], 'function', `${name}应是可点击的独立恢复入口`);
  await state.refreshRounds();
  await state.refreshChanges();
  await state.refresh();
  for (const pagination of [
    state.requestPagination,
    state.roundPagination,
    state.changePagination,
  ]) {
    assert.equal(pagination.index, 0);
    assert.deepEqual(pagination.cursors, [null]);
  }
  assert.equal(state.readingError.value, '');
  assert.equal(state.fixed.value.id, 'round1');
  assert.equal(state.reason.value, '保留审核理由');
  assert.equal(state.resolution.value.change1, '保留解除理由');
  h.stop();
});

test('initial review selection locates the twenty-first active request while keeping only twenty display rows', async () => {
  const h = await history('review');
  const metadata = Array.from({ length: 21 }, (_, index) => ({
    ...request('request' + (index + 1)),
    status: index === 20 ? 'REVIEWING' : 'MERGED',
  }));
  const reads = [];
  globalThis.fetch = reviewFetch(reads, (path) => {
    if (path.includes('/review-requests?limit=20'))
      return response(page(metadata.slice(0, 20), 'display-next'));
    if (path.includes('/review-requests?limit=100')) return response(page(metadata));
    if (path.includes('request21/rounds?')) return response(page([round('round1', 'request21')]));
  });
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
    ),
  );
  await state.load();
  assert.equal(state.request.value.id, 'request21');
  assert.equal(state.requests.value.length, 20);
  assert.equal(
    state.requests.value.some((value) => value.id === 'request21'),
    false,
  );
  assert.deepEqual(
    reads.filter((path) => /\/review-requests\/[^/?]+$/.test(path)),
    ['/v1/review-requests/request21'],
  );
  assert.equal(reads.filter((path) => path.includes('/review-requests?limit=100')).length, 1);
  h.stop();
});

test('repeated default-selection metadata cursors do not pick an arbitrary first-page review', async () => {
  const h = await history('review');
  const reads = [];
  globalThis.fetch = reviewFetch(reads, (path) =>
    path.includes('/review-requests?')
      ? response(page([{ ...request(), status: 'MERGED' }], 'repeated'))
      : undefined,
  );
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
    ),
  );
  await state.load();
  assert.equal(state.request.value, null);
  assert.match(state.readingError.value, /分页.*选择/);
  assert.equal(reads.filter((path) => path.includes('/review-requests?limit=100')).length, 2);
  assert.equal(
    reads.some((path) => /\/review-requests\/[^/?]+$/.test(path)),
    false,
  );
  h.stop();
});

test('default-selection metadata budget exhaustion leaves selection explicit without reading any candidate body', async () => {
  const h = await history('review');
  const reads = [];
  let pages = 0;
  globalThis.fetch = reviewFetch(reads, (path) => {
    if (path.includes('/review-requests?limit=20'))
      return response(page([{ ...request(), status: 'MERGED' }], 'display-next'));
    if (!path.includes('/review-requests?limit=100')) return;
    pages++;
    assert.ok(pages <= 20);
    return response(
      page([{ ...request('candidate' + pages), status: 'MERGED' }], 'scan' + (pages + 1)),
    );
  });
  const state = h.start(() =>
    h.useReviews(
      () => 'doc',
      () => 'me',
      () => false,
    ),
  );
  await state.load();
  assert.equal(state.request.value, null);
  assert.match(state.readingError.value, /较多.*选择/);
  assert.equal(pages, 20);
  assert.equal(
    reads.some((path) => /\/review-requests\/[^/?]+$/.test(path)),
    false,
  );
  h.stop();
});
