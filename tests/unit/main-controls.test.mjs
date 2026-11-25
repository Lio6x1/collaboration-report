import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';
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
async function loadControls() {
  const http =
    moduleURL(
      readFileSync(new URL('../../web/src/lib/http.ts', import.meta.url), 'utf8').replace(
        "from 'vue'",
        `from '${pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href}'`,
      ),
    ) +
    '#' +
    Math.random();
  const api = moduleURL(
    readFileSync(new URL('../../web/src/features/main/api.ts', import.meta.url), 'utf8'),
  );
  const source = readFileSync(
    new URL('../../web/src/features/main/useMainControls.ts', import.meta.url),
    'utf8',
  )
    .replace("'../../lib/http'", `'${http}'`)
    .replace("'./api'", `'${api}'`);
  return import(moduleURL(source));
}
const target = () => ({ id: '7', control_revision: '11', main_locked: false });
const response = (value, status = 200) =>
  new Response(JSON.stringify(value), { status, headers: { 'content-type': 'application/json' } });

// 如果动作先POST再结束本机保存，可靠切点可能固定到输入法提交之前的正文。
test('MAIN locking waits for its local reliable save before registering the control operation', async () => {
  const { useMainControls } = await loadControls();
  let finishSave;
  const saved = new Promise((resolve) => {
    finishSave = resolve;
  });
  let writes = 0;
  globalThis.fetch = async () => {
    writes++;
    return response({
      operation_id: 'lock-original',
      state: 'SUCCEEDED',
      result: { main_sync_state: 'READY' },
    });
  };
  const controls = useMainControls(
    target,
    async () => {},
    () => saved,
  );
  const work = controls.changeLock();
  try {
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.equal(writes, 0, '本机保存尚未确认，不能先登记锁定');
  } finally {
    finishSave();
    await work;
  }
  assert.equal(writes, 1);
});

// 已完成FLUSH后若控制进度丢响应，恢复必须只GET原号；重新FLUSH会受到已经变化的MAIN代次影响。
test('a pending MAIN control resumes without repeating its completed local save', async () => {
  const { useMainControls } = await loadControls();
  let saves = 0,
    writes = 0,
    reads = 0;
  globalThis.fetch = async (_path, options) => {
    if (options.method === 'POST') {
      writes++;
      return response({ operation_id: 'lock-original', state: 'PREPARING' }, 202);
    }
    if (++reads === 1) throw Error('lost progress response');
    return response({
      operation_id: 'lock-original',
      state: 'SUCCEEDED',
      result: { main_sync_state: 'READY' },
    });
  };
  const controls = useMainControls(
    target,
    async () => {},
    async () => {
      saves++;
    },
  );
  await controls.changeLock();
  assert.equal(controls.action.pending.value, 'lock-original');
  await controls.action.retry.value();
  assert.equal(saves, 1);
  assert.equal(writes, 1);
  assert.equal(reads, 2);
});
