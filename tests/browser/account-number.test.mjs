import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';

// 只拦截合成账号/文档：验收展示、复制失败反馈和补零输入的真实提交边界，不转让用户文档。
test('account number copies eight digits while transfer submits the original account ID', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    await context.addInitScript(() => {
      const slot = '1234567890abcdef1234567890abcdef';
      sessionStorage.setItem('collaboration-report.session-context', slot);
      sessionStorage.setItem(
        'collaboration-report.csrf',
        JSON.stringify({
          user_id: '7764',
          token: 'mock-account-number',
          context: slot,
        }),
      );
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          async writeText(value) {
            if (window.copyFailure) throw new Error('clipboard denied');
            window.copiedNumber = value;
          },
        },
      });
    });
    const page = await context.newPage(),
      writes = [],
      errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    const metadata = {
      id: '91001',
      title: '账号编号提交验证',
      responsible_user_id: '7764',
      control_revision: '3',
      acl_revision: '1',
      lifecycle: 'ACTIVE',
      main_locked: false,
      visibility_policy: 'COLLABORATORS',
      publication_state: 'UNPUBLISHED',
    };
    const emptyPage = { items: [], next_cursor: null, has_more: false };
    await page.route('**/v1/**', async (route) => {
      const request = route.request(),
        path = new URL(request.url()).pathname;
      if (request.method() !== 'GET') {
        assert.equal(path, '/v1/documents/91001/personal-transfers');
        writes.push(request.postDataJSON());
        return route.fulfill({
          json: {
            operation_id: writes.at(-1).operation_id,
            state: 'SUCCEEDED',
            scope_key: 'personal-transfer:91001',
            epoch: '1',
            missing_participants: [],
          },
        });
      }
      if (path === '/v1/account/me')
        return route.fulfill({
          json: {
            id: '7764',
            display_name: '编号验证',
            email: 'mock@example.test',
            state: 'ACTIVE',
            credential_epoch: '1',
            identity_epoch: '1',
          },
        });
      if (path === '/v1/documents/91001')
        return route.fulfill({
          headers: { 'X-Available-Actions': JSON.stringify(['MANAGE_INVITATIONS']) },
          json: metadata,
        });
      if (path.endsWith('/main-revision'))
        return route.fulfill({ json: { revision: 'b'.repeat(64) } });
      if (path.endsWith('/main'))
        return route.fulfill({
          json: {
            document: metadata,
            read_view_id: '97001',
            manifest_id: '96001',
            units: [],
            structure: { entries: [] },
            style: { style: { body_font: 'SimSun', body_size_pt: 12 } },
          },
        });
      if (
        ['/editable-units', '/invitations', '/applications'].some((suffix) => path.endsWith(suffix))
      )
        return route.fulfill({ json: emptyPage });
      return route.fulfill({ status: 404, json: { code: 'ENTITY_NOT_FOUND' } });
    });
    const base = 'https://localhost:' + state().ports.edge;
    await page.goto(base + '/account');
    const profile = page
      .locator('.account-grid .panel')
      .filter({ has: page.getByRole('heading', { name: '账号信息', exact: true }) });
    await expect(profile).toContainText('账号编号：00007764');
    await profile.getByRole('button', { name: '复制账号编号', exact: true }).click();
    await expect(profile.getByRole('status')).toHaveText('账号编号已复制');
    assert.equal(await page.evaluate(() => window.copiedNumber), '00007764');
    await page.evaluate(() => {
      window.copyFailure = true;
    });
    await profile.getByRole('button', { name: '复制账号编号', exact: true }).click();
    await expect(profile.getByRole('status')).toContainText('手动复制');
    assert.equal(writes.length, 0, '复制账号编号不能发起账号写请求');
    await page.goto(base + '/documents/91001?tab=collaboration');
    await page.getByRole('button', { name: '转交文档', exact: true }).click();
    await page.getByRole('button', { name: '转入他人个人空间', exact: true }).click();
    const target = page.getByLabel('个人转让接收人编号', { exact: true });
    const submit = page.getByRole('button', { name: '发送个人转让申请', exact: true });
    await target.fill('00000000');
    await expect(submit).toBeDisabled();
    await target.fill('00002002');
    await submit.click();
    await expect(page.getByRole('status')).toContainText('转让申请已发送');
    assert.equal(writes.length, 1);
    assert.equal(writes[0].target_user_id, '2002');
    assert.equal(writes[0].expected_revision, '3');
    assert.match(writes[0].operation_id, /^[a-f0-9-]{36}$/);
    assert.deepEqual(errors, []);
  } finally {
    await browser.close();
  }
});
