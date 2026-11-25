import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, pause } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { openReviewWorkspace, writeDraftBody, reviewDatabase } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { uploadPNG, binaryResourceGet } from '../helpers/resource.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

// 空间满额只调整自己的容量上限；转换状态、代次、正文与成功回执均来自真实产品。
// 在实际RUNNING期间终止原Doc，再由原worker恢复同一作业，不能手工改成成功或另建作业。
test(
  'full user quota permits export and the original RUNNING job recovers after Doc exits',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(0),
      d = await documentFixture(f),
      db = await reviewDatabase();
    let main,
      originalLimit,
      failure,
      stopped = false;
    try {
      await unlockStructurePreview({ d, user: f.owner });
      main = await openReviewWorkspace(f, d, f.owner, 'MAIN');
      const image = readFileSync(resolve(root, 'tests/fixtures/export-image.png'));
      await uploadPNG(main, image);
      const text =
        '固定版本恢复验证：空间容量与转换临时预算相互独立。数据库事实、图片和中文正文来自真实保存，进程退出后仍沿原作业恢复。';
      assert.equal(
        (
          await writeDraftBody(main, {
            type: 'doc',
            content: Array.from({ length: 80 }, (_, i) => ({
              type: 'paragraph',
              attrs: { id: 'recovery-' + i },
              content: [{ type: 'text', text: `第${i + 1}段：` + text.repeat(3) }],
            })),
          })
        ).type,
        'SAVED',
      );
      main.close();
      main = undefined;
      const viewResponse = await get(`/v1/documents/${d.id}/main`, f.owner.headers);
      assert.equal(viewResponse.status, 200, viewResponse.body);
      const view = json(viewResponse);
      const [[quota]] = await db.query(
        'SELECT limit_bytes,used_bytes,reserved_bytes FROM quota_account WHERE space_id=?',
        [f.space],
      );
      assert.ok(BigInt(quota.used_bytes) > 0n);
      originalLimit = quota.limit_bytes;
      const [filled] = await db.query(
        'UPDATE quota_account SET limit_bytes=used_bytes+reserved_bytes,revision=revision+1 WHERE space_id=?',
        [f.space],
      );
      assert.equal(filled.affectedRows, 1);
      const currentQuota = json(await get(`/v1/spaces/${f.space}/quota`, f.owner.headers));
      const deniedUpload = await post(
        `/v1/documents/${d.id}/resources`,
        {
          operation_id: operation('full-upload'),
          expected_quota_revision: currentQuota.revision,
          byte_size: String(image.length),
          media_type: 'image/png',
          checksum: createHash('sha256').update(image).digest('hex'),
        },
        f.owner.headers,
      );
      assert.equal(deniedUpload.status, 409, deniedUpload.body);
      assert.equal(json(deniedUpload).code, 'QUOTA_EXCEEDED');
      const input = {
        operation_id: operation('recover-export'),
        target: 'MAIN',
        format: 'PDF',
        expected_manifest_id: view.manifest_id,
      };
      const created = await post(`/v1/documents/${d.id}/exports`, input, f.owner.headers);
      assert.equal(created.status, 202, created.body);
      const job = json(created);
      let running;
      const claimDeadline = Date.now() + 10000;
      while (Date.now() < claimDeadline) {
        [[running]] = await db.query('SELECT state,generation FROM export_job WHERE id=?', [
          job.id,
        ]);
        if (running.state === 'RUNNING') break;
        assert.equal(running.state, 'QUEUED', '必须在真实转换中退出，不能伪造RUNNING');
        await pause(30);
      }
      assert.equal(running.state, 'RUNNING');
      stopOwnedRole('document');
      stopped = true;
      const [[interrupted]] = await db.query('SELECT state,generation FROM export_job WHERE id=?', [
        job.id,
      ]);
      assert.equal(interrupted.state, 'RUNNING', '进程退出不能冒充转换成功');
      restartOwnedRuntime();
      stopped = false;
      const deadline = Date.now() + 75000;
      let finished;
      while (Date.now() < deadline) {
        const response = await get(`/v1/export-jobs/${job.id}`, f.owner.headers);
        assert.equal(response.status, 200, response.body);
        finished = json(response);
        if (!['QUEUED', 'RUNNING'].includes(finished.state)) break;
        await pause(100);
      }
      assert.equal(finished.state, 'SUCCEEDED', JSON.stringify(finished.last_error));
      const [[actual]] = await db.query(
        'SELECT j.id,j.generation,c.artifact_checksum,c.reserved_bytes FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?',
        [job.id],
      );
      assert.equal(actual.id, job.id);
      assert.ok(BigInt(actual.generation) > BigInt(interrupted.generation));
      assert.equal(String(actual.reserved_bytes), '0');
      const download = await binaryResourceGet(
        `/v1/export-jobs/${job.id}/download`,
        f.owner.headers,
      );
      assert.equal(download.status, 200, download.body.toString());
      assert.equal(download.body.subarray(0, 5).toString(), '%PDF-');
      assert.equal(
        createHash('sha256').update(download.body).digest('hex'),
        actual.artifact_checksum.toString('hex'),
      );
      const replay = await post(`/v1/documents/${d.id}/exports`, input, f.owner.headers);
      assert.equal(replay.status, 202, replay.body);
      assert.equal(json(replay).id, job.id);
      const [[count]] = await db.query('SELECT COUNT(*) n FROM export_job WHERE operation_id=?', [
        input.operation_id,
      ]);
      assert.equal(String(count.n), '1');
      const output = resolve(root, 'artifacts/batches/07/exports');
      mkdirSync(output, { recursive: true });
      writeFileSync(resolve(output, 'recovered-full-quota.pdf'), download.body);
      console.log(
        'EXPORT_RECOVERY_ACTUAL_PASS: full quota rejects new upload; actual RUNNING process exit; same job resumes with newer generation; PDF checksum and original replay match',
      );
    } catch (error) {
      failure = error;
      throw error;
    } finally {
      main?.close();
      try {
        if (stopped) restartOwnedRuntime();
      } catch (error) {
        if (!failure) throw error;
        console.error('EXPORT_ENVIRONMENT_RESTORE_FAILED');
      } finally {
        try {
          if (originalLimit !== undefined)
            await db.query(
              'UPDATE quota_account SET limit_bytes=?,revision=revision+1 WHERE space_id=?',
              [originalLimit, f.space],
            );
        } finally {
          await db.end();
        }
      }
    }
  },
);
