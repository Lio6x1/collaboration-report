import type { EditorView } from 'prosemirror-view';
import type { EditorState } from 'prosemirror-state';
import { NodeSelection, Plugin, TextSelection } from 'prosemirror-state';
import { Slice } from 'prosemirror-model';
import { toggleMark } from 'prosemirror-commands';
import {
  addRowAfter,
  addColumnAfter,
  deleteRow,
  deleteColumn,
  mergeCells,
  splitCell,
} from 'prosemirror-tables';
import { schema, fromPM, toPM, canonical } from '../../../../packages/content/index.mjs';
import { importMarkdown } from '../../../../packages/content/markdown.mjs';
import { UserFacingError } from '../../lib/http';

export type FormatName = 'strong' | 'em' | 'strike' | 'code';
export type ParagraphAlignment = 'left' | 'center' | 'right';
export const tableActions = {
  addRowAfter,
  addColumnAfter,
  deleteRow,
  deleteColumn,
  mergeCells,
  splitCell,
};
export type TableAction = keyof typeof tableActions;

export type ToolbarState = {
  marks: Record<FormatName, boolean | 'mixed'>;
  block: 'paragraph' | 'heading' | 'code_block' | 'mixed' | null;
  alignment: ParagraphAlignment | 'mixed' | null;
  indent: boolean | 'mixed' | null;
};

// 普通编辑器和人工修复候选共用观察方式；不自行分派事务，不影响Yjs保存。
export function toolbarSelectionPlugin(changed: (state: EditorState) => void) {
  return new Plugin({
    view(editor) {
      changed(editor.state);
      return {
        update(current, previous) {
          if (
            current.state.doc !== previous.doc ||
            !current.state.selection.eq(previous.selection) ||
            current.state.storedMarks !== previous.storedMarks
          )
            changed(current.state);
        },
      };
    },
  });
}

// 从PM选区派生，不能在点击按钮时记一个本地布尔值：光标移动、撤销和远端协作
// 都会改变真实格式。多段格式不一致时返回mixed，避免同时高亮互斥选项。
export function toolbarState(state?: EditorState): ToolbarState {
  const result: ToolbarState = {
    marks: { strong: false, em: false, strike: false, code: false },
    block: null,
    alignment: null,
    indent: null,
  };
  if (!state) return result;
  const blocks = new Set<'paragraph' | 'heading' | 'code_block'>(),
    alignments = new Set<ParagraphAlignment>(),
    indents = new Set<boolean>(),
    marks = {
      strong: new Set<boolean>(),
      em: new Set<boolean>(),
      strike: new Set<boolean>(),
      code: new Set<boolean>(),
    };
  const { from, to, empty, $from } = state.selection;
  const names = Object.keys(result.marks) as FormatName[];
  if (empty) {
    const cursorMarks = state.storedMarks ?? $from.marks();
    for (const name of names)
      marks[name].add(Boolean(state.schema.marks[name].isInSet(cursorMarks)));
  }
  state.doc.nodesBetween(from, to, (node, pos) => {
    if (['paragraph', 'heading', 'code_block'].includes(node.type.name)) {
      blocks.add(node.type.name as 'paragraph' | 'heading' | 'code_block');
    }
    if (node.type.name === 'paragraph' || node.type.name === 'heading') {
      alignments.add(node.attrs.align ?? 'left');
      indents.add(Boolean(node.attrs.firstLineIndent));
    }
    if (empty || pos >= to || pos + node.nodeSize <= from) return;
    if (node.isText) {
      for (const name of names)
        marks[name].add(Boolean(state.schema.marks[name].isInSet(node.marks)));
    } else if (['formula', 'formula_block', 'image'].includes(node.type.name)) {
      const attrs = fromPM(node.toJSON()).attrs ?? {};
      const fields = { strong: 'atomStrong', em: 'atomEm', strike: 'atomStrike', code: 'atomCode' };
      for (const name of names) marks[name].add(Boolean(attrs[fields[name]]));
    }
  });
  const uniform = <T>(values: Set<T>): T | 'mixed' | null =>
    values.size > 1 ? 'mixed' : values.size === 1 ? [...values][0] : null;
  result.block = uniform(blocks);
  result.alignment = uniform(alignments);
  result.indent = uniform(indents);
  for (const name of names) result.marks[name] = uniform(marks[name]) ?? false;
  return result;
}

