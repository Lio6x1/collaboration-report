import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import {
  reviewFixture,
  reviewDatabase,
  writeDraft,
  flushDraft,
  captureInput,
  collaborationRPC,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { canonical } from '../../packages/content/index.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { wire } from '../helpers/wss.mjs';
import * as Y from 'yjs';

const digest = (value) => createHash('sha256').update(canonical(value)).digest();
test('an actual document-wide main barrier blocks a late MAIN room while BRANCH still saves', async () => {
  const f = await reviewFixture(),
    peer = collaborationRPC();
  const db = await reviewDatabase();
  let main;
  const intent = operation('main-merge');
  try {
    // 仅此新建测试文档解除默认锁，验证Node门闩；不把私有夹具当作已实现的主线锁API。
    await db.query('UPDATE document SET main_locked=FALSE WHERE id=?', [f.d.id]);
    const bound = await captureInput(f, await flushDraft(f));
    const control = json(await get('/v1/documents/' + f.d.id, f.user.headers)).control_revision;
    const input = {
      context: {
        ...bound.context,
        operation_id: intent,
        target: { ...bound.context.target, purpose: 'MERGE' },
      },
      barrier_generation: '1',
      expected_cut: {
        document_id: f.d.id,
        structure_id: f.d.main.structure.id,
        style_id: f.d.main.style.id,
        control_revision: control,
        rooms: [],
        dependency_digest: digest([]),
      },
    };
    const result = await rpcCall(peer, 'PrepareMainCommit', input);
    assert.equal(result.state, 'PREPARED');
    assert.deepEqual(result.prepared_cut.rooms, []);
    assert.deepEqual(await rpcCall(peer, 'PrepareMainCommit', input), result);
    await assert.rejects(
      () => rpcCall(peer, 'PrepareMainCommit', { ...input, barrier_generation: '2' }),
      (error) => error.code === 6,
    );
    assert.equal(
      (
        await rpcCall(peer, 'PrepareMainCommit', {
          ...input,
          context: { ...input.context, operation_id: operation('second-main-merge') },
        })
      ).state,
      'PROCESSING',
    );
    await writeDraft(f, '屏障期间支线仍可保存');
    const client = operation('main-client'),
      connection = operation('main-connection');
    const session = await post(
      `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/edit-sessions`,
      {
        operation_id: operation('main-session'),
        workspace_kind: 'MAIN',
        client_session_id: client,
        expected_structure_id: f.d.main.structure.id,
      },
      f.user.headers,
    );
    assert.equal(session.status, 200);
    main = wire(json(session), client, connection);
    // OPEN尚无已授权的输出绑定，运输协议直接关闭；不能绕过输出租约发送私有错误正文。
    await assert.rejects(main.ready, /SOCKET_CLOSED/);
    const node = await reviewDatabase('collaboration');
    try {
      const [[row]] = await node.query(
        "SELECT COUNT(*) AS n FROM room WHERE document_id=? AND workspace_kind='MAIN'",
        [f.d.id],
      );
      assert.equal(String(row.n), '0');
      const [[barrier]] = await node.query(
        'SELECT state,operation_id FROM main_barrier WHERE document_id=?',
        [f.d.id],
      );
      assert.equal(barrier.state, 'PREPARED');
      assert.equal(barrier.operation_id, intent);
    } finally {
      await node.end();
    }
  } finally {
    main?.close();
    peer.close();
    f.close();
    const node = await reviewDatabase('collaboration');
    try {
      await node.query('DELETE FROM main_barrier WHERE document_id=? AND operation_id=?', [
        f.d.id,
        intent,
      ]);
    } finally {
      await node.end();
    }
    await db.end();
  }
});

test('actual saved MAIN tail invalidates an old cut and a prepared barrier accepts no later update', async () => {
  const f = await reviewFixture(),
    peer = collaborationRPC(),
    db = await reviewDatabase();
  const intent = operation('tail-merge');
  let main;
  try {
    await db.query('UPDATE document SET main_locked=FALSE WHERE id=?', [f.d.id]);
    const client = operation('tail-client'),
      connection = operation('tail-connection');
    const made = await post(
      `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/edit-sessions`,
      {
        operation_id: operation('tail-session'),
        workspace_kind: 'MAIN',
        client_session_id: client,
        expected_structure_id: f.d.main.structure.id,
      },
      f.user.headers,
    );
    assert.equal(made.status, 200);
    const session = json(made),
      w = wire(session, client, connection),
      doc = new Y.Doc();
    main = { w, doc, session };
    const ready = await w.ready;
    Y.applyUpdate(doc, Buffer.from(ready.update, 'base64'));
    const saved = await writeDraft(main, '屏障准备前实际保存的主线');
    const bound = await captureInput(f, await flushDraft(f));
    const cut = {
      document_id: f.d.id,
      structure_id: f.d.main.structure.id,
      style_id: f.d.main.style.id,
      control_revision: json(await get('/v1/documents/' + f.d.id, f.user.headers)).control_revision,
      rooms: [
        { room_id: session.room_id, workspace_epoch: session.workspace_epoch, durable_seq: '0' },
      ],
    };
    const input = {
      context: {
        ...bound.context,
        operation_id: intent,
        target: { ...bound.context.target, purpose: 'MERGE' },
      },
      barrier_generation: '1',
      expected_cut: { ...cut, dependency_digest: digest(cut.rooms) },
    };
    const changed = await rpcCall(peer, 'PrepareMainCommit', input);
    assert.equal(changed.state, 'CHANGED');
    assert.equal(changed.prepared_cut.rooms[0].durable_seq, saved.durable_seq);
    const rooms = [{ ...cut.rooms[0], durable_seq: saved.durable_seq }];
    input.expected_cut = { ...cut, rooms, dependency_digest: digest(rooms) };
    assert.equal((await rpcCall(peer, 'PrepareMainCommit', input)).state, 'PREPARED');
    // 已有MAIN连接也不能发SAVED；授权错误正文发送同样受门闩限制，因此连接关闭且日志不增加。
    await assert.rejects(() => writeDraft(main, '屏障后不准保存'), /SOCKET_CLOSED/);
    const node = await reviewDatabase('collaboration');
    try {
      const [[row]] = await node.query('SELECT last_durable_seq FROM room WHERE id=?', [
        session.room_id,
      ]);
      assert.equal(row.last_durable_seq, saved.durable_seq);
    } finally {
      await node.end();
    }
    await writeDraft(f, '已有主线屏障期间支线仍保存');
  } finally {
    main?.w.close();
    main?.doc.destroy();
    peer.close();
    f.close();
    const node = await reviewDatabase('collaboration');
    try {
      await node.query('DELETE FROM main_barrier WHERE document_id=? AND operation_id=?', [
        f.d.id,
        intent,
      ]);
    } finally {
      await node.end();
    }
    await db.end();
  }
});
