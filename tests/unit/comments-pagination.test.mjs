import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';
import ts from 'typescript';

const require = createRequire(import.meta.url);
const read = (path) => readFileSync(new URL('../../web/src/' + path, import.meta.url), 'utf8');
const urlOf = (source) =>
  'data:text/javascript;base64,' +
  Buffer.from(
    ts.transpileModule(source, {
      compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
    }).outputText,
  ).toString('base64');
async function fixture() {
  const vueURL = pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href;
  const vue = await import(vueURL);
  const lifecycle = urlOf(
    `export * from '${vueURL}'; export { onScopeDispose as onBeforeUnmount } from '${vueURL}';`,
  );
  const http =
    urlOf(read('lib/http.ts').replace("from 'vue'", `from '${vueURL}'`)) + '#' + Math.random();
  const pagination = urlOf(read('lib/pagination.ts'));
  const api = urlOf(
    read('features/comments/api.ts').replace("from '../../lib/http'", `from '${http}'`),
  );
  const source = read('features/comments/useComments.ts')
    .replace("from 'vue'", `from '${lifecycle}'`)
    .replace("from '../../lib/http'", `from '${http}'`)
    .replace("from '../../lib/pagination'", `from '${pagination}'`)
    .replace("from './api'", `from '${api}'`);
  const { useComments } = await import(urlOf(source));
  const scope = vue.effectScope();
  let thread = {
    id: '7',
    document_id: '1',
    context_kind: 'MAIN',
    context_id: '2',
    anchor: { schema_version: 'anchor-v1', content_version_id: '3' },
    author_id: '9',
    status: 'OPEN',
    revision: '1',
  };
  const writes = [],
    reads = [];
  const response = (value, status = 200) =>
    new Response(JSON.stringify(value), {
      status,
      headers: { 'content-type': 'application/json' },
    });
  globalThis.fetch = async (path, options = {}) => {
    reads.push(path);
    const url = new URL(path, 'https://localhost');
    if (url.pathname.endsWith('/state')) {
      const input = JSON.parse(options.body);
      writes.push(input);
      if (input.expected_revision !== thread.revision)
        return response({ code: 'COMMENT_REVISION_CHANGED' }, 409);
      thread = { ...thread, status: input.state, revision: String(Number(thread.revision) + 1) };
      return response({
        operation_id: input.operation_id,
        state: 'SUCCEEDED',
        result: { entity_id: thread.id },
      });
    }
    if (url.pathname.endsWith('/comment-threads'))
      return response({
        items: thread.status === 'DELETED' ? [] : [thread],
        next_cursor: url.searchParams.has('cursor') ? 'third' : 'second',
        has_more: true,
      });
    if (url.pathname.endsWith('/comments'))
      return response({
        items: [
          {
            id: '8',
            thread_id: '7',
            author_id: '9',
            body: '已保存的原评论',
            revision: '1',
            deleted: false,
          },
        ],
        next_cursor: null,
        has_more: false,
      });
    if (url.pathname === '/v1/comment-threads/7') return response(thread);
    throw Error('UNEXPECTED_COMMENT_REQUEST');
  };
  const props = vue.reactive({
    document: '1',
    contextKind: 'MAIN',
    contextId: '2',
    version: '3',
    actor: '9',
    canComment: true,
    canManage: true,
    browseMainHistory: true,
  });
  const state = scope.run(() => useComments(props));
  await new Promise(setImmediate);
  return { state, writes, reads, stop: () => scope.stop() };
}

// 解决后若仍保留旧revision，重开就409；删除后若保留旧OPEN对象则会错误显示回复入口。
test('thread state changes refresh the selected revision while keeping its reply draft', async () => {
  const f = await fixture();
  try {
    await f.state.open(f.state.threads.value[0]);
    f.state.replyBody.value = '尚未提交的回复';
    await f.state.setState('RESOLVED');
    assert.equal(f.state.selected.value.status, 'RESOLVED');
    assert.equal(f.state.selected.value.revision, '2');
    assert.equal(f.state.replyBody.value, '尚未提交的回复');
    await f.state.setState('OPEN');
    assert.equal(f.state.selected.value.revision, '3');
    assert.equal(f.writes[1].expected_revision, '2');
    await f.state.setState('DELETED');
    assert.equal(f.state.selected.value, null);
    assert.equal(f.state.canReply.value, false);
    assert.equal(f.state.threads.value.length, 0);
    assert.equal(f.state.action.error.value, '');
  } finally {
    f.stop();
  }
});

test('explicit comment refresh returns to the first page after old cursors expire', async () => {
  const f = await fixture();
  try {
    await f.state.turnThreads(1);
    await f.state.turnThreads(2);
    await f.state.refresh();
    assert.equal(f.state.threadPage.index, 0);
    const request = new URL(f.reads.at(-1), 'https://localhost');
    assert.equal(request.pathname, '/v1/documents/1/comment-threads');
    assert.equal(request.searchParams.has('cursor'), false);
  } finally {
    f.stop();
  }
});