export function format(view: EditorView | undefined, name: FormatName) {
  // editable只限制DOM输入；工具栏主动事务也必须遵守加载、旧工作区与候选禁写边界。
  if (!view?.editable) return;
  const { state } = view;
  // 原子格式字段独立更新，不能用普通mark包裹公式再指望Yjs自动保真。
  const field = { strong: 'atomStrong', em: 'atomEm', strike: 'atomStrike', code: 'atomCode' }[
    name
  ];
  if (
    state.selection instanceof NodeSelection &&
    ['formula', 'formula_block', 'image'].includes(state.selection.node.type.name)
  ) {
    const node = state.selection.node,
      attrs = { ...node.attrs, ...fromPM(node.toJSON()).attrs },
      pos = state.selection.from,
      tr = state.tr.setNodeMarkup(pos, undefined, { ...attrs, [field]: !attrs[field] }, []);
    // setNodeMarkup的替换映射会丢NodeSelection；保留原子选中才能连续修改不同独立字段。
    view.dispatch(tr.setSelection(NodeSelection.create(tr.doc, pos)));
  } else if (state.selection.empty)
    toggleMark(schema.marks[name])(state, (tr) => view!.dispatch(tr));
  else {
    const atoms: Array<{ pos: number; attrs: Record<string, any> }> = [],
      { from, to } = state.selection;
    state.doc.nodesBetween(from, to, (node, pos) => {
      if (['formula', 'formula_block', 'image'].includes(node.type.name))
        atoms.push({ pos, attrs: { ...node.attrs, ...fromPM(node.toJSON()).attrs } });
    });
    // 范围选择同时含普通文字与原子：普通文字用mark，原子只改独立属性，避免两套格式相互覆盖。
    const enable = !(
        state.doc.rangeHasMark(from, to, schema.marks[name]) ||
        atoms.some((atom) => atom.attrs[field])
      ),
      tr = state.tr;
    if (enable) tr.addMark(from, to, schema.marks[name].create());
    else tr.removeMark(from, to, schema.marks[name]);
    for (const atom of atoms)
      tr.setNodeMarkup(atom.pos, undefined, { ...atom.attrs, [field]: enable }, []);
    view.dispatch(tr);
  }
  view.focus();
}

export function insertFormula(view: EditorView | undefined, latex: string) {
  if (!view?.editable || !latex.trim()) return;
  view.dispatch(view.state.tr.replaceSelectionWith(schema.nodes.formula.create({ latex: latex })));
  view.focus();
}

export function insertImage(view: EditorView, resource: string) {
  if (!view.editable) return;
  // 普通编辑事务继续经过Yjs和原可靠保存队列；不把上传成功误当作正文已保存。
  view.dispatch(
    view.state.tr
      .replaceSelectionWith(schema.nodes.image.create({ resourceId: resource, alt: '正文图片' }))
      .scrollIntoView(),
  );
  view.focus();
}

export function heading(view: EditorView | undefined) {
  setParagraphType(view, 'heading');
}

export function paragraph(view: EditorView | undefined) {
  setParagraphType(view, 'paragraph');
}

// 代码块仍是content-v1中的普通文本块；转换只改内容类型，不重建稳定身份或绕过可靠保存。
export function codeBlock(view: EditorView | undefined) {
  if (!view?.editable) return;
  const { state } = view;
  const blocks: Array<{ pos: number; node: import('prosemirror-model').Node }> = [];
  state.doc.nodesBetween(state.selection.from, state.selection.to, (node, pos) => {
    if (['paragraph', 'heading', 'code_block'].includes(node.type.name)) blocks.push({ pos, node });
  });
  if (blocks.length && blocks.every(({ node }) => node.type.name === 'code_block')) {
    view.focus();
    return;
  }
  const onlyText =
    blocks.length > 0 &&
    blocks.every(({ node, pos }) => {
      let valid = true;
      node.forEach((child) => {
        if (!child.isText && child.type.name !== 'hard_break') valid = false;
      });
      const at = state.doc.resolve(pos);
      return valid && at.parent.canReplaceWith(at.index(), at.index() + 1, schema.nodes.code_block);
    });
  const tr = state.tr;
  let cursor: number;
  if (onlyText) {
    for (const { pos, node } of [...blocks].reverse()) {
      const text = node.textBetween(0, node.content.size, '\n', '\n');
      tr.replaceWith(
        pos,
        pos + node.nodeSize,
        schema.nodes.code_block.create(
          { id: node.attrs.id, language: node.attrs.language ?? null },
          text ? schema.text(text) : undefined,
        ),
      );
    }
    cursor =
      blocks[0].pos +
      1 +
      Math.min(state.selection.$from.parentOffset, tr.doc.nodeAt(blocks[0].pos)!.content.size);
  } else {
    // 图片、公式和表格不能因“转换代码”被clearIncompatible静默删除；在当前块后另建空代码块。
    const end = state.selection.$to;
    let depth = end.depth;
    while (
      depth > 0 &&
      !end
        .node(depth - 1)
        .canReplaceWith(
          end.indexAfter(depth - 1),
          end.indexAfter(depth - 1),
          schema.nodes.code_block,
        )
    )
      depth--;
    const at = depth > 0 ? end.after(depth) : end.pos;
    tr.insert(at, schema.nodes.code_block.create({ id: crypto.randomUUID(), language: null }));
    cursor = at + 1;
  }
  tr.setSelection(TextSelection.create(tr.doc, cursor));
  view.dispatch(tr.scrollIntoView());
  view.focus();
}

