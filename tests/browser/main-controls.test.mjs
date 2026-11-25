import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import { organizationFixture, documentFixture, get, json } from '../helpers/workspace.mjs';
import { state, root } from '../../scripts/runtime-lib.mjs';

// 页面若把MAIN入口仍绑到BRANCH，本例真实锁定后的主线正文会保持旧值，不能凭按钮文案通过。
test('real browser unlocks MAIN, saves its own workspace and waits for locking installation', async () => {
  const f = await organizationFixture(0);
  const d = await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const page = await browser.newPage({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 960 },
    });
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    page.setDefaultTimeout(8000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(f.owner.email);
    await page.getByLabel('登录密码', { exact: true }).fill(f.owner.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page
      .getByRole('group', { name: '空间类型', exact: true })
      .getByRole('button', { name: '组织空间', exact: true })
      .click();
    await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
    await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
    await page.getByRole('button', { name: '协作报告', exact: true }).click();
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).waitFor();
    const unlock = page.getByRole('button', { name: '解锁主线', exact: true });
    assert.equal(await unlock.count(), 1, '管理页面缺少已授权的主线控制入口');
    await unlock.click();
    await page.getByRole('button', { name: '锁定主线', exact: true }).waitFor();
    await page.getByRole('button', { name: '编辑主线', exact: true }).first().click();
    const editor = page.getByRole('textbox', { name: '主线编辑器', exact: true });
    await editor.waitFor();
    await editor.fill('页面实际保存的主线正文');
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    await page.getByRole('button', { name: '保存当前修改', exact: true }).click();
    await page.getByText('当前修改已保存', { exact: true }).waitFor();
    assert.equal(await page.locator('.unit-editor .error').count(), 0);
    await editor.focus();
    const cdp = await page.context().newCDPSession(page);
    await cdp.send('Input.imeSetComposition', {
      text: '锁定前中文候选',
      selectionStart: 7,
      selectionEnd: 7,
    });
    await cdp.send('Input.insertText', { text: '锁定按钮前提交的中文输入' });
    // 不先等待SAVED或手动FLUSH：锁定入口必须完成相同IME/本机队列边界，再登记原控制动作。
    await page.getByRole('button', { name: '锁定主线', exact: true }).click();
    await page.getByRole('button', { name: '解锁主线', exact: true }).waitFor();
    assert.equal(await editor.count(), 0, '旧MAIN编辑器应随关闭代次退出');
    const main = json(await get(`/v1/documents/${d.id}/main`, f.owner.headers));
    assert.equal(main.document.main_locked, true);
    const fixed = json(
      await get(
        `/v1/documents/${d.id}/read-views/${main.read_view_id}/units/${main.units[0].unit_id}`,
        f.owner.headers,
      ),
    );
    assert.ok(JSON.stringify(fixed.body_json).includes('页面实际保存的主线正文'));
    assert.ok(JSON.stringify(fixed.body_json).includes('锁定按钮前提交的中文输入'));
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/main-controls-browser.png'),
      fullPage: true,
    });
  } finally {
    await browser.close();
  }
});
