import test from 'node:test';
import assert from 'node:assert/strict';
import {
  browserSessionContext,
  scopedResponseHeaders,
} from '../../scripts/browser-session-context.mjs';

const a = 'a'.repeat(32),
  b = 'b'.repeat(32);
const cookies = `__Host-collabSession=legacy; __Host-collabSession.${a}=session-a; __Host-collabCSRF.${a}=csrf-a; __Host-collabRecovery.${a}=recovery-a; __Host-collabSession.${b}=session-b; __Host-collabCSRF.${b}=csrf-b`;
const url = (path = '/v1/account/me') => new URL(path, 'https://localhost:20609');

// 选择器只选择浏览器已有的HttpOnly Cookie；缺少匹配不能退回其他账号。
test('a tab forwards only its selected session and paired CSRF/recovery cookies', () => {
  const selected = browserSessionContext(
    { cookie: cookies, 'x-collab-session-context': a },
    url(),
    'GET',
  );
  assert.equal(
    selected.cookie,
    '__Host-collabSession=session-a; __Host-collabCSRF=csrf-a; __Host-collabRecovery=recovery-a',
  );
  assert.equal(
    browserSessionContext({ cookie: cookies, 'x-collab-session-context': b }, url(), 'GET').cookie,
    '__Host-collabSession=session-b; __Host-collabCSRF=csrf-b',
  );
  assert.equal(
    browserSessionContext(
      { cookie: cookies, 'x-collab-session-context': 'c'.repeat(32) },
      url(),
      'GET',
    ).cookie,
    undefined,
  );
  assert.equal(
    browserSessionContext({ cookie: cookies }, url(), 'GET').cookie,
    '__Host-collabSession=legacy',
  );
});

test('binary resource and download URLs select a tab without exposing session secrets', () => {
  const target = url(
    `/v1/resources/7/content?document_id=8&context_kind=MAIN&context_id=9&session_context=${a}`,
  );
  assert.equal(browserSessionContext({ cookie: cookies }, target, 'GET').context, a);
  assert.equal(target.searchParams.has('session_context'), false);
  assert.equal(target.searchParams.get('context_id'), '9');
  assert.throws(() => browserSessionContext({}, url(`/v1/account/me?session_context=${a}`), 'GET'));
  assert.throws(() =>
    browserSessionContext(
      { 'x-collab-session-context': b },
      url(`/v1/export-jobs/7/download?session_context=${a}`),
      'GET',
    ),
  );
  assert.throws(() =>
    browserSessionContext({ 'x-collab-session-context': '../invalid' }, url(), 'GET'),
  );
  assert.throws(() =>
    browserSessionContext(
      {
        cookie: `__Host-collabSession.${a}=one; __Host-collabSession.${a}=two`,
        'x-collab-session-context': a,
      },
      url(),
      'GET',
    ),
  );
});

test('login and logout rewrite only selected cookie names and keep original security attributes', () => {
  const source = {
    'set-cookie': [
      '__Host-collabSession=secret; Path=/; Secure; HttpOnly; SameSite=Strict',
      '__Host-collabCSRF=proof; Path=/; Secure; SameSite=Strict',
      '__Host-collabRecovery=; Path=/; Secure; HttpOnly; Max-Age=0',
    ],
    'cache-control': 'no-store',
  };
  const result = scopedResponseHeaders(source, a);
  assert.match(result['set-cookie'][0], new RegExp(`^__Host-collabSession\\.${a}=secret;`));
  assert.ok(result['set-cookie'][0].endsWith('Secure; HttpOnly; SameSite=Strict'));
  assert.match(result['set-cookie'][2], /HttpOnly; Max-Age=0$/);
  assert.equal(source['set-cookie'][0].startsWith('__Host-collabSession='), true);
  assert.equal(scopedResponseHeaders(source, null), source);
});

// 关闭标签不会立即删除浏览器Cookie；限制新增会话，但不能把已有账号的正常请求锁死。
test('too many closed-tab sessions block a new login without blocking existing slots', () => {
  const full = Array.from(
    { length: 16 },
    (_, index) => `__Host-collabSession.${index.toString(16).padStart(32, '0')}=held`,
  ).join('; ');
  assert.throws(
    () =>
      browserSessionContext(
        { cookie: full, 'x-collab-session-context': a },
        url('/v1/account/login'),
        'POST',
      ),
    /BROWSER_SESSION_LIMIT/,
  );
  const selected = '0'.repeat(32);
  assert.equal(
    browserSessionContext({ cookie: full, 'x-collab-session-context': selected }, url(), 'GET')
      .cookie,
    '__Host-collabSession=held',
  );
  assert.equal(
    browserSessionContext(
      { cookie: full, 'x-collab-session-context': selected },
      url('/v1/account/login'),
      'POST',
    ).context,
    selected,
  );
});
