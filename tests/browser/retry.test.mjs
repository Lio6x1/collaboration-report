import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { state, runtime } from '../../scripts/runtime-lib.mjs';
import { createUser } from '../helpers/account.mjs';
test('lost logout response keeps the original operation number through browser retry', async () => {
  const u = await createUser(),
    browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(5000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(u.email);
    await page.getByLabel('登录密码', { exact: true }).fill(u.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    let first = true;
    const operations = [];
    await page.route('**/v1/account/logout', async (route) => {
      operations.push(route.request().postDataJSON().operation_id);
      const response = await route.fetch();
      if (first) {
        first = false;
        assert.equal(response.status(), 202);
        await route.abort('failed');
      } else await route.fulfill({ response });
    });
    await page.getByRole('button', { name: '退出登录', exact: true }).click();
    await page.getByRole('button', { name: '确认操作结果', exact: true }).click();
    await page.getByLabel('登录邮箱', { exact: true }).waitFor();
    assert.equal(operations.length, 2);
    assert.equal(operations[0], operations[1]);
    await context.close();
  } finally {
    await browser.close();
  }
});
test('lost signup response retries the same durable operation and then completes the login continuation', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(5000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByRole('tab', { name: '注册', exact: true }).click();
    await page
      .getByLabel('验证邮箱', { exact: true })
      .fill('retry-browser-' + randomUUID() + '@example.test');
    await page.getByLabel('显示名称', { exact: true }).fill('同号重试用户');
    const mail = page.waitForResponse((r) => r.url().endsWith('/v1/account/challenges'));
    await page.getByRole('button', { name: '发送验证码', exact: true }).click();
    const id = (await (await mail).json()).challenge_id;
    let code;
    for (const file of readdirSync(resolve(runtime, 'smtp/inbox'))) {
      const v = JSON.parse(readFileSync(resolve(runtime, 'smtp/inbox', file)));
      if (v.body.includes('Challenge: ' + id)) {
        code = /Code: (\d{6})/.exec(v.body)[1];
        break;
      }
    }
    assert.ok(code);
    await page.getByLabel('邮箱验证码', { exact: true }).fill(code);
    await page.getByLabel('新密码', { exact: true }).fill('retry-browser-password-123');
    let first = true;
    const operations = [];
    await page.route('**/v1/account/signup', async (route) => {
      operations.push(route.request().postDataJSON().operation_id);
      const response = await route.fetch();
      if (first) {
        first = false;
        assert.equal(response.status(), 200);
        await route.abort('failed');
      } else await route.fulfill({ response });
    });
    await page.getByRole('button', { name: '验证并注册', exact: true }).click();
    await page.getByRole('button', { name: '确认操作结果', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    assert.equal(operations.length, 2);
    assert.equal(operations[0], operations[1]);
    await context.close();
  } finally {
    await browser.close();
  }
});
