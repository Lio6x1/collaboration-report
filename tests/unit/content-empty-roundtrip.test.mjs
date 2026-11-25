import test from 'node:test';
import assert from 'node:assert/strict';
import { prosemirrorJSONToYDoc, yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  schema,
  toPM,
  fromPM,
  normalizeContent,
  canonical,
} from '../../packages/content/index.mjs';

test('empty editable blocks keep canonical bytes through the actual Yjs adapter', () => {
  for (const type of ['paragraph', 'heading', 'code_block']) {
    const attrs = {
      id: 'empty-block',
      ...(type === 'heading' ? { level: 1 } : {}),
      ...(type === 'code_block' ? { language: null } : {}),
    };
    const original = normalizeContent({ type: 'doc', content: [{ type, attrs, content: [] }] });
    const doc = prosemirrorJSONToYDoc(schema, toPM(original));
    try {
      // ProseMirror 省略空 content 是传输细节，不应让未编辑的草稿出现新的正文哈希。
      const restored = normalizeContent(fromPM(yDocToProsemirrorJSON(doc)));
      assert.equal(canonical(restored), canonical(original));
    } finally {
      doc.destroy();
    }
  }
});
