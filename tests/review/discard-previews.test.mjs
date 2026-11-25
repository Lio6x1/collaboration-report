import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, reviewDatabase, writeDraft } from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';

test('actual manager discard preview fixes reliable draft without changing MAIN or branch', async () => {
  const f = await reviewFixture(1);
  const database = await reviewDatabase();
  try {
    await writeDraft(f, '明确预览的旧草稿S1');
    const before = await get(`/v1/documents/${f.d.id}`, f.user.headers);
    assert.equal(before.status, 200, json(before).code);
    const [[originalBranch]] = await database.query(
      'SELECT current_lineage_id FROM branch WHERE id=? AND document_id=?',
      [f.session.branch_id, f.d.id],
    );
    const input = {
      operation_id: operation('discard-preview'),
      expected_branch_id: f.session.branch_id,
      expected_structure_id: f.d.main.structure.id,
    };
    const path = `/v1/documents/${f.d.id}/units/${f.d.main.units[0].unit_id}/discard-previews`;
    const response = await post(path, input, f.user.headers);
    assert.equal(response.status, 200, json(response).code);
    const preview = json(response);
    assert.equal(preview.preview_cut_ref, input.operation_id);
    assert.equal(preview.document_id, f.d.id);
    assert.equal(preview.unit_id, f.d.main.units[0].unit_id);
    assert.equal(preview.branch_id, f.session.branch_id);
    assert.equal(preview.workspace_epoch, f.session.workspace_epoch);
    assert.equal(preview.lineage_id, String(originalBranch.current_lineage_id));
    assert.equal(preview.main_manifest_id, json(before).current_manifest_id);
    assert.equal(preview.structure_id, f.d.main.structure.id);
    assert.match(preview.cut_digest, /^[a-f0-9]{64}$/);
    assert.ok(Date.parse(preview.expires_at) > Date.now());
    assert.deepEqual(preview.blocking_changes, []);
    assert.equal((await post(path, input, f.f.members[0].headers)).status, 403);
    await writeDraft(f, '预览后的可靠草稿S2必须保留');
    const replay = await post(path, input, f.user.headers);
    assert.equal(replay.status, 200, json(replay).code);
    assert.deepEqual(json(replay), preview);
    const [[version]] = await database.query(
      'SELECT canonical_body,source_kind FROM content_version WHERE id=? AND document_id=? AND unit_id=?',
      [preview.draft_content_version_id, f.d.id, preview.unit_id],
    );
    assert.ok(version, 'ACTUAL_DISCARD_DRAFT_VERSION_MISSING');
    assert.equal(version.source_kind, 'RESTORE');
    assert.match(version.canonical_body.toString(), /明确预览的旧草稿S1/);
    assert.doesNotMatch(version.canonical_body.toString(), /可靠草稿S2/);
    const after = await get(`/v1/documents/${f.d.id}`, f.user.headers);
    assert.equal(after.status, 200, json(after).code);
    for (const field of ['current_manifest_id', 'current_structure_id', 'control_revision'])
      assert.equal(json(after)[field], json(before)[field]);
    const [[branch]] = await database.query(
      'SELECT lifecycle,workspace_epoch FROM branch WHERE id=? AND document_id=? AND unit_id=?',
      [f.session.branch_id, f.d.id, preview.unit_id],
    );
    assert.equal(branch.lifecycle, 'ACTIVE');
    assert.equal(String(branch.workspace_epoch), f.session.workspace_epoch);
    const conflict = await post(
      path,
      { ...input, expected_branch_id: '999999999' },
      f.user.headers,
    );
    assert.equal(conflict.status, 409, json(conflict).code);
    assert.equal(json(conflict).code, 'OPERATION_ID_CONFLICT');
  } finally {
    f.close();
    await database.end();
  }
});
