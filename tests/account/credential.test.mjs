import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import mysql from 'mysql2/promise';
import { state, runtime } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';
import { redisCommand } from '../../scripts/redis-client.mjs';
const env = state(),
  origin = 'https://localhost:' + env.ports.edge;
const post = (path, input, headers = {}) =>
  request(env.ports.edge, path, {
    method: 'POST',
    headers: { origin, 'content-type': 'application/json', ...headers },
    body: JSON.stringify(input),
  });
const jar = (r) => (r.headers['set-cookie'] ?? []).map((x) => x.split(';')[0]).join('; ');
async function code(email, purpose, headers = {}) {
  const r = await post('/v1/account/challenges', { email, purpose }, headers);
  assert.equal(r.status, 200);
  const id = JSON.parse(r.body).challenge_id;
  const folder = resolve(runtime, 'smtp/inbox');
  for (const file of readdirSync(folder)) {
    const mail = JSON.parse(readFileSync(resolve(folder, file)));
    if (mail.body.includes('Challenge: ' + id))
      return { challenge_id: id, code: /Code: (\d{6})/.exec(mail.body)[1] };
  }
  throw Error('MAIL_MISSING');
}
async function signup() {
  const email = 'credential-' + randomUUID() + '@example.test',
    password = 'initial-password-123',
    challenge = await code(email, 'SIGNUP');
  const r = await post('/v1/account/signup', {
    operation_id: 'signup-' + randomUUID(),
    ...challenge,
    email,
    password,
    display_name: '凭据测试',
  });
  assert.equal(r.status, 200);
  return { email, password, id: JSON.parse(r.body).result.entity_id };
}
async function login(email, password) {
  const r = await post('/v1/account/login', { email, password });
  assert.equal(r.status, 200);
  return { cookie: jar(r), 'x-csrf-token': JSON.parse(r.body).csrf_token };
}
async function terminal(path, input, headers = {}) {
  let r;
  for (let n = 0; n < 60; n++) {
    r = await post(path, input, headers);
    assert.ok([200, 202].includes(r.status), String(r.status));
    if (JSON.parse(r.body).state === 'SUCCEEDED') return r;
    await new Promise((r) => setTimeout(r, 100));
  }
  assert.fail('OPERATION_NOT_TERMINAL');
}
test('verified email collision returns a stable conflict without consuming the challenge or closing the old session', async () => {
  const [actor, existing] = await Promise.all([signup(), signup()]),
    headers = await login(actor.email, actor.password);
  const proof = await code(existing.email, 'CHANGE_EMAIL', headers),
    input = {
      operation_id: 'collision-' + randomUUID(),
      expected_credential_epoch: '1',
      ...proof,
      new_email: existing.email,
    };
  const r = await post('/v1/account/email', input, headers);
  assert.equal(r.status, 409);
  assert.equal(JSON.parse(r.body).code, 'EMAIL_ALREADY_REGISTERED');
  assert.equal((await request(env.ports.edge, '/v1/account/me', { headers })).status, 200);
  assert.equal((await post('/v1/account/email', input, headers)).status, 409);
});
test('password reset closes old epoch and Redis restore cannot replay consumed verification', async () => {
  const u = await signup(),
    old = await login(u.email, u.password),
    proof = await code(u.email, 'RESET_PASSWORD');
  const key = 'acct:challenge:' + proof.challenge_id;
  const backup = await redisCommand(
    env.ports.redis,
    'bootstrap_admin',
    env.passwords.redisAdmin,
    ['DUMP', key],
    { binaryBulk: true },
  );
  assert.ok(Buffer.isBuffer(backup));
  assert.ok(!backup.includes(Buffer.from(proof.code)));
  const input = {
    operation_id: 'reset-' + randomUUID(),
    ...proof,
    new_password: 'reset-password-123',
  };
  await terminal('/v1/account/password-resets', input);
  assert.equal((await request(env.ports.edge, '/v1/account/me', { headers: old })).status, 401);
  assert.equal(
    (await post('/v1/account/login', { email: u.email, password: u.password })).status,
    401,
  );
  await login(u.email, input.new_password);
  // 恢复消费前的真实Redis字段快照；不同号仍必须被MySQL消费终态阻止。
  await redisCommand(env.ports.redis, 'bootstrap_admin', env.passwords.redisAdmin, ['DEL', key]);
  await redisCommand(env.ports.redis, 'bootstrap_admin', env.passwords.redisAdmin, [
    'RESTORE',
    key,
    '600000',
    backup,
  ]);
  assert.equal(
    (
      await post('/v1/account/password-resets', {
        ...input,
        operation_id: 'replay-' + randomUUID(),
      })
    ).status,
    409,
  );
});
test('authenticated password change, verified email change and session expiry all check current credentials', async () => {
  const u = await signup(),
    old = await login(u.email, u.password);
  const me = JSON.parse((await request(env.ports.edge, '/v1/account/me', { headers: old })).body);
  const input = {
    operation_id: 'password-' + randomUUID(),
    expected_credential_epoch: me.credential_epoch,
    current_password: u.password,
    new_password: 'changed-password-123',
  };
  await terminal('/v1/account/password', input, old);
  assert.equal((await request(env.ports.edge, '/v1/account/me', { headers: old })).status, 401);
  const current = await login(u.email, input.new_password),
    currentMe = JSON.parse(
      (await request(env.ports.edge, '/v1/account/me', { headers: current })).body,
    ),
    newEmail = 'changed-' + randomUUID() + '@example.test';
  const proof = await code(newEmail, 'CHANGE_EMAIL', current),
    emailInput = {
      operation_id: 'email-' + randomUUID(),
      expected_credential_epoch: currentMe.credential_epoch,
      ...proof,
      new_email: newEmail,
    };
  await terminal('/v1/account/email', emailInput, current);
  assert.equal(
    (await post('/v1/account/login', { email: u.email, password: input.new_password })).status,
    401,
  );
  const latest = await login(newEmail, input.new_password),
    db = await mysql.createConnection({
      host: '127.0.0.1',
      port: env.ports.mysql,
      user: 'collab_account',
      password: env.passwords.account,
      database: 'account_db',
    });
  try {
    await db.query(
      "UPDATE session SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE user_id=? AND state='ACTIVE'",
      [u.id],
    );
  } finally {
    await db.end();
  }
  assert.equal((await request(env.ports.edge, '/v1/account/me', { headers: latest })).status, 401);
});