function setParagraphType(view: EditorView | undefined, type: 'paragraph' | 'heading') {
  if (!view?.editable) return;
  const { state } = view,
    tr = state.tr,
    blocks: Array<{ pos: number; size: number; attrs: Record<string, unknown> }> = [];
  // 格式动作保留每个已有块的身份；多块选择也不能给所有标题复制同一个新ID。
  state.doc.nodesBetween(state.selection.from, state.selection.to, (node, pos) => {
    if (['paragraph', 'heading', 'code_block'].includes(node.type.name))
      blocks.push({ pos, size: node.nodeSize, attrs: node.attrs });
  });
  for (const block of blocks.reverse())
    tr.setBlockType(block.pos, block.pos + block.size, schema.nodes[type], {
      ...(type === 'heading' ? { level: 2 } : {}),
      id: block.attrs.id,
      align: block.attrs.align,
      firstLineIndent: block.attrs.firstLineIndent,
    });
  if (tr.docChanged) view.dispatch(tr);
  view.focus();
}

export function paragraphAlignment(view: EditorView | undefined, align: ParagraphAlignment) {
  setParagraphStyle(view, { align });
}

export function firstLineIndent(view: EditorView | undefined, enabled: boolean) {
  setParagraphStyle(view, { firstLineIndent: enabled });
}

function setParagraphStyle(view: EditorView | undefined, attrs: Record<string, unknown>) {
  if (!view?.editable) return;
  const { state } = view,
    tr = state.tr;
  // 一次事务覆盖所选段落，保留块ID及其他格式；公式、表格结构不能被当作段落改写。
  state.doc.nodesBetween(state.selection.from, state.selection.to, (node, pos) => {
    if (['paragraph', 'heading'].includes(node.type.name))
      tr.setNodeMarkup(pos, undefined, { ...node.attrs, ...attrs });
  });
  if (tr.docChanged) view.dispatch(tr);
  view.focus();
}

export function table(view: EditorView | undefined) {
  if (!view?.editable) return;
  const paragraph = () => schema.nodes.paragraph.create({ id: crypto.randomUUID() }),
    cell = () =>
      schema.nodes.table_cell.create(
        { id: crypto.randomUUID(), colspan: 1, rowspan: 1, colwidth: null },
        paragraph(),
      ),
    row = () => schema.nodes.table_row.create({ id: crypto.randomUUID() }, [cell(), cell()]);
  view.dispatch(
    view.state.tr.replaceSelectionWith(
      schema.nodes.table.create({ id: crypto.randomUUID() }, [row(), row()]),
    ),
  );
  view.focus();
}

export function tableAction(view: EditorView | undefined, command: typeof addRowAfter) {
  if (view?.editable) {
    command(view.state, (tr) => view!.dispatch(tr));
    view.focus();
  }
}

export function insertMarkdown(
  view: EditorView,
  markdown: string,
  reportLoss: (value: string) => void,
) {
  if (!view.editable) return;
  const imported = importMarkdown(markdown);
  // 单次规范正文有界，避免生成永远无法通过WSS帧预算的大增量。
  if (new TextEncoder().encode(canonical(imported.body)).length > 256 * 1024)
    throw new UserFacingError('单次导入内容过大，请分段导入（每次不超过256KiB）');
  reportLoss(imported.losses.join('；'));
  view.dispatch(
    view.state.tr
      .replaceSelection(new Slice(schema.nodeFromJSON(toPM(imported.body)).content, 0, 0))
      .scrollIntoView(),
  );
  view.focus();
}
