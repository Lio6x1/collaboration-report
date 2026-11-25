# services/collaboration/src/domain/structure_preview_protocol.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/305-structure-preview-protocol.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`structurePreviewBinding`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { validID, bad } from './shared_protocol.mjs';

const unsigned = (value, positive = false) =>
  typeof value === 'string' &&
  (positive ? /^[1-9][0-9]{0,19}$/ : /^(0|[1-9][0-9]{0,19})$/).test(value) &&
  BigInt(value) <= 18446744073709551615n;

// 全篇预览与单支线拉取是不同的准入目标；不能把浏览器传来的任意unit扩大成全篇范围。
export function structurePreviewBinding(input) {
  const context = input?.context,
    target = context?.target,
    value = context?.request_fingerprint,
    fingerprint =
      typeof value === 'string' ? Buffer.from(value, 'base64') : Buffer.from(value ?? []);
  // RPC bytes和protojson base64必须表示同一个32字节摘要；拒绝宽松解码接受的尾部空白或非规范字符串。
  if (typeof value === 'string' && fingerprint.toString('base64') !== value)
    throw bad('STRUCTURE_PREVIEW_INVALID');
  if (
    !validID(context?.operation_id) ||
    !unsigned(context?.actor_id, true) ||
    !target ||
    target.purpose !== 'STRUCTURE_PREVIEW' ||
    !unsigned(target.document_id, true) ||
    !unsigned(target.structure_id, true) ||
    !unsigned(target.workspace_epoch) ||
    target.unit_id !== '0' ||
    target.branch_id !== '0' ||
    input.expected_branch_id !== '0' ||
    input.expected_lineage_id !== '0' ||
    input.expected_structure_id !== target.structure_id ||
    !context.identity_admission_ref ||
    !context.document_admission_ref ||
    !Array.isArray(context.sources) ||
    !context.sources.length ||
    fingerprint.length !== 32
  )
    throw bad('STRUCTURE_PREVIEW_INVALID');
  return {
    ...input,
    context: {
      ...context,
      request_fingerprint: fingerprint.toString('base64'),
    },
  };
}
````

UTF-8/LF全文SHA-256：`4dbbe099cb3b1f01ca59ac4c592c946a412470f28cb8b83187902b1c772983ff`。[参考实现](../../../../services/collaboration/src/domain/structure_preview_protocol.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
