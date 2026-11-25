import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, flushDraft, reviewDatabase } from '../helpers/review.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import { post, get, json, operation } from '../helpers/workspace.mjs';

test('actual submission survives Node outage, real admission expiry and owned Go restart without another POST', async () => {
  const f = await reviewFixture(),
    database = await reviewDatabase();
  let interrupted = false;
  try {
    await writeDraft(f, '跨过真实准入期限仍按原号送审');
    const receipt = await flushDraft(f),
      id = operation('recover-submit'),
      unit = f.grant.context.target.unit_id;
    f.close();
    stopOwnedRole('collaboration');
    interrupted = true;
    const response = await post(
      `/v1/documents/${f.d.id}/units/${unit}/submissions`,
      {
        operation_id: id,
        mode: 'NEW',
        branch_id: f.session.branch_id,
        expected_structure_id: f.grant.context.target.structure_id,
        flush_receipt_id: receipt.receipt_id,
      },
      f.f.owner.headers,
    );
    assert.equal(response.status, 202);
    // 实际经过 31 秒，期间 Node 不可达；不是改数据库期限冒充长期恢复。
    await new Promise((resolve) => setTimeout(resolve, 31000));
    const pending = await get('/v1/operations/' + id, f.f.owner.headers);
    assert.equal(pending.status, 200);
    assert.equal(json(pending).state, 'PREPARING');
    const [[guard]] = await database.query(
      'SELECT submission_operation_id FROM unit_review_guard WHERE unit_id=?',
      [unit],
    );
    assert.equal(guard.submission_operation_id, id);
    const [[row]] = await database.query(
      "SELECT payload FROM durable_task WHERE kind='SUBMISSION_RECOVER' AND business_key=?",
      [id],
    );
    const fixed = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
    assert.ok(fixed.access.Proofs.every((p) => p.expires_unix_ms < Date.now()));
    assert.equal(JSON.stringify(fixed).includes('__Host-collabSession'), false);
    restartOwnedRuntime();
    interrupted = false;
    let result;
    for (let n = 0; n < 100; n++) {
      const poll = await get('/v1/operations/' + id, f.f.owner.headers);
      assert.equal(poll.status, 200);
      result = json(poll);
      if (result.state !== 'PREPARING') break;
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    assert.equal(result.state, 'SUCCEEDED');
    const [[content]] = await database.query(
      'SELECT canonical_body FROM content_version WHERE id=?',
      [result.result.content_version_id],
    );
    assert.match(content.canonical_body.toString('utf8'), /跨过真实准入期限仍按原号送审/);
    const node = await reviewDatabase('collaboration');
    try {
      let capture;
      for (let n = 0; n < 100; n++) {
        [[capture]] = await node.query(
          'SELECT state,go_result_ref FROM capture_operation WHERE operation_id=?',
          [id],
        );
        if (capture?.state === 'BOUND') break;
        await new Promise((resolve) => setTimeout(resolve, 50));
      }
      assert.equal(capture.state, 'BOUND');
      assert.ok(capture.go_result_ref.startsWith(id + ':'));
    } finally {
      await node.end();
    }
  } finally {
    f.close();
    await database.end();
    if (interrupted) restartOwnedRuntime();
  }
});
