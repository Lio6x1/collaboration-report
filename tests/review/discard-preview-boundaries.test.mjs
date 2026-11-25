import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createHash } from 'node:crypto';
import {
  reviewFixture,
  reviewDatabase,
  writeDraft,
  writeDraftBody,
  submitDraft,
  collaborationRPC,
  openReviewWorkspace,
} from '../helpers/review.mjs';
import {
  get,
  post,
  json,
  operation,
  organizationFixture,
  documentFixture,
} from '../helpers/workspace.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import {
  discardPreviewPath,
  discardPreviewInput,
  discardPreviewWire,
} from '../helpers/discard-previews.mjs';
import { request } from '../helpers/https.mjs';
import { runtime, state } from '../../scripts/runtime-lib.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { writeFileSync } from 'node:fs';

const certificate = (role) => ({
  key: readFileSync(resolve(runtime, `pki/${role}-key.pem`)),
  cert: readFileSync(resolve(runtime, `pki/${role}.pem`)),
});
const valueOf = (raw) => (typeof raw === 'string' ? JSON.parse(raw) : raw);

test('discard preview fixes reliable MAIN tail independently of room ID order', async () => {
  const organization = await organizationFixture(),
    d = await documentFixture(organization);
  await unlockStructurePreview({ d, user: organization.owner });
  const main = await openReviewWorkspace(organization, d, organization.owner, 'MAIN');
  const draft = await openReviewWorkspace(organization, d);
  const database = await reviewDatabase();
  try {
    await writeDraft(main, '固定主线的可靠尾部S1');
    await writeDraft(draft, '固定前的可靠共同草稿');
    const input = discardPreviewInput(draft);
    // 记录原操作身份用于失败后的原号续接；不保存证明、会话或正文到诊断输出。
    writeFileSync(
      resolve('artifacts/batches/04/discard-preview-main-tail-original.json'),
      JSON.stringify(
        {
          operation: input.operation_id,
          document: d.id,
          unit: d.main.units[0].unit_id,
          branch: draft.session.branch_id,
        },
        null,
        2,
      ),
    );
    const response = await post(discardPreviewPath(draft), input, draft.user.headers);
    assert.equal(response.status, 200, response.body);
    const preview = json(response);
    const [[fixedRow]] = await database.query(
      "SELECT payload FROM durable_task WHERE kind='DISCARD_PREVIEW_FIXED' AND business_key=?",
      [input.operation_id],
    );
    const fixed = valueOf(fixedRow.payload);
    assert.equal(fixed.main_cut.length, 1);
    assert.match(
      Buffer.from(fixed.main.body_json_utf8, 'base64').toString(),
      /固定主线的可靠尾部S1/,
    );
    assert.equal(fixed.main.body_json_utf8, fixed.candidate.body_json_utf8);
    await writeDraft(main, '预览之后的主线可靠尾部S2');
    const replay = await post(discardPreviewPath(draft), input, draft.user.headers);
    assert.equal(replay.status, 200, replay.body);
    assert.deepEqual(json(replay), preview);
  } finally {
    main.close();
    draft.close();
    await database.end();
  }
});

test('discard preview system result retains exact original binding beyond public receipt expiry', async () => {
  const f = await reviewFixture(),
    database = await reviewDatabase(),
    peer = documentRPC(),
    collaboration = collaborationRPC();
  let savedExpiry;
  try {
    await writeDraft(f, '原预览范围和系统终态');
    const input = discardPreviewInput(f),
      path = discardPreviewPath(f);
    const response = await post(path, input, f.user.headers);
    assert.equal(response.status, 200, response.body);
    const preview = json(response);
    const [[row]] = await database.query(
      "SELECT payload FROM durable_task WHERE kind='DISCARD_PREVIEW_CONTEXT' AND business_key=?",
      [input.operation_id],
    );
    const wire = discardPreviewWire(valueOf(row.payload));
    const query = {
      operation_id: input.operation_id,
      target: wire.context.target,
      request_fingerprint: Buffer.from(wire.context.request_fingerprint, 'base64'),
    };
    const original = await rpcCall(peer, 'GetOperationResult', query);
    assert.equal(original.state, 'SUCCEEDED');
    assert.deepEqual(JSON.parse(original.fixed_result_json_utf8.toString()), preview);
    assert.deepEqual(
      original.result_digest,
      createHash('sha256').update(original.fixed_result_json_utf8).digest(),
    );
    for (const wrong of [
      { ...query, target: { ...query.target, unit_id: '99999999' } },
      { ...query, request_fingerprint: Buffer.alloc(32) },
    ])
      await assert.rejects(
        () => rpcCall(peer, 'GetOperationResult', wrong),
        (error) => error.code === 6,
      );
    const options = {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(wire),
    };
    assert.equal(
      (
        await request(state().ports.collaborationHttp, '/infra/draft/fixed', {
          ...options,
          ...certificate('account'),
        })
      ).status,
      403,
    );
    assert.equal(
      (
        await request(state().ports.documentHttp, '/infra/draft/context', {
          ...options,
          ...certificate('document'),
        })
      ).status,
      403,
    );
    const fixed = await request(state().ports.collaborationHttp, '/infra/draft/fixed', {
      ...options,
      ...certificate('document'),
    });
    assert.equal(fixed.status, 200, fixed.body);
    assert.equal(json(fixed).preview_cut_ref, input.operation_id);
    for (const wrong of [
      {
        ...wire,
        context: { ...wire.context, request_fingerprint: Buffer.alloc(32).toString('base64') },
      },
      { ...wire, expected_lineage_id: '99999999' },
    ])
      await assert.rejects(
        () => rpcCall(collaboration, 'CaptureDraftPreview', wrong),
        (error) => error.code === 6,
      );
    const [[receipt]] = await database.query(
      'SELECT expires_at FROM operation_receipt WHERE operation_id=?',
      [input.operation_id],
    );
    savedExpiry = { id: input.operation_id, expires: receipt.expires_at };
    await database.query(
      'UPDATE operation_receipt SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE operation_id=?',
      [input.operation_id],
    );
    assert.equal((await get(`/v1/operations/${input.operation_id}`, f.user.headers)).status, 404);
    assert.equal((await post(path, input, f.user.headers)).status, 404);
    const authority = await rpcCall(peer, 'GetOperationResult', query);
    assert.equal(authority.state, 'SUCCEEDED');
    assert.deepEqual(authority.fixed_result_json_utf8, original.fixed_result_json_utf8);
    assert.deepEqual(authority.result_digest, original.result_digest);
  } finally {
    if (savedExpiry)
      await database.query('UPDATE operation_receipt SET expires_at=? WHERE operation_id=?', [
        savedExpiry.expires,
        savedExpiry.id,
      ]);
    f.close();
    peer.close();
    collaboration.close();
    await database.end();
  }
});

