import assert from 'node:assert/strict';
import test from 'node:test';
import { createHash } from 'node:crypto';
import { cleanupAuthority, cleanupReply } from '../src/domain/account_document_cleanup.mjs';

test('PURGE uses original document authority and keeps the frozen CLEANED reply shape', () => {
  const request = { operation_id: 'purge_original_0001', document_id: '18446744073709551615' };
  const account = {
    ...request,
    scope_key: 'doc-acl:' + request.document_id,
    closed_epoch: '18446744073709551615',
    revocation_id: 'purge_fence_original_001',
  };
  account.plan_digest = createHash('sha256')
    .update(
      [
        'ACCOUNT_DOCUMENT_CLEANUP',
        request.operation_id,
        request.document_id,
        account.scope_key,
        account.closed_epoch,
        account.revocation_id,
      ].join('\n'),
    )
    .digest('hex');
  // 私有kind来自Doc mTLS authority，不能扩展或重命名冻结platform回执。
  const purge = cleanupAuthority({ ...account, cleanup_kind: 'PURGE' }, request);
  assert.deepEqual(cleanupReply(purge), { ...account, state: 'CLEANED' });
  assert.deepEqual(cleanupReply(cleanupAuthority(account, request)), {
    ...account,
    state: 'CLEANED',
  });
  assert.throws(() => cleanupAuthority({ ...account, cleanup_kind: 'OTHER' }, request));
  assert.throws(() =>
    cleanupAuthority(
      { ...account, operation_id: 'other_original_0001', cleanup_kind: 'PURGE' },
      request,
    ),
  );
});
