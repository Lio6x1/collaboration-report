import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { resolve } from 'node:path';
import { reviewFixture, writeDraft, submitDraft } from '../helpers/review.mjs';
import { get, json } from '../helpers/workspace.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

test(
  '共同支线真实选区绑定固定送审正文，草稿改写及刷新不移动原锚点',
  { timeout: 90000 },
  async () => {
    const f = await reviewFixture();
    let browser;
    try {
      const text = '前缀中文🧪选区讨论尾部';
      assert.equal((await writeDraft(f, text)).type, 'SAVED');
      const submission = await submitDraft(f);
      assert.equal(submission.value.state, 'SUCCEEDED');
      const round = submission.value.result.round_id;
      const version = submission.value.result.content_version_id;
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const context = await browser.newContext({
        ignoreHTTPSErrors: true,
        viewport: { width: 1360, height: 960 },
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.goto('https://localhost:' + state().ports.edge);
      await page.getByLabel('登录邮箱', { exact: true }).fill(f.user.email);
      await page.getByLabel('登录密码', { exact: true }).fill(f.user.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
      async function openDiscussion() {
        await page
          .getByRole('group', { name: '空间类型', exact: true })
          .getByRole('button', { name: '组织空间', exact: true })
          .click();
        await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
        await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
        await page.getByRole('button', { name: '协作报告', exact: true }).click();
        await page.getByRole('button', { name: '编辑共同支线', exact: true }).first().click();
        await page.getByRole('button', { name: '共同草稿讨论', exact: true }).click();
      }
      await openDiscussion();
      const region = page.getByRole('region', { name: '共同草稿固定版本讨论', exact: true });
      await expect(region.getByLabel('讨论依据')).toContainText(round);
      await region.getByLabel('讨论依据').selectOption(round);
      const body = region.getByRole('document', { name: '共同草稿讨论固定正文', exact: true });
      await expect(body).toHaveText(text);
      // 操作浏览器原生DOM选区，交给PM selectionchange读取；不注入组件或编辑器状态。
      await body.locator('p').evaluate((node) => {
        const range = document.createRange();
        range.setStart(node.firstChild, 2);
        range.setEnd(node.firstChild, 8);
        node.closest('[role="document"]').focus();
        const selection = window.getSelection();
        selection.removeAllRanges();
        selection.addRange(range);
        document.dispatchEvent(new Event('selectionchange'));
      });
      const panel = region.getByRole('region', { name: '普通批注与讨论', exact: true });
      await expect(panel.getByLabel('新讨论选区锚点')).toContainText('中文🧪选区');
      await panel
        .getByLabel('对这份正文发起讨论', { exact: true })
        .fill('这一段中文和表情需要讨论');
      // 同页编辑与确认保存会更新工作区状态，但固定讨论版本和未提交输入必须保留。
      const editor = page.getByLabel('共同支线编辑器', { exact: true });
      await editor.click();
      await page.keyboard.press('End');
      await page.keyboard.insertText('本机继续编辑');
      await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
      await page.getByText('当前修改已保存', { exact: true }).waitFor();
      await expect(region.getByLabel('讨论依据')).toHaveValue(round);
      await expect(panel.getByLabel('对这份正文发起讨论', { exact: true })).toHaveValue(
        '这一段中文和表情需要讨论',
      );
      await expect(panel.getByLabel('新讨论选区锚点')).toContainText('中文🧪选区');
      const pending = page.waitForResponse(
        (response) =>
          response.request().method() === 'POST' &&
          new URL(response.url()).pathname === `/v1/documents/${f.d.id}/comment-threads`,
      );
      await panel.getByRole('button', { name: '发起讨论', exact: true }).click();
      const response = await pending;
      assert.equal(response.status(), 201, await response.text());
      const input = response.request().postDataJSON();
      assert.equal(input.context_kind, 'BRANCH');
      assert.equal(input.context_id, f.session.branch_id);
      assert.deepEqual(input.anchor, {
        schema_version: 'anchor-v1',
        content_version_id: version,
        block_id: 'review-paragraph',
        start_offset: 2,
        end_offset: 8,
        quote: '中文🧪选区',
      });
      const thread = (await response.json()).result.entity_id;
      await panel.getByRole('button', { name: new RegExp(`^讨论 ${thread} ·`) }).click();
      await expect(body.locator('.comment-anchor-highlight')).toHaveText('中文🧪选区');
      assert.equal((await writeDraft(f, '草稿已改写，旧讨论仍然指向原固定送审正文')).type, 'SAVED');
      await page.reload();
      await openDiscussion();
      // 默认基线与送审版本不同，历史线程仍可找到，并独立打开原正文而不是当前草稿。
      await panel.getByRole('button', { name: new RegExp(`^讨论 ${thread} ·`) }).click();
      await panel.getByRole('button', { name: '查看原文位置', exact: true }).click();
      const original = panel.getByRole('document', { name: '讨论原始锚点正文', exact: true });
      await expect(original).toHaveText(text);
      await expect(original.locator('.comment-anchor-highlight')).toHaveText('中文🧪选区');
      const persisted = await get(`/v1/comment-threads/${thread}`, f.user.headers);
      assert.equal(persisted.status, 200, persisted.body);
      assert.deepEqual(json(persisted).anchor, input.anchor);
      assert.deepEqual(errors, []);
      await region.screenshot({
        path: resolve(root, 'artifacts/comment-anchor-browser.png'),
      });
    } finally {
      await browser?.close();
      f.close();
    }
  },
);
