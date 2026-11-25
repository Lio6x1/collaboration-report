import test from 'node:test';
import assert from 'node:assert/strict';
import { canonical } from '../../../packages/content/index.mjs';
import { draftLineage } from '../src/domain/draft_protocol.mjs';

test('trusted MAIN lineage projects only font overrides without rewriting original facts', () => {
  const lineage = {
    lineage_id: '10',
    base_manifest_id: '11',
    base_version_id: '12',
    provenance: { accepted_captures: [{ capture_ref: 'original-cut' }] },
    body: {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'font-baseline' },
          content: [
            {
              type: 'text',
              text: '基线',
              marks: [{ type: 'font', attrs: { family: 'SimSun' } }, { type: 'strong' }],
            },
          ],
        },
      ],
    },
  };
  const original = canonical(lineage);
  const projected = draftLineage(lineage);
  assert.equal(canonical(lineage), original);
  assert.deepEqual(projected.body.content[0].content[0].marks, [{ type: 'strong' }]);
  assert.equal(projected.base_version_id, lineage.base_version_id);
  assert.deepEqual(projected.provenance, lineage.provenance);
  assert.deepEqual(draftLineage(projected), projected);
  assert.throws(() => draftLineage({ ...lineage, body: null }));
});
