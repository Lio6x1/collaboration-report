import { readFileSync, readdirSync } from 'node:fs';
import { resolve, join, relative } from 'node:path';
import { root, saveJSON } from './runtime-lib.mjs';

function files(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) =>
    entry.isDirectory() ? files(join(directory, entry.name)) : [join(directory, entry.name)],
  );
}
const source = (path) => readFileSync(path, 'utf8');
const routes = JSON.parse(source(resolve(root, 'contracts/http/routes.json')));
const registered = [];
for (const authority of ['account', 'document']) {
  for (const file of files(resolve(root, 'services', authority, 'internal/handler')).filter(
    (file) => file.endsWith('.go') && !file.endsWith('_test.go'),
  )) {
    for (const match of source(file).matchAll(/Method:\s*"(\w+)",\s*Path:\s*"([^"]+)"/g)) {
      registered.push({
        authority,
        method: match[1],
        path: match[2].replace(/:([^/]+)/g, '{$1}'),
        source: relative(root, file).replaceAll('\\', '/'),
      });
    }
  }
}
const missingHTTP = routes.filter(
  (route) =>
    !registered.some(
      (item) =>
        item.authority === route.authority &&
        item.method === route.method &&
        item.path === route.path,
    ),
);
const protocol = source(resolve(root, 'contracts/proto/internal.proto'));
const services = {
  IdentityInternal: 'account',
  DocumentInternal: 'document',
  CollaborationInternal: 'collaboration',
  SearchInternal: 'search',
};
const missingRPC = [];
for (const match of protocol.matchAll(/service\s+(\w+)\s*\{([\s\S]*?)\}/g)) {
  const authority = services[match[1]];
  const implemented =
    authority === 'collaboration'
      ? source(resolve(root, 'services/collaboration/src/transport/rpc.mjs'))
      : files(resolve(root, 'services', authority, 'internal/server'))
          .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
          .map(source)
          .join('\n');
  for (const method of match[2].matchAll(/rpc\s+(\w+)\s*\(/g)) {
    const pattern =
      authority === 'collaboration'
        ? new RegExp('handlers\\.' + method[1] + '\\s*=')
        : new RegExp('func\\s+\\([^)]*\\)\\s+' + method[1] + '\\s*\\(');
    if (!pattern.test(implemented))
      missingRPC.push({ authority, service: match[1], method: method[1] });
  }
}
// 这是源码登记审计，不代替真实行为验收。缺失的冻结RPC可能被强绑定
// 内部HTTP取代，但必须逐项解释；不能因为主页面可用就把stub也算完成。
const evidence = {
  state: missingHTTP.length || missingRPC.length ? 'REVIEW_REQUIRED' : 'REGISTRATION_COMPLETE',
  staticOnly: true,
  contractHTTP: routes.length,
  registeredHTTP: registered.length,
  missingHTTP,
  missingRPC,
  generatedAt: new Date().toISOString(),
};
saveJSON(resolve(root, 'artifacts/batches/08/product-registration-audit.json'), evidence);
console.log(JSON.stringify(evidence, null, 2));
