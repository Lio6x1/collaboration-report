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
  waitAuthorization,
  get,
} from '../helpers/workspace.mjs';
import { wire } from '../helpers/wss.mjs';

test('the actual edge WSS stores an update before SAVED, deduplicates bytes, and flushes pure deletion by update identity', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    unit = d.main.units[0].unit_id,
    client = operation('client'),
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
  const w = wire(json(r), client, operation('connection')),
    local = new Y.Doc();
  try {
    const ready = await w.ready;
    assert.equal(ready.protocol, 'collab-wss-v1');
    Y.applyUpdate(local, Buffer.from(ready.update, 'base64'));
    const before = Y.encodeStateVector(local),
      fragment = local.getXmlFragment('prosemirror'),
      paragraph = fragment.get(0);
    let text = paragraph.get(0);
    if (!text) {
      text = new Y.XmlText();
      paragraph.push([text]);
    }
    text.insert(0, '共同编辑');
    const bytes = Buffer.from(Y.encodeStateAsUpdate(local, before)),
      id = operation('update'),
      hash = createHash('sha256').update(bytes).digest('hex');
    const input = { type: 'UPDATE', client_update_id: id, update: bytes.toString('base64') };
    w.send(input);
    const saved = await w.next('SAVED');
    assert.equal(saved.client_update_id, id);
    assert.equal(saved.update_hash, hash);
    assert.equal(saved.durable_seq, '1');
    w.send(input);
    assert.deepEqual(await w.next('SAVED'), saved);
    w.send({ ...input, update: Buffer.from([0, 0]).toString('base64') });
    assert.equal((await w.next('ERROR')).code, 'UPDATE_ID_CONFLICT');
    const vector = Y.encodeStateVector(local);
    text.delete(0, 4);
    assert.deepEqual(Y.encodeStateVector(local), vector);
    const deletion = Buffer.from(Y.encodeStateAsUpdate(local, vector)),
      deleteId = operation('delete'),
      deleteHash = createHash('sha256').update(deletion).digest('hex');
    w.send({
      type: 'FLUSH',
      operation_id: operation('flush-wait'),
      dependencies: [
        { client_session_id: client, client_update_id: deleteId, update_hash: deleteHash },
      ],
    });
    assert.equal((await w.next('WAITING')).code, 'DEPENDENCIES_MISSING');
    w.send({ type: 'UPDATE', client_update_id: deleteId, update: deletion.toString('base64') });
    assert.equal((await w.next('SAVED')).durable_seq, '2');
    const flush = {
      type: 'FLUSH',
      operation_id: operation('flush'),
      dependencies: [
        { client_session_id: client, client_update_id: deleteId, update_hash: deleteHash },
      ],
    };
    w.send(flush);
    const receipt = await w.next('FLUSHED');
    assert.equal(receipt.covered_cut.durable_seq, '2');
    w.send(flush);
    assert.deepEqual(await w.next('FLUSHED'), receipt);
  } finally {
    w.close();
    local.destroy();
  }
});

async function edited(f, d, user = f.owner) {
  const client = operation('client'),
    r = await post(
      `/v1/documents/${d.id}/units/${d.main.units[0].unit_id}/edit-sessions`,
      {
        operation_id: operation('edit'),
        client_session_id: client,
        workspace_kind: 'BRANCH',
        expected_structure_id: d.main.structure.id,
      },
      user.headers,
    );
  assert.equal(r.status, 200);
  const session = json(r),
    w = wire(session, client, operation('connection'));
  return { w, session, client, ready: await w.ready };
}
const privatePool = () =>
  mysql.createPool(
    JSON.parse(readFileSync(resolve(runtime, 'services/collaboration.json'))).database,
  );
