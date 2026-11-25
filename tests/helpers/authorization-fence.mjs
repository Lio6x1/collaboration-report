import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { post, json, operation } from './workspace.mjs';
import { reviewDatabase } from './review.mjs';

const sessionId = (user) => {
  const token = /__Host-collabSession=([^;]+)/.exec(user.headers.cookie)?.[1];
  assert.ok(token, 'REAL_LOGIN_COOKIE_REQUIRED');
  return createHash('sha256').update(Buffer.from(token, 'base64url')).digest();
};

async function loginAgain(user) {
  const response = await post('/v1/account/login', { email: user.email, password: user.password });
  assert.equal(response.status, 200);
  return {
    ...user,
    headers: {
      cookie: response.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
      'x-csrf-token': json(response).csrf_token,
    },
  };
}

async function until(predicate, code, budget = 4000) {
  const deadline = Date.now() + budget;
  while (Date.now() < deadline) {
    const value = await predicate();
    if (value) return value;
    await new Promise((done) => setTimeout(done, 50));
  }
  assert.fail(code);
}

async function assertClosureFacts(user, opened, revocation) {
  const id = sessionId(user),
    scope = 'session:' + id.toString('hex');
  const targetDigest = createHash('sha256')
    .update('SESSION_LOGOUT\n' + id.toString('hex'))
    .digest('hex');
  for (const role of ['account', 'document', 'collaboration']) {
    const database = await reviewDatabase(role);
    try {
      const [[gate]] = await database.query(
        'SELECT state,CAST(max_epoch AS CHAR) epoch,last_revocation_id FROM access_gate WHERE scope_key=?',
        [scope],
      );
      assert.equal(gate?.state, 'BLOCKED', `${role} must retain source tombstone`);
      assert.equal(gate.epoch, '1');
      assert.ok(gate.last_revocation_id === revocation, 'ORIGINAL_REVOCATION_GATE_REQUIRED');
      const [[closure]] = await database.query(
        'SELECT state,scope_key,CAST(epoch AS CHAR) epoch,target_permission,participant_plan FROM authorization_change WHERE revocation_id=?',
        [revocation],
      );
      assert.equal(closure?.state, 'SUCCEEDED');
      assert.ok(closure.scope_key === scope, 'ORIGINAL_CLOSURE_SCOPE_REQUIRED');
      assert.equal(closure.epoch, '1');
      const target =
        typeof closure.target_permission === 'string'
          ? JSON.parse(closure.target_permission)
          : closure.target_permission;
      if (role === 'account') {
        assert.ok(target.session_hash === id.toString('hex'), 'ORIGINAL_SESSION_TARGET_REQUIRED');
        assert.ok(target.scope_key === scope, 'ORIGINAL_SESSION_SCOPE_REQUIRED');
      } else {
        assert.equal(target.generation, '1');
        assert.ok(
          target.bootstrap_target_digest === targetDigest,
          'ORIGINAL_FENCE_TARGET_REQUIRED',
        );
      }
      const [acks] = await database.query(
        'SELECT participant,phase,result_digest FROM authorization_ack WHERE revocation_id=? AND scope_key=? AND epoch=1',
        [revocation, scope],
      );
      const required = {
        account: ['document'],
        document: ['edge', 'collaboration'],
        collaboration: ['node-runtime'],
      }[role];
      const plan =
        typeof closure.participant_plan === 'string'
          ? JSON.parse(closure.participant_plan)
          : closure.participant_plan;
      const expectedPlan = role === 'collaboration' ? [] : required;
      assert.deepEqual([...plan].sort(), [...expectedPlan].sort());
      assert.equal(acks.length, required.length, 'EXACT_PARTICIPANT_ACK_SET_REQUIRED');
      for (const participant of required) {
        const ack = acks.find((value) => value.participant === participant);
        assert.equal(ack?.phase, 'OUTPUT_STOPPED');
        // schema中的ACK没有单独generation列；原固定绑定和摘要均包含generation=1。
        // 重算完整协议摘要，而非只检查32字节长度；失败只输出布尔/固定原因，不泄露摘要。
        const ackRole = participant === 'node-runtime' ? 'collaboration' : participant;
        const expected = createHash('sha256')
          .update(
            ['bootstrap-fence-v1', ackRole, revocation, scope, '1', '1', targetDigest].join('\n'),
          )
          .digest();
        assert.ok(
          Buffer.isBuffer(ack.result_digest) && ack.result_digest.equals(expected),
          'ORIGINAL_FENCE_ACK_DIGEST_REQUIRED',
        );
      }
      if (role === 'account') {
        const [[session]] = await database.query('SELECT state FROM session WHERE id=?', [id]);
        assert.equal(session.state, 'CLOSED');
      } else if (role === 'document') {
        const admission = opened.grant.context.document_admission_ref;
        const [[row]] = await database.query(
          'SELECT state FROM document_admission WHERE admission_id=?',
          [admission],
        );
        assert.equal(row?.state, 'CLOSED');
        const [[pending]] = await database.query(
          "SELECT COUNT(*) n FROM delivery_session WHERE admission_id=? AND state<>'SETTLED'",
          [admission],
        );
        assert.equal(Number(pending.n), 0);
      } else {
        const [[grant]] = await database.query(
          'SELECT state FROM collaboration_grant WHERE grant_id=?',
          [opened.grant.grant_id],
        );
        assert.equal(grant?.state, 'CLOSED');
        const [[pending]] = await database.query(
          "SELECT COUNT(*) n FROM delivery_session WHERE grant_id=? AND state<>'SETTLED'",
          [opened.grant.grant_id],
        );
        assert.equal(Number(pending.n), 0);
      }
    } finally {
      await database.end();
    }
  }
}

async function pendingTicket(document, user) {
  const client = operation('unredeemed-client');
  const response = await post(
    `/v1/documents/${document.id}/units/${document.main.units[0].unit_id}/edit-sessions`,
    {
      operation_id: operation('unredeemed-edit'),
      workspace_kind: 'BRANCH',
      client_session_id: client,
      expected_structure_id: document.main.structure.id,
    },
    user.headers,
  );
  assert.equal(response.status, 200);
  return { client, session: json(response) };
}

export { sessionId, loginAgain, until, assertClosureFacts, pendingTicket };
