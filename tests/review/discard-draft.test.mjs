import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  reviewDatabase,
  writeDraft,
  openReviewWorkspace,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { discardPreviewPath, discardPreviewInput } from '../helpers/discard-previews.mjs';
import { fromPM } from '../../packages/content/index.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';

async function completed(id, headers) {
  const deadline = Date.now() + 20000;
  do {
    const response = await get(`/v1/operations/${id}`, headers);
    assert.equal(response.status, 200, response.body);
    const value = json(response);
    if (!['PREPARING', 'WAITING'].includes(value.state)) return value;
    await new Promise((resolve) => setTimeout(resolve, 100));
  } while (Date.now() < deadline);
  assert.fail('DISCARD_INSTALL_NOT_COMPLETED');
}

test('explicit discard installs fixed MAIN in a new branch epoch and preserves old draft', async () => {
  const f = await reviewFixture();
  const database = await reviewDatabase();
  let next;
  try {
    const initial = fromPM(yDocToProsemirrorJSON(f.doc));
    await writeDraft(f, '管理员明确丢弃的可靠旧稿');
    const previewResponse = await post(
      discardPreviewPath(f),
      discardPreviewInput(f),
      f.user.headers,
    );
    assert.equal(previewResponse.status, 200, previewResponse.body);
    const preview = json(previewResponse);
    const input = {
      operation_id: operation('discard-confirm'),
      expected_branch_id: preview.branch_id,
      expected_structure_id: preview.structure_id,
      preview_cut_ref: preview.preview_cut_ref,
      changes_to_resolve: [],
    };
    const path = `/v1/documents/${f.d.id}/units/${preview.unit_id}/discard-draft`;
    const response = await post(path, input, f.user.headers);
    assert.equal(response.status, 202, response.body);
    const result = await completed(input.operation_id, f.user.headers);
    assert.equal(result.state, 'SUCCEEDED', JSON.stringify(result));
    next = await openReviewWorkspace(f.f, f.d);
    assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
    assert.deepEqual(fromPM(yDocToProsemirrorJSON(next.doc)), initial);
    const [[old]] = await database.query('SELECT canonical_body FROM content_version WHERE id=?', [
      preview.draft_content_version_id,
    ]);
    assert.match(old.canonical_body.toString(), /管理员明确丢弃的可靠旧稿/);
    const [[lineage]] = await database.query(
      'SELECT event_kind,source_operation_id FROM branch_lineage WHERE id=?',
      [next.session.lineage_id],
    );
    assert.equal(lineage.event_kind, 'DISCARDED');
    assert.equal(lineage.source_operation_id, input.operation_id);
    const current = await get(`/v1/documents/${f.d.id}`, f.user.headers);
    assert.equal(json(current).current_manifest_id, preview.main_manifest_id);
    await writeDraft(next, '恢复后新的S2');
    assert.equal((await post(path, input, f.user.headers)).status, 202);
    assert.equal((await completed(input.operation_id, f.user.headers)).state, 'SUCCEEDED');
    assert.match(JSON.stringify(fromPM(yDocToProsemirrorJSON(next.doc))), /恢复后新的S2/);
  } finally {
    next?.close();
    f.close();
    await database.end();
  }
});

test('discard rejects a preview followed by a reliable draft edit without changing epoch', async () => {
  const f = await reviewFixture();
  const database = await reviewDatabase();
  try {
    await writeDraft(f, '固定预览S1');
    const response = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
    assert.equal(response.status, 200, response.body);
    const preview = json(response);
    await writeDraft(f, '不可被旧预览抹掉的S2');
    const input = {
      operation_id: operation('changed-discard'),
      expected_branch_id: preview.branch_id,
      expected_structure_id: preview.structure_id,
      preview_cut_ref: preview.preview_cut_ref,
      changes_to_resolve: [],
    };
    const accepted = await post(
      `/v1/documents/${f.d.id}/units/${preview.unit_id}/discard-draft`,
      input,
      f.user.headers,
    );
    assert.equal(accepted.status, 202, accepted.body);
    const result = await completed(input.operation_id, f.user.headers);
    assert.equal(result.state, 'REJECTED', JSON.stringify(result));
    const [[branch]] = await database.query(
      'SELECT workspace_epoch,lifecycle FROM branch WHERE id=?',
      [preview.branch_id],
    );
    assert.equal(String(branch.workspace_epoch), preview.workspace_epoch);
    assert.equal(branch.lifecycle, 'ACTIVE');
    assert.match(JSON.stringify(fromPM(yDocToProsemirrorJSON(f.doc))), /不可被旧预览抹掉的S2/);
  } finally {
    f.close();
    await database.end();
  }
});
