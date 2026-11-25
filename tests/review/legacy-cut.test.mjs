import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { reviewFixture, writeDraft, submitDraft, reviewDatabase } from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { request } from '../helpers/https.mjs';
import { runtime, state } from '../../scripts/runtime-lib.mjs';

// 只在本测试新建的隔离轮次上去掉后来添加的元数据字段，以重现旧版本形状；正文/cut/终态均来自真实送审。
test('an older round recovers its actual committed cut after the original ACL source closes', async () => {
  const f = await reviewFixture(1),
    db = await reviewDatabase();
  let originalRound, round;
  try {
    await writeDraft(f, '旧固定轮次仍能正常合并');
    const submitted = await submitDraft(f),
      fixed = submitted.value.result;
    round = fixed.round_id;
    const [[row]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='REVIEW_ROUND_CONTEXT' AND business_key=?",
      [round],
    );
    originalRound = typeof row.payload === 'string' ? row.payload : JSON.stringify(row.payload);
    const old = JSON.parse(originalRound);
    assert.ok(old.cut);
    delete old.cut;
    await db.query(
      "UPDATE durable_task SET payload=? WHERE kind='REVIEW_ROUND_CONTEXT' AND business_key=?",
      [JSON.stringify(old), round],
    );
    const reviewer = f.f.members[0],
      current = json(await get(`/v1/documents/${f.d.id}`, f.f.owner.headers));
    await waitAuthorization(
      await post(
        `/v1/documents/${f.d.id}/authorization-changes`,
        {
          operation_id: operation('legacy-review'),
          expected_acl_revision: current.acl_revision,
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
    const [[registered]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='SUBMISSION_RECOVER' AND business_key=?",
      [submitted.input.operation_id],
    );
    const context = (
      typeof registered.payload === 'string' ? JSON.parse(registered.payload) : registered.payload
    ).capture.context;
    const target = Object.fromEntries(
      Object.entries(context.target).map(([key, value]) => [key, String(value)]),
    );
    const query = {
      operation_id: context.operation_id,
      target,
      request_fingerprint: context.request_fingerprint,
    };
    const options = {
      method: 'POST',
      body: JSON.stringify(query),
      cert: readFileSync(resolve(runtime, 'pki/document.pem')),
      key: readFileSync(resolve(runtime, 'pki/document-key.pem')),
    };
    const denied = await request(
      state().ports.collaborationHttp,
      '/infra/persistence/submission-cut',
      { ...options, cert: undefined, key: undefined },
    );
    assert.equal(denied.status, 403);
    const read = await request(
      state().ports.collaborationHttp,
      '/infra/persistence/submission-cut',
      options,
    );
    assert.equal(read.status, 200, JSON.parse(read.body).code);
    assert.equal(JSON.parse(read.body).content_version_id, fixed.content_version_id);
    assert.deepEqual(
      JSON.parse(Buffer.from(JSON.parse(read.body).cut_json_utf8, 'base64')),
      JSON.parse(originalRound).cut,
    );
    const wrong = await request(
      state().ports.collaborationHttp,
      '/infra/persistence/submission-cut',
      {
        ...options,
        body: JSON.stringify({
          ...query,
          target: { ...target, unit_id: String(BigInt(target.unit_id) + 1n) },
        }),
      },
    );
    assert.notEqual(wrong.status, 200);
    const id = operation('legacy-merge');
    const made = await post(
      `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
      {
        operation_id: id,
        round_id: round,
        content_version_id: fixed.content_version_id,
        own_changes_to_resolve: [],
      },
      reviewer.headers,
    );
    assert.equal(made.status, 202, json(made).code);
    const result = await pollMerge(id, reviewer.headers, db);
    assert.equal(result.state, 'SUCCEEDED', result.error?.code);
    const [[untouched]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='REVIEW_ROUND_CONTEXT' AND business_key=?",
      [round],
    );
    assert.equal(
      Object.hasOwn(
        typeof untouched.payload === 'string' ? JSON.parse(untouched.payload) : untouched.payload,
        'cut',
      ),
      false,
    );
    const [[artifact]] = await db.query(
      "SELECT COUNT(*) AS facts FROM durable_task WHERE kind='REVIEW_CAPTURE_CUT' AND business_key=? AND state='SUCCEEDED'",
      [round],
    );
    assert.equal(Number(artifact.facts), 1);
  } finally {
    // 恢复这个私有夹具的原元数据，保留真实合并与正文事实；不修改以前实验或用户记录。
    if (originalRound)
      await db.query(
        "UPDATE durable_task SET payload=? WHERE kind='REVIEW_ROUND_CONTEXT' AND business_key=?",
        [originalRound, round],
      );
    f.close();
    await db.end();
  }
});
