import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { binding, ack } from '../src/domain/fence_protocol.mjs';

function request(scope = 'publication:9007199254740993', epoch = '18446744073709551615') {
  return {
    revocation_id: 'publication-fence-operation',
    generation: '1',
    closed_scopes: [{ scope_key: scope, epoch }],
    target_digest: createHash('sha256').update(`AUTHORIZATION_CHANGE\n${scope}\n${epoch}`).digest(),
  };
}

// 这里只验证纯协议绑定；真正的旧公开HTTP停止和SQL ACK必须在产品集成验证。
test('publication fence preserves original full uint64 scope and epoch', () => {
  const input = request();
  const fixed = binding(input);
  assert.equal(fixed.generation, '1');
  assert.equal(fixed.bootstrap_target_digest, input.target_digest.toString('hex'));
  assert.deepEqual(ack(input).closed_scopes, input.closed_scopes);
  assert.equal(binding(request('publication:18446744073709551615')).generation, '1');
});

test('publication fence rejects changed original digest, generation and participant scope', () => {
  for (const mutate of [
    (input) => {
      input.generation = '2';
    },
    (input) => {
      input.closed_scopes[0].epoch = '3';
    },
    (input) => {
      input.target_digest[0] ^= 1;
    },
    (input) => {
      input.closed_scopes.push({ scope_key: 'publication:1', epoch: '1' });
    },
  ]) {
    const input = request();
    mutate(input);
    assert.throws(
      () => binding(input),
      (error) => error.code === 3 && error.message === 'BOOTSTRAP_FENCE_INVALID',
    );
  }
});

test('publication fence rejects ambiguous or overflowing scope and epoch', () => {
  for (const scope of [
    'publication:0',
    'publication:01',
    'publication:+1',
    'publication:1:2',
    'publication:18446744073709551616',
  ]) {
    assert.throws(() => binding(request(scope)), { code: 3 });
  }
  for (const epoch of ['0', '01', '+1', '1.0', '1e3', '18446744073709551616', 9007199254740992]) {
    assert.throws(() => binding(request('publication:1', epoch)), { code: 3 });
  }
});
