import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';

const pageOf = (items, next_cursor = null) => ({ items, next_cursor, has_more: !!next_cursor });
const member = (n) => ({
  user_id: String(n),
  display_name: '分页成员' + n,
  role: 'MEMBER',
  state: 'ACTIVE',
  revision: '1',
  knowledge_base_creation: 'NONE',
});

// 浏览器只替换外部HTTP边界；实际组件负责替换列表、记住游标、保持本人管理资格及手机布局。
test(
  'notifications and organization members replace pages and retain management outside the visible page',
  { timeout: 60000 },
  async () => {
    const browser = await chromium.launch({ channel: 'msedge', headless: true });
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1440, height: 1000 },
    });
    const page = await context.newPage();
    page.setDefaultTimeout(7000);
    const errors = [],
      requests = [];
    const organization = {
      id: '99101',
      name: '长列表分页验证',
      owner_id: '99999',
      lifecycle: 'ACTIVE',
      revision: '1',
      member_revision: '1',
      current_member: { user_id: '99001', role: 'ADMIN', state: 'ACTIVE', revision: '1' },
    };
    try {
      await context.addInitScript(() => {
        const slot = '1234567890abcdef1234567890abcdef';
        sessionStorage.setItem('collaboration-report.session-context', slot);
        sessionStorage.setItem(
          'collaboration-report.csrf',
          JSON.stringify({ user_id: '99001', token: 'pagination-ui', context: slot }),
        );
      });
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/v1/**', async (route) => {
        const url = new URL(route.request().url()),
          path = url.pathname;
        requests.push(url);
        const json = (value) => route.fulfill({ json: value });
        if (path === '/v1/account/me')
          return json({
            id: '99001',
            email: 'pagination@example.test',
            display_name: '分页管理员',
            state: 'ACTIVE',
            credential_epoch: '1',
            identity_epoch: '1',
          });
        if (path === '/v1/notifications') {
          const offset = url.searchParams.get('cursor') ? 10 : 0;
          return json(
            pageOf(
              Array.from({ length: offset ? 1 : 10 }, (_, i) => ({
                id: String(1000 + offset + i),
                kind: 'REVIEW_MERGED',
                title: '分页通知' + (offset + i + 1),
                target_id: '1',
                read_at: '2026-10-13T00:00:00Z',
              })),
              offset ? null : 'notice-next',
            ),
          );
        }
        if (path === '/v1/organizations') return json(pageOf([organization]));
        if (path === '/v1/organizations/' + organization.id) return json(organization);
        if (path.endsWith('/members')) {
          const query = url.searchParams.get('q');
          if (query) return json(pageOf([member(21)]));
          return json(
            url.searchParams.get('cursor')
              ? pageOf([member(21)])
              : pageOf(
                  Array.from({ length: 20 }, (_, i) => member(i + 1)),
                  'member-next',
                ),
          );
        }
        return json(pageOf([]));
      });
      await page.goto('https://localhost:' + state().ports.edge);
      await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
      await page.getByRole('button', { name: '通知', exact: true }).click();
      const notifications = page.getByRole('region', { name: '通知', exact: true });
      await expect(notifications.getByRole('listitem')).toHaveCount(10);
      const notificationPager = page.getByRole('navigation', { name: '通知分页', exact: true });
      await notificationPager.getByRole('button', { name: '下一页', exact: true }).click();
      await expect(notifications.getByRole('listitem')).toHaveCount(1);
      await expect(notifications.getByText('分页通知11', { exact: true })).toBeVisible();
      await notificationPager.getByRole('button', { name: '上一页', exact: true }).click();
      await expect(notifications.getByRole('listitem')).toHaveCount(10);
      assert.equal(
        requests.filter(
          (url) => url.pathname === '/v1/notifications' && !url.searchParams.has('cursor'),
        ).length,
        2,
        '上一页重读服务端，不恢复旧授权结果',
      );
      await page.getByRole('button', { name: '组织', exact: true }).click();
      await page.getByRole('button', { name: /长列表分页验证/ }).click();
      await page.getByRole('tab', { name: '成员与管理', exact: true }).click();
      await expect(page.getByRole('table').getByRole('row')).toHaveCount(21);
      await expect(page.getByRole('button', { name: '生成组织邀请', exact: true })).toBeVisible();
      const memberPager = page.getByRole('navigation', { name: '组织成员分页', exact: true });
      await memberPager.getByRole('button', { name: '下一页', exact: true }).click();
      await expect(page.getByRole('table').getByRole('row')).toHaveCount(2);
      await expect(page.getByRole('button', { name: '生成组织邀请', exact: true })).toBeVisible();
      await page.getByLabel('搜索组织成员', { exact: true }).fill('分页成员21');
      await page.getByRole('button', { name: '搜索成员', exact: true }).click();
      await expect(page.getByRole('table').getByRole('row')).toHaveCount(2);
      await expect(memberPager).toHaveCount(0);
      assert.ok(
        requests.some(
          (url) =>
            url.pathname.endsWith('/members') &&
            url.searchParams.get('q') === '分页成员21' &&
            !url.searchParams.has('cursor'),
        ),
      );
      await page.setViewportSize({ width: 390, height: 844 });
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        true,
      );
      assert.deepEqual(errors, []);
    } finally {
      await context.close();
      await browser.close();
    }
  },
);
