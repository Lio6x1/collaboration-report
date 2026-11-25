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
