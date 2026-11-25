import { createHash } from 'node:crypto';
import { bad, validID } from './shared_protocol.mjs';

const uint64 = (value, zero = false) =>
  typeof value === 'string' &&
  (zero ? /^(0|[1-9][0-9]{0,19})$/ : /^[1-9][0-9]{0,19}$/).test(value) &&
  BigInt(value) <= 18446744073709551615n;

export function transferStorageRequest(input) {
  if (
    !input ||
    !validID(input.operation_id) ||
    !uint64(input.document_id) ||
    input.generation !== '1' ||
    !Array.isArray(input.closures) ||
    input.closures.length !== 2 ||
    Object.keys(input).some(
      (key) => !['operation_id', 'document_id', 'generation', 'closures'].includes(key),
    )
  )
    throw bad('PERSONAL_TRANSFER_STORAGE_INVALID');
  const closures = input.closures
    .map((s) => {
      if (
        !s ||
        !validID(s.revocation_id) ||
        !uint64(s.epoch) ||
        !['doc-acl:' + input.document_id, 'publication:' + input.document_id].includes(
          s.scope_key,
        ) ||
        Object.keys(s).some((key) => !['revocation_id', 'scope_key', 'epoch'].includes(key))
      )
        throw bad('PERSONAL_TRANSFER_STORAGE_INVALID');
      return { revocation_id: s.revocation_id, scope_key: s.scope_key, epoch: s.epoch };
    })
    .sort((a, b) => a.scope_key.localeCompare(b.scope_key));
  if (
    closures[0].scope_key === closures[1].scope_key ||
    closures[0].revocation_id === closures[1].revocation_id
  )
    throw bad('PERSONAL_TRANSFER_STORAGE_INVALID');
  return {
    operation_id: input.operation_id,
    document_id: input.document_id,
    generation: '1',
    closures,
  };
}

export function transferStorageMeasurement(request, facts) {
  let bytes = 0n;
  // 只累加真实持久行的字节；编号/容量均保持uint64字符串，不把SUM或room ID转Number。
  for (const group of [facts.checkpoints, facts.updates, facts.captures])
    for (const row of group) {
      if (!uint64(String(row.bytes), true)) throw bad('PERSONAL_TRANSFER_STORAGE_OVERFLOW');
      bytes += BigInt(row.bytes);
    }
  if (bytes > 18446744073709551615n) throw bad('PERSONAL_TRANSFER_STORAGE_OVERFLOW');
  return {
    operation_id: request.operation_id,
    document_id: request.document_id,
    generation: '1',
    storage_bytes: bytes.toString(),
    snapshot_digest: createHash('sha256').update(JSON.stringify({ request, facts })).digest('hex'),
    source_closed: true,
  };
}
