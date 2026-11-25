import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { mergeParagraphFormattingExtension } from '../../scripts/load-content-contract.mjs';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc, yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  schema,
  toPM,
  fromPM,
  normalizeContent,
  canonical,
  prepareCandidate,
} from '../../packages/content/index.mjs';

const readContent = (name) =>
  JSON.parse(readFileSync(new URL(`../../contracts/content/${name}`, import.meta.url)));
const vectors = ['canonical-vectors.json', 'paragraph-formatting-vectors.json'].flatMap(
  (name) => readContent(name).vectors,
);

test('the explicit content extension cannot replace or loosen frozen contract rules', () => {
  const baseline = readContent('content.schema.json');
  const extension = readContent('paragraph-formatting.extension.json');
  const before = structuredClone(baseline);
  const merged = mergeParagraphFormattingExtension(baseline, extension);
  assert.deepEqual(baseline, before);
  for (const name of ['paragraph', 'heading']) {
    const originalAttrs = baseline.$defs[name].properties.attrs;
    const mergedAttrs = merged.$defs[name].properties.attrs;
    assert.equal(Object.hasOwn(originalAttrs.properties, 'align'), false);
    assert.equal(mergedAttrs.additionalProperties, false);
    assert.deepEqual(mergedAttrs.required, originalAttrs.required);
    assert.deepEqual(mergedAttrs.properties.id, originalAttrs.properties.id);
  }
  for (const invalid of [
    { ...extension, nodes: ['paragraph', 'code_block'] },
    { ...extension, required: ['align'] },
    { ...extension, attributes: { ...extension.attributes, id: { type: 'string' } } },
    { ...extension, attributes: { ...extension.attributes, align: { type: 'string' } } },
  ])
    assert.throws(() => mergeParagraphFormattingExtension(baseline, invalid), /EXTENSION_INVALID/);
  const alreadyExtended = structuredClone(baseline);
  alreadyExtended.$defs.paragraph.properties.attrs.properties.align = { type: 'string' };
  assert.throws(
    () => mergeParagraphFormattingExtension(alreadyExtended, extension),
    /EXTENSION_OVERRIDE/,
  );
});

test('old hashes stay fixed and new paragraph formatting survives the actual PM/Yjs roundtrip', () => {
  for (const vector of vectors) {
    const body = normalizeContent(vector.input);
    assert.equal(canonical(body), vector.canonical_utf8, vector.name);
    const doc = prosemirrorJSONToYDoc(schema, toPM(body));
    try {
      const restored = normalizeContent(fromPM(yDocToProsemirrorJSON(doc)));
      assert.equal(canonical(restored), vector.canonical_utf8, vector.name);
      assert.equal(createHash('sha256').update(canonical(restored)).digest('hex'), vector.sha256);
    } finally {
      doc.destroy();
    }
  }
});

test('paragraph defaults render as CSS but stay absent from normalized business content', () => {
  for (const type of ['paragraph', 'heading']) {
    const attrs = { id: 'p', ...(type === 'heading' ? { level: 2 } : {}) };
    const body = { type: 'doc', content: [{ type, attrs, content: [] }] };
    assert.deepEqual(normalizeContent(body), body);
    const pm = schema.nodeFromJSON(toPM(body));
    assert.equal(pm.firstChild.attrs.align, 'left');
    assert.equal(pm.firstChild.attrs.firstLineIndent, false);
    assert.deepEqual(fromPM(pm.toJSON()), body);
    const styled = pm.firstChild.type.create({ ...attrs, align: 'right', firstLineIndent: true });
    assert.equal(styled.type.spec.toDOM(styled)[1].style, 'text-align:right;text-indent:2em');
    // 受控DOM往返是剪贴板入口，无需启动浏览器也能验证格式没有被parseDOM丢弃。
    const parsed = styled.type.spec.parseDOM[0].getAttrs({
      style: { textAlign: 'center', textIndent: '2em' },
    });
    assert.equal(parsed.align, 'center');
    assert.equal(parsed.firstLineIndent, true);
  }
});

test('admitted paragraph format updates persist while unknown or invalid attributes remain rejected', () => {
  const original = {
    type: 'doc',
    content: [{ type: 'paragraph', attrs: { id: 'p' }, content: [] }],
  };
  const host = prosemirrorJSONToYDoc(schema, toPM(original));
  const client = new Y.Doc();
  Y.applyUpdate(client, Y.encodeStateAsUpdate(host));
  client.getXmlFragment('prosemirror').get(0).setAttribute('align', 'center');
  client.getXmlFragment('prosemirror').get(0).setAttribute('firstLineIndent', true);
  const candidate = prepareCandidate(
    host,
    Y.encodeStateAsUpdate(client, Y.encodeStateVector(host)),
  );
  try {
    assert.deepEqual(candidate.body.content[0].attrs, {
      id: 'p',
      align: 'center',
      firstLineIndent: true,
    });
    for (const attrs of [
      { align: 'justify' },
      { align: 'center;color:red' },
      { firstLineIndent: 2 },
      { firstLineIndent: 'true' },
      { unknown: true },
    ]) {
      const invalid = structuredClone(original);
      Object.assign(invalid.content[0].attrs, attrs);
      assert.throws(() => normalizeContent(invalid), { code: 'INVALID_CONTENT' });
    }
  } finally {
    candidate.doc.destroy();
    client.destroy();
    host.destroy();
  }
});
