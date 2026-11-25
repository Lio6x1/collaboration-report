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
