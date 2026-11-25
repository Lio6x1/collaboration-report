import test from 'node:test';
import assert from 'node:assert/strict';
import { authorizeCall } from '../src/transport/rpc_authorization.mjs';
const auth = (role) => ({
  transportSecurityType: 'ssl',
  sslPeerCertificate: {
    subjectaltname: `DNS:localhost, URI:spiffe://collaboration-report/${role}`,
  },
});
test('Node RPC rejects a caller without TLS identity', () =>
  assert.equal(authorizeCall({}, '/grpc.health.v1.Health/Check', Date.now() + 1000)?.code, 16));
test('Node RPC rejects a missing deadline', () =>
  assert.equal(authorizeCall(auth('document'), '/grpc.health.v1.Health/Check', Infinity)?.code, 3));
test('Node RPC rejects a deadline outside its resource budget', () =>
  assert.equal(
    authorizeCall(auth('document'), '/grpc.health.v1.Health/Check', Date.now() + 60000)?.code,
    3,
  ));
test('Node RPC denies a trusted but unauthorized role', () =>
  assert.equal(
    authorizeCall(
      auth('account'),
      '/collaboration.v1.CollaborationInternal/CaptureSubmission',
      Date.now() + 1000,
    )?.code,
    7,
  ));
test('Node RPC ignores a known certificate common name without URI identity', () =>
  assert.equal(
    authorizeCall(
      { transportSecurityType: 'ssl', sslPeerCertificate: { subject: { CN: 'document' } } },
      '/grpc.health.v1.Health/Check',
      Date.now() + 1000,
    )?.code,
    16,
  ));
test('Node RPC accepts an authorized bounded service call', () =>
  assert.equal(
    authorizeCall(
      auth('document'),
      '/collaboration.v1.CollaborationInternal/CaptureSubmission',
      Date.now() + 1000,
    ),
    null,
  ));
