import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import { reviewFixture, writeDraft, submitDraft } from '../helpers/review.mjs';
import { get, json } from '../helpers/workspace.mjs';
import { state, root } from '../../scripts/runtime-lib.mjs';

// 当前排版可以追加样式修订，已经固定送审的正文及样式必须继续指向原版本。
test('real browser changes unlocked MAIN style while the submitted round keeps its original style', async () => {
  const f = await reviewFixture();
  let browser;
  let page;
  const errors = [];
  try {
    await writeDraft(f, '样式页面的原固定送审正文');
    const fixed = (await submitDraft(f)).value.result;
    const before = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
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
    const font = page.getByLabel('正文中文字体', { exact: true });
    await font.waitFor();
    assert.equal(await font.count(), 1, '缺少已定稿的正文样式入口');
    assert.equal(await font.isDisabled(), true, '锁定主线不能调整样式');
    await page.getByRole('button', { name: '解锁主线', exact: true }).click();
    await page.getByRole('button', { name: '锁定主线', exact: true }).waitFor();
    await font.selectOption('FangSong');
    await page.getByRole('button', { name: '应用正文样式', exact: true }).click();
    await page.getByText(/A4 · 仿宋/).waitFor();
    await page.getByText('样式页面的原固定送审正文', { exact: true }).waitFor();
    const current = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    assert.equal(current.style.style.body_font, 'FangSong');
    assert.ok(
      (
        await page
          .locator('.reading-layout > .paper')
          .evaluate((node) => getComputedStyle(node).fontFamily)
      ).includes('FangSong'),
    );
    assert.notEqual(current.style.id, before.style.id);
    const original = json(
      await get(`/v1/review-requests/${fixed.request_id}/rounds/${fixed.round_id}`, f.user.headers),
    );
    assert.deepEqual(original.style, before.style);
    assert.ok(
      (await page.locator('.fixed-review-body [role=document]').getAttribute('style')).includes(
        'SimSun',
      ),
    );
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/main-style-browser.png'),
      fullPage: true,
    });
  } catch (error) {
    if (page) {
      console.error(
        JSON.stringify({
          pageErrors: errors,
          styleCount: await page.getByRole('region', { name: '正文样式', exact: true }).count(),
          styleMarkup: await page
            .getByRole('region', { name: '正文样式', exact: true })
            .evaluateAll((nodes) => nodes.map((node) => node.outerHTML)),
        }),
      );
      await page.screenshot({
        path: resolve(root, 'artifacts/batches/04/main-style-browser-failure.png'),
        fullPage: true,
      });
    }
    throw error;
  } finally {
    f.close();
    await browser?.close();
  }
});
