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

test(
  'organization and library pagination filter before limiting and bind search cursors',
  { timeout: 60000 },
  async (t) => {
    const f = await organizationFixture(2);
    const path = '/v1/organizations/' + f.organization;
    await t.test('own membership is available independently of a member page', async () => {
      const response = await get(path, f.members[1].headers);
      assert.equal(response.status, 200, response.body);
      assert.equal(json(response).current_member?.user_id, f.members[1].id);
      assert.equal(json(response).current_member?.role, 'MEMBER');
    });
    await t.test(
      'member search locates a later identity without consuming earlier pages',
      async () => {
        const response = await get(
          path + '/members?limit=1&q=' + f.members[1].id.padStart(8, '0'),
          f.owner.headers,
        );
        assert.equal(response.status, 200, response.body);
        assert.deepEqual(
          json(response).items.map((item) => item.user_id),
          [f.members[1].id],
        );
        assert.equal(json(response).has_more, false);
        const first = json(await get(path + '/members?limit=1', f.owner.headers));
        const reused = await get(
          path + '/members?limit=1&q=missing&cursor=' + encodeURIComponent(first.next_cursor),
          f.owner.headers,
        );
        assert.equal(reused.status, 400, reused.body);
        assert.equal(json(reused).code, 'CURSOR_INVALID');
      },
    );
    await t.test('library name search runs across the complete authorized directory', async () => {
      const made = await post(
        '/v1/spaces/' + f.space + '/knowledge-bases',
        { operation_id: operation('paged-kb'), name: '分页名称筛选命中' },
        f.owner.headers,
      );
      assert.equal(made.status, 201, made.body);
      const response = await get(
        '/v1/spaces/' +
          f.space +
          '/knowledge-bases?limit=1&q=' +
          encodeURIComponent('名称筛选命中'),
        f.owner.headers,
      );
      assert.equal(response.status, 200, response.body);
      assert.deepEqual(
        json(response).items.map((item) => item.id),
        [json(made).result.entity_id],
      );
      assert.equal(json(response).has_more, false);
    });
    await t.test(
      'library member search includes later authorized members and binds its filter cursor',
      async () => {
        for (const member of f.members) {
          const kb = json(await get('/v1/knowledge-bases/' + f.kb, f.owner.headers));
          const changed = await post(
            '/v1/knowledge-bases/' + f.kb + '/roles',
            {
              operation_id: operation('paged-kb-member'),
              expected_acl_revision: kb.acl_revision,
              user_id: member.id,
              role: 'MEMBER',
            },
            f.owner.headers,
          );
          assert.equal((await waitAuthorization(changed, f.owner)).state, 'SUCCEEDED');
        }
        const path = '/v1/knowledge-bases/' + f.kb + '/members';
        const response = await get(
          path + '?limit=1&q=' + f.members[1].id.padStart(8, '0'),
          f.owner.headers,
        );
        assert.equal(response.status, 200, response.body);
        assert.deepEqual(
          json(response).items.map((item) => item.user_id),
          [f.members[1].id],
        );
        const first = json(await get(path + '?limit=1', f.owner.headers));
        assert.ok(first.next_cursor);
        const reused = await get(
          path + '?limit=1&q=missing&cursor=' + encodeURIComponent(first.next_cursor),
          f.owner.headers,
        );
        assert.equal(reused.status, 409, reused.body);
        const oversized = await get(path + '?q=' + 'a'.repeat(101), f.owner.headers);
        assert.equal(oversized.status, 400, oversized.body);
      },
    );
  },
);
