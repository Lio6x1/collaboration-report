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

// 反例：撤回若清空意见，或重提另建请求，审核门槛便会被绕过。
test('real withdraw and resubmit preserve blocking changes and reject actions on the previous round', async () => {
  const f = await reviewFixture();
  const db = await reviewDatabase();
  try {
    await writeDraft(f, '需要继续审核的固定内容');
    const first = (await submitDraft(f)).value;
    assert.equal(first.state, 'SUCCEEDED');
    const fixed = first.result,
      path = '/v1/review-requests/' + fixed.request_id;
    const change = {
      operation_id: operation('changes'),
      round_id: fixed.round_id,
      content_version_id: fixed.content_version_id,
      type: 'REQUEST_CHANGES',
      reason: '补齐数据来源',
    };
    const made = await post(path + '/actions', change, f.f.owner.headers);
    assert.equal(made.status, 200, json(made).code);
    const changeID = json(made).result.entity_id;
    assert.ok(changeID);
    assert.deepEqual(
      json(await post(path + '/actions', change, f.f.owner.headers)).result,
      json(made).result,
    );
    assert.equal(
      (await post(path + '/actions', { ...change, reason: '换一个理由' }, f.f.owner.headers))
        .status,
      409,
    );
    assert.equal(
      (
        await post(
          path + '/actions',
          { ...change, operation_id: operation('blank'), reason: '  ' },
          f.f.owner.headers,
        )
      ).status,
      400,
    );
    assert.equal(
      (
        await post(
          path + '/actions',
          {
            ...change,
            operation_id: operation('wrong-content'),
            content_version_id: f.d.main.units[0].content_version_id,
          },
          f.f.owner.headers,
        )
      ).status,
      409,
    );
    const withdrawal = {
      operation_id: operation('withdraw'),
      expected_round_id: fixed.round_id,
      reason: '准备下一轮',
    };
    const withdrawn = await post(path + '/withdrawals', withdrawal, f.f.owner.headers);
    assert.equal(withdrawn.status, 200, json(withdrawn).code);
    const [[guard]] = await db.query(
      'SELECT ongoing_request_id,resumable_request_id FROM unit_review_guard WHERE unit_id=?',
      [f.grant.context.target.unit_id],
    );
    assert.equal(guard.ongoing_request_id, null);
    assert.equal(String(guard.resumable_request_id), fixed.request_id);
    const whileWithdrawn = await post(
      path + '/change-requests/' + changeID + '/resolve',
      {
        operation_id: operation('withdrawn-resolve'),
        expected_round_id: fixed.round_id,
        expected_change_revision: '1',
        reason: '撤回期间仍须保留审核门槛',
      },
      f.f.owner.headers,
    );
    assert.equal(whileWithdrawn.status, 409, '独立审核动作只能针对审核中的当前轮次');
    assert.equal(
      (
        await post(
          `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/submissions`,
          {
            operation_id: operation('bypass'),
            mode: 'NEW',
            branch_id: f.session.branch_id,
            expected_structure_id: f.grant.context.target.structure_id,
            flush_receipt_id: operation('unused-flush'),
          },
          f.f.owner.headers,
        )
      ).status,
      409,
    );
    // 原样重提也必须形成新轮次，意见和历次作者均保留。
    const second = (
      await submitDraft(f, {
        mode: 'RESUBMIT',
        request_id: fixed.request_id,
        expected_round_id: fixed.round_id,
      })
    ).value;
    assert.equal(second.state, 'SUCCEEDED');
    assert.equal(second.result.request_id, fixed.request_id);
    assert.notEqual(second.result.round_id, fixed.round_id);
    assert.equal(
      (
        await post(
          path + '/actions',
          { ...change, operation_id: operation('old-round') },
          f.f.owner.headers,
        )
      ).status,
      409,
    );
    assert.equal(
      (
        await post(
          path + '/withdrawals',
          { ...withdrawal, operation_id: operation('old-withdraw') },
          f.f.owner.headers,
        )
      ).status,
      409,
    );
    const [[kept]] = await db.query('SELECT status,revision FROM change_request WHERE id=?', [
      changeID,
    ]);
    assert.equal(kept.status, 'OPEN');
    const resolve = {
      operation_id: operation('resolve'),
      expected_round_id: second.result.round_id,
      expected_change_revision: String(kept.revision),
      reason: '已检查，数据来源充分',
    };
    assert.equal(
      (
        await post(
          path + '/change-requests/' + changeID + '/resolve',
          { ...resolve, expected_change_revision: '999' },
          f.f.owner.headers,
        )
      ).status,
      409,
    );
    const done = await post(
      path + '/change-requests/' + changeID + '/resolve',
      resolve,
      f.f.owner.headers,
    );
    assert.equal(done.status, 200, json(done).code);
    const [[facts]] = await db.query(
      "SELECT (SELECT COUNT(*) FROM review_action WHERE request_id=? AND kind='APPROVE_AND_MERGE') AS approvals,(SELECT COUNT(*) FROM request_submitter WHERE request_id=?) AS authors,(SELECT status FROM change_request WHERE id=?) AS changeStatus,(SELECT status FROM review_request WHERE id=?) AS requestStatus",
      [fixed.request_id, fixed.request_id, changeID, fixed.request_id],
    );
    assert.equal(Number(facts.approvals), 0);
    assert.equal(Number(facts.authors), 1);
    assert.equal(facts.changeStatus, 'RESOLVED');
    assert.equal(facts.requestStatus, 'REVIEWING');
  } finally {
    f.close();
    await db.end();
  }
});

