import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, submitDraft } from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';

test(
  '固定审核批注支持幂等、分页、作者修订与墓碑且不解除正式阻止意见',
  { timeout: 60000 },
  async () => {
    const f = await reviewFixture(1);
    const member = f.f.members[0];
    try {
      await writeDraft(f, '固定正文：中文讨论 🧪🙂，批注不能改变正式审核要求。');
      const submitted = (await submitDraft(f)).value;
      assert.equal(submitted.state, 'SUCCEEDED');
      const fixed = submitted.result;
      const threadPath = `/v1/documents/${f.d.id}/comment-threads`;
      const reviewPath = `/v1/review-requests/${fixed.request_id}`;
      const input = {
        operation_id: operation('comment-thread'),
        context_kind: 'REVIEW',
        context_id: fixed.round_id,
        anchor: { schema_version: 'anchor-v1', content_version_id: fixed.content_version_id },
        body: '整份固定版本的讨论 🧪',
        mentions: [],
      };
      const created = await post(threadPath, input, f.user.headers);
      assert.equal(created.status, 201, created.body);
      assert.equal(json(created).state, 'SUCCEEDED');
      const thread = json(created).result.entity_id;
      const replay = await post(threadPath, input, f.user.headers);
      assert.equal(replay.status, 201, replay.body);
      assert.deepEqual(json(replay), json(created));
      const threads = await get(threadPath + '?limit=1', f.user.headers);
      assert.equal(threads.status, 200, threads.body);
      assert.deepEqual(
        json(threads).items.map((item) => item.id),
        [thread],
      );
      assert.equal(json(threads).has_more, false, '创建原号重放不能复制线程');
      assert.deepEqual(json(threads).items[0].anchor, input.anchor);

      // 上下文筛选必须先于分页；其它上下文不能占满本页，游标也不能移用。
      const otherContext = await get(threadPath + '?limit=1&context_kind=MAIN', f.user.headers);
      assert.equal(otherContext.status, 200, otherContext.body);
      assert.deepEqual(json(otherContext).items, []);
      const another = await post(
        threadPath,
        { ...input, operation_id: operation('comment-filter-page'), body: '同一审核的第二条讨论' },
        f.user.headers,
      );
      assert.equal(another.status, 201, another.body);
      const filtered = await get(
        threadPath + '?limit=1&context_kind=REVIEW&context_id=' + fixed.round_id,
        f.user.headers,
      );
      assert.equal(filtered.status, 200, filtered.body);
      assert.ok(json(filtered).next_cursor);
      const changedFilter = await get(
        threadPath +
          '?limit=1&context_kind=MAIN&cursor=' +
          encodeURIComponent(json(filtered).next_cursor),
        f.user.headers,
      );
      assert.equal(changedFilter.status, 409, changedFilter.body);
      const removedFilterFixture = await post(
        '/v1/comment-threads/' + json(another).result.entity_id + '/state',
        {
          operation_id: operation('remove-filter-fixture'),
          expected_revision: '1',
          state: 'DELETED',
        },
        f.user.headers,
      );
      assert.equal(removedFilterFixture.status, 200, removedFilterFixture.body);

      const commentsPath = `/v1/comment-threads/${thread}/comments`;
      const initial = await get(commentsPath + '?limit=1', f.user.headers);
      assert.equal(initial.status, 200, initial.body);
      const first = json(initial).items[0];
      assert.equal(first.body, input.body);
      assert.equal(json(initial).has_more, false, '创建原号重放不能复制首条评论');
      const replyInput = {
        operation_id: operation('comment-reply'),
        body: '回复中文🙂并保留原评论关系',
        reply_to_id: first.id,
        mentions: [],
      };
      const replied = await post(commentsPath, replyInput, f.user.headers);
      assert.equal(replied.status, 201, replied.body);
      const reply = json(replied).result.entity_id;
      const page = await get(commentsPath + '?limit=1', f.user.headers);
      assert.equal(page.status, 200, page.body);
      assert.equal(json(page).items[0].id, first.id);
      assert.ok(json(page).has_more && json(page).next_cursor);
      const next = await get(
        commentsPath + '?limit=1&cursor=' + encodeURIComponent(json(page).next_cursor),
        f.user.headers,
      );
      assert.equal(next.status, 200, next.body);
      assert.equal(json(next).items[0].id, reply);
      assert.equal(json(next).items[0].body, replyInput.body);
      assert.equal(json(next).has_more, false);

      const edit = {
        operation_id: operation('comment-edit'),
        expected_revision: first.revision,
        body: '作者修订后的中文🙂',
      };
      const edited = await post(`/v1/comments/${first.id}`, edit, f.user.headers, 'PATCH');
      assert.equal(edited.status, 200, edited.body);
      const stale = await post(
        `/v1/comments/${first.id}`,
        { ...edit, operation_id: operation('comment-stale'), body: '旧版本不应覆盖' },
        f.user.headers,
        'PATCH',
      );
      assert.equal(stale.status, 409, stale.body);
      assert.equal(json(stale).code, 'COMMENT_REVISION_CHANGED');

      // 正式 REQUEST_CHANGES 与普通讨论是不同业务事实；解决讨论不得解除审核阻止。
      const change = await post(
        reviewPath + '/actions',
        {
          operation_id: operation('comment-formal-change'),
          round_id: fixed.round_id,
          content_version_id: fixed.content_version_id,
          type: 'REQUEST_CHANGES',
          reason: '正式审核要求补充证据',
        },
        f.user.headers,
      );
      assert.equal(change.status, 200, change.body);
      const statePath = `/v1/comment-threads/${thread}/state`;
      const resolved = await post(
        statePath,
        { operation_id: operation('thread-resolve'), expected_revision: '1', state: 'RESOLVED' },
        f.user.headers,
      );
      assert.equal(resolved.status, 200, resolved.body);
      const resolvedThreads = await get(threadPath, f.user.headers);
      assert.equal(resolvedThreads.status, 200, resolvedThreads.body);
      assert.equal(json(resolvedThreads).items[0].status, 'RESOLVED');
      const changes = await get(reviewPath + '/change-requests', f.user.headers);
      assert.equal(changes.status, 200, changes.body);
      assert.equal(json(changes).items.length, 1);
      assert.equal(json(changes).items[0].status, 'OPEN');
      assert.equal(json(changes).items[0].reason, '正式审核要求补充证据');
      const reopened = await post(
        statePath,
        { operation_id: operation('thread-reopen'), expected_revision: '2', state: 'OPEN' },
        f.user.headers,
      );
      assert.equal(reopened.status, 200, reopened.body);
      const removed = await post(
        `/v1/comments/${first.id}`,
        { operation_id: operation('comment-delete'), expected_revision: '2' },
        f.user.headers,
        'DELETE',
      );
      assert.equal(removed.status, 200, removed.body);
      const tombstones = await get(commentsPath, f.user.headers);
      assert.equal(tombstones.status, 200, tombstones.body);
      assert.deepEqual(
        json(tombstones).items.map((item) => item.id),
        [first.id, reply],
      );
      assert.equal(json(tombstones).items[0].body, '');
      assert.equal(json(tombstones).items[1].body, replyInput.body);

      // 仅单元 EDIT 不授予普通批注写权；真实授予文档 COMMENT 后同一用户方可创建。
      const grant = async (scopeKind, scopeID, capability) => {
        const document = await get(`/v1/documents/${f.d.id}`, f.user.headers);
        assert.equal(document.status, 200, document.body);
        const result = await waitAuthorization(
          await post(
            `/v1/documents/${f.d.id}/authorization-changes`,
            {
              operation_id: operation('comment-grant'),
              expected_acl_revision: json(document).acl_revision,
              changes: [
                {
                  action: 'GRANT',
                  grant: {
                    user_id: member.id,
                    scope_kind: scopeKind,
                    scope_id: scopeID,
                    capability,
                  },
                },
              ],
            },
            f.user.headers,
          ),
          f.user,
        );
        assert.equal(result.state, 'SUCCEEDED');
      };
      await grant('UNIT', f.d.main.units[0].unit_id, 'EDIT');
      const denied = await post(
        threadPath,
        { ...input, operation_id: operation('edit-only-comment') },
        member.headers,
      );
      assert.equal(denied.status, 403, denied.body);
      await grant('DOCUMENT', f.d.id, 'COMMENT');
      const permitted = await post(
        threadPath,
        { ...input, operation_id: operation('member-comment') },
        member.headers,
      );
      assert.equal(permitted.status, 201, permitted.body);
      const firstThreads = await get(threadPath + '?limit=1', f.user.headers);
      assert.equal(firstThreads.status, 200, firstThreads.body);
      assert.equal(json(firstThreads).items[0].status, 'OPEN');
      assert.ok(json(firstThreads).has_more && json(firstThreads).next_cursor);
      const secondThreads = await get(
        threadPath + '?limit=1&cursor=' + encodeURIComponent(json(firstThreads).next_cursor),
        f.user.headers,
      );
      assert.equal(secondThreads.status, 200, secondThreads.body);
      assert.equal(json(secondThreads).items[0].id, json(permitted).result.entity_id);
      assert.equal(json(secondThreads).has_more, false);
    } finally {
      f.close();
    }
  },
);
