import test from 'node:test';
import assert from 'node:assert/strict';
import { mergedReverseFixture, pollReverse } from '../helpers/reverse-changes.mjs';
import { openReviewWorkspace, writeDraftBody } from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { unlockStructurePreview, structurePreviewInput } from '../helpers/structure-previews.mjs';

const input = (id, manifest, record) => ({
  operation_id: id,
  expected_manifest_id: manifest,
  merge_record_id: String(record.id),
  reason: '验证当前完整范围与可靠正文边界',
});

test('reverse does not reintroduce S1 when reliable MAIN already equals the original before body', async () => {
  const { f, db, record } = await mergedReverseFixture();
  let main;
  try {
    await unlockStructurePreview(f);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    const [[before]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      f.d.main.units[0].content_version_id,
    ]);
    // 真实MAIN写入并收到SAVED；旧支线仍含原S1，不能把它重包装为新的反向审核。
    await writeDraftBody(main, JSON.parse(before.canonical_body.toString()));
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers)),
      id = operation('reverse-no-change'),
      response = await post(
        `/v1/documents/${f.d.id}/reverse-changes`,
        input(id, current.current_manifest_id, record),
        f.user.headers,
      );
    assert.equal(response.status, 202, response.body);
    const result = await pollReverse(id, f.user.headers);
    assert.equal(result.state, 'REJECTED', JSON.stringify(result));
    assert.equal(result.error.code, 'REVERSE_NO_CHANGES');
    const [[fixed]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='REVERSE_FIXED' AND business_key=?",
      [id],
    );
    const value = typeof fixed.payload === 'string' ? JSON.parse(fixed.payload) : fixed.payload;
    assert.equal(value.candidate.body_hash, value.main.body_hash);
    assert.match(
      Buffer.from(value.draft.body_json_utf8, 'base64').toString(),
      /这项已经合并的修改需要反向/,
    );
    const [[rounds]] = await db.query(
      'SELECT COUNT(*) n FROM review_round WHERE submission_operation_id=?',
      [id],
    );
    assert.equal(Number(rounds.n), 0);
    const [[branch]] = await db.query('SELECT lifecycle,workspace_epoch FROM branch WHERE id=?', [
      f.session.branch_id,
    ]);
    assert.equal(branch.lifecycle, 'ACTIVE');
    assert.equal(String(branch.workspace_epoch), f.session.workspace_epoch);
    assert.equal(
      json(await get(`/v1/documents/${f.d.id}`, f.user.headers)).current_manifest_id,
      current.current_manifest_id,
    );
  } finally {
    main?.close();
    f.close();
    await db.end();
  }
});

test('reverse accepts a later structure snapshot that preserves the complete original unit', async () => {
  const { f, db, merged, record } = await mergedReverseFixture();
  let next;
  try {
    // 新增另一章只推进目录与整篇清单；原单元完整身份和可靠正文没有改变。
    const created = await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/chapters`,
        {
          operation_id: operation('reverse-unrelated-chapter'),
          expected_structure_id: f.d.main.structure.id,
          title: '不影响原反向范围的新章',
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.equal(created.state, 'SUCCEEDED', JSON.stringify(created));
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers)),
      currentMain = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    assert.notEqual(current.current_manifest_id, merged.result.manifest_id);
    assert.notEqual(current.current_structure_id, f.d.main.structure.id);
    const id = operation('reverse-stable-range'),
      response = await post(
        `/v1/documents/${f.d.id}/reverse-changes`,
        input(id, current.current_manifest_id, record),
        f.user.headers,
      );
    assert.equal(response.status, 202, response.body);
    const result = await pollReverse(id, f.user.headers);
    assert.equal(result.state, 'SUCCEEDED', JSON.stringify(result));
    const [[version]] = await db.query(
      'SELECT structure_id,unit_id FROM content_version WHERE id=?',
      [result.result.content_version_id],
    );
    assert.equal(String(version.structure_id), current.current_structure_id);
    assert.equal(String(version.unit_id), f.d.main.units[0].unit_id);
    next = await openReviewWorkspace(f.f, {
      ...f.d,
      main: {
        ...currentMain,
        units: [currentMain.units.find((v) => v.unit_id === f.d.main.units[0].unit_id)],
      },
    });
    assert.equal(
      json(await get(`/v1/documents/${f.d.id}`, f.user.headers)).current_manifest_id,
      current.current_manifest_id,
    );
  } finally {
    next?.close();
    f.close();
    await db.end();
  }
});

test('reverse explicitly rejects a removed historical range before registering a new source operation', async () => {
  const { f, db, record } = await mergedReverseFixture();
  try {
    const document = await unlockStructurePreview(f),
      change = structurePreviewInput(f, document, 'REMOVE'),
      response = await post(`/v1/documents/${f.d.id}/structure-previews`, change, f.user.headers);
    assert.equal(response.status, 200, response.body);
    const preview = json(response),
      removed = await completedMainControl(
        await post(
          `/v1/documents/${f.d.id}/structure-changes`,
          {
            ...change,
            operation_id: operation('remove-reverse-range'),
            changes: change.changes.map((value) => ({
              ...value,
              preview_cut_ref: preview.preview_cut_ref,
            })),
          },
          f.user.headers,
        ),
        f.user,
      );
    assert.equal(removed.state, 'SUCCEEDED', JSON.stringify(removed));
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers)),
      id = operation('reverse-missing-range'),
      rejected = await post(
        `/v1/documents/${f.d.id}/reverse-changes`,
        input(id, current.current_manifest_id, record),
        f.user.headers,
      );
    // 旧合并仍可作为历史事实读取，但不存在可写的完整当前单元，须明确范围冲突而非503重试。
    assert.equal(rejected.status, 409, rejected.body);
    assert.equal(json(rejected).code, 'REVERSE_RANGE_CHANGED');
    const [[receipts]] = await db.query(
      'SELECT COUNT(*) n FROM operation_receipt WHERE operation_id=?',
      [id],
    );
    assert.equal(Number(receipts.n), 0);
  } finally {
    f.close();
    await db.end();
  }
});
