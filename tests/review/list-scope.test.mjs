import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, submitDraft } from '../helpers/review.mjs';
import { post, get, json, operation, waitAuthorization } from '../helpers/workspace.mjs';

// 候选范围必须包含真正的审核来源；COMMENT不可见待办，撤销REVIEW立即失效，组织管理来源不依赖负责人字段。
test('review directory keeps direct reviewer and organization manager scope without probing unrelated documents', async (t) => {
  const f = await reviewFixture(1),
    reader = f.f.members[0];
  try {
    await writeDraft(f, '候选范围仍需权威资格复核');
    const submitted = (await submitDraft(f)).value.result;
    f.close();
    const mine = async (user) => {
      const started = Date.now(),
        response = await get('/v1/me/review-requests', user.headers);
      assert.equal(response.status, 200, json(response).code);
      t.diagnostic('Scoped review list completed in ' + (Date.now() - started) + ' ms');
      return json(response).items;
    };
    const grant = {
      user_id: reader.id,
      scope_kind: 'DOCUMENT',
      scope_id: f.d.id,
      capability: 'REVIEW',
    };
    const change = async (action) => {
      const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${f.d.id}/authorization-changes`,
          {
            operation_id: operation('review-scope-acl'),
            expected_acl_revision: current.acl_revision,
            changes: [{ action, grant }],
          },
          f.user.headers,
        ),
        f.user,
      );
    };
    assert.equal((await mine(reader)).length, 0);
    await change('GRANT');
    assert.ok((await mine(reader)).some((r) => r.id === submitted.request_id));
    await change('REVOKE');
    assert.equal((await mine(reader)).length, 0);
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const transferred = await post(
      `/v1/documents/${f.d.id}/responsible-transfers`,
      {
        operation_id: operation('review-scope-transfer'),
        expected_revision: current.control_revision,
        target_user_id: reader.id,
      },
      f.user.headers,
    );
    assert.equal(transferred.status, 200, json(transferred).code);
    await waitAuthorization(transferred, f.user);
    assert.equal(
      json(await get(`/v1/documents/${f.d.id}`, f.user.headers)).responsible_user_id,
      reader.id,
    );
    assert.ok((await mine(f.user)).some((r) => r.id === submitted.request_id));
  } finally {
    f.close();
  }
});
