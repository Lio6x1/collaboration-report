import test from 'node:test';
import assert from 'node:assert/strict';
import { fromPM } from '../../packages/content/index.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  reviewFixture,
  writeDraftBody,
  submitDraft,
  openReviewWorkspace,
  reviewDatabase,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { pullMain } from '../helpers/pull-main.mjs';

const paragraph = (id, text) => ({
  type: 'paragraph',
  attrs: { id },
  content: [{ type: 'text', text }],
});
const body = (...content) => ({ type: 'doc', content });
const currentBody = (f) => fromPM(yDocToProsemirrorJSON(f.doc));

test('pull after accepting S1 retains S2 and compatible reliable MAIN additions for the next normal review', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let main, draft, next;
  try {
    await writeDraftBody(f, body(paragraph('pull-paragraph', 'S1固定轮次')));
    const round = (await submitDraft(f)).value.result;
    await writeDraftBody(f, body(paragraph('pull-paragraph', 'S2后来草稿')));
    const id = operation('pull-source-merge');
    assert.equal(
      (
        await post(
          `/v1/review-requests/${round.request_id}/approve-and-merge`,
          {
            operation_id: id,
            round_id: round.round_id,
            content_version_id: round.content_version_id,
            own_changes_to_resolve: [],
          },
          f.user.headers,
        )
      ).status,
      202,
    );
    assert.equal((await pollMerge(id, f.user.headers, db)).state, 'SUCCEEDED');
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/main-lock`,
        {
          operation_id: operation('pull-unlock'),
          expected_control_revision: current.control_revision,
          locked: false,
        },
        f.user.headers,
      ),
      f.user,
    );
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    await writeDraftBody(
      main,
      body(
        paragraph('pull-paragraph', 'S1固定轮次'),
        paragraph('main-new-paragraph', '可靠主线兼容新增'),
      ),
    );
    draft = await openReviewWorkspace(f.f, f.d);
    const [[before]] = await db.query('SELECT provenance_json FROM branch_lineage WHERE id=?', [
      draft.session.lineage_id,
    ]);
    const result = await pullMain(draft);
    assert.equal(result.value.state, 'SUCCEEDED', result.value.error?.code);
    next = await openReviewWorkspace(f.f, f.d);
    assert.deepEqual(
      currentBody(next),
      body(
        paragraph('pull-paragraph', 'S2后来草稿'),
        paragraph('main-new-paragraph', '可靠主线兼容新增'),
      ),
    );
    const [[after]] = await db.query('SELECT provenance_json FROM branch_lineage WHERE id=?', [
      next.session.lineage_id,
    ]);
    const provenance = (value) => (typeof value === 'string' ? JSON.parse(value) : value);
    assert.deepEqual(
      provenance(after.provenance_json).accepted_captures,
      provenance(before.provenance_json).accepted_captures,
    );
    const [[baseline]] = await db.query('SELECT provenance_json FROM content_version WHERE id=?', [
      next.session.base_version_id,
    ]);
    const newMainSource = provenance(baseline.provenance_json).block_sources.find(
      (source) => source.block_id === 'main-new-paragraph',
    );
    // 新可靠MAIN块尚未存在于正式旧版本，来源必须指向本次固定采样，不能虚构旧版本含有它。
    assert.equal(newMainSource.origin_capture_ref, result.input.operation_id);
    assert.equal(newMainSource.origin_content_version_id, undefined);
    const second = (await submitDraft(next)).value.result;
    assert.notEqual(second.request_id, round.request_id);
    const secondID = operation('after-pull-merge');
    assert.equal(
      (
        await post(
          `/v1/review-requests/${second.request_id}/approve-and-merge`,
          {
            operation_id: secondID,
            round_id: second.round_id,
            content_version_id: second.content_version_id,
            own_changes_to_resolve: [],
          },
          f.user.headers,
        )
      ).status,
      202,
    );
    const accepted = await pollMerge(secondID, f.user.headers, db);
    assert.equal(accepted.state, 'SUCCEEDED', accepted.error?.code);
    const [[version]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      accepted.result.content_version_id,
    ]);
    assert.deepEqual(JSON.parse(version.canonical_body.toString()), currentBody(next));
  } finally {
    next?.close();
    draft?.close();
    main?.close();
    f.close();
    await db.end();
  }
});

test('pull while an old review is open preserves its fixed round and later merges only that round', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let next;
  try {
    await writeDraftBody(f, body(paragraph('open-round', '原审核轮次S1')));
    const round = (await submitDraft(f)).value.result;
    await writeDraftBody(f, body(paragraph('open-round', '未送审的S2')));
    const result = await pullMain(f);
    assert.equal(result.value.state, 'SUCCEEDED', result.value.error?.code);
    next = await openReviewWorkspace(f.f, f.d);
    assert.deepEqual(currentBody(next), body(paragraph('open-round', '未送审的S2')));
    const fixed = await get(
      `/v1/review-requests/${round.request_id}/rounds/${round.round_id}`,
      f.user.headers,
    );
    assert.equal(fixed.status, 200);
    const id = operation('old-round-after-pull');
    assert.equal(
      (
        await post(
          `/v1/review-requests/${round.request_id}/approve-and-merge`,
          {
            operation_id: id,
            round_id: round.round_id,
            content_version_id: round.content_version_id,
            own_changes_to_resolve: [],
          },
          f.user.headers,
        )
      ).status,
      202,
    );
    const merged = await pollMerge(id, f.user.headers, db);
    assert.equal(merged.state, 'SUCCEEDED', merged.error?.code);
    const [[version]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      merged.result.content_version_id,
    ]);
    assert.deepEqual(
      JSON.parse(version.canonical_body.toString()),
      body(paragraph('open-round', '原审核轮次S1')),
    );
    assert.deepEqual(currentBody(next), body(paragraph('open-round', '未送审的S2')));
  } finally {
    next?.close();
    f.close();
    await db.end();
  }
});
