import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { state, runtime } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';
const env = state(),
  origin = 'https://localhost:' + env.ports.edge;
const post = (path, input) =>
  request(env.ports.edge, path, {
    method: 'POST',
    headers: { origin, 'content-type': 'application/json' },
    body: JSON.stringify(input),
  });
function mail(id) {
  const folder = resolve(runtime, 'smtp/inbox');
  for (const file of readdirSync(folder)) {
    const value = JSON.parse(readFileSync(resolve(folder, file)));
    if (value.body.includes('Challenge: ' + id)) return value;
  }
  throw Error('CHALLENGE_MAIL_NOT_RECEIVED');
}
test('signup uses real SMTP code and consumed challenge cannot register again', async () => {
  // 如果handler仅返回挑战ID却不投递SMTP、或没有MySQL消费终态，本例都会失败。
  const email = 'signup-' + randomUUID() + '@example.test';
  const first = await post('/v1/account/challenges', { purpose: 'SIGNUP', email });
  assert.equal(first.status, 200);
  const challenge = JSON.parse(first.body),
    received = mail(challenge.challenge_id),
    code = /Code: (\d{6})/.exec(received.body)?.[1];
  assert.ok(code);
  assert.equal((await post('/v1/account/challenges', { purpose: 'SIGNUP', email })).status, 429);
  const input = {
    operation_id: 'signup-' + randomUUID(),
    challenge_id: challenge.challenge_id,
    code,
    email,
    password: 'learner-password-123',
    display_name: '注册学习者',
  };
  const [signup, concurrent] = await Promise.all([
    post('/v1/account/signup', input),
    post('/v1/account/signup', input),
  ]);
  assert.equal(signup.status, 200);
  assert.equal(concurrent.status, 200);
  assert.equal(JSON.parse(signup.body).state, 'SUCCEEDED');
  assert.deepEqual(JSON.parse(signup.body), JSON.parse(concurrent.body));
  assert.equal((await post('/v1/account/signup', input)).status, 200);
  assert.equal(
    (await post('/v1/account/signup', { ...input, operation_id: 'replay-' + randomUUID() })).status,
    409,
  );
  assert.equal((await post('/v1/account/login', { email, password: input.password })).status, 200);
});
test('wrong purpose and bounded guesses never consume an unrelated challenge', async () => {
  const email = 'guesses-' + randomUUID() + '@example.test',
    r = await post('/v1/account/challenges', { purpose: 'SIGNUP', email });
  assert.equal(r.status, 200);
  const challenge = JSON.parse(r.body),
    code = /Code: (\d{6})/.exec(mail(challenge.challenge_id).body)[1];
  assert.equal(
    (
      await post('/v1/account/password-resets', {
        operation_id: 'reset-' + randomUUID(),
        challenge_id: challenge.challenge_id,
        code,
        new_password: 'new-pass-123',
      })
    ).status,
    400,
  );
  const wrong = code === '000000' ? '111111' : '000000';
  for (let i = 0; i < 5; i++)
    assert.equal(
      (
        await post('/v1/account/signup', {
          operation_id: 'wrong-' + randomUUID(),
          challenge_id: challenge.challenge_id,
          code: wrong,
          email,
          password: 'new-pass-123',
          display_name: '猜测者',
        })
      ).status,
      400,
    );
  assert.equal(
    (
      await post('/v1/account/signup', {
        operation_id: 'locked-' + randomUUID(),
        challenge_id: challenge.challenge_id,
        code,
        email,
        password: 'new-pass-123',
        display_name: '猜测者',
      })
    ).status,
    400,
  );
});
