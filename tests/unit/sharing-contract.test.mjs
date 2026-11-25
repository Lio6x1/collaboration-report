import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { mergeImplementationExtensions } from '../../scripts/http-implementation-extensions.mjs';

const baseline = JSON.parse(
  readFileSync(new URL('../../contracts/http/openapi.json', import.meta.url)),
);
test('sharing extension adds only scoped archive operations and read projections', () => {
  const path = '/v1/document-invitations/{invitationId}/archive';
  const post = structuredClone(
    baseline.paths['/v1/document-invitations/{invitationId}/cancel'].post,
  );
  post.operationId = 'archiveDocumentInvitation';
  const extension = { schemas: {}, paths: { [path]: { post } }, sharing_management: true };
  const result = mergeImplementationExtensions(baseline, extension);
  assert.equal(result.paths[path].post.operationId, 'archiveDocumentInvitation');
  assert.deepEqual(
    result.paths['/v1/documents/{documentId}/invitations'].get.parameters.find(
      (p) => p.name === 'status',
    ).schema.enum,
    ['ACTIVE', 'INACTIVE', 'ALL'],
  );
  assert.deepEqual(
    result.paths['/v1/documents/{documentId}/applications'].get.parameters.find(
      (p) => p.name === 'status',
    ).schema.enum,
    ['PENDING', 'PROCESSED', 'ALL'],
  );
  assert.equal(
    result.components.schemas.DocumentApplication.properties.applicant_display_name.readOnly,
    true,
  );
  assert.equal(
    baseline.components.schemas.DocumentApplication.properties.applicant_display_name,
    undefined,
  );
  assert.throws(() =>
    mergeImplementationExtensions(baseline, { ...extension, sharing_management: 'yes' }),
  );
  assert.throws(() =>
    mergeImplementationExtensions(baseline, {
      ...extension,
      paths: { '/v1/users/{userId}/archive': { post } },
    }),
  );
});

test('pagination read projections cannot reference an absent membership model', () => {
  assert.throws(
    () =>
      mergeImplementationExtensions(baseline, { schemas: {}, paths: {}, list_pagination: true }),
    /HTTP_IMPLEMENTATION_EXTENSION_REF_INVALID/,
  );
  const extension = JSON.parse(
    readFileSync(new URL('../../contracts/http/implementation-extensions.json', import.meta.url)),
  );
  const api = mergeImplementationExtensions(baseline, extension);
  assert.equal(
    api.components.schemas.Organization.properties.current_member.$ref,
    '#/components/schemas/OrganizationMembership',
  );
  assert.equal(baseline.components.schemas.Organization.properties.current_member, undefined);
  assert.equal(
    api.paths['/v1/documents/{documentId}/comment-threads'].get.parameters.find(
      (p) => p.name === 'context_kind',
    ).in,
    'query',
  );
  assert.deepEqual(
    api.paths['/v1/organizations/{organizationId}/members'].get.responses,
    baseline.paths['/v1/organizations/{organizationId}/members'].get.responses,
  );
});
