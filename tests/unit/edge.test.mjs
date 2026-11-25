import test from 'node:test';
import assert from 'node:assert/strict';
import { routeFor, filterProxyHeaders } from '../../scripts/edge-routing.mjs';
test('organization API is routed to the account authority', () =>
  assert.equal(routeFor('GET', '/v1/organizations/17')?.authority, 'account'));
test('personal document invitations remain in document authority', () =>
  assert.equal(routeFor('POST', '/v1/document-invitations/applications')?.authority, 'document'));
test('an unrecognized internal endpoint is not exposed as API', () =>
  assert.equal(
    routeFor('POST', '/collaboration.v1.IdentityInternal/AcquireIdentityAdmission'),
    null,
  ));
test('browser identity claims are stripped while real cookie and origin remain', () => {
  const actual = filterProxyHeaders({
    'x-internal-actor': '99',
    'x-actor-id': '99',
    'x-service-role': 'account',
    host: 'evil.invalid',
    cookie: 'example-session=fixture',
    origin: 'https://localhost:20600',
  });
  assert.equal(actual['x-internal-actor'], undefined);
  assert.equal(actual['x-actor-id'], undefined);
  assert.equal(actual['x-service-role'], undefined);
  assert.equal(actual.host, undefined);
  assert.equal(actual.cookie, 'example-session=fixture');
  assert.equal(actual.origin, 'https://localhost:20600');
});
