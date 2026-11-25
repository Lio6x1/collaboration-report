import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { root, runtime, state } from '../../scripts/runtime-lib.mjs';
const env = state(),
  ca = readFileSync(resolve(runtime, 'pki/ca.pem'));
const options = { longs: String, enums: String, defaults: true, keepCase: true };
const api = grpc.loadPackageDefinition(
  loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), options),
).collaboration.v1;
const Health = grpc.loadPackageDefinition(
  loader.loadSync(resolve(root, 'contracts/proto/health.proto'), options),
).grpc.health.v1.Health;
function client(role, target, Constructor = Health) {
  const cred = role
    ? grpc.credentials.createSsl(
        ca,
        readFileSync(resolve(runtime, 'pki', role + '-key.pem')),
        readFileSync(resolve(runtime, 'pki', role + '.pem')),
      )
    : grpc.credentials.createSsl(ca);
  return new Constructor('localhost:' + env.ports[target + 'Rpc'], cred, {
    'grpc.initial_reconnect_backoff_ms': 100,
    'grpc.max_reconnect_backoff_ms': 200,
  });
}
function call(c, method, request = {}, deadline = Date.now() + 2000) {
  return new Promise((done) => {
    c[method](request, deadline === null ? {} : { deadline }, (err, value) => done({ err, value }));
  });
}
for (const target of ['account', 'document', 'search', 'collaboration']) {
  test(`${target} RPC requires client certificate`, async () => {
    const c = client(null, target);
    try {
      const r = await call(c, 'Check');
      assert.equal(r.err?.code, grpc.status.UNAVAILABLE);
    } finally {
      c.close();
    }
  });
  test(`${target} RPC rejects a CA-trusted unknown URI identity`, async () => {
    const c = client('edge', target);
    try {
      const r = await call(c, 'Check');
      assert.equal(r.err?.code, grpc.status.UNAUTHENTICATED);
    } finally {
      c.close();
    }
  });
  test(`${target} RPC rejects missing and excessive deadlines`, async () => {
    const c = client('document', target);
    try {
      assert.equal((await call(c, 'Check', {}, null)).err?.code, grpc.status.INVALID_ARGUMENT);
      assert.equal(
        (await call(c, 'Check', {}, Date.now() + 20000)).err?.code,
        grpc.status.INVALID_ARGUMENT,
      );
    } finally {
      c.close();
    }
  });
  test(`${target} RPC accepts verified identity and bounded health request`, async () => {
    const c = client('document', target);
    try {
      const r = await call(c, 'Check');
      assert.equal(r.err, null);
      assert.equal(r.value.status, 'SERVING');
    } finally {
      c.close();
    }
  });
}
test('Node RPC enforces method permission before its handler', async () => {
  const c = client('account', 'collaboration', api.CollaborationInternal);
  try {
    assert.equal((await call(c, 'CaptureSubmission')).err?.code, grpc.status.PERMISSION_DENIED);
  } finally {
    c.close();
  }
});
test('authorized implemented capture rejects a missing fixed binding', async () => {
  const c = client('document', 'collaboration', api.CollaborationInternal);
  try {
    // 固定送审已实现：空输入应由业务绑定规则拒绝，不能仍要求它是底座阶段的UNIMPLEMENTED。
    // 上一项继续独立证明错误调用身份先于处理器拒绝。
    const result = await call(c, 'CaptureSubmission');
    assert.equal(result.err?.code, grpc.status.INVALID_ARGUMENT);
    assert.equal(result.err?.details, 'CAPTURE_INVALID');
    assert.equal(result.value, undefined);
  } finally {
    c.close();
  }
});
