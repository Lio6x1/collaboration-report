import assert from 'node:assert/strict';
import test from 'node:test';
import { createHash } from 'node:crypto';
import { WebSocket } from 'ws';
import {
  organizationFixture,
  documentFixture,
  post,
  get,
  operation,
} from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';
import {
  openReviewWorkspace,
  reviewDatabase,
  writeDraft,
  collaborationRPC,
} from '../helpers/review.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { wire } from '../helpers/wss.mjs';

import {
  sessionId,
  loginAgain,
  until,
  assertClosureFacts,
  pendingTicket,
} from '../helpers/authorization-fence.mjs';

test('product closure: one of two actual login sessions closes, independent session keeps writing', async () => {
  const fixture = await organizationFixture(0),
    document = await documentFixture(fixture);
  const independent = await loginAgain(fixture.owner);
  assert.equal(sessionId(fixture.owner).equals(sessionId(independent)), false);
  const a = await openReviewWorkspace(fixture, document);
  const b = await openReviewWorkspace(fixture, document, independent);
  const pending = await pendingTicket(document, fixture.owner);
  let aClosed = false;
  a.w.socket.once('close', () => {
    aClosed = true;
  });
  let late;
  try {
    const revocation = operation('logout');
    const response = await post(
      '/v1/account/logout',
      { operation_id: revocation },
      fixture.owner.headers,
    );
    await waitOperation(response, fixture.owner);
    // 两个客户端连接的回调先后不能代表服务器提交顺序；ACK 顺序另用下方实际 delivery 锁验证。
    await until(() => aClosed, 'OLD_SOCKET_DID_NOT_CLOSE');
    await assertClosureFacts(fixture.owner, a, revocation);
    assert.equal(b.w.socket.readyState, WebSocket.OPEN);
    assert.equal((await get('/v1/account/me', independent.headers)).status, 200);
    const saved = await writeDraft(b, '另一实际登录会话仍可可靠保存');
    assert.equal(saved.durable_seq, '1');
    const fresh = await openReviewWorkspace(fixture, document, independent);
    fresh.close();
    assert.equal((await get('/v1/account/me', fixture.owner.headers)).status, 401);
    // 原未兑付票据由真实旧 session 签出；不能注销后再造一个无意义假票据。
    late = wire(pending.session, pending.client, operation('late-connection'));
    await assert.rejects(late.ready);
  } finally {
    late?.close();
    a.close();
    b.close();
  }
});

