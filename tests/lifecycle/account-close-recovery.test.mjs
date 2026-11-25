import test from 'node:test';
import assert from 'node:assert/strict';
import { pause } from '../../scripts/runtime-lib.mjs';
import { createUser, get, post, json, operation } from '../helpers/account.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

// 在真实Node退出造成原排空ACK未知时，再终止Account进程。数据库和身份事实
// 不改成功；重启必须自行恢复原任务，用户只GET同一进度而不重新发起注销。
test(
  'account closure resumes the persisted original task after actual owner process exits',
  { timeout: 180000 },
  async () => {
    const user = await createUser();
    const meResponse = await get('/v1/account/me', user.headers);
    assert.equal(meResponse.status, 200, meResponse.body);
    const input = {
      operation_id: operation('close-restart'),
      expected_identity_epoch: json(meResponse).identity_epoch,
      current_password: user.password,
    };
    const database = await reviewDatabase('account');
    let stopped = false;
    try {
      stopOwnedRole('collaboration');
      stopped = true;
      const started = await post('/v1/account/closures', input, user.headers);
      assert.equal(started.status, 202, started.body);
      let original;
      const frozenDeadline = Date.now() + 10000;
      while (Date.now() < frozenDeadline) {
        const [[row]] = await database.query(
          `SELECT c.state,c.retention_plan,t.id,CAST(t.generation AS CHAR) generation,t.state task_state
        FROM account_close c JOIN durable_task t ON t.operation_id=c.operation_id AND t.kind='ACCOUNT_CLOSE_RECOVER' WHERE c.operation_id=?`,
          [input.operation_id],
        );
        if (row?.state === 'FROZEN') {
          original = row;
          break;
        }
        await pause(100);
      }
      assert.equal(
        original?.state,
        'FROZEN',
        'original closure must freeze without declaring absent Node ACK complete',
      );
      assert.notEqual(original.task_state, 'SUCCEEDED');
      const before = json(await get(`/v1/account/closures/${input.operation_id}`, user.headers));
      assert.equal(before.state, 'FROZEN');
      stopOwnedRole('account');
      console.log(
        'ACCOUNT_CLOSE_RECOVERY_ACTUAL_EXIT: original FROZEN identity and persisted task; no complete ACK; Account and Node owner processes terminated',
      );
      restartOwnedRuntime();
      stopped = false;
      let final;
      const deadline = Date.now() + 20000;
      while (Date.now() < deadline) {
        const response = await get(`/v1/account/closures/${input.operation_id}`, user.headers);
        assert.equal(response.status, 200, response.body);
        final = json(response);
        if (final.state === 'SUCCEEDED' || final.state === 'REJECTED') break;
        await pause(100);
      }
      assert.equal(
        final?.state,
        'SUCCEEDED',
        `ORIGINAL_CLOSE_RESTART_NOT_TERMINAL ${final?.state}`,
      );
      const [[recovered]] = await database.query(
        `SELECT c.retention_plan,t.id,CAST(t.generation AS CHAR) generation,t.state task_state,
       u.state user_state,u.password_hash,u.email FROM account_close c JOIN durable_task t ON t.operation_id=c.operation_id AND t.kind='ACCOUNT_CLOSE_RECOVER'
       JOIN user u ON u.id=c.user_id WHERE c.operation_id=?`,
        [input.operation_id],
      );
      assert.equal(String(recovered.id), String(original.id));
      assert.ok(BigInt(recovered.generation) > BigInt(original.generation));
      assert.equal(recovered.task_state, 'SUCCEEDED');
      assert.equal(recovered.user_state, 'CLOSED');
      assert.equal(recovered.email, null);
      assert.equal(recovered.password_hash, null);
      for (const field of [
        'session_hash',
        'credential_epoch',
        'identity_epoch',
        'binding',
        'organizations',
        'scopes',
      ])
        assert.deepEqual(
          recovered.retention_plan[field],
          original.retention_plan[field],
          'original ' + field,
        );
      const [[audit]] = await database.query(
        "SELECT COUNT(*) n FROM audit_event WHERE operation_id=? AND kind='ACCOUNT_CLOSE_COMPLETE'",
        [input.operation_id],
      );
      assert.equal(Number(audit.n), 1);
      assert.equal((await get('/v1/account/me', user.headers)).status, 401);
      console.log(
        'ACCOUNT_CLOSE_RECOVERY_ACTUAL_PASS: same original task ID/binding/scopes after real process exits; no second POST; actual ACK and Doc completion before credentials removed; one terminal audit',
      );
    } finally {
      await database.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
