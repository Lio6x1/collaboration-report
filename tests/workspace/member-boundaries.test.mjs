import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  createUser,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { waitOperation } from '../helpers/account.mjs';

// 个人空间管理不能冒充组织任免权，否则会生成接收方根本无法使用的角色。
test('personal space ownership cannot appoint knowledge-base peers or grant foreign creation rights', async () => {
  const owner = await createUser(),
    peer = await createUser();
  const space = json(await get('/v1/me/spaces', owner.headers)).items.find(
    (x) => x.kind === 'PERSONAL',
  );
  const made = await post(
    '/v1/spaces/' + space.id + '/knowledge-bases',
    { operation_id: operation('personal-kb'), name: '个人资料' },
    owner.headers,
  );
  assert.equal(made.status, 201);
  const kb = json(made).result.entity_id;
  const roles = await post(
    '/v1/knowledge-bases/' + kb + '/roles',
    {
      operation_id: operation('personal-role'),
      expected_acl_revision: '1',
      user_id: peer.id,
      role: 'ADMIN',
    },
    owner.headers,
  );
  assert.equal(roles.status, 403);
  const creation = await post(
    '/v1/knowledge-bases/' + kb + '/creation-grants',
    {
      operation_id: operation('personal-create'),
      expected_revision: '1',
      user_id: peer.id,
      allowed: true,
    },
    owner.headers,
  );
  assert.equal(creation.status, 403);
  const actions = JSON.parse(
    (await get('/v1/knowledge-bases/' + kb, owner.headers)).headers['x-available-actions'],
  );
  assert.ok(!actions.includes('CHANGE_ROLES'));
  assert.ok(!actions.includes('CHANGE_CREATION_GRANTS'));
});

// 历史库成员行仍需留存，但已退出组织者不能堵住当前成员的分页，也不能被输出。
test('knowledge-base directory omits departed ordinary members and preserves later keyset pages', async () => {
  const f = await organizationFixture(2),
    [departed, current] = f.members;
  for (const [i, member] of f.members.entries()) {
    await waitAuthorization(
      await post(
        '/v1/knowledge-bases/' + f.kb + '/roles',
        {
          operation_id: operation('member'),
          expected_acl_revision: String(i + 1),
          user_id: member.id,
          role: 'MEMBER',
        },
        f.owner.headers,
      ),
      f.owner,
    );
  }
  const members = json(
    await get('/v1/organizations/' + f.organization + '/members', f.owner.headers),
  );
  const revision = members.items.find((x) => x.user_id === departed.id).revision;
  await waitOperation(
    await post(
      '/v1/organizations/' + f.organization + '/leave',
      { operation_id: operation('leave'), expected_revision: revision },
      departed.headers,
    ),
    departed,
  );
  const first = await get('/v1/knowledge-bases/' + f.kb + '/members?limit=1', f.owner.headers);
  assert.equal(first.status, 200);
  const a = json(first);
  assert.equal(a.items.length, 1);
  assert.equal(a.has_more, true);
  const second = await get(
    '/v1/knowledge-bases/' + f.kb + '/members?limit=1&cursor=' + encodeURIComponent(a.next_cursor),
    f.owner.headers,
  );
  assert.equal(second.status, 200);
  const b = json(second);
  assert.equal(b.items.length, 1);
  assert.equal(b.has_more, false);
  assert.deepEqual(
    new Set([...a.items, ...b.items].map((x) => x.user_id)),
    new Set([f.owner.id, current.id]),
  );
});