test('discard preview retains old draft image references and reports actual open opinion without resolving it', async () => {
  const f = await reviewFixture(),
    database = await reviewDatabase();
  try {
    // READY图片仅是同空间数据库夹具，覆盖固定版本引用；物理上传/下载仍留待后续真实资源批次。
    const key = operation('discard-image'),
      [insert] = await database.query(
        "INSERT INTO resource(space_id,uploader_id,storage_key,checksum,byte_size,media_type,state,upload_operation_id) VALUES(?,?,?,UNHEX(REPEAT('00',32)),1,'image/png','READY',?)",
        [f.f.space, f.user.id, key, key],
      );
    const resource = String(insert.insertId);
    await writeDraftBody(f, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'discard-image-paragraph' },
          content: [
            {
              type: 'image',
              attrs: {
                resourceId: resource,
                alt: '旧草稿固定图片',
                title: null,
                width: 160,
                height: 100,
              },
            },
          ],
        },
      ],
    });
    const submitted = (await submitDraft(f)).value;
    assert.equal(submitted.state, 'SUCCEEDED', submitted.error?.code);
    const review = submitted.result,
      reviewPath = `/v1/review-requests/${review.request_id}`;
    const changed = await post(
      `${reviewPath}/actions`,
      {
        operation_id: operation('discard-open-change'),
        round_id: review.round_id,
        content_version_id: review.content_version_id,
        type: 'REQUEST_CHANGES',
        reason: '图片说明需要修改，预览不能自动解除',
      },
      f.user.headers,
    );
    assert.equal(changed.status, 200, changed.body);
    const [[change]] = await database.query(
      "SELECT id,revision,author_id FROM change_request WHERE request_id=? AND status='OPEN'",
      [review.request_id],
    );
    const input = discardPreviewInput(f),
      previewResponse = await post(discardPreviewPath(f), input, f.user.headers);
    assert.equal(previewResponse.status, 200, previewResponse.body);
    const preview = json(previewResponse);
    assert.deepEqual(preview.blocking_changes, [
      {
        id: String(change.id),
        revision: String(change.revision),
        author_id: String(change.author_id),
        reason: '图片说明需要修改，预览不能自动解除',
      },
    ]);
    const [refs] = await database.query(
      "SELECT owner_kind,owner_ref FROM resource_reference WHERE resource_id=? AND document_id=? AND ((owner_kind='VERSION' AND owner_ref=?) OR (owner_kind='OPERATION' AND owner_ref=?)) ORDER BY owner_kind",
      [resource, f.d.id, preview.draft_content_version_id, input.operation_id],
    );
    assert.deepEqual(
      refs.map((v) => v.owner_kind),
      ['OPERATION', 'VERSION'],
    );
    const after = json(await get(reviewPath, f.user.headers));
    assert.equal(after.status, 'REVIEWING');
    assert.equal(after.blocking_change_count, 1);
    assert.equal(after.archived_at, null);
    const withdrawal = await post(
      `${reviewPath}/withdrawals`,
      {
        operation_id: operation('discard-preview-withdraw'),
        expected_round_id: review.round_id,
        reason: '等待明确丢弃确认',
      },
      f.user.headers,
    );
    assert.equal(withdrawal.status, 200, withdrawal.body);
    const withdrawn = await post(
      discardPreviewPath(f),
      { ...input, operation_id: operation('discard-withdrawn-preview') },
      f.user.headers,
    );
    assert.equal(withdrawn.status, 200, withdrawn.body);
    assert.deepEqual(json(withdrawn).blocking_changes, preview.blocking_changes);
    const actual = json(await get(reviewPath, f.user.headers));
    assert.equal(actual.status, 'WITHDRAWN');
    assert.equal(actual.blocking_change_count, 1);
    assert.equal(actual.archived_at, null);
  } finally {
    f.close();
    await database.end();
  }
});
