import test from 'node:test';
import assert from 'node:assert/strict';
import {
  compareReviewBodies,
  validateReviewBase,
} from '../../web/src/features/review/reviewDiff.ts';

const text = (value, marks = undefined) => ({
  type: 'text',
  text: value,
  ...(marks ? { marks } : {}),
});
const paragraph = (id, value, attrs = {}) => ({
  type: 'paragraph',
  attrs: { id, ...attrs },
  content: typeof value === 'string' ? [text(value)] : value,
});
const doc = (...content) => ({ type: 'doc', content });
const compare = (before, after) => compareReviewBodies(doc(...before), doc(...after));
const changed = (block, kind) =>
  block.spans
    .filter((span) => span.kind === kind)
    .map((span) => span.node.text ?? '')
    .join('');
function frozen(value) {
  Object.freeze(value);
  for (const child of Object.values(value)) if (child && typeof child === 'object') frozen(child);
  return value;
}

test('中文修改只标精确片段，同时保留未改动文字与安全文本', () => {
  const before = frozen([paragraph('p', '这里原来允许十人，之后保存正文。<script>🙂</script>')]);
  const after = frozen([paragraph('p', '这里现在允许百人，之后保存正文。<script>🙂</script>')]);
  const result = compare(before, after);
  assert.equal(result.blocks[0].kind, 'change');
  assert.equal(changed(result.blocks[0], 'delete'), '原来十');
  assert.equal(changed(result.blocks[0], 'add'), '现在百');
  assert.ok(changed(result.blocks[0], 'same').includes('<script>🙂</script>'));
  assert.equal(result.limited, false);
  assert.equal(before[0].content[0].text, '这里原来允许十人，之后保存正文。<script>🙂</script>');
  const emoji = compare([paragraph('emoji', '用户👩🏽‍💻已确认')], [paragraph('emoji', '用户👨🏽‍💻已确认')])
    .blocks[0];
  assert.equal(changed(emoji, 'delete'), '👩🏽‍💻');
  assert.equal(changed(emoji, 'add'), '👨🏽‍💻');
});

test('格式与块类型变化单独标蓝，不误报文字增删，默认属性和重分割不构成改动', () => {
  const before = paragraph('p', [text('中文'), text('内容')]);
  const next = {
    ...paragraph('p', [text('中文', [{ type: 'strong' }]), text('内容')], { align: 'center' }),
    type: 'heading',
    attrs: { id: 'p', level: 2, align: 'center' },
  };
  const block = compare([before], [next]).blocks[0];
  assert.equal(block.kind, 'format');
  assert.equal(changed(block, 'format'), '中文');
  assert.equal(changed(block, 'add'), '');
  assert.equal(changed(block, 'delete'), '');
  assert.ok(block.format.some((value) => value.includes('段落 → 标题')));
  assert.ok(block.format.some((value) => value.includes('居中')));
  assert.equal(
    compare(
      [before],
      [paragraph('different-id', '中文内容', { align: 'left', firstLineIndent: false })],
    ).blocks[0].kind,
    'same',
  );
});

test('新增、删除、真实换序分别报告；插入导致序号变化不误报移动', () => {
  const a = paragraph('a', '第一段'),
    b = paragraph('b', '第二段'),
    c = paragraph('c', '第三段');
  const moved = compare([a, b, c], [c, a, b]).blocks;
  assert.equal(moved.length, 3);
  assert.equal(moved.filter((block) => block.moved).length, 1);
  assert.deepEqual(moved[0].moved, { from: 3, to: 1 });
  assert.ok(moved.every((block) => block.kind === 'same'));
  const inserted = compare([a, b], [paragraph('new', '新增段落'), a, b]).blocks;
  assert.equal(inserted.filter((block) => block.kind === 'add').length, 1);
  assert.equal(inserted.filter((block) => block.moved).length, 0);
  const deleted = compare([a, b, c], [a, c]).blocks;
  assert.equal(deleted.filter((block) => block.kind === 'delete')[0].before.attrs.id, 'b');
  assert.equal(deleted.filter((block) => block.moved).length, 0);
});

test('旧草稿重新生成块编号时仍对齐正文，不把整篇改动放大为整篇删除新增', () => {
  const result = compare(
    [
      paragraph('old-a', '固定开头'),
      paragraph('old-b', '需要修改原句'),
      paragraph('old-c', '固定结尾'),
    ],
    [
      paragraph('new-a', '固定开头'),
      paragraph('new-b', '需要修改新句'),
      paragraph('new-c', '固定结尾'),
    ],
  );
  assert.deepEqual(
    result.blocks.map((block) => block.kind),
    ['same', 'change', 'same'],
  );
  assert.equal(changed(result.blocks[1], 'delete'), '原');
  assert.equal(changed(result.blocks[1], 'add'), '新');
});

