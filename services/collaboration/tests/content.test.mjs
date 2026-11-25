import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc, yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  schema,
  normalizeContent,
  canonical,
  prepareCandidate,
  toPM,
  fromPM,
} from '../../../packages/content/index.mjs';

const paragraph = (id, text) => ({
  type: 'paragraph',
  attrs: { id },
  content: text ? [{ type: 'text', text }] : [],
});
const document = (...content) => ({ type: 'doc', content });
// 预期字节来自已定稿的跨语言向量，不从被测规范化函数反推。
const vectors = JSON.parse(
  readFileSync(new URL('../../../contracts/content/canonical-vectors.json', import.meta.url)),
).vectors;
for (const vector of vectors)
  test(`fixed canonical and PM/Yjs roundtrip preserve ${vector.name}`, () => {
    const body = normalizeContent(vector.input);
    assert.equal(canonical(body), vector.canonical_utf8);
    assert.equal(createHash('sha256').update(canonical(body)).digest('hex'), vector.sha256);
    assert.equal(Buffer.byteLength(canonical(body)), vector.byte_size);
    const y = prosemirrorJSONToYDoc(schema, toPM(body));
    try {
      assert.deepEqual(normalizeContent(fromPM(yDocToProsemirrorJSON(y))), body);
    } finally {
      y.destroy();
    }
  });

test('an isolated rejected candidate cannot install unknown nodes or duplicate block IDs in the shared document', () => {
  const host = prosemirrorJSONToYDoc(schema, toPM(document(paragraph('p1', '已保存'))));
  const candidate = new Y.Doc();
  Y.applyUpdate(candidate, Y.encodeStateAsUpdate(host));
  const before = Y.encodeStateAsUpdate(host);
  const xml = new Y.XmlElement('paragraph');
  xml.setAttribute('id', 'p1');
  candidate.getXmlFragment('prosemirror').push([xml]);
  const update = Y.encodeStateAsUpdate(candidate, Y.encodeStateVector(host));
  assert.throws(() => prepareCandidate(host, update), { code: 'INVALID_CONTENT' });
  assert.deepEqual(Y.encodeStateAsUpdate(host), before);
  host.destroy();
  candidate.destroy();
});
test('branch font, unsafe links, unknown attributes and malformed table geometry are rejected before admission', () => {
  for (const bad of [
    document(paragraph('p', 'a'), paragraph('p', 'b')),
    document({
      ...paragraph('p', '字'),
      content: [
        { type: 'text', text: '字', marks: [{ type: 'font', attrs: { family: 'SimSun' } }] },
      ],
    }),
    document({
      ...paragraph('p', '字'),
      content: [
        {
          type: 'text',
          text: '字',
          marks: [{ type: 'link', attrs: { href: 'javascript:alert(1)' } }],
        },
      ],
    }),
    document({ ...paragraph('p', '字'), attrs: { id: 'p', secret: '不应放行' } }),
    document({
      type: 'table',
      attrs: { id: 't' },
      content: [
        {
          type: 'table_row',
          attrs: { id: 'r' },
          content: [
            {
              type: 'table_cell',
              attrs: { id: 'c', colspan: 1, rowspan: 2, colwidth: null },
              content: [paragraph('p', '格')],
            },
          ],
        },
      ],
    }),
  ])
    assert.throws(() => normalizeContent(bad), { code: 'INVALID_CONTENT' });
});
test('text whitespace is preserved while unpaired Unicode surrogate and noninteger content are rejected', () => {
  const body = document(paragraph('p', ' 中😀  \n<&> '));
  assert.deepEqual(normalizeContent(body), body);
  assert.throws(() => normalizeContent(document(paragraph('p', '\ud800'))), {
    code: 'INVALID_CONTENT',
  });
  assert.throws(() => canonical({ dimension: 1.5 }), { code: 'INVALID_CONTENT' });
});
test('a pure deletion update changes accepted content despite an unchanged state vector', () => {
  const host = prosemirrorJSONToYDoc(schema, toPM(document(paragraph('p', '只删除'))));
  const client = new Y.Doc();
  Y.applyUpdate(client, Y.encodeStateAsUpdate(host));
  const vector = Y.encodeStateVector(client);
  client.getXmlFragment('prosemirror').get(0).get(0).delete(0, 3);
  assert.deepEqual(Y.encodeStateVector(client), vector);
  const update = Y.encodeStateAsUpdate(client, vector);
  const accepted = prepareCandidate(host, update);
  try {
    assert.deepEqual(accepted.body, document(paragraph('p', '')));
  } finally {
    accepted.doc?.destroy();
    host.destroy();
    client.destroy();
  }
});
