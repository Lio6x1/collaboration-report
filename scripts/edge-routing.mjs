import { readFileSync } from 'node:fs';
const routes = JSON.parse(readFileSync(new URL('../contracts/http/routes.json', import.meta.url)));
const compiled = routes.map((route) => ({
  ...route,
  pattern: new RegExp(
    '^' +
      route.path
        .split('/')
        .map((part) =>
          part.startsWith('{') ? '[^/]+' : part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'),
        )
        .join('/') +
      '$',
  ),
}));
export function routeFor(method, path) {
  const route = compiled.find((item) => item.method === method && item.pattern.test(path));
  return route ? { authority: route.authority, operationId: route.operationId } : null;
}
export function filterProxyHeaders(headers) {
  // 同源入口清除客户端自报身份；Cookie和Origin留给所属服务做真正的校验。
  const result = {};
  for (const [key, value] of Object.entries(headers)) {
    const name = key.toLowerCase();
    if (
      [
        'host',
        'connection',
        'transfer-encoding',
        'proxy-authorization',
        'x-actor-id',
        'x-user-id',
        'x-service-role',
        'x-role',
        'x-identity-admission',
        'x-document-admission',
        'x-edge-handler',
        'x-output-lease',
      ].includes(name) ||
      name.startsWith('x-internal-')
    )
      continue;
    result[name] = value;
  }
  return result;
}
