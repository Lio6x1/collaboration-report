import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, reviewDatabase } from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import { createHash } from 'node:crypto';

test('structure preview fixes its original whole-document cut and leaves current structure and draft intact', async () => {
  const f = await reviewFixture(1),
    db = await reviewDatabase(),
    peer = documentRPC();
  let receiptExpiry;
  try {
    const doc = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const path = `/v1/documents/${f.d.id}/structure-previews`;
    const input = {
      operation_id: operation('structure-preview'),
      expected_structure_id: f.d.main.structure.id,
      expected_control_revision: doc.control_revision,
      changes: [
        {
          type: 'RENAME',
          chapter_id: f.d.main.structure.entries[0].chapter_id,
          title: '预览中的标题',
        },
      ],
    };
    assert.equal((await post(path, input, f.f.members[0].headers)).status, 403);
    assert.equal((await post(path, input, f.user.headers)).status, 409);
    await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/main-lock`,
        {
          operation_id: operation('unlock-preview'),
          expected_control_revision: doc.control_revision,
          locked: false,
        },
        f.user.headers,
      ),
      f.user,
    );
    const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    input.expected_control_revision = before.control_revision;
    await writeDraft(f, '预览前可靠保存的草稿');
    const response = await post(path, input, f.user.headers);
    assert.equal(response.status, 200, json(response).code);
    const preview = json(response);
    assert.equal(preview.preview_cut_ref, input.operation_id);
    assert.match(preview.preview_cut_ref, /^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$/);
    assert.match(preview.cut_digest, /^[a-f0-9]{64}$/);
    assert.equal(preview.structure_id, input.expected_structure_id);
    assert.deepEqual(preview.blocking_units, []);
    assert.ok(Date.parse(preview.expires_at) > Date.now());
    const progress = await get(`/v1/operations/${input.operation_id}`, f.user.headers);
    assert.equal(progress.status, 200, json(progress).code);
    assert.deepEqual(json(progress), {
      operation_id: input.operation_id,
      state: 'SUCCEEDED',
      result: { entity_id: input.expected_structure_id },
    });
    assert.equal(
      (await get(`/v1/operations/${input.operation_id}`, f.f.members[0].headers)).status,
      403,
    );
    const [[row]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='STRUCTURE_PREVIEW_CONTEXT' AND business_key=?",
      [input.operation_id],
    );
    const fixed = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
    const query = {
      operation_id: input.operation_id,
      target: fixed.Context.target,
      request_fingerprint: Buffer.from(fixed.Context.request_fingerprint, 'base64'),
    };
    const authority = await rpcCall(peer, 'GetOperationResult', query);
    assert.equal(authority.state, 'SUCCEEDED');
    assert.deepEqual(JSON.parse(authority.fixed_result_json_utf8.toString()), preview);
    assert.deepEqual(
      authority.result_digest,
      createHash('sha256').update(authority.fixed_result_json_utf8).digest(),
    );
    for (const wrong of [
      { ...query, target: { ...query.target, structure_id: '999999999' } },
      { ...query, request_fingerprint: Buffer.alloc(32) },
    ]) {
      await assert.rejects(
        () => rpcCall(peer, 'GetOperationResult', wrong),
        (e) => e.code === 6,
      );
    }
    // 同号预览不能重采新尾部；新操作可以固定新切点，不能改写旧预览。
    await writeDraft(f, '预览之后继续可靠保存的S2');
    const replay = await post(path, input, f.user.headers);
    assert.equal(replay.status, 200, json(replay).code);
    assert.deepEqual(json(replay), preview);
    const next = await post(
      path,
      { ...input, operation_id: operation('fresh-structure-preview') },
      f.user.headers,
    );
    assert.equal(next.status, 200, json(next).code);
    assert.notEqual(json(next).cut_digest, preview.cut_digest);
    for (const [id, text] of [
      [input.operation_id, '预览前可靠保存的草稿'],
      [json(next).preview_cut_ref, '预览之后继续可靠保存的S2'],
    ]) {
      const [[snapshot]] = await db.query(
        "SELECT payload FROM durable_task WHERE kind='STRUCTURE_PREVIEW_FIXED' AND business_key=?",
        [id],
      );
      const value =
        typeof snapshot.payload === 'string' ? JSON.parse(snapshot.payload) : snapshot.payload;
      assert.ok(Buffer.from(value.drafts[0].body_json_utf8, 'base64').toString().includes(text));
    }
    assert.equal(
      (
        await post(
          path,
          { ...input, changes: [{ ...input.changes[0], title: '篡改原号输入' }] },
          f.user.headers,
        )
      ).status,
      409,
    );
    const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    assert.equal(after.current_structure_id, before.current_structure_id);
    assert.equal(after.current_manifest_id, before.current_manifest_id);
    assert.equal(after.control_revision, before.control_revision);
    const [[receipt]] = await db.query(
      'SELECT expires_at FROM operation_receipt WHERE operation_id=?',
      [input.operation_id],
    );
    receiptExpiry = { id: input.operation_id, expires: receipt.expires_at };
    await db.query(
      'UPDATE operation_receipt SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE operation_id=?',
      [input.operation_id],
    );
    assert.equal((await get(`/v1/operations/${input.operation_id}`, f.user.headers)).status, 404);
    assert.equal((await post(path, input, f.user.headers)).status, 404);
    assert.deepEqual(await rpcCall(peer, 'GetOperationResult', query), authority);
  } finally {
    if (receiptExpiry)
      await db.query('UPDATE operation_receipt SET expires_at=? WHERE operation_id=?', [
        receiptExpiry.expires,
        receiptExpiry.id,
      ]);
    peer.close();
    await db.end();
    f.close();
  }
});
