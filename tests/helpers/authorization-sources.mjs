import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { reviewDatabase } from './review.mjs';

const decode = (value) => (typeof value === 'string' ? JSON.parse(value) : value);
export function sourceFence(revocation, scope, epoch) {
  return {
    revocation_id: revocation,
    closed_scopes: [{ scope_key: scope, epoch: String(epoch) }],
    generation: '1',
    target_digest: createHash('sha256')
      .update('AUTHORIZATION_CHANGE\n' + scope + '\n' + epoch)
      .digest(),
  };
}

// 原始业务来源和完整ACK分别核验；不能把另一关闭的32字节摘要借给本次原操作。
export async function assertSourceClosure(revocation, scope, epoch, roles) {
  const request = sourceFence(revocation, scope, epoch);
  const target = request.target_digest.toString('hex');
  for (const role of roles) {
    const database = await reviewDatabase(role);
    try {
      const [[gate]] = await database.query(
        'SELECT state,CAST(max_epoch AS CHAR) epoch,last_revocation_id FROM access_gate WHERE scope_key=?',
        [scope],
      );
      assert.equal(gate?.state, 'BLOCKED');
      assert.equal(gate.epoch, String(epoch));
      assert.ok(gate.last_revocation_id === revocation, 'ORIGINAL_SOURCE_GATE_REQUIRED');
      const [[closure]] = await database.query(
        'SELECT state,scope_key,CAST(epoch AS CHAR) epoch,target_permission,participant_plan FROM authorization_change WHERE revocation_id=?',
        [revocation],
      );
      assert.equal(closure?.state, 'SUCCEEDED');
      assert.ok(closure.scope_key === scope, 'ORIGINAL_SOURCE_SCOPE_REQUIRED');
      assert.equal(closure.epoch, String(epoch));
      const binding = decode(closure.target_permission);
      if (role === 'account') {
        assert.ok(binding.scope_key === scope, 'ORIGINAL_ACCOUNT_SCOPE_REQUIRED');
        if ('closed_epoch' in binding) assert.equal(binding.closed_epoch, String(epoch));
      } else {
        assert.equal(binding.generation, '1');
        assert.ok(binding.bootstrap_target_digest === target, 'ORIGINAL_SOURCE_TARGET_REQUIRED');
      }
      const participants = {
        account: ['document'],
        document: ['edge', 'collaboration'],
        collaboration: ['node-runtime'],
      }[role];
      assert.deepEqual(
        [...decode(closure.participant_plan)].sort(),
        role === 'collaboration' ? [] : [...participants].sort(),
      );
      const [acks] = await database.query(
        'SELECT participant,phase,result_digest FROM authorization_ack WHERE revocation_id=? AND scope_key=? AND epoch=?',
        [revocation, scope, String(epoch)],
      );
      assert.equal(acks.length, participants.length);
      for (const participant of participants) {
        const ack = acks.find((value) => value.participant === participant);
        assert.equal(ack?.phase, 'OUTPUT_STOPPED');
        const protocolRole = participant === 'node-runtime' ? 'collaboration' : participant;
        const expected = createHash('sha256')
          .update(
            [
              'bootstrap-fence-v1',
              protocolRole,
              revocation,
              scope,
              String(epoch),
              '1',
              target,
            ].join('\n'),
          )
          .digest();
        assert.ok(
          Buffer.isBuffer(ack.result_digest) && ack.result_digest.equals(expected),
          'ORIGINAL_SOURCE_ACK_REQUIRED',
        );
      }
    } finally {
      await database.end();
    }
  }
  return request;
}

export async function assertWorkspaceClosed(opened) {
  for (const role of ['document', 'collaboration']) {
    const database = await reviewDatabase(role);
    try {
      const column = role === 'document' ? 'admission_id' : 'grant_id';
      const id =
        role === 'document' ? opened.grant.context.document_admission_ref : opened.grant.grant_id;
      const table = role === 'document' ? 'document_admission' : 'collaboration_grant';
      const [[binding]] = await database.query(
        'SELECT state FROM ' + table + ' WHERE ' + column + '=?',
        [id],
      );
      assert.equal(binding?.state, 'CLOSED');
      const [[pending]] = await database.query(
        'SELECT COUNT(*) n FROM delivery_session WHERE ' + column + "=? AND state<>'SETTLED'",
        [id],
      );
      assert.equal(Number(pending.n), 0);
    } finally {
      await database.end();
    }
  }
}

export async function assertNewSource(opened, scope, oldEpoch) {
  const source = opened.grant.context.sources.find((value) => value.scope_key === scope);
  assert.ok(source, 'CURRENT_FULL_SOURCE_REQUIRED');
  assert.ok(BigInt(source.epoch) > BigInt(oldEpoch), 'NEW_SOURCE_EPOCH_REQUIRED');
  for (const role of ['document', 'collaboration']) {
    const database = await reviewDatabase(role);
    try {
      const [[gate]] = await database.query(
        'SELECT state,CAST(max_epoch AS CHAR) epoch FROM access_gate WHERE scope_key=?',
        [scope],
      );
      assert.equal(gate?.state, 'OPEN');
      assert.equal(gate.epoch, String(source.epoch));
    } finally {
      await database.end();
    }
  }
}
