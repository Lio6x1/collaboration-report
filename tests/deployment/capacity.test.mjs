import assert from 'node:assert/strict';
import test from 'node:test';
import { performance } from 'node:perf_hooks';
import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import * as Y from 'yjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { normalizeContent, fromPM } from '../../packages/content/index.mjs';
import { root, runtime } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { openReviewWorkspace, writeDraftBody } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';

async function sync(peer) {
  peer.w.send({ type: 'SYNC' });
  const response = await peer.w.next('SYNC');
  Y.applyUpdate(peer.doc, Buffer.from(response.update, 'base64'));
  return normalizeContent(fromPM(yDocToProsemirrorJSON(peer.doc)));
}
async function save(peer, marker) {
  const vector = Y.encodeStateVector(peer.doc);
  const text = peer.doc.getXmlFragment('prosemirror').get(0).get(0);
  text.insert(text.length, marker);
  const id = operation('capacity-save');
  const started = performance.now();
  peer.w.send({
    type: 'UPDATE',
    client_update_id: id,
    update: Buffer.from(Y.encodeStateAsUpdate(peer.doc, vector)).toString('base64'),
  });
  const saved = await peer.w.next('SAVED');
  assert.equal(saved.client_update_id, id);
  return { saved, elapsedMs: performance.now() - started };
}
function percentile(values, percentile) {
  const sorted = [...values].sort((a, b) => a - b);
  return Number(sorted[Math.max(0, Math.ceil(sorted.length * percentile) - 1)].toFixed(2));
}
function memory() {
  const registry = JSON.parse(readFileSync(resolve(runtime, 'processes.json')));
  const ids = registry.map((item) => item.pid);
  assert.ok(ids.every((pid) => Number.isInteger(pid) && pid > 0));
  // 仅测本监督器登记的进程；不按名称抓取或终止其它项目。整数PID经过
  // 验证后进入固定脚本，命令不包含凭据或用户输入。
  const result = spawnSync(
    'powershell',
    [
      '-NoProfile',
      '-NonInteractive',
      '-Command',
      `Get-Process -Id ${ids.join(',')} | Select-Object Id,WorkingSet64,PrivateMemorySize64 | ConvertTo-Json -Compress`,
    ],
    { windowsHide: true, encoding: 'utf8', timeout: 10000 },
  );
  assert.equal(result.status, 0, 'CAPACITY_PROCESS_MEMORY_UNAVAILABLE');
  const rows = JSON.parse(result.stdout);
  return rows.map((row) => ({
    role: registry.find((item) => item.pid === row.Id).role,
    workingSetBytes: row.WorkingSet64,
    privateBytes: row.PrivateMemorySize64,
  }));
}

