import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';
import { parse, compileScript } from '@vue/compiler-sfc';
import ts from 'typescript';

const require = createRequire(import.meta.url);
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
async function componentPanel(name, input = {}) {
  // 编译真实setup并保留Vue响应性与HTTP边界；只隔离DOM生命周期和子组件显示。
  const vueURL = pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href;
  const vue = await import(vueURL);
  const lifecycle = moduleURL(
    `export * from '${vueURL}'; export const onMounted = () => {}; export const onUnmounted = () => {};`,
  );
  const read = (path) => readFileSync(new URL('../../web/src/' + path, import.meta.url), 'utf8');
  const httpURL =
    moduleURL(read('lib/http.ts').replace("from 'vue'", `from '${vueURL}'`)) + '#' + Math.random();
  const pagination = moduleURL(read('lib/pagination.ts'));
  const rules = moduleURL(read('features/search/rules.ts'));
  const navigation = moduleURL(
    read('features/workspace/navigation.ts').replace("from '../search/rules'", `from '${rules}'`),
  );
  const identity = moduleURL(read('features/account/identity.ts'));
  const { descriptor } = parse(read('features/workspace/' + name + '.vue'));
  const source = compileScript(descriptor, { id: 'workspace-pagination-' + name })
    .content.replaceAll("from 'vue'", `from '${lifecycle}'`)
    .replace("from '../../lib/http'", `from '${httpURL}'`)
    .replace("from '../../lib/pagination'", `from '${pagination}'`)
    .replace("from './navigation'", `from '${navigation}'`)
    .replace("from '../account/identity'", `from '${identity}'`)
    .replace(/import (\w+) from '[^']+\.vue';/g, 'const $1 = {};');
  globalThis.location = { pathname: '/', search: '' };
  globalThis.history = { pushState() {}, replaceState() {} };
  globalThis.window = { addEventListener() {}, removeEventListener() {} };
  const component = (await import(moduleURL(source))).default;
  const props = vue.reactive(input);
  const panel = component.setup(props, { expose() {}, emit() {} });
  return { panel, props, vue, http: await import(httpURL) };
}
const response = (value, actions) =>
  new Response(JSON.stringify(value), {
    headers: {
      'content-type': 'application/json',
      ...(actions ? { 'X-Available-Actions': JSON.stringify(actions) } : {}),
    },
  });
const page = (items, next_cursor = null) => ({ items, next_cursor, has_more: !!next_cursor });
const library = (id, name = '知识库' + id, space_id = '7') => ({
  id: String(id),
  space_id,
  name,
  lifecycle: 'ACTIVE',
  acl_revision: '1',
  visibility_policy: 'COLLABORATORS',
  visibility_revision: '1',
});
const member = (id, display_name = '成员' + id) => ({
  user_id: String(id),
  display_name,
  role: 'MEMBER',
  state: 'ACTIVE',
  revision: '1',
});
const application = (id) => ({
  id: String(id),
  document_id: '8',
  invitation_id: '9',
  applicant_id: '10',
  invitation_validated_at: '2026-10-13T00:00:00Z',
  requested_grants: [],
  state: 'PENDING',
  reason: '协作申请',
  revision: '1',
});
function deferred() {
  let resolve;
  const promise = new Promise((done) => (resolve = done));
  return { promise, resolve };
}

// 捕捉续页追加、回页只读缓存和游标没有随服务端重读更新三种目录错误。
test('knowledge-base pages replace the grid and previous pages are reauthorized over HTTP', async () => {
  const { panel } = await componentPanel('WorkspacePanel');
  panel.spaceId.value = '7';
  const requests = [];
  globalThis.fetch = async (path) => {
    requests.push(path);
    return response(
      path.includes('cursor=second')
        ? page([library('13')])
        : page(
            Array.from({ length: 12 }, (_, i) =>
              library(i + 1, requests.length > 1 ? '已重读' : undefined),
            ),
            requests.length > 1 ? 'renewed' : 'second',
          ),
    );
  };
  await panel.listKBs();
  assert.equal(panel.kbs.value.length, 12);
  await panel.listKBs(1);
  assert.deepEqual(
    panel.kbs.value.map((item) => item.id),
    ['13'],
  );
  assert.equal(panel.kbPage.index, 1);
  await panel.listKBs(0);
  assert.equal(panel.kbs.value[0].name, '已重读');
  assert.equal(panel.kbPage.next, 'renewed');
  assert.deepEqual(requests, [
    '/v1/spaces/7/knowledge-bases?limit=12',
    '/v1/spaces/7/knowledge-bases?limit=12&cursor=second',
    '/v1/spaces/7/knowledge-bases?limit=12',
  ]);
});

