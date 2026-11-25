import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { chromium, expect } from '@playwright/test';
import { organizationFixture, get, json } from '../helpers/workspace.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

// 集中覆盖本次反馈的真实入口。仅让本测试新建文档的一个阅读视图到期，
// 不改正文、权限、用户已有文档，也不靠长时间等待复现 EXPORT_MAIN_VIEW_EXPIRED。
test('existing KB creation, body formatting and expired-view exports work in one real browser flow', async () => {
  const fixture = await organizationFixture(0);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const database = await reviewDatabase();
  try {
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 960 },
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [],
      views = [],
      exportRequests = [],
      exportResponses = [];
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('response', async (response) => {
      if (/\/v1\/documents\/\d+\/main$/.test(new URL(response.url()).pathname) && response.ok())
        views.push(await response.json());
      if (/\/exports$|\/export-jobs\/\d+$/.test(new URL(response.url()).pathname)) {
        const value = await response.json();
        exportResponses.push({
          status: response.status(),
          id: value.id,
          format: value.format,
          state: value.state,
          code: value.code,
        });
      }
    });
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(fixture.owner.email);
    await page.getByLabel('登录密码', { exact: true }).fill(fixture.owner.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page
      .getByRole('group', { name: '空间类型', exact: true })
      .getByRole('button', { name: '组织空间', exact: true })
      .click();
    await page.getByLabel('组织', { exact: true }).selectOption({ label: '集成验证组织' });
    await page.getByRole('button', { name: '打开知识库：技术实践库', exact: true }).click();
    await page.getByRole('button', { name: '在本库新增文档', exact: true }).click();
    await expect(page.getByLabel('文档标题', { exact: true })).toBeFocused();
    await page.getByLabel('文档标题', { exact: true }).fill('阅读凭证到期验证');
    await page.getByRole('button', { name: '创建文档', exact: true }).click();
    await page.getByRole('heading', { name: '阅读凭证到期验证', exact: true }).waitFor();
    await page.getByRole('region', { name: '正文样式', exact: true }).waitFor();
    await expect.poll(() => views.length).toBeGreaterThan(0);
    const id = views.at(-1).document.id;
    const created = json(await get('/v1/documents/' + id, fixture.owner.headers));
    assert.equal(created.knowledge_base_id, fixture.kb);
    assert.equal(created.space_id, fixture.space);
    assert.equal(created.lifecycle, 'ACTIVE');

    await expect(page.getByLabel('正文中文字体', { exact: true })).toBeDisabled();
    await page.getByRole('button', { name: '解锁主线', exact: true }).click();
    await expect(page.getByLabel('正文中文字体', { exact: true })).toBeEnabled();
    await page.getByLabel('正文中文字体', { exact: true }).selectOption('FangSong');
    await page.getByRole('button', { name: '应用正文样式', exact: true }).click();
    await expect.poll(() => views.at(-1)?.style.style.body_font).toBe('FangSong');
    await page.getByRole('button', { name: '编辑主线', exact: true }).first().click();
    const editor = page.getByRole('textbox', { name: '主线编辑器', exact: true });
    await editor.fill('正文格式与导出保持一致');
    await page.getByRole('button', { name: '二级标题', exact: true }).click();
    await editor.locator('h2').waitFor();
    await page.getByRole('button', { name: '正文段落', exact: true }).click();
    await editor.locator('p').waitFor();
    assert.equal(await editor.locator('h2').count(), 0);
    await page.getByText('服务端已保存', { exact: true }).waitFor();
    await page.getByRole('button', { name: '刷新阅读', exact: true }).click();
    await page
      .getByLabel('主线正文', { exact: true })
      .getByText('正文格式与导出保持一致', { exact: true })
      .waitFor();
    const oldView = views.at(-1);
    const [expired] = await database.execute(
      'UPDATE read_view SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE view_id=? AND document_id=?',
      [oldView.read_view_id, id],
    );
    assert.equal(expired.affectedRows, 1);

    // 第一次让已登记的POST丢掉响应；续接仍须用原号、原快照。
    let loseReply = true;
    await page.route('**/v1/documents/' + id + '/exports', async (route) => {
      exportRequests.push(route.request().postDataJSON());
      if (loseReply) {
        loseReply = false;
        const response = await route.fetch();
        assert.equal(response.status(), 202);
        await route.abort('failed');
      } else await route.continue();
    });
    const panel = page.getByRole('region', { name: '导出固定版本', exact: true });
    const directory = resolve(root, 'artifacts/reported-usability');
    mkdirSync(directory, { recursive: true });
    const formats = [
      ['DOCX', 'docx', 'PK'],
      ['PDF', 'pdf', '%PDF'],
      ['MARKDOWN', 'zip', 'PK'],
    ];
    for (const [format, extension, signature] of formats) {
      await panel.getByLabel('文件格式').selectOption(format);
      await panel.getByRole('button', { name: '创建导出', exact: true }).click();
      if (format === 'DOCX') {
        await panel.getByRole('button', { name: '确认操作结果', exact: true }).click();
      }
      try {
        // 点击不会等待Vue的异步提交；先确认本次格式的新任务，不能误点上一份文件的下载链接。
        await expect
          .poll(
            () =>
              exportResponses.some(
                (response) => response.format === format && response.state === 'SUCCEEDED',
              ),
            { timeout: 60000 },
          )
          .toBe(true);
        const ready = exportResponses.findLast(
          (response) => response.format === format && response.state === 'SUCCEEDED',
        );
        await expect(
          panel.getByRole('link', { name: '下载导出文件', exact: true }),
        ).toHaveAttribute(
          'href',
          new RegExp('^/v1/export-jobs/' + ready.id + '/download\\?session_context=[a-f0-9]{32}$'),
        );
        await panel
          .getByRole('link', { name: '下载导出文件', exact: true })
          .waitFor({ timeout: 60000 });
      } catch (error) {
        console.error(JSON.stringify({ format, panel: await panel.innerText(), exportResponses }));
        throw error;
      }
      const waiting = page.waitForEvent('download');
      await panel.getByRole('link', { name: '下载导出文件', exact: true }).click();
      const download = await waiting;
      assert.equal(await download.failure(), null);
      const path = resolve(directory, 'unchanged-after-expiry.' + extension);
      await download.saveAs(path);
      assert.equal(readFileSync(path).subarray(0, signature.length).toString(), signature);
    }
    assert.equal(new Set(exportRequests.map((request) => request.operation_id)).size, 3);
    assert.equal(exportRequests[0].operation_id, exportRequests[1].operation_id);
    assert.equal(exportRequests[0].expected_manifest_id, exportRequests[1].expected_manifest_id);
    assert.notEqual(exportRequests[0].expected_manifest_id, oldView.manifest_id);
    assert.deepEqual(errors, []);
    await page.screenshot({ path: resolve(directory, 'product-fixed.png'), fullPage: true });
    await context.close();
  } finally {
    await database.end();
    await browser.close();
  }
});

