import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';

// 全部业务接口模拟：核验菜单、取消及未知删除的原号续接，绝不移动用户的真实文档。
test('document trash lives in the header menu and confirms before preserving the original retry', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width: 1440, height: 1000 },
  });
  const page = await context.newPage(),
    writes = [],
    authorizations = [],
    errors = [];
  let managing = true;
  const metadata = {
    id: '91001',
    space_id: '92001',
    title: '文档操作验证',
    lifecycle: 'ACTIVE',
    responsible_user_id: '90001',
    control_revision: '3',
    acl_revision: '1',
    main_locked: true,
    visibility_policy: 'COLLABORATORS',
    publication_state: 'PUBLISHED',
  };
  const empty = { items: [], next_cursor: null, has_more: false };
  try {
    await context.addInitScript(() => {
      const slot = '1234567890abcdef1234567890abcdef';
      sessionStorage.setItem('collaboration-report.session-context', slot);
      sessionStorage.setItem(
        'collaboration-report.csrf',
        JSON.stringify({ user_id: '90001', token: 'mock-document-actions', context: slot }),
      );
    });
    page.setDefaultTimeout(10000);
    page.on('pageerror', (error) => errors.push(error.message));
    await page.route('**/v1/**', async (route) => {
      const request = route.request(),
        url = new URL(request.url()),
        path = url.pathname;
      const json = (value) => route.fulfill({ json: value });
      if (request.method() === 'POST' && path.endsWith('/authorization-changes')) {
        authorizations.push(request.postDataJSON());
        return json({
          operation_id: authorizations.at(-1).operation_id,
          state: 'SUCCEEDED',
          scope_key: 'document-acl:91001',
          epoch: '2',
          missing_participants: [],
        });
      }
      if (request.method() !== 'GET') {
        assert.equal(request.method(), 'DELETE');
        assert.equal(path, '/v1/documents/91001');
        writes.push(request.postDataJSON());
        // 第一次回复丢失：弹窗不能取消或另发删除，重试使用同一操作号、目标和修订。
        if (writes.length === 1) return route.abort('failed');
        return json({ operation_id: writes.at(-1).operation_id, state: 'SUCCEEDED', result: {} });
      }
      if (path === '/v1/account/me')
        return json({
          id: '90001',
          email: 'mock@example.test',
          display_name: '操作验证',
          state: 'ACTIVE',
          credential_epoch: '1',
          identity_epoch: '1',
        });
      if (path === '/v1/documents/91001')
        return route.fulfill({
          headers: {
            'X-Available-Actions': JSON.stringify(managing ? ['CHANGE_AUTHORIZATION'] : []),
          },
          json: metadata,
        });
      if (path.endsWith('/main-revision')) return json({ revision: 'b'.repeat(64) });
      if (path.endsWith('/collaborators'))
        return json({
          items: url.searchParams.has('cursor')
            ? [{ user_id: '90002', display_name: '分页找到的小林' }]
            : [{ user_id: '90001', display_name: '操作验证' }],
          next_cursor: url.searchParams.has('cursor') ? null : 'more-collaborators',
        });
      if (path.endsWith('/main'))
        return json({
          document: metadata,
          read_view_id: '97001',
          manifest_id: '96001',
          units: [],
          structure: { id: '93001', entries: [] },
          style: { id: '94001', style: { body_font: 'SimSun', body_size_pt: 12, margin_cm: 2.54 } },
        });
      if (
        path === '/v1/me/spaces' ||
        path.endsWith('/editable-units') ||
        path.endsWith('/invitations') ||
        path.endsWith('/applications')
      )
        return json(empty);
      return route.fulfill({ status: 404, json: { code: 'ENTITY_NOT_FOUND' } });
    });
    const base = 'https://localhost:' + state().ports.edge;
    // 文档授权与知识库成员使用同一可见选择器；验证隐藏旧选择不会再被提交。
    await page.goto(base + '/documents/91001?tab=collaboration');
    const picker = page.locator('.collaborator-management .member-directory'),
      search = picker.getByLabel('查找协作者', { exact: true }),
      submit = page.getByRole('button', { name: '提交授权变更', exact: true });
    await expect(submit).toBeDisabled();
    await picker.getByRole('button', { name: '选择协作者', exact: true }).click();
    await picker.getByRole('radio', { name: '操作验证（我）', exact: true }).check();
    await expect(submit).toBeEnabled();
    await expect(search).not.toBeVisible();
    await picker.getByRole('button', { name: '更换协作者', exact: true }).click();
    await search.fill('dadalfaf');
    await expect(
      picker.getByText('没有匹配的成员，请核对姓名或账号编号。', { exact: false }),
    ).toBeVisible();
    await expect(submit).toBeDisabled();
    await search.fill('小林');
    await expect(search).toHaveValue('小林');
    await picker.getByRole('radio', { name: '分页找到的小林', exact: true }).check();
    mkdirSync('artifacts/reported-usability/document-actions', { recursive: true });
    await page.screenshot({
      path: 'artifacts/reported-usability/document-actions/member-picker-desktop.png',
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await page.screenshot({
      path: 'artifacts/reported-usability/document-actions/member-picker-mobile.png',
      fullPage: true,
    });
    await submit.click();
    await expect(
      page.getByRole('status').filter({ hasText: '授权已更新，旧输出已关闭' }),
    ).toBeVisible();
    assert.equal(authorizations.length, 1);
    assert.equal(authorizations[0].changes[0].grant.user_id, '90002');
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(base + '/documents/91001?tab=settings');
    const settings = page.getByRole('tabpanel', { name: '文档设置', exact: true }),
      dialog = page.getByRole('dialog', { name: '移入回收站', exact: true });
    await expect(settings.getByRole('heading', { name: '文档信息', exact: true })).toBeVisible();
    await expect(settings.getByRole('button', { name: '移入回收站', exact: true })).toHaveCount(0);
    await expect(dialog).not.toBeVisible();
    await expect(page.getByRole('button', { name: '移入回收站', exact: true })).not.toBeVisible();
    await page.getByRole('button', { name: '更多操作', exact: true }).click();
    await page.getByRole('button', { name: '移入回收站', exact: true }).click();
    await expect(dialog).toContainText('文档操作验证');
    assert.equal(writes.length, 0, '进入确认弹窗不能提前删除');
    mkdirSync('artifacts/reported-usability/document-actions', { recursive: true });
    await page.screenshot({
      path: 'artifacts/reported-usability/document-actions/confirmation-desktop.png',
    });
    await dialog.getByRole('button', { name: '取消', exact: true }).click();
    assert.equal(writes.length, 0);

    await page.setViewportSize({ width: 390, height: 844 });
    await page.getByRole('button', { name: '更多操作', exact: true }).click();
    await page.getByRole('button', { name: '移入回收站', exact: true }).click();
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await page.screenshot({
      path: 'artifacts/reported-usability/document-actions/confirmation-mobile.png',
    });
    await dialog.getByRole('button', { name: '确认移入回收站', exact: true }).click();
    const retry = dialog.getByRole('button', { name: '确认操作结果', exact: true });
    await expect(retry).toBeVisible();
    await expect(dialog.getByRole('button', { name: '取消', exact: true })).toBeDisabled();
    await retry.click();
    await expect(dialog).not.toBeVisible();
    assert.equal(writes.length, 2);
    assert.deepEqual(writes[0], writes[1]);
    assert.equal(writes[0].expected_revision, '3');
    assert.match(writes[0].operation_id, /^[a-f0-9-]{36}$/);

    managing = false;
    await page.goto(base + '/documents/91001');
    await expect(page.getByRole('heading', { name: metadata.title, exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: '导出文档', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: '更多操作', exact: true })).toHaveCount(0);
    assert.equal(writes.length, 2, '没有管理资格不能新增删除请求');
    assert.deepEqual(errors, []);
  } finally {
    await context.close();
    await browser.close();
  }
});
