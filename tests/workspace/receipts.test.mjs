import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
test('lost KB rename and document creation responses can return their original minimal receipt after qualification is removed', async () => {
  const f = await organizationFixture(1),
    u = f.members[0];
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/roles',
      { operation_id: operation('role'), expected_acl_revision: '1', user_id: u.id, role: 'ADMIN' },
      f.owner.headers,
    ),
    f.owner,
  );
  const path = '/v1/knowledge-bases/' + f.kb,
    input = { operation_id: operation('rename'), expected_revision: '2', name: '固定原结果' };
  await waitAuthorization(await post(path, input, u.headers, 'PATCH'), u);
  const created = {
    operation_id: operation('create'),
    space_id: f.space,
    knowledge_base_id: f.kb,
    title: '原创建结果',
  };
  const d = await post('/v1/documents', created, u.headers);
  assert.equal(d.status, 201);
  const kb = json(await get(path, f.owner.headers));
  await waitAuthorization(
    await post(
      path + '/roles',
      {
        operation_id: operation('remove'),
        expected_acl_revision: kb.acl_revision,
        user_id: u.id,
        role: 'REMOVE',
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const replay = await post(path, input, u.headers, 'PATCH');
  assert.equal(replay.status, 200);
  assert.equal(json(replay).state, 'SUCCEEDED');
  assert.equal(json(replay).operation_id, input.operation_id);
  const retry = await post('/v1/documents', created, u.headers);
  assert.equal(retry.status, 201);
  assert.equal(json(retry).result.entity_id, json(d).result.entity_id);
  assert.equal(
    (await post(path, { ...input, operation_id: operation('new') }, u.headers, 'PATCH')).status,
    403,
  );
  assert.equal(
    (await post('/v1/documents', { ...created, operation_id: operation('new-create') }, u.headers))
      .status,
    403,
  );
});
test('a manager revoking their own last grant can retry only the original authorization result', async () => {
  const f = await organizationFixture(1),
    u = f.members[0],
    d = await documentFixture(f),
    grant = { user_id: u.id, scope_kind: 'DOCUMENT', scope_id: d.id, capability: 'MANAGE' };
  await waitAuthorization(
    await post(
      '/v1/documents/' + d.id + '/authorization-changes',
      {
        operation_id: operation('manage'),
        expected_acl_revision: '1',
        changes: [{ action: 'GRANT', grant }],
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const input = {
      operation_id: operation('selfrevoke'),
      expected_acl_revision: '2',
      changes: [{ action: 'REVOKE', grant }],
    },
    path = '/v1/documents/' + d.id + '/authorization-changes';
  const r = await post(path, input, u.headers);
  assert.equal(r.status, 202);
  await waitAuthorization(r, u);
  assert.equal((await get('/v1/documents/' + d.id, u.headers)).status, 403);
  const retry = await post(path, input, u.headers);
  assert.equal(retry.status, 202);
  assert.equal(json(retry).state, 'SUCCEEDED');
  assert.equal(json(retry).operation_id, input.operation_id);
  assert.equal(
    (await post(path, { ...input, operation_id: operation('new') }, u.headers)).status,
    403,
  );
});
