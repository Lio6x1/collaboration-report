import { createHash } from 'node:crypto';
import { canonical, normalizeContent } from '../../../../packages/content/index.mjs';
import { draftBody } from './draft_protocol.mjs';

const failure = (code) => Object.assign(new Error(code), { code });

function headingTitle(heading) {
  const text = (node) =>
    node.text ??
    (node.type === 'formula'
      ? node.attrs.latex
      : node.type === 'image'
        ? (node.attrs.alt ?? '')
        : (node.content ?? []).map(text).join(''));
  const title = text(heading).trim();
  if (!title || [...title].length > 240) throw failure('STRUCTURE_TITLE_INVALID');
  return title;
}

function blockIds(nodes) {
  const result = [];
  const visit = (node) => {
    if (node.attrs?.id) result.push(node.attrs.id);
    for (const child of node.content ?? []) visit(child);
  };
  for (const node of nodes) visit(node);
  return result;
}

function emptyParagraph(id) {
  return { type: 'paragraph', attrs: { id }, content: [] };
}

// 只按顶层H2分界；表格、列表、H3及以下子树原样迁移。管理标题的完整原节点随映射保存供合章还原。
export function splitChapterBody(input, { workspaceKind = 'BRANCH' } = {}) {
  const body = normalizeContent(input, { workspaceKind });
  const pieces = [];
  let current = { kind: 'INTRODUCTION', title: '', nodes: [] };
  for (const node of body.content) {
    if (node.type === 'heading' && node.attrs.level === 2) {
      if (current.kind === 'SECTION' || current.nodes.length) pieces.push(current);
      current = { kind: 'SECTION', title: headingTitle(node), heading: node, nodes: [] };
    } else current.nodes.push(node);
  }
  if (current.kind === 'SECTION' || current.nodes.length) pieces.push(current);
  if (!pieces.some((piece) => piece.kind === 'SECTION')) throw failure('STRUCTURE_LEVEL2_MISSING');
  const originalIds = new Set(blockIds(body.content));
  return pieces.map(({ nodes, ...piece }) => {
    const part = {
      ...piece,
      blockIds: blockIds(piece.heading ? [piece.heading, ...nodes] : nodes),
    };
    if (!nodes.length) {
      // ProseMirror根节点要求至少一个块。确定性占位不冒充原来源；只有未改动的空占位可在合章时消除。
      part.emptyBlockId =
        'structure-empty-' +
        createHash('sha256').update(piece.heading.attrs.id).digest('hex').slice(0, 32);
      if (originalIds.has(part.emptyBlockId)) throw failure('STRUCTURE_BLOCK_ID_COLLISION');
      nodes = [emptyParagraph(part.emptyBlockId)];
    }
    part.body = normalizeContent({ type: 'doc', content: nodes }, { workspaceKind });
    return part;
  });
}

export function joinChapterBodies(parts, { workspaceKind = 'BRANCH' } = {}) {
  if (!Array.isArray(parts) || !parts.length) throw failure('STRUCTURE_PARTS_INVALID');
  const content = [];
  let sections = 0;
  for (const [index, part] of parts.entries()) {
    const body = normalizeContent(part.body, { workspaceKind });
    if (part.kind === 'INTRODUCTION') {
      if (index !== 0 || part.heading) throw failure('STRUCTURE_PARTS_INVALID');
    } else if (
      part.kind === 'SECTION' &&
      part.heading?.type === 'heading' &&
      part.heading.attrs?.level === 2
    ) {
      if (headingTitle(part.heading) !== part.title) throw failure('STRUCTURE_TITLE_CHANGED');
      content.push(structuredClone(part.heading));
      sections++;
    } else throw failure('STRUCTURE_PARTS_INVALID');
    const untouchedEmpty =
      part.emptyBlockId &&
      body.content.length === 1 &&
      canonical(body.content[0]) === canonical(emptyParagraph(part.emptyBlockId));
    if (!untouchedEmpty) content.push(...body.content);
  }
  if (!sections) throw failure('STRUCTURE_LEVEL2_MISSING');
  return normalizeContent({ type: 'doc', content }, { workspaceKind });
}

// MAIN与已对齐的可靠草稿分别迁移，绝不把S2当成已接纳主线。固定H2边界必须一致，支线自改边界须先正常送审。
export function splitChapterDrafts(mainInput, draftInput) {
  const main = splitChapterBody(mainInput, { workspaceKind: 'MAIN' });
  const draft = splitChapterBody(draftInput);
  const mainSections = main.filter((p) => p.kind === 'SECTION'),
    draftSections = draft.filter((p) => p.kind === 'SECTION');
  if (
    mainSections.length !== draftSections.length ||
    mainSections.some(
      (part, i) =>
        canonical(draftBody({ type: 'doc', content: [part.heading] }).content[0]) !==
        canonical(draftSections[i].heading),
    )
  ) {
    throw failure('DRAFT_STRUCTURE_CHANGED');
  }
  const output = [];
  const mainIntro = main.find((p) => p.kind === 'INTRODUCTION'),
    draftIntro = draft.find((p) => p.kind === 'INTRODUCTION');
  if (mainIntro || draftIntro) {
    const emptyId =
      'structure-intro-' +
      createHash('sha256').update(mainSections[0].heading.attrs.id).digest('hex').slice(0, 32);
    if (
      blockIds(mainInput.content).includes(emptyId) ||
      blockIds(draftInput.content).includes(emptyId)
    )
      throw failure('STRUCTURE_BLOCK_ID_COLLISION');
    const empty = { type: 'doc', content: [emptyParagraph(emptyId)] };
    output.push({
      kind: 'INTRODUCTION',
      title: '',
      main: mainIntro?.body ?? empty,
      draft: draftIntro?.body ?? structuredClone(empty),
      mainBlockIds: mainIntro?.blockIds ?? [],
      draftBlockIds: draftIntro?.blockIds ?? [],
      emptyBlockId: emptyId,
    });
  }
  for (const [index, part] of mainSections.entries()) {
    const pending = draftSections[index];
    output.push({
      kind: 'SECTION',
      title: part.title,
      heading: part.heading,
      main: part.body,
      draft: pending.body,
      mainBlockIds: part.blockIds,
      draftBlockIds: pending.blockIds,
      emptyBlockId: part.emptyBlockId ?? pending.emptyBlockId,
    });
  }
  return output;
}
