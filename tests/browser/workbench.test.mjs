import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import { state, root } from '../../scripts/runtime-lib.mjs';
import { createUser } from '../helpers/account.mjs';
test('browser uses real Cookie login and creates an organization knowledge base and complete readable document', async () => {
  const u = await createUser(),
    browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 900 },
      }),
      page = await context.newPage(),
      errors = [];
    page.on('pageerror', (e) => errors.push(e.message));
    page.setDefaultTimeout(5000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(u.email);
    await page.getByLabel('登录密码', { exact: true }).fill(u.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page.getByRole('button', { name: '组织', exact: true }).click();
    await page.getByLabel('组织名称', { exact: true }).fill('浏览器实际协作组');
    await page.getByRole('button', { name: '创建组织', exact: true }).click();
    await page.getByRole('button', { name: '浏览器实际协作组', exact: true }).waitFor();
    await page.getByRole('button', { name: '工作台', exact: true }).click();
    await page
      .getByRole('group', { name: '空间类型', exact: true })
      .getByRole('button', { name: '组织空间', exact: true })
      .click();
    await page.getByLabel('组织', { exact: true }).selectOption({ label: '浏览器实际协作组' });
    await page.getByRole('button', { name: '新建文档 / 知识库', exact: true }).click();
    await page.getByLabel('知识库名称', { exact: true }).fill('浏览器实践库');
    await page.getByRole('button', { name: '创建知识库', exact: true }).click();
    await page.getByLabel('文档标题', { exact: true }).fill('完整协作报告');
    await page.getByRole('button', { name: '创建文档', exact: true }).click();
    await page.getByRole('heading', { name: '完整协作报告', exact: true }).waitFor();
    await page.getByText('第一章', { exact: true }).first().waitFor();
    assert.equal(errors.length, 0);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/02/workbench-desktop.png'),
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/02/workbench-mobile.png'),
      fullPage: true,
    });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    await page.getByRole('button', { name: '退出登录', exact: true }).click();
    await page.getByLabel('登录邮箱', { exact: true }).waitFor();
    await context.close();
  } finally {
    await browser.close();
  }
});
