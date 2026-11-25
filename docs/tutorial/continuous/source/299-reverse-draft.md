# services/collaboration/src/domain/reverse_draft.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/299-reverse-draft.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`computeReverse`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { mergeContent } from './content_merge.mjs';
import { draftBody, computePull } from './draft_protocol.mjs';

// 先证明反向在当前MAIN仍兼容，再把同一修改叠加到可靠草稿；两个阶段都保留其它位置的新增。
// 任一阶段有重叠修改就停止，不能用历史整篇正文替换当前S2。
export function computeReverse(before, after, draft, main, branchBase) {
  const current = draftBody(main);
  const reversed = mergeContent({
    base: draftBody(after),
    submitted: draftBody(before),
    main: current,
  });
  if (!reversed.body) return reversed;
  // 支线未拉入的MAIN新增不代表用户删除。先按真实支线基线对齐，再叠加同一反向修改。
  const aligned = computePull(branchBase, draft, main);
  if (!aligned.body) return aligned;
  // 仍叠加原合并的反向，而非“当前MAIN到反向MAIN”的差量；MAIN已经恢复时后者为空，
  // 会错误保留支线里的旧S1。原前后正文才能区分应撤销的S1与后来独立的S2。
  const result = mergeContent({
    base: draftBody(after),
    submitted: draftBody(before),
    main: draftBody(aligned.body),
  });
  return { ...result, body: result.body && draftBody(result.body) };
}
````

UTF-8/LF全文SHA-256：`f3c37760c50277011633e3ebcf3086f540d0fe4586bfc84d5899ab1686f85322`。[参考实现](../../../../services/collaboration/src/domain/reverse_draft.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
