const publicPaths = new Set([
  '/v1/account/login',
  '/v1/account/challenges',
  '/v1/account/signup',
  '/v1/account/password-resets',
]);
// 与Account最小receipt边界一致：分享链接与任何目录不能混入操作进度的豁免。
export function accountUnleasedResponse(path, body) {
  if (publicPaths.has(path)) return true;
  let value;
  try {
    value = JSON.parse(body);
  } catch {
    return false;
  }
  if (minimalClosureResponse(path, value, body)) return true;
  if (
    !value ||
    typeof value !== 'object' ||
    Array.isArray(value) ||
    !Object.keys(value).every((k) => ['operation_id', 'state', 'poll_url', 'result'].includes(k))
  )
    return false;
  if (
    typeof value.operation_id !== 'string' ||
    !/^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$/.test(value.operation_id) ||
    !['PREPARING', 'FENCING', 'SUCCEEDED', 'REJECTED', 'CANCELLED'].includes(value.state)
  )
    return false;
  return (
    value.result === undefined ||
    (value.result &&
      Object.keys(value.result).length === 1 &&
      typeof value.result.entity_id === 'string' &&
      /^[1-9][0-9]{0,19}$/.test(value.result.entity_id))
  );
}

// 与Account专用最小计划一致。编号之外的个人资料、正文、重复键及其他路由
// 不能借冻结会话的恢复入口绕过输出租约。
function minimalClosureResponse(path, value, body) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
  const operation = /^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$/;
  const entity = (id) =>
    typeof id === 'string' && /^[1-9][0-9]{0,19}$/.test(id) && BigInt(id) <= 18446744073709551615n;
  if (
    typeof value.operation_id !== 'string' ||
    !operation.test(value.operation_id) ||
    (path !== '/v1/account/closures' && path !== '/v1/account/closures/' + value.operation_id) ||
    !['PREPARING', 'FROZEN', 'PLANNED', 'CLEANING', 'SUCCEEDED', 'REJECTED'].includes(
      value.state,
    ) ||
    !Array.isArray(value.required_successions) ||
    !Array.isArray(value.retained_release_ids)
  )
    return false;
  for (const item of value.required_successions) {
    if (
      !item ||
      !['ORGANIZATION_OWNER', 'ORGANIZATION_ADMIN', 'KNOWLEDGE_BASE_ADMIN'].includes(item.kind) ||
      !entity(item.target_id) ||
      Object.keys(item).length !== 2
    )
      return false;
  }
  if (!value.retained_release_ids.every(entity)) return false;
  const canonical = JSON.stringify({
    operation_id: value.operation_id,
    state: value.state,
    required_successions: value.required_successions,
    retained_release_ids: value.retained_release_ids,
  });
  const text = Buffer.isBuffer(body) ? body.toString('utf8') : body;
  return (
    typeof text === 'string' &&
    Buffer.byteLength(text) <= 2 << 20 &&
    (text === canonical || text === canonical + '\n')
  );
}
