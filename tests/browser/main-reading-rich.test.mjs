import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import { importMarkdown } from '../../packages/content/markdown.mjs';
import { reviewFixture, openReviewWorkspace, writeDraftBody } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { get, json } from '../helpers/workspace.mjs';
import { state, root } from '../../scripts/runtime-lib.mjs';

// 把可靠MAIN降成纯文本会丢掉表格、独立公式与格式；固定阅读应使用正文模型的只读视图。
test('real MAIN reader renders reliable tables, formulae and formatting without becoming an editor', async () => {
  const f = await reviewFixture();
  let mainWorkspace;
  let browser;
  let page;
  const errors = [];
  let reading;
  try {
    await unlockStructurePreview(f);
    mainWorkspace = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    let next = 0;
    const accepted = importMarkdown(
      '## 富文本阅读\n\n**加粗正文**与 $x^2$\n\n| 字段 | 值 |\n| --- | --- |\n| 主线 | 可靠正文 |',
      { newId: () => `main-reading-${++next}` },
    ).body;
    await writeDraftBody(mainWorkspace, accepted);
    const current = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    reading = json(
      await get(
        `/v1/documents/${f.d.id}/read-views/${current.read_view_id}/units/${current.units[0].unit_id}`,
        f.user.headers,
      ),
    );
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    page = await browser.newPage({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 960 },
    });
    page.on('pageerror', (error) => errors.push(error.message));
    page.setDefaultTimeout(8000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(f.user.email);
    await page.getByLabel('登录密码', { exact: true }).fill(f.user.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page
      .getByRole('group', { name: '空间类型', exact: true })
      .getByRole('button', { name: '组织空间', exact: true })
      .click();
    await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
    await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
    await page.getByRole('button', { name: '协作报告', exact: true }).click();
    await page.getByRole('region', { name: '主线控制', exact: true }).waitFor();
    const body = page.getByRole('document', { name: '主线正文', exact: true });
    await body.waitFor();
    assert.equal(await body.getAttribute('contenteditable'), 'false');
    assert.equal(await body.locator('strong').innerText(), '加粗正文');
    assert.equal(await body.locator('table').count(), 1);
    assert.equal(await body.locator('tbody tr').count(), 2);
    assert.ok((await body.innerText()).includes('可靠正文'));
    assert.equal(await body.locator('[data-formula="x^2"] .katex').count(), 1);
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/main-reading-rich-browser.png'),
      fullPage: true,
    });
  } catch (error) {
    console.error(
      JSON.stringify({
        pageErrors: errors,
        fixedBody: reading?.body_json,
        markup: page
          ? await page
              .locator('.content-view')
              .evaluateAll((nodes) => nodes.map((node) => node.outerHTML))
          : [],
      }),
    );
    if (page)
      await page.screenshot({
        path: resolve(root, 'artifacts/batches/04/main-reading-rich-browser-failure.png'),
        fullPage: true,
      });
    throw error;
  } finally {
    mainWorkspace?.close();
    f.close();
    await browser?.close();
  }
});
