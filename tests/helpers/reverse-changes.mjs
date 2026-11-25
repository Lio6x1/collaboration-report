import assert from 'node:assert/strict';
import {
  reviewFixture,
  reviewDatabase,
  writeDraftBody,
  submitDraft,
  openReviewWorkspace,
} from './review.mjs';
import { unlockStructurePreview } from './structure-previews.mjs';
import { post, get, json, operation } from './workspace.mjs';
import { pollMerge } from './merge.mjs';

export async function mergedReverseFixture({ mainTail = false, members = 0, beforeMerge } = {}) {
  const f = await reviewFixture(members),
    db = await reviewDatabase();
  let main;
  try {
    const [[initial]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
        f.d.main.units[0].content_version_id,
      ]),
      originalBlockId = JSON.parse(initial.canonical_body.toString()).content[0].attrs.id;
    if (mainTail) {
      await unlockStructurePreview(f);
      main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      await writeDraftBody(main, {
        type: 'doc',
        content: [
          { type: 'paragraph', attrs: { id: originalBlockId }, content: [] },
          {
            type: 'paragraph',
            attrs: { id: 'reliable-main-before-merge' },
            content: [{ type: 'text', text: '原合并前可靠保存但未物化的MAIN新增' }],
          },
        ],
      });
    }
    // 原位置仍有稳定块锚点；整块替换后同边界再新增属于另一项明确的INSERT_ORDER冲突反例。
    await writeDraftBody(f, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: originalBlockId },
          content: [{ type: 'text', text: '这项已经合并的修改需要反向' }],
        },
      ],
    });
    const submitted = (await submitDraft(f)).value;
    assert.equal(submitted.state, 'SUCCEEDED');
    const mergeOperation = operation('reverse-source-merge'),
      fixed = submitted.result;
    if (beforeMerge) await beforeMerge({ f, db, fixed });
    const response = await post(
      `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
      {
        operation_id: mergeOperation,
        round_id: fixed.round_id,
        content_version_id: fixed.content_version_id,
        own_changes_to_resolve: [],
      },
      f.user.headers,
    );
    assert.equal(response.status, 202, response.body);
    const merged = await pollMerge(mergeOperation, f.user.headers, db);
    assert.equal(merged.state, 'SUCCEEDED', JSON.stringify(merged));
    const [[record]] = await db.query(
      "SELECT id,CAST(JSON_OBJECT('operation',operation_id,'round',round_id,'input',input_manifest_id,'output',output_manifest_id,'version',result_version_id,'capture',accepted_capture_ref,'lineage',lineage_id) AS CHAR) raw FROM merge_record WHERE operation_id=?",
      [mergeOperation],
    );
    return { f, db, fixed, merged, record, originalBlockId };
  } catch (error) {
    f.close();
    await db.end();
    throw error;
  } finally {
    main?.close();
  }
}

export async function pollReverse(id, headers) {
  const deadline = Date.now() + 30000;
  let value;
  do {
    const response = await get(`/v1/operations/${id}`, headers);
    assert.equal(response.status, 200, response.body);
    value = json(response);
    if (!['PREPARING', 'WAITING'].includes(value.state)) return value;
    await new Promise((resolve) => setTimeout(resolve, 100));
  } while (Date.now() < deadline);
  assert.fail('REVERSE_NOT_TERMINAL ' + JSON.stringify(value));
}