test(
  'real 2 5 and 10 distinct collaborators durably save and FLUSH representative shared content',
  { timeout: 180000 },
  async () => {
    const organization = await organizationFixture(9),
      document = await documentFixture(organization);
    const owner = { f: organization, d: document, user: organization.owner };
    const peers = [],
      markers = [],
      measurements = [];
    const stageCounts = process.env.COLLAB_CAPACITY_STAGE === '10' ? [10] : [2, 5, 10];
    const directory = resolve(root, 'artifacts/batches/08');
    mkdirSync(directory, { recursive: true });
    const progress = { state: 'RUNNING', stageCounts, measurements };
    const record = () =>
      writeFileSync(
        resolve(directory, 'capacity-last-attempt.json'),
        JSON.stringify(progress, null, 2) + '\n',
      );
    try {
      await unlockStructurePreview(owner);
      const metadata = json(await get(`/v1/documents/${document.id}`, organization.owner.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${document.id}/authorization-changes`,
          {
            operation_id: operation('capacity-edit-grants'),
            expected_acl_revision: metadata.acl_revision,
            changes: organization.members.map((member) => ({
              action: 'GRANT',
              grant: {
                user_id: member.id,
                scope_kind: 'UNIT',
                scope_id: document.main.units[0].unit_id,
                capability: 'EDIT',
              },
            })),
          },
          organization.owner.headers,
        ),
        organization.owner,
      );
      peers.push(await openReviewWorkspace(organization, document, organization.owner, 'MAIN'));
      const body = {
        type: 'doc',
        content: Array.from({ length: 100 }, (_, index) => ({
          type: 'paragraph',
          attrs: { id: 'capacity-body-' + index },
          content: [{ type: 'text', text: '协作容量中文正文与可靠保存。'.repeat(64) }],
        })),
      };
      await writeDraftBody(peers[0], body);
      const bodyBytes = Buffer.byteLength(JSON.stringify(body));
      assert.ok(bodyBytes > 250000 && bodyBytes < 400000, 'REPRESENTATIVE_BODY_SIZE_CHANGED');
      for (const count of stageCounts) {
        progress.currentCollaborators = count;
        while (peers.length < count)
          peers.push(
            await openReviewWorkspace(
              organization,
              document,
              organization.members[peers.length - 1],
              'MAIN',
            ),
          );
        const samples = [];
        let latest;
        // 五轮真实并发输入用于本批明确容量关口；不重新排列历史正文上限
        // 或故障矩阵。各人使用自己的账号、准入与更新号，不共享owner身份。
        for (let round = 0; round < 5; round++) {
          progress.currentRound = round;
          record();
          await Promise.all(peers.map(sync));
          const currentMarkers = peers.map((_, index) => `cap-${count}-${round}-${index};`);
          markers.push(...currentMarkers);
          latest = await Promise.all(peers.map((peer, index) => save(peer, currentMarkers[index])));
          samples.push(...latest.map((result) => result.elapsedMs));
        }
        const started = performance.now();
        peers[0].w.send({
          type: 'FLUSH',
          operation_id: operation('capacity-flush'),
          dependencies: latest.map(({ saved }) => ({
            client_session_id: saved.client_session_id,
            client_update_id: saved.client_update_id,
            update_hash: saved.update_hash,
          })),
        });
        const flushed = await peers[0].w.next('FLUSHED');
        const flushMs = performance.now() - started;
        const high = latest.reduce(
          (value, { saved }) =>
            BigInt(saved.durable_seq) > value ? BigInt(saved.durable_seq) : value,
          0n,
        );
        assert.ok(BigInt(flushed.covered_cut.durable_seq) >= high);
        const contents = await Promise.all(peers.map(sync));
        for (const content of contents) {
          assert.equal(content.content.length, body.content.length);
          const text = JSON.stringify(content);
          for (const marker of markers)
            assert.ok(text.includes(marker), 'CAPACITY_CONCURRENT_INPUT_LOST');
        }
        measurements.push({
          collaborators: count,
          samples: samples.length,
          savedP50Ms: percentile(samples, 0.5),
          savedP95Ms: percentile(samples, 0.95),
          savedMaxMs: percentile(samples, 1),
          flushMs: Number(flushMs.toFixed(2)),
          coveredDurableSeq: flushed.covered_cut.durable_seq,
          memory: memory(),
        });
        record();
        console.log(
          JSON.stringify({
            event: 'COLLABORATION_CAPACITY_STAGE_COMPLETE',
            ...measurements.at(-1),
          }),
        );
      }
      // 可靠主线读取实际固定全部已提交输入，不能用本地Y.Doc一致替代持久性。
      const main = await get(`/v1/documents/${document.id}/main`, organization.owner.headers);
      assert.equal(main.status, 200);
      const fixed = json(main);
      // MAIN返回固定版本清单，正文须沿同一read_view读取；清单自身没有正文。
      // 不能把本地同步成功当持久化成功，也不能改读当前草稿绕过固定切点。
      const bodies = await Promise.all(
        fixed.units.map(async (unit) => {
          const response = await get(
            `/v1/documents/${document.id}/read-views/${fixed.read_view_id}/units/${unit.unit_id}`,
            organization.owner.headers,
          );
          assert.equal(response.status, 200);
          const body = json(response);
          assert.equal(body.id, unit.content_version_id);
          return body.body_json;
        }),
      );
      const content = JSON.stringify(bodies);
      for (const marker of markers)
        assert.ok(content.includes(marker), 'CAPACITY_FIXED_MAIN_INPUT_LOST');
      progress.state = 'PASS';
      record();
      writeFileSync(
        resolve(directory, 'capacity.json'),
        JSON.stringify(
          {
            state: 'PASS',
            document: document.id,
            bodyBytes,
            independentUsers: 10,
            roundsPerStage: 5,
            measurements,
            completedAt: new Date().toISOString(),
          },
          null,
          2,
        ) + '\n',
      );
      for (const measurement of measurements)
        console.log(
          JSON.stringify({
            event: 'COLLABORATION_CAPACITY_ACTUAL_PASS',
            bodyBytes,
            ...measurement,
          }),
        );
    } finally {
      for (const peer of peers) peer.close();
      if (progress.state !== 'PASS') {
        progress.state = 'FAIL';
        record();
      }
    }
  },
);
