import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
test('an ordinary organization member who administers a knowledge base must hand off that role before leaving', async () => {
  const f = await organizationFixture(1),
    member = f.members[0];
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/roles',
      {
        operation_id: operation('appoint'),
        expected_acl_revision: '1',
        user_id: member.id,
        role: 'ADMIN',
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const list = json(await get('/v1/organizations/' + f.organization + '/members', f.owner.headers)),
    m = list.items.find((x) => x.user_id === member.id);
  const leave = await post(
    '/v1/organizations/' + f.organization + '/leave',
    { operation_id: operation('leave'), expected_revision: m.revision },
    member.headers,
  );
  assert.equal(leave.status, 409);
  assert.equal(json(leave).code, 'ADMIN_RETIREMENT_REQUIRED');
  const remove = await post(
    '/v1/organizations/' + f.organization + '/members/' + member.id,
    { operation_id: operation('remove'), expected_revision: m.revision },
    f.owner.headers,
    'DELETE',
  );
  assert.equal(remove.status, 409);
  const kb = json(await get('/v1/knowledge-bases/' + f.kb, f.owner.headers));
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/roles',
      {
        operation_id: operation('handoff'),
        expected_acl_revision: kb.acl_revision,
        user_id: member.id,
        role: 'REMOVE',
      },
      f.owner.headers,
    ),
    f.owner,
  );
  assert.equal(
    (
      await post(
        '/v1/organizations/' + f.organization + '/leave',
        { operation_id: operation('leaveafter'), expected_revision: m.revision },
        member.headers,
      )
    ).status,
    202,
  );
});
