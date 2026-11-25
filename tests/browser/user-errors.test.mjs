import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';

// 拦截账号API，不注册用户或发送邮件；验证真正构建后的页面，而非只验证词条函数。
test('account screens show actionable messages and never render upstream errors', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  let failure = { status: 401, code: 'LOGIN_INVALID' };
  try {
    await page.clock.install();
    await page.route('**/v1/**', (route) => {
      const pathname = new URL(route.request().url()).pathname;
      if (pathname.endsWith('/me'))
        return route.fulfill({ status: 401, json: { code: 'SESSION_INVALID' } });
      return route.fulfill({
        status: failure.status,
        json: { code: failure.code, message: 'private SMTP host; SELECT password_hash FROM user' },
      });
    });
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill('error-display@example.test');
    await page.getByLabel('登录密码', { exact: true }).fill('display-test-password');
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveText('邮箱或密码不正确');
    failure = { status: 403, code: 'PRIVATE_UNKNOWN_CODE' };
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveText('你没有执行此操作的权限，请联系管理员。');
    failure = { status: 503, code: 'SERVICE_UNAVAILABLE' };
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveText('服务暂时不可用，请稍后重试。');
    await page.getByRole('tab', { name: '注册', exact: true }).click();
    await page.getByLabel('验证邮箱', { exact: true }).fill('error-display@example.test');
    failure = { status: 503, code: 'SMTP_DELIVERY_FAILED' };
    await page.getByRole('button', { name: '发送验证码', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveText(
      '邮件服务器暂时未能接收验证码，请等待倒计时结束后重新发送。',
    );
    await expect(page.getByRole('button', { name: /秒后重新发送/ })).toBeDisabled();
    const text = await page.getByRole('alert').innerText();
    assert.doesNotMatch(
      text,
      /SMTP_DELIVERY_FAILED|SERVICE_UNAVAILABLE|PRIVATE_UNKNOWN_CODE|password_hash|private SMTP/,
    );
  } finally {
    await context.close();
    await browser.close();
  }
});
