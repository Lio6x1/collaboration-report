import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  reviewDatabase,
  writeDraft,
  openReviewWorkspace,
} from '../helpers/review.mjs';
import { post, json, operation } from '../helpers/workspace.mjs';
import {
  discardPreviewPath,
  discardPreviewInput,
  discardDraftPath,
  discardDraftInput,
  pollDiscardDraft,
} from '../helpers/discard-previews.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';

test('late zero-tail MAIN room invalidates discard and retains both original cuts', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let main;
  try {
    // 专门构造“仍无MAIN房间但可打开”的旧文档状态；公开解锁本身会安装房间，不能拿它冒充零房间。
    await db.query('UPDATE document SET main_locked=0 WHERE id=?', [f.d.id]);
    await writeDraft(f, '零尾MAIN打开前已预览的草稿');
    const response = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
    assert.equal(response.status, 200, response.body);
    const preview = json(response);
    const [[row]] = await db.query(
      "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='DISCARD_PREVIEW_FIXED' AND business_key=?",
      [preview.preview_cut_ref],
    );
    assert.deepEqual(JSON.parse(row.raw).main_cut, []);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    const input = discardDraftInput(preview, operation('late-main-discard'));
    const accepted = await post(discardDraftPath(preview), input, f.user.headers);
    assert.equal(accepted.status, 202, accepted.body);
    const result = await pollDiscardDraft(input.operation_id, f.user.headers);
    assert.equal(result.state, 'REJECTED', JSON.stringify(result));
    assert.equal(result.error.code, 'MAIN_CHANGED');
    const [[same]] = await db.query(
      "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='DISCARD_PREVIEW_FIXED' AND business_key=?",
      [preview.preview_cut_ref],
    );
    assert.equal(same.raw, row.raw);
    const [[branch]] = await db.query('SELECT lifecycle,workspace_epoch FROM branch WHERE id=?', [
      preview.branch_id,
    ]);
    assert.equal(branch.lifecycle, 'ACTIVE');
    assert.equal(String(branch.workspace_epoch), preview.workspace_epoch);
  } finally {
    main?.close();
    f.close();
    await db.query('UPDATE document SET main_locked=1 WHERE id=?', [f.d.id]);
    await db.end();
  }
});

test('reliable MAIN S2 after preview rejects old discard without restoring an older MAIN cut', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let main;
  try {
    await unlockStructurePreview(f);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    await writeDraft(main, '预览时的主线S1');
    await writeDraft(f, '可靠共同旧稿');
    const response = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
    assert.equal(response.status, 200, response.body);
    const preview = json(response),
      input = discardDraftInput(preview);
    await writeDraft(main, 'MAIN后来可靠保存的S2');
    const accepted = await post(discardDraftPath(preview), input, f.user.headers);
    assert.equal(accepted.status, 202, accepted.body);
    const result = await pollDiscardDraft(input.operation_id, f.user.headers);
    assert.equal(result.state, 'REJECTED', JSON.stringify(result));
    assert.equal(result.error.code, 'MAIN_CHANGED');
    const [[branch]] = await db.query('SELECT lifecycle,workspace_epoch FROM branch WHERE id=?', [
      preview.branch_id,
    ]);
    assert.equal(branch.lifecycle, 'ACTIVE');
    assert.equal(String(branch.workspace_epoch), preview.workspace_epoch);
  } finally {
    main?.close();
    f.close();
    await db.end();
  }
});
