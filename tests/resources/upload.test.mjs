import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync, readdirSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import https from 'node:https';
import { createUser, get, post, json, operation, env, origin } from '../helpers/account.mjs';
import { request } from '../helpers/https.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { runtime } from '../../scripts/runtime-lib.mjs';

const pause = (ms) => new Promise((done) => setTimeout(done, ms));

test('真实资源上传、幂等结算及在途过期清理互斥', { timeout: 60000 }, async () => {
  const user = await createUser(),
    stranger = await createUser();
  const spaces = await get('/v1/me/spaces', user.headers);
  assert.equal(spaces.status, 200);
  const space = json(spaces).items.find((item) => item.kind === 'PERSONAL').id;
  const created = await post(
    '/v1/documents',
    { operation_id: operation('resource-doc'), space_id: space, title: '资源上传验收' },
    user.headers,
  );
  assert.equal(created.status, 201);
  const document = json(created).result.entity_id;
  const quota = async () => {
    const result = await get(`/v1/spaces/${space}/quota`, user.headers);
    assert.equal(result.status, 200);
    return json(result);
  };
  const body = Buffer.alloc(3 * 1024 * 1024, 'a');
  const checksum = createHash('sha256').update(body).digest('hex');
  // 当前空间配额同时计正文和资源；新建空文档也有规范正文容量，不是假定零余额。
  // 以真实初始余额核精确增量，重复完成/过期退款都不能重复收费或退掉正文费用。
  const initialQuota = await quota();
  assert.equal(initialQuota.reserved_bytes, '0');
  assert.ok(BigInt(initialQuota.used_bytes) > 0n, '新建文档的正文容量应已计费');
  const completedUsed = String(BigInt(initialQuota.used_bytes) + BigInt(body.length));
  const begin = async () => {
    const input = {
      operation_id: operation('resource-upload'),
      expected_quota_revision: (await quota()).revision,
      byte_size: String(body.length),
      media_type: 'application/octet-stream',
      checksum,
    };
    const result = await post(`/v1/documents/${document}/resources`, input, user.headers);
    assert.equal(result.status, 201, result.body);
    return { input, handle: json(result) };
  };
  const { input, handle } = await begin();
  assert.deepEqual(
    json(await post(`/v1/documents/${document}/resources`, input, user.headers)),
    handle,
  );
  assert.equal((await quota()).reserved_bytes, String(body.length));
  assert.equal((await quota()).used_bytes, initialQuota.used_bytes);
  const uploadPath = new URL(handle.upload_url).pathname;
  const put = (bytes, headers = user.headers) =>
    request(env.ports.edge, uploadPath, {
      method: 'PUT',
      headers: { origin, 'content-type': 'application/octet-stream', ...headers },
      body: bytes,
      timeoutMs: 10000,
    });
  assert.equal((await put(body, stranger.headers)).status, 403);
  assert.equal((await put(body.subarray(0, 10))).status, 409);
  const bytes = await put(body);
  assert.equal(bytes.status, 200, bytes.body);
  assert.equal(json(bytes).state, 'UPLOADING');
  const repeatedBytes = await put(body);
  assert.equal(repeatedBytes.status, 200, repeatedBytes.body);
  assert.deepEqual(json(repeatedBytes), json(bytes));
  const complete = {
    operation_id: operation('resource-complete'),
    byte_size: String(body.length),
    checksum,
  };
  const ready = await post(`/v1/resources/${handle.resource_id}/complete`, complete, user.headers);
  assert.equal(ready.status, 200, ready.body);
  assert.equal(json(ready).state, 'READY');
  assert.deepEqual(
    json(await post(`/v1/resources/${handle.resource_id}/complete`, complete, user.headers)),
    json(ready),
  );
  assert.equal((await quota()).used_bytes, completedUsed);
  assert.equal((await quota()).reserved_bytes, '0');

  const pending = await begin();
  const db = await reviewDatabase();
  let slow;
  try {
    const [[row]] = await db.query(
      'SELECT storage_key FROM resource WHERE id=? AND upload_operation_id=?',
      [pending.handle.resource_id, pending.input.operation_id],
    );
    assert.match(row.storage_key, /^[a-f0-9]{64}$/);
    const folder = resolve(runtime, 'resources/document');
    slow = https.request({
      hostname: 'localhost',
      port: env.ports.edge,
      path: new URL(pending.handle.upload_url).pathname,
      method: 'PUT',
      ca: readFileSync(resolve(runtime, 'pki/ca.pem')),
      headers: {
        origin,
        'content-type': 'application/octet-stream',
        'content-length': body.length,
        ...user.headers,
      },
    });
    slow.on('error', () => {});
    slow.on('response', (response) => response.resume());
    slow.write(body.subarray(0, 16));
    const deadline = Date.now() + 5000;
    while (
      !readdirSync(folder).some((name) => name.startsWith('.upload-' + row.storage_key + '-'))
    ) {
      assert.ok(Date.now() < deadline, 'UPLOAD_DID_NOT_START');
      await pause(50);
    }
    // 只推进本测试原号的到期时间，真实worker仍必须等待实际写入句柄退出。
    await db.query(
      "UPDATE quota_reservation SET expires_at=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE operation_id=? AND state='RESERVED'",
      [pending.input.operation_id],
    );
    await pause(5500);
    const [[inflight]] = await db.query(
      'SELECT r.state resource_state,q.state quota_state FROM resource r JOIN quota_reservation q ON q.operation_id=r.upload_operation_id WHERE r.id=?',
      [pending.handle.resource_id],
    );
    assert.deepEqual({ ...inflight }, { resource_state: 'UPLOADING', quota_state: 'RESERVED' });
    slow.destroy();
    const cleanupDeadline = Date.now() + 15000;
    for (;;) {
      const [[closed]] = await db.query(
        'SELECT r.state resource_state,q.state quota_state FROM resource r JOIN quota_reservation q ON q.operation_id=r.upload_operation_id WHERE r.id=?',
        [pending.handle.resource_id],
      );
      if (closed.resource_state === 'DELETED') {
        assert.equal(closed.quota_state, 'RELEASED');
        break;
      }
      assert.ok(Date.now() < cleanupDeadline, 'EXPIRED_UPLOAD_NOT_RECOVERED');
      await pause(200);
    }
    assert.equal(existsSync(resolve(folder, row.storage_key)), false);
    assert.equal(
      readdirSync(folder).some((name) => name.startsWith('.upload-' + row.storage_key + '-')),
      false,
    );
    assert.equal((await quota()).reserved_bytes, '0');
    assert.equal((await quota()).used_bytes, completedUsed);
    const late = await request(env.ports.edge, new URL(pending.handle.upload_url).pathname, {
      method: 'PUT',
      headers: { origin, 'content-type': 'application/octet-stream', ...user.headers },
      body,
      timeoutMs: 10000,
    });
    assert.equal(late.status, 409);
  } finally {
    slow?.destroy();
    await db.end();
  }
});
