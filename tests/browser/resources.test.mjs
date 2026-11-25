import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { state, root } from '../../scripts/runtime-lib.mjs';
import { resolve } from 'node:path';

test('真实浏览器上传图片、插入保存及重新打开正文', { timeout: 60000 }, async () => {
  const f = await organizationFixture(0);
  await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 900 },
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
    await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    async function openEditor() {
      await page
        .getByRole('group', { name: '空间类型', exact: true })
        .getByRole('button', { name: '组织空间', exact: true })
        .click();
      await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
      await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
      await page.getByRole('button', { name: '协作报告', exact: true }).click();
      await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
      await page.getByText('服务端已保存', { exact: true }).waitFor();
    }
    await openEditor();
    const png = Buffer.from(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jC1kAAAAASUVORK5CYII=',
      'base64',
    );
    await page
      .getByLabel('选择正文图片')
      .setInputFiles({ name: '校招项目图片.png', mimeType: 'image/png', buffer: png });
    await page.getByRole('button', { name: '上传图片', exact: true }).click();
    try {
      await page.getByRole('button', { name: '将已上传图片插入正文', exact: true }).click();
    } catch (error) {
      throw Error(await page.locator('.resource-upload').innerText(), { cause: error });
    }
    await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
    await page.getByText('当前修改已保存', { exact: true }).waitFor();
    // 使用浏览器解码结果验证图片真正显示；不能只检查HTML里存在img标签。
    const image = page
      .getByRole('textbox', { name: '共同支线编辑器', exact: true })
      .locator('.resource-image img');
    await image.waitFor({ state: 'visible' });
    assert.equal(await image.evaluate((node) => node.complete && node.naturalWidth === 1), true);
    assert.match(await image.getAttribute('src'), /^\/v1\/resources\/[0-9]+\/content\?/);
    await page.reload();
    await openEditor();
    await image.waitFor({ state: 'visible' });
    assert.equal(await image.evaluate((node) => node.complete && node.naturalWidth === 1), true);
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/06/resource-browser.png'),
      fullPage: true,
    });
    await context.close();
  } finally {
    await browser.close();
  }
});
