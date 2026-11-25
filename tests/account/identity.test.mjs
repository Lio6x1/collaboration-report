import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomUUID, createHash } from 'node:crypto';
import mysql from 'mysql2/promise';
import { createUser } from '../helpers/account.mjs';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { root, runtime, state } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';
const env = state(),
  seed = await createUser(),
  origin = 'https://localhost:' + env.ports.edge;
const api = grpc.loadPackageDefinition(
  loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), {
    longs: String,
    enums: String,
    defaults: true,
    keepCase: true,
  }),
).collaboration.v1;
const client = () =>
  new api.IdentityInternal(
    '127.0.0.1:' + env.ports.accountRpc,
    grpc.credentials.createSsl(
      readFileSync(resolve(runtime, 'pki/ca.pem')),
      readFileSync(resolve(runtime, 'pki/document-key.pem')),
      readFileSync(resolve(runtime, 'pki/document.pem')),
    ),
    { 'grpc.ssl_target_name_override': 'localhost' },
  );
const call = (c, method, input) =>
  new Promise((done) =>
    c[method](input, { deadline: Date.now() + 2000 }, (err, value) => done({ err, value })),
  );
test('identity admission cannot outlive the actual session expiry', async () => {
  const u = await createUser(),
    token = Buffer.from(/__Host-collabSession=([^;]+)/.exec(u.headers.cookie)[1], 'base64url'),
    hash = createHash('sha256').update(token).digest();
  const cfg = JSON.parse(readFileSync(resolve(runtime, 'services/account.json'))),
    db = await mysql.createConnection(cfg.database),
    until = new Date(Date.now() + 8000),
    c = client();
  try {
    await db.query('UPDATE session SET expires_at=? WHERE id=?', [until, hash]);
    const r = await call(c, 'AcquireIdentityAdmission', {
      operation_id: 'expiry-' + randomUUID(),
      session_token: token,
      purpose: 'WORKBENCH_READ',
      target: { purpose: 'WORKBENCH_READ' },
      required_identity_capability: 'ACTIVE_ACCOUNT',
      identity_target: { kind: 'USER', user_id: u.id },
    });
    assert.equal(r.err, null);
    assert.ok(Number(r.value.expires_unix_ms) <= until.getTime());
  } finally {
    c.close();
    await db.end();
  }
});
test('real identity admission binds session, operation, target and purpose then closes on logout', async () => {
  const response = await request(env.ports.edge, '/v1/account/login', {
    method: 'POST',
    headers: { origin, 'content-type': 'application/json' },
    body: JSON.stringify({ email: seed.email, password: seed.password }),
  });
  assert.equal(response.status, 200);
  const token = response.headers['set-cookie']
      .find((x) => x.startsWith('__Host-collabSession='))
      .split(';')[0]
      .split('=')[1],
    value = JSON.parse(response.body),
    cookie = response.headers['set-cookie'].map((x) => x.split(';')[0]).join('; ');
  const input = {
    operation_id: 'identity-' + randomUUID(),
    session_token: Buffer.from(token, 'base64url'),
    purpose: 'WORKBENCH_READ',
    target: { purpose: 'WORKBENCH_READ' },
    required_identity_capability: 'ACTIVE_ACCOUNT',
    identity_target: { kind: 'USER', user_id: value.user_id },
  };
  const c = client();
  try {
    const first = await call(c, 'AcquireIdentityAdmission', input);
    assert.equal(first.err, null);
    assert.equal(first.value.actor_id, value.user_id);
    assert.equal(first.value.sources.length, 3);
    const proof = {
      admission_id: first.value.admission_id,
      operation_id: input.operation_id,
      purpose: input.purpose,
      target: input.target,
      identity_target: input.identity_target,
    };
    assert.equal((await call(c, 'RevalidateIdentityAdmission', proof)).err, null);
    assert.equal(
      (await call(c, 'RevalidateIdentityAdmission', { ...proof, purpose: 'APPOINT_KB_ADMIN' })).err
        ?.code,
      grpc.status.PERMISSION_DENIED,
    );
    const logout = await request(env.ports.edge, '/v1/account/logout', {
      method: 'POST',
      headers: {
        origin,
        'content-type': 'application/json',
        cookie,
        'x-csrf-token': value.csrf_token,
      },
      body: JSON.stringify({ operation_id: 'logout-' + randomUUID() }),
    });
    assert.equal(logout.status, 202);
    assert.equal(
      (await call(c, 'RevalidateIdentityAdmission', proof)).err?.code,
      grpc.status.PERMISSION_DENIED,
    );
  } finally {
    c.close();
  }
});
