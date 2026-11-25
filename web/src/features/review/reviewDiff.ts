import type { FixedContent, ReviewRound } from '../../../../contracts/http/generated';

/** 这是只读比较模型，不生成可送审/合并的正文，也不复用批注锚点。 */
export type RichNode = {
  type: string;
  text?: string;
  attrs?: Record<string, unknown>;
  marks?: { type: string; attrs?: Record<string, unknown> }[];
  content?: RichNode[];
};
export type DiffKind = 'same' | 'add' | 'delete' | 'change' | 'format';
export type DiffSpan = {
  kind: Exclude<DiffKind, 'change'>;
  node: RichNode;
  before?: RichNode;
  format: string[];
};
export type DiffBlock = {
  key: string;
  kind: DiffKind;
  before?: RichNode;
  after?: RichNode;
  children?: DiffBlock[];
  spans?: DiffSpan[];
  format: string[];
  moved?: { from: number; to: number };
  limited?: boolean;
};
export type ReviewComparison = { blocks: DiffBlock[]; limited: boolean };
type Budget = { cells: number; limited: boolean };
type Token = { value: string; node: RichNode; format: string };

const textBlocks = new Set(['paragraph', 'heading', 'code_block']);
const atomTypes = new Set(['image', 'formula', 'formula_block', 'hard_break']);
const graphemes = new Intl.Segmenter('zh-CN', { granularity: 'grapheme' });
const defaults: Record<string, unknown> = {
  align: 'left',
  firstLineIndent: false,
  atomStrong: false,
  atomEm: false,
  atomCode: false,
  atomStrike: false,
  atomLink: null,
  language: null,
  alt: null,
  title: null,
  width: null,
  height: null,
  colwidth: null,
  order: 1,
};
const names: Record<string, string> = {
  align: '对齐',
  firstLineIndent: '首行缩进',
  level: '标题级别',
  language: '代码语言',
  width: '图片宽度',
  height: '图片高度',
  alt: '替代文字',
  title: '说明',
  atomStrong: '加粗',
  atomEm: '斜体',
  atomCode: '代码',
  atomStrike: '删除线',
  atomLink: '链接',
  colspan: '合并列数',
  rowspan: '合并行数',
  colwidth: '列宽',
  order: '列表起始序号',
};
const types: Record<string, string> = {
  paragraph: '段落',
  heading: '标题',
  code_block: '代码块',
  blockquote: '引用',
  bullet_list: '无序列表',
  ordered_list: '有序列表',
  table_cell: '单元格',
  table_header: '表头',
};

function stable(value: unknown): string {
  if (Array.isArray(value)) return '[' + value.map(stable).join(',') + ']';
  if (value && typeof value === 'object') {
    return (
      '{' +
      Object.entries(value)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([key, item]) => JSON.stringify(key) + ':' + stable(item))
        .join(',') +
      '}'
    );
  }
  return JSON.stringify(value) ?? '';
}
function attributes(node: RichNode) {
  // 身份是对齐线索，不属于用户可见格式；PM补出的默认值也不应产生假改动。
  return Object.fromEntries(
    Object.entries(node.attrs ?? {}).filter(
      ([key, value]) => key !== 'id' && key !== 'src' && stable(value) !== stable(defaults[key]),
    ),
  );
}
function markKey(node: RichNode) {
  return stable(
    (node.marks ?? [])
      .map((mark) => ({ ...mark, attrs: mark.attrs ?? {} }))
      .sort((a, b) => a.type.localeCompare(b.type)),
  );
}
function visible(value: unknown) {
  if (value === true) return '开启';
  if (value === false) return '关闭';
  if (value === null || value === undefined) return '默认';
  const translated: Record<string, string> = { left: '左对齐', center: '居中', right: '右对齐' };
  return typeof value === 'string' ? (translated[value] ?? value) : stable(value);
}
function markLabel(node: RichNode) {
  const labels: Record<string, string> = {
    strong: '加粗',
    em: '斜体',
    code: '行内代码',
    strike: '删除线',
    link: '链接',
    font: '字体',
  };
  return (
    (node.marks ?? [])
      .map(
        (mark) =>
          labels[mark.type] +
          (mark.attrs ? '（' + Object.values(mark.attrs).map(visible).join('，') + '）' : ''),
      )
      .join('、') || '普通文字'
  );
}
function formatChanges(before: RichNode, after: RichNode) {
  const result: string[] = [];
  if (before.type !== after.type)
    result.push((types[before.type] ?? before.type) + ' → ' + (types[after.type] ?? after.type));
  const old = attributes(before),
    next = attributes(after);
  for (const key of new Set([...Object.keys(old), ...Object.keys(next)])) {
    if (key === 'latex' || key === 'resourceId') continue;
    if (stable(old[key]) !== stable(next[key])) {
      result.push(
        (names[key] ?? key) +
          '：' +
          visible(old[key] ?? defaults[key]) +
          ' → ' +
          visible(next[key] ?? defaults[key]),
      );
    }
  }
  if (markKey(before) !== markKey(after))
    result.push('文字格式：' + markLabel(before) + ' → ' + markLabel(after));
  return result;
}
function contentKey(node: RichNode): string {
  if (node.type === 'text') return node.text ?? '';
  if (atomTypes.has(node.type))
    return stable([node.type, node.attrs?.resourceId, node.attrs?.latex]);
  return stable([
    textBlocks.has(node.type) ? 'textblock' : node.type,
    (node.content ?? []).map(contentKey),
  ]);
}
function id(node: RichNode) {
  return typeof node.attrs?.id === 'string' ? node.attrs.id : '';
}
function compatible(a: RichNode, b: RichNode) {
  return (
    a.type === b.type ||
    (textBlocks.has(a.type) && textBlocks.has(b.type)) ||
    (['bullet_list', 'ordered_list'].includes(a.type) &&
      ['bullet_list', 'ordered_list'].includes(b.type)) ||
    (['table_cell', 'table_header'].includes(a.type) &&
      ['table_cell', 'table_header'].includes(b.type))
  );
}