test('delivery mode and distinct search document identities are visible in the real UI', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const page = await browser.newPage({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 960 },
    });
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByRole('tab', { name: '注册', exact: true }).click();
    await page
      .getByLabel('验证邮箱', { exact: true })
      .fill('fixture-' + randomUUID() + '@example.test');
    await page.getByRole('button', { name: '发送验证码', exact: true }).click();
    await page
      .getByText('当前使用本地开发收件箱，邮件不会发送到外部邮箱。请从开发收件箱获取验证码。', {
        exact: true,
      })
      .waitFor();
    await page.goto('https://localhost:' + state().ports.edge + '/search');
    await page.getByLabel('搜索词', { exact: true }).fill('设定');
    await page.getByLabel('每页数量', { exact: true }).selectOption('100');
    const waiting = page.waitForResponse(
      (response) => new URL(response.url()).pathname === '/v1/search' && response.ok(),
    );
    await page.getByRole('button', { name: '搜索', exact: true }).click();
    const data = await (await waiting).json();
    const rows = page.locator('.search-list li');
    await expect(rows).toHaveCount(data.items.length);
    const ids = await rows.evaluateAll((nodes) => nodes.map((node) => node.dataset.documentId));
    assert.equal(new Set(ids).size, ids.length);
    await expect(page.locator('.search-identity')).toHaveCount(ids.length);
    assert.equal(data.items.length, new Set(data.items.map((item) => item.document_id)).size);
    const directory = resolve(root, 'artifacts/reported-usability');
    mkdirSync(directory, { recursive: true });
    await page.screenshot({ path: resolve(directory, 'search-identities.png'), fullPage: true });
    console.log(
      JSON.stringify({
        mailMode: 'DEVELOPMENT',
        searchTerm: '设定',
        items: ids.length,
        uniqueDocuments: new Set(ids).size,
        sameTitleReports: data.items.filter((item) => item.title === '协作报告').length,
      }),
    );
  } finally {
    await browser.close();
  }
});