test('document pages replace the selected knowledge-base directory', async () => {
  const { panel } = await componentPanel('WorkspacePanel');
  panel.spaceId.value = '7';
  panel.kbId.value = '8';
  const requests = [];
  globalThis.fetch = async (path) => {
    requests.push(path);
    return response(
      page([{ id: path.includes('cursor=second') ? '21' : '1', title: '报告' }], 'second'),
    );
  };
  await panel.listDocs();
  await panel.listDocs(1);
  assert.deepEqual(
    panel.docs.value.map((item) => item.id),
    ['21'],
  );
  await panel.listDocs(0);
  assert.deepEqual(
    panel.docs.value.map((item) => item.id),
    ['1'],
  );
  assert.deepEqual(requests, [
    '/v1/knowledge-bases/8/documents?limit=20',
    '/v1/knowledge-bases/8/documents?limit=20&cursor=second',
    '/v1/knowledge-bases/8/documents?limit=20',
  ]);
});

test('personal application pages replace records and reread previous pages', async () => {
  const { panel } = await componentPanel('WorkspacePanel');
  const requests = [];
  globalThis.fetch = async (path) => {
    requests.push(path);
    return response(page([application(path.includes('cursor=second') ? '21' : '1')], 'second'));
  };
  await panel.listOwnApps();
  await panel.listOwnApps(1);
  assert.deepEqual(
    panel.ownApps.value.map((item) => item.id),
    ['21'],
  );
  await panel.listOwnApps(0);
  assert.deepEqual(
    panel.ownApps.value.map((item) => item.id),
    ['1'],
  );
  assert.deepEqual(requests, [
    '/v1/document-applications?limit=20',
    '/v1/document-applications?limit=20&cursor=second',
    '/v1/document-applications?limit=20',
  ]);
});

// 目标原本在后页，搜索必须交给全目录接口；不能仅过滤已加载的一页。
test('knowledge-base search resets pagination and queries the complete authorized directory', async () => {
  const { panel } = await componentPanel('WorkspacePanel');
  panel.spaceId.value = '7';
  const requests = [];
  globalThis.fetch = async (path) => {
    requests.push(path);
    return response(
      path.includes('&q=') ? page([library('99', '后页库')]) : page([library('1')], 'second'),
    );
  };
  await panel.listKBs();
  await panel.listKBs(1);
  panel.kbQuery.value = '  后页库  ';
  await panel.searchKBs();
  assert.deepEqual(
    panel.kbs.value.map((item) => item.id),
    ['99'],
  );
  assert.equal(panel.kbPage.index, 0);
  assert.equal(panel.kbPage.next, null);
  assert.equal(
    requests.at(-1),
    '/v1/spaces/7/knowledge-bases?limit=12&q=%E5%90%8E%E9%A1%B5%E5%BA%93',
  );
  assert.equal(requests.length, 3, '一次搜索只读一个服务端结果页');
});

test('a superseded knowledge-base response cannot overwrite new search results', async () => {
  const { panel } = await componentPanel('WorkspacePanel');
  panel.spaceId.value = '7';
  const old = deferred();
  globalThis.fetch = (path) =>
    path.includes('&q=') ? Promise.resolve(response(page([library('99', '新结果')]))) : old.promise;
  const reading = panel.listKBs();
  panel.kbQuery.value = '新结果';
  await panel.searchKBs();
  old.resolve(response(page([library('1', '旧结果')], 'stale')));
  await reading;
  assert.deepEqual(
    panel.kbs.value.map((item) => item.id),
    ['99'],
  );
  assert.equal(panel.kbPage.next, null);
});

test('changing spaces clears application and document pages and rejects their late responses', async () => {
  const { panel } = await componentPanel('WorkspacePanel');
  panel.spaceId.value = '7';
  const oldDocs = deferred(),
    oldApps = deferred();
  globalThis.fetch = (path) =>
    path.includes('/documents?')
      ? oldDocs.promise
      : path.includes('/document-applications?')
        ? oldApps.promise
        : Promise.resolve(response(page([library('80', '新空间', '8')])));
  const docs = panel.listDocs(),
    apps = panel.listOwnApps();
  panel.spaceId.value = '8';
  await panel.changeSpace();
  oldDocs.resolve(response(page([{ id: '1', title: '旧文档' }], 'stale-docs')));
  oldApps.resolve(response(page([application('1')], 'stale-apps')));
  await Promise.all([docs, apps]);
  assert.deepEqual(panel.docs.value, []);
  assert.deepEqual(panel.ownApps.value, []);
  assert.equal(panel.docPage.next, null);
  assert.equal(panel.appPage.next, null);
  assert.equal(panel.kbs.value[0].space_id, '8');
});

