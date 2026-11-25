import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime, pause } from '../../scripts/runtime-lib.mjs';
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
import { stopOwnedRole } from '../helpers/owned-runtime.mjs';

// 本脚本只制造一个真实在途原作业。确认实际RUNNING后终止本owner的Doc，
// 后续冷备份入口再停止全部写者；不改作业状态、代次、回执或输出ACK。
const f = await organizationFixture(0);
const d = await documentFixture(f);
const db = await reviewDatabase();
let main;
try {
  await unlockStructurePreview({ d, user: f.owner });
  main = await openReviewWorkspace(f, d, f.owner, 'MAIN');
  await writeDraftBody(main, {
    type: 'doc',
    content: Array.from({ length: 80 }, (_, i) => ({
      type: 'paragraph',
      attrs: { id: 'cold-inflight-' + i },
      content: [
        {
          type: 'text',
          text: `第${i + 1}段：冷备份恢复必须继续原作业及原固定正文，未知转换不能冒充成功。`.repeat(
            20,
          ),
        },
      ],
    })),
  });
  main.close();
  main = undefined;
  const reading = await get(`/v1/documents/${d.id}/main`, f.owner.headers);
  assert.equal(reading.status, 200);
  const view = json(reading);
  const input = {
    operation_id: operation('cold-inflight-export'),
    target: 'MAIN',
    format: 'PDF',
    expected_manifest_id: view.manifest_id,
  };
  const response = await post(`/v1/documents/${d.id}/exports`, input, f.owner.headers);
  assert.equal(response.status, 202, response.body);
  const job = json(response);
  let actual;
  const deadline = Date.now() + 10000;
  do {
    [[actual]] = await db.query(
      'SELECT state,CAST(generation AS CHAR) generation FROM export_job WHERE id=?',
      [job.id],
    );
    if (actual.state === 'RUNNING') break;
    assert.equal(actual.state, 'QUEUED', 'BACKUP_REQUIRES_ACTUAL_INFLIGHT_JOB');
    await pause(20);
  } while (Date.now() < deadline);
  assert.equal(actual.state, 'RUNNING');
  stopOwnedRole('document');
  const [[stopped]] = await db.query(
    'SELECT j.state,CAST(j.generation AS CHAR) generation,HEX(c.fingerprint) fingerprint,c.snapshot FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?',
    [job.id],
  );
  assert.equal(stopped.state, 'RUNNING', 'DOC_EXIT_MUST_LEAVE_ORIGINAL_INFLIGHT_FACT');
  // services目录在私密配置归档内；仅恢复验收读取这个探针，不装配为服务配置。
  // 测试会话和口令随受保护备份保存，日志不输出它们。
  const evidence = {
    state: 'RUNNING',
    job: job.id,
    document: d.id,
    input,
    user: f.owner,
    generation: stopped.generation,
    fingerprint: stopped.fingerprint,
    snapshotDigest: createHash('sha256').update(stopped.snapshot).digest('hex'),
    units: view.units,
    preparedAt: new Date().toISOString(),
  };
  writeFileSync(
    resolve(runtime, 'services/restore-inflight-probe.json'),
    JSON.stringify(evidence, null, 2) + '\n',
    { flag: 'wx' },
  );
  console.log(
    JSON.stringify({
      event: 'REAL_INFLIGHT_BACKUP_PREPARED',
      state: stopped.state,
      generation: stopped.generation,
      fixedUnits: view.units.length,
    }),
  );
} finally {
  main?.close();
  await db.end();
}
