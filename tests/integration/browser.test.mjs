import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import { root, state } from '../../scripts/runtime-lib.mjs';
test('Windows Edge renders four real service states at the same origin', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    // 仅本测试的隔离浏览器上下文接受开发CA；后端运输测试仍真实验证CA。
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 900 },
    });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (e) => errors.push(e.message));
    await page.goto('https://localhost:' + state().ports.edge);
    await page.locator('article .ready').nth(3).waitFor();
    assert.equal(await page.locator('article .ready').count(), 4);
    assert.equal(errors.length, 0);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/01/browser-desktop.png'),
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/01/browser-mobile.png'),
      fullPage: true,
    });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    await context.close();
  } finally {
    await browser.close();
  }
});
