import { canonical, normalizeContent, resourceIds } from '../../../../packages/content/index.mjs';
import { hash } from './shared_protocol.mjs';
import { computePull, draftBody } from './draft_protocol.mjs';
import { splitChapterDrafts, joinChapterBodies } from './structure_content.mjs';

const failure = (code) => Object.assign(new Error(code), { code });
const rangeActions = new Set(['SPLIT_LEVEL2', 'JOIN_CHAPTER', 'REMOVE']);

function decodeBody(content, fixed, unit, branch, epoch) {
  const target = fixed.binding.context.target;
  if (
    !content ||
    content.document_id !== target.document_id ||
    content.unit_id !== unit ||
    content.structure_id !== target.structure_id ||
    content.branch_id !== branch ||
    content.operation_id !== fixed.binding.context.operation_id ||
    (epoch !== undefined && content.workspace_epoch !== epoch)
  )
    throw failure('STRUCTURE_CONTENT_UNKNOWN');
  try {
    const bytes = Buffer.from(content.body_json_utf8, 'base64');
    const body = normalizeContent(JSON.parse(bytes), {
      workspaceKind: branch === '0' ? 'MAIN' : 'BRANCH',
    });
    if (
      bytes.length > 16 << 20 ||
      String(bytes.length) !== content.byte_size ||
      hash(bytes).toString('base64') !== content.body_hash ||
      canonical(body) !== bytes.toString() ||
      canonical(resourceIds(body)) !== canonical(content.resource_ids)
    )
      throw failure('STRUCTURE_CONTENT_UNKNOWN');
    return body;
  } catch {
    throw failure('STRUCTURE_CONTENT_UNKNOWN');
  }
}

function indexExact(items, units) {
  if (!Array.isArray(items) || items.length !== units.size)
    throw failure('STRUCTURE_SCOPE_UNKNOWN');
  const index = new Map();
  for (const item of items) {
    if (!units.has(item.unit_id) || index.has(item.unit_id))
      throw failure('STRUCTURE_SCOPE_UNKNOWN');
    index.set(item.unit_id, item);
  }
  return index;
}

function chapterGroups(structure) {
  const chapters = new Map(),
    units = new Set(),
    nodes = new Set();
  for (const entry of structure.entries) {
    if (nodes.has(entry.node_id)) throw failure('STRUCTURE_SCOPE_UNKNOWN');
    nodes.add(entry.node_id);
    if (entry.kind === 'CHAPTER' && !entry.parent_node_id) {
      if (chapters.has(entry.chapter_id)) throw failure('STRUCTURE_SCOPE_UNKNOWN');
      chapters.set(entry.chapter_id, { root: entry, children: [] });
    }
  }
  for (const entry of structure.entries) {
    const chapter = chapters.get(entry.chapter_id);
    if (!chapter) throw failure('STRUCTURE_SCOPE_UNKNOWN');
    if (entry !== chapter.root) {
      if (
        !chapter.root.split_level2 ||
        entry.parent_node_id !== chapter.root.node_id ||
        !['INTRODUCTION', 'SECTION'].includes(entry.kind)
      )
        throw failure('STRUCTURE_SCOPE_UNKNOWN');
      chapter.children.push(entry);
    }
    if (entry.unit_id) {
      if (units.has(entry.unit_id)) throw failure('STRUCTURE_SCOPE_UNKNOWN');
      units.add(entry.unit_id);
    }
  }
  for (const chapter of chapters.values()) {
    if (chapter.root.split_level2) {
      if (
        chapter.root.unit_id ||
        !chapter.children.length ||
        chapter.children.some((v) => !v.unit_id)
      )
        throw failure('STRUCTURE_SCOPE_UNKNOWN');
      chapter.children.sort((a, b) => a.sort_order - b.sort_order);
      if (
        new Set(chapter.children.map((v) => v.sort_order)).size !== chapter.children.length ||
        chapter.children.filter((v) => v.kind === 'INTRODUCTION').length > 1 ||
        chapter.children.some((v, i) => v.kind === 'INTRODUCTION' && i !== 0) ||
        !chapter.children.some((v) => v.kind === 'SECTION')
      )
        throw failure('STRUCTURE_SCOPE_UNKNOWN');
      chapter.units = chapter.children;
    } else {
      if (!chapter.root.unit_id || chapter.children.length)
        throw failure('STRUCTURE_SCOPE_UNKNOWN');
      chapter.units = [chapter.root];
    }
  }
  return { chapters, units };
}

function blockIds(body) {
  const ids = [];
  const visit = (node) => {
    if (node.attrs?.id) ids.push(node.attrs.id);
    for (const child of node.content ?? []) visit(child);
  };
  visit(body);
  return ids;
}

function sourceRange(unit, main, draft, start = null, end = null) {
  return {
    source_unit_id: unit,
    source_scope: 'WHOLE_UNIT',
    start_boundary: start,
    end_boundary: end,
    main_block_ids: main,
    draft_block_ids: draft,
  };
}

