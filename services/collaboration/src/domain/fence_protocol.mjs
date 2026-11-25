import { createHash, timingSafeEqual } from 'node:crypto';
import { RPCFailure } from './shared_protocol.mjs';

// 固定目标摘要绑定撤权号、来源和代次；重试必须消费原计划，不能替换成当前可变权限。
export function binding(req) {
  const scope = req.closed_scopes?.[0],
    epoch = scope?.epoch;
  const key = scope?.scope_key ?? '',
    isSession = /^session:[a-f0-9]{64}$/.test(key),
    isAuthorizationSource =
      /^(?:(?:identity|credential|platform-admin|doc-acl|kb-acl|publication):[1-9][0-9]{0,19}|(?:org-member|org-admin|org-owner|org-create):[1-9][0-9]{0,19}:[1-9][0-9]{0,19})$/.test(
        key,
      );
  if (
    !/^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$/.test(req.revocation_id ?? '') ||
    String(req.generation) !== '1' ||
    req.closed_scopes?.length !== 1 ||
    (!isSession && !isAuthorizationSource) ||
    (isAuthorizationSource &&
      key
        .split(':')
        .slice(1)
        .some((x) => BigInt(x) > 18446744073709551615n)) ||
    // protobuf固定以十进制字符串传uint64；不能接纳已被Number舍入的代次或多种绑定拼法。
    (typeof epoch === 'number' && !Number.isSafeInteger(epoch)) ||
    !/^[1-9][0-9]{0,19}$/.test(String(epoch)) ||
    BigInt(epoch) < 1n ||
    BigInt(epoch) > 18446744073709551615n
  )
    throw new RPCFailure(3, 'BOOTSTRAP_FENCE_INVALID');
  const expected = createHash('sha256')
    .update(
      isSession
        ? 'SESSION_LOGOUT\n' + key.slice(8)
        : 'AUTHORIZATION_CHANGE\n' + key + '\n' + String(epoch),
    )
    .digest();
  const actual = Buffer.from(req.target_digest ?? []);
  if (actual.length !== 32 || !timingSafeEqual(actual, expected))
    throw new RPCFailure(3, 'BOOTSTRAP_FENCE_INVALID');
  return { bootstrap_target_digest: actual.toString('hex'), generation: '1' };
}
export function ack(req) {
  const scope = req.closed_scopes[0],
    digest = createHash('sha256')
      .update(
        [
          'bootstrap-fence-v1',
          'collaboration',
          req.revocation_id,
          scope.scope_key,
          String(scope.epoch),
          '1',
          Buffer.from(req.target_digest).toString('hex'),
        ].join('\n'),
      )
      .digest();
  return {
    state: 'ACKNOWLEDGED',
    revocation_id: req.revocation_id,
    closed_scopes: req.closed_scopes,
    generation: '1',
    result_digest: digest,
    missing_instances: [],
  };
}

export function assertSameBinding(row, scope, current) {
  const p =
    typeof row.target_permission === 'string'
      ? JSON.parse(row.target_permission)
      : row.target_permission;
  if (
    row.scope_key !== scope.scope_key ||
    String(row.epoch) !== String(scope.epoch) ||
    p.bootstrap_target_digest !== current.bootstrap_target_digest ||
    p.generation !== current.generation
  )
    throw new RPCFailure(6, 'FENCE_ID_CONFLICT');
}
