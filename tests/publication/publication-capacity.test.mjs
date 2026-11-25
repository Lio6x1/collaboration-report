import test from 'node:test';
import assert from 'node:assert/strict';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { schema, toPM, canonical, fromPM } from '../../packages/content/index.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { reviewFixture, reviewDatabase, openReviewWorkspace } from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';

async function appendPart(main, unit, part) {
  const before = Y.encodeStateVector(main.doc);
  const source = prosemirrorJSONToYDoc(
    schema,
    toPM({
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: `publication-capacity-${unit}-${part}` },
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
    const update = Y.encodeStateAsUpdate(main.doc, before);
    assert.ok(update.byteLength < 1 << 20, 'EACH_ACTUAL_UPDATE_MUST_FIT_WSS_BUDGET');
    main.w.send({
      type: 'UPDATE',
      client_update_id: operation('publication-capacity-part'),
      update: Buffer.from(update).toString('base64'),
    });
    const saved = await main.w.next('SAVED');
    assert.equal(saved.durable_seq, String(part + 1));
  } finally {
    source.destroy();
  }
}

async function populateUnit(f, view, unit, start, end, diagnostic) {
  const document = { id: f.d.id, main: { ...view, units: [unit] } };
  let main;
  try {
    for (let part = start; part < end; part++) {
      // 每10个真实提交重新打开当前编辑准入；原票据/准入期限不延长，不回填时间或SQL伪造SAVED。
      if (!main || part % 10 === 0) {
        main?.close();
        main = await openReviewWorkspace(f.f, document, f.user, 'MAIN');
        diagnostic(`CAPACITY_EDIT_OPEN part=${part}`);
      }
      await appendPart(main, unit.unit_id, part);
    }
    const body = canonical(fromPM(yDocToProsemirrorJSON(main.doc)));
    return { body, bytes: Buffer.byteLength(body) };
  } finally {
    main?.close();
  }
}

test(
  'actual reliable multi-unit MAIN publishes more than 19MiB and rejects more than 24MiB without partial release',
  { timeout: 300000 },
  async (t) => {
    const f = await reviewFixture(),
      db = await reviewDatabase(),
      node = await reviewDatabase('collaboration');
    try {
      await unlockStructurePreview(f);
      const metadata = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      await completedMainControl(
        await post(
          `/v1/documents/${f.d.id}/chapters`,
          {
            operation_id: operation('publication-capacity-chapter'),
            expected_structure_id: metadata.current_structure_id,
            title: '整篇容量的第二个实际单元',
          },
          f.user.headers,
        ),
        f.user,
      );
      const response = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
      assert.equal(response.status, 200, json(response).code);
      const view = json(response);
      assert.equal(view.units.length, 2);
      t.diagnostic('CAPACITY_MAIN_READY');
      const expected = [];
      for (const unit of view.units) {
        expected.push(await populateUnit(f, view, unit, 0, 17, (message) => t.diagnostic(message)));
        t.diagnostic(`CAPACITY_UNIT_SAVED count=${expected.length}`);
      }
      assert.ok(expected.reduce((total, unit) => total + unit.bytes, 0) > 19 << 20);
      t.diagnostic('CAPACITY_POSITIVE_PUBLICATION_START');
      const first = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
      t.diagnostic('CAPACITY_POSITIVE_PUBLICATION_SUCCEEDED');
      const [versions] = await db.query(
        'SELECT CAST(e.unit_id AS CHAR) unit_id,CAST(v.byte_size AS CHAR) bytes,v.canonical_body FROM `release` r JOIN manifest_entry e ON e.manifest_id=r.manifest_id JOIN content_version v ON v.id=e.content_version_id WHERE r.id=? ORDER BY e.unit_id',
        [first.result.entity_id],
      );
      assert.equal(versions.length, 2);
      for (let index = 0; index < versions.length; index++) {
        assert.equal(versions[index].unit_id, view.units[index].unit_id);
        assert.equal(versions[index].bytes, String(expected[index].bytes));
        assert.ok(
          versions[index].canonical_body.equals(Buffer.from(expected[index].body)),
          'COMPLETE_ACTUAL_MULTI_UNIT_RELEASE_REQUIRED',
        );
      }
      const enlarged = [];
      for (const unit of view.units) {
        enlarged.push(
          await populateUnit(f, view, unit, 17, 22, (message) => t.diagnostic(message)),
        );
        t.diagnostic(`CAPACITY_UNIT_ENLARGED count=${enlarged.length}`);
      }
      assert.ok(enlarged.every((unit) => unit.bytes < 16 << 20));
      assert.ok(enlarged.reduce((total, unit) => total + unit.bytes, 0) > 24 << 20);
      const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      const input = await releaseInput(f.d.id, f.user);
      const made = await post(`/v1/documents/${f.d.id}/releases`, input, f.user.headers);
      t.diagnostic('CAPACITY_NEGATIVE_PUBLICATION_REGISTERED');
      assert.equal(made.status, 202, json(made).code);
      const rejected = await pollPublication(input.operation_id, f.user);
      assert.equal(rejected.state, 'REJECTED');
      assert.equal(rejected.error.code, 'PUBLICATION_TRANSFER_LIMIT');
      const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      for (const field of ['current_release_id', 'publication_epoch', 'current_manifest_id'])
        assert.equal(after[field], before[field]);
      const [[counts]] = await db.query(
        'SELECT (SELECT COUNT(*) FROM `release` WHERE operation_id=?) releases,(SELECT COUNT(*) FROM main_manifest WHERE source_operation_id=?) manifests,(SELECT COUNT(*) FROM content_version WHERE source_operation_id=?) versions,(SELECT COUNT(*) FROM outbox WHERE event_id=?) events',
        Array(4).fill(input.operation_id),
      );
      for (const count of Object.values(counts)) assert.equal(Number(count), 0);
      const [[capture]] = await node.query(
        "SELECT COUNT(*) n FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND business_key=?",
        [input.operation_id + ':RELEASE_CREATE:1'],
      );
      assert.equal(Number(capture.n), 0, 'OVERSIZE_CAPTURE_TRANSACTION_MUST_ROLL_BACK');
    } catch (error) {
      if (error.cause?.operationId)
        t.diagnostic(
          'CAPACITY_TRANSPORT ' +
            JSON.stringify({
              operation: error.cause.operationId,
              method: error.cause.method,
              receivedHeaders: error.cause.receivedHeaders,
              elapsedMs: error.cause.elapsedMs,
              timeoutMs: error.cause.timeoutMs,
            }),
        );
      throw error;
    } finally {
      f.close();
      await node.end();
      await db.end();
    }
  },
);
