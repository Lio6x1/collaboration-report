import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createHash, randomUUID } from 'node:crypto';
import mysql from 'mysql2/promise';
import { request } from '../helpers/https.mjs';
import { state, runtime } from '../../scripts/runtime-lib.mjs';
import { redisCommand } from '../../scripts/redis-client.mjs';
import { createUser } from '../helpers/account.mjs';
// 测试身份走真实SMTP注册，不依赖上次学习环境遗留的seed文件。
const fixture = await createUser(),
  env = state(),
  origin = 'https://localhost:' + env.ports.edge,
  seed = { ...fixture, user_id: fixture.id };
const subject = createHash('sha256').update(seed.email).digest('hex');
test.beforeEach(async () => {
  await redisCommand(env.ports.redis, 'bootstrap_admin', env.passwords.redisAdmin, [
    'DEL',
    'acct:login:global',
    'acct:login:email:' + subject,
  ]);
});
function post(path, input, headers = {}) {
  return request(env.ports.edge, path, {
    method: 'POST',
    headers: { origin, 'content-type': 'application/json', ...headers },
    body: JSON.stringify(input),
  });
}
function jar(response) {
  return (response.headers['set-cookie'] ?? []).map((s) => s.split(';')[0]).join('; ');
}
async function login() {
  const r = await post('/v1/account/login', { email: seed.email, password: seed.password });
  assert.equal(r.status, 200);
  return { response: r, cookie: jar(r), value: JSON.parse(r.body) };
}
async function database() {
  return mysql.createConnection({
    host: '127.0.0.1',
    port: env.ports.mysql,
    user: 'collab_account',
    password: env.passwords.account,
    database: 'account_db',
  });
}
test('login rejects a cross-origin browser request before password processing', async () => {
  const r = await post(
    '/v1/account/login',
    { email: seed.email, password: 'abc' },
    { origin: 'https://evil.invalid' },
  );
  assert.equal(r.status, 403);
  assert.equal(JSON.parse(r.body).code, 'ORIGIN_FORBIDDEN');
});
test('account me rejects forged identity headers without a real session', async () => {
  const r = await request(env.ports.edge, '/v1/account/me', {
    headers: { 'x-user-id': '1', 'x-actor-id': '1' },
  });
  assert.equal(r.status, 401);
});
test('login creates a secure cookie, real me response and only hashes in storage', async () => {
  const s = await login(),
    cookie = s.response.headers['set-cookie'].find((x) => x.startsWith('__Host-collabSession='));
  assert.match(cookie, /HttpOnly/);
  assert.match(cookie, /Secure/);
  assert.match(cookie, /SameSite=Strict/);
  const me = await request(env.ports.edge, '/v1/account/me', { headers: { cookie: s.cookie } });
  assert.equal(me.status, 200);
  assert.equal(JSON.parse(me.body).id, seed.user_id);
  const raw = cookie.split(';')[0].split('=')[1],
    hash = createHash('sha256').update(Buffer.from(raw, 'base64url')).digest('hex');
  const db = await database();
  try {
    const [[row]] = await db.query('SELECT COUNT(*) AS n FROM session WHERE id=UNHEX(?)', [hash]);
    assert.equal(row.n, 1);
    const [[plain]] = await db.query('SELECT COUNT(*) AS n FROM session WHERE id=?', [
      Buffer.from(raw, 'base64url'),
    ]);
    assert.equal(plain.n, 0);
  } finally {
    await db.end();
  }
});
test('login rejects 75 UTF8 password bytes and invalid credentials', async () => {
  assert.equal(
    (await post('/v1/account/login', { email: seed.email, password: '中'.repeat(25) })).status,
    400,
  );
  assert.equal(
    (await post('/v1/account/login', { email: seed.email, password: 'wrong-password' })).status,
    401,
  );
});
test('logout without correct CSRF cannot close the real session', async () => {
  const s = await login();
  const r = await post(
    '/v1/account/logout',
    { operation_id: 'logout-' + randomUUID() },
    { cookie: s.cookie, 'x-csrf-token': 'wrong' },
  );
  assert.equal(r.status, 403);
  assert.equal(
    (await request(env.ports.edge, '/v1/account/me', { headers: { cookie: s.cookie } })).status,
    200,
  );
});
test('logout durably closes fixed participants and same-number retry returns its fact', async () => {
  const s = await login(),
    id = 'logout-' + randomUUID();
  const headers = { cookie: s.cookie, 'x-csrf-token': s.value.csrf_token };
  const first = await post('/v1/account/logout', { operation_id: id }, headers);
  assert.equal(first.status, 202);
  let terminal;
  for (let n = 0; n < 80; n++) {
    const r = await request(env.ports.edge, '/v1/account/operations/' + id, {
      headers: { cookie: jar(first) || s.cookie },
    });
    assert.equal(r.status, 200);
    terminal = JSON.parse(r.body);
    if (terminal.state === 'SUCCEEDED') break;
    await new Promise((r) => setTimeout(r, 100));
  }
  assert.equal(terminal.state, 'SUCCEEDED');
  assert.equal(
    (await request(env.ports.edge, '/v1/account/me', { headers: { cookie: s.cookie } })).status,
    401,
  );
  // 模拟第一次响应丢失：仍持有旧cookie与CSRF，只能重取原退出事实，不能创建新退出。
  assert.equal((await post('/v1/account/logout', { operation_id: id }, headers)).status, 202);
  assert.equal(
    (await post('/v1/account/logout', { operation_id: 'logout-' + randomUUID() }, headers)).status,
    401,
  );
  const other = await login();
  assert.equal(
    (
      await request(env.ports.edge, '/v1/account/operations/' + id, {
        headers: { cookie: other.cookie },
      })
    ).status,
    404,
  );
  const db = await database();
  try {
    const [[row]] = await db.query(
      'SELECT COUNT(*) AS n FROM operation_receipt WHERE operation_id=?',
      [id],
    );
    assert.equal(row.n, 1);
  } finally {
    await db.end();
  }
});
test('credential epoch invalidates an old cookie on the next actual request', async () => {
  const s = await login(),
    db = await database();
  try {
    await db.query('UPDATE user SET credential_epoch=credential_epoch+1 WHERE id=?', [
      seed.user_id,
    ]);
  } finally {
    await db.end();
  }
  assert.equal(
    (await request(env.ports.edge, '/v1/account/me', { headers: { cookie: s.cookie } })).status,
    401,
  );
  assert.equal(
    (await post('/v1/account/login', { email: seed.email, password: seed.password })).status,
    200,
  );
});
test('Redis prefix ACL denies unrelated keys and the login limit is real', async () => {
  await assert.rejects(
    redisCommand(env.ports.redis, 'account_login', env.passwords.redisAccount, [
      'GET',
      'doc:secret',
    ]),
    /REDIS_PERMISSION_DENIED/,
  );
  for (let n = 0; n < 10; n++)
    assert.equal(
      (await post('/v1/account/login', { email: seed.email, password: 'wrong-password' })).status,
      401,
    );
  assert.equal(
    (await post('/v1/account/login', { email: seed.email, password: 'wrong-password' })).status,
    429,
  );
});
