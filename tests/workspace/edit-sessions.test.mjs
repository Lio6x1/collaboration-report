import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  post,
  get,
  json,
  operation,
} from '../helpers/workspace.mjs';
// 真实HTTP边界：普通成员不能凭知道unit ID取得草稿票据，负责人可进入唯一共同支线。
test('a real edit session is scoped to the current unit branch and permission rather than just document reading', async () => {
  const f = await organizationFixture(1),
    d = await documentFixture(f),
    unit = d.main.units[0].unit_id;
  const input = {
    operation_id: operation('edit'),
    workspace_kind: 'BRANCH',
    client_session_id: operation('client'),
    expected_structure_id: d.main.structure.id,
  };
  const opened = await post(
    `/v1/documents/${d.id}/units/${unit}/edit-sessions`,
    input,
    f.owner.headers,
  );
  assert.equal(opened.status, 200);
  const session = json(opened);
  assert.ok(session.ticket);
  assert.equal(session.wss_url.includes('?'), false);
  assert.ok(session.branch_id);
  assert.ok(session.lineage_id);
  const denied = await post(
    `/v1/documents/${d.id}/units/${unit}/edit-sessions`,
    { ...input, operation_id: operation('denied') },
    f.members[0].headers,
  );
  assert.equal(denied.status, 403);
  const replay = await post(
    `/v1/documents/${d.id}/units/${unit}/edit-sessions`,
    input,
    f.owner.headers,
  );
  assert.equal(replay.status, 200);
  assert.deepEqual(json(replay), session);
  const locked = await post(
    `/v1/documents/${d.id}/units/${unit}/edit-sessions`,
    { ...input, operation_id: operation('main'), workspace_kind: 'MAIN' },
    f.owner.headers,
  );
  assert.equal(locked.status, 409);
  const stale = await post(
    `/v1/documents/${d.id}/units/${unit}/edit-sessions`,
    { ...input, operation_id: operation('stale'), expected_structure_id: '1' },
    f.owner.headers,
  );
  assert.equal(stale.status, 409);
});
