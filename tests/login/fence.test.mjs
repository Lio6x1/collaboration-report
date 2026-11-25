import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomBytes, randomUUID, createHash } from 'node:crypto';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import mysql from 'mysql2/promise';
import { root, runtime, state } from '../../scripts/runtime-lib.mjs';
import { bootstrapFence, RPCFailure } from '../../services/collaboration/src/fence.mjs';
import { createStore } from '../../services/collaboration/src/bootstrap/store.mjs';
const env = state(),
  api = grpc.loadPackageDefinition(
    loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), {
      longs: String,
      enums: String,
      defaults: true,
      keepCase: true,
    }),
  ).collaboration.v1;
function client(role, target) {
  return new api[target === 'document' ? 'DocumentInternal' : 'CollaborationInternal'](
    'localhost:' + env.ports[target + 'Rpc'],
    grpc.credentials.createSsl(
      readFileSync(resolve(runtime, 'pki/ca.pem')),
      readFileSync(resolve(runtime, 'pki', role + '-key.pem')),
      readFileSync(resolve(runtime, 'pki', role + '.pem')),
    ),
  );
}
function request() {
  const hash = randomBytes(32).toString('hex');
  return {
    revocation_id: 'logout-' + randomUUID(),
    closed_scopes: [{ scope_key: 'session:' + hash, epoch: '1' }],
    generation: '1',
    target_digest: createHash('sha256')
      .update('SESSION_LOGOUT\n' + hash)
      .digest(),
  };
}
function call(c, body) {
  return new Promise((done) =>
    c.FenceAccess(body, { deadline: Date.now() + 2500 }, (err, value) => done({ err, value })),
  );
}
async function db(role) {
  const cfg = JSON.parse(readFileSync(resolve(runtime, 'services', role + '.json')));
  return mysql.createConnection(cfg.database);
}
test('fixed account to document to Node closure stores both real tombstones and ACKs', async () => {
  const req = request(),
    c = client('account', 'document');
  try {
    const first = await call(c, req);
    assert.equal(first.err, null);
    assert.equal(first.value.state, 'ACKNOWLEDGED');
    const repeat = await call(c, req);
    assert.equal(repeat.err, null);
    assert.deepEqual(repeat.value.result_digest, first.value.result_digest);
    for (const role of ['document', 'collaboration']) {
      const conn = await db(role);
      try {
        const [[row]] = await conn.query(
          'SELECT state,max_epoch FROM access_gate WHERE scope_key=?',
          [req.closed_scopes[0].scope_key],
        );
        assert.equal(row.state, 'BLOCKED');
        assert.equal(String(row.max_epoch), '1');
        const [[change]] = await conn.query(
          'SELECT state FROM authorization_change WHERE revocation_id=?',
          [req.revocation_id],
        );
        assert.equal(change.state, 'SUCCEEDED');
      } finally {
        await conn.end();
      }
    }
    const wrong = request();
    wrong.revocation_id = req.revocation_id;
    assert.equal((await call(c, wrong)).err?.code, grpc.status.ALREADY_EXISTS);
  } finally {
    c.close();
  }
});
test('the bootstrap fallback never ACKs an existing room without the full runtime drainer', async () => {
  const conn = await db('collaboration'),
    store = createStore(JSON.parse(readFileSync(resolve(runtime, 'services/collaboration.json'))));
  let id;
  try {
    const [row] = await conn.query(
      "INSERT INTO room(document_id,unit_id,workspace_kind,workspace_id,state) VALUES(999999,999999,'MAIN',999999,'OPEN')",
    );
    id = row.insertId;
    // 第三批RPC已接入真实房间排空器；这个反例直接调用仍保留的底座退路。
    // 它必须拒绝非空业务事实，不能借运行中的完整排空器返回ACK。
    await assert.rejects(bootstrapFence(store, request()), (error) => {
      assert.ok(error instanceof RPCFailure);
      assert.equal(error.code, grpc.status.FAILED_PRECONDITION);
      assert.equal(error.details, 'FULL_OUTPUT_DRAIN_REQUIRED');
      return true;
    });
  } finally {
    if (id) await conn.query('DELETE FROM room WHERE id=?', [id]);
    await conn.end();
    await store.pool.end();
  }
});
test('a reordered old closure never blocks a newer authority-validated gate', async () => {
  const source = 'credential:' + String(1000000 + Math.floor(Math.random() * 1000000)),
    req = {
      revocation_id: 'reordered-' + randomUUID(),
      closed_scopes: [{ scope_key: source, epoch: '7' }],
      generation: '1',
      target_digest: createHash('sha256')
        .update('AUTHORIZATION_CHANGE\n' + source + '\n7')
        .digest(),
    },
    c = client('account', 'document');
  const connections = [];
  try {
    for (const role of ['document', 'collaboration']) {
      const conn = await db(role);
      connections.push(conn);
      await conn.query("INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,8,'OPEN')", [
        source,
      ]);
    }
    const result = await call(c, req);
    assert.equal(result.err?.code ?? 0, 0);
    for (const conn of connections) {
      const [[row]] = await conn.query(
        'SELECT state,max_epoch FROM access_gate WHERE scope_key=?',
        [source],
      );
      assert.equal(String(row.max_epoch), '8');
      assert.equal(row.state, 'OPEN');
    }
  } finally {
    for (const conn of connections) {
      await conn.query('DELETE FROM authorization_ack WHERE revocation_id=?', [req.revocation_id]);
      await conn.query('DELETE FROM authorization_change WHERE revocation_id=?', [
        req.revocation_id,
      ]);
      await conn.query('DELETE FROM access_gate WHERE scope_key=?', [source]);
      await conn.end();
    }
    c.close();
  }
});
