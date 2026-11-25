import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import ts from 'typescript';
import { pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';
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
async function transport() {
  return import(
    moduleURL(
      readFileSync(new URL('../../web/src/lib/http.ts', import.meta.url), 'utf8').replace(
        "from 'vue'",
        "from '" + pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href + "'",
      ),
    ) +
      '#' +
      Math.random()
  );
}
const response = (body, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });

// 展示补零不能改变权限目标，尤其不能经过Number损失uint64精度。
test('account number display and input preserve canonical uint64 identities', async () => {
  const { accountNumber, accountIDFromInput, publicNumber, idFromPublicNumber, matchesAccount } =
    await import(
      moduleURL(
        readFileSync(
          new URL('../../web/src/features/account/identity.ts', import.meta.url),
          'utf8',
        ),
      )
    );
  assert.equal(accountNumber('7764'), '00007764');
  assert.equal(accountIDFromInput(' 00007764 '), '7764');
  assert.equal(accountIDFromInput('7764'), '7764');
  assert.equal(publicNumber('4483'), '00004483');
  assert.equal(idFromPublicNumber('00004483'), '4483');
  assert.equal(idFromPublicNumber('18446744073709551615'), '18446744073709551615');
  assert.equal(accountNumber('18446744073709551615'), '18446744073709551615');
  assert.equal(accountIDFromInput('18446744073709551615'), '18446744073709551615');
  for (const invalid of ['0', '00000000', '-1', '7e3', '77.64', '18446744073709551616', '１２３４'])
    assert.equal(accountIDFromInput(invalid), null);
  const member = { user_id: '7764', display_name: '小林' };
  assert.equal(matchesAccount(member, '00007764'), true);
  assert.equal(matchesAccount(member, '7764'), true);
  assert.equal(matchesAccount(member, '小林'), true);
  assert.equal(matchesAccount(member, '00007765'), false);
});

// 面向用户的提示必须说明原因和下一步；内部码、驱动消息不能穿过展示边界。
test('user error messages distinguish signup, permissions and service failures without leaking codes', async () => {
  const h = await transport();
  assert.match(
    h.explain(new h.ApiError('EMAIL_ALREADY_REGISTERED', 409)),
    /已注册.*登录.*找回密码/,
  );
  assert.match(h.explain(new h.ApiError('SIGNUP_CONFLICT', 409)), /已注册/);
  assert.match(h.explain(new h.ApiError('SERVICE_UNAVAILABLE', 503)), /暂时.*稍后/);
  assert.match(h.explain(new h.ApiError('UNKNOWN_PRIVATE_CODE', 403)), /权限/);
  assert.match(h.explain(new h.ApiError('UNKNOWN_PRIVATE_CODE', 429)), /频繁/);
  assert.equal(
    h.explain(new Error('SELECT password_hash FROM user; private-host')),
    '操作未完成，请稍后重试。',
  );
  assert.equal(h.explain(new h.UserFacingError('请先选择文档。')), '请先选择文档。');
  assert.equal(
    h.explain(new Error('private failure'), '图片暂时无法读取，请稍后重试。'),
    '图片暂时无法读取，请稍后重试。',
  );
});

// 用户反复刷新仍失败：缺少H2和两侧标题不一致均是正文前置条件，不能显示成泛化的版本冲突。
test('structure preview errors explain missing headings and draft boundaries instead of asking to refresh', async () => {
  const h = await transport();
  const missing = h.explain(new h.ApiError('STRUCTURE_LEVEL2_MISSING', 409));
  assert.match(missing, /主线缺少二级标题.*共同支线.*自动对齐/);
  assert.doesNotMatch(missing, /刷新|STRUCTURE_LEVEL2_MISSING/);
  const changed = h.explain(new h.ApiError('DRAFT_STRUCTURE_CHANGED', 409));
  assert.match(changed, /二级标题.*不一致.*审核/);
  assert.doesNotMatch(changed, /DRAFT_STRUCTURE_CHANGED/);
});

