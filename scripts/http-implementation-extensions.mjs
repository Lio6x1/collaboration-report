const object = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);
const shape = (path) => path.replace(/\{[^}]+\}/g, '{}');
const paths = new Map([
  ['/v1/documents/{documentId}/personal-transfers', ['post']],
  ['/v1/personal-transfers/{transferId}', ['get']],
  ['/v1/personal-transfers', ['get']],
  ['/v1/personal-transfers/{transferId}/accept', ['post']],
  ['/v1/personal-transfers/{transferId}/cancel', ['post']],
  ['/v1/personal-transfers/{transferId}/reject', ['post']],
  ['/v1/document-invitations/{invitationId}/archive', ['post']],
  ['/v1/document-applications/{applicationId}/archive', ['post']],
]);
const models = new Set([
  'PersonalTransferMetadata',
  'PersonalTransferPage',
  'PersonalTransferAccept',
  'OrganizationMembership',
]);

// 实施协议缺口只允许这组明确新入口/模型。拒绝覆盖冻结写入、路径参数别名与旧模型，
// 避免把“扩展”变成修改已有权限请求的旁路。此函数不修改传入baseline。
export function mergeImplementationExtensions(baseline, extension) {
  if (
    !object(extension) ||
    Object.keys(extension).some(
      (k) => !['schemas', 'paths', 'sharing_management', 'list_pagination'].includes(k),
    ) ||
    (extension.sharing_management !== undefined && extension.sharing_management !== true) ||
    (extension.list_pagination !== undefined && extension.list_pagination !== true) ||
    !object(extension.schemas) ||
    !object(extension.paths)
  )
    throw Error('HTTP_IMPLEMENTATION_EXTENSION_INVALID');
  const api = structuredClone(baseline);
  // 固定的只读增量，不提供任意覆盖冻结请求的机制。
  if (extension.sharing_management) {
    for (const [path, values] of [
      ['/v1/documents/{documentId}/invitations', ['ACTIVE', 'INACTIVE', 'ALL']],
      ['/v1/documents/{documentId}/applications', ['PENDING', 'PROCESSED', 'ALL']],
      ['/v1/document-applications', ['PENDING', 'PROCESSED', 'ALL']],
    ]) {
      api.paths[path].get.parameters ??= [];
      api.paths[path].get.parameters.push({
        in: 'query',
        name: 'status',
        schema: { type: 'string', enum: values, default: 'ALL' },
      });
    }
    api.components.schemas.DocumentApplication.properties.applicant_display_name = {
      type: 'string',
      readOnly: true,
    };
    for (const name of ['DocumentInvitation', 'DocumentApplication'])
      api.components.schemas[name].properties.created_at = {
        type: 'string',
        format: 'date-time',
        readOnly: true,
      };
  }
  if (extension.list_pagination) {
    for (const path of [
      '/v1/organizations/{organizationId}/members',
      '/v1/knowledge-bases/{knowledgeBaseId}/members',
      '/v1/spaces/{spaceId}/knowledge-bases',
    ]) {
      api.paths[path].get.parameters ??= [];
      api.paths[path].get.parameters.push({
        in: 'query',
        name: 'q',
        schema: { type: 'string', maxLength: 100 },
      });
    }
    api.components.schemas.Organization.properties.current_member = {
      $ref: '#/components/schemas/OrganizationMembership',
      readOnly: true,
    };
    const parameters = api.paths['/v1/documents/{documentId}/comment-threads'].get.parameters;
    for (const [name, schema] of [
      ['context_kind', { type: 'string', enum: ['MAIN', 'BRANCH', 'REVIEW'] }],
      ['context_id', { type: 'string', pattern: '^[1-9][0-9]{0,19}$' }],
      ['content_version_id', { type: 'string', pattern: '^[1-9][0-9]{0,19}$' }],
    ])
      parameters.push({ in: 'query', name, schema });
  }
  const operations = new Set(
    Object.values(api.paths).flatMap((item) =>
      Object.values(item)
        .map((op) => op?.operationId)
        .filter(Boolean),
    ),
  );
  const shapes = new Set(Object.keys(api.paths).map(shape));
  for (const [name, schema] of Object.entries(extension.schemas)) {
    if (!models.has(name) || Object.hasOwn(api.components.schemas, name) || !object(schema))
      throw Error('HTTP_IMPLEMENTATION_EXTENSION_MODEL_INVALID');
    api.components.schemas[name] = structuredClone(schema);
  }
  function refs(value) {
    if (!value || typeof value !== 'object') return;
    if (
      '$ref' in value &&
      (typeof value.$ref !== 'string' ||
        !value.$ref.startsWith('#/components/schemas/') ||
        !Object.hasOwn(api.components.schemas, value.$ref.slice('#/components/schemas/'.length)))
    )
      throw Error('HTTP_IMPLEMENTATION_EXTENSION_REF_INVALID');
    for (const child of Object.values(value)) refs(child);
  }
  refs(extension.schemas);
  if (extension.list_pagination)
    refs(api.components.schemas.Organization.properties.current_member);
  for (const [path, item] of Object.entries(extension.paths)) {
    const methods = paths.get(path);
    if (!methods || !object(item) || Object.keys(item).length === 0 || shapes.has(shape(path)))
      throw Error('HTTP_IMPLEMENTATION_EXTENSION_PATH_INVALID');
    for (const [method, operation] of Object.entries(item)) {
      if (
        !methods.includes(method) ||
        !object(operation) ||
        operation['x-authority'] !== 'document' ||
        typeof operation.operationId !== 'string' ||
        !operation.operationId ||
        operations.has(operation.operationId) ||
        !object(operation.responses) ||
        (method === 'get' && operation.requestBody)
      )
        throw Error('HTTP_IMPLEMENTATION_EXTENSION_OPERATION_INVALID');
      refs(operation);
      operations.add(operation.operationId);
    }
    api.paths[path] = structuredClone(item);
    shapes.add(shape(path));
  }
  return api;
}
