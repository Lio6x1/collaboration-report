import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

test('真实浏览器通知已读持久化并通过当前资格打开负责人文档', { timeout: 60000 }, async () => {
  const f = await organizationFixture(1);
  const member = f.members[0];
  const document = await documentFixture(f);
  const current = await get(`/v1/documents/${document.id}`, f.owner.headers);
  assert.equal(current.status, 200, current.body);
  // 通知由真实转让事务创建；浏览器只使用接任成员的登录会话。
  const transferred = await post(
    `/v1/documents/${document.id}/responsible-transfers`,
    {
      operation_id: operation('browser-notification-transfer'),
      expected_revision: json(current).control_revision,
      target_user_id: member.id,
    },
    f.owner.headers,
  );
  assert.equal(transferred.status, 200, transferred.body);
  assert.equal((await waitAuthorization(transferred, f.owner)).state, 'SUCCEEDED');

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
    await page.getByLabel('登录邮箱', { exact: true }).fill(member.email);
    await page.getByLabel('登录密码', { exact: true }).fill(member.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page.getByRole('button', { name: '通知', exact: true }).click();
    const panel = page.getByRole('region', { name: '通知', exact: true });
    const notice = panel.getByRole('listitem').filter({ hasText: '你已成为文档负责人' });
    await notice.getByText('未读', { exact: true }).waitFor();
    assert.match(await notice.innerText(), new RegExp(`目标：\\s*${document.id}(?:\\s|$)`));
    await notice.getByRole('button', { name: '标记已读', exact: true }).click();
    await notice.getByText('已读', { exact: true }).waitFor();
    await notice.getByRole('button', { name: '打开相关内容', exact: true }).click();
    await page.getByRole('heading', { name: '协作报告', exact: true, level: 1 }).waitFor();
    await page.getByRole('navigation', { name: '固定目录', exact: true }).waitFor();
    await page.getByRole('button', { name: '返回通知', exact: true }).click();
    await notice.getByText('已读', { exact: true }).waitFor();

    // 整页刷新后重新进入通知中心，避免把组件内的乐观状态当作已读持久化。
    await page.reload();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page.getByRole('button', { name: '通知', exact: true }).click();
    await notice.getByText('已读', { exact: true }).waitFor();
    assert.equal(await notice.getByRole('button', { name: '标记已读', exact: true }).count(), 0);
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/06/notifications-browser.png'),
      fullPage: true,
    });
    await context.close();
  } finally {
    await browser.close();
  }
});
