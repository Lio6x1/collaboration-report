import test from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { resolve } from 'node:path';
import { chromium, expect } from '@playwright/test';
import { state, root } from '../../scripts/runtime-lib.mjs';
import { organizationFixture } from '../helpers/workspace.mjs';
import { pageVisibility, publishSearchDocument } from '../helpers/search-pages.mjs';
import { searchDatabase } from '../helpers/search.mjs';

// 共用三篇真实创建/保存/发布/原消费者夹具，验收整个页面流程。
// 权限后端、规模与自然到期已有独立证据，这里只验证新增页面的真实接线和交互。
async function fixture() {
  const f = await organizationFixture(0);
  const q = 'browsersearch' + randomBytes(10).toString('hex');
  const db = await searchDatabase();
  const docs = [];
  try {
    await pageVisibility(`/v1/knowledge-bases/${f.kb}`, f.owner, 'PUBLIC');
    for (const [index, policy] of ['PUBLIC', 'PUBLIC', 'SPACE_MEMBERS'].entries()) {
      docs.push(
        await publishSearchDocument(
          f,
          {
            q,
            title: `搜索正式标题${index + 1} ${q}`,
            body: `正式正文 ${q} 中文😀`,
            policy,
          },
          db,
        ),
      );
    }
    return { ...f, q, docs };
  } finally {
    await db.end();
  }
}

function isSearch(response) {
  return response.request().method() === 'GET' && new URL(response.url()).pathname === '/v1/search';
}

async function search(page, q, limit = '20') {
  await page.getByLabel('搜索词', { exact: true }).fill(q);
  await page.getByLabel('每页数量', { exact: true }).selectOption(limit);
  const received = page.waitForResponse(isSearch);
  await page.getByRole('button', { name: '搜索', exact: true }).click();
  const response = await received;
  assert.equal(response.status(), 200, 'BROWSER_SEARCH_HTTP_FAILED');
  const value = await response.json();
  await expect(
    page.getByRole('region', { name: '搜索结果', exact: true }).getByRole('link'),
  ).toHaveCount(value.items.length);
  return value;
}

test(
  '真实搜索页面：匿名完整分页、原锚点重试、正式章节与登录空间资格',
  { timeout: 180000 },
  async (t) => {
    const f = await fixture();
    const browser = await chromium.launch({ channel: 'msedge', headless: true });
    const origin = 'https://localhost:' + state().ports.edge;
    const errors = [];
    try {
      const page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1360, height: 960 },
      });
      page.on('pageerror', (error) => errors.push(error.message));
      page.setDefaultTimeout(8000);
      await page.goto(origin + '/search');
      const results = page.getByRole('region', { name: '搜索结果', exact: true });
      const first = await search(page, f.q, '1');
      assert.equal(first.items.length, 1);
      assert.equal(first.has_more, true);
      assert.ok(f.docs.slice(0, 2).some((doc) => doc.id === first.items[0].document_id));
      await expect(page.getByRole('link', { name: f.docs[2].title, exact: true })).toHaveCount(0);

      // 服务端已交付真实下一页后，浏览器传输丢弃回包；不伪造成功内容。
      // 重试必须使用原请求锚点，不能把第一页或旧结果当成成功的第二页。
      const attempts = [];
      let lost = false;
      await page.route('**/v1/search?*', async (route) => {
        if (!new URL(route.request().url()).searchParams.has('cursor')) return route.continue();
        attempts.push(route.request().url());
        if (lost) return route.continue();
        const actual = await route.fetch();
        assert.equal(actual.status(), 200, 'BROWSER_SEARCH_LOSS_REQUIRES_REAL_SUCCESS');
        lost = true;
        await route.abort('failed');
      });
      await page.getByRole('button', { name: '下一页', exact: true }).click();
      const retry = page.getByRole('button', { name: '重试本次查询', exact: true });
      await expect(retry).toBeVisible();
      await expect(results.getByRole('link')).toHaveCount(0);
      const repeated = page.waitForResponse(isSearch);
      await retry.click();
      const response = await repeated;
      assert.equal(response.status(), 200, 'BROWSER_SEARCH_RETRY_HTTP_FAILED');
      const second = await response.json();
      await expect(results.getByRole('link')).toHaveCount(1);
      assert.equal(attempts.length, 2);
      assert.ok(attempts[0] === attempts[1], 'BROWSER_SEARCH_RETRY_CHANGED_ORIGINAL_CURSOR');
      assert.equal(second.has_more, false);
      assert.equal(second.next_cursor, null);
      assert.deepEqual(
        [first.items[0].document_id, second.items[0].document_id].sort(),
        f.docs
          .slice(0, 2)
          .map((doc) => doc.id)
          .sort(),
      );
      await expect(page.getByRole('button', { name: '下一页', exact: true })).toHaveCount(0);
      await page.unroute('**/v1/search?*');
      t.diagnostic('真实匿名两页完整；丢失回包后原cursor重试，受限标题未展示。');

      const selected = f.docs.find((doc) => doc.id === second.items[0].document_id);
      const link = results.getByRole('link', { name: selected.title, exact: true });
      await expect(link).toHaveAttribute(
        'href',
        `/public/documents/${selected.id}#chapter-${selected.chapter}`,
      );
      await link.click();
      const content = page.getByRole('document', { name: '正式正文', exact: true });
      await expect(content).toContainText(f.q);
      await expect(page.locator(`[data-current-release="${selected.release}"]`)).toBeVisible();
      await expect(page.locator(`#chapter-${selected.chapter}`)).toBeInViewport();
      assert.equal(new URL(page.url()).searchParams.has('release_id'), false);
      await expect(page.getByRole('region', { name: '正式版本历史', exact: true })).toHaveCount(0);
      await page.screenshot({
        path: resolve(root, 'artifacts/batches/06/search-browser-chapter.png'),
        fullPage: true,
      });
      t.diagnostic('结果链接重新读取当前正式版，并定位真实章节。');

      // 沿真实登录表单和工作台空间入口接线，不注入Cookie或绕过session状态。
      await page.goto(origin + '/');
      await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await expect(page.getByRole('button', { name: '退出登录', exact: true })).toBeVisible();
      await page
        .getByRole('group', { name: '空间类型', exact: true })
        .getByRole('button', { name: '组织空间', exact: true })
        .click();
      await page.getByLabel('组织', { exact: true }).selectOption(f.space);
      await page.getByRole('button', { name: '搜索当前空间', exact: true }).click();
      await expect(page.getByLabel('搜索范围', { exact: true })).toHaveValue(f.space);
      const member = await search(page, f.q);
      assert.deepEqual(
        member.items.map((item) => item.document_id).sort(),
        f.docs.map((doc) => doc.id).sort(),
      );
      await expect(results.getByRole('link', { name: f.docs[2].title, exact: true })).toBeVisible();
      await page.screenshot({
        path: resolve(root, 'artifacts/batches/06/search-browser-results.png'),
        fullPage: true,
      });
      await page.getByRole('button', { name: '退出登录', exact: true }).click();
      await expect(page.getByRole('button', { name: '退出登录', exact: true })).toHaveCount(0);
      await expect(page.getByRole('link', { name: f.docs[2].title, exact: true })).toHaveCount(0);
      t.diagnostic('真实登录空间结果完整，注销后旧受限结果清除。');
      assert.deepEqual(errors, []);
    } finally {
      await browser.close();
    }
  },
);
