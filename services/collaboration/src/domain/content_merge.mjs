import { normalizeContent } from '../../../../packages/content/index.mjs';
import { MergeValues, clone, same, equal, semantic, hasFont } from './merge/values.mjs';
import { MergeBlocks } from './merge/blocks.mjs';

// 三方计算只消费固定业务正文。任何冲突都不交付可安装候选，调用方不能偷偷选择保底主线。
export function mergeContent({ base, submitted, main }) {
  const validate = (body) => normalizeContent(body, { workspaceKind: 'MAIN' });
  [base, submitted, main] = [base, submitted, main].map(validate);
  if (same(base, submitted) || same(main, submitted)) return { body: clone(main), conflicts: [] };
  if (equal(base, main) && !hasFont(main))
    return { body: validate(semantic(submitted)), conflicts: [] };
  const values = new MergeValues();
  const result = { type: 'doc', content: new MergeBlocks(values).roots(base, submitted, main) };
  if (!values.conflicts.length) {
    try {
      return { body: validate(result), conflicts: [] };
    } catch {
      values.report(
        'INVALID_MERGE_RESULT',
        '',
        '$root',
        '组合后的正文无法通过结构与表格几何校验。',
      );
    }
  }
  return { body: null, conflicts: values.conflicts };
}
