# services/collaboration/src/domain/account_document_cleanup.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/285-account-document-cleanup.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`cleanupRequest`、`cleanupAuthority`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { createHash } from 'node:crypto';
import { validID, bad } from './shared_protocol.mjs';

const canonicalID = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;

export function cleanupRequest(input) {
  if (
    !input ||
    !validID(input.operation_id) ||
    !canonicalID(input.document_id) ||
    Object.keys(input).some((key) => !['operation_id', 'document_id'].includes(key))
  )
    throw bad('ACCOUNT_DOCUMENT_CLEANUP_INVALID');
  return { operation_id: input.operation_id, document_id: input.document_id };
}

export function cleanupAuthority(input, request) {
  const expected = createHash('sha256')
    .update(
      [
        'ACCOUNT_DOCUMENT_CLEANUP',
        request.operation_id,
        request.document_id,
        input?.scope_key,
        input?.closed_epoch,
        input?.revocation_id,
      ].join('\n'),
    )
    .digest('hex');
  if (
    (input?.cleanup_kind !== undefined && input.cleanup_kind !== 'PURGE') ||
    input?.operation_id !== request.operation_id ||
    input?.document_id !== request.document_id ||
    input?.scope_key !== 'doc-acl:' + request.document_id ||
    !canonicalID(input?.closed_epoch) ||
    !validID(input?.revocation_id) ||
    input?.plan_digest !== expected
  )
    throw bad('ACCOUNT_DOCUMENT_CLEANUP_AUTHORITY_INVALID');
  return input;
}

// cleanup_kind仅是已验Doc authority的内部业务分派，不进入冻结清理回执的字节。
// Account没有此字段，历史回执保持原形状与原摘要。
export const cleanupReply = (context) => {
  const { cleanup_kind: kind, ...fixed } = context;
  return { ...fixed, state: 'CLEANED' };
};
````

UTF-8/LF全文SHA-256：`5b795e4ec4eade8ad02a12c0fbe69f63209a91cda410e7af224e63603212d41e`。[参考实现](../../../../services/collaboration/src/domain/account_document_cleanup.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
