import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
  reviewFixture,
  writeDraftBody,
  submitDraft,
  reviewDatabase,
  collaborationRPC,
} from '../helpers/review.mjs';
import { post, get, json, operation } from '../helpers/workspace.mjs';
import {
  unlockStructurePreview,
  structurePreviewInput,
  structurePreviewWire,
} from '../helpers/structure-previews.mjs';
import { request } from '../helpers/https.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { runtime, state } from '../../scripts/runtime-lib.mjs';

const certificate = (role) => ({
  key: readFileSync(resolve(runtime, `pki/${role}-key.pem`)),
  cert: readFileSync(resolve(runtime, `pki/${role}.pem`)),
});

test('structure preview rejects stale scope, missing move anchor and mismatched system bindings', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase(),
    peer = collaborationRPC();
  let closedAdmission;
  try {
    const document = await unlockStructurePreview(f),
      path = `/v1/documents/${f.d.id}/structure-previews`;
    const base = structurePreviewInput(f, document);
    for (const changed of [
      { expected_structure_id: '999999999' },
      { expected_control_revision: String(BigInt(base.expected_control_revision) + 1n) },
      { changes: [{ type: 'REMOVE', chapter_id: '999999999' }] },
      {
        changes: [
          { type: 'MOVE', chapter_id: base.changes[0].chapter_id, after_chapter_id: '999999999' },
        ],
      },
      { changes: [{ type: 'JOIN_CHAPTER', chapter_id: base.changes[0].chapter_id }] },
    ]) {
      const response = await post(
        path,
        { ...base, ...changed, operation_id: operation('invalid-preview-scope') },
        f.user.headers,
      );
      assert.equal(response.status, 409, response.body);
    }
    const response = await post(path, base, f.user.headers);
    assert.equal(response.status, 200, response.body);
    const [[row]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='STRUCTURE_PREVIEW_CONTEXT' AND business_key=?",
      [base.operation_id],
    );
    const fixed = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload,
      wire = structurePreviewWire(fixed);
    const options = {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(wire),
    };
    assert.equal(
      (
        await request(state().ports.collaborationHttp, '/infra/structure/fixed', {
          ...options,
          ...certificate('account'),
        })
      ).status,
      403,
    );
    assert.equal(
      (
        await request(state().ports.documentHttp, '/infra/structure/context', {
          ...options,
          ...certificate('document'),
        })
      ).status,
      403,
    );
    const exact = await request(state().ports.collaborationHttp, '/infra/structure/fixed', {
      ...options,
      ...certificate('document'),
    });
    assert.equal(exact.status, 200, exact.body);
    assert.equal(json(exact).preview_cut_ref, base.operation_id);
    for (const wrong of [
      {
        ...wire,
        context: { ...wire.context, request_fingerprint: Buffer.alloc(32).toString('base64') },
      },
      {
        ...wire,
        context: { ...wire.context, target: { ...wire.context.target, document_id: '999999999' } },
      },
    ])
      await assert.rejects(
        () => rpcCall(peer, 'CaptureDraftPreview', wrong),
        (error) => error.code === 6,
      );
    await assert.rejects(
      () =>
        rpcCall(peer, 'CaptureDraftPreview', {
          ...wire,
          context: { ...wire.context, operation_id: operation('unregistered-preview') },
        }),
      (error) => error.code === 14,
    );
    const source = await request(state().ports.documentHttp, '/infra/structure/context', {
      ...options,
      ...certificate('collaboration'),
    });
    assert.equal(source.status, 200, source.body);
    const admission = wire.context.document_admission_ref;
    const [[header]] = await db.query('SELECT state FROM document_admission WHERE admission_id=?', [
      admission,
    ]);
    closedAdmission = { id: admission, state: header.state };
    // 单独注入本夹具准入关闭，确保已有固定引用/服务证书不能重开原准入；来源关闭整链另由第五批验证。
    await db.query("UPDATE document_admission SET state='CLOSED' WHERE admission_id=?", [
      admission,
    ]);
    const closed = await request(state().ports.documentHttp, '/infra/structure/context', {
      ...options,
      ...certificate('collaboration'),
    });
    assert.equal(closed.status, 403, closed.body);
  } finally {
    if (closedAdmission)
      await db.query('UPDATE document_admission SET state=? WHERE admission_id=?', [
        closedAdmission.state,
        closedAdmission.id,
      ]);
    peer.close();
    f.close();
    await db.end();
  }
});