test('product closure: actual socket stops while delivery settlement is locked, no participant may ACK early', async () => {
  const fixture = await organizationFixture(0),
    document = await documentFixture(fixture);
  const opened = await openReviewWorkspace(fixture, document);
  const node = await reviewDatabase('collaboration'),
    observer = await reviewDatabase('collaboration');
  const account = await reviewDatabase('account');
  const peer = collaborationRPC();
  let closed = false;
  opened.w.socket.once('close', () => {
    closed = true;
  });
  try {
    const [[delivery]] = await observer.query(
      "SELECT delivery_id FROM delivery_session WHERE grant_id=? AND state='OPEN'",
      [opened.grant.grant_id],
    );
    assert.ok(delivery, 'ACTUAL_OPEN_DELIVERY_REQUIRED');
    await node.beginTransaction();
    await node.query('SELECT delivery_id FROM delivery_session WHERE delivery_id=? FOR UPDATE', [
      delivery.delivery_id,
    ]);
    const revocation = operation('logout');
    const response = await post(
      '/v1/account/logout',
      { operation_id: revocation },
      fixture.owner.headers,
    );
    await until(() => closed, 'ACTUAL_SOCKET_CLOSE_REQUIRED');
    // Edge 可以先关闭浏览器连接，随后 Doc 才登记 Node 的原关闭任务。
    // 等到实际 Node 已进入 FENCING 再检查 ACK，不能用客户端回调推断跨服务执行顺序。
    await until(async () => {
      const [[row]] = await observer.query(
        'SELECT state FROM authorization_change WHERE revocation_id=?',
        [revocation],
      );
      return row?.state === 'FENCING';
    }, 'NODE_ORIGINAL_CLOSE_NOT_REGISTERED');
    // 真实 close 已发生但原持久结算还被本用例行锁挡住；不写假的输出成功或任务状态。
    const [[pending]] = await observer.query(
      'SELECT state FROM delivery_session WHERE delivery_id=?',
      [delivery.delivery_id],
    );
    assert.notEqual(pending.state, 'SETTLED');
    const [[nodeClosure]] = await observer.query(
      'SELECT state FROM authorization_change WHERE revocation_id=?',
      [revocation],
    );
    assert.equal(nodeClosure?.state, 'FENCING');
    const [[nodeAck]] = await observer.query(
      'SELECT COUNT(*) n FROM authorization_ack WHERE revocation_id=?',
      [revocation],
    );
    assert.equal(Number(nodeAck.n), 0);
    const [[accountClosure]] = await account.query(
      'SELECT state FROM authorization_change WHERE revocation_id=?',
      [revocation],
    );
    assert.equal(accountClosure?.state, 'FENCING');
    const [[accountAck]] = await account.query(
      'SELECT COUNT(*) n FROM authorization_ack WHERE revocation_id=?',
      [revocation],
    );
    assert.equal(Number(accountAck.n), 0);
    const scope = 'session:' + sessionId(fixture.owner).toString('hex');
    const target = createHash('sha256')
      .update('SESSION_LOGOUT\n' + scope.slice(8))
      .digest();
    const original = {
      revocation_id: revocation,
      closed_scopes: [{ scope_key: scope, epoch: '1' }],
      generation: '1',
      target_digest: target,
    };
    await new Promise((done, reject) =>
      peer.waitForReady(Date.now() + 2500, (error) => (error ? reject(error) : done())),
    );
    // 原实际结算仍被锁住。客户端真实RPC期限结束只能得到未知，不能借deadline产生完成ACK。
    await assert.rejects(
      new Promise((done, reject) =>
        peer.FenceAccess(original, { deadline: Date.now() + 100 }, (error, value) =>
          error ? reject(error) : done(value),
        ),
      ),
      (error) => error.code === 4,
    );
    const [[unknownAck]] = await observer.query(
      'SELECT COUNT(*) n FROM authorization_ack WHERE revocation_id=?',
      [revocation],
    );
    assert.equal(Number(unknownAck.n), 0);
    await node.rollback();
    await waitOperation(response, fixture.owner);
    await assertClosureFacts(fixture.owner, opened, revocation);
    const result = await rpcCall(peer, 'GetFenceResult', original);
    const repeated = await rpcCall(peer, 'FenceAccess', original);
    assert.equal(result.state, 'ACKNOWLEDGED');
    assert.ok(result.revocation_id === revocation, 'ORIGINAL_UNKNOWN_RPC_RESULT_REQUIRED');
    assert.equal(result.generation, '1');
    assert.deepEqual(result.closed_scopes, original.closed_scopes);
    const expected = createHash('sha256')
      .update(
        [
          'bootstrap-fence-v1',
          'collaboration',
          revocation,
          scope,
          '1',
          '1',
          target.toString('hex'),
        ].join('\n'),
      )
      .digest();
    assert.ok(
      Buffer.from(result.result_digest).equals(expected),
      'FULL_ORIGINAL_UNKNOWN_ACK_REQUIRED',
    );
    assert.ok(Buffer.from(repeated.result_digest).equals(expected), 'SAME_CLOSE_EFFECT_REQUIRED');
  } finally {
    peer.close();
    await node.rollback();
    await Promise.all([node.end(), observer.end(), account.end()]);
    opened.close();
  }
});

test('product closure: future session deadline expires naturally and closes actual WSS with durable ACKs', async () => {
  const fixture = await organizationFixture(0),
    document = await documentFixture(fixture);
  const expiring = await loginAgain(fixture.owner),
    account = await reviewDatabase('account');
  const id = sessionId(expiring);
  let opened;
  try {
    // 只改变本用例尚未准入的新 session，期限固定在未来；不回填过去、不改时钟、不调用 logout。
    await account.query(
      "UPDATE session SET expires_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 8 SECOND) WHERE id=? AND state='ACTIVE'",
      [id],
    );
    const [[deadline]] = await account.query(
      'SELECT UNIX_TIMESTAMP(expires_at)*1000 deadline_ms,UNIX_TIMESTAMP(UTC_TIMESTAMP(6))*1000 now_ms FROM session WHERE id=?',
      [id],
    );
    assert.ok(Number(deadline.deadline_ms) > Number(deadline.now_ms));
    opened = await openReviewWorkspace(fixture, document, expiring);
    assert.ok(Number(opened.grant.expires_unix_ms) <= Number(deadline.deadline_ms));
    await until(
      async () => {
        const [[row]] = await account.query('SELECT state FROM session WHERE id=?', [id]);
        return row.state === 'CLOSED';
      },
      'NATURAL_SESSION_CLOSE_NOT_COMPLETED',
      12000,
    );
    assert.equal(opened.w.socket.readyState, WebSocket.CLOSED);
    await assertClosureFacts(expiring, opened, 'expiry-' + id.toString('hex'));
    // 独立长期会话仍然有效；自然样本本身只靠时间与后台恢复器完成。
    assert.equal((await get('/v1/account/me', fixture.owner.headers)).status, 200);
    assert.equal((await get('/v1/account/me', expiring.headers)).status, 401);
  } finally {
    opened?.close();
    await account.end();
  }
});