/** O(n log n)的顺序骨架。插入/删除导致的序号平移不当成移动，真正换序的块才标位置。 */
function increasing(values: { old: number; next: number }[]) {
  const tails: number[] = [],
    previous = new Int32Array(values.length).fill(-1);
  for (let index = 0; index < values.length; index++) {
    let low = 0,
      high = tails.length;
    while (low < high) {
      const middle = (low + high) >>> 1;
      if (values[tails[middle]].old < values[index].old) low = middle + 1;
      else high = middle;
    }
    if (low) previous[index] = tails[low - 1];
    tails[low] = index;
  }
  const result = new Set<number>();
  let cursor = tails.at(-1) ?? -1;
  while (cursor >= 0) {
    result.add(values[cursor].next);
    cursor = previous[cursor];
  }
  return result;
}

function matchBlocks(before: RichNode[], after: RichNode[]) {
  const matches = new Map<number, number>(),
    used = new Set<number>();
  function uniqueMatch(key: (node: RichNode) => string) {
    const old = new Map<string, number>(),
      next = new Map<string, number>();
    before.forEach((node, index) => {
      if (!used.has(index)) {
        const k = key(node);
        if (k) old.set(k, old.has(k) ? -1 : index);
      }
    });
    after.forEach((node, index) => {
      if (!matches.has(index)) {
        const k = key(node);
        if (k) next.set(k, next.has(k) ? -1 : index);
      }
    });
    next.forEach((index, key) => {
      const oldIndex = old.get(key);
      if (
        index >= 0 &&
        oldIndex !== undefined &&
        oldIndex >= 0 &&
        compatible(before[oldIndex], after[index])
      ) {
        matches.set(index, oldIndex);
        used.add(oldIndex);
      }
    });
  }
  uniqueMatch(id);
  // 老草稿可能重新生成全部块ID。可见内容相同仍能对齐，避免整篇误报删除再新增。
  uniqueMatch(contentKey);
  const skeleton = increasing(
    [...matches].sort(([a], [b]) => a - b).map(([next, old]) => ({ old, next })),
  );
  const anchors = [...skeleton]
    .sort((a, b) => a - b)
    .map((next) => ({ next, old: matches.get(next)! }));
  let oldStart = 0,
    nextStart = 0;
  for (const anchor of [...anchors, { old: before.length, next: after.length }]) {
    const oldGap: number[] = [],
      nextGap: number[] = [];
    for (let i = oldStart; i < anchor.old; i++) if (!used.has(i)) oldGap.push(i);
    for (let i = nextStart; i < anchor.next; i++) if (!matches.has(i)) nextGap.push(i);
    // 同一有界窗口内按原顺序配对；数量不等时不猜测哪个旧段落变成哪个新段落。
    if (oldGap.length === nextGap.length)
      for (let i = 0; i < oldGap.length; i++) {
        if (compatible(before[oldGap[i]], after[nextGap[i]])) {
          matches.set(nextGap[i], oldGap[i]);
          used.add(oldGap[i]);
        }
      }
    oldStart = anchor.old + 1;
    nextStart = anchor.next + 1;
  }
  const ordered = [...matches].sort(([a], [b]) => a - b).map(([next, old]) => ({ old, next }));
  return { matches, used, stationary: increasing(ordered) };
}

