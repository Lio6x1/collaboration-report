import { randomUUID } from 'node:crypto';

const emptySearchBytes = Buffer.from('{"items":[],"next_cursor":null,"has_more":false}');
const emptySearchEncoderBytes = Buffer.concat([emptySearchBytes, Buffer.from('\n')]);

// 唯一无租约的Doc成功输出是匿名、确实耗尽的搜索空页。直接比较原UTF-8字节，
// 不解析/再序列化JSON，避免重复键遮住私密前缀却将原正文释放给浏览器。
export function anonymousSearchEmptyResponse({ authority, method, path, status, cookie, body }) {
  if (
    authority !== 'document' ||
    method !== 'GET' ||
    path !== '/v1/search' ||
    status !== 200 ||
    !Buffer.isBuffer(body)
  )
    return false;
  if (cookie !== undefined) {
    if (typeof cookie !== 'string') return false;
    // 与Doc OptionalActor一样检查原Cookie名，包括空值、重复和不可解析的会话。
    // NEL是Go TrimSpace的空白而不是JS trim的空白；多拒绝FEFF也不会扩大豁免。
    const trim = (part) => part.replace(/^[\s\u0085]+|[\s\u0085]+$/gu, '');
    if (
      cookie.split(';').some((part) => trim(trim(part).split('=', 1)[0]) === '__Host-collabSession')
    )
      return false;
  }
  // Go json.Marshal无换行，platform.WriteJSON的Encoder恰好一次LF；其它表示拒绝。
  return body.equals(emptySearchBytes) || body.equals(emptySearchEncoderBytes);
}

export function fail(res, status, code) {
  if (res.headersSent) {
    res.destroy();
    return;
  }
  res.writeHead(status, {
    'content-type': 'application/json; charset=utf-8',
    'cache-control': 'no-store',
  });
  res.end(
    JSON.stringify({
      code,
      message: '请求暂时无法处理',
      trace_id: randomUUID(),
      retry_action: status === 503 ? 'RETRY_SAME_OPERATION' : 'NONE',
    }),
  );
}

export function sendBufferedResponse(res, { status, headers, body, canRelease }) {
  if (res.destroyed) return;
  // 原生负载保护可以只写 5xx 状态。补齐公开错误契约，客户端才可沿原号续查；
  // 不携带上游 content-length:0 等头，且不关闭负载保护或重新提交业务操作。
  if (status >= 500 && status <= 599 && body.length === 0) {
    fail(res, 503, 'SERVICE_UNAVAILABLE');
    return;
  }
  // 成功正文只在完整接收后检查原输出准入；调用者提供实时检查，不能缓存旧授权。
  if (status < 400 && !canRelease()) {
    fail(res, 503, 'OUTPUT_LEASE_REQUIRED');
    return;
  }
  res.writeHead(status, headers);
  res.end(body);
}
