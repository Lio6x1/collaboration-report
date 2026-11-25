import test from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { compose, ownedContainer, pause, docker } from '../../scripts/runtime-lib.mjs';
import { redisCommand } from '../../scripts/redis-client.mjs';
import { env, post, json } from '../helpers/account.mjs';
test('owned Redis AOF restart retains challenge hash and cooldown without granting account access', async () => {
  const email = 'restart-' + randomUUID() + '@example.test',
    r = await post('/v1/account/challenges', { purpose: 'SIGNUP', email });
  assert.equal(r.status, 200);
  const key = 'acct:challenge:' + json(r).challenge_id;
  const admin = (args) =>
      redisCommand(env.ports.redis, 'bootstrap_admin', env.passwords.redisAdmin, args),
    before = await admin(['HGET', key, 'digest']);
  assert.match(before, /^[a-f0-9]{64}$/);
  const disabled = await admin(['ACL', 'GETUSER', 'default']);
  assert.ok(disabled[disabled.indexOf('flags') + 1].includes('off'));
  assert.equal(disabled[disabled.indexOf('passwords') + 1].length, 0);
  await assert.rejects(redisCommand(env.ports.redis, 'default', 'not-a-password', ['PING']));
  const original = ownedContainer('redis'),
    started = new Date().toISOString();
  compose('restart', '-t', '2', 'redis');
  assert.equal(ownedContainer('redis'), original);
  let ready = false;
  for (let n = 0; n < 40; n++) {
    try {
      if ((await admin(['PING'])) === 'PONG') {
        ready = true;
        break;
      }
    } catch {}
    await pause(250);
  }
  assert.ok(ready);
  assert.equal(await admin(['HGET', key, 'digest']), before);
  assert.ok(!docker('logs', '--since', started, original).includes('AOF-loading-client'));
  assert.equal((await post('/v1/account/challenges', { purpose: 'SIGNUP', email })).status, 429);
  assert.equal((await post('/v1/account/login', { email, password: 'never-created' })).status, 401);
});
