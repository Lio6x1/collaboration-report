import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, submitDraft, reviewDatabase } from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import { pollMerge } from '../helpers/merge.mjs';

// 登记只占用原请求，不代表已批准；测试真实receipt/任务/请求必须原子落库，正文仍是固定S1。
test('approve-and-merge registers one durable original operation without approving early', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let interrupted = false;
  try {
    await writeDraft(f, '合并只读取送审S1');
    const fixed = (await submitDraft(f)).value.result;
    await writeDraft(f, '送审后的S2仍是共同草稿');
    // 真实Node退出使登记与批准两个时刻可观察；不能靠计算恰巧较慢得到“尚未批准”的脆弱断言。
    f.close();
    stopOwnedRole('collaboration');
    interrupted = true;
    const path = `/v1/review-requests/${fixed.request_id}/approve-and-merge`;
    const input = {
      operation_id: operation('merge'),
      round_id: fixed.round_id,
      content_version_id: fixed.content_version_id,
      own_changes_to_resolve: [],
    };
    const made = await post(path, input, f.f.owner.headers);
    assert.equal(made.status, 202, json(made).code);
    assert.equal(json(made).operation_id, input.operation_id);
    assert.equal(json(made).poll_url, `/v1/merge-operations/${input.operation_id}`);
    const repeated = await post(path, input, f.f.owner.headers);
    assert.equal(repeated.status, 202, json(repeated).code);
    assert.equal(json(repeated).operation_id, input.operation_id);
    assert.equal(
      (
        await post(
          path,
          { ...input, content_version_id: f.d.main.units[0].content_version_id },
          f.f.owner.headers,
        )
      ).status,
      409,
    );
    assert.equal(
      (
        await post(
          path,
          { ...input, operation_id: operation('competing-merge') },
          f.f.owner.headers,
        )
      ).status,
      409,
    );
    const polled = await get(`/v1/merge-operations/${input.operation_id}`, f.f.owner.headers);
    assert.equal(polled.status, 200, json(polled).code);
    const [[facts]] = await db.query(
      "SELECT m.content_version_id,m.actor_id,r.active_merge_operation_id,r.status,(SELECT COUNT(*) FROM durable_task WHERE kind='MERGE_RECOVER' AND operation_id=m.operation_id) AS tasks,(SELECT COUNT(*) FROM review_action WHERE operation_id=m.operation_id AND kind='APPROVE_AND_MERGE') AS approvals FROM merge_operation m JOIN review_request r ON r.id=m.request_id WHERE m.operation_id=?",
      [input.operation_id],
    );
    assert.equal(String(facts.content_version_id), fixed.content_version_id);
    assert.equal(String(facts.actor_id), f.f.owner.id);
    assert.equal(facts.active_merge_operation_id, input.operation_id);
    assert.equal(facts.status, 'REVIEWING');
    assert.equal(Number(facts.tasks), 1);
    assert.equal(Number(facts.approvals), 0);
    // 让这次只验证登记的操作正常取消；将来完整流水线另有终态/S1/S2测试。
    const withdrawn = await post(
      `/v1/review-requests/${fixed.request_id}/withdrawals`,
      {
        operation_id: operation('cancel-pending-merge'),
        expected_round_id: fixed.round_id,
        reason: '结束登记反例',
      },
      f.f.owner.headers,
    );
    assert.equal(withdrawn.status, 200, json(withdrawn).code);
    restartOwnedRuntime();
    interrupted = false;
    const rejected = await pollMerge(input.operation_id, f.f.owner.headers, db);
    assert.equal(rejected.state, 'REJECTED');
    assert.equal(rejected.error.code, 'REVIEW_REQUEST_NOT_REVIEWING');
    const [[after]] = await db.query(
      "SELECT status,active_merge_operation_id,(SELECT COUNT(*) FROM review_action WHERE operation_id=? AND kind='APPROVE_AND_MERGE') AS approvals FROM review_request WHERE id=?",
      [input.operation_id, fixed.request_id],
    );
    assert.equal(after.status, 'WITHDRAWN');
    assert.equal(after.active_merge_operation_id, null);
    assert.equal(Number(after.approvals), 0);
  } finally {
    f.close();
    await db.end();
    if (interrupted) restartOwnedRuntime();
  }
});