test('actual room row lock prevents SAVED until commit and a rejected candidate leaves durable SYNC unchanged', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    { w, session, ready } = await edited(f, d),
    local = new Y.Doc(),
    pool = privatePool(),
    c = await pool.getConnection();
  try {
    Y.applyUpdate(local, Buffer.from(ready.update, 'base64'));
    const vector = Y.encodeStateVector(local),
      p = local.getXmlFragment('prosemirror').get(0),
      text = new Y.XmlText();
    p.push([text]);
    text.insert(0, '事务边界');
    const update = Buffer.from(Y.encodeStateAsUpdate(local, vector)),
      id = operation('durable');
    await c.query('START TRANSACTION');
    await c.query('SELECT id FROM room WHERE id=? FOR UPDATE', [session.room_id]);
    w.send({ type: 'UPDATE', client_update_id: id, update: update.toString('base64') });
    // PONG证明同一TCP上的前一帧已到服务端；实际房间行锁仍挡住日志提交。
    const pong = new Promise((r) => w.socket.once('pong', r));
    w.socket.ping('commit-barrier');
    await pong;
    await new Promise((r) => setTimeout(r, 100));
    assert.equal(
      w.messages.some((m) => m.type === 'SAVED'),
      false,
    );
    await c.query('COMMIT');
    const saved = await w.next('SAVED');
    const [[durable]] = await pool.query(
      'SELECT payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND seq=?',
      [session.room_id, session.workspace_epoch, saved.durable_seq],
    );
    assert.deepEqual(durable.payload, update);
    assert.equal(durable.payload_hash.toString('hex'), saved.update_hash);
    const invalid = new Y.Doc();
    Y.applyUpdate(invalid, Y.encodeStateAsUpdate(local));
    const bad = new Y.XmlElement('paragraph');
    bad.setAttribute('id', p.getAttribute('id'));
    invalid.getXmlFragment('prosemirror').push([bad]);
    w.send({
      type: 'UPDATE',
      client_update_id: operation('invalid'),
      update: Buffer.from(Y.encodeStateAsUpdate(invalid, Y.encodeStateVector(local))).toString(
        'base64',
      ),
    });
    assert.equal((await w.next('ERROR')).code, 'INVALID_CONTENT');
    invalid.destroy();
    w.send({ type: 'SYNC' });
    const synced = await w.next('SYNC');
    assert.equal(synced.durable_seq, saved.durable_seq);
    const actual = new Y.Doc();
    Y.applyUpdate(actual, Buffer.from(synced.update, 'base64'));
    assert.equal(actual.getXmlFragment('prosemirror').length, 1);
    assert.equal(actual.getXmlFragment('prosemirror').get(0).get(0).toString(), '事务边界');
    actual.destroy();
  } finally {
    try {
      await c.query('ROLLBACK');
    } catch {}
    c.release();
    await pool.end();
    w.close();
    local.destroy();
  }
});

test('a completed real edit revocation has already closed the member browser socket and a fresh higher epoch still opens', async () => {
  const f = await organizationFixture(1),
    d = await documentFixture(f),
    member = f.members[0],
    unit = d.main.units[0].unit_id;
  const acl = async (action, revision) =>
    waitAuthorization(
      await post(
        `/v1/documents/${d.id}/authorization-changes`,
        {
          operation_id: operation('acl'),
          expected_acl_revision: revision,
          changes: [
            {
              grant: { user_id: member.id, scope_kind: 'UNIT', scope_id: unit, capability: 'EDIT' },
              action,
            },
          ],
        },
        f.owner.headers,
      ),
      f.owner,
    );
  await acl('GRANT', '1');
  const { w } = await edited(f, d, member);
  let closed = false;
  w.socket.once('close', () => {
    closed = true;
  });
  try {
    await acl('REVOKE', '2');
    assert.equal(closed, true);
    await acl('GRANT', '3');
    const newer = await edited(f, d, member);
    try {
      assert.equal(newer.ready.type, 'READY');
    } finally {
      newer.w.close();
    }
  } finally {
    w.close();
  }
});
