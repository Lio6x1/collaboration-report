import test from 'node:test';
import assert from 'node:assert/strict';
import { createUser, get, post, json, operation } from '../helpers/account.mjs';
test('personal document becomes ACTIVE only with a complete immutable structure style and manifest', async () => {
  const user = await createUser(),
    spaces = await get('/v1/me/spaces', user.headers);
  assert.equal(spaces.status, 200);
  const personal = json(spaces).items.find(
    (s) => s.kind === 'PERSONAL' && s.owner_user_id === user.id,
  );
  assert.ok(personal);
  const input = {
      operation_id: operation('document'),
      space_id: personal.id,
      title: '我的校招报告',
    },
    created = await post('/v1/documents', input, user.headers);
  assert.equal(created.status, 201);
  const id = json(created).result.entity_id,
    doc = await get('/v1/documents/' + id, user.headers);
  assert.equal(doc.status, 200);
  const value = json(doc);
  assert.equal(value.lifecycle, 'ACTIVE');
  assert.equal(value.main_locked, false);
  for (const key of ['current_structure_id', 'current_style_id', 'current_manifest_id'])
    assert.match(value[key], /^[1-9][0-9]*$/);
  const main = await get('/v1/documents/' + id + '/main', user.headers);
  assert.equal(main.status, 200);
  assert.ok(json(main).units.length > 0);
  const replay = await post('/v1/documents', input, user.headers);
  assert.equal(replay.status, 201);
  assert.equal(json(replay).result.entity_id, id);
});
test('ordinary organization membership gives neither knowledge-base nor document creation by default', async () => {
  const owner = await createUser(),
    member = await createUser(),
    created = await post(
      '/v1/organizations',
      { operation_id: operation('organization'), name: '工作台权限组' },
      owner.headers,
    );
  assert.equal(created.status, 201);
  const id = json(created).result.entity_id;
  const applied = await post(
    '/v1/organizations/' + id + '/applications',
    { operation_id: operation('apply'), reason: '加入' },
    member.headers,
  );
  assert.equal(applied.status, 200);
  assert.equal(
    (
      await post(
        '/v1/applications/' + json(applied).result.entity_id + '/decisions',
        {
          operation_id: operation('accept'),
          expected_revision: '1',
          decision: 'ACCEPT',
          reason: '通过',
        },
        owner.headers,
      )
    ).status,
    200,
  );
  const spaces = await get('/v1/me/spaces', member.headers);
  assert.equal(spaces.status, 200);
  const space = json(spaces).items.find((s) => s.organization_id === id);
  assert.ok(space);
  assert.equal(
    (
      await post(
        '/v1/spaces/' + space.id + '/knowledge-bases',
        { operation_id: operation('kb'), name: '普通成员越权' },
        member.headers,
      )
    ).status,
    403,
  );
  assert.equal(
    (
      await post(
        '/v1/documents',
        { operation_id: operation('doc'), space_id: space.id, title: '普通成员越权' },
        member.headers,
      )
    ).status,
    403,
  );
});
