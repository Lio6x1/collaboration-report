import test from 'node:test';
import assert from 'node:assert/strict';
import { performance } from 'node:perf_hooks';
import { normalizeContent } from '../../../packages/content/index.mjs';
import { mergeContent } from '../src/domain/content_merge.mjs';

const text = (value, marks) => ({ type: 'text', text: value, ...(marks ? { marks } : {}) });
const p = (id, value, marks) => ({
  type: 'paragraph',
  attrs: { id },
  content: value ? [text(value, marks)] : [],
});
const doc = (...content) => ({ type: 'doc', content });
const normalized = (value) => normalizeContent(value, { workspaceKind: 'MAIN' });
const merge = (base, submitted, main) => mergeContent({ base, submitted, main });
function expectBody(result, expected) {
  assert.deepEqual(result.conflicts, []);
  assert.deepEqual(result.body, normalized(expected));
}

// 三方输入只能是固定业务JSON；这些预期独立写出，不从被测引擎反推。
test('independent stable blocks keep both fixed changes without modifying inputs', () => {
  const base = doc(p('a', '原甲'), p('b', '原乙'));
  const submitted = doc(p('a', '原甲'), p('b', '送审乙'));
  const main = doc(p('a', '主线甲'), p('b', '原乙'));
  const before = JSON.stringify({ base, submitted, main });
  expectBody(merge(base, submitted, main), doc(p('a', '主线甲'), p('b', '送审乙')));
  assert.equal(JSON.stringify({ base, submitted, main }), before);
});
test('separate grapheme insertions inside one paragraph both survive', () => {
  expectBody(
    merge(doc(p('p', '甲乙丙丁')), doc(p('p', '甲新乙丙丁')), doc(p('p', '甲乙丙新丁'))),
    doc(p('p', '甲新乙丙新丁')),
  );
  expectBody(
    merge(doc(p('p', '开👩‍💻始')), doc(p('p', '新开👩‍💻始')), doc(p('p', '开👩‍💻始好'))),
    doc(p('p', '新开👩‍💻始好')),
  );
});
test('same insertion deduplicates but different insertions at the same position conflict', () => {
  const base = doc(p('p', '甲乙'));
  expectBody(merge(base, doc(p('p', '甲新乙')), doc(p('p', '甲新乙'))), doc(p('p', '甲新乙')));
  const result = merge(base, doc(p('p', '甲新乙')), doc(p('p', '甲旧乙')));
  assert.equal(result.body, null);
  assert.equal(result.conflicts[0].kind, 'TEXT_OVERLAP');
  assert.equal(result.conflicts[0].block_id, 'p');
});
test('independent text mark types combine while conflicting link targets do not', () => {
  expectBody(
    merge(
      doc(p('p', '字')),
      doc(p('p', '字', [{ type: 'em' }])),
      doc(p('p', '字', [{ type: 'strong' }])),
    ),
    doc(p('p', '字', [{ type: 'em' }, { type: 'strong' }])),
  );
  const link = (href) => [{ type: 'link', attrs: { href, title: null } }];
  const result = merge(
    doc(p('p', '字')),
    doc(p('p', '字', link('https://one.example'))),
    doc(p('p', '字', link('https://two.example'))),
  );
  assert.equal(result.body, null);
  assert.ok(result.conflicts.some((c) => c.kind === 'ATTRIBUTE_CONFLICT'));
});
test('deleting text cannot silently discard the other side formatting', () => {
  const result = merge(
    doc(p('p', '甲乙')),
    doc(p('p', '甲')),
    doc(p('p', '甲乙', [{ type: 'strong' }])),
  );
  assert.equal(result.body, null);
  assert.ok(result.conflicts.some((c) => c.kind === 'DELETE_FORMAT'));
});

