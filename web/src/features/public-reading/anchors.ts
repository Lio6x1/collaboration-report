import type { StructureEntry } from '../../../../contracts/http/generated';

// 章节是稳定业务身份，不能把release/unit临时目录锚点当作搜索结果中的历史版本选择器。
export function chapterFromHash(hash: string): string | null {
  const match = /^#chapter-([1-9][0-9]{0,19})$/.exec(hash);
  return match && BigInt(match[1]) <= 18446744073709551615n ? match[1] : null;
}
export const chapterAnchor = (chapter: string) => 'chapter-' + chapter;
export function firstChapterUnit(
  entries: readonly Pick<StructureEntry, 'chapter_id' | 'unit_id'>[],
  chapter: string,
): string | null {
  return entries.find((entry) => entry.chapter_id === chapter && entry.unit_id)?.unit_id ?? null;
}
