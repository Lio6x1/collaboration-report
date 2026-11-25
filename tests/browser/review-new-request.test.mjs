import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { reviewFixture, writeDraft, submitDraft, reviewDatabase } from '../helpers/review.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

// 用户完成一项批准后继续写下一项；新的普通NEW审核不能仍显示上一次已合并请求。
test('a new browser submission after a merged request selects its actual new fixed review', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let browser;
  try {
    await writeDraft(f, '第一次审核的固定正文');
    const first = (await submitDraft(f)).value.result;
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true }),
      writes = [];
    page.setDefaultTimeout(12000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(f.user.email);
    await page.getByLabel('登录密码', { exact: true }).fill(f.user.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page
      .getByRole('group', { name: '空间类型', exact: true })
      .getByRole('button', { name: '组织空间', exact: true })
      .click();
    await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
    await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
    await page.locator('a[href="/documents/' + f.d.id + '"]').click();
    await page.getByRole('tab', { name: '审核', exact: true }).click();
    const panel = page.getByRole('region', { name: '审核记录', exact: true });
    await panel.getByRole('button', { name: '通过并合并', exact: true }).waitFor();
    await panel.getByRole('button', { name: '通过并合并', exact: true }).click();
    await panel.getByLabel('我已核对整个固定送审版本，确认通过并合并', { exact: true }).check();
    await panel.getByRole('button', { name: '确认通过并合并', exact: true }).click();
    await panel.locator('.review-state').filter({ hasText: '已合并' }).waitFor();
    await page.getByRole('tab', { name: '正文', exact: true }).click();
    await page.getByRole('button', { name: '编辑文档', exact: true }).click();
    await page.getByRole('link', { name: '编辑共同支线', exact: true }).first().click();
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await expect(page.getByRole('region', { name: '共同支线编辑区', exact: true })).toHaveAttribute(
      'aria-busy',
      'false',
    );
    await expect(editor).toBeEditable();
    await editor.fill('第二次普通送审应自动打开新请求');
    await page.route('**/v1/documents/*/units/*/submissions', async (route) => {
      writes.push(route.request().postDataJSON());
      await route.continue();
    });
    await page.getByRole('button', { name: '送审当前草稿', exact: true }).click();
    await page.getByText('固定送审已完成，后续草稿可继续编辑', { exact: true }).waitFor();
    await page.getByRole('button', { name: '保存并查看审核记录', exact: true }).click();
    assert.equal(writes.length, 1);
    assert.equal(writes[0].mode, 'NEW');
    const [[created]] = await db.query(
      'SELECT CAST(request_id AS CHAR) request_id FROM review_round WHERE submission_operation_id=?',
      [writes[0].operation_id],
    );
    assert.ok(created);
    assert.notEqual(created.request_id, first.request_id);
    await panel.locator('.review-state').filter({ hasText: '审核中' }).waitFor();
    assert.equal(
      await panel
        .locator('[data-request-id="' + created.request_id + '"]')
        .getAttribute('aria-pressed'),
      'true',
    );
    await panel.getByText('展开送审正文', { exact: true }).click();
    await panel.getByRole('button', { name: '完整送审稿', exact: true }).click();
    assert.match(
      await panel.getByRole('document', { name: '固定送审正文', exact: true }).innerText(),
      /第二次普通送审/,
    );
  } finally {
    f.close();
    await browser?.close();
    await db.end();
  }
});
