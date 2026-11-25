import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdirSync } from 'node:fs';
import { chromium, expect } from '@playwright/test';
import { organizationFixture, documentFixture, post, json } from '../helpers/workspace.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

// 同一真实主线流程覆盖行内样式、纯文本粘贴、语言高亮、复制和刷新持久化，避免只测工具函数。
test(
  'inline code and highlighted code blocks survive real main save and read-only viewing',
  { timeout: 60000 },
  async () => {
    const owner = JSON.parse(readFileSync(process.env.COLLABORATION_TEST_ACCOUNT, 'utf8'));
    const login = await post('/v1/account/login', { email: owner.email, password: owner.password });
    assert.equal(login.status, 200);
    owner.id = json(login).user_id;
    owner.headers = {
      cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
      'x-csrf-token': json(login).csrf_token,
    };
    const f = await organizationFixture(0, owner);
    const doc = await documentFixture(f);
    await unlockStructurePreview({ d: doc, user: owner });
    const browser = await chromium.launch({ channel: 'msedge', headless: true });
    const origin = 'https://localhost:' + state().ports.edge;
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1440, height: 1100 },
      permissions: ['clipboard-read', 'clipboard-write'],
    });
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    try {
      await page.goto(origin + '/documents/' + doc.id);
      await page.getByLabel('登录邮箱', { exact: true }).fill(owner.email);
      await page.getByLabel('登录密码', { exact: true }).fill(owner.password);
      await page.getByRole('button', { name: '登录', exact: true }).click();
      await page.getByRole('button', { name: '编辑文档', exact: true }).click();
      await page.getByRole('link', { name: '编辑主线', exact: true }).first().click();
      const editor = page.getByRole('textbox', { name: '主线编辑器', exact: true });
      await expect(editor).toBeEditable();
      await expect(page.getByRole('region', { name: '主线编辑区', exact: true })).toHaveAttribute(
        'aria-busy',
        'false',
      );
      await page.getByText('服务端已保存', { exact: true }).waitFor();
      await editor.click();
      await page.keyboard.insertText('fmt.Println("行内代码")');
      await page.keyboard.press('Control+a');
      await page.getByRole('button', { name: '行内代码', exact: true }).click();
      const inline = editor.locator('p > code').first();
      await expect(inline).toContainText('fmt.Println');
      await expect(inline).toHaveCSS('background-color', 'rgb(237, 242, 239)');
      await editor.focus();
      await page.keyboard.press('Control+End');
      // 原生光标移动通过selectionchange异步送到PM；等待两帧再按Enter，避免自动化仍沿用Ctrl+A。
      await page.evaluate(
        () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
      );
      await page.keyboard.press('Enter');
      await expect(editor.locator('p')).toHaveCount(2);
      await page.getByRole('button', { name: '代码块', exact: true }).click();
      const source = 'package main\n\nfunc main() {\n\tfmt.Println("<安全原文>")\n}';
      await editor.evaluate((node, source) => {
        const data = new DataTransfer();
        data.setData('text/plain', source);
        data.setData('text/html', '<p>不应覆盖代码原文</p>');
        node.dispatchEvent(
          new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }),
        );
      }, source);
      const block = editor.locator('.code-block');
      await expect(block.locator('pre code')).toHaveText(source);
      assert.equal(await block.locator('pre code').textContent(), source);
      await block.getByRole('combobox', { name: '代码语言', exact: true }).selectOption('go');
      await expect(block.locator('.hljs-keyword').first()).toBeVisible();
      await block.getByRole('button', { name: '复制代码', exact: true }).click();
      await expect(block.getByRole('status')).toHaveText('已复制');
      // Windows系统剪贴板会将LF转为CRLF；只归一化系统换行，空白、缩进和源码必须相同。
      assert.equal(
        (await page.evaluate(() => navigator.clipboard.readText())).replaceAll('\r\n', '\n'),
        source,
      );
      mkdirSync('artifacts/reported-usability', { recursive: true });
      await page
        .locator('.unit-editor')
        .screenshot({ path: 'artifacts/reported-usability/code-editor-desktop.png' });
      await page.getByRole('button', { name: '保存并返回阅读', exact: true }).click();
      const body = page.getByLabel('主线正文', { exact: true }).first();
      await expect(body.locator('.code-block pre code')).toHaveText(source);
      assert.equal(await body.locator('.code-block pre code').textContent(), source);
      await expect(body.getByRole('combobox', { name: '代码语言', exact: true })).toBeDisabled();
      await page.reload();
      await expect(body.locator('.code-block pre code')).toHaveText(source);
      await expect(body.locator('.hljs-keyword').first()).toBeVisible();
      await expect(body.locator('p > code').first()).toHaveText('fmt.Println("行内代码")');
      await body.getByRole('button', { name: '复制代码', exact: true }).click();
      assert.equal(
        (await page.evaluate(() => navigator.clipboard.readText())).replaceAll('\r\n', '\n'),
        source,
      );
      await page.setViewportSize({ width: 375, height: 900 });
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        false,
      );
      await body.screenshot({ path: 'artifacts/reported-usability/code-reading-mobile.png' });
      await page.setViewportSize({ width: 1440, height: 1100 });
      await page.getByRole('button', { name: '导出文档', exact: true }).click();
      const exportPanel = page.getByRole('region', { name: '导出固定版本', exact: true });
      await exportPanel.getByLabel('文件格式').selectOption('MARKDOWN');
      await exportPanel.getByRole('button', { name: '创建导出', exact: true }).click();
      const link = exportPanel.getByRole('link', { name: '下载导出文件', exact: true });
      await expect(link).toBeVisible({ timeout: 20000 });
      const href = await link.getAttribute('href');
      const slot = await page.evaluate(() =>
        sessionStorage.getItem('collaboration-report.session-context'),
      );
      assert.equal(new URL(href, origin).searchParams.get('session_context'), slot);
      const denied = await page.evaluate(async (href) => {
        const target = new URL(href, location.origin);
        target.searchParams.set('session_context', 'f'.repeat(32));
        return (await fetch(target)).status;
      }, href);
      // 下载入口对无权身份隐藏任务是否存在，沿原协议返回404，不交付文件正文。
      assert.equal(denied, 404);
      const waiting = page.waitForEvent('download');
      await link.click();
      const download = await waiting;
      assert.equal(await download.failure(), null);
      const archive = 'artifacts/reported-usability/code-export.zip';
      await download.saveAs(archive);
      assert.equal(readFileSync(archive).subarray(0, 2).toString(), 'PK');
      assert.deepEqual(errors, []);
    } finally {
      await browser.close();
    }
  },
);
