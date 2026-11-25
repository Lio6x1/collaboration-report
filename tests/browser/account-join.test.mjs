import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { state, runtime, root } from '../../scripts/runtime-lib.mjs';
import { organizationFixture } from '../helpers/workspace.mjs';
const url = 'https://localhost:' + state().ports.edge;
function mailCode(id) {
  for (const file of readdirSync(resolve(runtime, 'smtp/inbox'))) {
    const m = JSON.parse(readFileSync(resolve(runtime, 'smtp/inbox', file)));
    if (m.body.includes('Challenge: ' + id)) return /Code: (\d{6})/.exec(m.body)[1];
  }
  throw Error('ACTUAL_SMTP_MAIL_MISSING');
}
async function login(page, u) {
  await page.goto(url);
  await page.getByLabel('登录邮箱', { exact: true }).fill(u.email);
  await page.getByLabel('登录密码', { exact: true }).fill(u.password);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
}
test('browser registers with actual SMTP and applies; an owner approves and grants actual KB creation eligibility', async () => {
  const f = await organizationFixture(0),
    browser = await chromium.launch({ channel: 'msedge', headless: true }),
    errors = [];
  try {
    const c = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await c.newPage();
    page.setDefaultTimeout(8000);
    page.on('pageerror', (e) => errors.push(e.message));
    await page.goto(url);
    await page.getByRole('tab', { name: '注册', exact: true }).click();
    const email = 'browser-' + randomUUID() + '@example.test';
    await page.getByLabel('验证邮箱', { exact: true }).fill(email);
    await page.getByLabel('显示名称', { exact: true }).fill('浏览器新人');
    const response = page.waitForResponse(
      (r) => r.url().endsWith('/v1/account/challenges') && r.request().method() === 'POST',
    );
    await page.getByRole('button', { name: '发送验证码', exact: true }).click();
    const challenge = await (await response).json();
    await page.getByLabel('邮箱验证码', { exact: true }).fill(mailCode(challenge.challenge_id));
    await page.getByLabel('新密码', { exact: true }).fill('browser-password-123');
    await page.getByRole('button', { name: '验证并注册', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    const me = await page.evaluate(async () => await (await fetch('/v1/account/me')).json());
    assert.equal(me.display_name, '浏览器新人');
    await page.getByRole('button', { name: '组织', exact: true }).click();
    await page.getByRole('button', { name: '创建或加入组织', exact: true }).click();
    await page.getByRole('button', { name: '申请加入', exact: true }).click();
    await page.getByLabel('组织编号', { exact: true }).fill(f.organization.padStart(8, '0'));
    await page.getByLabel('申请说明', { exact: true }).fill('浏览器申请实际参与');
    await page.getByRole('button', { name: '提交加入申请', exact: true }).click();
    await page.getByRole('status').filter({ hasText: '申请已提交，等待管理员审批' }).waitFor();
    const ownerContext = await browser.newContext({ ignoreHTTPSErrors: true }),
      owner = await ownerContext.newPage();
    owner.setDefaultTimeout(8000);
    owner.on('pageerror', (e) => errors.push(e.message));
    await login(owner, f.owner);
    await owner.getByRole('button', { name: '组织', exact: true }).click();
    await owner.getByRole('button', { name: '集成验证组织', exact: true }).click();
    await owner.getByRole('tab', { name: '成员与管理', exact: true }).click();
    await owner.getByRole('button', { name: '同意加入', exact: true }).click();
    await owner.getByRole('row').filter({ hasText: '浏览器新人' }).waitFor();
    await owner.getByRole('button', { name: '工作台', exact: true }).click();
    await owner
      .getByRole('group', { name: '空间类型', exact: true })
      .getByRole('button', { name: '组织空间', exact: true })
      .click();
    await owner.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
    await owner.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
    await owner.getByText('知识库权限与管理', { exact: true }).click();
    await owner.getByLabel('目标成员账号 ID', { exact: true }).fill(me.id);
    await owner.getByRole('button', { name: '更新本库创建资格', exact: true }).click();
    await owner.getByText('本库创建资格已更新', { exact: true }).waitFor();
    await page.getByRole('button', { name: '工作台', exact: true }).click();
    await page
      .getByRole('group', { name: '空间类型', exact: true })
      .getByRole('button', { name: '组织空间', exact: true })
      .click();
    await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
    await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
    await page.getByLabel('文档标题', { exact: true }).fill('新人创建的组织报告');
    await page.getByRole('button', { name: '创建文档', exact: true }).click();
    await page.getByRole('button', { name: '新人创建的组织报告', exact: true }).waitFor();
    assert.equal(errors.length, 0);
    await owner.screenshot({
      path: resolve(root, 'artifacts/batches/02/organization-management.png'),
      fullPage: true,
    });
    await c.close();
    await ownerContext.close();
  } finally {
    await browser.close();
  }
});
