import MarkdownIt from 'markdown-it';
import { normalizeContent, atomFields } from './index.mjs';
const markdown = new MarkdownIt({ html: false, linkify: false, typographer: false, breaks: false });
// 数学只产生受控原子节点；LaTeX本身不是HTML，也不能让导入器执行宏或外部链接。
markdown.inline.ruler.before('escape', 'collab_math', (state, silent) => {
  const start = state.pos;
  if (state.src[start] !== '$' || state.src[start + 1] === '$') return false;
  let end = start + 1;
  while ((end = state.src.indexOf('$', end)) >= 0 && state.src[end - 1] === '\\') end++;
  if (end < 0 || end === start + 1 || state.src.slice(start + 1, end).includes('\n')) return false;
  if (!silent) {
    const token = state.push('collab_formula', '', 0);
    token.content = state.src.slice(start + 1, end);
  }
  state.pos = end + 1;
  return true;
});
markdown.block.ruler.before(
  'fence',
  'collab_math_block',
  (state, start, end, silent) => {
    const first = state.src
      .slice(state.bMarks[start] + state.tShift[start], state.eMarks[start])
      .trim();
    if (!first.startsWith('$$')) return false;
    let stop = start,
      latex;
    if (first.length > 4 && first.endsWith('$$')) latex = first.slice(2, -2);
    else {
      if (first !== '$$') return false;
      stop = start + 1;
      while (
        stop < end &&
        state.src.slice(state.bMarks[stop] + state.tShift[stop], state.eMarks[stop]).trim() !== '$$'
      )
        stop++;
      if (stop >= end) return false;
      latex = state.getLines(start + 1, stop, state.blkIndent, false).replace(/\n$/, '');
    }
    if (silent) return true;
    const token = state.push('collab_formula_block', '', 0);
    token.block = true;
    token.content = latex;
    token.map = [start, stop + 1];
    state.line = stop + 1;
    return true;
  },
  { alt: ['paragraph', 'reference', 'blockquote', 'list'] },
);

export function importMarkdown(
  text,
  { newId = () => crypto.randomUUID(), resources = new Map(), workspaceKind = 'BRANCH' } = {},
) {
  if (typeof text !== 'string' || new TextEncoder().encode(text).length > 2 * 1024 * 1024)
    throw Error('MARKDOWN_TOO_LARGE');
  const body = { type: 'doc', content: [] },
    stack = [body],
    losses = [],
    externalImages = [];
  let count = 0;
  const current = () => stack.at(-1),
    push = (n) => {
      if (++count > 100000) throw Error('MARKDOWN_TOO_LARGE');
      current().content ??= [];
      current().content.push(n);
    },
    open = (type, attrs = {}) => {
      const n = { type, attrs: { id: newId(), ...attrs }, content: [] };
      push(n);
      stack.push(n);
    };
  function inline(tokens) {
    const marks = [],
      content = [];
    for (const token of tokens ?? []) {
      if (token.type.endsWith('_open')) {
        const type = { strong_open: 'strong', em_open: 'em', s_open: 'strike', link_open: 'link' }[
          token.type
        ];
        if (type)
          marks.push(
            type === 'link'
              ? {
                  type,
                  attrs: { href: token.attrGet('href'), title: token.attrGet('title') ?? null },
                }
              : { type },
          );
        else losses.push('无法表示的内联格式已保留为文本');
        continue;
      }
      if (token.type.endsWith('_close')) {
        marks.pop();
        continue;
      }
      if (token.type === 'image') {
        const url = token.attrGet('src'),
          resource = resources.get(url);
        if (resource) {
          content.push({
            type: 'image',
            attrs: {
              resourceId: resource,
              alt: token.content || null,
              title: token.attrGet('title') ?? null,
              ...atomFields(marks),
            },
          });
        } else {
          externalImages.push({ url, alt: token.content });
          losses.push('外部图片尚未转存为同空间READY资源');
          content.push({
            type: 'text',
            text: '[图片待转存：' + (token.content || url) + ']',
            ...(marks.length ? { marks: structuredClone(marks) } : {}),
          });
        }
        continue;
      }
      if (token.type === 'collab_formula') {
        content.push({ type: 'formula', attrs: { latex: token.content, ...atomFields(marks) } });
        continue;
      }
      if (token.type === 'hardbreak') {
        content.push({ type: 'hard_break' });
        continue;
      }
      const value = token.type === 'softbreak' ? '\n' : token.content;
      if (!value) continue;
      const ownMarks = token.type === 'code_inline' ? [...marks, { type: 'code' }] : marks;
      content.push({
        type: 'text',
        text: value,
        ...(ownMarks.length ? { marks: structuredClone(ownMarks) } : {}),
      });
    }
    return content;
  }
  for (const token of markdown.parse(text, {})) {
    const block = {
      paragraph_open: 'paragraph',
      heading_open: 'heading',
      blockquote_open: 'blockquote',
      bullet_list_open: 'bullet_list',
      ordered_list_open: 'ordered_list',
      list_item_open: 'list_item',
      table_open: 'table',
      tr_open: 'table_row',
    }[token.type];
    if (block) {
      const attrs =
        block === 'heading'
          ? { level: Number(token.tag.slice(1)) }
          : block === 'ordered_list'
            ? { order: Number(token.attrGet('start') ?? 1) }
            : {};
      open(block, attrs);
      continue;
    }
    if (['thead_open', 'thead_close', 'tbody_open', 'tbody_close'].includes(token.type)) continue;
    if (token.type === 'th_open' || token.type === 'td_open') {
      if (token.attrGet('style')) losses.push('Markdown表格对齐已规范化');
      open(token.type === 'th_open' ? 'table_header' : 'table_cell', {
        colspan: 1,
        rowspan: 1,
        colwidth: null,
      });
      open('paragraph');
      continue;
    }
    if (token.type === 'th_close' || token.type === 'td_close') {
      stack.pop();
      stack.pop();
      continue;
    }
    if (token.nesting === -1) {
      stack.pop();
      continue;
    }
    if (token.type === 'inline') {
      current().content.push(...inline(token.children));
      continue;
    }
    if (token.type === 'fence' || token.type === 'code_block') {
      push({
        type: 'code_block',
        attrs: { id: newId(), language: token.info.trim().split(/\s+/)[0] || null },
        content: token.content ? [{ type: 'text', text: token.content }] : [],
      });
      continue;
    }
    if (token.type === 'hr') {
      push({ type: 'horizontal_rule', attrs: { id: newId() } });
      continue;
    }
    if (token.type === 'collab_formula_block') {
      push({
        type: 'formula_block',
        attrs: { id: newId(), latex: token.content, ...atomFields() },
      });
      continue;
    }
    if (token.content) {
      losses.push('无法表示的块格式已保留为普通文本');
      push({
        type: 'paragraph',
        attrs: { id: newId() },
        content: [{ type: 'text', text: token.content }],
      });
    }
  }
  if (!body.content.length)
    body.content.push({ type: 'paragraph', attrs: { id: newId() }, content: [] });
  return {
    body: normalizeContent(body, { workspaceKind }),
    losses: [...new Set(losses)],
    externalImages,
  };
}
