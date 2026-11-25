import test from 'node:test';
import assert from 'node:assert/strict';
import * as Y from 'yjs';
import { createHash } from 'node:crypto';
import {
  organizationFixture,
  documentFixture,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import { wire } from '../helpers/wss.mjs';

test('a confirmed Doc protection with no Node log cannot block the same pending UPDATE after a fresh grant', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    client = operation('client'),
    unit = d.main.units[0].unit_id,
    rpc = documentRPC();
  let old, newer;
  async function mint() {
    const r = await post(
      `/v1/documents/${d.id}/units/${unit}/edit-sessions`,
      {
        operation_id: operation('edit'),
        workspace_kind: 'BRANCH',
        client_session_id: client,
        expected_structure_id: d.main.structure.id,
      },
      f.owner.headers,
    );
    assert.equal(r.status, 200);
    return json(r);
  }
  try {
    const session = await mint(),
      connection = operation('connection');
    old = wire(session, client, connection);
    const seed = Buffer.from((await old.ready).update, 'base64'),
      doc = new Y.Doc();
    Y.applyUpdate(doc, seed);
    const vector = Y.encodeStateVector(doc),
      text = new Y.XmlText();
    doc.getXmlFragment('prosemirror').get(0).push([text]);
    text.insert(0, '跨grant重试');
    const update = Buffer.from(Y.encodeStateAsUpdate(doc, vector));
    doc.destroy();
    const id = operation('pending'),
      hash = createHash('sha256').update(update).digest(),
      grant = await rpcCall(rpc, 'RedeemCollaborationTicket', {
        ticket: Buffer.from(session.ticket),
        client_session_id: client,
        connection_id: connection,
      }),
      protectionId = createHash('sha256')
        .update([session.room_id, session.workspace_epoch, client, id, grant.grant_id].join('\n'))
        .digest('hex');
    // 真实Doc保护已提交，Node从未收到此UPDATE。相当于两服务之间掉线的可证明停点。
    assert.equal(
      (
        await rpcCall(rpc, 'ProtectDraftResources', {
          context: { ...grant.context, operation_id: protectionId, request_fingerprint: hash },
          grant_id: grant.grant_id,
          room_id: session.room_id,
          workspace_epoch: session.workspace_epoch,
          generation: '1',
          resource_ids: [],
          update_hash: hash,
          client_session_id: client,
          client_update_id: id,
        })
      ).state,
      'PROTECTED',
    );
    old.close();
    const renewed = await mint();
    newer = wire(renewed, client, operation('connection'));
    await newer.ready;
    newer.send({ type: 'UPDATE', client_update_id: id, update: update.toString('base64') });
    assert.equal((await newer.next('SAVED')).durable_seq, '1');
  } finally {
    old?.close();
    newer?.close();
    rpc.close();
  }
});
