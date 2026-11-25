import assert from 'node:assert/strict';
import test from 'node:test';
import { performance } from 'node:perf_hooks';
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { schema, toPM, canonical, normalizeContent } from '../../packages/content/index.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { openReviewWorkspace } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { request } from '../helpers/https.mjs';
import { collaborationMetrics } from '../helpers/runtime-metrics.mjs';

function bodyFor(bytes) {
  // 每段约16KiB，分批种子仍遵守单条UPDATE的1MiB上限；容量变化仅来自
  // 正文总大小，账号、房间和追加保存动作保持一致，不展开组合矩阵。
  const text = '真实中文正文与持久保存。'.repeat(400);
  return normalizeContent({
    type: 'doc',
    content: Array.from({ length: Math.ceil(bytes / Buffer.byteLength(text)) }, (_, i) => ({
      type: 'paragraph',
      attrs: { id: 'body-scale-' + i },
      content: [{ type: 'text', text }],
    })),
  });
}
async function update(peer, mutate) {
  const vector = Y.encodeStateVector(peer.doc);
  mutate(peer.doc.getXmlFragment('prosemirror'));
  const id = operation('body-capacity-save');
  const bytes = Y.encodeStateAsUpdate(peer.doc, vector);
  assert.ok(bytes.length <= 1 << 20, 'BODY_CAPACITY_UPDATE_LIMIT');
  const started = performance.now();
  peer.w.send({
    type: 'UPDATE',
    client_update_id: id,
    update: Buffer.from(bytes).toString('base64'),
  });
  const saved = await peer.w.next('SAVED');
  assert.equal(saved.client_update_id, id);
  return { saved, elapsedMs: performance.now() - started };
}
async function seed(peer, body) {
  for (let offset = 0; offset < body.content.length; offset += 12) {
    const source = prosemirrorJSONToYDoc(
      schema,
      toPM({ type: 'doc', content: body.content.slice(offset, offset + 12) }),
    );
    try {
      await update(peer, (fragment) => {
        if (!offset) fragment.delete(0, fragment.length);
        fragment.insert(
          fragment.length,
          source
            .getXmlFragment('prosemirror')
            .toArray()
            .map((node) => node.clone()),
        );
      });
    } finally {
      source.destroy();
    }
  }
}
test(
  'actual 100KiB 1MiB and 5MiB body samples retain reliable fixed bytes and report CPU RSS loop and queue',
  { timeout: 300000 },
  async () => {
    const f = await organizationFixture(0);
    const d = await documentFixture(f);
    const results = [];
    const directory = resolve(root, 'artifacts/batches/08');
    mkdirSync(directory, { recursive: true });
    const evidence = { state: 'RUNNING', independentUsers: 1, measurements: results };
    const record = () =>
      writeFileSync(
        resolve(directory, 'body-capacity-last-attempt.json'),
        JSON.stringify(evidence, null, 2) + '\n',
      );
    let peer;
    try {
      const unauthorized = await request(state().ports.collaborationHttp, '/infra/metrics');
      assert.equal(unauthorized.status, 403, '指标仅接受真实Doc服务证书');
      await unlockStructurePreview({ d, user: f.owner });
      peer = await openReviewWorkspace(f, d, f.owner, 'MAIN');
      for (const nominalBytes of [100 << 10, 1 << 20, 5 << 20]) {
        evidence.currentNominalBytes = nominalBytes;
        record();
        const body = bodyFor(nominalBytes);
        const byteSize = Buffer.byteLength(canonical(body));
        assert.ok(byteSize >= nominalBytes && byteSize < nominalBytes * 1.2);
        const before = await collaborationMetrics();
        const started = performance.now();
        await seed(peer, body);
        const samples = [];
        let latest;
        const markers = [];
        for (let round = 0; round < 5; round++) {
          const marker = `正文规模${nominalBytes}轮${round};`;
          markers.push(marker);
          latest = await update(peer, (fragment) => {
            const text = fragment.get(0).get(0);
            text.insert(text.length, marker);
          });
          samples.push(latest.elapsedMs);
        }
        peer.w.send({
          type: 'FLUSH',
          operation_id: operation('body-capacity-flush'),
          dependencies: [
            {
              client_session_id: latest.saved.client_session_id,
              client_update_id: latest.saved.client_update_id,
              update_hash: latest.saved.update_hash,
            },
          ],
        });
        const flushed = await peer.w.next('FLUSHED');
        assert.ok(BigInt(flushed.covered_cut.durable_seq) >= BigInt(latest.saved.durable_seq));
        const viewResponse = await get(`/v1/documents/${d.id}/main`, f.owner.headers);
        assert.equal(viewResponse.status, 200);
        const view = json(viewResponse);
        const contentResponse = await get(
          `/v1/documents/${d.id}/read-views/${view.read_view_id}/units/${view.units[0].unit_id}`,
          f.owner.headers,
        );
        assert.equal(contentResponse.status, 200);
        const actual = json(contentResponse);
        assert.equal(actual.id, view.units[0].content_version_id);
        assert.equal(actual.body_json.content.length, body.content.length);
        const expected = structuredClone(body);
        expected.content[0].content[0].text += markers.join('');
        assert.deepEqual(actual.body_json, expected, '真实固定正文须完整比对，不能仅查一个标记');
        const after = await collaborationMetrics();
        assert.equal(after.queue.pending, 0, '原FLUSH之后没有遗留房间动作');
        const elapsedMs = performance.now() - started;
        const sorted = [...samples].sort((a, b) => a - b);
        const measurement = {
          nominalBytes,
          canonicalBytes: byteSize,
          seedAndReadMs: Number(elapsedMs.toFixed(2)),
          savedP50Ms: Number(sorted[2].toFixed(2)),
          savedMaxMs: Number(sorted[4].toFixed(2)),
          cpuCorePercent: Number(
            (
              (after.cpuUserMicros +
                after.cpuSystemMicros -
                before.cpuUserMicros -
                before.cpuSystemMicros) /
              (elapsedMs * 10)
            ).toFixed(2),
          ),
          metrics: after,
        };
        results.push(measurement);
        record();
        console.log(JSON.stringify({ event: 'BODY_CAPACITY_STAGE_PASS', ...measurement }));
      }
      evidence.state = 'PASS';
      evidence.completedAt = new Date().toISOString();
      record();
      writeFileSync(
        resolve(directory, 'body-capacity.json'),
        JSON.stringify(evidence, null, 2) + '\n',
      );
    } finally {
      peer?.close();
      if (evidence.state !== 'PASS') {
        evidence.state = 'FAIL';
        record();
      }
    }
  },
);