// 只模拟验证码响应，不发邮件。浏览器跨时区时仍须显示同一北京时间，
// 防止把界面的本地时刻误当成服务端按600秒校验的实际有效期。
test('challenge expiry explicitly shows Beijing time across browser time zones', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    for (const timezoneId of ['Asia/Tokyo', 'America/Los_Angeles']) {
      const context = await browser.newContext({ ignoreHTTPSErrors: true, timezoneId });
      const page = await context.newPage();
      await page.clock.install();
      await page.route('**/v1/account/challenges', (route) =>
        route.fulfill({
          status: 200,
          headers: { 'content-type': 'application/json', 'X-Collaboration-Mail-Delivery': 'SMTP' },
          body: JSON.stringify({
            challenge_id: randomUUID(),
            expires_at: '2026-10-09T06:00:38Z',
            retry_after_seconds: 60,
          }),
        }),
      );
      await page.goto('https://localhost:' + state().ports.edge);
      await page.getByRole('tab', { name: '注册', exact: true }).click();
      await page.getByLabel('验证邮箱', { exact: true }).fill('time-zone-check@example.test');
      await page.getByRole('button', { name: '发送验证码', exact: true }).click();
      await expect(page.getByText(/验证码有效至/)).toContainText('2026/10/9 14:00:38（北京时间）');
      // 重发后不能把上一封邮件的输入带入新的挑战；响应仍完全模拟，不消耗真实验证码。
      await page.getByLabel('邮箱验证码', { exact: true }).fill('000000');
      await expect(page.getByRole('button', { name: /秒后重新发送/ })).toBeDisabled();
      await page.clock.fastForward(60000);
      await page.getByRole('button', { name: '发送验证码', exact: true }).click();
      await expect(page.getByLabel('邮箱验证码', { exact: true })).toHaveValue('');
      await context.close();
    }
  } finally {
    await browser.close();
  }
});

