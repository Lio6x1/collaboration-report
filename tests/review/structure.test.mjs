import test from 'node:test';
import assert from 'node:assert/strict';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraft,
  submitDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { pollMerge } from '../helpers/merge.mjs';

test('rename and move append a structure snapshot, preserve fixed reviews and retained drafts', async () => {
  const f = await reviewFixture(1),
    db = await reviewDatabase();
  let currentDraft;
  try {
    await writeDraft(f, '目录改名移动之前固定的S1');
    const submitted = (await submitDraft(f)).value.result;
    await writeDraft(f, '目录改动以后仍要保留的S2');
    await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/chapters`,
        {
          operation_id: operation('chapter-for-move'),
          expected_structure_id: f.d.main.structure.id,
          title: '第二章',
        },
        f.user.headers,
      ),
      f.user,
    );
    const before = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    const doc = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const first = before.structure.entries[0].chapter_id,
      second = before.structure.entries[1].chapter_id;
    const input = {
      operation_id: operation('rename-and-move'),
      expected_structure_id: before.structure.id,
      expected_control_revision: doc.control_revision,
      changes: [
        { type: 'RENAME', chapter_id: first, title: '新的章节名' },
        { type: 'MOVE', chapter_id: first, after_chapter_id: second },
      ],
    };
    const path = `/v1/documents/${f.d.id}/structure-changes`;
    // 当前管理资格和期待版本必须先成立；移动不改变单元的范围身份。
    assert.equal((await post(path, input, f.f.members[0].headers)).status, 403);
    assert.equal((await post(path, input, f.user.headers)).status, 409);
    await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/main-lock`,
        {
          operation_id: operation('unlock-structure'),
          expected_control_revision: doc.control_revision,
          locked: false,
        },
        f.user.headers,
      ),
      f.user,
    );
    input.expected_control_revision = json(
      await get(`/v1/documents/${f.d.id}`, f.user.headers),
    ).control_revision;
    const changed = await completedMainControl(await post(path, input, f.user.headers), f.user);
    assert.equal(changed.state, 'SUCCEEDED', changed.error?.code);
    const after = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    assert.notEqual(after.structure.id, before.structure.id);
    assert.deepEqual(
      after.structure.entries.map((e) => e.chapter_id),
      [second, first],
    );
    assert.equal(after.structure.entries[1].title, '新的章节名');
    assert.deepEqual(
      (await completedMainControl(await post(path, input, f.user.headers), f.user)).result,
      changed.result,
    );
    assert.equal(
      (await post(path, { ...input, operation_id: operation('stale-structure') }, f.user.headers))
        .status,
      409,
    );
    const [[old]] = await db.query(
      'SELECT title FROM structure_entry WHERE structure_id=? AND chapter_id=?',
      [before.structure.id, first],
    );
    assert.notEqual(old.title, '新的章节名');
    const merge = operation('merge-after-move');
    assert.equal(
      (
        await post(
          `/v1/review-requests/${submitted.request_id}/approve-and-merge`,
          {
            operation_id: merge,
            round_id: submitted.round_id,
            content_version_id: submitted.content_version_id,
            own_changes_to_resolve: [],
          },
          f.user.headers,
        )
      ).status,
      202,
    );
    assert.equal((await pollMerge(merge, f.user.headers, db)).state, 'SUCCEEDED');
    const mergedMain = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    const firstUnit = mergedMain.units.find((u) => u.unit_id === f.d.main.units[0].unit_id);
    currentDraft = await openReviewWorkspace(f.f, {
      ...f.d,
      main: { ...mergedMain, units: [firstUnit] },
    });
    assert.match(JSON.stringify(yDocToProsemirrorJSON(currentDraft.doc)), /仍要保留的S2/);
  } finally {
    currentDraft?.close();
    f.close();
    await db.end();
  }
});
