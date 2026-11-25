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

test('responsible transfer is direct and replay returns the original receipt after management is lost', async () => {
  const f = await organizationFixture(2),
    [creator, next] = f.members;
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/creation-grants',
      {
        operation_id: operation('create'),
        expected_revision: '1',
        user_id: creator.id,
        allowed: true,
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const d = await documentFixture(f, creator),
    input = {
      operation_id: operation('transfer'),
      expected_revision: '1',
      target_user_id: next.id,
    },
    path = '/v1/documents/' + d.id + '/responsible-transfers';
  const first = await post(path, input, creator.headers);
  assert.equal(first.status, 200);
  await waitAuthorization(first, creator);
  const changed = await get('/v1/documents/' + d.id, next.headers);
  assert.equal(changed.status, 200);
  assert.equal(json(changed).responsible_user_id, next.id);
  assert.equal((await get('/v1/documents/' + d.id, creator.headers)).status, 403);
  const replay = await post(path, input, creator.headers);
  assert.equal(replay.status, 200);
  assert.equal(json(replay).result.entity_id, d.id);
  const renamed = await post(
    '/v1/documents/' + d.id,
    {
      operation_id: operation('rename'),
      expected_control_revision: json(changed).control_revision,
      title: '直接接任',
    },
    next.headers,
    'PATCH',
  );
  assert.equal(renamed.status, 200);
});
test('document deletion is durably fenced and an old operation can still be queried by its actor', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    doc = await get('/v1/documents/' + d.id, f.owner.headers);
  assert.equal(doc.status, 200);
  const deleted = await post(
    '/v1/documents/' + d.id,
    { operation_id: operation('delete'), expected_revision: json(doc).control_revision },
    f.owner.headers,
    'DELETE',
  );
  assert.equal(deleted.status, 202);
  await waitAuthorization(deleted, f.owner);
  assert.equal((await get('/v1/documents/' + d.id, f.owner.headers)).status, 404);
});
