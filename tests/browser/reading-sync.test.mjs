import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { organizationFixture, documentFixture, get, post, json } from '../helpers/workspace.mjs';
import { openReviewWorkspace, writeDraftBody } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

async function owner() {
  const { email, password } = JSON.parse(
    readFileSync(process.env.COLLABORATION_TEST_ACCOUNT, 'utf8'),
  );
  const login = await post('/v1/account/login', { email, password });
  assert.equal(login.status, 200);
  return {
    email,
    password,
    id: json(login).user_id,
    headers: {
      cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
      'x-csrf-token': json(login).csrf_token,
    },
  };
}
const body = (ending) => ({
  type: 'doc',
  content: Array.from({ length: 30 }, (_, index) => ({
    type: 'paragraph',
    attrs: { id: 'reading-sync-' + index },
    content: [
      {
        type: 'text',
        text: index === 29 ? ending : '用于检查阅读位置的正文第' + (index + 1) + '段。',
      },
    ],
  })),
});

// 对应用户实际故障：截图焦点往返不能重建页面；另一个真实WSS客户端SAVED后，
// 可见阅读页应自己更新。评论引用固定版本，有草稿时必须延迟安装新正文。
test(
  'reading checks durable changes without focus reloads or lost comment drafts',
  { timeout: 90000 },
  async () => {
    const f = await organizationFixture(0, await owner());
    const d = await documentFixture(f);
    const revisionPath = '/v1/documents/' + d.id + '/main-revision';
    assert.ok([401, 403].includes((await get(revisionPath)).status), '变化摘要也必须核验身份');
    await unlockStructurePreview({ d, user: f.owner });
    const workspace = await openReviewWorkspace(f, d, f.owner, 'MAIN');
    const browser = await chromium.launch({ channel: 'msedge', headless: true });
    let page;
    const responses = [];
    try {
      await writeDraftBody(workspace, body('最初已保存的正文'));
      const revisionResponse = await get(revisionPath, f.owner.headers);
      assert.equal(revisionResponse.status, 200, json(revisionResponse).code);
      const before = json(revisionResponse).revision;
      assert.match(before, /^[a-f0-9]{64}$/);
      assert.equal(
        json(await get(revisionPath, f.owner.headers)).revision,
        before,
        '未变化时版本摘要稳定',
      );
      page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 900 },
      });
      page.setDefaultTimeout(12000);
      let captures = 0,
        checks = 0;
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('response', async (response) => {
        const path = new URL(response.url()).pathname;
        if (path === revisionPath || path === '/v1/documents/' + d.id + '/main') {
          const value = await response.json().catch(() => ({}));
          responses.push({
            path: path.endsWith('/main') ? 'main' : 'revision',
            status: response.status(),
            code: value.code,
          });
        }
      });
      page.on('request', (request) => {
        const path = new URL(request.url()).pathname;
        if (path === '/v1/documents/' + d.id + '/main') captures++;
        if (path === revisionPath) checks++;
      });
      await page.goto('https://localhost:' + state().ports.edge + '/documents/' + d.id);
      await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      const reading = page.getByLabel('主线正文', { exact: true }).first();
      await expect(reading).toContainText('最初已保存的正文');
      await expect(page.locator('.busy-feedback')).toHaveCount(0);
      await page.evaluate(() => document.fonts.ready.then(() => undefined));
      await page.evaluate(() => {
        window.__readingElement = document.querySelector('[aria-label="主线正文"]');
        window.scrollTo(0, 550);
        window.dispatchEvent(new Event('blur'));
        window.dispatchEvent(new Event('focus'));
        window.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', altKey: true }));
        window.dispatchEvent(new Event('focus'));
      });
      const count = captures,
        initialChecks = checks;
      await expect.poll(() => checks, { timeout: 12000 }).toBeGreaterThan(initialChecks);
      assert.equal(captures, count, '焦点往返与无变化的定时检查都不能重新捕获全文');
      assert.equal(
        await page.evaluate(
          () => window.__readingElement === document.querySelector('[aria-label="主线正文"]'),
        ),
        true,
      );
      assert.equal(await page.evaluate(() => scrollY), 550);

      // 不切换浏览器焦点，通过另一个真实协作连接提交新内容，验证定时发现实际SAVED。
      await writeDraftBody(workspace, body('另一位编辑者刚刚保存的新内容'));
      assert.notEqual(json(await get(revisionPath, f.owner.headers)).revision, before);
      // 一个检查刚好已发出时，新SAVED会由下一个5秒周期发现，另留正文获取时间。
      await expect(reading).toContainText('另一位编辑者刚刚保存的新内容', { timeout: 12000 });
      assert.equal(await page.evaluate(() => scrollY), 550, '背景更新保留阅读位置');

      await page.getByRole('tab', { name: '评论', exact: true }).click();
      const draft = page.getByLabel('对这份正文发起讨论', { exact: true });
      await draft.fill('还没有提交的评论草稿');
      await writeDraftBody(workspace, body('评论输入期间保存的第二次更新'));
      await expect(page.getByText('正文有更新。', { exact: false })).toBeVisible({
        timeout: 12000,
      });
      await expect(draft).toHaveValue('还没有提交的评论草稿');
      await page.getByRole('tab', { name: '正文', exact: true }).click();
      await expect(reading).not.toContainText('评论输入期间保存的第二次更新');
      await page.getByRole('button', { name: '刷新阅读', exact: true }).click();
      await expect(page.getByText('评论草稿或文字定位已保留', { exact: false })).toBeVisible();
      await page.getByRole('tab', { name: '评论', exact: true }).click();
      await expect(draft).toHaveValue('还没有提交的评论草稿');
      await draft.fill('');
      await page.getByRole('tab', { name: '正文', exact: true }).click();
      await expect(reading).toContainText('评论输入期间保存的第二次更新', { timeout: 12000 });

      // 故障提示与正文更新分开：临时503保留已读正文，恢复后自动重试；确认撤权才清除。
      let unavailable = true;
      const revisionURL = '**' + revisionPath;
      await page.route(revisionURL, (route) =>
        unavailable
          ? route.fulfill({
              status: 503,
              json: { code: 'SERVICE_UNAVAILABLE' },
            })
          : route.continue(),
      );
      await page.evaluate(() => document.dispatchEvent(new Event('visibilitychange')));
      await expect(page.getByText('暂时无法检查新内容', { exact: false })).toBeVisible({
        timeout: 12000,
      });
      await expect(reading).toContainText('评论输入期间保存的第二次更新');
      unavailable = false;
      await page.evaluate(() => document.dispatchEvent(new Event('visibilitychange')));
      await expect(page.getByText('暂时无法检查新内容', { exact: false })).toHaveCount(0, {
        timeout: 12000,
      });
      await page.unroute(revisionURL);
      await page.route(revisionURL, (route) =>
        route.fulfill({ status: 403, json: { code: 'DOCUMENT_READ_NOT_ALLOWED' } }),
      );
      await page.evaluate(() => document.dispatchEvent(new Event('visibilitychange')));
      await expect(reading).toHaveCount(0, { timeout: 12000 });
      await expect(page.getByText('当前没有文档读取资格', { exact: true })).toBeVisible();
      assert.deepEqual(errors, []);
    } catch (failure) {
      console.error(
        'READING_SYNC_TRACE',
        JSON.stringify({
          responses,
          state: page
            ? await page
                .evaluate(() => ({
                  hidden: document.hidden,
                  selection: {
                    collapsed: getSelection()?.isCollapsed,
                    count: getSelection()?.rangeCount,
                  },
                  sync: document.querySelector('.reading-sync')?.textContent,
                  scroll: scrollY,
                }))
                .catch(() => null)
            : null,
        }),
      );
      throw failure;
    } finally {
      workspace.close();
      await browser.close();
    }
  },
);