function splitTargets(change, main, draft, lineage) {
  // 预览内部虚拟拉取同一固定MAIN：旧基线、原草稿与MAIN都来自原固定事实，不写回房间或原草稿。
  // 缺失谱系不能拿MAIN冒充base；否则会吞掉支线与主线对同一块的真实冲突。
  if (!lineage?.body) throw failure('STRUCTURE_LINEAGE_UNKNOWN');
  const aligned = computePull(lineage.body, draft, main);
  if (aligned.conflicts.length) throw failure('STRUCTURE_DRAFT_CONFLICT');
  // 三方对齐只补主线变化。支线独自改过的H2仍由严格分界校验拒绝，不能借拆分接纳未审核目录。
  const pieces = splitChapterDrafts(main, aligned.body);
  const sections = pieces.filter((v) => v.kind === 'SECTION');
  return pieces.map((part, i) => {
    const section = sections.indexOf(part);
    const start = part.kind === 'INTRODUCTION' ? null : part.heading.attrs.id;
    const end =
      part.kind === 'INTRODUCTION'
        ? sections[0].heading.attrs.id
        : (sections[section + 1]?.heading.attrs.id ?? null);
    return {
      key: `${change.chapter_id}:SPLIT:${i}`,
      kind: part.kind,
      title: part.title,
      main_body: part.main,
      draft_body: part.draft,
      boundary: {
        ...(part.heading && { heading: part.heading }),
        ...(part.emptyBlockId && { empty_block_id: part.emptyBlockId }),
      },
      source_ranges: [
        sourceRange(change.source_units[0], part.mainBlockIds, part.draftBlockIds, start, end),
      ],
    };
  });
}

function joinTarget(change, chapter, bodies, boundaries) {
  const parts = chapter.units.map((entry) => {
    const boundary = boundaries.get(entry.unit_id);
    // SECTION管理标题的原节点必须来自已固定的完整谱系，不能由标题文字重新生成一个身份。
    if (!boundary || (entry.kind === 'SECTION' && !boundary.heading))
      throw failure('STRUCTURE_BOUNDARY_UNKNOWN');
    return {
      kind: entry.kind,
      title: entry.title,
      heading: boundary.heading,
      emptyBlockId: boundary.empty_block_id,
    };
  });
  const main = joinChapterBodies(
    parts.map((part, i) => ({ ...part, body: bodies.get(chapter.units[i].unit_id).main })),
    { workspaceKind: 'MAIN' },
  );
  const draft = joinChapterBodies(
    parts.map((part, i) => ({
      ...part,
      heading: part.heading && draftBody({ type: 'doc', content: [part.heading] }).content[0],
      body: bodies.get(chapter.units[i].unit_id).draft,
    })),
  );
  return {
    key: `${change.chapter_id}:JOIN`,
    kind: 'CHAPTER',
    title: chapter.root.title,
    main_body: main,
    draft_body: draft,
    boundary: {},
    source_ranges: chapter.units.map((entry, i) => {
      const pair = bodies.get(entry.unit_id);
      return {
        ...sourceRange(entry.unit_id, blockIds(pair.main), blockIds(pair.draft)),
        target_scope: 'INTERVAL',
        target_start_boundary: parts[i].heading?.attrs.id ?? null,
        target_end_boundary: parts[i + 1]?.heading?.attrs.id ?? null,
      };
    }),
  };
}

// 只消费同一个固定预览；不访问时钟、数据库或活动房间，候选与范围声明一起受原摘要保护。
export function buildStructureMigration(fixed) {
  const source = fixed.source,
    target = fixed.binding?.context?.target;
  if (
    !target ||
    !source?.structure ||
    source.structure.id !== target.structure_id ||
    source.structure.document_id !== target.document_id ||
    !Array.isArray(source.structure.entries) ||
    !Array.isArray(source.changes)
  )
    throw failure('STRUCTURE_SCOPE_UNKNOWN');
  const { chapters, units } = chapterGroups(source.structure);
  const main = indexExact(fixed.main_units, units),
    drafts = indexExact(fixed.drafts, units);
  const workspaces = indexExact(source.drafts, units),
    bodies = new Map();
  for (const unit of units) {
    const workspace = workspaces.get(unit);
    bodies.set(unit, {
      main: decodeBody(main.get(unit), fixed, unit, '0'),
      draft: decodeBody(
        drafts.get(unit),
        fixed,
        unit,
        workspace.branch_id,
        workspace.workspace_epoch,
      ),
    });
  }
  const boundaries = new Map();
  for (const boundary of source.unit_boundaries ?? []) {
    if (!units.has(boundary.unit_id) || boundaries.has(boundary.unit_id))
      throw failure('STRUCTURE_BOUNDARY_UNKNOWN');
    boundaries.set(boundary.unit_id, boundary);
  }
  const changes = [],
    affected = new Set();
  for (const input of source.changes) {
    if (!rangeActions.has(input.type)) continue;
    const chapter = chapters.get(input.chapter_id);
    if (!chapter) throw failure('STRUCTURE_SCOPE_UNKNOWN');
    if (affected.has(input.chapter_id)) throw failure('STRUCTURE_RANGE_OVERLAP');
    affected.add(input.chapter_id);
    const change = {
      type: input.type,
      chapter_id: input.chapter_id,
      source_units: chapter.units.map((v) => v.unit_id),
      targets: [],
    };
    if (input.type === 'SPLIT_LEVEL2') {
      if (chapter.root.split_level2) throw failure('STRUCTURE_MODE_CHANGED');
      const pair = bodies.get(chapter.root.unit_id);
      change.targets = splitTargets(
        change,
        pair.main,
        pair.draft,
        workspaces.get(chapter.root.unit_id).lineage,
      );
    } else if (input.type === 'JOIN_CHAPTER') {
      if (!chapter.root.split_level2) throw failure('STRUCTURE_MODE_CHANGED');
      change.targets = [joinTarget(change, chapter, bodies, boundaries)];
    }
    for (const candidate of change.targets)
      if (
        Buffer.byteLength(canonical(candidate.main_body)) > 16 << 20 ||
        Buffer.byteLength(canonical(candidate.draft_body)) > 16 << 20
      )
        throw failure('STRUCTURE_CONTENT_LIMIT');
    changes.push(change);
  }
  return { schema_version: 'structure-migration-v1', changes };
}