test('transport normalizes network, timeout and invalid response errors without showing upstream text', async () => {
  const h = await transport();
  globalThis.fetch = async () => {
    throw new TypeError('Failed to fetch private URL');
  };
  await assert.rejects(
    () => h.api('/v1/account/me'),
    (error) => {
      assert.equal(error.code, 'NETWORK_UNAVAILABLE');
      assert.match(h.explain(error), /网络连接/);
      return true;
    },
  );
  globalThis.fetch = async () => {
    throw new DOMException('private timeout', 'AbortError');
  };
  await assert.rejects(
    () => h.api('/v1/account/me'),
    (error) => error.code === 'REQUEST_TIMEOUT',
  );
  globalThis.fetch = async () => ({
    ok: true,
    status: 200,
    json: async () => {
      throw new DOMException('private timeout while reading body', 'AbortError');
    },
  });
  await assert.rejects(
    () => h.api('/v1/account/me'),
    (error) => error.code === 'REQUEST_TIMEOUT' && error.status === 0,
  );
  globalThis.fetch = async () =>
    new Response('<html>private proxy failure</html>', { status: 502 });
  await assert.rejects(
    () => h.api('/v1/account/me'),
    (error) => error.status === 502 && error.code === 'REQUEST_FAILED',
  );
  globalThis.fetch = async () => response(null, 200);
  await assert.rejects(
    () => h.api('/v1/account/me'),
    (error) => error.code === 'RESPONSE_INVALID',
  );
});

// Go成功仍可能等待Node安装；页面不能先显示锁/解锁或合并已经可用。
test('a committed main operation waits for real installation before reporting completion', async () => {
  const h = await transport();
  let reads = 0;
  globalThis.fetch = async (_path, options) => {
    if (options.method === 'POST')
      return response({
        operation_id: 'main-install-original',
        state: 'SUCCEEDED',
        result: { main_sync_state: 'SYNCING' },
      });
    reads++;
    return response({
      operation_id: 'main-install-original',
      state: 'SUCCEEDED',
      result: { main_sync_state: 'READY' },
    });
  };
  const value = await h.mutate('/v1/documents/7/main-lock', {
    expected_control_revision: '1',
    locked: false,
  });
  assert.equal(reads, 1);
  assert.equal(value.result.main_sync_state, 'READY');
});

// 确定冲突须保留服务器交付的原attempt入口，不能折成无上下文的通用失败。
// 结构安装推进ACL时，已登记原号的GET暂时409仍属于关闭中；改成终止错误会丢掉原操作续接。
test('known scope transition polls retain the original operation and wait for READY without another POST', async () => {
  const h = await transport();
  const calls = [];
  let reads = 0;
  globalThis.fetch = async (path, options) => {
    calls.push({ path, method: options.method });
    if (options.method === 'POST')
      return response({ operation_id: 'scope-original', state: 'PREPARING' }, 202);
    const code = ['SCOPE_TRANSITION_PENDING', 'DOCUMENT_ACCESS_CHANGED'][reads++];
    if (code) return response({ code }, 409);
    return response({
      operation_id: 'scope-original',
      state: 'SUCCEEDED',
      result: { main_sync_state: 'READY' },
    });
  };
  const value = await h.mutate('/v1/documents/7/structure-changes', {
    operation_id: 'scope-original',
  });
  assert.equal(value.result.main_sync_state, 'READY');
  assert.deepEqual(calls, [
    { path: '/v1/documents/7/structure-changes', method: 'POST' },
    { path: '/v1/operations/scope-original', method: 'GET' },
    { path: '/v1/operations/scope-original', method: 'GET' },
    { path: '/v1/operations/scope-original', method: 'GET' },
  ]);
});

