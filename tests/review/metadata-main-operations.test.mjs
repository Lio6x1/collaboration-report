import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraft,
  writeDraftBody,
  submitDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { unlockStructurePreview, structurePreviewInput } from '../helpers/structure-previews.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { discardPreviewInput, discardPreviewPath } from '../helpers/discard-previews.mjs';
import { pullMain } from '../helpers/pull-main.mjs';
import { pollMerge } from '../helpers/merge.mjs';

async function metadataFixture({ branchBaseline = false } = {}) {
  const f = await reviewFixture();
  try {
    await unlockStructurePreview(f);
    f.main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    await writeDraftBody(f.main, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'metadata-main' },
          content: [{ type: 'text', text: '元数据前可靠MAIN原文' }],
        },
      ],
    });
    if (branchBaseline) {
      // 先用真实PULL建立共同基线；两个空基线上的同时插入是合法合并冲突，不能当成代次缺陷。
      const result = await pullMain(f);
      assert.equal(result.value.state, 'SUCCEEDED', result.value.error?.code);
      const main = f.main;
      f.close();
      Object.assign(f, await openReviewWorkspace(f.f, f.d, f.user), { main });
    }
    const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const renamed = await post(
      `/v1/documents/${f.d.id}`,
      {
        operation_id: operation('metadata-rename'),
        expected_control_revision: before.control_revision,
        title: '元数据更新后',
      },
      f.user.headers,
      'PATCH',
    );
    assert.equal(renamed.status, 200, json(renamed).code);
    const close = f.close;
    f.close = () => {
      f.main.close();
      close();
    };
    return f;
  } catch (error) {
    f.main?.close();
    f.close();
    throw error;
  }
}

async function assertMainText(f, text) {
  const response = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
  assert.equal(response.status, 200, json(response).code);
  const view = json(response);
  const unit = await get(
    `/v1/documents/${f.d.id}/read-views/${view.read_view_id}/units/${view.units[0].unit_id}`,
    f.user.headers,
  );
  assert.equal(unit.status, 200, json(unit).code);
  assert.ok(JSON.stringify(json(unit).body_json).includes(text));
  return view;
}

// 真实MAIN控制必须接受原物理cut，并将已保存正文安装到新的实际代次。
test('style and chapter controls after metadata change retain reliable MAIN content', async () => {
  const f = await metadataFixture();
  try {
    const before = await assertMainText(f, '可靠MAIN原文');
    const style = await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/styles`,
        {
          operation_id: operation('metadata-style'),
          expected_style_id: before.style.id,
          expected_control_revision: before.document.control_revision,
          style: { ...before.style.style, body_font: 'FangSong' },
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.equal(style.state, 'SUCCEEDED', style.error?.code);
    const styled = await assertMainText(f, '可靠MAIN原文');
    assert.equal(styled.style.style.body_font, 'FangSong');
    const chapter = await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/chapters`,
        {
          operation_id: operation('metadata-chapter'),
          expected_structure_id: styled.structure.id,
          title: '追加章节',
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.equal(chapter.state, 'SUCCEEDED', chapter.error?.code);
    const expanded = await assertMainText(f, '可靠MAIN原文');
    assert.equal(expanded.units.length, 2);
  } finally {
    f.close();
  }
});

// 草稿预览与PULL必须从同一物理MAIN读取；管理修订不能让预览退回旧清单正文。
test('discard and structure previews plus pull after metadata change bind the saved physical MAIN', async () => {
  const f = await metadataFixture(),
    db = await reviewDatabase();
  try {
    const preview = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
    assert.equal(preview.status, 200, json(preview).code);
    const value = json(preview);
    const [[capture]] = await db.query(
      "SELECT FROM_BASE64(JSON_UNQUOTE(JSON_EXTRACT(payload,'$.main.body_json_utf8'))) body FROM durable_task WHERE kind='DISCARD_PREVIEW_FIXED' AND business_key=?",
      [value.preview_cut_ref],
    );
    assert.ok(capture && capture.body.toString().includes('元数据前可靠MAIN原文'));
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const structure = await post(
      `/v1/documents/${f.d.id}/structure-previews`,
      structurePreviewInput(f, current),
      f.user.headers,
    );
    assert.equal(structure.status, 200, json(structure).code);
    const pulled = await pullMain(f);
    assert.equal(pulled.value.state, 'SUCCEEDED', pulled.value.error?.code);
    await assertMainText(f, '可靠MAIN原文');
  } finally {
    f.close();
    await db.end();
  }
});

test('submitted branch merges after metadata change without discarding the saved MAIN block', async () => {
  const f = await metadataFixture({ branchBaseline: true }),
    db = await reviewDatabase();
  try {
    await writeDraftBody(f, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'metadata-main' },
          content: [{ type: 'text', text: '元数据前可靠MAIN原文' }],
        },
        {
          type: 'paragraph',
          attrs: { id: 'metadata-branch' },
          content: [{ type: 'text', text: '送审支线新增块' }],
        },
      ],
    });
    const submitted = (await submitDraft(f)).value.result;
    const main = await assertMainText(f, '可靠MAIN原文');
    const id = operation('metadata-merge');
    const response = await post(
      `/v1/review-requests/${submitted.request_id}/approve-and-merge`,
      {
        operation_id: id,
        round_id: submitted.round_id,
        content_version_id: submitted.content_version_id,
        own_changes_to_resolve: [],
      },
      f.user.headers,
    );
    assert.equal(response.status, 202, json(response).code);
    const result = await pollMerge(id, f.user.headers, db);
    assert.equal(result.state, 'SUCCEEDED', result.error?.code);
    await assertMainText(f, '可靠MAIN原文');
    await assertMainText(f, '送审支线新增块');
  } finally {
    f.close();
    await db.end();
  }
});

