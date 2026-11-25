import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import {
  organizationFixture,
  documentFixture,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';

// 即使候选没有图片，保护协议也应绑定真实grant/update，并保留释放墓碑，不能伪造空成功。
test('actual Doc protection binds even an empty resource set and rejects conflicting update hashes', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    unit = d.main.units[0].unit_id,
    client = operation('client'),
    created = await post(
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
  const rpc = documentRPC();
  try {
    const grant = await rpcCall(rpc, 'RedeemCollaborationTicket', {
        ticket: Buffer.from(json(created).ticket),
        client_session_id: client,
        connection_id: operation('connection'),
      }),
      update = operation('update'),
      digest = createHash('sha256')
        .update(Buffer.from([0, 0]))
        .digest(),
      id = createHash('sha256')
        .update(
          [
            grant.room_id,
            grant.context.target.workspace_epoch,
            client,
            update,
            grant.grant_id,
          ].join('\n'),
        )
        .digest('hex');
    const input = {
      context: { ...grant.context, operation_id: id, request_fingerprint: digest },
      grant_id: grant.grant_id,
      room_id: grant.room_id,
      workspace_epoch: grant.context.target.workspace_epoch,
      generation: '1',
      resource_ids: [],
      update_hash: digest,
      client_session_id: client,
      client_update_id: update,
    };
    const receipt = await rpcCall(rpc, 'ProtectDraftResources', input);
    assert.equal(receipt.state, 'PROTECTED');
    assert.equal(receipt.operation_id, id);
    assert.deepEqual(await rpcCall(rpc, 'ProtectDraftResources', input), receipt);
    await assert.rejects(
      () => rpcCall(rpc, 'ProtectDraftResources', { ...input, update_hash: Buffer.alloc(32, 1) }),
      (e) => e.code === 6 || e.code === 3,
    );
    await assert.rejects(
      () =>
        rpcCall(rpc, 'ConfirmDraftResources', {
          query: {
            context: input.context,
            generation: '1',
            resource_digest: receipt.resource_digest,
          },
          node_receipt_ref: grant.room_id + ':' + input.workspace_epoch + ':1',
          durable_seq: '1',
          update_hash: digest,
          proven_node_outcome: 'COMMITTED',
        }),
      (e) => [9, 14].includes(e.code),
    );
  } finally {
    rpc.close();
  }
});
