import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';

const organization = (id, name) => ({
  id,
  name,
  owner_id: '90001',
  lifecycle: 'ACTIVE',
  revision: '1',
  member_revision: '1',
});
const knowledgeBase = (id, space, name) => ({
  id,
  space_id: space,
  name,
  lifecycle: 'ACTIVE',
  acl_revision: '1',
  visibility_policy: 'COLLABORATORS',
  visibility_revision: '1',
});
const document = (id, space, kb, title) => ({
  id,
  space_id: space,
  knowledge_base_id: kb,
  title,
  lifecycle: 'ACTIVE',
  main_locked: true,
  current_structure_id: '94001',
  current_style_id: '95001',
  current_manifest_id: '96001',
  publication_state: 'UNPUBLISHED',
  control_revision: '1',
  acl_revision: '1',
  publication_epoch: '1',
  visibility_policy: 'COLLABORATORS',
  visibility_revision: '1',
});
const pageOf = (items, cursor = null) => ({ items, next_cursor: cursor, has_more: !!cursor });

// 组织入口只验证本次变化：目录复用、真实分页、切换的迟到响应和撤权清屏。
// 所有业务API均模拟，不创建组织/文档，不读取用户已有正文。
test('organization content reuses the authorized directory and isolates switched or revoked scopes', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width: 1440, height: 1000 },
  });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  const firstOrg = organization('91001', '组织甲'),
    secondOrg = { ...organization('91002', '组织乙'), owner_id: '99999' };
  const firstKB = knowledgeBase('93001', '92001', '甲的知识库'),
    laterKB = knowledgeBase('93002', '92001', '分页后的知识库');
  const otherKB = knowledgeBase('93003', '92002', '乙的知识库');
  const firstDoc = document('94001', '92001', firstKB.id, '甲的文档'),
    secondDoc = document('94002', '92001', firstKB.id, '甲的下一页文档');
  const libraryDoc = document('94003', '92001', laterKB.id, '分页知识库中的文档'),
    otherDoc = document('94004', '92002', otherKB.id, '乙的文档');
  const requests = [],
    errors = [],
    healthRequests = [];
  let delayLibrary = false,
    releaseLibrary,
    denyOtherDocuments = false,
    emptyOrganizations = false;
  try {
    await context.addInitScript(() => {
      const slot = '1234567890abcdef1234567890abcdef';
      sessionStorage.setItem('collaboration-report.session-context', slot);
      sessionStorage.setItem(
        'collaboration-report.csrf',
        JSON.stringify({ user_id: '90001', token: 'mock-organization-content', context: slot }),
      );
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: async (text) => {
            window.copiedOrganizationNumber = text;
          },
        },
      });
    });
    page.on('pageerror', (failure) => errors.push(failure.message));
    page.on('request', (request) => {
      if (new URL(request.url()).pathname.startsWith('/ops/health/'))
        healthRequests.push(request.url());
    });
    await page.route('**/v1/**', async (route) => {
      const request = route.request(),
        url = new URL(request.url()),
        path = url.pathname;
      requests.push({
        path,
        cursor: url.searchParams.get('cursor'),
        query: url.searchParams.get('q'),
        method: request.method(),
        input: request.method() === 'GET' ? null : request.postDataJSON(),
      });
      const json = (value) => route.fulfill({ json: value });
      if (
        request.method() === 'POST' &&
        path === '/v1/organizations/' + firstOrg.id + '/applications'
      )
        return json({
          operation_id: request.postDataJSON().operation_id,
          state: 'SUCCEEDED',
          result: {},
        });
      if (path === '/v1/account/operations/mock-organization-close')
        return json({ operation_id: 'mock-organization-close', state: 'SUCCEEDED' });
      if (path === '/v1/account/me')
        return json({
          id: '90001',
          email: 'mock@example.test',
          display_name: '组织浏览验证',
          state: 'ACTIVE',
          credential_epoch: '1',
          identity_epoch: '1',
        });
      if (path === '/v1/organizations')
        return json(pageOf(emptyOrganizations ? [] : [firstOrg, secondOrg]));
      if (path === '/v1/organizations/' + firstOrg.id) return json(firstOrg);
      if (path === '/v1/organizations/' + secondOrg.id) return json(secondOrg);
      if (path === '/v1/account/applications')
        return json(
          pageOf([
            {
              id: '97001',
              organization_id: secondOrg.id,
              state: 'PENDING',
              reason: '模拟申请进度',
            },
          ]),
        );
      if (path.endsWith('/applications') || path.endsWith('/invitations')) return json(pageOf([]));
      if (path.endsWith('/members'))
        return json(
          pageOf([
            {
              user_id: '90001',
              display_name: '组织浏览验证',
              role: path.includes(secondOrg.id) ? 'MEMBER' : 'ADMIN',
              state: 'ACTIVE',
              revision: '1',
              knowledge_base_creation: 'ROLE',
            },
            ...(path.includes(secondOrg.id)
              ? [
                  {
                    user_id: secondOrg.owner_id,
                    display_name: '组织乙的所有者',
                    role: 'ADMIN',
                    state: 'ACTIVE',
                    revision: '1',
                    knowledge_base_creation: 'ROLE',
                  },
                ]
              : []),
          ]),
        );
      if (path === '/v1/me/spaces')
        return json(
          url.searchParams.has('cursor')
            ? pageOf([
                {
                  id: '92001',
                  kind: 'ORGANIZATION',
                  organization_id: firstOrg.id,
                  lifecycle: 'ACTIVE',
                  actions: [],
                },
              ])
            : pageOf(
                [
                  {
                    id: '92999',
                    kind: 'PERSONAL',
                    owner_user_id: '90001',
                    lifecycle: 'ACTIVE',
                    actions: ['CREATE_DOCUMENT'],
                  },
                  {
                    id: '92002',
                    kind: 'ORGANIZATION',
                    organization_id: secondOrg.id,
                    lifecycle: 'ACTIVE',
                    actions: [],
                  },
                ],
                'find-first-organization',
              ),
        );
      if (path === '/v1/spaces/92001/knowledge-bases') {
        const query = url.searchParams.get('q')?.trim().toLocaleLowerCase();
        // 搜索在完整授权目录中先执行；不能只对当前卡片页作客户端筛选。
        if (query)
          return json(
            pageOf([firstKB, laterKB].filter((kb) => kb.name.toLocaleLowerCase().includes(query))),
          );
        return json(
          url.searchParams.has('cursor')
            ? pageOf([laterKB])
            : pageOf([firstKB], 'next-knowledge-bases'),
        );
      }
      if (path === '/v1/spaces/92002/knowledge-bases') return json(pageOf([otherKB]));
      if (path === '/v1/spaces/92001/documents')
        return json(
          url.searchParams.has('cursor')
            ? pageOf([secondDoc])
            : pageOf([firstDoc], 'next-documents'),
        );
      if (path === '/v1/spaces/92002/documents')
        return denyOtherDocuments
          ? route.fulfill({ status: 403, json: { code: 'DOCUMENT_READ_NOT_ALLOWED' } })
          : json(pageOf([otherDoc], 'next-other-documents'));
      if (path === '/v1/knowledge-bases/' + laterKB.id) {
        if (delayLibrary) await new Promise((resolve) => (releaseLibrary = resolve));
        // 组织所有者也不能从身份推导本库创建动作，本例有权浏览但动作摘要为空。
        return route.fulfill({
          headers: { 'content-type': 'application/json', 'X-Available-Actions': '[]' },
          body: JSON.stringify(laterKB),
        });
      }
      if (path === '/v1/knowledge-bases/' + laterKB.id + '/documents')
        return json(pageOf([libraryDoc]));
      return route.fulfill({ status: 403, json: { code: 'DOCUMENT_READ_NOT_ALLOWED' } });
    });
    await page.goto('https://localhost:' + state().ports.edge + '/organizations');
    const content = page.getByRole('tabpanel', { name: '内容', exact: true });
    const organizationSelector = page.getByRole('region', { name: '我的组织', exact: true }),
      membership = page.getByRole('dialog', { name: '创建或加入组织', exact: true });
    await expect(
      content.getByRole('button', { name: '打开知识库：' + firstKB.name, exact: true }),
    ).toBeVisible();
    await expect(page.getByText('服务运行状态', { exact: true })).toHaveCount(0);
    await expect(page.getByText('组织 ID ' + firstOrg.id, { exact: false })).toHaveCount(0);
    await expect(page.getByText('所有者：组织浏览验证（你）', { exact: true })).toBeVisible();
    const heading = page.locator('.organization-detail-heading');
    await expect(heading.getByRole('button', { name: '更多操作', exact: true })).toBeVisible();
    await expect(page.locator('.organization-secondary-actions')).toHaveCount(0);
    await expect(page.getByText('组织编号：000' + firstOrg.id, { exact: true })).toBeVisible();
    await page.getByRole('button', { name: '复制组织编号', exact: true }).click();
    await expect(page.getByRole('status').filter({ hasText: '组织编号已复制' })).toBeVisible();
    assert.equal(await page.evaluate(() => window.copiedOrganizationNumber), '000' + firstOrg.id);
    assert.equal(
      requests.some((item) => item.path.endsWith('/documents')),
      false,
      '知识库首页不预取各库文档',
    );
    await content.getByLabel('搜索知识库', { exact: true }).fill('不存在的库');
    await content.getByRole('search').getByRole('button', { name: '搜索', exact: true }).click();
    await expect(
      content.getByRole('button', { name: '打开知识库：' + firstKB.name, exact: true }),
    ).toHaveCount(0);
    assert.ok(
      requests.some(
        (item) =>
          item.path === '/v1/spaces/92001/knowledge-bases' &&
          item.query === '不存在的库' &&
          !item.cursor,
      ),
      '搜索必须把q交给服务端并从首页开始',
    );
    await content.getByLabel('搜索知识库', { exact: true }).fill(laterKB.name);
    await content.getByRole('search').getByRole('button', { name: '搜索', exact: true }).click();
    await expect(
      content.getByRole('button', { name: '打开知识库：' + laterKB.name, exact: true }),
    ).toBeVisible();
    await expect(
      content.getByRole('button', { name: '打开知识库：' + firstKB.name, exact: true }),
    ).toHaveCount(0);
    await content.getByLabel('搜索知识库', { exact: true }).fill('');
    await content.getByRole('search').getByRole('button', { name: '搜索', exact: true }).click();
    await expect(
      content.getByRole('button', { name: '打开知识库：' + firstKB.name, exact: true }),
    ).toBeVisible();
    mkdirSync('artifacts/reported-usability/organization-content', { recursive: true });
    await page.screenshot({
      path: 'artifacts/reported-usability/organization-content/library-gallery.png',
      fullPage: true,
    });
    await content.getByRole('button', { name: '全部文档', exact: true }).click();
    await expect(content.getByText(firstDoc.title, { exact: true })).toBeVisible();
    // 首屏优先当前内容；短弹窗保留填写草稿，申请进度有独立入口。
    await expect(content.getByText(firstDoc.title, { exact: true })).toBeInViewport();
    await expect(
      organizationSelector.getByRole('button', { name: firstOrg.name, exact: true }),
    ).toHaveAttribute('aria-pressed', 'true');
    await expect(membership).not.toBeVisible();
    await expect(membership.getByLabel('组织名称', { exact: true })).not.toBeVisible();
    await page.getByRole('button', { name: '创建或加入组织', exact: true }).click();
    await expect(membership).toBeVisible();
    await membership.getByLabel('组织名称', { exact: true }).fill('尚未提交的组织草稿');
    await expect(membership.getByText('模拟申请进度', { exact: true })).toHaveCount(0);
    await membership.getByRole('button', { name: '关闭弹窗', exact: true }).click();
    await expect(membership.getByLabel('组织名称', { exact: true })).not.toBeVisible();
    await page.getByRole('button', { name: '创建或加入组织', exact: true }).click();
    await expect(membership.getByLabel('组织名称', { exact: true })).toHaveValue(
      '尚未提交的组织草稿',
    );
    await membership.getByRole('button', { name: '关闭弹窗', exact: true }).click();
    await page.getByRole('button', { name: '我的组织申请', exact: true }).click();
    await expect(
      page.getByRole('region', { name: '组织申请进度' }).getByText('模拟申请进度', { exact: true }),
    ).toBeVisible();
    await expect(content).toHaveCount(0);
    await page.getByRole('button', { name: '组织内容', exact: true }).click();
    await content.getByRole('button', { name: '全部文档', exact: true }).click();
    await expect(content.getByText(firstDoc.title, { exact: true })).toBeVisible();
    await page.evaluate(() => window.scrollTo(0, 0));
    mkdirSync('artifacts/reported-usability/organization-content', { recursive: true });
    await page.screenshot({
      path: 'artifacts/reported-usability/organization-content/desktop.png',
      fullPage: true,
    });
    await expect(page.getByRole('tab', { name: '内容', exact: true })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await expect(content.getByRole('group', { name: '空间类型', exact: true })).toHaveCount(0);
    await expect(content.getByLabel('组织', { exact: true })).toHaveCount(0);
    await expect(content.getByText('个人文档协作邀请与我的申请', { exact: true })).toHaveCount(0);
    assert.equal(
      requests.some((item) => item.path.startsWith('/v1/spaces/92999/')),
      false,
    );
    assert.equal(
      requests.some((item) => item.path.startsWith('/v1/spaces/92002/')),
      false,
    );
    assert.equal(
      requests.filter((item) => item.path === '/v1/me/spaces').length,
      4,
      '切回组织内容时重新核验后页空间，不能复用旧权限',
    );
    assert.equal(
      requests.some((item) => item.path.endsWith('/members')),
      false,
      '内容页不预取成员管理数据',
    );
    await content
      .getByRole('navigation', { name: '文档分页', exact: true })
      .getByRole('button', { name: '下一页', exact: true })
      .click();
    await expect(content.getByText(secondDoc.title, { exact: true })).toBeVisible();
    await expect(content.getByText(firstDoc.title, { exact: true })).toHaveCount(0);
    await content
      .getByRole('navigation', { name: '工作空间导航' })
      .getByRole('button', { name: '知识库', exact: true })
      .click();
    await content
      .getByRole('navigation', { name: '知识库分页', exact: true })
      .getByRole('button', { name: '下一页', exact: true })
      .click();
    await expect(
      content.getByRole('button', { name: '打开知识库：' + firstKB.name, exact: true }),
    ).toHaveCount(0);
    await content.getByRole('button', { name: '打开知识库：' + laterKB.name, exact: true }).click();
    await expect(content.getByText(libraryDoc.title, { exact: true })).toBeVisible();
    await expect(content.getByRole('button', { name: '新建', exact: true })).toHaveCount(0);
    await expect(content.getByLabel('文档标题', { exact: true })).toHaveCount(0);
    await expect(content.getByText('知识库设置', { exact: true })).toHaveCount(0);
    await page.getByRole('tab', { name: '成员与管理', exact: true }).click();
    await expect(page.getByRole('button', { name: '生成组织邀请', exact: true })).toBeVisible();
    // 关闭组织只在二级菜单中，误触入口不会提交；名称确认在每次重开时清空。
    const closeDialog = page.getByRole('dialog', { name: '关闭组织', exact: true });
    await expect(page.getByRole('button', { name: '关闭组织…', exact: true })).not.toBeVisible();
    await page.getByRole('button', { name: '更多操作', exact: true }).click();
    await page.getByRole('button', { name: '关闭组织…', exact: true }).click();
    await expect(closeDialog).toBeVisible();
    const confirmClose = closeDialog.getByRole('button', { name: '关闭组织并清理', exact: true }),
      confirmationName = closeDialog.getByLabel('输入组织名称以确认关闭', { exact: true });
    await expect(confirmClose).toBeDisabled();
    await confirmationName.fill(secondOrg.name);
    await expect(confirmClose).toBeDisabled();
    await confirmationName.fill(firstOrg.name);
    await expect(confirmClose).toBeEnabled();
    await page.screenshot({
      path: 'artifacts/reported-usability/organization-content/close-confirmation.png',
      fullPage: true,
    });
    await closeDialog.getByRole('button', { name: '取消', exact: true }).click();
    await page.getByRole('button', { name: '更多操作', exact: true }).click();
    await page.getByRole('button', { name: '关闭组织…', exact: true }).click();
    await expect(confirmationName).toHaveValue('');
    await expect(confirmClose).toBeDisabled();
    await closeDialog.getByRole('button', { name: '关闭弹窗', exact: true }).click();
    await page.setViewportSize({ width: 390, height: 844 });
    await heading.getByRole('button', { name: '更多操作', exact: true }).click();
    await expect(page.getByRole('button', { name: '关闭组织…', exact: true })).toBeInViewport();
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await page.screenshot({
      path: 'artifacts/reported-usability/organization-content/owner-menu-mobile.png',
    });
    await heading.getByRole('button', { name: '更多操作', exact: true }).press('Escape');
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.getByRole('tab', { name: '内容', exact: true }).click();
    await expect(
      content.getByRole('navigation', { name: '知识库位置', exact: true }),
    ).toContainText(laterKB.name);
    await content.getByRole('button', { name: '刷新组织内容', exact: true }).click();
    await expect(content.getByText(libraryDoc.title, { exact: true })).toBeVisible();
    await expect(
      content.getByRole('navigation', { name: '知识库位置', exact: true }),
    ).toContainText(laterKB.name);
    await content
      .getByRole('navigation', { name: '工作空间导航' })
      .getByRole('button', { name: '知识库', exact: true })
      .click();
    await expect(
      content.getByRole('button', { name: '打开知识库：' + laterKB.name, exact: true }),
    ).toHaveCount(0);
    await content
      .getByRole('navigation', { name: '知识库分页', exact: true })
      .getByRole('button', { name: '下一页', exact: true })
      .click();
    await expect(
      content.getByRole('button', { name: '打开知识库：' + laterKB.name, exact: true }),
    ).toHaveCount(1);
    await content.getByRole('button', { name: '打开知识库：' + laterKB.name, exact: true }).click();
    await expect(content.getByText(libraryDoc.title, { exact: true })).toBeVisible();
    delayLibrary = true;
    const libraryReads = requests.filter(
      (item) => item.path === '/v1/knowledge-bases/' + laterKB.id + '/documents',
    ).length;
    const delayed = page.waitForRequest(
      (request) => new URL(request.url()).pathname === '/v1/knowledge-bases/' + laterKB.id,
    );
    await content.getByRole('button', { name: '刷新组织内容', exact: true }).click();
    await delayed;
    // 原生按钮通常会禁用；这里派发已排队的选择事件，专门验证旧组件请求晚到时不能继续安装/续读。
    await page.getByRole('button', { name: secondOrg.name, exact: true }).dispatchEvent('click');
    await expect(
      content.getByRole('button', { name: '打开知识库：' + otherKB.name, exact: true }),
    ).toBeVisible();
    await expect(content.getByText(libraryDoc.title, { exact: true })).toHaveCount(0);
    delayLibrary = false;
    releaseLibrary();
    await expect(page.locator('.busy-feedback')).toHaveCount(0);
    await page.getByRole('tab', { name: '组织成员', exact: true }).click();
    await expect(page.getByText('所有者：组织乙的所有者', { exact: true })).toBeVisible();
    await expect(page.getByText('组织编号：000' + secondOrg.id, { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: '更多操作', exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '生成组织邀请', exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '更新组织名称', exact: true })).toHaveCount(0);
    await page.getByRole('tab', { name: '内容', exact: true }).click();
    await content.getByRole('button', { name: '全部文档', exact: true }).click();
    assert.equal(
      requests.filter((item) => item.path === '/v1/knowledge-bases/' + laterKB.id + '/documents')
        .length,
      libraryReads,
      '卸载后的旧组织读取不能接着请求其文档目录',
    );
    await expect(content.getByText(otherDoc.title, { exact: true })).toBeVisible();
    denyOtherDocuments = true;
    await content
      .getByRole('navigation', { name: '文档分页', exact: true })
      .getByRole('button', { name: '下一页', exact: true })
      .click();
    await expect(content.getByRole('alert')).toHaveText('当前没有文档读取资格');
    await expect(content.getByText(otherDoc.title, { exact: true })).toHaveCount(0);
    await expect(
      content.getByRole('button', { name: '打开知识库：' + otherKB.name, exact: true }),
    ).toHaveCount(0);

    await page.getByRole('button', { name: firstOrg.name, exact: true }).click();
    await expect(
      content.getByRole('button', { name: '打开知识库：' + firstKB.name, exact: true }),
    ).toBeVisible();
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(membership).not.toBeVisible();
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: 'artifacts/reported-usability/organization-content/mobile.png',
      fullPage: true,
    });
    await content.getByRole('button', { name: '全部文档', exact: true }).click();
    await content.getByRole('link').filter({ hasText: firstDoc.title }).click();
    await expect(page).toHaveURL(new RegExp('/documents/' + firstDoc.id + '$'));

    // 邀请与加入页主动呈现办理入口；没有组织时也保留显眼入口，以上检查都不提交业务写入。
    emptyOrganizations = true;
    await page.goto('https://localhost:' + state().ports.edge + '/organizations');
    await expect(organizationSelector.getByText('还没有组织', { exact: false })).toBeVisible();
    await expect(membership).not.toBeVisible();
    const emptyEntry = page.getByRole('button', {
      name: '创建或加入组织',
      exact: true,
    });
    await expect(emptyEntry).toHaveClass(/primary/);
    await emptyEntry.click();
    await expect(membership.getByLabel('组织名称', { exact: true })).toBeVisible();
    await page.goto('https://localhost:' + state().ports.edge + '/join');
    await expect(membership).toBeVisible();
    await expect(membership.getByLabel('组织编号', { exact: true })).toBeVisible();
    await page.goto(
      'https://localhost:' +
        state().ports.edge +
        '/organizations?invitation_token=mock-organization-invitation',
    );
    await expect(membership).toBeVisible();
    await expect(membership.getByLabel('组织邀请 token', { exact: true })).toHaveValue(
      'mock-organization-invitation',
    );
    // 收起危险入口不能破坏原号恢复：刷新仍自动打开原操作进度，只查询，不重发关闭。
    await page.goto(
      'https://localhost:' +
        state().ports.edge +
        '/organizations?organization_close_operation=mock-organization-close',
    );
    const closeProgress = page.getByRole('dialog', { name: '组织关闭进度', exact: true });
    await expect(
      closeProgress.getByText('组织已关闭，个人账号继续有效', { exact: true }),
    ).toBeVisible();
    await closeProgress.getByRole('button', { name: '关闭弹窗', exact: true }).click();
    await page.getByRole('button', { name: '查看组织关闭进度', exact: true }).click();
    await expect(closeProgress).toBeVisible();
    await closeProgress.getByRole('button', { name: '结束查看此操作', exact: true }).click();
    assert.equal(new URL(page.url()).searchParams.has('organization_close_operation'), false);
    assert.equal(
      requests.some((item) => item.method !== 'GET'),
      false,
    );
    assert.equal(
      requests.some((item) => item.path === '/v1/document-applications'),
      false,
    );
    // 复制出来的八位编号可以直接申请；写接口仍绑定原始组织ID，不把补零带进路由。
    await page.getByRole('button', { name: '创建或加入组织', exact: true }).click();
    await membership.getByRole('button', { name: '申请加入', exact: true }).click();
    const organizationNumber = membership.getByLabel('组织编号', { exact: true }),
      submitApplication = membership.getByRole('button', { name: '提交加入申请', exact: true });
    await organizationNumber.fill('00000000');
    await expect(submitApplication).toBeDisabled();
    await organizationNumber.fill('000' + firstOrg.id);
    await membership.getByLabel('申请说明', { exact: true }).fill('八位编号申请验证');
    await submitApplication.click();
    await expect(
      page.getByRole('status').filter({ hasText: '申请已提交，等待管理员审批' }),
    ).toBeVisible();
    const writes = requests.filter((item) => item.method !== 'GET');
    assert.equal(writes.length, 1);
    assert.equal(writes[0].path, '/v1/organizations/' + firstOrg.id + '/applications');
    assert.equal(writes[0].input.reason, '八位编号申请验证');
    assert.deepEqual(errors, []);
    assert.deepEqual(healthRequests, [], '用户界面不应轮询内部服务健康状态');
  } finally {
    releaseLibrary?.();
    await context.close();
    await browser.close();
  }
});
