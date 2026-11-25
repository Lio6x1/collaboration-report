import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import test from 'node:test';
import * as Y from 'yjs';
import { organizationFixture, documentFixture, post, operation } from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';
import { openReviewWorkspace, reviewDatabase } from '../helpers/review.mjs';
import {
  sessionId,
  loginAgain,
  until,
  assertClosureFacts,
} from '../helpers/authorization-fence.mjs';

function candidate(opened, text) {
  const before = Y.encodeStateVector(opened.doc),
    paragraph = opened.doc.getXmlFragment('prosemirror').get(0);
  let node = paragraph.get(0);
  if (!node) {
    node = new Y.XmlText();
    paragraph.push([node]);
  }
  node.insert(0, text);
  const bytes = Buffer.from(Y.encodeStateAsUpdate(opened.doc, before));
  return {
    bytes,
    hash: createHash('sha256').update(bytes).digest(),
    input: {
      type: 'UPDATE',
      client_update_id: operation('ordered-update'),
      update: bytes.toString('base64'),
    },
  };
}

async function pair() {
  const fixture = await organizationFixture(0),
    document = await documentFixture(fixture);
  const independent = await loginAgain(fixture.owner);
  // 旁观会话先登记：输出广播先到 B，才能用 B 的真实 grant 锁阻挡发送而不挡住 A 的原写事务。
  const b = await openReviewWorkspace(fixture, document, independent);
  const a = await openReviewWorkspace(fixture, document);
  return { fixture, document, independent, a, b };
}

test('product write order: durable UPDATE commits first, closure keeps exact log but never sends old SAVED', async () => {
  const f = await pair(),
    locker = await reviewDatabase('collaboration'),
    observer = await reviewDatabase('collaboration');
  const authority = await reviewDatabase('account');
  const update = candidate(f.a, '可靠写入先提交，关闭后仍保留完整原日志');
  const scope = 'session:' + sessionId(f.fixture.owner).toString('hex');
  let closing;
  try {
    await locker.beginTransaction();
    await locker.query('SELECT grant_id FROM collaboration_grant WHERE grant_id=? FOR UPDATE', [
      f.b.grant.grant_id,
    ]);
    f.a.w.send(update.input);
    // 只读另一连接看到完整日志才发关闭，不以客户端 send 回调或固定毫秒猜测 COMMIT。
    const committed = await until(async () => {
      const [[row]] = await observer.query(
        'SELECT seq,payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?',
        [
          f.a.session.room_id,
          f.a.session.workspace_epoch,
          f.a.session.client_session_id ?? f.a.grant.client_session_id,
          update.input.client_update_id,
        ],
      );
      return row;
    }, 'ORIGINAL_UPDATE_NOT_COMMITTED');
    assert.equal(String(committed.seq), '1');
    assert.equal(committed.payload.equals(update.bytes), true);
    assert.equal(committed.payload_hash.equals(update.hash), true);
    const revocation = operation('logout');
    closing = post('/v1/account/logout', { operation_id: revocation }, f.fixture.owner.headers);
    await until(async () => {
      const [[gate]] = await authority.query('SELECT state FROM access_gate WHERE scope_key=?', [
        scope,
      ]);
      return gate?.state === 'BLOCKED';
    }, 'SOURCE_CLOSURE_NOT_COMMITTED');
    // Account开始关闭与Node本地禁止旧输出不是同一时刻。只有原Node来源已关闭，
    // 才释放B的广播锁并判断A是否违规补发SAVED；不能用202/上游门闩替代本地关闭点。
    await until(async () => {
      const [[gate]] = await observer.query('SELECT state FROM access_gate WHERE scope_key=?', [
        scope,
      ]);
      return gate?.state === 'BLOCKED';
    }, 'NODE_OUTPUT_SOURCE_NOT_CLOSED');
    // 及时释放 B 的真实锁，保持全部原 SQL/RPC 期限。
    await locker.rollback();
    await waitOperation(await closing, f.fixture.owner);
    const frame = await f.b.w.next('UPDATE');
    assert.equal(frame.client_update_id, update.input.client_update_id);
    assert.equal(frame.durable_seq, '1');
    assert.ok(frame.update_hash === update.hash.toString('hex'), 'ORIGINAL_UPDATE_HASH_REQUIRED');
    const received = Buffer.from(frame.update, 'base64');
    assert.equal(received.equals(update.bytes), true);
    Y.applyUpdate(f.b.doc, received);
    assert.equal(
      f.b.doc.getXmlFragment('prosemirror').toString(),
      f.a.doc.getXmlFragment('prosemirror').toString(),
    );
    await assertClosureFacts(f.fixture.owner, f.a, revocation);
    assert.equal(
      f.a.w.messages.some((message) => message.type === 'SAVED'),
      false,
    );
    const [[count]] = await observer.query(
      'SELECT COUNT(*) n FROM update_log WHERE room_id=? AND epoch=? AND client_update_id=?',
      [f.a.session.room_id, f.a.session.workspace_epoch, update.input.client_update_id],
    );
    assert.equal(Number(count.n), 1);
  } finally {
    await locker.rollback();
    if (closing) await closing.catch(() => {});
    f.a.close();
    f.b.close();
    await Promise.all([locker.end(), observer.end(), authority.end()]);
  }
});

