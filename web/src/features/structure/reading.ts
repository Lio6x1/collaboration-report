import type { StructureEntry } from '../../../../contracts/http/generated';

// sort_order只在同一父节点内有意义；重建章组后再读正文，避免子节落到其他章后面。
// 阅读、写作入口和正式版复用同一结构投影，不修改任何固定目录或正文身份。
export function chapterGroups(entries: readonly StructureEntry[]) {
  const children = new Map<string, StructureEntry[]>();
  for (const entry of entries) {
    if (!entry.parent_node_id || !entry.unit_id) continue;
    const group = children.get(entry.parent_node_id) ?? [];
    group.push(entry);
    children.set(entry.parent_node_id, group);
  }
  return entries
    .filter((entry) => entry.kind === 'CHAPTER' && !entry.parent_node_id)
    .sort((a, b) => a.sort_order - b.sort_order)
    .map((chapter) => ({
      chapter,
      units: chapter.unit_id
        ? [chapter]
        : (children.get(chapter.node_id) ?? []).sort((a, b) => a.sort_order - b.sort_order),
    }));
}

export function unitTitle(entry: StructureEntry) {
  // 章前内容的固定title允许为空；只补显示名，不能写回正文或伪造一个H2边界。
  return entry.kind === 'INTRODUCTION' ? '章前内容' : entry.title;
}
