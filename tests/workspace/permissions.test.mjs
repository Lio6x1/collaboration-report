import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  createUser,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';

// 用真实组织身份、库角色和文档ACL组合验证，避免只测试纯权限函数而漏掉资格来源。
test('actual inherited and direct role combinations expose only the current management and edit range', async () => {
  const f = await organizationFixture(7),
    [orgAdmin, kbAdmin, responsible, manager, reviewer, editor, none] = f.members;
  const members = json(
    await get('/v1/organizations/' + f.organization + '/members', f.owner.headers),
  );
  await import('../helpers/account.mjs').then(async ({ waitOperation }) =>
    waitOperation(
      await post(
        '/v1/organizations/' + f.organization + '/members/' + orgAdmin.id + '/role',
        {
          operation_id: operation('org-admin'),
          expected_revision: members.items.find((x) => x.user_id === orgAdmin.id).revision,
          role: 'ADMIN',
        },
        f.owner.headers,
      ),
      f.owner,
    ),
  );
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/roles',
      {
        operation_id: operation('kb-admin'),
        expected_acl_revision: '1',
        user_id: kbAdmin.id,
        role: 'ADMIN',
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const d = await documentFixture(f),
    unit = d.main.units[0].unit_id;
  await waitAuthorization(
    await post(
      '/v1/documents/' + d.id + '/responsible-transfers',
      {
        operation_id: operation('responsible'),
        expected_revision: '1',
        target_user_id: responsible.id,
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const doc = json(await get('/v1/documents/' + d.id, f.owner.headers));
  const grant = (u, cap, kind = 'DOCUMENT', id = d.id) => ({
    grant: { user_id: u.id, scope_kind: kind, scope_id: id, capability: cap },
    action: 'GRANT',
  });
  await waitAuthorization(
    await post(
      '/v1/documents/' + d.id + '/authorization-changes',
      {
        operation_id: operation('matrix'),
        expected_acl_revision: doc.acl_revision,
        changes: [
          grant(manager, 'MANAGE'),
          grant(manager, 'EDIT', 'UNIT', unit),
          grant(reviewer, 'REVIEW'),
          grant(editor, 'EDIT', 'UNIT', unit),
        ],
      },
      f.owner.headers,
    ),
    f.owner,
  );
  for (const u of [f.owner, orgAdmin, kbAdmin, responsible, manager]) {
    const header = await get('/v1/documents/' + d.id, u.headers);
    assert.equal(header.status, 200);
    assert.ok(JSON.parse(header.headers['x-available-actions']).includes('CHANGE_AUTHORIZATION'));
    assert.equal(
      json(await get('/v1/documents/' + d.id + '/editable-units', u.headers)).items.length,
      1,
    );
  }
  assert.equal((await get('/v1/documents/' + d.id + '/main', reviewer.headers)).status, 200);
  assert.equal(
    json(await get('/v1/documents/' + d.id + '/editable-units', reviewer.headers)).items.length,
    0,
  );
  assert.equal((await get('/v1/documents/' + d.id + '/main', editor.headers)).status, 200);
  assert.equal(
    json(await get('/v1/documents/' + d.id + '/editable-units', editor.headers)).items.length,
    1,
  );
  assert.equal((await get('/v1/documents/' + d.id + '/main', none.headers)).status, 403);
  const current = json(await get('/v1/documents/' + d.id, f.owner.headers));
  await waitAuthorization(
    await post(
      '/v1/documents/' + d.id + '/authorization-changes',
      {
        operation_id: operation('remove-management'),
        expected_acl_revision: current.acl_revision,
        changes: [{ ...grant(manager, 'MANAGE'), action: 'REVOKE' }],
      },
      f.owner.headers,
    ),
    f.owner,
  );
  assert.equal(
    json(await get('/v1/documents/' + d.id + '/editable-units', manager.headers)).items.length,
    1,
  );
  assert.ok(
    !JSON.parse(
      (await get('/v1/documents/' + d.id, manager.headers)).headers['x-available-actions'],
    ).includes('CHANGE_AUTHORIZATION'),
  );
});

test('knowledge-base admin cannot appoint peers and targets must currently belong to its organization', async () => {
  const f = await organizationFixture(),
    [member, other] = f.members;
  const appointed = await post(
    '/v1/knowledge-bases/' + f.kb + '/roles',
    {
      operation_id: operation('role'),
      expected_acl_revision: '1',
      user_id: member.id,
      role: 'ADMIN',
    },
    f.owner.headers,
  );
  await waitAuthorization(appointed, f.owner);
  assert.equal(
    (
      await post(
        '/v1/knowledge-bases/' + f.kb + '/roles',
        {
          operation_id: operation('peer'),
          expected_acl_revision: '2',
          user_id: other.id,
          role: 'ADMIN',
        },
        member.headers,
      )
    ).status,
    403,
  );
  const outside = await createUser();
  assert.equal(
    (
      await post(
        '/v1/knowledge-bases/' + f.kb + '/roles',
        {
          operation_id: operation('cross'),
          expected_acl_revision: '2',
          user_id: outside.id,
          role: 'ADMIN',
        },
        f.owner.headers,
      )
    ).status,
    403,
  );
  assert.equal(
    (
      await post(
        '/v1/documents',
        {
          operation_id: operation('document'),
          space_id: f.space,
          knowledge_base_id: f.kb,
          title: '知识库管理员创建',
        },
        member.headers,
      )
    ).status,
    201,
  );
});
test('creation eligibility is independent from management or organization admin roles', async () => {
  const f = await organizationFixture(1),
    member = f.members[0];
  assert.equal(
    (
      await post(
        '/v1/documents',
        {
          operation_id: operation('denied'),
          space_id: f.space,
          knowledge_base_id: f.kb,
          title: '默认拒绝',
        },
        member.headers,
      )
    ).status,
    403,
  );
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/creation-grants',
      {
        operation_id: operation('allow'),
        expected_revision: '1',
        user_id: member.id,
        allowed: true,
      },
      f.owner.headers,
    ),
    f.owner,
  );
  await documentFixture(f, member);
  assert.equal(
    (
      await post(
        '/v1/knowledge-bases/' + f.kb,
        { operation_id: operation('rename'), expected_revision: '2', name: '越权改库' },
        member.headers,
        'PATCH',
      )
    ).status,
    403,
  );
  const personal = await get('/v1/me/spaces', member.headers),
    space = json(personal).items.find((x) => x.kind === 'PERSONAL');
  assert.equal(
    (
      await post(
        '/v1/spaces/' + space.id + '/knowledge-bases',
        { operation_id: operation('foreign'), name: '组织管理员越界' },
        f.owner.headers,
      )
    ).status,
    403,
  );
});
test('scope identifiers belong to the document and revoking a direct edit preserves inherited chapter edit', async () => {
  const f = await organizationFixture(1),
    member = f.members[0],
    d = await documentFixture(f),
    other = await documentFixture(f),
    unit = d.main.units[0].unit_id,
    chapter = d.main.structure.entries[0].chapter_id;
  const change = (revision, changes) =>
    post(
      '/v1/documents/' + d.id + '/authorization-changes',
      { operation_id: operation('acl'), expected_acl_revision: revision, changes },
      f.owner.headers,
    );
  const grant = (kind, id) => ({
    user_id: member.id,
    scope_kind: kind,
    scope_id: id,
    capability: 'EDIT',
  });
  assert.equal(
    (await change('1', [{ grant: grant('UNIT', other.main.units[0].unit_id), action: 'GRANT' }]))
      .status,
    400,
  );
  await waitAuthorization(
    await change('1', [
      { grant: grant('CHAPTER', chapter), action: 'GRANT' },
      { grant: grant('UNIT', unit), action: 'GRANT' },
    ]),
    f.owner,
  );
  assert.equal((await get('/v1/documents/' + d.id + '/main', member.headers)).status, 200);
  let editable = await get('/v1/documents/' + d.id + '/editable-units', member.headers);
  assert.equal(editable.status, 200);
  assert.equal(json(editable).items[0].unit_id, unit);
  await waitAuthorization(
    await change('2', [{ grant: grant('UNIT', unit), action: 'REVOKE' }]),
    f.owner,
  );
  editable = await get('/v1/documents/' + d.id + '/editable-units', member.headers);
  assert.equal(editable.status, 200);
  assert.equal(json(editable).items[0].unit_id, unit);
  const value = await get('/v1/documents/' + d.id, member.headers);
  assert.equal(value.status, 200);
  assert.ok(!JSON.parse(value.headers['x-available-actions']).includes('CHANGE_AUTHORIZATION'));
});
