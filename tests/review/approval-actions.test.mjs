import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraft,
  submitDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { pollMerge } from '../helpers/merge.mjs';

async function grant(f, user, capabilities, action = 'GRANT') {
  const current = json(await get(`/v1/documents/${f.d.id}`, f.f.owner.headers));
  await waitAuthorization(
    await post(
      `/v1/documents/${f.d.id}/authorization-changes`,
      {
        operation_id: operation('approval-actions-acl'),
        expected_acl_revision: current.acl_revision,
        changes: capabilities.map((capability) => ({
          action,
          grant: {
            user_id: user.id,
            scope_kind: capability === 'EDIT' ? 'UNIT' : 'DOCUMENT',
            scope_id: capability === 'EDIT' ? f.grant.context.target.unit_id : f.d.id,
            capability,
          },
        })),
      },
      f.f.owner.headers,
    ),
    f.f.owner,
  );
}

async function advertised(f, request, user, expected) {
  const detail = await get('/v1/review-requests/' + request, user.headers);
  assert.equal(detail.status, 200, json(detail).code);
  assert.equal(
    json(detail).actions.includes('APPROVE_AND_MERGE'),
    expected,
    '详情必须返回当前真实合并资格',
  );
  const list = await get(`/v1/documents/${f.d.id}/review-requests`, user.headers);
  assert.equal(list.status, 200, json(list).code);
  const item = json(list).items.find((x) => x.id === request);
  assert.ok(item, '列表必须包含实际可读请求');
  assert.equal(
    item.actions.includes('APPROVE_AND_MERGE'),
    expected,
    '列表和详情必须使用相同全部资格事实',
  );
}

async function changes(fixed, user) {
  const made = await post(
    `/v1/review-requests/${fixed.request_id}/actions`,
    {
      operation_id: operation('approval-own-change'),
      round_id: fixed.round_id,
      content_version_id: fixed.content_version_id,
      type: 'REQUEST_CHANGES',
      reason: '明确处理后才能通过',
    },
    user.headers,
  );
  assert.equal(made.status, 200, json(made).code);
  return json(made).result.entity_id;
}

async function withdraw(fixed, user) {
  const response = await post(
    `/v1/review-requests/${fixed.request_id}/withdrawals`,
    {
      operation_id: operation('approval-withdraw'),
      expected_round_id: fixed.round_id,
    },
    user.headers,
  );
  assert.equal(response.status, 200, json(response).code);
}

// 权限提示须和真实批准门槛一致：本人意见可明确一并处理，管理者也不能借此处理别人的意见。
test('approval actions reflect manager exception, complete opinions, withdrawal and the actual merged state', async () => {
  const f = await reviewFixture(2),
    db = await reviewDatabase();
  let next;
  try {
    await writeDraft(f, '固定正文资格与真实合并保持一致');
    const fixed = (await submitDraft(f)).value.result,
      [reviewer, other] = f.f.members;
    await advertised(f, fixed.request_id, f.user, true);
    await grant(f, reviewer, ['REVIEW']);
    await grant(f, other, ['REVIEW']);
    await advertised(f, fixed.request_id, reviewer, true);
    const own = await changes(fixed, reviewer);
    await advertised(f, fixed.request_id, reviewer, true);
    await advertised(f, fixed.request_id, f.user, false);
    const foreign = await changes(fixed, other);
    await advertised(f, fixed.request_id, reviewer, false);
    for (const id of [foreign, own]) {
      const resolved = await post(
        `/v1/review-requests/${fixed.request_id}/change-requests/${id}/resolve`,
        {
          operation_id: operation('approval-resolve'),
          expected_round_id: fixed.round_id,
          expected_change_revision: '1',
          reason: '管理者逐条核对后解除这项要求',
        },
        f.user.headers,
      );
      assert.equal(resolved.status, 200, json(resolved).code);
      if (id === foreign) await advertised(f, fixed.request_id, reviewer, true);
    }
    await advertised(f, fixed.request_id, f.user, true);
    await withdraw(fixed, f.user);
    await advertised(f, fixed.request_id, f.user, false);
    f.close();
    next = await openReviewWorkspace(f.f, f.d);
    const resubmitted = (
      await submitDraft(next, {
        mode: 'RESUBMIT',
        request_id: fixed.request_id,
        expected_round_id: fixed.round_id,
      })
    ).value.result;
    await advertised(f, fixed.request_id, f.user, true);
    const input = {
      operation_id: operation('approval-actions-merge'),
      round_id: resubmitted.round_id,
      content_version_id: resubmitted.content_version_id,
      own_changes_to_resolve: [],
    };
    assert.equal(
      (
        await post(
          `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
          input,
          f.user.headers,
        )
      ).status,
      202,
    );
    assert.equal((await pollMerge(input.operation_id, f.user.headers, db)).state, 'SUCCEEDED');
    await advertised(f, fixed.request_id, f.user, false);
    await advertised(f, fixed.request_id, reviewer, false);
  } finally {
    next?.close();
    f.close();
    await db.end();
  }
});

// 第一页和当前轮次都由管理者提交，普通成员的历史身份藏在第二页，不能据局部轮次猜资格。
test('approval actions use all actual submitters even when the member round is outside the first history page', async () => {
  const f = await reviewFixture(2);
  let memberWorkspace, next;
  try {
    await writeDraft(f, '管理者第一轮');
    const first = (await submitDraft(f)).value.result,
      [member, reviewer] = f.f.members;
    await grant(f, member, ['EDIT', 'REVIEW']);
    await grant(f, reviewer, ['REVIEW']);
    await withdraw(first, f.user);
    memberWorkspace = await openReviewWorkspace(f.f, f.d, member);
    await writeDraft(memberWorkspace, '成员第二轮留下永久提交事实');
    const second = (
      await submitDraft(memberWorkspace, {
        mode: 'RESUBMIT',
        request_id: first.request_id,
        expected_round_id: first.round_id,
      })
    ).value.result;
    await withdraw(second, member);
    memberWorkspace.close();
    next = await openReviewWorkspace(f.f, f.d);
    await writeDraft(next, '管理者第三轮');
    const third = (
      await submitDraft(next, {
        mode: 'RESUBMIT',
        request_id: first.request_id,
        expected_round_id: second.round_id,
      })
    ).value.result;
    const history = json(
      await get(`/v1/review-requests/${first.request_id}/rounds?limit=1`, member.headers),
    );
    assert.equal(history.items[0].submitter_id, f.user.id);
    assert.ok(history.next_cursor);
    const detail = json(await get(`/v1/review-requests/${first.request_id}`, member.headers));
    assert.equal(detail.current_round.submitter_id, f.user.id);
    await advertised(f, first.request_id, member, false);
    await advertised(f, first.request_id, reviewer, true);
    await advertised(f, first.request_id, f.user, true);
    const denied = await post(
      `/v1/review-requests/${first.request_id}/approve-and-merge`,
      {
        operation_id: operation('historical-member-denied'),
        round_id: third.round_id,
        content_version_id: third.content_version_id,
        own_changes_to_resolve: [],
      },
      member.headers,
    );
    assert.equal(denied.status, 403);
    assert.equal(json(denied).code, 'MERGE_APPROVAL_NOT_ALLOWED');
    await grant(f, reviewer, ['REVIEW'], 'REVOKE');
    assert.equal(
      (await get(`/v1/review-requests/${first.request_id}`, reviewer.headers)).status,
      403,
    );
  } finally {
    memberWorkspace?.close();
    next?.close();
    f.close();
  }
});