test('independent hyperlink attributes and edits around a hard break retain both changes', () => {
  const link = (href, title) => [{ type: 'link', attrs: { href, title } }];
  expectBody(
    merge(
      doc(p('p', '链接', link('https://old.example', null))),
      doc(p('p', '链接', link('https://new.example', null))),
      doc(p('p', '链接', link('https://old.example', '主线说明'))),
    ),
    doc(p('p', '链接', link('https://new.example', '主线说明'))),
  );
  const lines = (first, second) =>
    doc({
      type: 'paragraph',
      attrs: { id: 'p' },
      content: [text(first), { type: 'hard_break' }, text(second)],
    });
  expectBody(
    merge(lines('甲', '乙'), lines('新甲', '乙'), lines('甲', '乙新')),
    lines('新甲', '乙新'),
  );
});
test('inserted branch text inherits main font while keeping submitted emphasis', () => {
  const font = { type: 'font', attrs: { family: 'SimSun' } };
  const submitted = doc({
    type: 'paragraph',
    attrs: { id: 'p' },
    content: [text('甲'), text('新', [{ type: 'em' }]), text('乙')],
  });
  expectBody(
    merge(doc(p('p', '甲乙')), submitted, doc(p('p', '甲乙', [font]))),
    doc({
      type: 'paragraph',
      attrs: { id: 'p' },
      content: [text('甲', [font]), text('新', [{ type: 'em' }, font]), text('乙', [font])],
    }),
  );
});
test('formula independent attributes merge without ordinary marks or lost latex', () => {
  const formula = (attrs) =>
    doc({
      type: 'paragraph',
      attrs: { id: 'p' },
      content: [{ type: 'formula', attrs: { latex: 'x', ...attrs } }],
    });
  expectBody(
    merge(formula({}), formula({ atomEm: true }), formula({ atomStrong: true })),
    formula({ atomEm: true, atomStrong: true }),
  );
  expectBody(
    merge(formula({}), formula({ latex: 'x^2' }), formula({ atomStrong: true })),
    formula({ latex: 'x^2', atomStrong: true }),
  );
});
const row = (id, value) => ({
  type: 'table_row',
  attrs: { id },
  content: [
    {
      type: 'table_cell',
      attrs: { id: id + '-cell', colspan: 1, rowspan: 1 },
      content: [p(id + '-p', value)],
    },
  ],
});
const table = (...rows) => doc({ type: 'table', attrs: { id: 'table' }, content: rows });
test('table edits in different rows preserve cell and row identity', () => {
  expectBody(
    merge(
      table(row('r1', '甲'), row('r2', '乙')),
      table(row('r1', '甲'), row('r2', '送审')),
      table(row('r1', '主线'), row('r2', '乙')),
    ),
    table(row('r1', '主线'), row('r2', '送审')),
  );
});
test('deleted table row is not resurrected and a concurrent edit is an explicit conflict', () => {
  const base = table(row('r1', '甲'), row('r2', '乙'));
  expectBody(
    merge(base, table(row('r2', '送审')), table(row('r1', '甲'), row('r2', '乙'))),
    table(row('r2', '送审')),
  );
  const result = merge(base, table(row('r2', '乙')), table(row('r1', '后来修改'), row('r2', '乙')));
  assert.equal(result.body, null);
  assert.ok(result.conflicts.some((c) => c.kind === 'DELETE_EDIT'));
});
const heading = (id, value) => ({
  type: 'heading',
  attrs: { id, level: 2 },
  content: [text(value)],
});
test('deleted heading subtree conflicts with later descendant content', () => {
  const base = doc(heading('h1', '甲'), p('p1', '旧'), heading('h2', '乙'), p('p2', '保留'));
  const result = merge(
    base,
    doc(heading('h2', '乙'), p('p2', '保留')),
    doc(heading('h1', '甲'), p('p1', '后来修改'), heading('h2', '乙'), p('p2', '保留')),
  );
  assert.equal(result.body, null);
  assert.ok(result.conflicts.some((c) => c.kind === 'DELETE_EDIT'));
});
test('different moves and independent insertions into the same structural slot require resolution', () => {
  const base = doc(p('a', '甲'), p('b', '乙'), p('c', '丙'));
  let result = merge(
    base,
    doc(p('b', '乙'), p('a', '甲'), p('c', '丙')),
    doc(p('a', '甲'), p('c', '丙'), p('b', '乙')),
  );
  assert.equal(result.body, null);
  assert.ok(result.conflicts.some((c) => c.kind === 'ORDER_CONFLICT'));
  result = merge(
    doc(p('a', '甲')),
    doc(p('a', '甲'), p('b', '乙')),
    doc(p('a', '甲'), p('c', '丙')),
  );
  assert.equal(result.body, null);
  assert.ok(result.conflicts.some((c) => c.kind === 'INSERT_ORDER'));
});
test('invalid table geometry and duplicate IDs reject fixed inputs', () => {
  assert.throws(
    () => merge(doc(p('a', '甲')), doc(p('a', '甲'), p('a', '乙')), doc(p('a', '甲'))),
    { code: 'INVALID_CONTENT' },
  );
  assert.throws(
    () =>
      merge(
        table(row('r1', '甲')),
        table(row('r1', '甲'), { ...row('r2', '乙'), content: [] }),
        table(row('r1', '甲')),
      ),
    { code: 'INVALID_CONTENT' },
  );
});
test('large unchanged text uses the safe fast path and complex dual edits stop explicitly', () => {
  const value = '甲'.repeat(70000);
  const base = doc(p('p', value));
  expectBody(merge(base, base, doc(p('p', value + '主线'))), doc(p('p', value + '主线')));
  const started = performance.now();
  const result = merge(base, doc(p('p', '送审' + value)), doc(p('p', value + '主线')));
  assert.equal(result.body, null);
  assert.ok(result.conflicts.some((c) => c.kind === 'MERGE_COMPLEXITY_LIMIT'));
  assert.ok(
    performance.now() - started < 2000,
    '超预算正文应在分词和diff之前拒绝，不能长时间阻塞事件循环',
  );
});