// 全部账号API均拦截，测试在途/刷新/限流及密码误输时绝不创建真实用户或触发SMTP。
test('account verification cooldown survives refresh and password confirmation stays client-side', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  const page = await context.newPage();
  const challengeRequests = [],
    signupRequests = [];
  let releaseChallenge,
    challengeFailure = '';
  try {
    await page.clock.install();
    await page.route('**/v1/**', async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path === '/v1/account/challenges') {
        challengeRequests.push(route.request().postDataJSON());
        if (challengeRequests.length === 1)
          await new Promise((resolve) => (releaseChallenge = resolve));
        return route.fulfill({
          status: challengeFailure === 'CHALLENGE_RATE_LIMITED' ? 429 : 200,
          headers: { 'content-type': 'application/json', 'X-Collaboration-Mail-Delivery': 'SMTP' },
          body: JSON.stringify(
            challengeFailure
              ? { code: challengeFailure }
              : {
                  challenge_id: randomUUID(),
                  expires_at: new Date(Date.now() + 600000).toISOString(),
                  retry_after_seconds: 60,
                },
          ),
        });
      }
      if (path === '/v1/account/signup') {
        signupRequests.push(route.request().postDataJSON());
        return route.fulfill({
          status: 409,
          json: { code: 'EMAIL_ALREADY_REGISTERED' },
        });
      }
      return route.fulfill({ status: 401, json: { code: 'SESSION_INVALID' } });
    });
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByRole('tab', { name: '注册', exact: true }).click();
    await page.getByLabel('验证邮箱', { exact: true }).fill('cooldown-check@example.test');
    await page.getByRole('button', { name: '发送验证码', exact: true }).click();
    await expect(page.getByRole('button', { name: '发送中…', exact: true })).toBeDisabled();
    assert.equal(challengeRequests.length, 1);
    releaseChallenge();
    await expect(page.getByRole('button', { name: '60 秒后重新发送', exact: true })).toBeDisabled();
    await page.clock.fastForward(15000);
    await expect(page.getByRole('button', { name: '45 秒后重新发送', exact: true })).toBeDisabled();
    await page.getByRole('tab', { name: '找回密码', exact: true }).click();
    await expect(page.getByRole('button', { name: '发送验证码', exact: true })).toBeEnabled();
    await page.getByRole('tab', { name: '注册', exact: true }).click();
    await expect(page.getByRole('button', { name: /秒后重新发送/ })).toBeDisabled();
    await page.reload();
    await page.getByRole('tab', { name: '注册', exact: true }).click();
    await page.getByLabel('验证邮箱', { exact: true }).fill('COOLDOWN-CHECK@example.test');
    await expect(page.getByRole('button', { name: /秒后重新发送/ })).toBeDisabled();
    const stored = await page.evaluate(() =>
      sessionStorage.getItem('collaboration-report.challenge-cooldowns.v1'),
    );
    assert.ok(stored);
    assert.equal(stored.includes('cooldown-check@example.test'), false);
    await page.clock.fastForward(46000);
    await expect(page.getByRole('button', { name: '发送验证码', exact: true })).toBeEnabled();
    await page.getByRole('button', { name: '发送验证码', exact: true }).click();
    await expect(page.getByText('验证码请求已受理，请查看投递提示', { exact: true })).toBeVisible();
    await page.getByLabel('显示名称', { exact: true }).fill('仅用于前端验证');
    await page.getByLabel('邮箱验证码', { exact: true }).fill('000000');
    for (const weak of ['abcdefgh', '12345678', '!!!!!!!!', 'abcdEFGH']) {
      await page.getByLabel('新密码', { exact: true }).fill(weak);
      await page.getByLabel('确认密码', { exact: true }).fill(weak);
      await page.getByRole('button', { name: '验证并注册', exact: true }).click();
      await expect(page.getByRole('alert')).toContainText('英文字母、数字、特殊字符至少包含两类');
      assert.equal(signupRequests.length, 0, '只有一类字符时不得发送注册写入');
    }
    await page.getByLabel('新密码', { exact: true }).fill('typed-password');
    await page.getByLabel('确认密码', { exact: true }).fill('different-password');
    await page.getByRole('button', { name: '验证并注册', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveText('两次输入的密码不一致，请重新确认。');
    assert.equal(signupRequests.length, 0);
    await page.getByLabel('确认密码', { exact: true }).fill('typed-password');
    await page.getByRole('button', { name: '验证并注册', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveText(
      '该邮箱已注册，请直接登录；忘记密码可使用“找回密码”。',
    );
    assert.equal(signupRequests.length, 1);
    assert.deepEqual(Object.keys(signupRequests[0]).sort(), [
      'challenge_id',
      'code',
      'display_name',
      'email',
      'operation_id',
      'password',
    ]);
    challengeFailure = 'CHALLENGE_RATE_LIMITED';
    await page.getByRole('tab', { name: '找回密码', exact: true }).click();
    await page.getByRole('button', { name: '发送验证码', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveText(
      '验证码请求过于频繁，请等待倒计时结束后重试。',
    );
    await expect(page.getByRole('button', { name: /秒后重新发送/ })).toBeDisabled();
    assert.equal(
      await page.getByText('验证码请求已受理，请查看投递提示', { exact: true }).count(),
      0,
    );
    assert.equal(await page.getByText(/验证码有效至/).count(), 0);
    await page.clock.fastForward(60000);
    challengeFailure = '';
    await page.getByRole('button', { name: '发送验证码', exact: true }).click();
    await expect(page.getByRole('button', { name: /秒后重新发送/ })).toBeDisabled();
    assert.equal(challengeRequests.at(-1).purpose, 'RESET_PASSWORD');
    await page.getByLabel('邮箱验证码', { exact: true }).fill('000000');
    await page.getByLabel('新密码', { exact: true }).fill('abcdefgh');
    await page.getByRole('button', { name: '更新密码', exact: true }).click();
    await expect(page.getByRole('alert')).toContainText('密码至少8个字符');
  } finally {
    releaseChallenge?.();
    await context.close();
    await browser.close();
  }
});

test('change-email verification shows in-flight state and the same sixty-second cooldown', async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  const page = await context.newPage();
  const challenges = [];
  let releaseChallenge;
  try {
    await page.clock.install();
    await context.addInitScript(() =>
      sessionStorage.setItem(
        'collaboration-report.csrf',
        JSON.stringify({ user_id: '987654321', token: 'mock-csrf-for-account-ui' }),
      ),
    );
    await page.route('**/v1/**', async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path === '/v1/account/me')
        return route.fulfill({
          status: 200,
          json: {
            id: '987654321',
            email: 'mock-account@example.test',
            display_name: '账号界面验证',
            state: 'ACTIVE',
            credential_epoch: '1',
            identity_epoch: '1',
          },
        });
      if (path === '/v1/account/challenges') {
        challenges.push(route.request().postDataJSON());
        if (challenges.length === 1) await new Promise((resolve) => (releaseChallenge = resolve));
        return route.fulfill({
          status: 200,
          headers: { 'content-type': 'application/json', 'X-Collaboration-Mail-Delivery': 'SMTP' },
          body: JSON.stringify({
            challenge_id: randomUUID(),
            expires_at: new Date(Date.now() + 600000).toISOString(),
            retry_after_seconds: 60,
          }),
        });
      }
      return route.fulfill({ status: 403, json: { code: 'MOCK_UI_ONLY' } });
    });
    await page.goto('https://localhost:' + state().ports.edge + '/account');
    await page.getByLabel('新邮箱', { exact: true }).fill('new-address@example.test');
    await page.getByRole('button', { name: '验证新邮箱', exact: true }).click();
    await expect(page.getByRole('button', { name: '发送中…', exact: true })).toBeDisabled();
    releaseChallenge();
    await expect(page.getByRole('button', { name: '60 秒后重新发送', exact: true })).toBeDisabled();
    await page.getByLabel('新邮箱验证码', { exact: true }).fill('000000');
    await page.clock.fastForward(60000);
    await page.getByRole('button', { name: '验证新邮箱', exact: true }).click();
    await expect(page.getByLabel('新邮箱验证码', { exact: true })).toHaveValue('');
    await expect(page.getByRole('button', { name: /秒后重新发送/ })).toBeDisabled();
    assert.equal(challenges.length, 2);
    assert.ok(challenges.every((request) => request.purpose === 'CHANGE_EMAIL'));
    await page.reload();
    await page.getByLabel('新邮箱', { exact: true }).fill('new-address@example.test');
    await expect(page.getByRole('button', { name: /秒后重新发送/ })).toBeDisabled();
  } finally {
    releaseChallenge?.();
    await context.close();
    await browser.close();
  }
});