// 原POST响应丢失后，后台目录任务可能正在推进ACL；这个409不能证明原操作失败。
test('an unknown write retains its fixed input through a transient scope transition', async () => {
  for (const code of ['DOCUMENT_ACCESS_CHANGED', 'SCOPE_TRANSITION_PENDING']) {
    const h = await transport();
    const inputs = [];
    globalThis.fetch = async (_path, options) => {
      inputs.push(JSON.parse(options.body));
      if (inputs.length === 1) throw new TypeError('lost original response');
      if (inputs.length === 2) return response({ code }, 409);
      return response({ operation_id: 'unknown-scope-original', state: 'SUCCEEDED' });
    };
    let pending;
    try {
      await h.write('/v1/documents/7/chapters', {
        operation_id: 'unknown-scope-original',
        title: '第二章',
        expected_structure_revision: '3',
      });
      assert.fail('the lost response must remain unknown');
    } catch (error) {
      assert.equal(typeof error.resume, 'function');
      pending = error;
    }
    try {
      await pending.resume();
      assert.fail('a transient transition cannot decide an unknown original result');
    } catch (error) {
      assert.equal(typeof error.resume, 'function');
      assert.equal(error.id, pending.id);
      pending = error;
    }
    assert.equal((await pending.resume()).state, 'SUCCEEDED');
    assert.deepEqual(inputs, Array(3).fill(inputs[0]));
  }
});

test('known first conflicts and non-transition retry conflicts remain definite failures', async () => {
  for (const code of ['DOCUMENT_ACCESS_CHANGED', 'SCOPE_TRANSITION_PENDING']) {
    const h = await transport();
    globalThis.fetch = async () => response({ code }, 409);
    await assert.rejects(
      () => h.write('/v1/documents/7/chapters', {}),
      (error) => error instanceof h.ApiError && error.code === code,
    );
  }
  const h = await transport();
  let calls = 0;
  globalThis.fetch = async () => {
    if (++calls === 1) throw new TypeError('lost response');
    return response({ code: 'REVISION_CHANGED' }, 409);
  };
  let pending;
  try {
    await h.write('/v1/documents/7/chapters', {});
  } catch (error) {
    pending = error;
  }
  assert.equal(typeof pending.resume, 'function');
  await assert.rejects(
    () => pending.resume(),
    (error) => error instanceof h.ApiError && error.code === 'REVISION_CHANGED',
  );
});

// 并非任何409或权限否定都可继续；真正版本冲突和403必须停止，不能假报安装完成。
test('a real revision conflict or denied progress is not treated as a scope transition', async () => {
  for (const [code, status] of [
    ['REVISION_CHANGED', 409],
    ['DOCUMENT_READ_NOT_ALLOWED', 403],
  ]) {
    const h = await transport();
    let reads = 0;
    globalThis.fetch = async (_path, options) => {
      if (options.method === 'POST')
        return response({ operation_id: 'denied-original', state: 'PREPARING' }, 202);
      reads++;
      return response({ code }, status);
    };
    await assert.rejects(
      () => h.mutate('/v1/documents/7/structure-changes', { operation_id: 'denied-original' }),
      (error) => error.code === code && error.status === status,
    );
    assert.equal(reads, 1);
  }
});

test('a rejected merge keeps its business code and original conflict context', async () => {
  const h = await transport();
  const value = {
    operation_id: 'merge-original',
    state: 'REJECTED',
    error: { code: 'MERGE_CONFLICT_REQUIRES_RESOLUTION' },
    result: { conflict_context_ref: 'merge-original' },
  };
  globalThis.fetch = async () => response(value);
  await assert.rejects(
    () => h.mutate('/v1/review-requests/8/approve-and-merge', { round_id: '9' }),
    (error) => {
      assert.equal(error.code, value.error.code);
      assert.deepEqual(error.operation, value);
      return true;
    },
  );
});

