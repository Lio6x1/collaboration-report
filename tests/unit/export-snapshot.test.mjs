import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';
import ts from 'typescript';
import { createRenderer, h, nextTick } from 'vue';

const require = createRequire(import.meta.url);
const moduleURL = (value) => 'data:text/javascript;base64,' + Buffer.from(value).toString('base64');
const vueURL = pathToFileURL(require.resolve('vue')).href;
const httpURL = moduleURL(`import { ref } from '${vueURL}';
export class ApiError extends Error { constructor(code,status){super(code);this.code=code;this.status=status} }
export class UserFacingError extends Error {}
export const actionsBlocked=ref(false), transportEpoch=ref(0), explain=e=>e.message;
export const api=(...args)=>globalThis.exportTestTransport(...args);
export function useAction(){const a={busy:ref(false),pending:ref(''),error:ref(''),notice:ref('')};a.write=(path,input)=>api(path,'POST',input);a.run=async(task,notice='')=>{a.busy.value=true;a.error.value='';try{await task();a.notice.value=notice}catch(e){a.error.value=explain(e)}finally{a.busy.value=false}};return a;}
`);
const sessionURL = moduleURL(
  `import { ref } from '${vueURL}'; export const user=ref({id:'fixture-actor'});`,
);
const session = await import(sessionURL);
const source = readFileSync(
  new URL('../../web/src/features/exports/useExport.ts', import.meta.url),
  'utf8',
)
  .replace("from 'vue'", `from '${pathToFileURL(require.resolve('vue')).href}'`)
  .replace("from '../../lib/http'", `from '${httpURL}'`)
  .replace("from '../account/session'", `from '${sessionURL}'`);
const { useExport } = await import(
  'data:text/javascript;base64,' +
    Buffer.from(
      ts.transpileModule(source, {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
      }).outputText,
    ).toString('base64')
);
const renderer = createRenderer({
  createElement: () => ({}),
  createText: () => ({}),
  createComment: () => ({}),
  insert() {},
  remove() {},
  setText() {},
  setElementText() {},
  patchProp() {},
  parentNode: () => null,
  nextSibling: () => null,
});
function mount(freshHash = 'same-body') {
  const requests = [],
    stored = new Map();
  globalThis.sessionStorage = {
    getItem: (key) => stored.get(key) ?? null,
    setItem: (key, value) => stored.set(key, value),
    removeItem: (key) => stored.delete(key),
  };
  const old = {
    document: { id: '1', title: '报告' },
    manifest_id: '100',
    structure_id: '10',
    style_id: '20',
    units: [{ unit_id: '3', content_version_id: '30' }],
  };
  const fresh = {
    ...old,
    manifest_id: '101',
    read_view_id: 'new-read-view',
    units: [{ unit_id: '3', content_version_id: '31' }],
  };
  globalThis.exportTestTransport = async (path, method, input) => {
    requests.push({ path, method, input });
    if (path.endsWith('/main')) return fresh;
    if (path.includes('/read-views/')) return { id: '31', body_hash: freshHash };
    if (path.endsWith('/exports')) {
      if (input.expected_manifest_id !== '101') throw Error('EXPORT_MAIN_VIEW_EXPIRED');
      return {
        id: '9',
        document_id: '1',
        manifest_id: '101',
        format: input.format,
        state: 'SUCCEEDED',
      };
    }
    if (path === '/v1/export-jobs/9')
      return { id: '9', document_id: '1', manifest_id: '101', format: 'DOCX', state: 'SUCCEEDED' };
    throw Error('Unexpected export request: ' + path);
  };
  let control;
  const app = renderer.createApp({
    setup() {
      control = useExport(() => ({
        document: '1',
        target: 'MAIN',
        reference: '100',
        mainView: old,
        mainBodies: { 3: { id: '30', body_hash: 'same-body' } },
      }));
      return () => h('div');
    },
  });
  app.mount({});
  return { control, requests, close: () => app.unmount() };
}

// 未编辑也会让旧阅读准入自然过期；新快照编号不同不等于正文改变。
test('export renews an unchanged main snapshot before POST without a full-page reload', async () => {
  const f = mount();
  try {
    await f.control.create();
    await nextTick();
    assert.equal(f.control.action.error.value, '');
    assert.equal(f.control.job.value?.state, 'SUCCEEDED');
    assert.equal(f.requests.filter((r) => r.path.endsWith('/exports')).length, 1);
    assert.equal(
      f.requests.find((r) => r.path.endsWith('/exports')).input.expected_manifest_id,
      '101',
    );
  } finally {
    f.close();
  }
});

// 不把新内容静默替换为用户之前阅读的版本，也不能在确认之前登记持久任务。
test('changed main content requires the reader to review it and creates no export job', async () => {
  const f = mount('different-body');
  try {
    await f.control.create();
    assert.match(f.control.action.error.value, /主线.*变化/);
    assert.equal(f.requests.filter((r) => r.path.endsWith('/exports')).length, 0);
  } finally {
    f.close();
  }
});

// 应用会定期更新user对象；账号编号和会话代次都未变化时，不能丢掉在途导出结果。
test('refreshing the same account during export preserves its confirmed job', async () => {
  const f = mount();
  const transport = globalThis.exportTestTransport;
  globalThis.exportTestTransport = async (...args) => {
    if (args[0].endsWith('/exports')) {
      session.user.value = { id: 'fixture-actor' };
      await nextTick();
    }
    return transport(...args);
  };
  try {
    await f.control.create();
    await nextTick();
    assert.equal(f.control.job.value?.state, 'SUCCEEDED');
  } finally {
    f.close();
  }
});
