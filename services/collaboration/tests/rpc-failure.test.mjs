import test from 'node:test';
import assert from 'node:assert/strict';
import { RPCFailure } from '../src/fence.mjs';
// 下游驱动/传输错误不能借ERROR帧穿透私有边界；只允许稳定业务码。
test('RPC failures expose only stable bounded codes rather than raw driver or transport details', () => {
  for (const raw of [
    'Access denied for user private_actor password private_fixture',
    'SELECT canonical_body FROM private_fixture',
    'socket hang up',
    'X'.repeat(200),
  ]) {
    const failure = new RPCFailure(2, raw);
    assert.equal(failure.details, 'SERVICE_UNAVAILABLE');
    assert.equal(failure.message, 'SERVICE_UNAVAILABLE');
    assert.equal(failure.code, 14);
  }
  assert.equal(new RPCFailure(9, 'RESOURCE_NOT_READY').details, 'RESOURCE_NOT_READY');
});
