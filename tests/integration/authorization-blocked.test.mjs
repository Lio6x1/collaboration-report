import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import test from 'node:test';
import { root, runtime, state } from '../../scripts/runtime-lib.mjs';
import { ProcessOwner } from '../../services/collaboration/src/client/process_owner.mjs';
import {
  organizationFixture,
  documentFixture,
  post,
  get,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { openReviewWorkspace, reviewDatabase } from '../helpers/review.mjs';
import { request } from '../helpers/https.mjs';
import { sessionId, until, assertClosureFacts } from '../helpers/authorization-fence.mjs';
import { blockOwnedCollaboration } from '../helpers/owned-blocking.mjs';

test('product fault: actual live Node is blocked, original close remains pending and resumes without restarting or a second POST', async () => {
  const fixture = await organizationFixture(0);
  const document = await documentFixture(fixture);
  const opened = await openReviewWorkspace(fixture, document);
  const account = await reviewDatabase('account');
  const doc = await reviewDatabase('document');
  const original = JSON.parse(readFileSync(resolve(runtime, 'processes.json'))).find(
    (item) => item.role === 'collaboration',
  );
  const scope = 'session:' + sessionId(fixture.owner).toString('hex');
  const revocation = operation('actual-blocked-logout');
  let fault;
  try {
    fault = await blockOwnedCollaboration();
    await assert.rejects(
      request(state().ports.collaborationHttp, '/health/ready', { timeoutMs: 800 }),
    );
    // 可达性超时与退出证据不同：原 PID/入口/owner 仍存在，不能借超时宣称已退出。
    assert.throws(
      () => new ProcessOwner(root).exitProof(original),
      /OLD_OUTPUT_PROCESS_STILL_RUNNING/,
    );
    const response = await post(
      '/v1/account/logout',
      { operation_id: revocation },
      fixture.owner.headers,
    );
    assert.ok([200, 202].includes(response.status));
    assert.equal(json(response).operation_id, revocation);
    for (const database of [account, doc]) {
      await until(async () => {
        const [[gate]] = await database.query('SELECT state FROM access_gate WHERE scope_key=?', [
          scope,
        ]);
        return gate?.state === 'BLOCKED';
      }, 'BLOCKED_NODE_AUTHORITY_GATE_REQUIRED');
      const [[closure]] = await database.query(
        'SELECT state FROM authorization_change WHERE revocation_id=?',
        [revocation],
      );
      assert.equal(closure?.state, 'FENCING');
    }
    const [[pending]] = await account.query(
      'SELECT COUNT(*) n FROM authorization_ack WHERE revocation_id=?',
      [revocation],
    );
    assert.equal(Number(pending.n), 0, '阻塞且未停止的 Node 不能产生完成 ACK');
    assert.equal((await get('/v1/account/me', fixture.owner.headers)).status, 401);
    await fault.release();
    fault = null;
    await until(async () => {
      const [[closure]] = await account.query(
        'SELECT state FROM authorization_change WHERE revocation_id=?',
        [revocation],
      );
      return closure?.state === 'SUCCEEDED';
    }, 'ORIGINAL_BLOCKED_CLOSE_DID_NOT_RESUME');
    const current = JSON.parse(readFileSync(resolve(runtime, 'processes.json'))).find(
      (item) => item.role === 'collaboration',
    );
    assert.equal(current.pid, original.pid);
    assert.ok(current.owner === original.owner, 'ORIGINAL_PROCESS_REQUIRED');
    await assertClosureFacts(fixture.owner, opened, revocation);
    const progress = await get('/v1/account/operations/' + revocation, fixture.owner.headers);
    assert.equal(progress.status, 200);
    assert.equal(json(progress).state, 'SUCCEEDED');
    assert.equal(json(progress).operation_id, revocation);
  } finally {
    await fault?.release();
    opened.close();
    await Promise.all([account.end(), doc.end()]);
  }
});
