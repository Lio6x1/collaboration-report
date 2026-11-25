# services/collaboration/src/domain/resolution_draft.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/298-resolution-draft.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`computeResolution`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`622abb6f1f69677b431b875d54016752f96dbcfacf0b3d94b9e521d85c2e18cd`。[参考实现](../../../../services/collaboration/src/domain/resolution_draft.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
