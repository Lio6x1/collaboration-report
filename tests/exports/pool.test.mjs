import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, pause } from '../../scripts/runtime-lib.mjs';
import {
  createUser,
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { existingTestAccount } from '../helpers/account.mjs';
import { openReviewWorkspace, writeDraftBody, reviewDatabase } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { binaryResourceGet } from '../helpers/resource.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

// 默认开发SMTP创建隔离身份；真实SMTP可显式提供专用测试账号，不依赖体验产品凭据。
// 仅创建独立组织/文档，通过真实WSS和HTTP登记作业。
// 同时RUNNING时退出Doc，再降低并发恢复，覆盖锁、原号、预算及制品隔离的实际风险。
test(
  'bounded export pool resumes original concurrent jobs after downscaling',
  { timeout: 240000 },
  async () => {
    const configPath = resolve(root, '.runtime/services/document.json');
    const originalConfig = readFileSync(configPath);
    const config = JSON.parse(originalConfig);
    const python = process.env.EXPORT_TEST_PYTHON ?? 'python';
    execFileSync(python, ['-c', 'import pdfplumber'], { windowsHide: true });
    const output = resolve(root, 'artifacts/reported-usability/export-pool');
    mkdirSync(output, { recursive: true });
    const db = await reviewDatabase();
    let needsRestore = false;
    const sessions = [];
    try {
      const [[pending]] = await db.query(
        "SELECT COUNT(*) n FROM export_job WHERE state IN ('QUEUED','RUNNING')",
      );
      assert.equal(String(pending.n), '0', '不能中断其他用户的未完成导出');
      assert.ok(
        !config.exports?.workers || config.exports.workers === 1,
        '测试结束必须恢复默认配置',
      );
      // 私密配置不复制到公开产物或日志，finally原字节恢复。
      writeFileSync(
        configPath,
        JSON.stringify({ ...config, exports: { ...config.exports, workers: 2 } }, null, 2),
      );
      needsRestore = true;
      restartOwnedRuntime();
      const owner = (await existingTestAccount()) ?? (await createUser());
      const f = await organizationFixture(0, owner);
      const docs = [];
      for (const marker of ['POOL_SOURCE_ALPHA', 'POOL_SOURCE_BETA']) {
        const d = await documentFixture(f);
        await unlockStructurePreview({ d, user: owner });
        const main = await openReviewWorkspace(f, d, owner, 'MAIN');
        sessions.push(main);
        const saved = await writeDraftBody(main, {
          type: 'doc',
          content: Array.from({ length: 80 }, (_, index) => ({
            type: 'paragraph',
            attrs: { id: marker + '-' + index },
            content: [
              {
                type: 'text',
                text:
                  `${marker}_${index} ` +
                  '并行转换保留各自固定正文，任务恢复不能重复领取，也不能混用另一个任务的临时目录。'.repeat(
                    3,
                  ),
              },
            ],
          })),
        });
        assert.equal(saved.type, 'SAVED');
        main.close();
        const view = await get(`/v1/documents/${d.id}/main`, owner.headers);
        assert.equal(view.status, 200, view.body);
        docs.push({ ...d, marker, manifest: json(view).manifest_id });
      }
      const requests = [
        { doc: docs[0], format: 'PDF' },
        { doc: docs[1], format: 'PDF' },
        { doc: docs[0], format: 'DOCX' },
      ].map((item) => ({
        ...item,
        input: {
          operation_id: operation('export-pool'),
          target: 'MAIN',
          format: item.format,
          expected_manifest_id: item.doc.manifest,
        },
      }));
      for (const item of requests) {
        const response = await post(
          `/v1/documents/${item.doc.id}/exports`,
          item.input,
          owner.headers,
        );
        assert.equal(response.status, 202, response.body);
        item.job = json(response);
      }
      const ids = requests.map((item) => item.job.id);
      let running;
      const claimDeadline = Date.now() + 15000;
      while (Date.now() < claimDeadline) {
        [running] = await db.query(
          'SELECT j.id,j.state,j.generation,c.reserved_bytes FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id IN (?) ORDER BY j.id',
          [ids],
        );
        writeFileSync(resolve(output, 'claim-observation.json'), JSON.stringify(running, null, 2));
        assert.ok(
          running.filter((item) => item.state === 'RUNNING').length <= 2,
          '不可超过配置并发',
        );
        if (running.filter((item) => item.state === 'RUNNING').length === 2) break;
        assert.ok(
          running.every((item) => ['QUEUED', 'RUNNING'].includes(item.state)),
          '必须实际观察到重叠转换',
        );
        await pause(20);
      }
      const claimedJobs = running.filter((item) => item.state === 'RUNNING');
      assert.equal(claimedJobs.length, 2);
      assert.equal(running.filter((item) => item.state === 'QUEUED').length, 1);
      for (const item of claimedJobs) {
        assert.equal(String(item.generation), '2', '初始代次为1，任务只能被领取一次');
        assert.equal(String(item.reserved_bytes), String(192 << 20));
      }
      // 归属核验会调用Windows CIM，期间真实转换可能已经完成。先锁住本测试的
      // 完成事务边界，再确认进程退出，不能拿几秒前的RUNNING冒充中断证据。
      // 只读锁定实际行，不修改状态/代次/产物；恢复仍由真实worker和转换器完成。
      let interruptedJobs;
      await db.beginTransaction();
      try {
        const [atBoundary] = await db.query(
          'SELECT id,state,generation FROM export_job WHERE id IN (?) ORDER BY id FOR UPDATE',
          [ids],
        );
        interruptedJobs = atBoundary.filter((item) => item.state === 'RUNNING');
        assert.equal(interruptedJobs.length, 2, '必须确实中断两个尚未完成的原作业');
        assert.equal(atBoundary.filter((item) => item.state === 'QUEUED').length, 1);
        const pid = stopOwnedRole('document');
        const exitDeadline = Date.now() + 5000;
        let exited = false;
        while (Date.now() < exitDeadline) {
          try {
            process.kill(pid, 0);
          } catch (error) {
            if (error.code !== 'ESRCH') throw error;
            exited = true;
            break;
          }
          await pause(20);
        }
        assert.ok(exited, '原Doc必须实际退出后才解除完成事务边界');
      } finally {
        await db.rollback();
      }
      const [afterExit] = await db.query(
        'SELECT id,state,generation FROM export_job WHERE id IN (?) ORDER BY id',
        [ids],
      );
      writeFileSync(
        resolve(output, 'interrupted-observation.json'),
        JSON.stringify(afterExit, null, 2),
      );
      assert.deepEqual(
        afterExit.filter((item) => item.state === 'RUNNING'),
        interruptedJobs,
      );
      writeFileSync(configPath, originalConfig);
      restartOwnedRuntime();
      needsRestore = false;
      let finished;
      const finishDeadline = Date.now() + 75000;
      while (Date.now() < finishDeadline) {
        [finished] = await db.query(
          'SELECT j.id,j.state,j.generation,c.reserved_bytes,c.artifact_checksum FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id IN (?) ORDER BY j.id',
          [ids],
        );
        assert.ok(
          finished.filter((item) => item.state === 'RUNNING').length <= 2,
          '旧RUNNING保留，不能伪造退出',
        );
        if (finished.every((item) => item.state === 'SUCCEEDED')) break;
        assert.ok(
          finished.every((item) => ['QUEUED', 'RUNNING', 'SUCCEEDED'].includes(item.state)),
          JSON.stringify(finished),
        );
        await pause(100);
      }
      assert.ok(
        finished.every((item) => item.state === 'SUCCEEDED'),
        JSON.stringify(finished),
      );
      writeFileSync(
        resolve(output, 'recovery-observation.json'),
        JSON.stringify(
          finished.map(({ artifact_checksum, ...item }) => item),
          null,
          2,
        ),
      );
      for (const item of requests) {
        const actual = finished.find((row) => row.id === item.job.id);
        assert.equal(String(actual.reserved_bytes), '0');
        assert.equal(
          String(actual.generation),
          interruptedJobs.some((row) => row.id === item.job.id) ? '3' : '2',
          '恢复沿原job且只增加一次代次',
        );
        const file = await binaryResourceGet(
          `/v1/export-jobs/${item.job.id}/download`,
          owner.headers,
        );
        assert.equal(file.status, 200);
        assert.equal(
          createHash('sha256').update(file.body).digest('hex'),
          actual.artifact_checksum.toString('hex'),
        );
        const path = resolve(output, item.doc.marker + '.' + item.format.toLowerCase());
        writeFileSync(path, file.body);
        const text = execFileSync(
          python,
          [
            '-c',
            item.format === 'PDF'
              ? 'import pdfplumber,sys,logging; logging.getLogger("pdfminer").setLevel(logging.ERROR); p=pdfplumber.open(sys.argv[1]); print("\\n".join(x.extract_text() for x in p.pages))'
              : 'import zipfile,sys,xml.etree.ElementTree as E; z=zipfile.ZipFile(sys.argv[1]); d=E.fromstring(z.read("word/document.xml")); print("".join(x.text or "" for x in d.iter() if x.tag.endswith("}t")))',
            path,
          ],
          { encoding: 'utf8', windowsHide: true },
        );
        assert.ok(text.includes(item.doc.marker), '制品必须来自原固定正文');
        assert.ok(
          !text.includes(docs.find((doc) => doc.id !== item.doc.id).marker),
          '并行任务正文不能串用',
        );
        const replay = await post(
          `/v1/documents/${item.doc.id}/exports`,
          item.input,
          owner.headers,
        );
        assert.equal(replay.status, 202, replay.body);
        assert.equal(json(replay).id, item.job.id);
        const anonymous = await binaryResourceGet(`/v1/export-jobs/${item.job.id}/download`, {});
        assert.equal(anonymous.status, 404, '私有MAIN导出向其他主体隐藏作业存在性');
      }
      const [[budget]] = await db.query('SELECT reserved_bytes FROM export_budget WHERE id=1');
      assert.equal(String(budget.reserved_bytes), '0');
      writeFileSync(
        resolve(output, 'facts.json'),
        JSON.stringify(
          {
            workers: 2,
            afterRestartWorkers: 1,
            interrupted: interruptedJobs,
            finished: finished.map(({ artifact_checksum, ...item }) => item),
            reserved: budget.reserved_bytes,
          },
          null,
          2,
        ),
      );
      console.log(
        'EXPORT_POOL_ACTUAL_PASS: two RUNNING + one QUEUED; original jobs resume after downscaling; checksums, fixed bodies, replay and authorization verified; default single worker restored',
      );
    } finally {
      sessions.forEach((session) => session.close());
      writeFileSync(configPath, originalConfig);
      try {
        if (needsRestore) restartOwnedRuntime();
      } finally {
        await db.end();
      }
    }
  },
);
