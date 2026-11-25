import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, submitDraft, reviewDatabase } from '../helpers/review.mjs';
import { post, get, json, operation } from '../helpers/workspace.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

const mergeInput = (id, fixed) => ({
  operation_id: id,
  round_id: fixed.round_id,
  content_version_id: fixed.content_version_id,
  own_changes_to_resolve: [],
});

test('original merge resumes after real identity expiry and owned runtime restart without another POST', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let stopped = false;
  try {
    await writeDraft(f, '原轮次S1跨过真实身份准入期限');
    const fixed = (await submitDraft(f)).value.result;
    await writeDraft(f, '较晚的S2保持在草稿');
    f.close();
    stopOwnedRole('collaboration');
    stopped = true;
    const id = operation('recover-merge');
    const made = await post(
      `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
      mergeInput(id, fixed),
      f.f.owner.headers,
    );
    assert.equal(made.status, 202, json(made).code);
    // 等待实际31秒，不改数据库证明期限。原会话/来源仍有效才能续接原号。
    await new Promise((resolve) => setTimeout(resolve, 31000));
    const pending = json(await get(`/v1/merge-operations/${id}`, f.f.owner.headers));
    assert.equal(pending.state, 'PREPARING');
    const [[registered]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='MERGE_RECOVER' AND business_key=?",
      [id],
    );
    const context =
      typeof registered.payload === 'string' ? JSON.parse(registered.payload) : registered.payload;
    assert.ok(context.access.Proofs.every((proof) => proof.expires_unix_ms < Date.now()));
    assert.equal(JSON.stringify(context).includes('__Host-collabSession'), false);
    restartOwnedRuntime();
    stopped = false;
    const result = await pollMerge(id, f.f.owner.headers, db);
    assert.equal(result.state, 'SUCCEEDED', result.error?.code);
    assert.equal(result.result.main_sync_state, 'READY');
    const [[body]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      result.result.content_version_id,
    ]);
    assert.match(body.canonical_body.toString(), /原轮次S1/);
    assert.doesNotMatch(body.canonical_body.toString(), /较晚的S2/);
  } finally {
    f.close();
    await db.end();
    if (stopped) restartOwnedRuntime();
  }
});

test('Go committed merge remains successful and syncing while real Node installation is interrupted', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase(),
    node = await reviewDatabase('collaboration');
  let stopped = false,
    locked = false;
  try {
    await writeDraft(f, '已提交的固定主线事实');
    const fixed = (await submitDraft(f)).value.result;
    const id = operation('install-recover');
    // 锁实际install_receipt的目标空隙，使Node不能先完成安装；不改receipt或伪造SUCCEEDED。
    await node.beginTransaction();
    locked = true;
    await node.query('SELECT operation_id FROM install_receipt WHERE operation_id=? FOR UPDATE', [
      id,
    ]);
    assert.equal(
      (
        await post(
          `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
          mergeInput(id, fixed),
          f.f.owner.headers,
        )
      ).status,
      202,
    );
    let value;
    for (let n = 0; n < 100; n++) {
      value = json(await get(`/v1/merge-operations/${id}`, f.f.owner.headers));
      if (value.state === 'SUCCEEDED') break;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    assert.equal(value.state, 'SUCCEEDED', value.error?.code);
    assert.equal(value.result.main_sync_state, 'SYNCING');
    const record = value.result.merge_record_id,
      manifest = value.result.manifest_id;
    const [[missing]] = await node.query(
      'SELECT COUNT(*) AS receipts FROM install_receipt WHERE operation_id=?',
      [id],
    );
    assert.equal(Number(missing.receipts), 0);
    f.close();
    stopOwnedRole('collaboration');
    stopped = true;
    await node.rollback();
    locked = false;
    const still = json(await get(`/v1/merge-operations/${id}`, f.f.owner.headers));
    assert.equal(still.state, 'SUCCEEDED');
    assert.equal(still.result.main_sync_state, 'SYNCING');
    restartOwnedRuntime();
    stopped = false;
    const result = await pollMerge(id, f.f.owner.headers, db);
    assert.equal(result.state, 'SUCCEEDED');
    assert.equal(result.result.main_sync_state, 'READY');
    assert.equal(result.result.merge_record_id, record);
    assert.equal(result.result.manifest_id, manifest);
    const [[installed]] = await node.query(
      'SELECT COUNT(*) AS receipts FROM install_receipt WHERE operation_id=?',
      [id],
    );
    assert.equal(Number(installed.receipts), 1);
  } finally {
    if (locked) await node.rollback();
    f.close();
    await node.end();
    await db.end();
    if (stopped) restartOwnedRuntime();
  }
});

test('changes requested during actual Node outage reject the original merge without an approval', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let stopped = false;
  try {
    await writeDraft(f, '计算期间新增意见仍有决定权');
    const fixed = (await submitDraft(f)).value.result,
      id = operation('blocked-merge');
    f.close();
    stopOwnedRole('collaboration');
    stopped = true;
    const path = `/v1/review-requests/${fixed.request_id}`;
    assert.equal(
      (await post(path + '/approve-and-merge', mergeInput(id, fixed), f.f.owner.headers)).status,
      202,
    );
    const made = await post(
      path + '/actions',
      {
        operation_id: operation('later-changes'),
        round_id: fixed.round_id,
        content_version_id: fixed.content_version_id,
        type: 'REQUEST_CHANGES',
        reason: '计算期间提出的新要求必须保留',
      },
      f.f.owner.headers,
    );
    assert.equal(made.status, 200, json(made).code);
    restartOwnedRuntime();
    stopped = false;
    const result = await pollMerge(id, f.f.owner.headers, db);
    assert.equal(result.state, 'REJECTED');
    assert.equal(result.error.code, 'CHANGES_PENDING');
    assert.equal(result.error.message, 'CHANGES_PENDING');
    assert.equal(result.error.trace_id, id);
    assert.equal(result.error.retry_action, 'NONE');
    const [[facts]] = await db.query(
      "SELECT (SELECT COUNT(*) FROM review_action WHERE operation_id=? AND kind='APPROVE_AND_MERGE') AS approvals,(SELECT status FROM change_request WHERE id=?) AS change_status,(SELECT status FROM review_request WHERE id=?) AS request_status",
      [id, json(made).result.entity_id, fixed.request_id],
    );
    assert.equal(Number(facts.approvals), 0);
    assert.equal(facts.change_status, 'OPEN');
    assert.equal(facts.request_status, 'REVIEWING');
  } finally {
    f.close();
    await db.end();
    if (stopped) restartOwnedRuntime();
  }
});