test('restoring a later-page knowledge base keeps its selection outside the current card page', async () => {
  const { panel } = await componentPanel('WorkspacePanel');
  panel.spaceId.value = '7';
  panel.kbId.value = '99';
  panel.section.value = 'documents';
  globalThis.fetch = async (path) => {
    if (path === '/v1/me/spaces?limit=20')
      return response(page([{ id: '7', kind: 'PERSONAL', actions: ['CREATE_DOCUMENT'] }]));
    if (path === '/v1/knowledge-bases/99')
      return response(library('99', '后页当前库'), ['CREATE_DOCUMENT']);
    if (path.includes('/knowledge-bases?')) return response(page([library('1')], 'second'));
    return response(page([]));
  };
  await panel.refresh();
  assert.equal(panel.kbId.value, '99');
  assert.equal(panel.currentKB.value.id, '99');
  assert.equal(panel.canCreateDoc.value, true);
  assert.deepEqual(
    panel.kbs.value.map((item) => item.id),
    ['1'],
  );
  assert.equal(panel.kbPage.next, 'second');
});

async function loadedKnowledgeBase() {
  const context = await componentPanel('KnowledgeBasePanel', { id: '8', organization: '5' });
  globalThis.fetch = async (path) => {
    if (path === '/v1/knowledge-bases/8')
      return response(library('8'), ['VIEW_MEMBERS', 'CHANGE_CREATION_GRANTS']);
    if (path.includes('/organizations/'))
      return response(page([member('999', '可选组织成员')], 'candidate-next'));
    return response(page([member('1')], 'second'));
  };
  await context.panel.load();
  return context;
}

test('member paging and nickname or account search use independent authorized result pages', async () => {
  const { panel } = await loadedKnowledgeBase();
  const requests = [];
  globalThis.fetch = async (path) => {
    requests.push(path);
    return response(
      page([member(path.includes('&q=') ? '42' : path.includes('cursor=') ? '21' : '1')], 'second'),
    );
  };
  await panel.memberPage(1);
  assert.deepEqual(
    panel.members.value.map((item) => item.user_id),
    ['21'],
  );
  await panel.memberPage(0);
  panel.memberQuery.value = ' 00000042 ';
  await panel.searchMembers();
  assert.deepEqual(
    panel.members.value.map((item) => item.user_id),
    ['42'],
  );
  assert.equal(panel.memberPagination.index, 0);
  assert.deepEqual(
    panel.candidates.value.map((item) => item.user_id),
    ['999'],
  );
  assert.equal(panel.candidateCursor.value, 'candidate-next');
  assert.deepEqual(requests, [
    '/v1/knowledge-bases/8/members?limit=20&cursor=second',
    '/v1/knowledge-bases/8/members?limit=20',
    '/v1/knowledge-bases/8/members?limit=20&q=00000042',
  ]);
});

test('late member pages cannot replace a submitted member search', async () => {
  const { panel } = await loadedKnowledgeBase();
  const old = deferred();
  globalThis.fetch = (path) =>
    path.includes('&q=') ? Promise.resolve(response(page([member('42')]))) : old.promise;
  const reading = panel.memberPage(1);
  panel.memberQuery.value = '42';
  await panel.searchMembers();
  old.resolve(response(page([member('21')], 'stale')));
  await reading;
  assert.deepEqual(
    panel.members.value.map((item) => item.user_id),
    ['42'],
  );
  assert.equal(panel.memberPagination.index, 0);
  assert.equal(panel.memberPagination.next, null);
});

test('revoked member-page authorization clears management data and candidate targets', async () => {
  const { panel } = await loadedKnowledgeBase();
  panel.target.value = '999';
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ code: 'FORBIDDEN' }), { status: 403 });
  await assert.rejects(panel.memberPage(1));
  assert.equal(panel.kb.value, null);
  assert.deepEqual(panel.members.value, []);
  assert.deepEqual(panel.actions.value, []);
  assert.deepEqual(panel.candidates.value, []);
  assert.equal(panel.target.value, '');
});

test('account changes reset visible workspace pages before an old response completes', async () => {
  const { panel, http, vue } = await componentPanel('WorkspacePanel');
  panel.spaceId.value = '7';
  panel.ownApps.value = [application('1')];
  const old = deferred();
  globalThis.fetch = () => old.promise;
  const reading = panel.listKBs();
  http.setTransportToken('new-account');
  await vue.nextTick();
  old.resolve(response(page([library('1')], 'old-account')));
  await reading;
  assert.deepEqual(panel.kbs.value, []);
  assert.deepEqual(panel.ownApps.value, []);
  assert.equal(panel.kbPage.next, null);
});
