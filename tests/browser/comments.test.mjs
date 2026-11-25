import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

test('真实浏览器重开主线后保留历史讨论原始锚点并可继续回复', { timeout: 60000 }, async () => {
  const f = await organizationFixture(0);
  const document = await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 900 },
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
    await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    try {
      await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    } catch (error) {
      throw Error(await page.locator('body').innerText(), { cause: error });
    }
    async function openDocument() {
      await page
        .getByRole('group', { name: '空间类型', exact: true })
        .getByRole('button', { name: '组织空间', exact: true })
        .click();
      await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
      await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
      await page.getByRole('button', { name: '协作报告', exact: true }).click();
      await page.getByRole('region', { name: '普通批注与讨论', exact: true }).waitFor();
    }
    await openDocument();
    const panel = page.getByRole('region', { name: '普通批注与讨论', exact: true });
    const detail = panel.getByRole('region', { name: '当前讨论', exact: true });
    const originalBody = '主线历史讨论：刷新后仍保留原始正文锚点 🧪';
    await panel.getByLabel('对这份正文发起讨论', { exact: true }).fill(originalBody);
    const createdResponse = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' &&
        new URL(response.url()).pathname === `/v1/documents/${document.id}/comment-threads`,
    );
    await panel.getByRole('button', { name: '发起讨论', exact: true }).click();
    const created = await createdResponse;
    assert.equal(created.status(), 201, await created.text());
    const input = created.request().postDataJSON();
    assert.equal(input.context_kind, 'MAIN');
    assert.match(input.context_id, /^[1-9][0-9]*$/);
    assert.match(input.anchor.content_version_id, /^[1-9][0-9]*$/);
    const thread = (await created.json()).result.entity_id;
    const threadButton = panel.getByRole('button', {
      name: new RegExp(`^讨论 ${thread} ·`),
    });
    await threadButton.click();
    await detail.getByText(originalBody, { exact: true }).waitFor();
    const anchorText = `原始正文版本 ${input.anchor.content_version_id} · 原始主线清单 ${input.context_id}`;
    await detail.getByText(anchorText, { exact: true }).waitFor();

    // 整页重载会取得新的阅读视图，历史讨论仍应从文档级列表找到，不能只过滤新清单。
    await page.reload();
    await openDocument();
    await threadButton.click();
    await detail.getByText(originalBody, { exact: true }).waitFor();
    await detail.getByText(anchorText, { exact: true }).waitFor();
    await detail.getByRole('button', { name: '回复这条批注', exact: true }).click();
    const replyBody = '重新打开后回复：历史清单与正文版本保持不变 🙂';
    await detail.getByLabel('发表批注', { exact: true }).fill(replyBody);
    await detail.getByRole('button', { name: '发表批注', exact: true }).click();
    await detail.getByText(replyBody, { exact: true }).waitFor();
    await detail.getByText(anchorText, { exact: true }).waitFor();

    // 再次重载确认回复已经服务端持久保存，不把乐观渲染当成成功。
    await page.reload();
    await openDocument();
    await threadButton.click();
    await detail.getByText(originalBody, { exact: true }).waitFor();
    await detail.getByText(replyBody, { exact: true }).waitFor();
    await detail.getByText(anchorText, { exact: true }).waitFor();
    assert.deepEqual(errors, []);
    await context.close();
  } finally {
    await browser.close();
  }
});
