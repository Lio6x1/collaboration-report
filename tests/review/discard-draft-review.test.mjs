import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  reviewDatabase,
  writeDraft,
  submitDraft,
  openReviewWorkspace,
  flushDraft,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import {
  discardPreviewPath,
  discardPreviewInput,
  discardDraftPath,
  discardDraftInput,
  pollDiscardDraft,
} from '../helpers/discard-previews.mjs';

test('discard requires withdrawal and exact opinions, then archives only after actual restoration', async () => {
  const f = await reviewFixture(1),
    db = await reviewDatabase();
  let next;
  try {
    await writeDraft(f, '带真实意见的旧草稿');
    const submitted = (await submitDraft(f)).value;
    assert.equal(submitted.state, 'SUCCEEDED');
    const review = submitted.result,
      path = `/v1/review-requests/${review.request_id}`;
    const changeInput = () => ({
      operation_id: operation('discard-change'),
      round_id: review.round_id,
      content_version_id: review.content_version_id,
      type: 'REQUEST_CHANGES',
      reason: '请补齐真实证据',
    });
    assert.equal((await post(`${path}/actions`, changeInput(), f.user.headers)).status, 200);
    const original = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
    assert.equal(original.status, 200, original.body);
    const preview = json(original),
      input = { ...discardDraftInput(preview), expected_round_id: review.round_id };
    const ongoing = await post(discardDraftPath(preview), input, f.user.headers);
    assert.equal(ongoing.status, 409, ongoing.body);
    assert.equal(json(ongoing).code, 'REVIEW_WITHDRAW_REQUIRED');
    // 预览之后新增意见不能被原意见列表略过。
    assert.equal((await post(`${path}/actions`, changeInput(), f.user.headers)).status, 200);
    const withdrawn = await post(
      `${path}/withdrawals`,
      {
        operation_id: operation('discard-withdraw'),
        expected_round_id: review.round_id,
        reason: '明确结束本次修改工作',
      },
      f.user.headers,
    );
    assert.equal(withdrawn.status, 200, withdrawn.body);
    const changed = await post(discardDraftPath(preview), input, f.user.headers);
    assert.equal(changed.status, 409, changed.body);
    assert.equal(json(changed).code, 'DISCARD_CHANGES_CHANGED');
    const fresh = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
    assert.equal(fresh.status, 200, fresh.body);
    const current = json(fresh),
      confirm = { ...discardDraftInput(current), expected_round_id: review.round_id };
    assert.equal(current.blocking_changes.length, 2);
    for (const [patch, status] of [
      [{ expected_round_id: undefined }, 409],
      [{ expected_round_id: '999999999' }, 409],
      [{ changes_to_resolve: confirm.changes_to_resolve.slice(1) }, 409],
      [{ changes_to_resolve: [...confirm.changes_to_resolve, confirm.changes_to_resolve[0]] }, 400],
      [
        {
          changes_to_resolve: confirm.changes_to_resolve.map((change) => ({
            ...change,
            reason: ' ',
          })),
        },
        400,
      ],
    ]) {
      const response = await post(
        discardDraftPath(current),
        { ...confirm, ...patch, operation_id: operation('invalid-discard') },
        f.user.headers,
      );
      assert.equal(response.status, status, response.body);
    }
    assert.equal(
      (await post(discardDraftPath(current), confirm, f.f.members[0].headers)).status,
      403,
    );
    const accepted = await post(discardDraftPath(current), confirm, f.user.headers);
    assert.equal(accepted.status, 202, accepted.body);
    assert.equal((await pollDiscardDraft(confirm.operation_id, f.user.headers)).state, 'SUCCEEDED');
    const [[request]] = await db.query('SELECT status,archived_at FROM review_request WHERE id=?', [
      review.request_id,
    ]);
    assert.equal(request.status, 'WITHDRAWN');
    assert.ok(request.archived_at);
    const [[guard]] = await db.query(
      'SELECT ongoing_request_id,resumable_request_id FROM unit_review_guard WHERE unit_id=?',
      [current.unit_id],
    );
    assert.equal(guard.ongoing_request_id, null);
    assert.equal(guard.resumable_request_id, null);
    const [actions] = await db.query(
      'SELECT a.kind,a.reason,c.status,c.revision FROM review_action a LEFT JOIN change_request c ON c.resolved_action_id=a.id WHERE a.operation_id=? ORDER BY item_no',
      [confirm.operation_id],
    );
    assert.deepEqual(
      actions.map((action) => action.kind),
      ['ARCHIVE', 'RESOLVE_CHANGES', 'RESOLVE_CHANGES'],
    );
    for (const action of actions.slice(1)) {
      assert.equal(action.status, 'RESOLVED');
      assert.equal(String(action.revision), '2');
      assert.equal(action.reason, '明确保留旧稿并结束此项修改要求');
    }
    const [[audit]] = await db.query(
      "SELECT COUNT(*) total FROM audit_event WHERE operation_id=? AND kind='ARCHIVE'",
      [confirm.operation_id],
    );
    assert.equal(Number(audit.total), 1);
    next = await openReviewWorkspace(f.f, f.d);
    await writeDraft(next, '恢复之后新工作须走新审核');
    const flush = await flushDraft(next);
    const old = await post(
      `/v1/documents/${f.d.id}/units/${current.unit_id}/submissions`,
      {
        operation_id: operation('archived-resubmit'),
        mode: 'RESUBMIT',
        branch_id: next.session.branch_id,
        expected_structure_id: current.structure_id,
        flush_receipt_id: flush.receipt_id,
        request_id: review.request_id,
        expected_round_id: review.round_id,
      },
      f.user.headers,
    );
    assert.equal(old.status, 409, old.body);
    assert.equal(json(old).code, 'RESUBMIT_POSITION_CHANGED');
    const newReview = (await submitDraft(next)).value;
    assert.equal(newReview.state, 'SUCCEEDED', JSON.stringify(newReview));
    assert.notEqual(newReview.result.request_id, review.request_id);
  } finally {
    next?.close();
    f.close();
    await db.end();
  }
});
