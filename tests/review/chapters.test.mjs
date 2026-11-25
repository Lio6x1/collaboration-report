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

test('an already fixed review survives adding an unrelated chapter and merges only S1', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let next;
  try {
    await writeDraft(f, '新增别章之前固定的S1');
    const submitted = (await submitDraft(f)).value.result;
    await writeDraft(f, '已经固定轮次之后共同草稿的S2');
    const creation = await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/chapters`,
        {
          operation_id: operation('chapter-with-fixed-review'),
          expected_structure_id: f.d.main.structure.id,
          title: '不改变原审核范围的另一章',
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.equal(creation.state, 'SUCCEEDED');
    const id = operation('fixed-review-after-chapter');
    assert.equal(
      (
        await post(
          `/v1/review-requests/${submitted.request_id}/approve-and-merge`,
          {
            operation_id: id,
            round_id: submitted.round_id,
            content_version_id: submitted.content_version_id,
            own_changes_to_resolve: [],
          },
          f.user.headers,
        )
      ).status,
      202,
    );
    const merged = await pollMerge(id, f.user.headers, db);
    assert.equal(merged.state, 'SUCCEEDED', merged.error?.code);
    const [[body]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      merged.result.content_version_id,
    ]);
    assert.match(body.canonical_body.toString(), /新增别章之前固定的S1/);
    assert.doesNotMatch(body.canonical_body.toString(), /共同草稿的S2/);
    const history = json(
      await get(
        `/v1/review-requests/${submitted.request_id}/rounds/${submitted.round_id}`,
        f.user.headers,
      ),
    );
    assert.equal(history.structure.id, f.d.main.structure.id);
    const current = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    next = await openReviewWorkspace(f.f, { ...f.d, main: current });
    assert.match(JSON.stringify(yDocToProsemirrorJSON(next.doc)), /共同草稿的S2/);
  } finally {
    next?.close();
    f.close();
    await db.end();
  }
});

test('an unchanged existing unit can submit and merge its retained draft after chapter creation', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let next;
  try {
    await writeDraft(f, '保留单元的旧基线与新结构不能混淆');
    const result = await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/chapters`,
        {
          operation_id: operation('chapter-existing-unit'),
          expected_structure_id: f.d.main.structure.id,
          title: '另一章',
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.equal(result.state, 'SUCCEEDED');
    const current = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    next = await openReviewWorkspace(f.f, { ...f.d, main: current });
    const submitted = (await submitDraft(next)).value;
    assert.equal(submitted.state, 'SUCCEEDED', submitted.error?.code);
    const fixed = submitted.result,
      id = operation('retained-unit-merge');
    assert.equal(
      (
        await post(
          `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
          {
            operation_id: id,
            round_id: fixed.round_id,
            content_version_id: fixed.content_version_id,
            own_changes_to_resolve: [],
          },
          f.user.headers,
        )
      ).status,
      202,
    );
    const merged = await pollMerge(id, f.user.headers, db);
    assert.equal(merged.state, 'SUCCEEDED', merged.error?.code);
    const [[body]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      merged.result.content_version_id,
    ]);
    assert.match(body.canonical_body.toString(), /保留单元的旧基线与新结构不能混淆/);
  } finally {
    next?.close();
    f.close();
    await db.end();
  }
});

test('chapter creation requires management, binds the expected structure and keeps its original result', async () => {
  const f = await reviewFixture(1);
  try {
    const path = `/v1/documents/${f.d.id}/chapters`,
      input = {
        operation_id: operation('create-second-chapter'),
        expected_structure_id: f.d.main.structure.id,
        title: '第二章：实现与验证',
        after_chapter_id: f.d.main.structure.entries[0].chapter_id,
      };
    assert.equal((await post(path, input, f.f.members[0].headers)).status, 403);
    const created = await completedMainControl(await post(path, input, f.user.headers), f.user);
    assert.equal(created.state, 'SUCCEEDED', created.error?.code);
    assert.equal(created.result.main_sync_state, 'READY');
    const current = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    assert.equal(current.structure.entries.length, 2);
    assert.equal(current.structure.entries[1].title, input.title);
    assert.equal(current.structure.entries[1].chapter_id, created.result.entity_id);
    assert.equal(current.units.length, 2);
    const replay = await completedMainControl(await post(path, input, f.user.headers), f.user);
    assert.deepEqual(replay.result, created.result);
    assert.equal(
      (await post(path, { ...input, operation_id: operation('stale-chapter') }, f.user.headers))
        .status,
      409,
    );
    assert.equal(
      (
        await post(
          path,
          {
            ...input,
            operation_id: operation('wrong-after-chapter'),
            expected_structure_id: current.structure.id,
            after_chapter_id: '18446744073709551615',
          },
          f.user.headers,
        )
      ).status,
      409,
    );
  } finally {
    f.close();
  }
});

test('chapter creation materializes reliable MAIN, preserves existing drafts and enables a real second-unit review and merge', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let main, next, second;
  try {
    const doc = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/main-lock`,
        {
          operation_id: operation('chapter-main-unlock'),
          expected_control_revision: doc.control_revision,
          locked: false,
        },
        f.user.headers,
      ),
      f.user,
    );
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    await writeDraft(main, '新增章节时必须保留可靠MAIN尾部');
    await writeDraft(f, '原章节尚未送审的共同草稿');
    const input = {
      operation_id: operation('chapter-with-reliable-input'),
      expected_structure_id: f.d.main.structure.id,
      title: '新章节独立协作',
    };
    const result = await completedMainControl(
      await post(`/v1/documents/${f.d.id}/chapters`, input, f.user.headers),
      f.user,
    );
    assert.equal(result.state, 'SUCCEEDED', result.error?.code);
    const current = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    const [[reliable]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      current.units[0].content_version_id,
    ]);
    assert.match(reliable.canonical_body.toString(), /新增章节时必须保留可靠MAIN尾部/);
    const [[history]] = await db.query(
      'SELECT COUNT(*) AS n FROM structure_entry WHERE structure_id=?',
      [f.d.main.structure.id],
    );
    assert.equal(String(history.n), '1');
    const updated = { ...f.d, main: current };
    next = await openReviewWorkspace(f.f, updated);
    assert.equal(next.session.branch_id, f.session.branch_id);
    assert.match(JSON.stringify(yDocToProsemirrorJSON(next.doc)), /原章节尚未送审的共同草稿/);
    second = await openReviewWorkspace(f.f, {
      ...updated,
      main: { ...current, units: [current.units[1]] },
    });
    await writeDraft(second, '第二协作单元的独立送审');
    const submission = (await submitDraft(second)).value;
    assert.equal(submission.state, 'SUCCEEDED', submission.error?.code);
    const fixed = submission.result,
      merge = operation('second-unit-merge');
    assert.equal(
      (
        await post(
          `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
          {
            operation_id: merge,
            round_id: fixed.round_id,
            content_version_id: fixed.content_version_id,
            own_changes_to_resolve: [],
          },
          f.user.headers,
        )
      ).status,
      202,
    );
    assert.equal((await pollMerge(merge, f.user.headers, db)).state, 'SUCCEEDED');
    const merged = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    const [bodies] = await db.query(
      'SELECT canonical_body FROM content_version WHERE id IN (?,?)',
      merged.units.map((u) => u.content_version_id),
    );
    const text = bodies.map((row) => row.canonical_body.toString()).join('\n');
    assert.match(text, /第二协作单元的独立送审/);
    assert.match(text, /新增章节时必须保留可靠MAIN尾部/);
  } finally {
    main?.close();
    next?.close();
    second?.close();
    f.close();
    await db.end();
  }
});
