import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import {
  openReviewWorkspace,
  writeDraftBody,
  writeDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { uploadPNG, binaryResourceGet } from '../helpers/resource.mjs';
import { publishRelease, releaseInput } from '../helpers/publication.mjs';
import { chromium, expect } from '@playwright/test';
import { origin } from '../helpers/account.mjs';

const paragraph = (id, text) => ({
  type: 'paragraph',
  attrs: { id },
  content: [{ type: 'text', text }],
});
const atom = {
  atomStrong: false,
  atomEm: false,
  atomCode: false,
  atomStrike: false,
  atomLink: null,
};
// 实际解码器检查PNG完整CRC；旧下载夹具只检查字节传输，不能作为格式转换夹具。
const exportPNG = readFileSync(resolve(root, 'tests/fixtures/export-image.png'));
function richBody(resource) {
  const cell = (id, text, header) => ({
    type: header ? 'table_header' : 'table_cell',
    attrs: { id, colspan: 1, rowspan: 1, colwidth: null },
    content: [paragraph(id + '-p', text)],
  });
  return {
    type: 'doc',
    content: [
      {
        type: 'heading',
        attrs: { id: 'export-heading', level: 2 },
        content: [{ type: 'text', text: '固定版本的中文正文' }],
      },
      {
        type: 'paragraph',
        attrs: { id: 'export-intro' },
        content: [
          {
            type: 'text',
            text: 'S1 正式发布，保留中文与 English spaces。',
            marks: [{ type: 'strong' }],
          },
          {
            type: 'text',
            text: '仿宋字体',
            marks: [{ type: 'font', attrs: { family: 'FangSong' } }],
          },
          { type: 'formula', attrs: { latex: 'E=mc^2', ...atom } },
        ],
      },
      {
        type: 'formula_block',
        attrs: { id: 'export-math', latex: '\\frac{a+b}{2}=\\sqrt{x}', ...atom },
      },
      {
        type: 'table',
        attrs: { id: 'export-table' },
        content: [
          {
            type: 'table_row',
            attrs: { id: 'export-head' },
            content: [cell('export-c1', '项目', true), cell('export-c2', '说明', true)],
          },
          {
            type: 'table_row',
            attrs: { id: 'export-row' },
            content: [
              cell('export-c3', '固定版本', false),
              cell('export-c4', '这张表来自正式发布的 S1', false),
            ],
          },
        ],
      },
      {
        type: 'paragraph',
        attrs: { id: 'export-image' },
        content: [
          {
            type: 'image',
            attrs: {
              resourceId: resource,
              alt: '固定图片',
              title: null,
              width: 320,
              height: 80,
              ...atom,
            },
          },
        ],
      },
      ...Array.from({ length: 14 }, (_, i) =>
        paragraph(
          'export-detail-' + i,
          `第${i + 1}段：协作内容经过可靠保存后，导出固定不可变版本。撤权与正式版变化会在下载时再次检查；后续编辑不会改变这个文件。`,
        ),
      ),
    ],
  };
}

async function finishJob(job, user) {
  const deadline = Date.now() + 75000;
  while (Date.now() < deadline) {
    const response = await get(`/v1/export-jobs/${job.id}`, user?.headers);
    assert.equal(response.status, 200, response.body);
    const current = json(response);
    if (!['QUEUED', 'RUNNING'].includes(current.state)) {
      assert.equal(current.state, 'SUCCEEDED', JSON.stringify(current.last_error));
      return current;
    }
    await new Promise((done) => setTimeout(done, 150));
  }
  assert.fail('EXPORT_NOT_TERMINAL');
}

// 一次真实HTTP/WSS/SQL流程覆盖三格式、固定主线与正式版、原号重放和当前权限。
// 文件由真实worker生成；正文、ACK、作业成功与资源引用都不以SQL夹具伪造。
test(
  'fixed MAIN and anonymous PUBLIC export actual DOCX Markdown and PDF with current authorization',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(1),
      d = await documentFixture(f),
      db = await reviewDatabase();
    let main, browser;
    const output = resolve(root, 'artifacts/batches/07/exports');
    mkdirSync(output, { recursive: true });
    try {
      await unlockStructurePreview({ d, user: f.owner });
      main = await openReviewWorkspace(f, d, f.owner, 'MAIN');
      const resource = await uploadPNG(main, exportPNG);
      assert.equal((await writeDraftBody(main, richBody(resource))).type, 'SAVED');
      const release = (await publishRelease(d.id, f.owner, await releaseInput(d.id, f.owner)))
        .result.entity_id;
      await writeDraft(main, 'S2 未发布的后来主线内容');
      for (const path of [`/v1/knowledge-bases/${f.kb}`, `/v1/documents/${d.id}`]) {
        const facts = json(await get(path, f.owner.headers));
        await waitAuthorization(
          await post(
            path + '/visibility',
            {
              operation_id: operation('export-public'),
              expected_visibility_revision: facts.visibility_revision,
              policy: 'PUBLIC',
            },
            f.owner.headers,
          ),
          f.owner,
        );
      }
      const view = json(await get(`/v1/documents/${d.id}/main`, f.owner.headers));
      // 可见范围变化已关闭旧来源连接，后续编辑重新取得当前代次票据。
      main.close();
      main = await openReviewWorkspace(f, d, f.owner, 'MAIN');
      const input = {
        operation_id: operation('export-main'),
        target: 'MAIN',
        format: 'MARKDOWN',
        expected_manifest_id: view.manifest_id,
      };
      const anonymousMain = await post(`/v1/documents/${d.id}/exports`, input);
      assert.equal(anonymousMain.status, 401, anonymousMain.body);
      const created = await post(`/v1/documents/${d.id}/exports`, input, f.owner.headers);
      assert.equal(created.status, 202, created.body);
      const mainJob = json(created);
      // 作业已经持久登记，后续S3不能改写原固定输入；未知回复只重取同一个原号。
      await writeDraft(main, 'S3 不得进入之前导出的内容');
      const replay = await post(`/v1/documents/${d.id}/exports`, input, f.owner.headers);
      assert.equal(replay.status, 202, replay.body);
      assert.equal(json(replay).id, mainJob.id);
      assert.equal(
        (await post(`/v1/documents/${d.id}/exports`, { ...input, format: 'PDF' }, f.owner.headers))
          .status,
        409,
      );
      assert.equal((await get(`/v1/export-jobs/${mainJob.id}`, f.members[0].headers)).status, 404);
      await finishJob(mainJob, f.owner);
      const mainFile = await binaryResourceGet(
        `/v1/export-jobs/${mainJob.id}/download`,
        f.owner.headers,
      );
      assert.equal(mainFile.status, 200, mainFile.body.toString());
      writeFileSync(resolve(output, 'main-fixed.zip'), mainFile.body);
      const publicJobs = [];
      for (const format of ['DOCX', 'MARKDOWN', 'PDF']) {
        const body = {
          operation_id: operation('export-public-' + format),
          target: 'PUBLIC',
          format,
          expected_release_id: release,
        };
        assert.equal(
          (await post(`/v1/documents/${d.id}/exports`, body, { cookie: f.owner.headers.cookie }))
            .status,
          403,
        );
        const registered = await post(`/v1/documents/${d.id}/exports`, body);
        assert.equal(registered.status, 202, registered.body);
        const job = await finishJob(json(registered));
        assert.equal(job.release_id, release);
        const file = await binaryResourceGet(`/v1/export-jobs/${job.id}/download`);
        assert.equal(file.status, 200, file.body.toString());
        assert.match(file.headers['content-disposition'], /^attachment; filename=/);
        assert.equal(
          file.body.subarray(0, format === 'PDF' ? 4 : 2).toString(),
          format === 'PDF' ? '%PDF' : 'PK',
        );
        writeFileSync(
          resolve(output, 'public-fixed.' + { DOCX: 'docx', MARKDOWN: 'zip', PDF: 'pdf' }[format]),
          file.body,
        );
        publicJobs.push(job);
      }
      const [[pins]] = await db.query(
        "SELECT COUNT(*) n FROM resource_reference WHERE owner_kind='EXPORT' AND owner_ref IN (?,?,?,?)",
        [mainJob.id, ...publicJobs.map((j) => j.id)],
      );
      assert.equal(String(pins.n), '0', 'terminal jobs must release exact resource pins');
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const page = await browser.newPage({ ignoreHTTPSErrors: true });
      await page.goto(`${origin}/public/documents/${d.id}`);
      const panel = page.getByRole('region', { name: '导出固定版本', exact: true });
      await expect(panel).toBeVisible();
      const originalOperations = [];
      let loseReply = true;
      await page.route(`**/v1/documents/${d.id}/exports`, async (route) => {
        originalOperations.push(route.request().postDataJSON().operation_id);
        if (loseReply) {
          loseReply = false;
          const response = await route.fetch();
          assert.equal(response.status(), 202);
          await route.abort('failed');
        } else await route.continue();
      });
      await panel.getByRole('button', { name: '创建导出', exact: true }).click();
      await panel.getByRole('button', { name: '确认操作结果', exact: true }).click();
      const download = panel.getByRole('link', { name: '下载导出文件', exact: true });
      await expect(download).toBeVisible({ timeout: 30000 });
      assert.equal(originalOperations.length, 2);
      assert.equal(new Set(originalOperations).size, 1, '未知回复必须续接原导出号');
      await page.reload();
      await expect(download).toBeVisible();
      const completedDownload = page.waitForEvent('download');
      await download.click();
      await (await completedDownload).saveAs(resolve(output, 'browser-fixed.docx'));
      await panel.screenshot({ path: resolve(output, 'export-browser.png') });
      const current = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${d.id}/visibility`,
          {
            operation_id: operation('export-withdraw-audience'),
            expected_visibility_revision: current.visibility_revision,
            policy: 'COLLABORATORS',
          },
          f.owner.headers,
        ),
        f.owner,
      );
      assert.equal((await get(`/v1/export-jobs/${publicJobs[0].id}`)).status, 401);
      assert.equal(
        (await binaryResourceGet(`/v1/export-jobs/${publicJobs[0].id}/download`)).status,
        401,
      );
      writeFileSync(
        resolve(output, 'actual-facts.json'),
        JSON.stringify(
          {
            document: d.id,
            release,
            main_manifest: mainJob.manifest_id,
            public_manifest: publicJobs[0].manifest_id,
            jobs: [mainJob.id, ...publicJobs.map((j) => j.id)],
          },
          null,
          2,
        ) + '\n',
      );
    } finally {
      main?.close();
      await browser?.close();
      await db.end();
    }
  },
);
