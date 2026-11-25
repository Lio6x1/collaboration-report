import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, reviewDatabase, writeDraft } from '../helpers/review.mjs';
import { post, json, operation } from '../helpers/workspace.mjs';
import {
  discardPreviewPath,
  discardPreviewInput,
  discardDraftPath,
  discardDraftInput,
} from '../helpers/discard-previews.mjs';

test('discard binds original session, preview kind, document, branch and structure', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  try {
    await writeDraft(f, '绑定边界的旧稿');
    const response = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
    assert.equal(response.status, 200, response.body);
    const preview = json(response),
      input = discardDraftInput(preview);
    const login = await post('/v1/account/login', {
      email: f.user.email,
      password: f.user.password,
    });
    assert.equal(login.status, 200, login.body);
    const anotherSession = {
      cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
      'x-csrf-token': json(login).csrf_token,
    };
    const wrongSession = await post(discardDraftPath(preview), input, anotherSession);
    assert.equal(wrongSession.status, 409, wrongSession.body);
    assert.equal(json(wrongSession).code, 'DISCARD_PREVIEW_BINDING_CHANGED');
    for (const patch of [
      { expected_branch_id: '99999999' },
      { expected_structure_id: '99999999' },
      { preview_cut_ref: operation('unknown-preview') },
    ]) {
      const rejected = await post(
        discardDraftPath(preview),
        { ...input, ...patch, operation_id: operation('bad-confirm') },
        f.user.headers,
      );
      assert.equal(rejected.status, 409, rejected.body);
    }
    const wrongUnit = await post(
      `/v1/documents/${f.d.id}/units/99999999/discard-draft`,
      input,
      f.user.headers,
    );
    assert.equal(wrongUnit.status, 409, wrongUnit.body);
    const [[branch]] = await db.query('SELECT lifecycle,workspace_epoch FROM branch WHERE id=?', [
      preview.branch_id,
    ]);
    assert.equal(branch.lifecycle, 'ACTIVE');
    assert.equal(String(branch.workspace_epoch), preview.workspace_epoch);
    const [[count]] = await db.query(
      "SELECT COUNT(*) total FROM branch_lineage WHERE branch_id=? AND event_kind='DISCARDED'",
      [preview.branch_id],
    );
    assert.equal(Number(count.total), 0);
  } finally {
    f.close();
    await db.end();
  }
});
