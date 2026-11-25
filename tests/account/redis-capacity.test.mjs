import test from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { redisCommand } from '../../scripts/redis-client.mjs';
import { env, post, json } from '../helpers/account.mjs';
const admin = (args) =>
  redisCommand(env.ports.redis, 'bootstrap_admin', env.passwords.redisAdmin, args);
test('Redis noeviction exhaustion makes challenge issuance fail closed and restores original budget', async () => {
  const config = await admin(['CONFIG', 'GET', 'maxmemory']);
  assert.equal(config[0], 'maxmemory');
  const original = config[1];
  try {
    assert.equal(await admin(['CONFIG', 'SET', 'maxmemory', '1']), 'OK');
    const r = await post('/v1/account/challenges', {
      purpose: 'SIGNUP',
      email: 'full-' + randomUUID() + '@example.test',
    });
    assert.equal(r.status, 503);
    assert.equal(json(r).retry_action, 'RETRY_SAME_OPERATION');
  } finally {
    assert.equal(await admin(['CONFIG', 'SET', 'maxmemory', original]), 'OK');
  }
});
