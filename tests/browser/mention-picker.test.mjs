import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { chromium, expect } from '@playwright/test';
import { documentFixture, organizationFixture, get, post, json } from '../helpers/workspace.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

test('提及协作者显示提示、按姓名选择，并支持分页、移除和重名区分', { timeout: 60000 }, async () => {
  // 真实SMTP环境复用体验身份，只创建隔离业务文档，不发送验证码或测试提及通知。
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
  const endpoint = `/v1/documents/${doc.id}/mention-candidates`;
  const realCandidates = await get(endpoint, owner.headers);
  assert.equal(realCandidates.status, 200, realCandidates.body);
  assert.ok(
    json(realCandidates).items.some((member) => member.user_id === owner.id && member.display_name),
  );
  assert.equal((await get(endpoint)).status, 401);

  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1440, height: 1000 },
    });
    // 候选展示的分页/同名只用读取替身；真实接口与身份准入已在上方核验。
    // 不把替身账号提交给评论接口，更不生成额外账号或扩大通知范围。
    await context.route('**' + endpoint + '?**', async (route) => {
      const next = new URL(route.request().url()).searchParams.has('cursor');
      await route.fulfill({
        json: {
          items: next
            ? [{ user_id: '91003', display_name: '陈清' }]
            : [
                { user_id: owner.id, display_name: '本人' },
                { user_id: '91001', display_name: '林晓' },
                { user_id: '91002', display_name: '林晓' },
              ],
          has_more: !next,
          next_cursor: next ? null : 'mention-ui-second-page',
        },
      });
    });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    page.setDefaultTimeout(10000);
    await page.goto('https://localhost:' + state().ports.edge + '/documents/' + doc.id);
    await page.getByLabel('登录邮箱', { exact: true }).fill(email);
    await page.getByLabel('登录密码', { exact: true }).fill(password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('tab', { name: '评论', exact: true }).click();
    const picker = page.getByRole('group', { name: '@ 提及协作者（可选）', exact: true });
    await expect(
      picker.getByText('按姓名选择要提醒的人；发表后，有权查看这条评论的协作者会收到通知。', {
        exact: true,
      }),
    ).toBeVisible();
    const search = picker.getByLabel('查找要提及的协作者', { exact: true });
    await search.fill('林晓');
    const first = picker.getByRole('button', { name: '提及 林晓（编号 91001）', exact: true });
    await expect(picker.getByRole('button', { name: '提及 本人', exact: true })).toHaveCount(0);
    await first.click();
    await expect(picker.getByRole('list', { name: '已选提及对象' })).toContainText(
      '@林晓（编号 91001）',
    );
    await expect(first).toBeDisabled();
    await picker.getByRole('button', { name: '移除 林晓（编号 91001）', exact: true }).click();
    await expect(first).toBeEnabled();
    await search.fill('陈清');
    await expect(
      picker.getByText('暂未找到这个姓名。可以换个名字，或继续加载更多协作者。', { exact: true }),
    ).toBeVisible();
    await picker.getByRole('button', { name: '加载更多协作者', exact: true }).click();
    await picker.getByRole('button', { name: '提及 陈清', exact: true }).click();
    await expect(picker.getByRole('list', { name: '已选提及对象' })).toContainText('@陈清');

    const artifacts = resolve(root, 'artifacts/reported-usability');
    mkdirSync(artifacts, { recursive: true });
    await picker.scrollIntoViewIfNeeded();
    await page.screenshot({
      path: resolve(artifacts, 'mention-picker-desktop.png'),
      fullPage: true,
    });
    await page.setViewportSize({ width: 320, height: 900 });
    await picker.scrollIntoViewIfNeeded();
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
      false,
    );
    await page.screenshot({
      path: resolve(artifacts, 'mention-picker-mobile.png'),
      fullPage: true,
    });
    assert.deepEqual(errors, []);
  } finally {
    await browser.close();
  }
});
