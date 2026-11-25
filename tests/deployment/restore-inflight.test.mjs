import assert from 'node:assert/strict';
import test from 'node:test';
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { restoredDatabase } from './restore-baseline.mjs';
import { restoredRequest } from './restored-http.mjs';

test(
  'cold restore resumes the original real RUNNING export with unchanged fixed input and one receipt',
  { timeout: 90000 },
  async () => {
    assert.ok(process.env.COLLAB_RESTORE_ROOT, 'COLLAB_RESTORE_ROOT_REQUIRED');
    const target = resolve(process.env.COLLAB_RESTORE_ROOT);
    const probe = JSON.parse(
      readFileSync(resolve(target, '.runtime/services/restore-inflight-probe.json')),
    );
    assert.equal(probe.state, 'RUNNING');
    const environment = JSON.parse(readFileSync(resolve(target, '.runtime/dev.json')));
    const db = await restoredDatabase(target, 'document');
    const request = (path, options) =>
      restoredRequest(target, environment.ports.edge, path, options);
    const json = (response) => JSON.parse(response.body.toString('utf8'));
    try {
      // 冷备份包含原会话；直接使用原凭据，不能重新登记相似作业代替恢复。
      const me = await request('/v1/account/me', { headers: probe.user.headers });
      assert.equal(me.status, 200);
      assert.equal(json(me).id, probe.user.id);
      let finished;
      const deadline = Date.now() + 75000;
      do {
        const response = await request(`/v1/export-jobs/${probe.job}`, {
          headers: probe.user.headers,
        });
        assert.equal(response.status, 200);
        finished = json(response);
        if (!['QUEUED', 'RUNNING'].includes(finished.state)) break;
        await new Promise((done) => setTimeout(done, 100));
      } while (Date.now() < deadline);
      assert.equal(finished.state, 'SUCCEEDED', 'RESTORED_ORIGINAL_EXPORT_NOT_SUCCEEDED');
      const [[actual]] = await db.query(
        'SELECT j.id,CAST(j.generation AS CHAR) generation,c.snapshot,HEX(c.fingerprint) fingerprint,c.artifact_checksum,c.reserved_bytes FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?',
        [probe.job],
      );
      assert.ok(BigInt(actual.generation) > BigInt(probe.generation));
      assert.equal(actual.fingerprint, probe.fingerprint);
      assert.equal(
        createHash('sha256').update(actual.snapshot).digest('hex'),
        probe.snapshotDigest,
      );
      assert.equal(String(actual.reserved_bytes), '0');
      const download = await request(`/v1/export-jobs/${probe.job}/download`, {
        headers: probe.user.headers,
      });
      assert.equal(download.status, 200);
      assert.equal(download.body.subarray(0, 5).toString(), '%PDF-');
      assert.equal(
        createHash('sha256').update(download.body).digest('hex'),
        actual.artifact_checksum.toString('hex'),
      );
      const replay = await request(`/v1/documents/${probe.document}/exports`, {
        method: 'POST',
        // 隔离部署换端口，当前同源地址随配置迁移；保留原Cookie/CSRF值，
        // 不能发送原站Origin导致真实CSRF拒绝，再把拒绝误判为任务丢失。
        headers: { ...probe.user.headers, origin: `https://localhost:${environment.ports.edge}` },
        body: probe.input,
      });
      assert.equal(replay.status, 202);
      assert.equal(json(replay).id, probe.job);
      const [[count]] = await db.query('SELECT COUNT(*) n FROM export_job WHERE operation_id=?', [
        probe.input.operation_id,
      ]);
      assert.equal(String(count.n), '1');
      const evidence = {
        state: 'PASS',
        originalState: 'RUNNING',
        originalGeneration: probe.generation,
        restoredGeneration: actual.generation,
        fixedInputUnchanged: true,
        originalReplay: true,
        artifactBytes: download.body.length,
        artifactChecksum: actual.artifact_checksum.toString('hex'),
        completedAt: new Date().toISOString(),
      };
      writeFileSync(
        resolve(target, 'artifacts/batch08-inflight-restore.json'),
        JSON.stringify(evidence, null, 2) + '\n',
      );
      console.log(JSON.stringify({ event: 'REAL_INFLIGHT_RESTORE_PASS', ...evidence }));
    } finally {
      await db.end();
    }
  },
);
