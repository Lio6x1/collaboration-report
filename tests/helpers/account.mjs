import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { state, runtime } from '../../scripts/runtime-lib.mjs';
import { request } from './https.mjs';
import { observe } from './schema.mjs';
export const env = state(),
  origin = 'https://localhost:' + env.ports.edge;
export const operation = (prefix) => prefix + '-' + randomUUID();
export const post = (path, input, headers = {}, method = 'POST') =>
  request(env.ports.edge, path, {
    method,
    headers: { origin, 'content-type': 'application/json', ...headers },
    body: JSON.stringify(input),
  }).then((r) => observe(method, path, r));
export const get = (path, headers = {}) =>
  request(env.ports.edge, path, { headers }).then((r) => observe('GET', path, r));
export const json = (r) => JSON.parse(r.body);
// 真实SMTP环境显式复用本机专用体验身份；密钥只在进程内使用，不进入测试输出。
export async function existingTestAccount() {
  const path = process.env.COLLABORATION_TEST_ACCOUNT;
  if (!path) return null;
  const { email, password } = JSON.parse(readFileSync(path, 'utf8'));
  const login = await post('/v1/account/login', { email, password });
  assert.equal(login.status, 200, '专用体验账号须能登录');
  return {
    email,
    password,
    id: json(login).user_id,
    headers: {
      cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
      'x-csrf-token': json(login).csrf_token,
    },
  };
}
export async function createUser(options = {}) {
  // 真实SMTP实例上的工作流验收使用显式开启的隔离身份预置；不改投递配置、不向虚构邮箱发信。
  // 这里只预置账号，其后的登录、授权、正文和幂等结果仍由真实服务完成。
  if (process.env.COLLABORATION_TEST_IDENTITIES_FROM_ACCOUNT === '1') {
    assert.ok(process.env.COLLABORATION_TEST_ACCOUNT, 'TEST_IDENTITY_TEMPLATE_REQUIRED');
    assert.equal(options.password, undefined, 'CUSTOM_PASSWORD_REQUIRES_SIGNUP_TEST');
    const template = JSON.parse(readFileSync(process.env.COLLABORATION_TEST_ACCOUNT, 'utf8'));
    const config = JSON.parse(readFileSync(resolve(runtime, 'services/account.json'), 'utf8'));
    const mysql = await import('mysql2/promise');
    const db = await mysql.default.createConnection(config.database);
    const email = 'workflow-' + randomUUID() + '@example.test';
    try {
      const [made] = await db.execute(
        `INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at)
         SELECT ?,?,'工作流验证成员',password_hash,password_algorithm,'ACTIVE',UTC_TIMESTAMP(6)
         FROM user WHERE email_key=? AND state='ACTIVE' AND password_hash IS NOT NULL`,
        [email, Buffer.from(email), Buffer.from(template.email.trim().toLowerCase())],
      );
      assert.equal(made.affectedRows, 1, 'TEST_IDENTITY_TEMPLATE_UNAVAILABLE');
    } finally {
      await db.end();
    }
    const login = await post('/v1/account/login', { email, password: template.password });
    assert.equal(login.status, 200, 'TEST_IDENTITY_LOGIN_FAILED');
    return {
      id: json(login).user_id,
      email,
      password: template.password,
      headers: {
        cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
        'x-csrf-token': json(login).csrf_token,
      },
    };
  }
  const email = 'fixture-' + randomUUID() + '@example.test',
    password = options.password ?? 'fixture-password-123';
  const r = await post('/v1/account/challenges', { email, purpose: 'SIGNUP' });
  assert.equal(r.status, 200);
  const id = json(r).challenge_id;
  let code;
  const folder = resolve(runtime, 'smtp/inbox');
  for (const f of readdirSync(folder)) {
    const mail = JSON.parse(readFileSync(resolve(folder, f)));
    if (mail.body.includes('Challenge: ' + id)) {
      code = /Code: (\d{6})/.exec(mail.body)[1];
      break;
    }
  }
  assert.ok(code);
  const signup = await post('/v1/account/signup', {
    operation_id: operation('signup'),
    challenge_id: id,
    code,
    email,
    password,
    display_name: '组织测试成员',
  });
  assert.equal(signup.status, 200);
  const login = await post('/v1/account/login', { email, password });
  assert.equal(login.status, 200);
  return {
    id: json(signup).result.entity_id,
    email,
    password,
    headers: {
      cookie: login.headers['set-cookie'].map((x) => x.split(';')[0]).join('; '),
      'x-csrf-token': json(login).csrf_token,
    },
  };
}
export async function waitOperation(r, user) {
  assert.ok([200, 201, 202].includes(r.status), String(r.status));
  const first = json(r);
  if (first.state === 'SUCCEEDED') return first;
  for (let n = 0; n < 80; n++) {
    const poll = await get('/v1/account/operations/' + first.operation_id, user.headers);
    assert.equal(poll.status, 200);
    const value = json(poll);
    if (value.state === 'SUCCEEDED') return value;
    await new Promise((r) => setTimeout(r, 100));
  }
  assert.fail('OPERATION_NOT_TERMINAL');
}
