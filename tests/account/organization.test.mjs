import test from 'node:test';
import assert from 'node:assert/strict';
import { createUser, post, get, json, operation, waitOperation } from '../helpers/account.mjs';
async function organization(owner) {
  const r = await post(
    '/v1/organizations',
    { operation_id: operation('org'), name: '校招协作组' },
    owner.headers,
  );
  assert.equal(r.status, 201);
  return json(r).result.entity_id;
}
async function apply(id, user) {
  const r = await post(
    '/v1/organizations/' + id + '/applications',
    { operation_id: operation('apply'), reason: '加入项目学习' },
    user.headers,
  );
  assert.equal(r.status, 200);
  return json(r).result.entity_id;
}
async function accept(id, owner) {
  const r = await post(
    '/v1/applications/' + id + '/decisions',
    {
      operation_id: operation('accept'),
      expected_revision: '1',
      decision: 'ACCEPT',
      reason: '通过',
    },
    owner.headers,
  );
  assert.equal(r.status, 200);
}
test('organization creates one owner, only owner appoints admins, and transfer needs a current member', async () => {
  const owner = await createUser(),
    member = await createUser(),
    outsider = await createUser(),
    id = await organization(owner);
  await accept(await apply(id, member), owner);
  assert.equal((await get('/v1/organizations/' + id, outsider.headers)).status, 404);
  const role = await post(
    '/v1/organizations/' + id + '/members/' + member.id + '/role',
    { operation_id: operation('role'), expected_revision: '1', role: 'ADMIN' },
    owner.headers,
  );
  await waitOperation(role, owner);
  assert.equal(
    (
      await post(
        '/v1/organizations/' + id + '/members/' + owner.id + '/role',
        { operation_id: operation('role'), expected_revision: '1', role: 'MEMBER' },
        member.headers,
      )
    ).status,
    403,
  );
  assert.equal(
    (
      await post(
        '/v1/organizations/' + id + '/members/' + owner.id,
        { operation_id: operation('remove'), expected_revision: '1' },
        member.headers,
        'DELETE',
      )
    ).status,
    403,
  );
  const org = json(await get('/v1/organizations/' + id, owner.headers));
  assert.equal(
    (
      await post(
        '/v1/organizations/' + id + '/owner-transfers',
        {
          operation_id: operation('transfer'),
          expected_revision: org.revision,
          target_user_id: outsider.id,
        },
        owner.headers,
      )
    ).status,
    409,
  );
  await waitOperation(
    await post(
      '/v1/organizations/' + id + '/owner-transfers',
      {
        operation_id: operation('transfer'),
        expected_revision: org.revision,
        target_user_id: member.id,
      },
      owner.headers,
    ),
    owner,
  );
  assert.equal(json(await get('/v1/organizations/' + id, member.headers)).owner_id, member.id);
});
test('invitation means application only, cancellation after application does not invalidate approval', async () => {
  const owner = await createUser(),
    applicant = await createUser(),
    id = await organization(owner),
    org = json(await get('/v1/organizations/' + id, owner.headers));
  const r = await post(
    '/v1/organizations/' + id + '/invitations',
    {
      operation_id: operation('invite'),
      expected_member_revision: org.member_revision,
      expires_in_hours: 48,
    },
    owner.headers,
  );
  assert.equal(r.status, 200);
  const invitation = json(r),
    token = new URL(invitation.share_url).searchParams.get('invitation_token');
  assert.ok(token);
  const accepted = await post(
    '/v1/invitations/accept',
    { operation_id: operation('link'), invitation_token: token, reason: '受邀申请' },
    applicant.headers,
  );
  assert.equal(accepted.status, 200);
  assert.equal((await get('/v1/organizations/' + id, applicant.headers)).status, 404);
  assert.equal(
    (
      await post(
        '/v1/invitations/' + invitation.invitation.id + '/cancel',
        { operation_id: operation('cancel'), expected_revision: '1' },
        owner.headers,
      )
    ).status,
    200,
  );
  await accept(json(accepted).result.entity_id, owner);
  assert.equal((await get('/v1/organizations/' + id, applicant.headers)).status, 200);
  assert.equal(
    (await get('/v1/organizations/' + id + '/invitations', applicant.headers)).status,
    403,
  );
  assert.equal(
    (await get('/v1/organizations/' + id + '/applications', applicant.headers)).status,
    403,
  );
});
test('member removal closes old identity sources and creation grants do not become organization admin', async () => {
  const owner = await createUser(),
    member = await createUser(),
    id = await organization(owner);
  await accept(await apply(id, member), owner);
  let org = json(await get('/v1/organizations/' + id, owner.headers));
  assert.equal(
    (
      await post(
        '/v1/organizations/' + id + '/creation-grants',
        {
          operation_id: operation('grant'),
          expected_revision: org.member_revision,
          user_id: member.id,
          allowed: true,
        },
        owner.headers,
      )
    ).status,
    200,
  );
  assert.equal(
    (
      await post(
        '/v1/organizations/' + id + '/creation-grants',
        {
          operation_id: operation('grant'),
          expected_revision: org.member_revision,
          user_id: owner.id,
          allowed: true,
        },
        member.headers,
      )
    ).status,
    403,
  );
  await waitOperation(
    await post(
      '/v1/organizations/' + id + '/members/' + member.id,
      { operation_id: operation('remove'), expected_revision: '1' },
      owner.headers,
      'DELETE',
    ),
    owner,
  );
  assert.equal((await get('/v1/organizations/' + id, member.headers)).status, 404);
  assert.equal((await get('/v1/account/applications', member.headers)).status, 200);
});
