import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';
import { parse, compileScript } from '@vue/compiler-sfc';
import ts from 'typescript';

const require = createRequire(import.meta.url);
const read = (path) => readFileSync(new URL('../../web/src/' + path, import.meta.url), 'utf8');
const moduleURL = (source) =>
  'data:text/javascript;base64,' +
  Buffer.from(
    ts.transpileModule(source, {
      compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
    }).outputText,
  ).toString('base64');
const response = (items, next_cursor = null) =>
  new Response(JSON.stringify({ items, next_cursor, has_more: !!next_cursor }), {
    headers: { 'content-type': 'application/json' },
  });
const settle = () => new Promise(setImmediate);

for (const [pane, key, kind] of [
  ['invitations', 'invitations', 'invites'],
  ['applications', 'applications', 'apps'],
]) {
  test(`${pane} paging removes the old page and clears protected records on authorization loss`, async () => {
    const vueURL = pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href;
    const vue = await import(vueURL);
    const lifecycle = moduleURL(`export * from '${vueURL}'; export const onUnmounted = () => {};`);
    const http =
      moduleURL(read('lib/http.ts').replace("from 'vue'", `from '${vueURL}'`)) +
      '#' +
      Math.random();
    const identity = moduleURL(read('features/account/identity.ts'));
    const { descriptor } = parse(read('features/workspace/SharingPanel.vue'));
    const source = compileScript(descriptor, { id: 'sharing-pagination' })
      .content.replaceAll("from 'vue'", `from '${lifecycle}'`)
      .replace("from '../../lib/http'", `from '${http}'`)
      .replace("from '../account/identity'", `from '${identity}'`)
      .replace(/import (\w+) from '[^']+\.vue';/g, 'const $1 = {};');
    let release;
    globalThis.fetch = async (path) => {
      if (new URL(path, 'https://localhost').searchParams.get('cursor') === 'next')
        return new Promise((done) => {
          release = done;
        });
      return response([{ id: 'protected-record', document_id: '1', revision: '1' }], 'next');
    };
    const component = (await import(moduleURL(source))).default;
    const scope = vue.effectScope();
    const state = scope.run(() =>
      component.setup(
        { document: { id: '1', acl_revision: '1' }, entries: [], pane, reload: async () => {} },
        { expose() {}, emit() {} },
      ),
    );
    try {
      await settle();
      assert.equal(state[key].value.length, 1);
      state.turn(kind, 1);
      await settle();
      assert.deepEqual(state[key].value, [], '旧页不能在后页请求期间继续冒充本页');
      release(
        new Response(JSON.stringify({ code: 'DOCUMENT_MANAGEMENT_REQUIRED' }), {
          status: 403,
          headers: { 'content-type': 'application/json' },
        }),
      );
      await settle();
      assert.equal(state[key].value.length, 0);
      assert.equal(state.inviteNext.value, null);
      assert.equal(state.appNext.value, null);
      assert.notEqual(state.action.error.value, '');
    } finally {
      release?.(response([]));
      scope.stop();
    }
  });
}
