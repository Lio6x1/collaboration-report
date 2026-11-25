import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdir } from 'node:fs/promises';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';

// 真实浏览器验证页面组织；替换业务HTTP，不发送邮件、不创建/删除用户数据。
test('工作台固定入口、历史恢复和短弹窗保持各自的工作范围', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width: 1440, height: 1000 },
  });
  await context.addInitScript(() => {
    const slot = 'a123456789abcdefa123456789abcdef';
    sessionStorage.setItem('collaboration-report.session-context', slot);
    sessionStorage.setItem(
      'collaboration-report.csrf',
      JSON.stringify({
        user_id: '91001',
        token: 'mock-navigation',
        context: slot,
      }),
    );
  });
  const page = await context.newPage();
  page.setDefaultTimeout(7000);
  const errors = [],
    requests = [];
  let noOrganization = false;
  page.on('pageerror', (error) => errors.push(error.message));
  await page.route('**/v1/**', async (route) => {
    const request = route.request(),
      url = new URL(request.url()),
      path = url.pathname;
    requests.push({ path, method: request.method() });
    const json = (value, actions) =>
      route.fulfill({
        json: value,
        ...(actions ? { headers: { 'X-Available-Actions': JSON.stringify(actions) } } : {}),
      });
    const pageOf = (items) => ({ items, next_cursor: null, has_more: false });
    if (path === '/v1/account/me')
      return json({
        id: '91001',
        email: 'navigation@example.test',
        display_name: '导航体验',
        state: 'ACTIVE',
        credential_epoch: '1',
        identity_epoch: '1',
      });
    if (path === '/v1/organizations')
      return json(
        pageOf(
          noOrganization
            ? []
            : [
                { id: '96001', name: '导航协作组' },
                { id: '96002', name: '第二个组织' },
              ],
        ),
      );
    if (path === '/v1/me/spaces') {
      if (url.searchParams.get('cursor'))
        return json(
          pageOf(
            noOrganization
              ? []
              : [
                  {
                    id: '92002',
                    kind: 'ORGANIZATION',
                    organization_id: '96001',
                    actions: ['CREATE_KNOWLEDGE_BASE'],
                  },
                  { id: '92003', kind: 'ORGANIZATION', organization_id: '96002', actions: [] },
                ],
          ),
        );
      return json({
        ...pageOf([
          { id: '92001', kind: 'PERSONAL', actions: ['CREATE_KNOWLEDGE_BASE', 'CREATE_DOCUMENT'] },
        ]),
        next_cursor: noOrganization ? null : 'organization-page',
        has_more: !noOrganization,
      });
    }
    if (path === '/v1/spaces/92002/knowledge-bases')
      return json(
        pageOf([
          {
            id: '93002',
            space_id: '92002',
            name: '组织专属知识库',
            acl_revision: '1',
            visibility_revision: '1',
            visibility_policy: 'COLLABORATORS',
          },
        ]),
      );
    if (path === '/v1/spaces/92003/knowledge-bases')
      return json(
        pageOf([
          {
            id: '93003',
            space_id: '92003',
            name: '第二组织的知识库',
            acl_revision: '1',
            visibility_revision: '1',
            visibility_policy: 'COLLABORATORS',
          },
        ]),
      );
    if (path === '/v1/spaces/92001/knowledge-bases')
      return json(
        pageOf([
          {
            id: '93001',
            space_id: '92001',
            name: '导航样本知识库',
            acl_revision: '1',
            visibility_revision: '1',
            visibility_policy: 'COLLABORATORS',
          },
        ]),
      );
    if (path === '/v1/knowledge-bases/93001')
      return json(
        {
          id: '93001',
          space_id: '92001',
          name: '导航样本知识库',
          acl_revision: '1',
          visibility_revision: '1',
          visibility_policy: 'COLLABORATORS',
        },
        ['RENAME', 'DELETE', 'CHANGE_VISIBILITY', 'CREATE_DOCUMENT', 'VIEW_MEMBERS'],
      );
    if (path === '/v1/knowledge-bases/93001/members')
      return json(
        pageOf([{ user_id: '91001', display_name: '导航体验', role: 'ADMIN', state: 'ACTIVE' }]),
      );
    if (path === '/v1/document-applications')
      return json(
        pageOf([{ id: '95001', document_id: '94001', state: 'PENDING', reason: '导航样本申请' }]),
      );
    if (path.includes('/trash')) return json(pageOf([]));
    if (path.endsWith('/documents'))
      return json(
        pageOf([
          { id: '94001', title: '导航样本文档', publication_state: 'DRAFT', main_locked: false },
        ]),
      );
    return json(pageOf([]));
  });
  try {
    const origin = 'https://localhost:' + state().ports.edge;
    await page.goto(origin + '/');
    const nav = page.getByRole('navigation', { name: '工作空间导航', exact: true });
    await expect(nav).toBeVisible();
    await expect(
      page.getByRole('button', { name: '打开知识库：导航样本知识库', exact: true }),
    ).toBeVisible();
    const scope = page.getByRole('group', { name: '空间类型', exact: true }),
      breadcrumb = page.getByRole('navigation', { name: '知识库位置', exact: true });
    await expect(breadcrumb).toHaveText('个人空间');
    await expect(scope.getByRole('button', { name: '个人空间', exact: true })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    // 组织在后页：类型切换不能把个人知识库当组织内容，也不能冒称已读完所有组织。
    await scope.getByRole('button', { name: '组织空间', exact: true }).click();
    await expect(breadcrumb).toHaveText('组织空间');
    await expect(
      page.getByRole('button', { name: '打开知识库：导航样本知识库', exact: true }),
    ).not.toBeVisible();
    await page.getByRole('button', { name: '更多组织空间', exact: true }).click();
    await expect(page.getByLabel('组织', { exact: true })).toHaveValue('92002');
    await expect(breadcrumb).toHaveText('组织空间 · 导航协作组');
    await expect(
      page.getByRole('button', { name: '打开知识库：组织专属知识库', exact: true }),
    ).toBeVisible();
    await expect(nav.getByRole('button', { name: '协作申请', exact: true })).not.toBeVisible();
    await page.getByRole('button', { name: '新建', exact: true }).click();
    const organizationCreate = page.getByRole('dialog', { name: '新建内容', exact: true });
    await expect(
      organizationCreate.getByText('创建到：组织空间 · 导航协作组', { exact: true }),
    ).toBeVisible();
    await expect(organizationCreate.getByLabel('知识库名称', { exact: true })).toBeVisible();
    await expect(
      organizationCreate.getByRole('button', { name: '文档', exact: true }),
    ).not.toBeVisible();
    await page.keyboard.press('Escape');
    await page.getByLabel('组织', { exact: true }).selectOption('92003');
    await expect(breadcrumb).toHaveText('组织空间 · 第二个组织');
    await expect(
      page.getByRole('button', { name: '打开知识库：第二组织的知识库', exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole('button', { name: '打开知识库：组织专属知识库', exact: true }),
    ).not.toBeVisible();
    await expect(page.getByRole('button', { name: '新建', exact: true })).not.toBeVisible();
    await page.getByLabel('组织', { exact: true }).selectOption('92002');
    await page.reload();
    await expect(scope.getByRole('button', { name: '组织空间', exact: true })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await expect(breadcrumb).toHaveText('组织空间 · 导航协作组');
    await mkdir('artifacts/frontend-navigation', { recursive: true });
    await page.screenshot({
      path: 'artifacts/frontend-navigation/organization-scope-desktop.png',
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await page.screenshot({
      path: 'artifacts/frontend-navigation/organization-scope-mobile.png',
      fullPage: true,
    });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await scope.getByRole('button', { name: '个人空间', exact: true }).click();
    await expect(breadcrumb).toHaveText('个人空间');
    await expect(
      page.getByRole('button', { name: '打开知识库：导航样本知识库', exact: true }),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: '刷新工作台', exact: true })).toBeEnabled();
    await page.goBack();
    await expect(breadcrumb).toHaveText('组织空间 · 导航协作组');
    await expect(page.getByRole('button', { name: '刷新工作台', exact: true })).toBeEnabled();
    await page.goForward();
    await expect(breadcrumb).toHaveText('个人空间');
    assert.equal(
      await page.locator('.workspace-shell details').count(),
      0,
      '核心功能不再依赖大块折叠',
    );
    await nav.getByRole('button', { name: '全部文档', exact: true }).click();
    await expect(page.getByRole('link').filter({ hasText: '导航样本文档' })).toBeVisible();
    await page.reload();
    await expect(nav.getByRole('button', { name: '全部文档', exact: true })).toHaveAttribute(
      'aria-current',
      'page',
    );
    await nav.getByRole('button', { name: '协作申请', exact: true }).click();
    await expect(page.getByText('导航样本申请', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: '申请文档协作', exact: true }).click();
    const invite = page.getByRole('dialog', { name: '申请文档协作', exact: true });
    await invite.getByLabel('文档申请说明', { exact: true }).fill('保留我的输入');
    await page.keyboard.press('Escape');
    await expect(invite).not.toBeVisible();
    await page.getByRole('button', { name: '申请文档协作', exact: true }).click();
    await expect(invite.getByLabel('文档申请说明', { exact: true })).toHaveValue('保留我的输入');
    await page.keyboard.press('Escape');
    await nav.getByRole('button', { name: '回收站', exact: true }).click();
    await expect(page.getByRole('region', { name: '文档回收站', exact: true })).toBeVisible();
    await page.goBack();
    await expect(page.getByText('导航样本申请', { exact: true })).toBeVisible();
    await nav.getByRole('button', { name: '知识库', exact: true }).click();
    await page.getByRole('button', { name: '打开知识库：导航样本知识库', exact: true }).click();
    await page.getByRole('button', { name: '知识库设置', exact: true }).click();
    await expect(page.getByRole('tab', { name: '基本信息', exact: true })).toBeVisible();
    await page.getByRole('tab', { name: '成员', exact: true }).click();
    await expect(
      page.getByRole('cell', { name: '导航体验 账号 00091001', exact: true }),
    ).toBeVisible();
    await page.getByRole('button', { name: '返回文档目录', exact: true }).click();
    await page.getByRole('button', { name: '新建', exact: true }).click();
    const create = page.getByRole('dialog', { name: '新建内容', exact: true });
    await expect(create.getByLabel('文档标题', { exact: true })).toBeVisible();
    await page.keyboard.press('Escape');
    await mkdir('artifacts/frontend-navigation', { recursive: true });
    await page.screenshot({
      path: 'artifacts/frontend-navigation/workspace-desktop.png',
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await expect(nav.getByRole('button', { name: '回收站', exact: true })).toBeVisible();
    await page.screenshot({
      path: 'artifacts/frontend-navigation/workspace-mobile.png',
      fullPage: true,
    });
    await scope.getByRole('button', { name: '组织空间', exact: true }).click();
    await page.getByRole('button', { name: '更多组织空间', exact: true }).click();
    await expect(breadcrumb).toHaveText('组织空间 · 导航协作组');
    noOrganization = true;
    await page.getByRole('button', { name: '刷新工作台', exact: true }).click();
    await expect(breadcrumb).toHaveText('组织空间');
    await expect(
      page.getByText('你还没有可访问的组织空间，可在组织页面创建或申请加入组织。', { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole('button', { name: '打开知识库：组织专属知识库', exact: true }),
    ).not.toBeVisible();
    await expect(page.getByRole('button', { name: '新建', exact: true })).not.toBeVisible();
    assert.equal(
      requests.some((request) => request.method !== 'GET'),
      false,
    );
    assert.deepEqual(errors, []);
  } finally {
    await context.close();
    await browser.close();
  }
});
