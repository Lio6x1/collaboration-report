import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import test from 'node:test';
import { get, json } from '../helpers/account.mjs';

test('search HTTP route returns an exhausted anonymous page and rejects forged bindings', async () => {
  // 新词保证没有既有正式版命中；真实Doc→Search→ES空EOF仍必须完整成功，不能501占位。
  const response = await get('/v1/search?q=' + randomUUID());
  assert.equal(response.status, 200, json(response).code);
  assert.deepEqual(json(response), { items: [], next_cursor: null, has_more: false });
  for (const path of ['/v1/search?q=a&q=b', '/v1/search?q=a&scope_hash=fake']) {
    const rejected = await get(path);
    assert.equal(rejected.status, 400);
    assert.equal(json(rejected).code, 'SEARCH_PARAMETERS_INVALID');
  }
  const session = await get('/v1/search?q=a', { cookie: '__Host-collabSession=bad' });
  assert.equal(session.status, 401);
});