// 真实持久元数据故障：摘要或代次被篡改时必须拒绝读取，恢复原字节后才可继续。
test('unknown installed binding and future physical room context fail closed after metadata change', async () => {
  const f = await metadataFixture(),
    doc = await reviewDatabase(),
    node = await reviewDatabase('collaboration');
  let originalReceipt, originalRoom;
  try {
    const [[receipt]] = await doc.query(
      'SELECT o.operation_id,o.request_fingerprint FROM document d JOIN main_manifest m ON m.id=d.current_manifest_id JOIN operation_receipt o ON o.operation_id=m.source_operation_id WHERE d.id=?',
      [f.d.id],
    );
    originalReceipt = receipt;
    await doc.query('UPDATE operation_receipt SET request_fingerprint=? WHERE operation_id=?', [
      Buffer.alloc(32),
      receipt.operation_id,
    ]);
    const invalid = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
    assert.equal(invalid.status, 503, json(invalid).code);
    await doc.query('UPDATE operation_receipt SET request_fingerprint=? WHERE operation_id=?', [
      receipt.request_fingerprint,
      receipt.operation_id,
    ]);
    originalReceipt = undefined;
    await assertMainText(f, '可靠MAIN原文');
    const [[room]] = await node.query(
      "SELECT t.id,CAST(t.payload AS CHAR) payload FROM room r JOIN durable_task t ON t.kind='ROOM_MAIN_CONTEXT' AND t.business_key=CONCAT(r.id,':',r.epoch) WHERE r.document_id=? AND r.workspace_kind='MAIN' AND r.state='OPEN'",
      [f.d.id],
    );
    originalRoom = room;
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await node.query(
      "UPDATE durable_task SET payload=JSON_SET(payload,'$.control_revision',?) WHERE id=?",
      [current.control_revision, room.id],
    );
    const future = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
    assert.equal(future.status, 503, json(future).code);
    await node.query('UPDATE durable_task SET payload=? WHERE id=?', [room.payload, room.id]);
    originalRoom = undefined;
    await assertMainText(f, '可靠MAIN原文');
  } finally {
    if (originalReceipt)
      await doc.query('UPDATE operation_receipt SET request_fingerprint=? WHERE operation_id=?', [
        originalReceipt.request_fingerprint,
        originalReceipt.operation_id,
      ]);
    if (originalRoom)
      await node.query('UPDATE durable_task SET payload=? WHERE id=?', [
        originalRoom.payload,
        originalRoom.id,
      ]);
    f.close();
    await node.end();
    await doc.end();
  }
});
