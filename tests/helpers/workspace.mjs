import assert from 'node:assert/strict';
import { createUser, get, post, json, operation } from './account.mjs';
export { createUser, get, post, json, operation };
export async function organizationFixture(count = 2, existingOwner = null) {
  // 已配置真实SMTP时可复用专用体验身份；业务数据仍在本次新建的组织中隔离。
  const owner = existingOwner ?? (await createUser()),
    members = [];
  const made = await post(
    '/v1/organizations',
    { operation_id: operation('organization'), name: '集成验证组织' },
    owner.headers,
  );
  assert.equal(made.status, 201);
  const organization = json(made).result.entity_id;
  for (let n = 0; n < count; n++) {
    const member = await createUser(),
      applied = await post(
        '/v1/organizations/' + organization + '/applications',
        { operation_id: operation('apply'), reason: '协作' },
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
    members.push(member);
  }
  // 复用身份可能已有多页组织；新建组织不会保证出现在目录首页。
  let space,
    cursor = '';
  do {
    const directory = await get(
      '/v1/me/spaces?limit=5' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
      owner.headers,
    );
    assert.equal(directory.status, 200);
    const page = json(directory);
    space = page.items.find((s) => s.organization_id === organization);
    cursor = page.has_more ? page.next_cursor : '';
  } while (!space && cursor);
  assert.ok(space);
  const kbMade = await post(
    '/v1/spaces/' + space.id + '/knowledge-bases',
    { operation_id: operation('kb'), name: '技术实践库' },
    owner.headers,
  );
  assert.equal(kbMade.status, 201);
  const kb = json(kbMade).result.entity_id;
  return { owner, members, organization, space: space.id, kb };
}
export async function documentFixture(f, user = f.owner) {
  const r = await post(
    '/v1/documents',
    {
      operation_id: operation('doc'),
      space_id: f.space,
      knowledge_base_id: f.kb,
      title: '协作报告',
    },
    user.headers,
  );
  assert.equal(r.status, 201);
  const id = json(r).result.entity_id;
  const main = await get('/v1/documents/' + id + '/main', user.headers);
  assert.equal(main.status, 200);
  return { id, main: json(main) };
}
export async function waitAuthorization(r, user) {
  assert.ok([200, 202].includes(r.status));
  let v = json(r);
  if (v.state === 'SUCCEEDED') return v;
  for (let i = 0; i < 100; i++) {
    const p = await get('/v1/authorization-changes/' + v.operation_id, user.headers);
    assert.equal(p.status, 200);
    v = json(p);
    if (v.state === 'SUCCEEDED') return v;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.fail('AUTHORIZATION_NOT_TERMINAL');
}