function tokenise(nodes: RichNode[]): Token[] {
  const result: Token[] = [];
  for (const node of nodes) {
    const format = stable([attributes(node), markKey(node)]);
    if (node.type === 'text') {
      // 按用户看见的字形切分，不能把组合音标、肤色或ZWJ表情拆成几个红绿片段。
      for (const part of graphemes.segment(node.text ?? ''))
        result.push({ value: part.segment, node, format });
    } else result.push({ value: contentKey(node), node, format });
  }
  return result;
}
function inlineDiff(before: RichNode[], after: RichNode[], budget: Budget) {
  const result: DiffSpan[] = [];
  function append(kind: DiffSpan['kind'], token: Token, old?: Token) {
    const format = kind === 'format' && old ? formatChanges(old.node, token.node) : [];
    const last = result.at(-1);
    if (
      token.node.type === 'text' &&
      last?.node.type === 'text' &&
      last.kind === kind &&
      markKey(last.node) === markKey(token.node) &&
      stable(last.format) === stable(format)
    ) {
      last.node.text += token.value;
    } else
      result.push({
        kind,
        node: token.node.type === 'text' ? { ...token.node, text: token.value } : token.node,
        before: old?.node,
        format,
      });
  }
  // 可见文字完全相同的长段落按文本run线性对齐；格式变化不能因字符预算误报整段增删。
  if (
    before.every((node) => node.type === 'text') &&
    after.every((node) => node.type === 'text') &&
    before.map((node) => node.text ?? '').join('') === after.map((node) => node.text ?? '').join('')
  ) {
    let i = 0,
      j = 0,
      oldOffset = 0,
      nextOffset = 0;
    while (i < before.length && j < after.length) {
      const oldNode = before[i],
        nextNode = after[j];
      const size = Math.min(
        (oldNode.text?.length ?? 0) - oldOffset,
        (nextNode.text?.length ?? 0) - nextOffset,
      );
      if (size) {
        const value = (nextNode.text ?? '').slice(nextOffset, nextOffset + size);
        const old = { value, node: oldNode, format: markKey(oldNode) },
          next = { value, node: nextNode, format: markKey(nextNode) };
        append(old.format === next.format ? 'same' : 'format', next, old);
      }
      oldOffset += size;
      nextOffset += size;
      if (oldOffset === (oldNode.text?.length ?? 0)) {
        i++;
        oldOffset = 0;
      }
      if (nextOffset === (nextNode.text?.length ?? 0)) {
        j++;
        nextOffset = 0;
      }
    }
    return { spans: result, limited: false };
  }
  const length = (nodes: RichNode[]) =>
    nodes.reduce((total, node) => total + (node.text?.length ?? 1), 0);
  // 超大单段不制造数百万字符对象；完整显示旧/新段，并明确说明无法逐字细分。
  if (length(before) + length(after) > 24000) {
    budget.limited = true;
    return {
      spans: [
        ...before.map((node) => ({ kind: 'delete' as const, node, format: [] })),
        ...after.map((node) => ({ kind: 'add' as const, node, format: [] })),
      ],
      limited: true,
    };
  }
  const old = tokenise(before),
    next = tokenise(after);
  let prefix = 0,
    suffix = 0;
  const equal = (a: Token, b: Token) => a.value === b.value && a.node.type === b.node.type;
  const same = (a: Token, b: Token) => append(a.format === b.format ? 'same' : 'format', b, a);
  while (prefix < old.length && prefix < next.length && equal(old[prefix], next[prefix])) {
    same(old[prefix], next[prefix]);
    prefix++;
  }
  while (
    suffix < old.length - prefix &&
    suffix < next.length - prefix &&
    equal(old[old.length - suffix - 1], next[next.length - suffix - 1])
  )
    suffix++;
  const a = old.slice(prefix, old.length - suffix),
    b = next.slice(prefix, next.length - suffix);
  const cells = (a.length + 1) * (b.length + 1);
  let limited = false;
  // 单次与整份文档双重预算，复杂度永远受限；普通长文仍逐段精确比较。
  if (a.length && b.length && cells <= 160000 && cells <= budget.cells) {
    budget.cells -= cells;
    const width = b.length + 1,
      matrix = new Uint16Array(cells);
    for (let i = a.length - 1; i >= 0; i--)
      for (let j = b.length - 1; j >= 0; j--) {
        matrix[i * width + j] = equal(a[i], b[j])
          ? matrix[(i + 1) * width + j + 1] + 1
          : Math.max(matrix[(i + 1) * width + j], matrix[i * width + j + 1]);
      }
    let i = 0,
      j = 0;
    while (i < a.length || j < b.length) {
      if (i < a.length && j < b.length && equal(a[i], b[j])) {
        same(a[i++], b[j++]);
      } else if (
        i < a.length &&
        (j === b.length || matrix[(i + 1) * width + j] >= matrix[i * width + j + 1])
      )
        append('delete', a[i++]);
      else append('add', b[j++]);
    }
  } else {
    limited = !!a.length && !!b.length;
    if (limited) budget.limited = true;
    for (const token of a) append('delete', token);
    for (const token of b) append('add', token);
  }
  for (let i = suffix; i > 0; i--) same(old[old.length - i], next[next.length - i]);
  return { spans: result, limited };
}