// READY图片是同空间私有元数据夹具，只核验预览保护；物理上传/下载属于后续整站批次。
test('structure preview protects fixed resources and reports active or withdrawn blocking review without changing it', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  try {
    const document = await unlockStructurePreview(f),
      path = `/v1/documents/${f.d.id}/structure-previews`,
      key = operation('preview-resource');
    const [insert] = await db.query(
      "INSERT INTO resource(space_id,uploader_id,storage_key,checksum,byte_size,media_type,state,upload_operation_id) VALUES(?,?,?,UNHEX(REPEAT('00',32)),1,'image/png','READY',?)",
      [f.f.space, f.user.id, key, key],
    );
    const resource = String(insert.insertId);
    await writeDraftBody(f, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'preview-resource-paragraph' },
          content: [
            {
              type: 'image',
              attrs: {
                resourceId: resource,
                alt: '可靠预览引用',
                title: null,
                width: 160,
                height: 100,
              },
            },
          ],
        },
      ],
    });
    const input = structurePreviewInput(f, document);
    assert.equal((await post(path, input, f.user.headers)).status, 200);
    const [refs] = await db.query(
      "SELECT CAST(resource_id AS CHAR) id FROM resource_reference WHERE document_id=? AND owner_kind='OPERATION' AND owner_ref=?",
      [f.d.id, input.operation_id],
    );
    assert.deepEqual(
      refs.map((v) => v.id),
      [resource],
    );
    assert.equal((await post(path, input, f.user.headers)).status, 200);
    const submitted = (await submitDraft(f)).value;
    assert.equal(submitted.state, 'SUCCEEDED', submitted.error?.code);
    const review = submitted.result,
      reviewPath = `/v1/review-requests/${review.request_id}`;
    const active = await post(
      path,
      { ...input, operation_id: operation('preview-active-review') },
      f.user.headers,
    );
    assert.equal(active.status, 200, active.body);
    assert.deepEqual(json(active).blocking_units, [
      { unit_id: f.grant.context.target.unit_id, reason: 'ACTIVE_REVIEW' },
    ]);
    assert.equal(
      (
        await post(
          `${reviewPath}/actions`,
          {
            operation_id: operation('preview-review-change'),
            round_id: review.round_id,
            content_version_id: review.content_version_id,
            type: 'REQUEST_CHANGES',
            reason: '保留这个审核要求',
          },
          f.user.headers,
        )
      ).status,
      200,
    );
    assert.equal(
      (
        await post(
          `${reviewPath}/withdrawals`,
          {
            operation_id: operation('preview-withdraw'),
            expected_round_id: review.round_id,
            reason: '准备调整结构',
          },
          f.user.headers,
        )
      ).status,
      200,
    );
    const withdrawn = await post(
      path,
      { ...input, operation_id: operation('preview-withdrawn-review') },
      f.user.headers,
    );
    assert.equal(withdrawn.status, 200, withdrawn.body);
    assert.deepEqual(json(withdrawn).blocking_units, [
      { unit_id: f.grant.context.target.unit_id, reason: 'OPEN_CHANGE_REQUEST' },
    ]);
    const requestState = json(await get(reviewPath, f.user.headers));
    assert.equal(requestState.status, 'WITHDRAWN');
    assert.equal(requestState.blocking_change_count, 1);
    assert.equal(requestState.archived_at, null);
  } finally {
    f.close();
    await db.end();
  }
});
