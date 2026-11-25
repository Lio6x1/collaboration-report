import assert from 'node:assert/strict';
import {
  reviewFixture,
  reviewDatabase,
  openReviewWorkspace,
  writeDraftBody,
  submitDraft,
} from './review.mjs';
import { post, operation } from './workspace.mjs';
import { pollMerge } from './merge.mjs';
import { unlockStructurePreview } from './structure-previews.mjs';

export const conflictParagraph = (id, text) => ({
  type: 'paragraph',
  attrs: { id },
  content: [{ type: 'text', text }],
});
export const conflictBody = (...content) => ({ type: 'doc', content });

export async function conflictedReviewFixture({ members = 0, owner = null } = {}) {
  const f = await reviewFixture(members, owner),
    db = await reviewDatabase();
  let main;
  try {
    const [[initial]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      f.d.main.units[0].content_version_id,
    ]);
    const block = JSON.parse(initial.canonical_body.toString()).content[0].attrs.id;
    await unlockStructurePreview(f);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    // 同一稳定块分别修改，走真实提交和批准计算出冲突；不预置attempt或冲突JSON。
    await writeDraftBody(main, conflictBody(conflictParagraph(block, '原计算的可靠MAIN修改')));
    await writeDraftBody(f, conflictBody(conflictParagraph(block, '原计算的固定提交S1')));
    const submitted = (await submitDraft(f)).value;
    assert.equal(submitted.state, 'SUCCEEDED', JSON.stringify(submitted));
    const fixed = submitted.result,
      id = operation('fixed-conflict-source');
    const response = await post(
      `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
      {
        operation_id: id,
        round_id: fixed.round_id,
        content_version_id: fixed.content_version_id,
        own_changes_to_resolve: [],
      },
      f.user.headers,
    );
    assert.equal(response.status, 202, response.body);
    const result = await pollMerge(id, f.user.headers, db);
    assert.equal(result.state, 'REJECTED', JSON.stringify(result));
    assert.equal(result.error.code, 'MERGE_CONFLICT_REQUIRES_RESOLUTION');
    assert.equal(result.result.conflict_context_ref, id);
    return { f, db, main, fixed, result, block, id };
  } catch (error) {
    main?.close();
    f.close();
    await db.end();
    throw error;
  }
}
