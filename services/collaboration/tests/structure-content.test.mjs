import test from 'node:test';
import assert from 'node:assert/strict';
import { importMarkdown } from '../../../packages/content/markdown.mjs';
import { canonical, resourceIds } from '../../../packages/content/index.mjs';
import {
  splitChapterBody,
  joinChapterBodies,
  splitChapterDrafts,
} from '../src/domain/structure_content.mjs';

test('level-two split and join preserve exact rich blocks, headings and resources', () => {
  let next = 0;
  const body = importMarkdown(
    '**概述**\n\n## 第一节\n\n![图](https://example.test/p.png)\n\n### 内部标题\n\n|甲|乙|\n|---|---|\n|一|二|\n\n## 第二节\n\n```go\nfmt.Println("好")\n```\n\n$$\nx+y\n$$',
    {
      newId: () => `structure-block-${++next}`,
      resources: new Map([['https://example.test/p.png', '9007199254740997']]),
    },
  ).body;
  const original = canonical(body),
    parts = splitChapterBody(body);
  // 目录标题拥有原H2身份；正文保留标题下的完整富文本子树，不按字符串编号截断。
  assert.deepEqual(
    parts.map((p) => p.kind),
    ['INTRODUCTION', 'SECTION', 'SECTION'],
  );
  assert.deepEqual(
    parts.map((p) => p.title),
    ['', '第一节', '第二节'],
  );
  assert.equal(
    parts[1].body.content.some((n) => n.type === 'heading' && n.attrs.level === 3),
    true,
  );
  assert.deepEqual(resourceIds(parts[1].body), ['9007199254740997']);
  assert.equal(canonical(joinChapterBodies(parts)), original);
  assert.equal(canonical(body), original);
  const mapped = parts.flatMap((p) => p.blockIds);
  assert.equal(new Set(mapped).size, mapped.length);
  for (const part of parts.filter((p) => p.kind === 'SECTION'))
    assert.ok(part.blockIds.includes(part.heading.attrs.id));
});

test('split omits an absent introduction, preserves an empty section and rejects malformed headings', () => {
  let next = 0;
  const body = importMarkdown('## 空节\n\n## 非空节\n\n文字', {
    newId: () => `empty-block-${++next}`,
  }).body;
  const parts = splitChapterBody(body);
  assert.equal(parts.length, 2);
  assert.equal(parts[0].body.content.length, 1);
  assert.equal(parts[0].body.content[0].attrs.id, parts[0].emptyBlockId);
  assert.equal(canonical(joinChapterBodies(parts)), canonical(body));
  const empty = structuredClone(body);
  empty.content[0].content = [];
  assert.throws(() => splitChapterBody(empty), /STRUCTURE_TITLE_INVALID/);
  const absent = importMarkdown('只有整章正文', { newId: () => `whole-block-${++next}` }).body;
  assert.throws(() => splitChapterBody(absent), /STRUCTURE_LEVEL2_MISSING/);
});

test('migration keeps S2 blocks in their original section and rejects changed draft boundaries', () => {
  let next = 0;
  const main = importMarkdown('## 第一节\n\n第一节已接纳\n\n## 第二节\n\n第二节已接纳', {
    newId: () => `pair-block-${++next}`,
  }).body;
  const draft = structuredClone(main);
  draft.content.splice(2, 0, {
    type: 'paragraph',
    attrs: { id: 'later-S2' },
    content: [{ type: 'text', text: 'S2第一节后续草稿' }],
  });
  draft.content.unshift({
    type: 'paragraph',
    attrs: { id: 'later-intro' },
    content: [{ type: 'text', text: '新增概述草稿' }],
  });
  const migrated = splitChapterDrafts(main, draft);
  assert.deepEqual(
    migrated.map((p) => p.kind),
    ['INTRODUCTION', 'SECTION', 'SECTION'],
  );
  assert.match(JSON.stringify(migrated[1].draft), /S2第一节后续草稿/);
  assert.doesNotMatch(JSON.stringify(migrated[1].main), /S2第一节后续草稿/);
  assert.doesNotMatch(JSON.stringify(migrated[2].draft), /S2第一节后续草稿/);
  assert.match(JSON.stringify(migrated[0].draft), /新增概述草稿/);
  const changed = structuredClone(draft);
  changed.content.find((n) => n.type === 'heading').attrs.id = 'changed-boundary';
  assert.throws(() => splitChapterDrafts(main, changed), /DRAFT_STRUCTURE_CHANGED/);
  const missing = structuredClone(draft);
  missing.content = missing.content.filter(
    (n) => n.type !== 'heading' || n.attrs.id !== main.content[2].attrs.id,
  );
  assert.throws(() => splitChapterDrafts(main, missing), /DRAFT_STRUCTURE_CHANGED/);
});