test('product write order: actual old frame arrives before close, authoritative source closes first and no log/SAVED/SYNC pollution follows', async () => {
  const f = await pair(),
    locker = await reviewDatabase('collaboration'),
    observer = await reviewDatabase('collaboration');
  const authority = await reviewDatabase('document');
  const update = candidate(f.a, '旧帧进入处理后，权威关闭先提交则拒绝保护和可靠写入');
  const scope = 'session:' + sessionId(f.fixture.owner).toString('hex');
  let closing, fresh;
  try {
    await locker.beginTransaction();
    await locker.query('SELECT grant_id FROM collaboration_grant WHERE grant_id=? FOR UPDATE', [
      f.a.grant.grant_id,
    ]);
    f.a.w.send(update.input);
    // 实际 UPDATE 先锁 Node source 再等待本用例 grant。NOWAIT 冲突证明帧已进入用例，
    // 不能拿已关闭 socket 上的 send 或客户端队列当服务器收到旧帧的证据。
    await until(async () => {
      await observer.beginTransaction();
      try {
        await observer.query(
          'SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE NOWAIT',
          [scope],
        );
        return false;
      } catch (error) {
        if (error.code === 'ER_LOCK_NOWAIT') return true;
        throw error;
      } finally {
        await observer.rollback();
      }
    }, 'OLD_FRAME_DID_NOT_REACH_SOURCE_LOCK');
    const revocation = operation('logout');
    closing = post('/v1/account/logout', { operation_id: revocation }, f.fixture.owner.headers);
    await until(async () => {
      const [[gate]] = await authority.query('SELECT state FROM access_gate WHERE scope_key=?', [
        scope,
      ]);
      return gate?.state === 'BLOCKED';
    }, 'AUTHORITATIVE_CLOSURE_NOT_COMMITTED');
    // Doc 权威门闩提交在前；释放后 Node 的资源保护必须拒绝，原 UPDATE 事务回滚。
    await locker.rollback();
    await waitOperation(await closing, f.fixture.owner);
    await assertClosureFacts(f.fixture.owner, f.a, revocation);
    const [[count]] = await observer.query(
      'SELECT COUNT(*) n FROM update_log WHERE room_id=? AND epoch=? AND client_update_id=?',
      [f.a.session.room_id, f.a.session.workspace_epoch, update.input.client_update_id],
    );
    assert.equal(Number(count.n), 0);
    assert.equal(
      f.a.w.messages.some((message) => message.type === 'SAVED'),
      false,
    );
    assert.equal(
      f.b.w.messages.some((message) => message.type === 'UPDATE'),
      false,
    );
    fresh = await openReviewWorkspace(f.fixture, f.document, f.independent);
    assert.equal(
      fresh.doc.getXmlFragment('prosemirror').toString(),
      f.b.doc.getXmlFragment('prosemirror').toString(),
    );
    const [[room]] = await observer.query('SELECT last_durable_seq FROM room WHERE id=?', [
      f.a.session.room_id,
    ]);
    assert.equal(String(room.last_durable_seq), '0');
  } finally {
    await locker.rollback();
    if (closing) await closing.catch(() => {});
    fresh?.close();
    f.a.close();
    f.b.close();
    await Promise.all([locker.end(), observer.end(), authority.end()]);
  }
});
