import test from 'node:test';
import assert from 'node:assert/strict';
import * as Y from 'yjs';
import { reviewFixture, writeDraft, submitDraft } from '../helpers/review.mjs';
import { wire } from '../helpers/wss.mjs';
import { completedMainControl as completed } from '../helpers/main-controls.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';

// 解锁是完整受权控制动作；普通成员不能开放MAIN输入，原号重试不能再次增加control。
test('main unlock requires current management and preserves the original control result', async () => {
  const f = await organizationFixture(1),
    d = await documentFixture(f);
  const current = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
  assert.equal(current.main_locked, true);
  const path = `/v1/documents/${d.id}/main-lock`,
    input = {
      operation_id: operation('main-unlock'),
      expected_control_revision: current.control_revision,
      locked: false,
    };
  const denied = await post(path, input, f.members[0].headers);
  assert.equal(denied.status, 403, json(denied).code);
  const result = await completed(await post(path, input, f.owner.headers), f.owner);
  assert.equal(result.state, 'SUCCEEDED', result.error?.code);
  assert.equal(result.result.main_sync_state, 'READY');
  const changed = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
  assert.equal(changed.main_locked, false);
  assert.equal(BigInt(changed.control_revision), BigInt(current.control_revision) + 1n);
  const replay = await completed(await post(path, input, f.owner.headers), f.owner);
  assert.deepEqual(replay.result, result.result);
  assert.equal(
    json(await get(`/v1/documents/${d.id}`, f.owner.headers)).control_revision,
    changed.control_revision,
  );
  const stale = await post(
    path,
    { ...input, operation_id: operation('stale-main-lock'), locked: true },
    f.owner.headers,
  );
  assert.equal(stale.status, 409, json(stale).code);
  assert.equal(json(await get(`/v1/documents/${d.id}`, f.owner.headers)).main_locked, false);
});

// 新样式只能追加修订；审核者已经取得的轮次和读取快照必须保留原样式。
test('style control appends a revision and preserves submitted style history', async () => {
  const f = await reviewFixture();
  try {
    await writeDraft(f, '样式切换前固定送审正文');
    const submitted = (await submitDraft(f)).value.result;
    const locked = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await completed(
      await post(
        `/v1/documents/${f.d.id}/main-lock`,
        {
          operation_id: operation('style-unlock'),
          expected_control_revision: locked.control_revision,
          locked: false,
        },
        f.user.headers,
      ),
      f.user,
    );
    const before = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    const path = `/v1/documents/${f.d.id}/styles`;
    const input = {
      operation_id: operation('style-change'),
      expected_style_id: before.style.id,
      expected_control_revision: before.document.control_revision,
      style: { ...before.style.style, body_font: 'FangSong' },
    };
    const result = await completed(await post(path, input, f.user.headers), f.user);
    assert.equal(result.state, 'SUCCEEDED', result.error?.code);
    const after = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    assert.notEqual(after.style.id, before.style.id);
    assert.equal(after.style.style.body_font, 'FangSong');
    assert.equal(after.style.id, result.result.entity_id);
    const history = await get(
      `/v1/review-requests/${submitted.request_id}/rounds/${submitted.round_id}`,
      f.user.headers,
    );
    assert.equal(history.status, 200, json(history).code);
    assert.deepEqual(json(history).style, before.style);
    const stale = await post(
      path,
      { ...input, operation_id: operation('stale-style') },
      f.user.headers,
    );
    assert.equal(stale.status, 409, json(stale).code);
    assert.equal(
      json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers)).style.id,
      after.style.id,
    );
  } finally {
    f.close();
  }
});

// 冻结接口要求MAIN_UNLOCKED；管理资格不能在锁定状态直接绕过样式前置条件。
test('locked main rejects style changes before reserving an operation', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f);
  const before = json(await get(`/v1/documents/${d.id}/main`, f.owner.headers));
  const changed = await post(
    `/v1/documents/${d.id}/styles`,
    {
      operation_id: operation('locked-style'),
      expected_style_id: before.style.id,
      expected_control_revision: before.document.control_revision,
      style: { ...before.style.style, body_font: 'FangSong' },
    },
    f.owner.headers,
  );
  assert.equal(changed.status, 409, json(changed).code);
  assert.equal(json(changed).code, 'MAIN_LOCKED');
  assert.equal(
    json(await get(`/v1/documents/${d.id}/main`, f.owner.headers)).style.id,
    before.style.id,
  );
});

// 使用公开解锁入口和真实Yjs可靠尾部；锁定成功必须包含已SAVED正文，并关闭旧MAIN输出。
test('locking materializes reliable MAIN edits and leaves the concurrent branch intact', async () => {
  const f = await reviewFixture();
  let main;
  try {
    await writeDraft(f, '支线后续草稿必须保留');
    const control = async (locked) => {
      const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      return completed(
        await post(
          `/v1/documents/${f.d.id}/main-lock`,
          {
            operation_id: operation('main-control'),
            expected_control_revision: current.control_revision,
            locked,
          },
          f.user.headers,
        ),
        f.user,
      );
    };
    assert.equal((await control(false)).state, 'SUCCEEDED');
    const client = operation('main-client'),
      connection = operation('main-connection');
    const made = await post(
      `/v1/documents/${f.d.id}/units/${f.d.main.units[0].unit_id}/edit-sessions`,
      {
        operation_id: operation('main-edit'),
        workspace_kind: 'MAIN',
        client_session_id: client,
        expected_structure_id: f.d.main.structure.id,
      },
      f.user.headers,
    );
    assert.equal(made.status, 200, json(made).code);
    const session = json(made),
      w = wire(session, client, connection),
      doc = new Y.Doc();
    main = { session, w, doc };
    Y.applyUpdate(doc, Buffer.from((await w.ready).update, 'base64'));
    const saved = await writeDraft(main, '锁定前已可靠保存的主线正文');
    assert.ok(BigInt(saved.durable_seq) > 0n);
    const closed = new Promise((resolve) => w.socket.once('close', resolve));
    const locked = await control(true);
    assert.equal(locked.state, 'SUCCEEDED', locked.error?.code);
    await Promise.race([
      closed,
      new Promise((_, reject) => setTimeout(() => reject(Error('OLD_MAIN_OUTPUT_ALIVE')), 5000)),
    ]);
    const after = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    assert.equal(after.document.main_locked, true);
    const body = await get(
      `/v1/documents/${f.d.id}/read-views/${after.read_view_id}/units/${after.units[0].unit_id}`,
      f.user.headers,
    );
    assert.equal(body.status, 200, json(body).code);
    assert.match(JSON.stringify(json(body).body_json), /锁定前已可靠保存的主线正文/);
    const denied = await post(
      `/v1/documents/${f.d.id}/units/${f.d.main.units[0].unit_id}/edit-sessions`,
      {
        operation_id: operation('locked-main-edit'),
        workspace_kind: 'MAIN',
        client_session_id: operation('denied-client'),
        expected_structure_id: f.d.main.structure.id,
      },
      f.user.headers,
    );
    assert.equal(denied.status, 409, json(denied).code);
    assert.equal(json(denied).code, 'MAIN_LOCKED');
    const submitted = (await submitDraft(f)).value.result;
    const round = json(
      await get(
        `/v1/review-requests/${submitted.request_id}/rounds/${submitted.round_id}`,
        f.user.headers,
      ),
    );
    assert.match(JSON.stringify(round.content.body_json), /支线后续草稿必须保留/);
    assert.doesNotMatch(JSON.stringify(round.content.body_json), /锁定前已可靠保存/);
  } finally {
    main?.w.close();
    main?.doc.destroy();
    f.close();
  }
});
