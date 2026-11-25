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

test('space and document keyset pages honor limit and bind cursors to the current actor', async () => {
  const f = await organizationFixture(1);
  const first = await get('/v1/me/spaces?limit=1', f.owner.headers);
  assert.equal(first.status, 200);
  assert.equal(json(first).items.length, 1);
  assert.equal(json(first).has_more, true);
  const next = await get(
    '/v1/me/spaces?limit=1&cursor=' + encodeURIComponent(json(first).next_cursor),
    f.owner.headers,
  );
  assert.equal(next.status, 200);
  assert.equal(json(next).items.length, 1);
  assert.equal(json(next).items[0].organization_id, f.organization);
  assert.ok(
    [400, 409].includes(
      (
        await get(
          '/v1/me/spaces?limit=1&cursor=' + encodeURIComponent(json(first).next_cursor),
          f.members[0].headers,
        )
      ).status,
    ),
  );
  await documentFixture(f);
  await documentFixture(f);
  const listed = await get('/v1/knowledge-bases/' + f.kb + '/documents?limit=1', f.owner.headers);
  assert.equal(listed.status, 200);
  assert.equal(json(listed).items.length, 1);
  assert.equal(json(listed).has_more, true);
  const second = await get(
    '/v1/knowledge-bases/' +
      f.kb +
      '/documents?limit=1&cursor=' +
      encodeURIComponent(json(listed).next_cursor),
    f.owner.headers,
  );
  assert.equal(second.status, 200);
  assert.notEqual(json(second).items[0].id, json(listed).items[0].id);
  assert.equal(
    (await get('/v1/knowledge-bases/' + f.kb + '/documents', f.members[0].headers)).status,
    200,
  );
});
test('knowledge-base members use current scope and owner appoints then removes a role with epoch closure', async () => {
  const f = await organizationFixture(1),
    member = f.members[0];
  const original = await get('/v1/knowledge-bases/' + f.kb, f.owner.headers);
  assert.equal(original.status, 200);
  assert.equal(json(original).acl_revision, '1');
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/roles',
      {
        operation_id: operation('role'),
        expected_acl_revision: '1',
        user_id: member.id,
        role: 'ADMIN',
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const members = await get('/v1/knowledge-bases/' + f.kb + '/members', f.owner.headers);
  assert.equal(members.status, 200);
  assert.ok(json(members).items.some((x) => x.user_id === member.id && x.role === 'ADMIN'));
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/roles',
      {
        operation_id: operation('remove'),
        expected_acl_revision: '2',
        user_id: member.id,
        role: 'REMOVE',
      },
      f.owner.headers,
    ),
    f.owner,
  );
  assert.equal((await get('/v1/knowledge-bases/' + f.kb + '/members', member.headers)).status, 403);
});
