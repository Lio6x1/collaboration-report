import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { state, runtime } from '../../scripts/runtime-lib.mjs';
import { createUser, post } from '../helpers/account.mjs';
const url = 'https://localhost:' + state().ports.edge;
function mailCode(id) {
  for (const f of readdirSync(resolve(runtime, 'smtp/inbox'))) {
    const m = JSON.parse(readFileSync(resolve(runtime, 'smtp/inbox', f)));
    if (m.body.includes('Challenge: ' + id)) return /Code: (\d{6})/.exec(m.body)[1];
  }
  throw Error('ACTUAL_SMTP_MAIL_MISSING');
}
async function login(page, email, password) {
  await page.getByLabel('登录邮箱', { exact: true }).fill(email);
  await page.getByLabel('登录密码', { exact: true }).fill(password);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
}
// 后台操作已开始但查询暂不可达，界面必须保留同号续接，不能把503当操作失败后重发新号。
test('browser password change survives a lost progress query and then email change and public reset use actual SMTP', async () => {
  const u = await createUser(),
    browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(8000);
    await page.goto(url);
    await login(page, u.email, u.password);
    await page.getByRole('button', { name: '账号设置', exact: true }).click();
    await page.getByLabel('当前密码', { exact: true }).fill(u.password);
    await page.getByLabel('修改后的密码', { exact: true }).fill('new-browser-password-123');
    let first = true;
    const operations = [];
    const queries = [];
    await page.route('**/v1/account/password', async (route) => {
      operations.push(route.request().postDataJSON().operation_id);
      const res = await route.fetch();
      await route.fulfill({ response: res, json: { ...(await res.json()), state: 'PREPARING' } });
    });
    await page.route('**/v1/account/operations/*', async (route) => {
      const request = route.request();
      assert.equal(request.method(), 'GET');
      const query = { id: new URL(request.url()).pathname.split('/').at(-1) };
      queries.push(query);
      if (first) {
        first = false;
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({ code: 'DEPENDENCY_UNAVAILABLE' }),
        });
      } else {
        const response = await route.fetch();
        const value = await response.json();
        query.status = response.status();
        query.original = value.operation_id === operations[0];
        query.state = value.state;
        await route.fulfill({ response });
      }
    });
    await page.getByRole('button', { name: '更新密码并关闭旧会话', exact: true }).click();
    await page.getByRole('button', { name: '确认操作结果', exact: true }).waitFor();
    // 等过周期身份核验：账号页被卸载后，应用级内存仍保存原改密输入与编号。
    await page.waitForTimeout(11000);
    assert.equal(await page.getByRole('button', { name: '确认操作结果', exact: true }).count(), 1);
    await page.getByRole('button', { name: '确认操作结果', exact: true }).click();
    await expect(page.getByRole('button', { name: '登录', exact: true })).toBeEnabled();
    // 登记响应已到达，丢的是后续查询：恢复必须沿原号 GET，不再提交改密 POST。
    // 原编号和终态一起核验，不能只把按钮恢复或登录页出现当作成功。
    assert.equal(operations.length, 1);
    assert.ok(queries.length >= 2, 'ORIGINAL_PROGRESS_MUST_BE_RESUMED');
    assert.ok(
      queries.every((query) => query.id === operations[0]),
      'ORIGINAL_QUERY_ID_REQUIRED',
    );
    assert.ok(
      queries.some(
        (query) => query.status === 200 && query.original && query.state === 'SUCCEEDED',
      ),
      'ORIGINAL_QUERY_TERMINAL_REQUIRED',
    );
    await page.unroute('**/v1/account/password');
    await page.unroute('**/v1/account/operations/*');
    await login(page, u.email, 'new-browser-password-123');
    await page.getByRole('button', { name: '账号设置', exact: true }).click();
    const email = 'changed-browser-' + randomUUID() + '@example.test';
    await page.getByLabel('新邮箱', { exact: true }).fill(email);
    let response = page.waitForResponse((r) => r.url().endsWith('/v1/account/challenges'));
    await page.getByRole('button', { name: '验证新邮箱', exact: true }).click();
    let challenge = await (await response).json();
    await page.getByLabel('新邮箱验证码', { exact: true }).fill(mailCode(challenge.challenge_id));
    await page.getByRole('button', { name: '确认更新邮箱', exact: true }).click();
    await page.getByLabel('登录邮箱', { exact: true }).waitFor();
    await page.getByRole('tab', { name: '找回密码', exact: true }).click();
    await page.getByLabel('验证邮箱', { exact: true }).fill(email);
    response = page.waitForResponse((r) => r.url().endsWith('/v1/account/challenges'));
    await page.getByRole('button', { name: '发送验证码', exact: true }).click();
    challenge = await (await response).json();
    await page.getByLabel('邮箱验证码', { exact: true }).fill(mailCode(challenge.challenge_id));
    await page.getByLabel('新密码', { exact: true }).fill('reset-browser-password-123');
    await page.getByRole('button', { name: '更新密码', exact: true }).click();
    await page.getByLabel('登录邮箱', { exact: true }).waitFor();
    await login(page, email, 'reset-browser-password-123');
    assert.equal(
      (await post('/v1/account/login', { email: u.email, password: u.password })).status,
      401,
    );
    await context.close();
  } finally {
    await browser.close();
  }
});
