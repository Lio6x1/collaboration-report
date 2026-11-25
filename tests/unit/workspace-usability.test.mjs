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
async function workspace() {
  const vue = pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href;
  const lifecycle = moduleURL(
    `export * from '${vue}'; export const onMounted = () => {}; export const onUnmounted = () => {};`,
  );
  const http =
    moduleURL(
      readFileSync(new URL('../../web/src/lib/http.ts', import.meta.url), 'utf8').replace(
        "from 'vue'",
        `from '${vue}'`,
      ),
    ) +
    '#' +
    Math.random();
  const file = readFileSync(
    new URL('../../web/src/features/workspace/WorkspacePanel.vue', import.meta.url),
    'utf8',
  );
  const { descriptor } = parse(file);
  const rules = moduleURL(
    readFileSync(new URL('../../web/src/features/search/rules.ts', import.meta.url), 'utf8'),
  );
  const navigation = moduleURL(
    readFileSync(
      new URL('../../web/src/features/workspace/navigation.ts', import.meta.url),
      'utf8',
    ).replace("from '../search/rules'", `from '${rules}'`),
  );
  const pagination = moduleURL(
    readFileSync(new URL('../../web/src/lib/pagination.ts', import.meta.url), 'utf8'),
  );
  const source = compileScript(descriptor, { id: 'workspace-usability' })
    .content.replaceAll("from 'vue'", `from '${lifecycle}'`)
    .replace("'../../lib/http'", `'${http}'`)
    .replace("'../../lib/pagination'", `'${pagination}'`)
    .replace(
      "import PaginationControls from '../../components/PaginationControls.vue';",
      'const PaginationControls = {};',
    )
    .replace(
      "import ActionStatus from '../../components/ActionStatus.vue';",
      'const ActionStatus = {};',
    )
    .replace("import AppIcon from '../../components/AppIcon.vue';", 'const AppIcon = {};')
    .replace("import AppDialog from '../../components/AppDialog.vue';", 'const AppDialog = {};')
    .replace("import AppPopover from '../../components/AppPopover.vue';", 'const AppPopover = {};')
    .replace("from './navigation'", `from '${navigation}'`);
  globalThis.location = { pathname: '/', search: '' };
  const component = (await import(moduleURL(source))).default;
  const events = [];
  return Object.assign(component.setup({}, { expose() {}, emit: (...args) => events.push(args) }), {
    events,
  });
}
const response = (value) =>
  new Response(JSON.stringify(value), { headers: { 'content-type': 'application/json' } });

// 入口必须完成真正的归属读取再打开；仅刷新首页目录会让新文档在分页后不可发现。
test('creating a document in an existing knowledge base verifies its persisted destination and opens it', async () => {
  const panel = await workspace();
  panel.spaceId.value = '7';
  panel.kbId.value = '8';
  panel.currentKB.value = { id: '8', space_id: '7', name: '已有知识库' };
  panel.kbActions.value = ['CREATE_DOCUMENT'];
  panel.docTitle.value = '已有库的新报告';
  const reads = [];
  globalThis.fetch = async (path, options) => {
    if (options.method === 'POST') {
      const input = JSON.parse(options.body);
      assert.equal(input.space_id, '7');
      assert.equal(input.knowledge_base_id, '8');
      return response({
        operation_id: input.operation_id,
        state: 'SUCCEEDED',
        result: { entity_id: '9' },
      });
    }
    reads.push(path);
    return response(
      path === '/v1/documents/9'
        ? { id: '9', space_id: '7', knowledge_base_id: '8', lifecycle: 'ACTIVE' }
        : { items: [], next_cursor: null },
    );
  };
  await panel.createDoc();
  assert.ok(reads.includes('/v1/documents/9'), '创建后必须通过文档元数据确认实际知识库归属');
  assert.deepEqual(panel.events, [['open', '9']], '新文档不在目录首页时仍应直接打开');
  assert.equal(panel.action.error.value, '');
});

// 服务端归属异常不能以“已初始化”覆盖；这个反例保护入口与实际模型一致。
test('an unexpected persisted knowledge base prevents the new-document success state', async () => {
  const panel = await workspace();
  panel.spaceId.value = '7';
  panel.kbId.value = '8';
  panel.currentKB.value = { id: '8', space_id: '7', name: '已有知识库' };
  panel.kbActions.value = ['CREATE_DOCUMENT'];
  panel.docTitle.value = '归属反例';
  globalThis.fetch = async (path, options) =>
    options.method === 'POST'
      ? response({
          operation_id: JSON.parse(options.body).operation_id,
          state: 'SUCCEEDED',
          result: { entity_id: '9' },
        })
      : response(
          path === '/v1/documents/9'
            ? { id: '9', space_id: '7', knowledge_base_id: '10', lifecycle: 'ACTIVE' }
            : { items: [], next_cursor: null },
        );
  await panel.createDoc();
  assert.deepEqual(panel.events, []);
  assert.match(panel.action.error.value, /归属/);
  assert.equal(panel.action.notice.value.includes('已初始化'), false);
});

// 个人空间的创建能力不能替代已选知识库的服务端动作摘要，也不能借旧空间的库快照创建。
test('a selected knowledge base requires its own creation action and matching current space', async () => {
  const panel = await workspace();
  panel.spaces.value = [{ id: '7', kind: 'PERSONAL', actions: ['CREATE_DOCUMENT'] }];
  panel.spaceId.value = '7';
  panel.kbId.value = '8';
  panel.currentKB.value = { id: '8', space_id: '7' };
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    throw Error('不应登记创建');
  };
  await panel.createDoc();
  assert.equal(calls, 0, '已选库没有CREATE_DOCUMENT摘要时应关闭创建入口');
  panel.kbActions.value = ['CREATE_DOCUMENT'];
  panel.currentKB.value = { id: '8', space_id: '10' };
  await panel.createDoc();
  assert.equal(calls, 0, '旧空间的知识库快照不能用于当前空间');
  panel.currentKB.value = { id: '11', space_id: '7' };
  await panel.createDoc();
  assert.equal(calls, 0, '同一空间的另一个知识库动作不能用于当前已选库');
});
