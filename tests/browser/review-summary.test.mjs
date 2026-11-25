import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { chromium, expect } from '@playwright/test';
import {
  documentFixture,
  organizationFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import {
  openReviewWorkspace,
  writeDraft,
  writeDraftBody,
  submitDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

test(
  '真实审核列表用固定摘要区分请求，长文默认收起并在切换后保持折叠',
  { timeout: 120000 },
  async () => {
    // 复用本机体验身份；只在新建组织中送审，不注册账号或发送验证码邮件。
    const { email, password } = JSON.parse(
      readFileSync(process.env.COLLABORATION_TEST_ACCOUNT, 'utf8'),
    );
    const login = await post('/v1/account/login', { email, password });
    assert.equal(login.status, 200);
    const owner = {
      email,
      password,
      id: json(login).user_id,
      headers: {
        cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
        'x-csrf-token': json(login).csrf_token,
      },
    };
    const f = await organizationFixture(0, owner);
    const doc = await documentFixture(f);
    const workspace = await openReviewWorkspace(f, doc);
    const db = await reviewDatabase();
    let browser;
    try {
      await unlockStructurePreview(workspace);
      const oldText = '上一版：登录流程与邮箱验证说明。';
      const unchanged = {
        type: 'paragraph',
        attrs: { id: 'diff-unchanged' },
        content: [{ type: 'text', text: '这一段在两个固定版本中保持原样。' }],
      };
      const formatText = '同样的文字，本轮调整了格式。';
      await writeDraftBody(workspace, {
        type: 'doc',
        content: [
          {
            type: 'paragraph',
            attrs: { id: 'review-summary-0' },
            content: [{ type: 'text', text: oldText }],
          },
          unchanged,
          {
            type: 'paragraph',
            attrs: { id: 'diff-format' },
            content: [{ type: 'text', text: formatText }],
          },
          {
            type: 'paragraph',
            attrs: { id: 'diff-removed' },
            content: [{ type: 'text', text: '需移除的旧说明' }],
          },
        ],
      });
      const first = (await submitDraft(workspace)).value;
      assert.equal(first.state, 'SUCCEEDED');

      const currentText = '本次送审：审核列表展示章节标题与固定正文摘要。';
      const ending = '审核正文末尾标记';
      const paragraphs = [
        currentText,
        ...Array.from(
          { length: 70 },
          (_, index) =>
            '第' +
            (index + 1) +
            '段：这是一份较长的送审正文，需要按需展开阅读，后续共同草稿不会替换这份固定内容。',
        ),
        ending,
      ];
      await writeDraftBody(workspace, {
        type: 'doc',
        content: [
          ...paragraphs.map((text, index) => ({
            type: 'paragraph',
            attrs: { id: 'review-summary-' + index },
            content: [{ type: 'text', text }],
          })),
          unchanged,
          {
            type: 'paragraph',
            attrs: { id: 'diff-format', align: 'center' },
            content: [{ type: 'text', text: formatText, marks: [{ type: 'strong' }] }],
          },
        ],
      });
      const mergeID = operation('review-summary-first-merge');
      const merged = await post(
        `/v1/review-requests/${first.result.request_id}/approve-and-merge`,
        {
          operation_id: mergeID,
          round_id: first.result.round_id,
          content_version_id: first.result.content_version_id,
          own_changes_to_resolve: [],
        },
        owner.headers,
      );
      assert.equal(merged.status, 202);
      assert.equal((await pollMerge(mergeID, owner.headers, db)).state, 'SUCCEEDED');
      const second = (await submitDraft(workspace)).value;
      assert.equal(second.state, 'SUCCEEDED');
      assert.notEqual(first.result.request_id, second.result.request_id);
      await writeDraft(workspace, '送审后的草稿不得出现在摘要里');

      const listing = await get(`/v1/documents/${doc.id}/review-requests?limit=100`, owner.headers);
      assert.equal(listing.status, 200);
      const records = json(listing).items;
      const previous = records.find((record) => record.id === first.result.request_id);
      const current = records.find((record) => record.id === second.result.request_id);
      assert.ok(previous.summary.excerpt.startsWith(oldText));
      assert.ok(current.summary.excerpt.startsWith(currentText));
      assert.ok([...current.summary.excerpt].length <= 141);
      assert.ok(!current.summary.excerpt.includes('送审后的草稿'));
      assert.equal(current.current_round, undefined, '列表只返回轻量摘要，不重复传递完整固定正文');
      const title = doc.main.structure.entries.find(
        (entry) => entry.unit_id === doc.main.units[0].unit_id,
      ).title;
      assert.equal(current.summary.title, title);

      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1440, height: 1000 },
      });
      page.setDefaultTimeout(10000);
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.goto(
        'https://localhost:' + state().ports.edge + '/documents/' + doc.id + '?tab=review',
      );
      await page.getByLabel('登录邮箱', { exact: true }).fill(email);
      await page.getByLabel('登录密码', { exact: true }).fill(password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      const panel = page.getByRole('region', { name: '审核记录', exact: true });
      const firstCard = panel.locator('[data-request-id="' + first.result.request_id + '"]');
      const secondCard = panel.locator('[data-request-id="' + second.result.request_id + '"]');
      await expect(firstCard).toContainText(oldText);
      await expect(secondCard).toContainText(currentText);
      await expect(secondCard).toContainText(title);
      await expect(secondCard).toHaveAttribute('aria-pressed', 'true');
      await expect(page.getByLabel('原冲突操作编号', { exact: true })).not.toBeVisible();
      await page.locator('summary').filter({ hasText: '人工修复工作台' }).click();
      await expect(page.getByLabel('原冲突操作编号', { exact: true })).toBeVisible();
      await page.locator('summary').filter({ hasText: '人工修复工作台' }).click();
      const body = panel.getByRole('document', { name: '固定送审正文', exact: true });
      await expect(body).toHaveCount(0);
      await expect(panel.getByRole('region', { name: '普通批注与讨论', exact: true })).toHaveCount(
        0,
      );
      await panel.getByText('展开送审正文', { exact: true }).click();
      const difference = panel.getByRole('region', { name: '审核正文差异', exact: true });
      await expect(difference).toContainText(ending);
      await expect(difference).toContainText('需移除的旧说明');
      await expect(difference.locator('[data-diff-kind="add"]').first()).toBeVisible();
      await expect(difference.locator('[data-diff-kind="delete"]').first()).toBeVisible();
      await expect(difference.locator('[data-diff-kind="format"]').first()).toBeVisible();
      await expect(difference.locator('summary').filter({ hasText: '未改动内容' })).toBeVisible();
      await expect(body).toHaveCount(0);
      mkdirSync(resolve(root, 'artifacts/reported-usability'), { recursive: true });
      await panel.getByRole('region', { name: '送审正文阅读区', exact: true }).screenshot({
        path: resolve(root, 'artifacts/reported-usability/review-diff-desktop.png'),
      });
      await panel.getByRole('button', { name: '完整送审稿', exact: true }).click();
      await expect(body).toContainText(ending);
      const viewport = panel.getByRole('region', { name: '送审正文阅读区', exact: true });
      const dimensions = await viewport.evaluate((node) => ({
        height: node.clientHeight,
        scroll: node.scrollHeight,
      }));
      assert.ok(dimensions.scroll > dimensions.height && dimensions.height <= 650);
      await panel.getByText('收起送审正文', { exact: true }).click();
      await expect(body).toHaveCount(0);
      await firstCard.click();
      await expect(firstCard).toHaveAttribute('aria-pressed', 'true');
      await expect(body).toHaveCount(0);
      await panel.locator('summary').filter({ hasText: '展开送审正文' }).focus();
      await page.keyboard.press('Enter');
      await expect(difference).toBeVisible();
      await panel.getByRole('button', { name: '完整送审稿', exact: true }).click();
      await expect(body).toContainText(oldText);
      await secondCard.click();
      await expect(secondCard).toHaveAttribute('aria-pressed', 'true');
      await expect(body).toHaveCount(0);

      // 折叠讨论只影响展示，未提交的评论输入仍保留在同一固定轮次中。
      await panel.getByText('本轮讨论', { exact: true }).click();
      const comment = panel.getByLabel('对这份正文发起讨论', { exact: true });
      await comment.fill('尚未提交的本轮讨论');
      await panel.getByText('本轮讨论', { exact: true }).click();
      await panel.getByText('本轮讨论', { exact: true }).click();
      await expect(comment).toHaveValue('尚未提交的本轮讨论');
      await comment.fill('');
      await panel.getByText('本轮讨论', { exact: true }).click();

      const artifacts = resolve(root, 'artifacts/reported-usability');
      mkdirSync(artifacts, { recursive: true });
      await page.mouse.click(3, 3);
      await page.evaluate(() => scrollTo(0, 0));
      await page.screenshot({
        path: resolve(artifacts, 'review-summary-desktop.png'),
        fullPage: true,
      });
      await page.setViewportSize({ width: 320, height: 900 });
      await page.evaluate(() => scrollTo(0, 0));
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        false,
      );
      await page.screenshot({
        path: resolve(artifacts, 'review-summary-mobile.png'),
        fullPage: true,
      });
      await panel.getByText('展开送审正文', { exact: true }).click();
      await expect(difference).toContainText(ending);
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        false,
      );
      await viewport.screenshot({ path: resolve(artifacts, 'review-diff-mobile.png') });
      await panel.getByRole('button', { name: '完整送审稿', exact: true }).click();
      await expect(body).toContainText(ending);
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        false,
      );
      await viewport.screenshot({ path: resolve(artifacts, 'review-body-mobile.png') });

      await panel.getByLabel('审核理由', { exact: true }).fill('请补充参考来源');
      await panel.getByRole('button', { name: '要求修改', exact: true }).click();
      await expect(panel.getByText('请补充参考来源', { exact: true })).toBeVisible();
      await expect(secondCard).toContainText('1 项待处理要求');
      await expect(body).toHaveCount(0);
      assert.deepEqual(errors, []);
    } finally {
      workspace.close();
      await browser?.close();
      await db.end();
    }
  },
);