// 已登记之后只有进度丢响应；恢复闭包须继续GET原号，不能重新登记一次写请求。
test('an accepted operation resumes its progress query after a lost poll response', async () => {
  const h = await transport(),
    action = h.useAction();
  let writes = 0,
    reads = 0;
  globalThis.fetch = async (_path, options) => {
    if (options.method === 'POST') {
      writes++;
      return response({ operation_id: 'accepted-original', state: 'PREPARING' }, 202);
    }
    if (++reads === 1) throw Error('lost progress response');
    return response({ operation_id: 'accepted-original', state: 'SUCCEEDED' });
  };
  await action.run(() =>
    action.mutate('/v1/documents/7/main-lock', {
      operation_id: 'accepted-original',
      locked: false,
    }),
  );
  assert.equal(action.pending.value, 'accepted-original');
  await action.retry.value();
  assert.equal(writes, 1);
  assert.equal(reads, 2);
  assert.equal(action.pending.value, '');
});
// 写入已确认与刷新失败是两种事实。重试只能补刷新，不能生成第二个实体。
test('confirmed write plus failed refresh retains a refresh-only continuation', async () => {
  const h = await transport(),
    a = h.useAction();
  let writes = 0,
    reads = 0;
  globalThis.fetch = async (path, options) =>
    options.method === 'POST'
      ? (writes++,
        response({ operation_id: 'fixed', state: 'SUCCEEDED', result: { entity_id: '9' } }))
      : ++reads === 1
        ? response({ code: 'TEMPORARY' }, 503)
        : response({ items: [] });
  await a.run(async () => {
    await a.mutate('/v1/documents', { title: '首稿' });
    await h.api('/v1/documents');
  });
  assert.equal(writes, 1);
  assert.ok(a.retry.value, '刷新失败必须保留续接入口');
  await a.retry.value();
  assert.equal(writes, 1);
  assert.equal(reads, 2);
  assert.equal(a.pending.value, '');
});
// 任意刷新/导航回调不能覆盖未知写入的编号和固定参数。
test('ordinary refresh cannot replace an unknown original mutation', async () => {
  const h = await transport(),
    a = h.useAction();
  globalThis.fetch = async () => {
    throw Error('network');
  };
  await a.run(() => a.mutate('/v1/documents', { title: '固定输入' }));
  const id = a.pending.value;
  let refreshed = false;
  await a.run(async () => {
    refreshed = true;
  });
  assert.equal(refreshed, false);
  assert.equal(a.pending.value, id);
  assert.ok(a.retry.value);
});
test('a stale identity refresh cannot erase a newer login', async () => {
  const h = await transport(),
    store = new Map();
  globalThis.sessionStorage = {
    getItem: (k) => store.get(k) ?? null,
    setItem: (k, v) => store.set(k, v),
    removeItem: (k) => store.delete(k),
  };
  const source = readFileSync(
    new URL('../../web/src/features/account/session.ts', import.meta.url),
    'utf8',
  )
    .replace(
      "from 'vue'",
      "from '" + pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href + "'",
    )
    .replace(
      "from '../../lib/http'",
      "from '" +
        moduleURL(
          readFileSync(new URL('../../web/src/lib/http.ts', import.meta.url), 'utf8').replace(
            "from 'vue'",
            "from '" +
              pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href +
              "'",
          ),
        ) +
        "'",
    )
    .replace("typeof BroadcastChannel === 'undefined'", 'true');
  const s = await import(moduleURL(source) + '#' + Math.random());
  let release,
    me = 0;
  globalThis.fetch = async (path) =>
    path.endsWith('/login')
      ? response({ user_id: '2', csrf_token: 'new' })
      : ++me === 1
        ? await new Promise((r) => (release = r))
        : response({ id: '2', display_name: '新会话' });
  const old = s.refreshSession();
  await s.login('new@example.test', 'x');
  release(response({ id: '1', display_name: '旧会话' }));
  await old;
  assert.equal(s.user.value?.id, '2');
  assert.ok(store.get('collaboration-report.csrf'));
});
test('temporary identity failure preserves the owned input UI while actual denial clears identity and credentials', async () => {
  const source = readFileSync(
    new URL('../../web/src/features/account/session.ts', import.meta.url),
    'utf8',
  )
    .replace(
      "from 'vue'",
      "from '" + pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href + "'",
    )
    .replace(
      "from '../../lib/http'",
      "from '" +
        moduleURL(
          readFileSync(new URL('../../web/src/lib/http.ts', import.meta.url), 'utf8').replace(
            "from 'vue'",
            "from '" +
              pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href +
              "'",
          ),
        ) +
        "'",
    )
    .replace("typeof BroadcastChannel === 'undefined'", 'true');
  const store = new Map();
  globalThis.sessionStorage = {
    getItem: (k) => store.get(k) ?? null,
    setItem: (k, v) => store.set(k, v),
    removeItem: (k) => store.delete(k),
  };
  const s = await import(moduleURL(source) + '#' + Math.random());
  globalThis.fetch = async (path) =>
    path.endsWith('/login')
      ? response({ user_id: '3', csrf_token: 'kept' })
      : response({ id: '3', display_name: '会话' });
  await s.login('fixture@example.test', 'x');
  globalThis.fetch = async () => response({ code: 'UNAVAILABLE' }, 503);
  await s.refreshSession();
  assert.ok(store.get('collaboration-report.csrf'));
  assert.equal(s.user.value?.id, '3', '网络失败不能卸载所属账号的本机输入');
  assert.equal(s.identityUnavailable.value, true, '保留缓存身份时必须明确待核验');
  // 资格否定与503分开：实际401仍清空旧身份和凭据，不能把本机保留升级成有效权限。
  globalThis.fetch = async () => response({ code: 'SESSION_INVALID' }, 401);
  await s.refreshSession();
  assert.equal(s.user.value, null);
  assert.equal(store.get('collaboration-report.csrf'), undefined);
  assert.equal(s.identityUnavailable.value, false);
});

