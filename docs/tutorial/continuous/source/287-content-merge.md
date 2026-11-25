# services/collaboration/src/domain/content_merge.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/287-content-merge.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`mergeContent`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`0cc7dc2d0287e1b59d04db6dd0861ed0d64046cdbcc60307ec7a881572bd3aea`。[参考实现](../../../../services/collaboration/src/domain/content_merge.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
