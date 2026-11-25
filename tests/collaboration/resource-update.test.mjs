import test from 'node:test';
import assert from 'node:assert/strict';
import * as Y from 'yjs';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import mysql from 'mysql2/promise';
import { runtime } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { wire } from '../helpers/wss.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import { uploadPNG } from '../helpers/resource.mjs';

test('actual image UPDATE protects only same-space READY fixtures and durable proof transfers references before a release tombstone', async () => {
  const f = await organizationFixture(0),
    other = await organizationFixture(0),
    d = await documentFixture(f),
    client = operation('client'),
    connection = operation('connection'),
    unit = d.main.units[0].unit_id,
    r = await post(
      `/v1/documents/${d.id}/units/${unit}/edit-sessions`,
      {
        operation_id: operation('edit'),
        client_session_id: client,
        workspace_kind: 'BRANCH',
        expected_structure_id: d.main.structure.id,
      },
      f.owner.headers,
    );
  assert.equal(r.status, 200);
  const session = json(r),
    w = wire(session, client, connection),
    pool = mysql.createPool(
      JSON.parse(readFileSync(resolve(runtime, 'services/document.json'))).database,
    ),
    rpc = documentRPC();
  async function fixture(space, state) {
    const c = await pool.getConnection();
    try {
      await c.query(
        "INSERT INTO resource(space_id,storage_key,byte_size,media_type,state,upload_operation_id) VALUES(?,?,1,'image/png',?,?)",
        [space, operation('fixture-storage'), state, operation('fixture-upload')],
      );
      const [[row]] = await c.query('SELECT CAST(LAST_INSERT_ID() AS CHAR) AS id');
      return row.id;
    } finally {
      c.release();
    }
  }
  try {
    const seed = Buffer.from((await w.ready).update, 'base64');
    function update(resource) {
      const host = new Y.Doc(),
        candidate = new Y.Doc();
      Y.applyUpdate(host, seed);
      Y.applyUpdate(candidate, seed);
      const image = new Y.XmlElement('image');
      image.setAttribute('resourceId', resource);
      candidate.getXmlFragment('prosemirror').get(0).push([image]);
      const bytes = Buffer.from(Y.encodeStateAsUpdate(candidate, Y.encodeStateVector(host)));
      candidate.destroy();
      host.destroy();
      return bytes;
    }
    const otherDocument = await documentFixture(other);
    // 010起READY还要求真实计费分配。正例及跨空间反例走已有上传流程，
    // 保留原UPLOAD审计；DELETING仅作为明确拒绝状态的私有DB夹具。
    const foreign = await uploadPNG({ f: other, d: otherDocument, user: other.owner });
    for (const id of [foreign, await fixture(f.space, 'DELETING')]) {
      w.send({
        type: 'UPDATE',
        client_update_id: operation('rejected-image'),
        update: update(id).toString('base64'),
      });
      assert.equal((await w.next('ERROR')).code, 'RESOURCE_NOT_READY');
    }
    const resource = await uploadPNG({ f, d, user: f.owner }),
      bytes = update(resource),
      updateId = operation('protected-image'),
      hash = createHash('sha256').update(bytes).digest();
    w.send({ type: 'UPDATE', client_update_id: updateId, update: bytes.toString('base64') });
    const saved = await w.next('SAVED');
    assert.equal(saved.durable_seq, '1');
    const grant = await rpcCall(rpc, 'RedeemCollaborationTicket', {
        ticket: Buffer.from(session.ticket),
        client_session_id: client,
        connection_id: connection,
      }),
      id = createHash('sha256')
        .update(
          [session.room_id, session.workspace_epoch, client, updateId, grant.grant_id].join('\n'),
        )
        .digest('hex'),
      [[fact]] = await pool.query('SELECT state FROM resource_protection WHERE operation_id=?', [
        id,
      ]);
    assert.equal(fact.state, 'CONFIRMED');
    const [refs] = await pool.query(
      "SELECT owner_kind FROM resource_reference WHERE resource_id=? AND purpose<>'UPLOAD'",
      [resource],
    );
    assert.deepEqual(
      refs.map((x) => x.owner_kind),
      ['DRAFT'],
    );
    const context = { ...grant.context, operation_id: id, request_fingerprint: hash },
      resourceDigest = createHash('sha256')
        .update(JSON.stringify([resource]))
        .digest();
    const finalize = {
      query: { context, generation: '1', resource_digest: resourceDigest },
      node_receipt_ref: session.room_id + ':' + session.workspace_epoch + ':' + saved.durable_seq,
      durable_seq: saved.durable_seq,
      update_hash: hash,
      proven_node_outcome: 'COMMITTED',
    };
    assert.equal((await rpcCall(rpc, 'ReleaseDraftResources', finalize)).state, 'RELEASED');
    await assert.rejects(
      () =>
        rpcCall(rpc, 'ProtectDraftResources', {
          context,
          grant_id: grant.grant_id,
          room_id: session.room_id,
          workspace_epoch: session.workspace_epoch,
          generation: '1',
          resource_ids: [resource],
          update_hash: hash,
          client_session_id: client,
          client_update_id: updateId,
        }),
      (e) => e.code === 9,
    );
    const [retained] = await pool.query(
      "SELECT owner_kind FROM resource_reference WHERE resource_id=? AND purpose<>'UPLOAD'",
      [resource],
    );
    assert.deepEqual(
      retained.map((x) => x.owner_kind),
      ['DRAFT'],
    );
    const [[audit]] = await pool.query(
      "SELECT COUNT(*) count FROM resource_reference WHERE resource_id=? AND owner_kind='OPERATION' AND purpose='UPLOAD'",
      [resource],
    );
    assert.equal(Number(audit.count), 1);
  } finally {
    w.close();
    rpc.close();
    await pool.end();
  }
});
