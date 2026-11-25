import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import mysql from 'mysql2/promise';
import { runtime } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';
const db = () =>
  mysql.createConnection(
    JSON.parse(readFileSync(resolve(runtime, 'services/document.json'))).database,
  );
async function memberRevision(f, u) {
  return json(
    await get('/v1/organizations/' + f.organization + '/members', f.owner.headers),
  ).items.find((m) => m.user_id === u.id).revision;
}
async function pausedQuery(observer, fragment) {
  for (let i = 0; i < 100; i++) {
    const [rows] = await observer.query('SHOW FULL PROCESSLIST');
    if (rows.some((r) => r.Info?.includes(fragment))) return;
    await new Promise((r) => setTimeout(r, 10));
  }
  assert.fail('DOC_WRITE_NOT_PAUSED');
}
// 在最后一次身份RPC之后卡住Doc写入，专门覆盖“多查一次仍不是跨库原子性”的窗口。
test('an ADMIN appointment and organization departure serialize through the same durable member gate', async () => {
  const f = await organizationFixture(1),
    u = f.members[0];
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/roles',
      {
        operation_id: operation('member'),
        expected_acl_revision: '1',
        user_id: u.id,
        role: 'MEMBER',
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const revision = await memberRevision(f, u),
    lock = await db(),
    observer = await db();
  let assigning, leaving;
  try {
    await lock.beginTransaction();
    await lock.query(
      'SELECT user_id FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE',
      [f.kb, u.id],
    );
    assigning = post(
      '/v1/knowledge-bases/' + f.kb + '/roles',
      {
        operation_id: operation('admin-race'),
        expected_acl_revision: '2',
        user_id: u.id,
        role: 'ADMIN',
      },
      f.owner.headers,
    );
    await pausedQuery(observer, 'INSERT INTO knowledge_base_member');
    leaving = post(
      '/v1/organizations/' + f.organization + '/leave',
      { operation_id: operation('leave-race'), expected_revision: revision },
      u.headers,
    );
    await new Promise((r) => setTimeout(r, 150));
    await lock.rollback();
    const [assigned, left] = await Promise.all([assigning, leaving]);
    assert.equal(assigned.status, 202);
    await waitAuthorization(assigned, f.owner);
    assert.equal(left.status, 409);
    assert.equal(json(left).code, 'ADMIN_RETIREMENT_REQUIRED');
  } finally {
    await lock.rollback();
    await Promise.allSettled([assigning, leaving]);
    await lock.end();
    await observer.end();
  }
});
test('knowledge base creation cannot commit an administrator after its creator leaves', async () => {
  const f = await organizationFixture(1),
    u = f.members[0],
    org = json(await get('/v1/organizations/' + f.organization, f.owner.headers));
  assert.equal(
    (
      await post(
        '/v1/organizations/' + f.organization + '/creation-grants',
        {
          operation_id: operation('grant'),
          expected_revision: org.member_revision,
          user_id: u.id,
          allowed: true,
        },
        f.owner.headers,
      )
    ).status,
    200,
  );
  const revision = await memberRevision(f, u),
    lock = await db(),
    observer = await db();
  let creating, leaving;
  try {
    await lock.beginTransaction();
    await lock.query('SELECT id FROM space WHERE id=? FOR UPDATE', [f.space]);
    creating = post(
      '/v1/spaces/' + f.space + '/knowledge-bases',
      { operation_id: operation('kb-race'), name: '并发建库' },
      u.headers,
    );
    // 前置资格查询也读取space；只有事务中的FOR UPDATE才证明member gate已持有。
    // 精确绑定当前空间，避免把另一请求或未加锁的快照误当成指定竞争窗口。
    await pausedQuery(observer, 'FROM space WHERE id=' + f.space + ' FOR UPDATE');
    leaving = post(
      '/v1/organizations/' + f.organization + '/leave',
      { operation_id: operation('leave-kb-race'), expected_revision: revision },
      u.headers,
    );
    await new Promise((r) => setTimeout(r, 150));
    await lock.rollback();
    const [made, left] = await Promise.all([creating, leaving]);
    assert.equal(made.status, 201);
    assert.equal(left.status, 409);
    assert.equal(json(left).code, 'ADMIN_RETIREMENT_REQUIRED');
  } finally {
    await lock.rollback();
    await Promise.allSettled([creating, leaving]);
    await lock.end();
    await observer.end();
  }
});
test('leaving then rejoining does not resurrect library grants or document responsibility', async () => {
  const f = await organizationFixture(1),
    u = f.members[0];
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/creation-grants',
      {
        operation_id: operation('create-grant'),
        expected_revision: '1',
        user_id: u.id,
        allowed: true,
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const d = await documentFixture(f, u),
    shared = await documentFixture(f);
  await waitAuthorization(
    await post(
      '/v1/documents/' + shared.id + '/authorization-changes',
      {
        operation_id: operation('old-edit'),
        expected_acl_revision: '1',
        changes: [
          {
            grant: {
              user_id: u.id,
              scope_kind: 'UNIT',
              scope_id: shared.main.units[0].unit_id,
              capability: 'EDIT',
            },
            action: 'GRANT',
          },
        ],
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const left = await post(
    '/v1/organizations/' + f.organization + '/leave',
    { operation_id: operation('leave'), expected_revision: await memberRevision(f, u) },
    u.headers,
  );
  assert.equal(left.status, 202);
  await waitOperation(left, u);
  const applied = await post(
    '/v1/organizations/' + f.organization + '/applications',
    { operation_id: operation('rejoin'), reason: '重新申请' },
    u.headers,
  );
  assert.equal(applied.status, 200);
  assert.equal(
    (
      await post(
        '/v1/applications/' + json(applied).result.entity_id + '/decisions',
        {
          operation_id: operation('readmit'),
          expected_revision: '1',
          decision: 'ACCEPT',
          reason: '通过',
        },
        f.owner.headers,
      )
    ).status,
    200,
  );
  const made = await post(
    '/v1/documents',
    {
      operation_id: operation('must-not-create'),
      space_id: f.space,
      knowledge_base_id: f.kb,
      title: '必须重新授权',
    },
    u.headers,
  );
  assert.equal(made.status, 403);
  assert.equal((await get('/v1/documents/' + d.id, u.headers)).status, 403);
  assert.equal(
    (await get('/v1/documents/' + shared.id + '/editable-units', u.headers)).status,
    403,
  );
  const current = await get('/v1/documents/' + d.id, f.owner.headers);
  assert.equal(current.status, 200);
  assert.equal(json(current).responsible_user_id, undefined);
});

test('organization role downgrade preserves independent document creation and edit grants', async () => {
  const f = await organizationFixture(1),
    u = f.members[0];
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/creation-grants',
      { operation_id: operation('grant'), expected_revision: '1', user_id: u.id, allowed: true },
      f.owner.headers,
    ),
    f.owner,
  );
  const d = await documentFixture(f, u);
  await waitOperation(
    await post(
      '/v1/organizations/' + f.organization + '/members/' + u.id + '/role',
      {
        operation_id: operation('promote'),
        expected_revision: await memberRevision(f, u),
        role: 'ADMIN',
      },
      f.owner.headers,
    ),
    f.owner,
  );
  await waitOperation(
    await post(
      '/v1/organizations/' + f.organization + '/members/' + u.id + '/role',
      {
        operation_id: operation('demote'),
        expected_revision: await memberRevision(f, u),
        role: 'MEMBER',
      },
      f.owner.headers,
    ),
    f.owner,
  );
  assert.equal((await get('/v1/documents/' + d.id + '/editable-units', u.headers)).status, 200);
  const made = await post(
    '/v1/documents',
    {
      operation_id: operation('still-create'),
      space_id: f.space,
      knowledge_base_id: f.kb,
      title: '独立资格保留',
    },
    u.headers,
  );
  assert.equal(made.status, 201);
});

test('abandoned departure prepare only reopens after an authoritative serialized rollback proof', async () => {
  const f = await organizationFixture(1),
    u = f.members[0],
    account = await mysql.createConnection(
      JSON.parse(readFileSync(resolve(runtime, 'services/account.json'))).database,
    ),
    document = await db();
  const { request } = await import('../helpers/https.mjs'),
    { env } = await import('../helpers/account.mjs'),
    id = operation('abandoned-prepare'),
    scope = 'org-member:' + f.organization + ':' + u.id;
  try {
    await account.beginTransaction();
    await account.query('SELECT id FROM organization WHERE id=? FOR UPDATE', [f.organization]);
    const prepared = await request(
      env.ports.documentHttp,
      '/infra/lifecycle/member-departure-eligibility?organization_id=' +
        f.organization +
        '&user_id=' +
        u.id +
        '&operation_id=' +
        id +
        '&epoch=1',
      {
        key: readFileSync(resolve(runtime, 'pki/account-key.pem')),
        cert: readFileSync(resolve(runtime, 'pki/account.pem')),
      },
    );
    assert.equal(prepared.status, 200);
    const [[g]] = await document.query(
      'SELECT state,last_revocation_id FROM access_gate WHERE scope_key=?',
      [scope],
    );
    assert.equal(g.state, 'BLOCKED');
    assert.equal(g.last_revocation_id, id);
    await new Promise((r) => setTimeout(r, 200));
    const [[still]] = await document.query('SELECT state FROM access_gate WHERE scope_key=?', [
      scope,
    ]);
    assert.equal(still.state, 'BLOCKED');
    await account.rollback();
    let reopened = false;
    for (let i = 0; i < 60; i++) {
      const [[gate]] = await document.query('SELECT state FROM access_gate WHERE scope_key=?', [
        scope,
      ]);
      if (gate.state === 'OPEN') {
        reopened = true;
        break;
      }
      await new Promise((r) => setTimeout(r, 50));
    }
    assert.equal(reopened, true);
    await waitAuthorization(
      await post(
        '/v1/knowledge-bases/' + f.kb + '/roles',
        {
          operation_id: operation('after-rollback'),
          expected_acl_revision: '1',
          user_id: u.id,
          role: 'ADMIN',
        },
        f.owner.headers,
      ),
      f.owner,
    );
  } finally {
    await account.rollback();
    await account.end();
    await document.end();
  }
});
