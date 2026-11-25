import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root } from './runtime-lib.mjs';

// 只生成未被冻结接口覆盖的接收协议。原OpenAPI/BASELINE字节不变，普通组织直接转交
// 仍沿原接口；生成DTO和响应校验必须共同读取这一版本化实施扩展。
const baseline = JSON.parse(readFileSync(resolve(root, 'contracts/http/openapi.json'), 'utf8'));
const ref = (name) => ({ $ref: '#/components/schemas/' + name });
const id = { type: 'string', pattern: '^[1-9][0-9]{0,19}$' };
const revision = { type: 'string', pattern: '^(0|[1-9][0-9]{0,19})$' };
const operation = baseline.components.schemas.VersionedAction.properties.operation_id;
const model = (properties) => ({
  type: 'object',
  properties,
  required: Object.keys(properties),
  additionalProperties: false,
});
const metadata = model({
  id,
  document_id: id,
  document_title: { type: 'string' },
  source_user_id: id,
  target_user_id: id,
  source_space_id: id,
  expected_control_revision: revision,
  revision,
  state: {
    type: 'string',
    enum: ['PENDING', 'ACCEPTING', 'ACCEPTED', 'CANCELLED', 'REJECTED', 'EXPIRED'],
  },
  expires_at: { type: 'string', format: 'date-time' },
});
metadata.properties.target_space_id = id;
metadata.properties.target_knowledge_base_id = id;
// 刷新后ACCEPTING仍可查询已登记原操作，不能通过再次生成新accept号“恢复”。
metadata.properties.accept_operation_id = operation;
const schemas = {
  OrganizationMembership: model({
    user_id: id,
    role: { type: 'string', enum: ['MEMBER', 'ADMIN'] },
    state: { type: 'string', enum: ['ACTIVE', 'REVOKING', 'CLOSED'] },
    revision,
  }),
  PersonalTransferMetadata: metadata,
  PersonalTransferPage: model({
    items: { type: 'array', items: ref('PersonalTransferMetadata') },
    next_cursor: { type: 'string', nullable: true },
  }),
  PersonalTransferAccept: model({
    operation_id: operation,
    expected_revision: revision,
    target_knowledge_base_id: id,
    expected_quota_revision: revision,
  }),
};
const paths = {};
function add(path, method, name, output, input) {
  const params = [...path.matchAll(/\{(\w+)\}/g)].map((m) => ({
    in: 'path',
    name: m[1],
    required: true,
    schema: id,
  }));
  const responses = {};
  for (const status of [400, 401, 403, 404, 409, 422, 429, 503])
    responses[status] = {
      description: 'Business error; unknown results must recover the original operation.',
      content: { 'application/json': { schema: ref('Error') } },
    };
  responses[input ? '202' : '200'] = {
    description: input
      ? 'Original operation registered; follow poll_url until terminal.'
      : 'Current authorized metadata',
    content: { 'application/json': { schema: ref(output) } },
  };
  const value = {
    operationId: name,
    summary: 'PERSONAL_DOCUMENT_TRANSFER',
    description:
      'Only the original personal owner may propose; only the named receiver may accept into their current personal knowledge base with sufficient current quota. Proposal success does not move ownership.',
    tags: ['personal-transfers'],
    'x-authority': 'document',
    'x-required-capability': input === 'OwnerChange' ? 'MANAGE_DOCUMENT' : 'ACTIVE_ACCOUNT',
    parameters: params,
    security: [{ sessionCookie: [] }],
    responses,
  };
  if (input)
    value.requestBody = { required: true, content: { 'application/json': { schema: ref(input) } } };
  paths[path] ??= {};
  paths[path][method] = value;
}
add(
  '/v1/documents/{documentId}/personal-transfers',
  'post',
  'proposePersonalTransfer',
  'Operation',
  'OwnerChange',
);
add(
  '/v1/personal-transfers/{transferId}',
  'get',
  'getPersonalTransfer',
  'PersonalTransferMetadata',
);
add('/v1/personal-transfers', 'get', 'listPersonalTransfers', 'PersonalTransferPage');
paths['/v1/personal-transfers'].get.parameters.push(
  {
    in: 'query',
    name: 'direction',
    required: true,
    schema: { type: 'string', enum: ['incoming', 'outgoing'] },
  },
  {
    in: 'query',
    name: 'limit',
    required: false,
    schema: { type: 'integer', minimum: 1, maximum: 100 },
  },
  { in: 'query', name: 'cursor', required: false, schema: { type: 'string' } },
);
add(
  '/v1/personal-transfers/{transferId}/accept',
  'post',
  'acceptPersonalTransfer',
  'Operation',
  'PersonalTransferAccept',
);
add(
  '/v1/personal-transfers/{transferId}/cancel',
  'post',
  'cancelPersonalTransfer',
  'Operation',
  'VersionedAction',
);
add(
  '/v1/personal-transfers/{transferId}/reject',
  'post',
  'rejectPersonalTransfer',
  'Operation',
  'VersionedAction',
);
writeFileSync(
  resolve(root, 'contracts/http/implementation-extensions.json'),
  JSON.stringify(
    {
      schemas,
      paths: {
        ...paths,
        ...Object.fromEntries(
          [
            ['invitationId', 'document-invitations', 'archiveDocumentInvitation'],
            ['applicationId', 'document-applications', 'archiveDocumentApplication'],
          ].map(([parameter, resource, operationId]) => {
            const post = structuredClone(
              baseline.paths['/v1/document-invitations/{invitationId}/cancel'].post,
            );
            post.operationId = operationId;
            post.summary = '归档失效邀请或已处理申请，保留授权和审批记录';
            post.parameters = post.parameters.map((p) =>
              p.in === 'path' ? { ...p, name: parameter } : p,
            );
            return ['/v1/' + resource + '/{' + parameter + '}/archive', { post }];
          }),
        ),
      },
      sharing_management: true,
      list_pagination: true,
    },
    null,
    2,
  ) + '\n',
);
