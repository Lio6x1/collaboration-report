import test from 'node:test';
import assert from 'node:assert/strict';
import { importMarkdown } from '../../packages/content/markdown.mjs';
import { canonical } from '../../packages/content/index.mjs';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraftBody,
  reviewDatabase,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { unlockStructurePreview, structurePreviewInput } from '../helpers/structure-previews.mjs';

test('actual structure preview fixes split candidates from reliable MAIN and S2 with the original digest', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let main;
  try {
    const document = await unlockStructurePreview(f);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    let next = 0;
    const accepted = importMarkdown(
      '## 空节\n\n## 内容\n\n### 内部标题\n\n**已接纳正文**\n\n|甲|乙|\n|---|---|\n|一|二|',
      { newId: () => `actual-split-${++next}` },
    ).body;
    const pending = structuredClone(accepted);
    pending.content.push({
      type: 'paragraph',
      attrs: { id: 'actual-S2' },
      content: [{ type: 'text', text: '只属于待审核草稿' }],
    });
    await writeDraftBody(main, accepted);
    await writeDraftBody(f, pending);
    const input = structurePreviewInput(f, document, 'SPLIT_LEVEL2');
    const response = await post(
      `/v1/documents/${f.d.id}/structure-previews`,
      input,
      f.user.headers,
    );
    assert.equal(response.status, 200, json(response).code);
    const [[row]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='STRUCTURE_PREVIEW_FIXED' AND business_key=?",
      [input.operation_id],
    );
    const fixed = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
    assert.equal(fixed.migration?.schema_version, 'structure-migration-v1');
    assert.equal(fixed.migration.changes.length, 1);
    const split = fixed.migration.changes[0];
    assert.deepEqual(split.source_units, [f.d.main.units[0].unit_id]);
    assert.deepEqual(
      split.targets.map((v) => v.kind),
      ['SECTION', 'SECTION'],
    );
    assert.match(canonical(split.targets[1].draft_body), /只属于待审核草稿/);
    assert.doesNotMatch(canonical(split.targets[1].main_body), /只属于待审核草稿/);
    assert.equal(split.targets[0].boundary.heading.attrs.id, accepted.content[0].attrs.id);
    // 后续保存只能进入另一个新预览，不得改原候选、切点或到期时间。
    pending.content.at(-1).content[0].text = '固定之后才输入的草稿';
    await writeDraftBody(f, pending);
    const replay = await post(`/v1/documents/${f.d.id}/structure-previews`, input, f.user.headers);
    assert.deepEqual(json(replay), json(response));
    const [[again]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='STRUCTURE_PREVIEW_FIXED' AND business_key=?",
      [input.operation_id],
    );
    assert.equal(
      canonical(typeof again.payload === 'string' ? JSON.parse(again.payload) : again.payload),
      canonical(fixed),
    );
    const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    assert.equal(after.current_structure_id, document.current_structure_id);
    assert.equal(after.current_manifest_id, document.current_manifest_id);
    assert.equal(after.control_revision, document.control_revision);
  } finally {
    main?.close();
    f.close();
    await db.end();
  }
});

test('changed reliable draft boundaries reject the same preview without an endless retry or MAIN mutation', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let main;
  try {
    const document = await unlockStructurePreview(f);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    let next = 0;
    const accepted = importMarkdown('## 小节\n\n原正文', {
      newId: () => `changed-split-${++next}`,
    }).body;
    await writeDraftBody(main, accepted);
    const pending = structuredClone(accepted);
    pending.content[0].attrs.id = 'changed-draft-heading';
    await writeDraftBody(f, pending);
    const input = structurePreviewInput(f, document, 'SPLIT_LEVEL2');
    const response = await post(
      `/v1/documents/${f.d.id}/structure-previews`,
      input,
      f.user.headers,
    );
    assert.equal(response.status, 409, json(response).code);
    assert.equal(json(response).code, 'DRAFT_STRUCTURE_CHANGED');
    const [[row]] = await db.query('SELECT state FROM operation_receipt WHERE operation_id=?', [
      input.operation_id,
    ]);
    assert.equal(row.state, 'REJECTED');
    const [[facts]] = await db.query(
      "SELECT COUNT(*) AS n FROM durable_task WHERE kind='STRUCTURE_PREVIEW_FIXED' AND business_key=?",
      [input.operation_id],
    );
    assert.equal(facts.n, '0');
    const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    assert.equal(after.current_structure_id, document.current_structure_id);
    assert.equal(after.current_manifest_id, document.current_manifest_id);
  } finally {
    main?.close();
    f.close();
    await db.end();
  }
});
