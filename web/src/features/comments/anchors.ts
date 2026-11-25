import type { Node as ProseMirrorNode } from 'prosemirror-model';
import { TextSelection, type Selection } from 'prosemirror-state';
import type { Anchor } from '../../../../contracts/http/generated';

type FlatBlock = { text: string; starts: number[]; ends: number[] };

// 后端将图片/行内公式视为一个UTF-16占位符；PM容器边界不属于正文字符。
// 分别保留起止边界，才能处理列表/表格等容器中的段落间隙，绝不从DOM猜偏移。
function flattenBlock(node: ProseMirrorNode, position: number): FlatBlock {
  const flat: FlatBlock = { text: '', starts: [], ends: [] };
  const append = (text: string, from: number) => {
    const offset = flat.text.length;
    for (let i = 0; i < text.length; i++) {
      flat.starts[offset + i] = from + i;
      flat.ends[offset + i + 1] = from + i + 1;
    }
    flat.text += text;
  };
  const visit = (child: ProseMirrorNode, from: number) => {
    if (child.isText) append(child.text ?? '', from);
    else if (child.type.name === 'image' || child.type.name === 'formula') append('\ufffc', from);
    else if (child.type.name === 'hard_break') append('\n', from);
    else child.forEach((nested, offset) => visit(nested, from + 1 + offset));
  };
  visit(node, position);
  flat.starts[flat.text.length] ??= flat.ends[flat.text.length] ?? position + 1;
  flat.ends[0] = flat.starts[0] ?? position + 1;
  return flat;
}

function validBoundary(text: string, offset: number): boolean {
  if (!Number.isSafeInteger(offset) || offset < 0 || offset > text.length) return false;
  const before = text.charCodeAt(offset - 1);
  const after = text.charCodeAt(offset);
  return !(before >= 0xd800 && before <= 0xdbff && after >= 0xdc00 && after <= 0xdfff);
}

export function selectionAnchor(
  selection: Selection,
  version: string,
): {
  anchor: Anchor | null;
  hint: string;
} {
  if (selection.empty) return { anchor: null, hint: '' };
  if (!(selection instanceof TextSelection) || !selection.$from.sameParent(selection.$to)) {
    return { anchor: null, hint: '请在同一个段落、标题或代码块内选择批注内容。' };
  }
  const block = selection.$from.parent;
  if (!block.isTextblock || typeof block.attrs.id !== 'string' || !block.attrs.id) {
    return { anchor: null, hint: '此选区没有稳定的正文块标识，无法创建选区批注。' };
  }
  const flat = flattenBlock(block, selection.$from.before());
  const start = flat.starts.indexOf(selection.from);
  const end = flat.ends.indexOf(selection.to);
  if (!validBoundary(flat.text, start) || !validBoundary(flat.text, end) || end <= start) {
    return { anchor: null, hint: '选区边界无效，请重新选择完整字符。' };
  }
  const quote = flat.text.slice(start, end);
  const tooLong = Array.from(quote).length > 500;
  return {
    anchor: {
      schema_version: 'anchor-v1',
      content_version_id: version,
      block_id: block.attrs.id,
      start_offset: start,
      end_offset: end,
      ...(tooLong ? {} : { quote }),
    },
    hint: tooLong ? '已选中正文；选区超过500字，批注保留完整位置但不附带引用文本。' : '',
  };
}

export type AnchorLocation =
  | { kind: 'none' }
  | { kind: 'orphan'; hint: string }
  | { kind: 'range' | 'block'; from: number; to: number };

// 只为阅读视图建立位置投影，绝不改写持久锚点。历史引用必须在同一稳定块、原偏移精确匹配；
// 缺少引用或内容已变时仍展示历史原文入口，不能靠模糊搜词把评论挂到另一处。
export function projectAnchor(
  doc: ProseMirrorNode,
  version: string,
  anchor: Anchor,
): Anchor | null {
  if (
    anchor.content_version_id !== version &&
    (anchor.quote === undefined ||
      anchor.start_offset === undefined ||
      anchor.end_offset === undefined)
  )
    return null;
  const projected = { ...anchor, content_version_id: version };
  const location = locateAnchor(doc, version, projected);
  return location.kind === 'block' || location.kind === 'range' ? projected : null;
}

export function locateAnchor(
  doc: ProseMirrorNode,
  version: string,
  anchor?: Anchor | null,
): AnchorLocation {
  if (!anchor) return { kind: 'none' };
  const orphan: AnchorLocation = {
    kind: 'orphan',
    hint: '批注原始位置在当前固定版本中无法定位；未将批注迁移到其他正文。',
  };
  if (anchor.schema_version !== 'anchor-v1' || anchor.content_version_id !== version) return orphan;
  if (anchor.block_id === undefined) {
    return anchor.start_offset === undefined &&
      anchor.end_offset === undefined &&
      anchor.quote === undefined
      ? { kind: 'none' }
      : orphan;
  }
  let found: { node: ProseMirrorNode; position: number } | undefined;
  let matches = 0;
  doc.descendants((node, position) => {
    if (node.attrs.id === anchor.block_id) {
      found = { node, position };
      matches++;
    }
  });
  if (!found || matches !== 1) return orphan;
  const { node, position } = found;
  const start = anchor.start_offset;
  const end = anchor.end_offset;
  if (start === undefined && end === undefined) {
    return anchor.quote === undefined
      ? { kind: 'block', from: position, to: position + node.nodeSize }
      : orphan;
  }
  if (start === undefined || end === undefined) return orphan;
  const flat = flattenBlock(node, position);
  if (!validBoundary(flat.text, start) || !validBoundary(flat.text, end) || end < start)
    return orphan;
  // 引用只用于校验既有位置，不参与模糊搜索，避免把旧批注贴到新文本上。
  if (
    anchor.quote !== undefined &&
    (Array.from(anchor.quote).length > 500 || anchor.quote !== flat.text.slice(start, end))
  )
    return orphan;
  return {
    kind: 'range',
    from: flat.starts[start]!,
    to: start === end ? flat.starts[start]! : flat.ends[end]!,
  };
}
