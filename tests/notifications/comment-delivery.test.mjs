import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, submitDraft, reviewDatabase } from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';

async function until(read, accepts, code) {
  const deadline = Date.now() + 12000;
  for (;;) {
    const value = await read();
    if (accepts(value)) return value;
    assert.ok(Date.now() < deadline, code + ' ' + JSON.stringify(value));
    await new Promise((done) => setTimeout(done, 200));
  }
}

test('批注通知在身份依赖未知后可靠重试，精确去重收件且撤权后隐藏', { timeout: 60000 }, async () => {
  const f = await reviewFixture(2);
  let docDB, accountDB;
  const [member, outsider] = f.f.members;
  try {
    await writeDraft(f, '通知测试的真实固定审核正文');
    const submission = (await submitDraft(f)).value;
    assert.equal(submission.state, 'SUCCEEDED');
    const fixed = submission.result;
    const grant = {
      user_id: member.id,
      scope_kind: 'DOCUMENT',
      scope_id: f.d.id,
      capability: 'COMMENT',
    };
    async function permission(action) {
      const current = await get(`/v1/documents/${f.d.id}`, f.user.headers);
      assert.equal(current.status, 200, current.body);
      const value = await waitAuthorization(
        await post(
          `/v1/documents/${f.d.id}/authorization-changes`,
          {
            operation_id: operation('notification-comment-permission'),
            expected_acl_revision: json(current).acl_revision,
            changes: [{ action, grant }],
          },
          f.user.headers,
        ),
        f.user,
      );
      assert.equal(value.state, 'SUCCEEDED');
    }
    await permission('GRANT');
    docDB = await reviewDatabase();
    accountDB = await reviewDatabase('account');
    const input = {
      operation_id: operation('comment-mention-recovery'),
      context_kind: 'REVIEW',
      context_id: fixed.round_id,
      anchor: { schema_version: 'anchor-v1', content_version_id: fixed.content_version_id },
      body: '请核对这份固定内容，重复提及与无权成员不能多收通知。',
      mentions: [member.id, member.id, outsider.id, f.user.id],
    };
    const task = async (id) => {
      const [[row]] = await docDB.query(
        "SELECT state,attempts,last_error FROM durable_task WHERE kind='COMMENT_NOTIFICATION' AND operation_id=?",
        [id],
      );
      return row ?? null;
    };
    let created;
    // 只锁住真实候选用户行模拟跨服务资格读取超时，不伪造任务/通知或修改身份。
    // 评论自身提交不得依赖后台通知完成；未知资格只能重试，不能当成无资格永久跳过。
    await accountDB.beginTransaction();
    try {
      const [[locked]] = await accountDB.query('SELECT id FROM user WHERE id=? FOR UPDATE', [
        member.id,
      ]);
      assert.ok(locked);
      created = await post(`/v1/documents/${f.d.id}/comment-threads`, input, f.user.headers);
      assert.equal(created.status, 201, created.body);
      assert.equal(json(created).state, 'SUCCEEDED');
      const retry = await until(
        () => task(input.operation_id),
        (row) => row?.state === 'RETRY',
        'COMMENT_NOTIFICATION_NOT_RETRIED',
      );
      assert.ok(Number(retry.attempts) >= 1);
      assert.equal(retry.last_error, 'NOTIFICATION_DELIVERY_RETRY');
    } finally {
      await accountDB.rollback();
    }
    const thread = json(created).result.entity_id;
    const notifications = async (user) => {
      const response = await get('/v1/notifications', user.headers);
      assert.equal(response.status, 200, response.body);
      return json(response).items;
    };
    const received = await until(
      () => notifications(member),
      (items) => items.some((item) => item.event_id === input.operation_id),
      'COMMENT_MENTION_NOT_DELIVERED',
    );
    const mention = received.filter((item) => item.event_id === input.operation_id);
    assert.equal(mention.length, 1);
    assert.equal(mention[0].kind, 'COMMENT_MENTION');
    assert.equal(mention[0].target_id, thread);
    await until(
      () => task(input.operation_id),
      (row) => row?.state === 'SUCCEEDED',
      'COMMENT_TASK_NOT_FINISHED',
    );
    const [recipients] = await docDB.query(
      'SELECT CAST(recipient_id AS CHAR) recipient,kind FROM notification WHERE event_id=? ORDER BY recipient_id,kind',
      [input.operation_id],
    );
    assert.deepEqual(
      recipients.map((row) => ({ ...row })),
      [{ recipient: member.id, kind: 'COMMENT_MENTION' }],
    );
    assert.equal(
      (await notifications(outsider)).some((item) => item.event_id === input.operation_id),
      false,
    );

    const replay = await post(`/v1/documents/${f.d.id}/comment-threads`, input, f.user.headers);
    assert.equal(replay.status, 201, replay.body);
    assert.deepEqual(json(replay), json(created));
    assert.equal(
      (await notifications(member)).filter((item) => item.event_id === input.operation_id).length,
      1,
    );
    const [[counts]] = await docDB.query('SELECT COUNT(*) n FROM notification WHERE event_id=?', [
      input.operation_id,
    ]);
    assert.equal(Number(counts.n), 1);

    // 没有显式@仍通知原参与者；回复者本人不收到自己的回复提醒。
    const replyInput = {
      operation_id: operation('comment-reply-notice'),
      body: '已核对，回复原讨论。',
      mentions: [],
    };
    const reply = await post(`/v1/comment-threads/${thread}/comments`, replyInput, member.headers);
    assert.equal(reply.status, 201, reply.body);
    const ownerItems = await until(
      () => notifications(f.user),
      (items) => items.some((item) => item.event_id === replyInput.operation_id),
      'COMMENT_REPLY_NOT_DELIVERED',
    );
    const replyNotice = ownerItems.filter((item) => item.event_id === replyInput.operation_id);
    assert.equal(replyNotice.length, 1);
    assert.equal(replyNotice[0].kind, 'COMMENT_REPLY');
    assert.equal(replyNotice[0].target_id, thread);
    await until(
      () => task(replyInput.operation_id),
      (row) => row?.state === 'SUCCEEDED',
      'REPLY_TASK_NOT_FINISHED',
    );

    await permission('REVOKE');
    assert.equal(
      (await notifications(member)).some((item) => item.event_id === input.operation_id),
      false,
    );
  } finally {
    await accountDB?.end();
    await docDB?.end();
    f.close();
  }
});
