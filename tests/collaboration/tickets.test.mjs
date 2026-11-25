import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
test('an actual one-time ticket only restores its original client and connection and carries complete source epochs', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    unit = d.main.units[0].unit_id,
    client = operation('client');
  const created = await post(
    `/v1/documents/${d.id}/units/${unit}/edit-sessions`,
    {
      operation_id: operation('ticket'),
      workspace_kind: 'BRANCH',
      client_session_id: client,
      expected_structure_id: d.main.structure.id,
    },
    f.owner.headers,
  );
  assert.equal(created.status, 200);
  const session = json(created),
    rpc = documentRPC(),
    input = {
      ticket: Buffer.from(session.ticket),
      client_session_id: client,
      connection_id: operation('connection'),
    };
  try {
    const grant = await rpcCall(rpc, 'RedeemCollaborationTicket', input);
    assert.equal(grant.room_id, session.room_id);
    assert.equal(grant.context.target.unit_id, unit);
    const keys = grant.context.sources.map((s) => s.scope_key);
    for (const scope of [
      'identity:' + f.owner.id,
      'credential:' + f.owner.id,
      'doc-acl:' + d.id,
      'kb-acl:' + f.kb,
    ])
      assert.ok(keys.includes(scope));
    assert.ok(keys.some((s) => s.startsWith('session:')));
    assert.deepEqual(await rpcCall(rpc, 'RedeemCollaborationTicket', input), grant);
    await assert.rejects(
      () =>
        rpcCall(rpc, 'RedeemCollaborationTicket', {
          ...input,
          connection_id: operation('another'),
        }),
      (e) => e.code === 7,
    );
  } finally {
    rpc.close();
  }
});
test('a ticket issued before direct edit revocation cannot be redeemed after the real closure completes', async () => {
  const f = await organizationFixture(1),
    d = await documentFixture(f),
    member = f.members[0],
    unit = d.main.units[0].unit_id;
  const changes = (allowed) =>
    post(
      `/v1/documents/${d.id}/authorization-changes`,
      {
        operation_id: operation('edit-grant'),
        expected_acl_revision: allowed ? '1' : '2',
        changes: [
          {
            grant: { user_id: member.id, scope_kind: 'UNIT', scope_id: unit, capability: 'EDIT' },
            action: allowed ? 'GRANT' : 'REVOKE',
          },
        ],
      },
      f.owner.headers,
    );
  await waitAuthorization(await changes(true), f.owner);
  const client = operation('client'),
    created = await post(
      `/v1/documents/${d.id}/units/${unit}/edit-sessions`,
      {
        operation_id: operation('ticket'),
        workspace_kind: 'BRANCH',
        client_session_id: client,
        expected_structure_id: d.main.structure.id,
      },
      member.headers,
    );
  assert.equal(created.status, 200);
  await waitAuthorization(await changes(false), f.owner);
  const rpc = documentRPC();
  try {
    await assert.rejects(
      () =>
        rpcCall(rpc, 'RedeemCollaborationTicket', {
          ticket: Buffer.from(json(created).ticket),
          client_session_id: client,
          connection_id: operation('connection'),
        }),
      (e) => e.code === 7,
    );
  } finally {
    rpc.close();
  }
});
