import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { mergeImplementationExtensions } from './http-implementation-extensions.mjs';

const defaultRoot = fileURLToPath(new URL('../', import.meta.url));
const object = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);
const shape = (path) => path.replace(/\{[^}]+\}/g, '{}');

// 实施扩展只补只读入口/明确只读响应字段；冻结字节、已有字段与路径参数不能覆盖。
// 导出纯合并函数便于验证拒绝边界，调用者拿到副本，不能意外修改原契约。
export function mergeReadExtensions(baseline, extension) {
  if (
    !object(extension) ||
    Object.keys(extension).some((key) => !['paths', 'response_fields', 'schemas'].includes(key)) ||
    !object(extension.paths)
  )
    throw Error('HTTP_READ_EXTENSION_INVALID');
  const api = structuredClone(baseline);
  if (extension.schemas !== undefined) {
    // 只追加此读取用例的新响应模型，不能覆盖User/写入DTO或借机开放任意账号目录协议。
    const models = new Set([
      'DocumentCollaborator',
      'DocumentCollaboratorPage',
      'ReviewRequestSummary',
      'MainReadingRevision',
    ]);
    if (!object(extension.schemas)) throw Error('HTTP_READ_EXTENSION_SCHEMA_INVALID');
    for (const [name, schema] of Object.entries(extension.schemas)) {
      if (!models.has(name) || Object.hasOwn(api.components.schemas, name) || !object(schema))
        throw Error('HTTP_READ_EXTENSION_SCHEMA_INVALID');
      if (name === 'ReviewRequestSummary') {
        // 审核摘要仅允许固定轮次的五个轻量字段，不能借新模型引入完整正文或私人账号资料。
        const properties = {
          title: { type: 'string', maxLength: 240, readOnly: true },
          excerpt: { type: 'string', maxLength: 141, readOnly: true },
          round_no: { type: 'string', pattern: '^[1-9][0-9]{0,19}$', readOnly: true },
          submitter_id: { type: 'string', pattern: '^[1-9][0-9]{0,19}$', readOnly: true },
          submitted_at: { type: 'string', format: 'date-time', readOnly: true },
        };
        if (
          schema.type !== 'object' ||
          schema.additionalProperties !== false ||
          !object(schema.properties) ||
          !Array.isArray(schema.required) ||
          Object.keys(schema).some(
            (key) =>
              !['type', 'additionalProperties', 'properties', 'required', 'description'].includes(
                key,
              ),
          ) ||
          Object.keys(schema.properties).length !== 5 ||
          schema.required.length !== 5 ||
          new Set(schema.required).size !== 5 ||
          Object.entries(properties).some(([key, expected]) => {
            const actual = schema.properties[key];
            return (
              !schema.required.includes(key) ||
              !object(actual) ||
              Object.keys(actual).some(
                (field) => field !== 'description' && !Object.hasOwn(expected, field),
              ) ||
              Object.entries(expected).some(([field, value]) => actual[field] !== value)
            );
          })
        )
          throw Error('HTTP_READ_EXTENSION_SCHEMA_INVALID');
      }
      api.components.schemas[name] = structuredClone(schema);
    }
  }
  if (extension.response_fields !== undefined) {
    const fields = extension.response_fields;
    // 只追加明确的只读投影，不提供任意字段覆盖器或改变写入DTO。
    const allowed = {
      User: 'identity_epoch',
      ReviewRequest: 'summary',
      Member: 'knowledge_base_creation',
    };
    if (!object(fields)) throw Error('HTTP_READ_EXTENSION_FIELD_INVALID');
    for (const [model, additions] of Object.entries(fields)) {
      const field = Object.hasOwn(allowed, model) ? allowed[model] : undefined;
      const schema = additions?.[field];
      if (
        !field ||
        !object(additions) ||
        Object.keys(additions).length !== 1 ||
        !object(schema) ||
        schema.readOnly !== true ||
        Object.hasOwn(api.components.schemas[model].properties, field)
      )
        throw Error('HTTP_READ_EXTENSION_FIELD_INVALID');
      if (model === 'User') {
        if (
          Object.keys(schema).some(
            (key) => !['type', 'pattern', 'readOnly', 'description'].includes(key),
          ) ||
          schema.type !== 'string' ||
          schema.pattern !== '^[1-9][0-9]{0,19}$'
        )
          throw Error('HTTP_READ_EXTENSION_FIELD_INVALID');
        api.components.schemas.User.required.push(field);
      } else if (model === 'Member') {
        if (
          Object.keys(schema).some(
            (key) => !['type', 'enum', 'readOnly', 'description'].includes(key),
          ) ||
          schema.type !== 'string' ||
          JSON.stringify(schema.enum) !== JSON.stringify(['ROLE', 'GRANTED', 'NONE'])
        )
          throw Error('HTTP_READ_EXTENSION_FIELD_INVALID');
      } else if (
        Object.keys(schema).some((key) => !['$ref', 'readOnly', 'description'].includes(key)) ||
        schema.$ref !== '#/components/schemas/ReviewRequestSummary' ||
        !Object.hasOwn(api.components.schemas, 'ReviewRequestSummary')
      ) {
        throw Error('HTTP_READ_EXTENSION_FIELD_INVALID');
      }
      api.components.schemas[model].properties[field] = structuredClone(schema);
    }
  }
  const operations = new Set();
  const shapes = new Map();
  for (const [path, item] of Object.entries(api.paths)) {
    shapes.set(shape(path), path);
    for (const value of Object.values(item))
      if (value?.operationId) operations.add(value.operationId);
  }
  function checkRefs(value) {
    if (!value || typeof value !== 'object') return;
    if ('$ref' in value) {
      const prefix = '#/components/schemas/';
      if (
        typeof value.$ref !== 'string' ||
        !value.$ref.startsWith(prefix) ||
        !Object.hasOwn(api.components.schemas, value.$ref.slice(prefix.length))
      )
        throw Error('HTTP_READ_EXTENSION_SCHEMA_INVALID');
    }
    for (const child of Object.values(value)) checkRefs(child);
  }
  checkRefs(extension.schemas);
  for (const [path, item] of Object.entries(extension.paths)) {
    if (!path.startsWith('/v1/') || !object(item) || Object.keys(item).length === 0)
      throw Error('HTTP_READ_EXTENSION_PATH_INVALID');
    const existing = shapes.get(shape(path));
    if (existing && existing !== path) throw Error('HTTP_READ_EXTENSION_PATH_ALIAS');
    for (const [method, operation] of Object.entries(item)) {
      if (!['get', 'head'].includes(method) || !object(operation) || operation.requestBody)
        throw Error('HTTP_READ_EXTENSION_METHOD_INVALID');
      if (Object.hasOwn(api.paths[path] ?? {}, method)) throw Error('HTTP_READ_EXTENSION_OVERRIDE');
      if (
        typeof operation.operationId !== 'string' ||
        !operation.operationId ||
        operations.has(operation.operationId)
      )
        throw Error('HTTP_READ_EXTENSION_OPERATION_INVALID');
      if (operation['x-authority'] !== 'document' || !object(operation.responses))
        throw Error('HTTP_READ_EXTENSION_OPERATION_INVALID');
      // 两个姓名目录复用最小只读DTO，但准入不能互换；新增提及入口不放宽管理目录。
      const candidateCapability = {
        '/v1/documents/{documentId}/collaborators': 'DOCUMENT_MANAGEMENT',
        '/v1/documents/{documentId}/mention-candidates': 'COMMENT',
      }[path];
      if (
        candidateCapability &&
        (method !== 'get' ||
          operation['x-required-capability'] !== candidateCapability ||
          operation.responses['200']?.content?.['application/json']?.schema?.$ref !==
            '#/components/schemas/DocumentCollaboratorPage')
      )
        throw Error('HTTP_READ_EXTENSION_CANDIDATE_INVALID');
      checkRefs(operation);
      operations.add(operation.operationId);
      api.paths[path] ??= {};
      api.paths[path][method] = structuredClone(operation);
    }
    shapes.set(shape(path), path);
  }
  return api;
}

export function loadHTTPContract(root = defaultRoot) {
  const baseline = JSON.parse(readFileSync(resolve(root, 'contracts/http/openapi.json'), 'utf8'));
  const extension = JSON.parse(
    readFileSync(resolve(root, 'contracts/http/read-extensions.json'), 'utf8'),
  );
  const implementation = JSON.parse(
    readFileSync(resolve(root, 'contracts/http/implementation-extensions.json'), 'utf8'),
  );
  return mergeReadExtensions(mergeImplementationExtensions(baseline, implementation), extension);
}
