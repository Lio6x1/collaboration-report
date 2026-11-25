import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadHTTPContract } from './load-http-contract.mjs';
const root = fileURLToPath(new URL('../', import.meta.url));
const api = loadHTTPContract(root);
const literal = (x) => JSON.stringify(x);
function type(schema) {
  if (!schema) return 'unknown';
  if (schema.$ref) return schema.$ref.split('/').at(-1);
  if (schema.oneOf || schema.anyOf) return (schema.oneOf || schema.anyOf).map(type).join(' | ');
  if (schema.allOf) return schema.allOf.map(type).join(' & ');
  if (schema.enum) return schema.enum.map(literal).join(' | ') + (schema.nullable ? ' | null' : '');
  let t;
  switch (schema.type) {
    case 'string':
      t = 'string';
      break;
    case 'integer':
    case 'number':
      t = 'number';
      break;
    case 'boolean':
      t = 'boolean';
      break;
    case 'array':
      t = `Array<${type(schema.items)}>`;
      break;
    case 'object': {
      const required = new Set(schema.required ?? []);
      const fields = Object.entries(schema.properties ?? {}).map(
        ([k, v]) => `${literal(k)}${required.has(k) ? '' : '?'}: ${type(v)};`,
      );
      if (schema.additionalProperties === true) fields.push('[key: string]: unknown;');
      else if (schema.additionalProperties && typeof schema.additionalProperties === 'object')
        fields.push(`[key: string]: ${type(schema.additionalProperties)};`);
      t = `{ ${fields.join(' ')} }`;
      break;
    }
    default:
      t = 'unknown';
  }
  return t + (schema.nullable ? ' | null' : '');
}
const definitions = Object.entries(api.components.schemas).map(
  ([name, schema]) => `export type ${name} = ${type(schema)};`,
);
writeFileSync(
  resolve(root, 'contracts/http/generated.ts'),
  '// Generated from the fixed OpenAPI specification. Do not edit.\n' +
    definitions.join('\n') +
    '\n',
);
const fieldName = (k) =>
  k
    .split(/[^A-Za-z0-9]+/)
    .filter(Boolean)
    .map((x) => x[0].toUpperCase() + x.slice(1))
    .join('');
function goType(schema) {
  if (schema.$ref) return schema.$ref.split('/').at(-1);
  if (schema.oneOf || schema.anyOf || schema.allOf) return 'json.RawMessage';
  switch (schema.type) {
    case 'string':
      return 'string'; // uint64实体ID在HTTP中始终是十进制字符串。
    case 'integer':
      return 'int64';
    case 'number':
      return 'float64';
    case 'boolean':
      return 'bool';
    case 'array':
      return '[]' + goType(schema.items);
    case 'object': {
      if (!schema.properties) return 'map[string]json.RawMessage';
      const required = new Set(schema.required ?? []);
      return (
        'struct {\n' +
        Object.entries(schema.properties)
          .map(
            ([k, v]) =>
              `${fieldName(k)} ${(!required.has(k) || v.nullable ? '*' : '') + goType(v)} ${'`'}json:"${k}${required.has(k) ? '' : ',omitempty'}"${'`'}`,
          )
          .join('\n') +
        '\n}'
      );
    }
    default:
      return 'json.RawMessage';
  }
}
mkdirSync(resolve(root, 'contracts/httpdto'), { recursive: true });
const requiredFields = Object.entries(api.components.schemas)
  .filter(([, schema]) => schema.type === 'object')
  .map(
    ([name, schema]) =>
      `${literal(name)}: {${(schema.required ?? []).map((key) => `${literal(key)}: ${Boolean(schema.properties[key]?.nullable)}`).join(',')}},`,
  )
  .join('\n');
writeFileSync(
  resolve(root, 'contracts/httpdto/generated.go'),
  '// Code generated from fixed OpenAPI; DO NOT EDIT.\npackage httpdto\nimport "encoding/json"\n' +
    Object.entries(api.components.schemas)
      .map(([name, schema]) => `type ${name} ${goType(schema)}`)
      .join('\n') +
    '\n// RequiredFields maps each required field to whether JSON null is allowed.\nvar RequiredFields = map[string]map[string]bool{\n' +
    requiredFields +
    '\n}\n',
);
const routes = [];
for (const [path, item] of Object.entries(api.paths))
  for (const [method, operation] of Object.entries(item))
    if (['get', 'post', 'put', 'patch', 'delete', 'head', 'options'].includes(method))
      routes.push({
        path,
        method: method.toUpperCase(),
        authority: operation['x-authority'],
        operationId: operation.operationId,
      });
writeFileSync(resolve(root, 'contracts/http/routes.json'), JSON.stringify(routes, null, 2) + '\n');
const proto = readFileSync(resolve(root, 'contracts/proto/internal.proto'), 'utf8');
const policy = {
  '/grpc.health.v1.Health/Check': ['account', 'document', 'search', 'collaboration'],
};
for (const match of proto.matchAll(/service (\w+)\s*\{([^}]+)\}/g)) {
  const service = match[1];
  for (const rpc of match[2].matchAll(/rpc (\w+)\(/g)) {
    const method = rpc[1];
    let callers;
    if (service === 'IdentityInternal') callers = ['document'];
    else if (service === 'CollaborationInternal') callers = ['document'];
    else if (service === 'SearchInternal') callers = ['document'];
    else if (method === 'GetPublicSnapshot') callers = ['search'];
    else if (
      [
        'FenceAccess',
        'GetFenceResult',
        'ReserveAdminRetirement',
        'PrepareLifecyclePlan',
        'ApplyLifecyclePlan',
      ].includes(method)
    )
      callers = ['account'];
    else callers = ['collaboration'];
    policy[`/collaboration.v1.${service}/${method}`] = callers;
  }
}
const policyText = JSON.stringify(policy, null, 2) + '\n';
writeFileSync(resolve(root, 'contracts/proto/caller-policy.json'), policyText);
mkdirSync(resolve(root, 'packages/platform'), { recursive: true });
writeFileSync(resolve(root, 'packages/platform/rpc-policy.json'), policyText);
console.log(
  JSON.stringify({
    httpOperations: routes.length,
    tsModels: definitions.length,
    rpcMethods: Object.keys(policy).length - 1,
  }),
);
