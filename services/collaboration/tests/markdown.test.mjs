import test from 'node:test';
import assert from 'node:assert/strict';
import { importMarkdown } from '../../../packages/content/markdown.mjs';
import { normalizeContent } from '../../../packages/content/index.mjs';

test('Markdown imports lists tables code and formatted formula atoms with fresh unique block IDs', () => {
  let i = 0;
  const value = importMarkdown(
    '# 标题\n\n**粗体 $x^2$** 和 ~~删除~~\n\n- 条目\n- 第二项\n\n|甲|乙|\n|---|---|\n|1|2|\n\n```go\nfmt.Println(1)\n```\n\n$$\nx+y\n$$',
    { newId: () => `block-${++i}` },
  );
  assert.equal(value.body.content[0].type, 'heading');
  assert.equal(value.body.content[0].attrs.level, 1);
  assert.equal(
    value.body.content[1].content.find((n) => n.type === 'formula').attrs.atomStrong,
    true,
  );
  assert.ok(value.body.content.some((n) => n.type === 'table'));
  assert.ok(value.body.content.some((n) => n.type === 'bullet_list'));
  assert.ok(value.body.content.some((n) => n.type === 'code_block' && n.attrs.language === 'go'));
  assert.ok(value.body.content.some((n) => n.type === 'formula_block' && n.attrs.latex === 'x+y'));
  assert.doesNotThrow(() => normalizeContent(value.body));
});
test('external images stay explicit pending text until a READY resource mapping exists and raw HTML never executes', () => {
  const source = '**![示意](https://example.test/a.png)**\n\n<script>alert(1)</script>',
    pending = importMarkdown(source, { newId: () => crypto.randomUUID() });
  assert.equal(pending.externalImages.length, 1);
  assert.ok(pending.losses.length);
  assert.ok(JSON.stringify(pending.body).includes('图片待转存'));
  assert.equal(pending.body.content[1].content[0].type, 'text');
  const ready = importMarkdown(source, {
    newId: () => crypto.randomUUID(),
    resources: new Map([['https://example.test/a.png', '9007199254740997']]),
  });
  const image = ready.body.content[0].content[0];
  assert.equal(image.type, 'image');
  assert.equal(image.attrs.resourceId, '9007199254740997');
  assert.equal(image.attrs.atomStrong, true);
  assert.equal(image.attrs.src, undefined);
  assert.doesNotThrow(() => normalizeContent(ready.body));
});
