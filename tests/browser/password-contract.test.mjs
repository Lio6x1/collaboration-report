import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';
import { createUser } from '../helpers/account.mjs';
// 契约允许非空、最多72个UTF-8字节。浏览器必须能登录API合法创建的短密码账号。
test('browser login accepts a one-byte password allowed by the fixed contract', async () => {
  const u = await createUser({ password: 'x' }),
    browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(4000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(u.email);
    await page.getByLabel('登录密码', { exact: true }).fill(u.password);
    assert.equal(
      await page.getByLabel('登录密码', { exact: true }).evaluate((input) => input.checkValidity()),
      true,
    );
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await context.close();
  } finally {
    await browser.close();
  }
});
