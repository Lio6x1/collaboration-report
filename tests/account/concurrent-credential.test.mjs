import test from 'node:test';
import assert from 'node:assert/strict';
import { createUser, get, post, json, operation } from '../helpers/account.mjs';
test('concurrent retries of one password operation return one original result', async () => {
  const u = await createUser(),
    me = json(await get('/v1/account/me', u.headers)),
    input = {
      operation_id: operation('passwordrace'),
      expected_credential_epoch: me.credential_epoch,
      current_password: u.password,
      new_password: 'concurrent-password-123',
    };
  const responses = await Promise.all(
    Array.from({ length: 4 }, () => post('/v1/account/password', input, u.headers)),
  );
  for (const r of responses) {
    assert.equal(r.status, 202);
    assert.equal(json(r).operation_id, input.operation_id);
  }
});
