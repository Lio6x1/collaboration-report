import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import mysql from 'mysql2/promise';
import { createUser, get, json, env } from '../helpers/account.mjs';
import { request } from '../helpers/https.mjs';
import { runtime } from '../../scripts/runtime-lib.mjs';
// 后端直连必须与Doc一样拒绝未受控敏感输出；业务入口才拥有可排空的真实浏览器handler。
test('account sensitive responses cannot bypass the controlled edge by calling its private HTTP port', async () => {
  const u = await createUser(),
    direct = await request(env.ports.accountHttp, '/v1/account/me', { headers: u.headers });
  assert.equal(direct.status, 503);
  assert.ok(!direct.body.includes(u.email));
  const visible = await get('/v1/account/me', u.headers);
  assert.equal(visible.status, 200);
  assert.equal(json(visible).id, u.id);
});
test('real account me output persists its exact session credential and identity dependencies', async () => {
  const u = await createUser();
  assert.equal((await get('/v1/account/me', u.headers)).status, 200);
  const db = await mysql.createConnection(
    JSON.parse(readFileSync(resolve(runtime, 'services/document.json'))).database,
  );
  try {
    const [rows] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='OUTPUT_LEASE' ORDER BY id DESC LIMIT 20",
    );
    const lease = rows
      .map((r) => (typeof r.payload === 'string' ? JSON.parse(r.payload) : r.payload))
      .find(
        (l) =>
          l.authority === 'account' && l.sources.some((s) => s.scope_key === 'identity:' + u.id),
      );
    assert.ok(lease, '账号输出必须登记实际来源');
    assert.ok(lease.sources.some((s) => s.scope_key === 'credential:' + u.id));
    assert.ok(lease.sources.some((s) => s.scope_key.startsWith('session:')));
  } finally {
    await db.end();
  }
});

// 目录页会并发拉组织和自己的申请。相同来源不能先取S锁再互相升级到X锁。
test('parallel account outputs acquire an existing source gate without a shared-lock upgrade deadlock', async () => {
  const u = await createUser();
  const statuses = [];
  for (let n = 0; n < 8; n++)
    statuses.push(
      ...(await Promise.all(Array.from({ length: 4 }, () => get('/v1/account/me', u.headers)))).map(
        (r) => r.status,
      ),
    );
  assert.deepEqual(new Set(statuses), new Set([200]));
});
