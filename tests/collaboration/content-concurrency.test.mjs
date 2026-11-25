import test from 'node:test';
import assert from 'node:assert/strict';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc, yDocToProsemirrorJSON } from 'y-prosemirror';
import { schema, toPM, fromPM, normalizeContent } from '../../packages/content/index.mjs';
import {
  organizationFixture,
  documentFixture,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { wire } from '../helpers/wss.mjs';
async function open(f, d) {
  const client = operation('client'),
    r = await post(
      `/v1/documents/${d.id}/units/${d.main.units[0].unit_id}/edit-sessions`,
      {
        operation_id: operation('edit'),
        workspace_kind: 'BRANCH',
        client_session_id: client,
        expected_structure_id: d.main.structure.id,
      },
      f.owner.headers,
    );
  assert.equal(r.status, 200);
  const w = wire(json(r), client, operation('connection')),
    doc = new Y.Doc();
  Y.applyUpdate(doc, Buffer.from((await w.ready).update, 'base64'));
  return { w, doc };
}
async function seed(a, body) {
  const before = Y.encodeStateVector(a.doc),
    source = prosemirrorJSONToYDoc(schema, toPM(body)),
    root = a.doc.getXmlFragment('prosemirror');
  root.delete(0, root.length);
  root.insert(
    0,
    source
      .getXmlFragment('prosemirror')
      .toArray()
      .map((n) => n.clone()),
  );
  source.destroy();
  a.w.send({
    type: 'UPDATE',
    client_update_id: operation('seed'),
    update: Buffer.from(Y.encodeStateAsUpdate(a.doc, before)).toString('base64'),
  });
  await a.w.next('SAVED');
}
async function send(a, vector) {
  a.w.send({
    type: 'UPDATE',
    client_update_id: operation('update'),
    update: Buffer.from(Y.encodeStateAsUpdate(a.doc, vector)).toString('base64'),
  });
  await a.w.next('SAVED');
}
async function current(a) {
  a.w.send({ type: 'SYNC' });
  const doc = new Y.Doc();
  try {
    Y.applyUpdate(doc, Buffer.from((await a.w.next('SYNC')).update, 'base64'));
    return normalizeContent(fromPM(yDocToProsemirrorJSON(doc)));
  } finally {
    doc.destroy();
  }
}
test('actual independent atom fields merge through durable WSS without overwriting a concurrent format', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    a = await open(f, d);
  let b;
  try {
    await seed(a, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'atom-p' },
          content: [{ type: 'formula', attrs: { latex: 'x^2' } }],
        },
      ],
    });
    b = await open(f, d);
    const av = Y.encodeStateVector(a.doc),
      bv = Y.encodeStateVector(b.doc);
    a.doc.getXmlFragment('prosemirror').get(0).get(0).setAttribute('atomStrong', true);
    b.doc.getXmlFragment('prosemirror').get(0).get(0).setAttribute('atomEm', true);
    await Promise.all([send(a, av), send(b, bv)]);
    const atom = (await current(a)).content[0].content[0];
    assert.equal(atom.attrs.atomStrong, true);
    assert.equal(atom.attrs.atomEm, true);
  } finally {
    a.w.close();
    a.doc.destroy();
    b?.w.close();
    b?.doc.destroy();
  }
});
test('actual table row deletion wins over an offline edit in that row while retaining the other row identities', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    a = await open(f, d);
  let b;
  const cell = (id, text) => ({
      type: 'table_cell',
      attrs: { id, colspan: 1, rowspan: 1, colwidth: null },
      content: [{ type: 'paragraph', attrs: { id: id + '-p' }, content: [{ type: 'text', text }] }],
    }),
    row = (id) => ({
      type: 'table_row',
      attrs: { id },
      content: [cell(id + '-a', '左'), cell(id + '-b', '右')],
    });
  try {
    await seed(a, {
      type: 'doc',
      content: [
        { type: 'table', attrs: { id: 'table' }, content: [row('deleted'), row('retained')] },
      ],
    });
    b = await open(f, d);
    const av = Y.encodeStateVector(a.doc),
      bv = Y.encodeStateVector(b.doc);
    a.doc.getXmlFragment('prosemirror').get(0).delete(0, 1);
    b.doc.getXmlFragment('prosemirror').get(0).get(0).get(0).get(0).get(0).insert(0, '离线改动');
    await send(a, av);
    await send(b, bv);
    const table = (await current(a)).content[0];
    assert.equal(table.content.length, 1);
    assert.equal(table.content[0].attrs.id, 'retained');
    assert.equal(table.content[0].content[0].attrs.id, 'retained-a');
    assert.equal(table.content[0].content[0].content[0].content[0].text, '左');
  } finally {
    a.w.close();
    a.doc.destroy();
    b?.w.close();
    b?.doc.destroy();
  }
});
