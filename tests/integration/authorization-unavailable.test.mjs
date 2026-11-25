import assert from 'node:assert/strict';
import test from 'node:test';
import {
  organizationFixture,
  documentFixture,
  post,
  get,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { openReviewWorkspace, reviewDatabase } from '../helpers/review.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import {
  sessionId,
  loginAgain,
  until,
  assertClosureFacts,
} from '../helpers/authorization-fence.mjs';

test('product closure: actual unavailable Node leaves the original close processing, restart completes without a new POST', async () => {
  const fixture = await organizationFixture(0),
    document = await documentFixture(fixture);
  const independent = await loginAgain(fixture.owner),
    opened = await openReviewWorkspace(fixture, document);
  const account = await reviewDatabase('account'),
    doc = await reviewDatabase('document');
  const scope = 'session:' + sessionId(fixture.owner).toString('hex');
  const revocation = operation('logout');
  let recoveryNeeded = false;
  try {
    recoveryNeeded = true;
    // 只终止登记表中 PID/绝对入口/owner 三项实际匹配的本项目 Node，不按程序名批量终止。
    stopOwnedRole('collaboration');
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
      }, 'AUTHORITY_SOURCE_GATE_NOT_CLOSED');
    }
    const [[pending]] = await account.query(
      'SELECT state FROM authorization_change WHERE revocation_id=?',
      [revocation],
    );
    assert.equal(pending.state, 'FENCING');
    const [[ack]] = await account.query(
      'SELECT COUNT(*) n FROM authorization_ack WHERE revocation_id=?',
      [revocation],
    );
    assert.equal(Number(ack.n), 0, 'Node 不可达不能编造已经停止的 ACK');
    assert.equal((await get('/v1/account/me', fixture.owner.headers)).status, 401);
    assert.equal((await get('/v1/account/me', independent.headers)).status, 200);
    // 只恢复基础环境；原 actor 操作既不换号也不重 POST，由实际后台任务推进。
    restartOwnedRuntime();
    recoveryNeeded = false;
    await until(async () => {
      const [[row]] = await account.query(
        'SELECT state FROM authorization_change WHERE revocation_id=?',
        [revocation],
      );
      return row.state === 'SUCCEEDED';
    }, 'ORIGINAL_CLOSE_DID_NOT_AUTO_RESUME');
    await assertClosureFacts(fixture.owner, opened, revocation);
    const progress = await get('/v1/account/operations/' + revocation, fixture.owner.headers);
    assert.equal(progress.status, 200);
    assert.equal(json(progress).state, 'SUCCEEDED');
    assert.equal(json(progress).operation_id, revocation);
  } finally {
    opened.close();
    await Promise.all([account.end(), doc.end()]);
    if (recoveryNeeded) restartOwnedRuntime();
  }
});