test('图片资源替换、图片尺寸、公式与表格单元格分别保留真实变化', () => {
  const image = (resourceId, width) => ({
    type: 'image',
    attrs: { resourceId, width, height: 50, alt: '正文图' },
  });
  const imageBlock = compare(
    [paragraph('p', [image('1', 100)])],
    [paragraph('p', [image('1', 200)])],
  ).blocks[0];
  assert.equal(imageBlock.kind, 'format');
  assert.ok(imageBlock.spans[0].format.some((value) => value.includes('图片宽度：100 → 200')));
  const replacement = compare(
    [paragraph('p', [image('1', 100)])],
    [paragraph('p', [image('2', 100)])],
  ).blocks[0];
  assert.deepEqual(
    replacement.spans.map((span) => span.kind),
    ['delete', 'add'],
  );
  assert.equal(replacement.spans[0].node.attrs.resourceId, '1');
  assert.equal(replacement.spans[1].node.attrs.resourceId, '2');
  const formula = (latex) => ({ type: 'formula_block', attrs: { id: 'formula', latex } });
  assert.equal(compare([formula('x')], [formula('x^2')]).blocks[0].kind, 'change');
  const table = (value, colspan = 1) => ({
    type: 'table',
    attrs: { id: 'table' },
    content: [
      {
        type: 'table_row',
        attrs: { id: 'row' },
        content: [
          {
            type: 'table_cell',
            attrs: { id: 'cell', colspan, rowspan: 1 },
            content: [paragraph('cell-p', value)],
          },
        ],
      },
    ],
  });
  const grid = compare([table('原值')], [table('新值', 2)]).blocks[0];
  assert.equal(grid.kind, 'change');
  assert.ok(grid.children[0].children[0].format.some((value) => value.includes('合并列数')));
  assert.equal(changed(grid.children[0].children[0].children[0], 'add'), '新');
});

test('固定基线必须同时匹配版本、文档、单元；其他当前MAIN不能代替本轮基线', () => {
  const round = {
    content_version_id: 'submitted',
    base_version_id: 'base',
    content: { id: 'submitted', document_id: 'd', unit_id: 'u' },
  };
  const base = { id: 'base', document_id: 'd', unit_id: 'u' };
  assert.equal(validateReviewBase(round, base, 'd'), base);
  for (const wrong of [
    { ...base, id: 'current-main' },
    { ...base, document_id: 'other' },
    { ...base, unit_id: 'other' },
  ]) {
    assert.throws(() => validateReviewBase(round, wrong, 'd'), /归属不一致/);
  }
  assert.throws(
    () => validateReviewBase({ ...round, content_version_id: 'other' }, base, 'd'),
    /归属不一致/,
  );
});

test('长文逐段对比有界，极大重写明确降级并完整保留两侧文字', () => {
  // 此例对应长文审核要求：许多正常段落应精确，超大单段不得分配无界LCS矩阵。
  const before = Array.from({ length: 400 }, (_, i) =>
    paragraph('p' + i, '第' + i + '段：原说明。'),
  );
  const after = before.map((node, i) =>
    i % 20 === 0 ? paragraph('p' + i, '第' + i + '段：新说明。') : node,
  );
  const result = compare(before, after);
  assert.equal(result.blocks.filter((block) => block.kind === 'change').length, 20);
  assert.equal(result.limited, false);
  const oldText = '旧'.repeat(30000),
    newText = '新'.repeat(30000);
  const huge = compare([paragraph('huge', oldText)], [paragraph('huge', newText)]);
  assert.equal(huge.limited, true);
  assert.equal(changed(huge.blocks[0], 'delete'), oldText);
  assert.equal(changed(huge.blocks[0], 'add'), newText);
  const unchanged = compare([paragraph('huge', oldText)], [paragraph('huge', oldText)]);
  assert.equal(unchanged.blocks[0].kind, 'same');
  assert.equal(unchanged.limited, false);
  const formatted = compare(
    [paragraph('huge', oldText)],
    [paragraph('huge', [text(oldText, [{ type: 'strong' }])])],
  );
  assert.equal(formatted.blocks[0].kind, 'format');
  assert.equal(formatted.limited, false);
});
