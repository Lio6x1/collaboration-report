import { normalizeContent } from '../../../../packages/content/index.mjs';
import { mergeContent } from './content_merge.mjs';
import { bad } from './shared_protocol.mjs';

// 修复页面基于旧送审快照，操作只叠加“原submitted→人工结果”的修改。
// 最新可靠草稿中的独立S2由三方规则保留；同处竞争返回冲突，不能整篇覆盖当前房间。
export function computeResolution(submitted, resolution, draft) {
  const [base, current] = [submitted, draft].map((body) =>
    normalizeContent(body, { workspaceKind: 'BRANCH' }),
  );
  let proposed;
  try {
    proposed = normalizeContent(resolution, { workspaceKind: 'BRANCH' });
  } catch {
    // 用户正文的确定错误可以沿原号拒绝；固定submitted/draft坏掉仍保留未知来源错误。
    throw bad('RESOLUTION_CONTENT_INVALID');
  }
  const result = mergeContent({ base, submitted: proposed, main: current });
  return {
    ...result,
    body: result.body && normalizeContent(result.body, { workspaceKind: 'BRANCH' }),
  };
}
