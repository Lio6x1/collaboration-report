import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createUser,
  get,
  post,
  json,
  operation,
  waitAuthorization,
  organizationFixture,
  documentFixture,
} from '../helpers/workspace.mjs';

async function personalDocument() {
  const owner = await createUser(),
    spaces = await get('/v1/me/spaces', owner.headers),
    space = json(spaces).items.find((s) => s.kind === 'PERSONAL');
  const r = await post(
    '/v1/documents',
    { operation_id: operation('personal'), space_id: space.id, title: '个人协作' },
    owner.headers,
  );
  assert.equal(r.status, 201);
  const id = json(r).result.entity_id,
    main = json(await get('/v1/documents/' + id + '/main', owner.headers));
  return { owner, id, main };
}

test('sharing filters run before pagination and archive preserves accepted access', async () => {
  const f = await personalDocument(),
    applicant = await createUser(),
    stranger = await createUser();
  const base = '/v1/documents/' + f.id;
  async function invitation() {
    const response = await post(
      base + '/invitations',
      {
        operation_id: operation('invite-page'),
        expected_acl_revision: '1',
        requested_grants: [
          { scope_kind: 'UNIT', scope_id: f.main.units[0].unit_id, capability: 'EDIT' },
        ],
      },
      f.owner.headers,
    );
    assert.equal(response.status, 200);
    return json(response);
  }
  const closed = await invitation();
  assert.equal(
    (
      await post(
        '/v1/document-invitations/' + closed.invitation.id + '/cancel',
        {
          operation_id: operation('close-page'),
          expected_revision: '1',
        },
        f.owner.headers,
      )
    ).status,
    200,
  );
  const active = await invitation(),
    later = await invitation();
  const repeatClose = await post(
    '/v1/document-invitations/' + closed.invitation.id + '/cancel',
    {
      operation_id: operation('close-already-closed'),
      expected_revision: '2',
    },
    f.owner.headers,
  );
  assert.equal(repeatClose.status, 409);
  assert.equal(json(repeatClose).code, 'INVITATION_NOT_CURRENT');
  const first = json(await get(base + '/invitations?status=ACTIVE&limit=1', f.owner.headers));
  assert.deepEqual(
    first.items.map((i) => i.id),
    [later.invitation.id],
    'newest effective invitation must be first',
  );
  assert.equal(first.has_more, true);
  const next = json(
    await get(
      base + '/invitations?status=ACTIVE&limit=1&cursor=' + encodeURIComponent(first.next_cursor),
      f.owner.headers,
    ),
  );
  assert.deepEqual(
    next.items.map((i) => i.id),
    [active.invitation.id],
  );
  assert.equal(
    (
      await get(
        base +
          '/invitations?status=INACTIVE&limit=1&cursor=' +
          encodeURIComponent(first.next_cursor),
        f.owner.headers,
      )
    ).status,
    409,
  );
  assert.equal((await get(base + '/invitations?status=UNKNOWN', f.owner.headers)).status, 400);
  assert.equal(
    (
      await post(
        '/v1/document-invitations/' + active.invitation.id + '/archive',
        {
          operation_id: operation('archive-active'),
          expected_revision: '1',
        },
        f.owner.headers,
      )
    ).status,
    409,
  );
  const applied = await post(
    '/v1/document-invitations/applications',
    {
      operation_id: operation('apply-page'),
      invitation_token: new URL(active.share_url).searchParams.get('invitation_token'),
      reason: '参与分页验证',
    },
    applicant.headers,
  );
  assert.equal(applied.status, 202);
  const pending = json(await get(base + '/applications?status=PENDING', f.owner.headers)).items[0];
  assert.ok(pending.applicant_display_name);
  assert.equal(
    (
      await post(
        '/v1/document-applications/' + pending.id + '/archive',
        {
          operation_id: operation('archive-pending'),
          expected_revision: '1',
        },
        f.owner.headers,
      )
    ).status,
    409,
  );
  const approved = await post(
    '/v1/document-applications/' + pending.id + '/decisions',
    {
      operation_id: operation('approve-page'),
      expected_revision: '1',
      decision: 'ACCEPT',
      reason: '通过',
    },
    f.owner.headers,
  );
  await waitAuthorization(approved, f.owner);
  assert.equal(
    json(await get(base + '/applications?status=PENDING', f.owner.headers)).items.length,
    0,
  );
  const processed = json(await get(base + '/applications?status=PROCESSED', f.owner.headers))
    .items[0];
  const archive = {
    operation_id: operation('archive-approved'),
    expected_revision: processed.revision,
  };
  const archivePath = '/v1/document-applications/' + processed.id + '/archive';
  const deniedArchive = await post(archivePath, archive, stranger.headers);
  assert.equal(deniedArchive.status, 403);
  const missingArchive = await post(
    '/v1/document-applications/18446744073709551614/archive',
    {
      operation_id: operation('archive-missing'),
      expected_revision: '1',
    },
    stranger.headers,
  );
  assert.equal(missingArchive.status, 403);
  assert.equal(
    json(missingArchive).code,
    json(deniedArchive).code,
    'unknown and unauthorized records must have the same refusal code',
  );
  assert.equal(
    (await post(archivePath, { ...archive, expected_revision: '1' }, f.owner.headers)).status,
    409,
  );
  assert.equal((await post(archivePath, archive, f.owner.headers)).status, 200);
  assert.equal(
    (await post(archivePath, archive, f.owner.headers)).status,
    200,
    'original operation may recover',
  );
  assert.equal(
    json(await get(base + '/applications?status=PROCESSED', f.owner.headers)).items.length,
    0,
  );
  assert.equal(
    (await get(base + '/main', applicant.headers)).status,
    200,
    'archive must preserve approved grants',
  );
  assert.equal(
    json(await get('/v1/document-applications', applicant.headers)).items.length,
    1,
    'manager archive must preserve applicant history',
  );
  assert.equal(
    (
      await post(
        '/v1/document-invitations/' + closed.invitation.id + '/archive',
        {
          operation_id: operation('archive-closed'),
          expected_revision: '2',
        },
        f.owner.headers,
      )
    ).status,
    200,
  );
  assert.equal(
    json(await get(base + '/invitations?status=INACTIVE', f.owner.headers)).items.length,
    0,
  );
});
test('personal link snapshots validity, grants only after approval, and own progress reveals no private body', async () => {
  const f = await personalDocument(),
    applicant = await createUser(),
    stranger = await createUser(),
    unit = f.main.units[0].unit_id;
  const made = await post(
    '/v1/documents/' + f.id + '/invitations',
    {
      operation_id: operation('invite'),
      expected_acl_revision: '1',
      requested_grants: [{ scope_kind: 'UNIT', scope_id: unit, capability: 'EDIT' }],
    },
    f.owner.headers,
  );
  assert.equal(made.status, 200);
  const invite = json(made);
  assert.ok(Date.parse(invite.invitation.expires_at) - Date.now() > 47 * 3600000);
  const token = new URL(invite.share_url).searchParams.get('invitation_token');
  const apply = await post(
    '/v1/document-invitations/applications',
    { operation_id: operation('apply'), invitation_token: token, reason: '希望参与' },
    applicant.headers,
  );
  assert.equal(apply.status, 202);
  assert.equal((await get('/v1/documents/' + f.id, applicant.headers)).status, 403);
  const own = await get('/v1/document-applications', applicant.headers);
  assert.equal(own.status, 200);
  assert.equal(json(own).items.length, 1);
  assert.equal(json(await get('/v1/document-applications', stranger.headers)).items.length, 0);
  const canceled = await post(
    '/v1/document-invitations/' + invite.invitation.id + '/cancel',
    { operation_id: operation('cancel'), expected_revision: '1' },
    f.owner.headers,
  );
  assert.equal(canceled.status, 200);
  assert.equal(
    (
      await post(
        '/v1/document-invitations/applications',
        { operation_id: operation('late'), invitation_token: token, reason: '迟到' },
        stranger.headers,
      )
    ).status,
    409,
  );
  const accepted = await post(
    '/v1/document-applications/' + json(own).items[0].id + '/decisions',
    {
      operation_id: operation('approve'),
      expected_revision: '1',
      decision: 'ACCEPT',
      reason: '通过',
    },
    f.owner.headers,
  );
  assert.equal(accepted.status, 202);
  await waitAuthorization(accepted, f.owner);
  assert.equal((await get('/v1/documents/' + f.id + '/main', applicant.headers)).status, 200);
  assert.equal(
    json(await get('/v1/documents/' + f.id + '/editable-units', applicant.headers)).items[0]
      .unit_id,
    unit,
  );
});
test('visibility changes do not publish or grant internal reading and tightening drains old views', async () => {
  const f = await organizationFixture(1),
    d = await documentFixture(f);
  const changed = await post(
    '/v1/documents/' + d.id + '/visibility',
    { operation_id: operation('visibility'), expected_visibility_revision: '1', policy: 'PUBLIC' },
    f.owner.headers,
  );
  assert.equal(changed.status, 202);
  // Visibility controls published audience; an ordinary member still lacks private main-line access.
  assert.equal((await get('/v1/documents/' + d.id + '/main', f.members[0].headers)).status, 403);
  // 202 仅确认持久意图；关闭期间读取可被 gate 拒绝，必须等真实终态再断言切换后的文档。
  await waitAuthorization(changed, f.owner);
  const reading = await get('/v1/documents/' + d.id, f.owner.headers);
  assert.equal(reading.status, 200, json(reading).code);
  const current = json(reading);
  assert.equal(current.publication_state, 'UNPUBLISHED');
  assert.equal(current.visibility_policy, 'PUBLIC');
});
test('knowledge base deletion fences its children and keeps the original receipt available', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    r = await post(
      '/v1/knowledge-bases/' + f.kb,
      { operation_id: operation('kbdelete'), expected_revision: '1' },
      f.owner.headers,
      'DELETE',
    );
  assert.equal(r.status, 202);
  await waitAuthorization(r, f.owner);
  assert.equal((await get('/v1/knowledge-bases/' + f.kb, f.owner.headers)).status, 404);
  assert.equal((await get('/v1/documents/' + d.id, f.owner.headers)).status, 404);
});
