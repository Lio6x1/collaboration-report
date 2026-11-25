import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';
import { createUser } from '../helpers/account.mjs';
import {
  organizationFixture,
  documentFixture,
  post,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
// 真实文档已经初始化，随后目录503。页面只能补刷新，不能再初始化第二份文档。
test('confirmed document creation survives failed refresh without duplicate POST or navigation loss', async () => {
  const u = await createUser(),
    browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(6000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(u.email);
    await page.getByLabel('登录密码', { exact: true }).fill(u.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByLabel('文档标题', { exact: true }).waitFor();
    let confirmed = false,
      failed = false,
      writes = 0;
    await page.route('**/v1/**', async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path === '/v1/documents' && route.request().method() === 'POST') {
        writes++;
        const res = await route.fetch();
        assert.equal(res.status(), 201);
        confirmed = true;
        await route.fulfill({ response: res });
        return;
      }
      if (confirmed && !failed && /^\/v1\/spaces\/\d+\/documents$/.test(path)) {
        failed = true;
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({ code: 'DEPENDENCY_UNAVAILABLE' }),
        });
        return;
      }
      await route.continue();
    });
    await page.getByLabel('文档标题', { exact: true }).fill('只创建一份的报告');
    await page.getByRole('button', { name: '创建文档', exact: true }).click();
    await page.getByRole('button', { name: '确认操作结果', exact: true }).waitFor();
    assert.equal(
      await page.getByRole('button', { name: '刷新工作台', exact: true }).isDisabled(),
      true,
    );
    assert.equal(await page.getByRole('button', { name: '组织', exact: true }).isDisabled(), true);
    await page.getByRole('button', { name: '确认操作结果', exact: true }).click();
    await page.getByRole('button', { name: '只创建一份的报告', exact: true }).waitFor();
    assert.equal(writes, 1);
    assert.equal(await page.getByRole('button', { name: '确认操作结果', exact: true }).count(), 0);
    await context.close();
  } finally {
    await browser.close();
  }
});

test('confirmed member appointment refresh resumes its original organization after the detail is cleared', async () => {
  const f = await organizationFixture(1),
    browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(6000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
    await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '组织', exact: true }).click();
    await page.getByRole('button', { name: '集成验证组织', exact: true }).click();
    let changed = false,
      failed = false,
      writes = 0;
    await page.route('**/v1/organizations/**', async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith('/role') && route.request().method() === 'POST') {
        writes++;
        const res = await route.fetch();
        assert.equal(res.status(), 202);
        changed = true;
        await route.fulfill({ response: res });
        return;
      }
      if (
        changed &&
        !failed &&
        path === '/v1/organizations/' + f.organization &&
        route.request().method() === 'GET'
      ) {
        failed = true;
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({ code: 'DEPENDENCY_UNAVAILABLE' }),
        });
        return;
      }
      await route.continue();
    });
    await page
      .getByRole('row')
      .filter({ hasText: f.members[0].id })
      .getByRole('button', { name: '任命管理员', exact: true })
      .click();
    await page.getByRole('button', { name: '确认操作结果', exact: true }).click();
    await expect(page.getByRole('row').filter({ hasText: f.members[0].id })).toContainText(
      '组织管理员',
    );
    assert.equal(writes, 1);
    await context.close();
  } finally {
    await browser.close();
  }
});

test('confirmed responsible transfer reports success when the old reader subsequently receives 403', async () => {
  const f = await organizationFixture(2),
    u = f.members[0];
  await waitAuthorization(
    await post(
      '/v1/knowledge-bases/' + f.kb + '/creation-grants',
      { operation_id: operation('create'), expected_revision: '1', user_id: u.id, allowed: true },
      f.owner.headers,
    ),
    f.owner,
  );
  const d = await documentFixture(f, u),
    browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage();
    page.setDefaultTimeout(6000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(u.email);
    await page.getByLabel('登录密码', { exact: true }).fill(u.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page.goto('https://localhost:' + state().ports.edge + '/documents/' + d.id);
    await page.getByLabel('目标账号 ID', { exact: true }).fill(f.members[1].id);
    await page.getByRole('button', { name: '直接转交给目标账号', exact: true }).click();
    await page.getByText('负责人已转交；当前资格已变化，旧视图已关闭', { exact: true }).waitFor();
    assert.equal(await page.getByRole('alert').count(), 0);
    assert.equal(await page.getByRole('button', { name: '确认操作结果', exact: true }).count(), 0);
    await context.close();
  } finally {
    await browser.close();
  }
});
