import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { resolve } from 'node:path';
import { state, root } from '../../scripts/runtime-lib.mjs';
import { createUser, get, post, json, operation } from '../helpers/workspace.mjs';

// 真实API构造隔离个人文档，验证20条分页、状态筛选、昵称和弹窗原号恢复。
test('sharing management pages real records and archives without losing the original operation', async () => {
  const owner = await createUser(),
    applicant = await createUser();
  const space = json(await get('/v1/me/spaces', owner.headers)).items.find(
    (s) => s.kind === 'PERSONAL',
  );
  const made = await post(
    '/v1/documents',
    {
      operation_id: operation('sharing-ui-document'),
      space_id: space.id,
      title: '分享管理界面验证',
    },
    owner.headers,
  );
  assert.equal(made.status, 201);
  const id = json(made).result.entity_id;
  const base = '/v1/documents/' + id;
  const invites = [];
  for (let index = 0; index < 21; index++) {
    const response = await post(
      base + '/invitations',
      {
        operation_id: operation('sharing-ui-invite'),
        expected_acl_revision: '1',
        expires_in_hours: 48,
        requested_grants: [{ scope_kind: 'DOCUMENT', scope_id: id, capability: 'COMMENT' }],
      },
      owner.headers,
    );
    assert.equal(response.status, 200);
    invites.push(json(response));
  }
  const applied = await post(
    '/v1/document-invitations/applications',
    {
      operation_id: operation('sharing-ui-apply'),
      invitation_token: new URL(invites.at(-1).share_url).searchParams.get('invitation_token'),
      reason: '希望参与内容讨论，验证昵称与申请范围',
    },
    applicant.headers,
  );
  assert.equal(applied.status, 202);
  const applicantName = json(await get('/v1/account/me', applicant.headers)).display_name;
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const page = await browser.newPage({
    ignoreHTTPSErrors: true,
    viewport: { width: 1280, height: 960 },
  });
  page.setDefaultTimeout(10000);
  const errors = [],
    inputs = [],
    lists = [];
  page.on('pageerror', (e) => errors.push(e.message));
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (request.method() === 'GET' && url.pathname === base + '/invitations')
      lists.push(url.searchParams);
  });
  try {
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(owner.email);
    await page.getByLabel('登录密码', { exact: true }).fill(owner.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page.goto('https://localhost:' + state().ports.edge + '/documents/' + id);
    await page.getByRole('tab', { name: '协作', exact: true }).click();
    await page.getByRole('button', { name: '邀请链接', exact: true }).click();
    const panel = page.getByRole('region', { name: '邀请链接管理', exact: true });
    await expect(panel.locator('.sharing-list > li')).toHaveCount(20);
    await panel.getByRole('button', { name: '下一页', exact: true }).click();
    await expect(panel.locator('.sharing-list > li')).toHaveCount(1);
    await expect(panel.getByText('第 2 页', { exact: true })).toBeVisible();
    await panel.getByRole('button', { name: '上一页', exact: true }).click();
    await expect(panel.locator('.sharing-list > li')).toHaveCount(20);
    await panel.getByRole('button', { name: '关闭链接', exact: true }).first().click();
    await expect(panel.locator('.sharing-list > li')).toHaveCount(20);
    await expect(panel.getByRole('button', { name: '下一页', exact: true })).toBeDisabled();
    await panel.getByLabel('邀请状态', { exact: true }).selectOption('INACTIVE');
    await expect(panel.locator('.sharing-list > li')).toHaveCount(1);
    await expect(panel.getByText('已关闭', { exact: true })).toBeVisible();
    await expect(panel.getByRole('button', { name: '关闭链接', exact: true })).toHaveCount(0);
    await page.route('**/v1/document-invitations/*/archive', async (route) => {
      inputs.push(route.request().postDataJSON());
      const response = await route.fetch();
      assert.equal(response.status(), 200);
      if (inputs.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    await panel.getByRole('button', { name: '删除记录', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: '删除这条记录？', exact: true });
    await dialog.getByRole('button', { name: '确认删除记录', exact: true }).click();
    await dialog.getByRole('button', { name: '确认操作结果', exact: true }).click();
    await expect(dialog).not.toBeVisible();
    assert.equal(inputs.length, 2);
    assert.deepEqual(inputs[1], inputs[0]);
    await expect(panel.locator('.sharing-list > li')).toHaveCount(0);
    assert.ok(lists.every((params) => params.get('limit') === '20'));
    await page.getByRole('button', { name: '加入申请', exact: true }).click();
    const applications = page.getByRole('region', { name: '协作申请管理', exact: true });
    await expect(applications.getByText(applicantName, { exact: true })).toBeVisible();
    await expect(
      applications.getByText('希望参与内容讨论，验证昵称与申请范围', { exact: true }),
    ).toBeVisible();
    await applications.getByRole('button', { name: '拒绝', exact: true }).click();
    await expect(applications.locator('.sharing-list > li')).toHaveCount(0);
    await applications.getByRole('button', { name: '已处理', exact: true }).click();
    await expect(applications.getByText('已拒绝', { exact: true })).toBeVisible();
    await expect(applications.getByRole('button', { name: '同意', exact: true })).toHaveCount(0);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/sharing-management-browser.png'),
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
      false,
    );
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/sharing-management-mobile.png'),
      fullPage: true,
    });
    assert.deepEqual(errors, []);
  } finally {
    await browser.close();
  }
});