test('a current reviewer can request and resolve changes but cannot withdraw the submission', async () => {
  const f = await reviewFixture(1);
  try {
    await writeDraft(f, '审核身份与编辑身份独立');
    const fixed = (await submitDraft(f)).value.result,
      reviewer = f.f.members[0],
      path = '/v1/review-requests/' + fixed.request_id;
    const input = {
      operation_id: operation('reviewer-change'),
      round_id: fixed.round_id,
      content_version_id: fixed.content_version_id,
      type: 'REQUEST_CHANGES',
      reason: '补齐说明',
    };
    assert.equal((await post(path + '/actions', input, reviewer.headers)).status, 403);
    const document = json(await get('/v1/documents/' + f.d.id, f.f.owner.headers));
    await waitAuthorization(
      await post(
        '/v1/documents/' + f.d.id + '/authorization-changes',
        {
          operation_id: operation('grant-review'),
          expected_acl_revision: document.acl_revision,
          changes: [
            {
              action: 'GRANT',
              grant: {
                user_id: reviewer.id,
                scope_kind: 'DOCUMENT',
                scope_id: f.d.id,
                capability: 'REVIEW',
              },
            },
          ],
        },
        f.f.owner.headers,
      ),
      f.f.owner,
    );
    const made = await post(path + '/actions', input, reviewer.headers);
    assert.equal(made.status, 200, json(made).code);
    assert.equal(
      (
        await post(
          path + '/withdrawals',
          { operation_id: operation('reviewer-withdraw'), expected_round_id: fixed.round_id },
          reviewer.headers,
        )
      ).status,
      403,
    );
    const resolvePath = path + '/change-requests/' + json(made).result.entity_id + '/resolve';
    assert.equal(
      (
        await post(
          resolvePath,
          {
            operation_id: operation('reviewer-resolve'),
            expected_round_id: fixed.round_id,
            expected_change_revision: '1',
            reason: '确认已满足',
          },
          reviewer.headers,
        )
      ).status,
      200,
    );
  } finally {
    f.close();
  }
});

test('a reviewer who resubmitted remains a historical submitter and needs manager resolution', async () => {
  const f = await reviewFixture(1);
  let editor;
  const db = await reviewDatabase();
  try {
    await writeDraft(f, '历次作者不能通过重提洗掉自审限制');
    const first = (await submitDraft(f)).value.result,
      member = f.f.members[0],
      path = '/v1/review-requests/' + first.request_id;
    const d = json(await get('/v1/documents/' + f.d.id, f.f.owner.headers));
    await waitAuthorization(
      await post(
        '/v1/documents/' + f.d.id + '/authorization-changes',
        {
          operation_id: operation('editor-reviewer'),
          expected_acl_revision: d.acl_revision,
          changes: [
            {
              action: 'GRANT',
              grant: {
                user_id: member.id,
                scope_kind: 'UNIT',
                scope_id: f.grant.context.target.unit_id,
                capability: 'EDIT',
              },
            },
            {
              action: 'GRANT',
              grant: {
                user_id: member.id,
                scope_kind: 'DOCUMENT',
                scope_id: f.d.id,
                capability: 'REVIEW',
              },
            },
          ],
        },
        f.f.owner.headers,
      ),
      f.f.owner,
    );
    assert.equal(
      (
        await post(
          path + '/withdrawals',
          { operation_id: operation('withdraw'), expected_round_id: first.round_id },
          member.headers,
        )
      ).status,
      200,
    );
    editor = await openReviewWorkspace(f.f, f.d, member);
    const second = (
      await submitDraft(editor, {
        mode: 'RESUBMIT',
        request_id: first.request_id,
        expected_round_id: first.round_id,
      })
    ).value;
    assert.equal(second.state, 'SUCCEEDED');
    const change = await post(
      path + '/actions',
      {
        operation_id: operation('own-change'),
        round_id: second.result.round_id,
        content_version_id: second.result.content_version_id,
        type: 'REQUEST_CHANGES',
        reason: '本人提出的复核意见',
      },
      member.headers,
    );
    assert.equal(change.status, 200);
    const resolvePath = path + '/change-requests/' + json(change).result.entity_id + '/resolve',
      input = {
        operation_id: operation('self-resolve'),
        expected_round_id: second.result.round_id,
        expected_change_revision: '1',
        reason: '已确认',
      };
    const denied = await post(resolvePath, input, member.headers);
    assert.equal(denied.status, 403);
    assert.equal(json(denied).code, 'CHANGE_RESOLUTION_NOT_ALLOWED');
    assert.equal(
      (
        await post(
          resolvePath,
          { ...input, operation_id: operation('manager-resolve'), reason: '管理例外复核完成' },
          f.f.owner.headers,
        )
      ).status,
      200,
    );
    const [authors] = await db.query(
      'SELECT CAST(user_id AS CHAR) AS id FROM request_submitter WHERE request_id=? ORDER BY user_id',
      [first.request_id],
    );
    assert.deepEqual(authors.map((x) => x.id).sort(), [f.f.owner.id, member.id].sort());
  } finally {
    editor?.close();
    f.close();
    await db.end();
  }
});
