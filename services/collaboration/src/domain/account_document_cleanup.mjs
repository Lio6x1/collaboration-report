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