// 注册写入已成功、自动登录暂时失败时，续接同一标签槽且不能再创建第二个账号。
// 槽在useAction绑定代次之前准备；普通旧操作仍不能借后来登录的身份恢复。
test('confirmed signup resumes its failed login in the prepared slot without repeating signup', async () => {
  const store = new Map();
  globalThis.sessionStorage = {
    getItem: (key) => store.get(key) ?? null,
    setItem: (key, value) => store.set(key, value),
    removeItem: (key) => store.delete(key),
  };
  const vueURL = pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href;
  const httpURL =
    moduleURL(
      readFileSync(new URL('../../web/src/lib/http.ts', import.meta.url), 'utf8').replace(
        "from 'vue'",
        `from '${vueURL}'`,
      ),
    ) +
    '#' +
    Math.random();
  const h = await import(httpURL);
  const s = await import(
    moduleURL(
      readFileSync(new URL('../../web/src/features/account/session.ts', import.meta.url), 'utf8')
        .replace("from 'vue'", `from '${vueURL}'`)
        .replace("from '../../lib/http'", `from '${httpURL}'`)
        .replace("typeof BroadcastChannel === 'undefined'", 'true'),
    )
  );
  let signupCalls = 0;
  const loginContexts = [];
  globalThis.fetch = async (path, options) => {
    if (path.endsWith('/signup')) {
      signupCalls++;
      return response({ user_id: '42' });
    }
    if (path.endsWith('/login')) {
      loginContexts.push(options.headers['X-Collab-Session-Context']);
      if (loginContexts.length === 1) return response({ code: 'SERVICE_UNAVAILABLE' }, 503);
      return response({ user_id: '42', csrf_token: 'signup-session' });
    }
    return response({ id: '42', display_name: '注册续接' });
  };
  const action = h.useAction();
  const establishSession = s.prepareLogin('signup@example.test', 'fixture-password');
  const preparedContext = h.sessionContextID();
  await action.run(async () => {
    await action.write('/v1/account/signup', { operation_id: 'signup-original' });
    await establishSession();
  });
  assert.equal(action.pending.value, 'signup-original');
  assert.equal(h.actionsBlocked.value, true);
  assert.ok(action.retry.value);
  await action.retry.value();
  assert.equal(action.error.value, '');
  assert.equal(action.pending.value, '');
  assert.equal(h.actionsBlocked.value, false);
  assert.equal(signupCalls, 1);
  assert.deepEqual(loginContexts, [preparedContext, preparedContext]);
  assert.equal(s.user.value?.id, '42');
  h.rotateSessionContext();
  await assert.rejects(establishSession, (error) => error.code === 'SESSION_CHANGED');
});
