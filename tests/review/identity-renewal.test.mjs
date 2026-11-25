import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import mysql from 'mysql2/promise';
import { root, runtime, state } from '../../scripts/runtime-lib.mjs';
import { createUser, operation, post, json } from '../helpers/account.mjs';
import { request } from '../helpers/https.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';

const api = grpc.loadPackageDefinition(
  loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), {
    longs: String,
    enums: String,
    defaults: true,
    keepCase: true,
  }),
).collaboration.v1;
const certificate = (role) => ({
  key: readFileSync(resolve(runtime, 'pki/' + role + '-key.pem')),
  cert: readFileSync(resolve(runtime, 'pki/' + role + '.pem')),
});

async function fixture(
  purpose = 'SUBMISSION',
  target = { document_id: '1', unit_id: '1', purpose },
) {
  const actor = await createUser(),
    c = certificate('document');
  const peer = new api.IdentityInternal(
    '127.0.0.1:' + state().ports.accountRpc,
    grpc.credentials.createSsl(readFileSync(resolve(runtime, 'pki/ca.pem')), c.key, c.cert),
    { 'grpc.ssl_target_name_override': 'localhost' },
  );
  const proof = await rpcCall(peer, 'AcquireIdentityAdmission', {
    operation_id: operation('persistent-submit'),
    session_token: Buffer.from(
      /__Host-collabSession=([^;]+)/.exec(actor.headers.cookie)[1],
      'base64url',
    ),
    purpose,
    target,
    required_identity_capability: 'ACTIVE_ACCOUNT',
    identity_target: { kind: 'USER', user_id: actor.id },
  });
  return {
    actor,
    peer,
    proof,
    query: {
      admission_id: proof.admission_id,
      operation_id: proof.operation_id,
      purpose: proof.purpose,
      target: proof.target,
      identity_target: proof.identity_target,
    },
  };
}
const renew = (query, role = 'document') =>
  request(state().ports.accountHttp, '/infra/identity/renew-operation', {
    method: 'POST',
    ...certificate(role),
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(query),
  });

test('expired ordinary admission stays invalid while actual SUBMISSION renewal issues a new current proof', async () => {
  const f = await fixture(),
    db = await mysql.createConnection(
      JSON.parse(readFileSync(resolve(runtime, 'services/account.json'))).database,
    );
  try {
    // 这里注入SQL期限边界；真正跨30秒的后台恢复另由送审流程测试覆盖。
    await db.query(
      'UPDATE identity_admission SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE admission_id=?',
      [f.proof.admission_id],
    );
    const sessionUntil = new Date(Date.now() + 15000);
    await db.query('UPDATE session SET expires_at=? WHERE id=?', [
      sessionUntil,
      Buffer.from(f.proof.session_ref),
    ]);
    await assert.rejects(
      () => rpcCall(f.peer, 'RevalidateIdentityAdmission', f.query),
      (e) => e.code === 7,
    );
    const response = await renew(f.query);
    assert.equal(response.status, 200);
    const current = json(response);
    assert.notEqual(current.admission_id, f.proof.admission_id);
    assert.equal(current.actor_id, f.actor.id);
    assert.equal(current.operation_id, f.proof.operation_id);
    assert.deepEqual(current.sources, f.proof.sources);
    assert.ok(Number(current.expires_unix_ms) > Date.now());
    assert.ok(Number(current.expires_unix_ms) <= sessionUntil.getTime());
    await assert.rejects(
      () => rpcCall(f.peer, 'RevalidateIdentityAdmission', f.query),
      (e) => e.code === 7,
    );
    await rpcCall(f.peer, 'RevalidateIdentityAdmission', {
      ...f.query,
      admission_id: current.admission_id,
    });
    assert.equal(
      (await renew({ ...f.query, target: { ...f.query.target, unit_id: '2' } })).status,
      403,
    );
    assert.equal((await renew(f.query, 'collaboration')).status, 403);
  } finally {
    f.peer.close();
    await db.end();
  }
});

test('renewal refuses ordinary read purposes and logout closes even an expired original proof', async () => {
  const read = await fixture('WORKBENCH_READ'),
    submit = await fixture();
  const db = await mysql.createConnection(
    JSON.parse(readFileSync(resolve(runtime, 'services/account.json'))).database,
  );
  try {
    assert.equal((await renew(read.query)).status, 403);
    await db.query(
      'UPDATE identity_admission SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE admission_id=?',
      [submit.proof.admission_id],
    );
    const response = await post(
      '/v1/account/logout',
      { operation_id: operation('logout') },
      submit.actor.headers,
    );
    assert.ok([200, 202].includes(response.status));
    assert.equal((await renew(submit.query)).status, 403);
  } finally {
    read.peer.close();
    submit.peer.close();
    await db.end();
  }
});

test('natural expiry permits original STRUCTURE_PREVIEW renewal without changing its whole-document binding', async () => {
  const f = await fixture('STRUCTURE_PREVIEW', {
    document_id: '1',
    structure_id: '2',
    workspace_epoch: '3',
    purpose: 'STRUCTURE_PREVIEW',
  });
  try {
    // 真正等待短准入自然失效，证明恢复没有把过期旧证明重新开放。
    await new Promise((resolve) =>
      setTimeout(resolve, Math.max(0, Number(f.proof.expires_unix_ms) - Date.now() + 100)),
    );
    await assert.rejects(
      () => rpcCall(f.peer, 'RevalidateIdentityAdmission', f.query),
      (e) => e.code === 7,
    );
    const response = await renew(f.query);
    assert.equal(response.status, 200, json(response).code);
    const current = json(response);
    assert.notEqual(current.admission_id, f.proof.admission_id);
    assert.equal(current.actor_id, f.proof.actor_id);
    assert.deepEqual({ unit_id: '0', branch_id: '0', ...current.target }, f.proof.target);
    assert.deepEqual(current.sources, f.proof.sources);
    assert.equal(current.session_ref, Buffer.from(f.proof.session_ref).toString('base64'));
    assert.ok(Number(current.expires_unix_ms) > Date.now());
    for (const changed of [
      { structure_id: '9' },
      { workspace_epoch: '4' },
      { unit_id: '1' },
      { branch_id: '1' },
    ]) {
      assert.equal(
        (await renew({ ...f.query, target: { ...f.query.target, ...changed } })).status,
        403,
      );
    }
    assert.equal((await renew(f.query, 'collaboration')).status, 403);
  } finally {
    f.peer.close();
  }
});

test('renewal never replaces an original identity epoch with later authority', async () => {
  const f = await fixture(),
    db = await mysql.createConnection(
      JSON.parse(readFileSync(resolve(runtime, 'services/account.json'))).database,
    );
  try {
    // 注入权威来源代次变化，核验新准入仍必须来自原代次，不能自动改绑后来权限。
    await db.query('UPDATE user SET identity_epoch=identity_epoch+1 WHERE id=?', [f.actor.id]);
    assert.equal((await renew(f.query)).status, 403);
  } finally {
    f.peer.close();
    await db.end();
  }
});
