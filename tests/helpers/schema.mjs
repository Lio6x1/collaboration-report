import { after } from 'node:test';
import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { resolve } from 'node:path';
import { root } from '../../scripts/runtime-lib.mjs';
import { loadHTTPContract } from '../../scripts/load-http-contract.mjs';
const api = loadHTTPContract(root),
  routes = JSON.parse(readFileSync(resolve(root, 'contracts/http/routes.json'))),
  observed = [];
const compiled = routes.map((r) => ({
  ...r,
  pattern: new RegExp('^' + r.path.replace(/\{[^}]+\}/g, '[^/]+') + '$'),
}));
export function observe(method, path, response) {
  if (response.status < 200 || response.status >= 300) return response;
  const route = compiled.find((r) => r.method === method && r.pattern.test(path.split('?')[0]));
  if (!route) return response;
  const schema =
    api.paths[route.path][method.toLowerCase()].responses[String(response.status)]?.content?.[
      'application/json'
    ]?.schema;
  if (!schema?.$ref)
    throw Error('UNDECLARED_SUCCESS_STATUS: ' + method + ' ' + route.path + ' ' + response.status);
  observed.push({ schema: schema.$ref.split('/').at(-1), value: JSON.parse(response.body) });
  return response;
}
after(() => {
  if (observed.length) {
    execFileSync('python', [resolve(root, 'scripts/validate-responses.py')], {
      input: JSON.stringify(observed),
      encoding: 'utf8',
      windowsHide: true,
      maxBuffer: 4096,
    });
  }
});
