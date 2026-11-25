import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';

test('真实负责人转让通知仅本人可标读，分页稳定且撤权后隐藏原提醒', { timeout: 60000 }, async () => {
  const f = await organizationFixture(1);
  const member = f.members[0];
  const first = await documentFixture(f);
  const second = await documentFixture(f);
  async function transfer(document, target) {
    const current = await get(`/v1/documents/${document}`, f.owner.headers);
    assert.equal(current.status, 200, current.body);
    const response = await post(
      `/v1/documents/${document}/responsible-transfers`,
      {
        operation_id: operation('notification-transfer'),
        expected_revision: json(current).control_revision,
        target_user_id: target,
      },
      f.owner.headers,
    );
    assert.equal(response.status, 200, response.body);
    const result = await waitAuthorization(response, f.owner);
    assert.equal(result.state, 'SUCCEEDED');
  }
  await transfer(first.id, member.id);
  await transfer(second.id, member.id);

  // 两条通知均由真实负责人转让事务产生，不借数据库伪造收件记录。
  const page = await get('/v1/notifications?limit=1', member.headers);
  assert.equal(page.status, 200, page.body);
  assert.equal(json(page).items.length, 1);
  assert.ok(json(page).has_more && json(page).next_cursor);
  const next = await get(
    '/v1/notifications?limit=1&cursor=' + encodeURIComponent(json(page).next_cursor),
    member.headers,
  );
  assert.equal(next.status, 200, next.body);
  assert.equal(json(next).items.length, 1);
  assert.equal(json(next).has_more, false);
  const notifications = [...json(page).items, ...json(next).items];
  assert.equal(new Set(notifications.map((item) => item.id)).size, 2);
  assert.deepEqual(
    notifications.map((item) => item.target_id).sort(),
    [first.id, second.id].sort(),
  );
  for (const notification of notifications) {
    assert.equal(notification.kind, 'RESPONSIBLE_TRANSFER');
    assert.equal(notification.title, '你已成为文档负责人');
    assert.equal(notification.read_at, null);
  }
  const notice = notifications.find((item) => item.target_id === first.id);
  const readPath = `/v1/notifications/${notice.id}/read`;
  const denied = await post(readPath, { operation_id: operation('other-read') }, f.owner.headers);
  assert.equal(denied.status, 404, denied.body);

  const input = { operation_id: operation('notification-read') };
  const marked = await post(readPath, input, member.headers);
  assert.equal(marked.status, 200, marked.body);
  assert.equal(json(marked).state, 'SUCCEEDED');
  assert.equal(json(marked).result.entity_id, notice.id);
  const readList = await get('/v1/notifications', member.headers);
  assert.equal(readList.status, 200, readList.body);
  const readAt = json(readList).items.find((item) => item.id === notice.id).read_at;
  assert.ok(readAt && Number.isFinite(Date.parse(readAt)));
  const replay = await post(readPath, input, member.headers);
  assert.equal(replay.status, 200, replay.body);
  assert.deepEqual(json(replay), json(marked));
  const replayList = await get('/v1/notifications', member.headers);
  assert.equal(replayList.status, 200, replayList.body);
  assert.equal(json(replayList).items.find((item) => item.id === notice.id).read_at, readAt);

  // 原负责人失去文档资格后列表必须隐藏旧目标；已读操作仍只校验收件人本人。
  await transfer(first.id, f.owner.id);
  const forbiddenDocument = await get(`/v1/documents/${first.id}`, member.headers);
  assert.equal(forbiddenDocument.status, 403, forbiddenDocument.body);
  const hidden = await get('/v1/notifications', member.headers);
  assert.equal(hidden.status, 200, hidden.body);
  assert.deepEqual(
    json(hidden).items.map((item) => item.target_id),
    [second.id],
  );
  assert.equal(
    json(hidden).items.some((item) => item.id === notice.id),
    false,
  );
  const ownHiddenRead = await post(
    readPath,
    { operation_id: operation('hidden-notification-read') },
    member.headers,
  );
  assert.equal(ownHiddenRead.status, 200, ownHiddenRead.body);
  assert.equal(json(ownHiddenRead).state, 'SUCCEEDED');
  const stillHidden = await get('/v1/notifications', member.headers);
  assert.equal(stillHidden.status, 200, stillHidden.body);
  assert.equal(
    json(stillHidden).items.some((item) => item.id === notice.id),
    false,
  );
});
