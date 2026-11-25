import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { createUser, get } from '../helpers/account.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

test(
  'actual expired-session backlog closes original scopes after owned restart through bounded bootstrap and readiness stages',
  { timeout: 180000 },
  async (t) => {
    const db = await reviewDatabase('account');
    const users = [],
      sessions = [];
    let stopped = false;
    try {
      for (let index = 0; index < 32; index++) {
        const user = await createUser();
        users.push(user);
        const token = /__Host-collabSession=([^;]+)/.exec(user.headers.cookie)[1];
        sessions.push(createHash('sha256').update(Buffer.from(token, 'base64url')).digest());
      }
      stopOwnedRole('collaboration');
      stopped = true;
      // 仅推进本例真实会话的到期事实；不插入关闭任务、不补ACK，也不改原身份准入期限。
      const [changed] = await db.query(
        'UPDATE session SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE id IN (' +
          sessions.map(() => '?').join(',') +
          ')',
        sessions,
      );
      assert.equal(changed.affectedRows, 32);
      await new Promise((resolve) => setTimeout(resolve, 500));
      const started = Date.now();
      const restarted = restartOwnedRuntime();
      stopped = false;
      const stages = restarted.stages;
      assert.ok(stages.indexOf('COLLABORATION_BOOTSTRAPPED') >= 0);
      assert.ok(stages.indexOf('COLLABORATION_BOOTSTRAPPED') < stages.indexOf('LAUNCH_ACCOUNT'));
      assert.ok(stages.indexOf('LAUNCH_ACCOUNT') < stages.indexOf('OUTPUTS_READY'));
      t.diagnostic('EXPIRY_BACKLOG_READY elapsed_ms=' + String(Date.now() - started));
      for (let index = 0; index < sessions.length; index++) {
        const session = sessions[index],
          original = 'expiry-' + session.toString('hex');
        const [[fact]] = await db.query(
          "SELECT s.state session_state,o.state receipt_state,t.state task_state,(SELECT COUNT(*) FROM authorization_ack a WHERE a.revocation_id=o.operation_id AND a.scope_key=? AND a.participant='document' AND a.phase='OUTPUT_STOPPED' AND OCTET_LENGTH(a.result_digest)=32) ack_count FROM session s JOIN operation_receipt o ON o.operation_id=? JOIN durable_task t ON t.kind='SESSION_LOGOUT' AND t.business_key=o.operation_id WHERE s.id=?",
          ['session:' + session.toString('hex'), original, session],
        );
        assert.ok(fact, '每个实际到期会话必须保留原关闭任务');
        assert.equal(fact.session_state, 'CLOSED');
        assert.equal(fact.receipt_state, 'SUCCEEDED');
        assert.equal(fact.task_state, 'SUCCEEDED');
        assert.equal(String(fact.ack_count), '1');
        assert.equal((await get('/v1/account/me', users[index].headers)).status, 401);
      }
    } finally {
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