function compareBlock(
  before: RichNode | undefined,
  after: RichNode | undefined,
  key: string,
  budget: Budget,
): DiffBlock {
  if (!before || !after) return { key, kind: before ? 'delete' : 'add', before, after, format: [] };
  const format = formatChanges(before, after);
  const result: DiffBlock = { key, before, after, kind: 'same', format };
  if (textBlocks.has(before.type) && textBlocks.has(after.type)) {
    Object.assign(result, inlineDiff(before.content ?? [], after.content ?? [], budget));
    if (result.spans?.some((span) => span.kind === 'add' || span.kind === 'delete'))
      result.kind = 'change';
    else if (result.spans?.some((span) => span.kind === 'format')) result.kind = 'format';
  } else if (atomTypes.has(before.type)) {
    if (contentKey(before) !== contentKey(after)) result.kind = 'change';
  } else {
    result.children = compareBlocks(before.content ?? [], after.content ?? [], key, budget);
    if (
      result.children.some(
        (child) => ['add', 'delete', 'change'].includes(child.kind) || child.moved,
      )
    )
      result.kind = 'change';
    else if (result.children.some((child) => child.kind === 'format')) result.kind = 'format';
  }
  if (result.kind === 'same' && format.length) result.kind = 'format';
  return result;
}
function compareBlocks(
  before: RichNode[],
  after: RichNode[],
  path: string,
  budget: Budget,
): DiffBlock[] {
  const { matches, used, stationary } = matchBlocks(before, after);
  const result: DiffBlock[] = [];
  let oldCursor = 0;
  for (let next = 0; next < after.length; next++) {
    const old = matches.get(next);
    if (old !== undefined && stationary.has(next)) {
      for (; oldCursor < old; oldCursor++)
        if (!used.has(oldCursor))
          result.push(
            compareBlock(before[oldCursor], undefined, path + ':old:' + oldCursor, budget),
          );
      oldCursor = old + 1;
    }
    const block = compareBlock(
      old === undefined ? undefined : before[old],
      after[next],
      path + ':new:' + next,
      budget,
    );
    if (old !== undefined && !stationary.has(next)) block.moved = { from: old + 1, to: next + 1 };
    result.push(block);
  }
  for (; oldCursor < before.length; oldCursor++)
    if (!used.has(oldCursor))
      result.push(compareBlock(before[oldCursor], undefined, path + ':old:' + oldCursor, budget));
  return result;
}

export function compareReviewBodies(before: unknown, after: unknown): ReviewComparison {
  const base = before as RichNode,
    submitted = after as RichNode;
  if (
    base?.type !== 'doc' ||
    submitted?.type !== 'doc' ||
    !Array.isArray(base.content) ||
    !Array.isArray(submitted.content)
  )
    throw new Error('固定正文格式无法比较');
  const budget: Budget = { cells: 1200000, limited: false };
  return {
    blocks: compareBlocks(base.content, submitted.content, 'review', budget),
    limited: budget.limited,
  };
}

/** API返回后再核对绑定，不能把缓存中的另一单元、另一轮次或当前MAIN作为审核基线。 */
export function validateReviewBase(round: ReviewRound, base: FixedContent, document: string) {
  if (
    round.content.document_id !== document ||
    round.content.id !== round.content_version_id ||
    base.id !== round.base_version_id ||
    base.document_id !== document ||
    base.unit_id !== round.content.unit_id
  ) {
    throw new Error('审核基线与本轮固定正文归属不一致，请刷新审核记录');
  }
  return base;
}
