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
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

async function grant(f, user, capabilities) {
  const current = json(await get(`/v1/documents/${f.d.id}`, f.f.owner.headers));
  await waitAuthorization(
    await post(
      `/v1/documents/${f.d.id}/authorization-changes`,
      {
        operation_id: operation('merge-permissions'),
        expected_acl_revision: current.acl_revision,
        changes: capabilities.map((capability) => ({
          action: 'GRANT',
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

const mergeInput = (fixed) => ({
  operation_id: operation('permission-merge'),
  round_id: fixed.round_id,
  content_version_id: fixed.content_version_id,
  own_changes_to_resolve: [],
});

async function requestChange(fixed, user) {
  const response = await post(
    `/v1/review-requests/${fixed.request_id}/actions`,
    {
      operation_id: operation('merge-change'),
      round_id: fixed.round_id,
      content_version_id: fixed.content_version_id,
      type: 'REQUEST_CHANGES',
      reason: '合并前必须明确解除的意见',
    },
    user.headers,
  );
  assert.equal(response.status, 200, json(response).code);
  return { id: json(response).result.entity_id, revision: '1' };
}

// 新取得REVIEW不抹掉历次送审者身份；真实账号授权与送审记录共同决定是否允许批准。
test('a submitter later granted REVIEW cannot approve their own request', async () => {
  const f = await reviewFixture(1),
    db = await reviewDatabase();
  let memberWorkspace;
  try {
    const member = f.f.members[0];
    await grant(f, member, ['EDIT']);
    memberWorkspace = await openReviewWorkspace(f.f, f.d, member);
    await writeDraft(memberWorkspace, '成员送审后才获得审核权');
    const fixed = (await submitDraft(memberWorkspace)).value.result;
    await grant(f, member, ['REVIEW']);
    const path = `/v1/review-requests/${fixed.request_id}/approve-and-merge`;
    const deniedInput = mergeInput(fixed);
    const denied = await post(path, deniedInput, member.headers);
    assert.equal(denied.status, 403);
    assert.equal(json(denied).code, 'MERGE_APPROVAL_NOT_ALLOWED');
    const [[facts]] = await db.query(
      'SELECT COUNT(*) AS operations FROM merge_operation WHERE operation_id=?',
      [deniedInput.operation_id],
    );
    assert.equal(Number(facts.operations), 0);
    const managerInput = mergeInput(fixed);
    assert.equal((await post(path, managerInput, f.f.owner.headers)).status, 202);
    const result = await pollMerge(managerInput.operation_id, f.f.owner.headers, db);
    assert.equal(result.state, 'SUCCEEDED', result.error?.code);
  } finally {
    memberWorkspace?.close();
    f.close();
    await db.end();
  }
});

// 拟本人解除在最终合并事务才写动作。管理者也不能借这个字段解除其他作者的意见。
test('own resolutions use exact revisions and commit atomically with the successful merge', async () => {
  const f = await reviewFixture(1),
    db = await reviewDatabase();
  let interrupted = false;
  try {
    await writeDraft(f, '本人意见与批准必须同事务');
    const fixed = (await submitDraft(f)).value.result,
      reviewer = f.f.members[0];
    await grant(f, reviewer, ['REVIEW']);
    const path = `/v1/review-requests/${fixed.request_id}/approve-and-merge`;
    const own = await requestChange(fixed, reviewer),
      foreign = await requestChange(fixed, f.f.owner);
    for (const user of [reviewer, f.f.owner]) {
      const response = await post(
        path,
        { ...mergeInput(fixed), own_changes_to_resolve: [own, foreign] },
        user.headers,
      );
      assert.equal(response.status, 403, json(response).code);
      assert.equal(json(response).code, 'MERGE_RESOLUTION_NOT_OWN');
    }
    const foreignResolved = await post(
      `/v1/review-requests/${fixed.request_id}/change-requests/${foreign.id}/resolve`,
      {
        operation_id: operation('separate-owner-resolution'),
        expected_round_id: fixed.round_id,
        expected_change_revision: foreign.revision,
        reason: '管理者确认自己的意见已满足',
      },
      f.f.owner.headers,
    );
    assert.equal(foreignResolved.status, 200, json(foreignResolved).code);
    const blocked = await post(path, mergeInput(fixed), reviewer.headers);
    assert.equal(blocked.status, 409);
    assert.equal(json(blocked).code, 'CHANGES_PENDING');
    for (const revision of ['0', '999']) {
      const stale = await post(
        path,
        { ...mergeInput(fixed), own_changes_to_resolve: [{ ...own, revision }] },
        reviewer.headers,
      );
      assert.equal(stale.status, 409, json(stale).code);
      assert.equal(json(stale).code, 'CHANGE_REQUEST_CHANGED');
    }
    const duplicate = await post(
      path,
      { ...mergeInput(fixed), own_changes_to_resolve: [own, own] },
      reviewer.headers,
    );
    assert.equal(duplicate.status, 400);
    const requestPath = `/v1/review-requests/${fixed.request_id}`;
    assert.ok(json(await get(requestPath, reviewer.headers)).actions.includes('APPROVE_AND_MERGE'));
    f.close();
    stopOwnedRole('collaboration');
    interrupted = true;
    const input = { ...mergeInput(fixed), own_changes_to_resolve: [own] };
    const made = await post(path, input, reviewer.headers);
    assert.equal(made.status, 202, json(made).code);
    // 已登记原操作占位时只允许续接该号，不能由资格提示鼓励另开一个合并。
    assert.equal(
      json(await get(requestPath, reviewer.headers)).actions.includes('APPROVE_AND_MERGE'),
      false,
    );
    const [[before]] = await db.query(
      'SELECT status,(SELECT COUNT(*) FROM review_action WHERE operation_id=?) AS actions FROM change_request WHERE id=?',
      [input.operation_id, own.id],
    );
    assert.equal(before.status, 'OPEN');
    assert.equal(Number(before.actions), 0);
    restartOwnedRuntime();
    interrupted = false;
    const result = await pollMerge(input.operation_id, reviewer.headers, db);
    assert.equal(result.state, 'SUCCEEDED', result.error?.code);
    const [actions] = await db.query(
      'SELECT kind,item_no,target_change_id FROM review_action WHERE operation_id=? ORDER BY item_no',
      [input.operation_id],
    );
    assert.deepEqual(
      actions.map((row) => [row.kind, Number(row.item_no)]),
      [
        ['APPROVE_AND_MERGE', 0],
        ['RESOLVE_CHANGES', 1],
      ],
    );
    assert.equal(String(actions[1].target_change_id), own.id);
    const [[after]] = await db.query('SELECT status FROM change_request WHERE id=?', [own.id]);
    assert.equal(after.status, 'RESOLVED');
  } finally {
    f.close();
    await db.end();
    if (interrupted) restartOwnedRuntime();
  }
});
