const cookieNames = ['__Host-collabSession', '__Host-collabCSRF', '__Host-collabRecovery'];
const contextPattern = /^[a-f0-9]{32}$/;

// context只是标签页的Cookie选择器，不是会话秘密，也不是身份或权限证明。
// Edge只把所选的HttpOnly凭据还原给原有服务；各服务继续完成准入、CSRF和撤权核验。
export function browserSessionContext(headers, url, method) {
  const header = headers['x-collab-session-context'];
  const query = url.searchParams.getAll('session_context');
  if (query.length) {
    if (
      method !== 'GET' ||
      !/^\/v1\/(resources\/[1-9][0-9]*\/content|export-jobs\/[1-9][0-9]*\/download)$/.test(
        url.pathname,
      ) ||
      query.length !== 1
    )
      throw Error('SESSION_CONTEXT_INVALID');
    url.searchParams.delete('session_context');
  }
  const context = header ?? query[0] ?? null;
  if (
    (context !== null && (typeof context !== 'string' || !contextPattern.test(context))) ||
    (header !== undefined && query.length && header !== query[0])
  )
    throw Error('SESSION_CONTEXT_INVALID');
  if (headers.cookie !== undefined && typeof headers.cookie !== 'string')
    throw Error('SESSION_CONTEXT_INVALID');
  const parts = (headers.cookie ?? '')
    .split(';')
    .map((part) => part.trim())
    .filter(Boolean);
  const selected = [];
  if (context) {
    for (const name of cookieNames) {
      const exact = name + '.' + context;
      const matches = parts.filter((part) => part.split('=', 1)[0] === exact);
      if (matches.length > 1) throw Error('SESSION_CONTEXT_INVALID');
      if (matches.length) selected.push(name + matches[0].slice(exact.length));
    }
    // 有上限地保留并行登录，防止关闭标签后遗留的12小时Cookie撑满浏览器请求头。
    // 已有slot的请求不受影响；退出当前账号会由原登出路径删除它的Cookie。
    const sessionPrefix = cookieNames[0] + '.';
    const sessions = parts.filter((part) => part.startsWith(sessionPrefix));
    if (
      method === 'POST' &&
      url.pathname === '/v1/account/login' &&
      !sessions.some((part) => part.startsWith(sessionPrefix + context + '=')) &&
      sessions.length >= 16
    )
      throw Error('BROWSER_SESSION_LIMIT');
  } else {
    // 保持原API/脚本的Cookie协议；新标签页Cookie不能混进无选择器的匿名或旧客户端请求。
    selected.push(
      ...parts.filter((part) => !cookieNames.some((name) => part.startsWith(name + '.'))),
    );
  }
  return { context, cookie: selected.length ? selected.join('; ') : undefined };
}

export function scopedResponseHeaders(headers, context) {
  if (!context || !headers['set-cookie']) return headers;
  const original = Array.isArray(headers['set-cookie'])
    ? headers['set-cookie']
    : [headers['set-cookie']];
  return {
    ...headers,
    'set-cookie': original.map((value) => {
      const name = cookieNames.find((name) => value.startsWith(name + '='));
      // 只改名，Secure/HttpOnly/SameSite/有效期/删除属性全部保留原服务给出的值。
      return name ? name + '.' + context + value.slice(name.length) : value;
    }),
  };
}
