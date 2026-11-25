import assert from 'node:assert/strict';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc, yDocToProsemirrorJSON } from 'y-prosemirror';
import { schema, toPM, fromPM, canonical } from '../../packages/content/index.mjs';
import { openReviewWorkspace } from './review.mjs';
import { operation } from './workspace.mjs';

// 容量验收仍经实际WSS/SAVED逐块写入，每个UPDATE小于既有1MiB；不向数据库伪造正文。
export async function populatePublicUnit(fixture, view, unit, parts = 17) {
  let main;
  try {
    for (let part = 0; part < parts; part++) {
      if (!main || part % 10 === 0) {
        main?.close();
        main = await openReviewWorkspace(
          fixture.f,
          { id: fixture.d.id, main: { ...view, units: [unit] } },
          fixture.user,
          'MAIN',
        );
      }
      const vector = Y.encodeStateVector(main.doc);
      const source = prosemirrorJSONToYDoc(
        schema,
        toPM({
          type: 'doc',
          content: [
            {
              type: 'paragraph',
              attrs: { id: `public-capacity-${unit.unit_id}-${part}` },
              content: [{ type: 'text', text: '容'.repeat(200000) }],
            },
          ],
        }),
      );
      try {
        const fragment = main.doc.getXmlFragment('prosemirror');
        fragment.insert(
          fragment.length,
          source
            .getXmlFragment('prosemirror')
            .toArray()
            .map((node) => node.clone()),
        );
        const update = Y.encodeStateAsUpdate(main.doc, vector);
        assert.ok(update.byteLength < 1 << 20);
        main.w.send({
          type: 'UPDATE',
          client_update_id: operation('public-capacity-part'),
          update: Buffer.from(update).toString('base64'),
        });
        assert.equal((await main.w.next('SAVED')).durable_seq, String(part + 1));
      } finally {
        source.destroy();
      }
    }
    return canonical(fromPM(yDocToProsemirrorJSON(main.doc)));
  } finally {
    main?.close();
  }
}
