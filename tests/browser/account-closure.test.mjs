import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { state } from '../../scripts/runtime-lib.mjs';
import { createUser } from '../helpers/account.mjs';
import { organizationFixture } from '../helpers/workspace.mjs';

async function enter(page, user) {
  page.setDefaultTimeout(8000);
  await page.goto('https://localhost:' + state().ports.edge);
  await page.getByLabel('登录邮箱', { exact: true }).fill(user.email);
  await page.getByLabel('登录密码', { exact: true }).fill(user.password);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await page.getByRole('button', { name: '账号设置', exact: true }).click();
  await page.getByRole('button', { name: '申请注销账号', exact: true }).click();
  await page.getByLabel('注销确认密码', { exact: true }).fill(user.password);
  await page
    .getByRole('checkbox', { name: '我理解账号冻结后将无法继续使用，确认申请注销' })
    .check();
}

// 响应丢失和查询503都不能显示成功；刷新必须靠原号恢复，且存储中不得包含密码。
test('account closure resumes the same operation after a lost response and reload', async () => {
  const user = await createUser();
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    const page = await context.newPage();
    await enter(page, user);
    const inputs = [];
    await page.route('**/v1/account/closures', async (route) => {
      inputs.push(route.request().postDataJSON());
      const result = await route.fetch();
      assert.equal(result.status(), 202);
      await route.abort('failed');
    });
    const progress = '**/v1/account/closures/*';
    await page.route(progress, (route) =>
      route.fulfill({ status: 503, json: { code: 'DEPENDENCY_UNAVAILABLE' } }),
    );
    await page.getByRole('button', { name: '确认申请注销', exact: true }).click();
    await page.getByText('注销申请结果尚未确认；', { exact: false }).waitFor();
    assert.equal(inputs.length, 1);
    assert.match(inputs[0].expected_identity_epoch, /^[1-9][0-9]*$/);
    await page.getByRole('button', { name: '查询注销进度', exact: true }).click();
    await page.getByText('DEPENDENCY_UNAVAILABLE', { exact: false }).waitFor();
    assert.equal(await page.getByText('账号已注销', { exact: true }).count(), 0);
    const stored = await page.evaluate(() =>
      sessionStorage.getItem('collaboration-report.account-closure'),
    );
    assert.equal(JSON.parse(stored).operation, inputs[0].operation_id);
    assert.equal(stored.includes(user.password), false);
    await page.unroute(progress);
    await page.reload();
    await page.getByText('账号已注销', { exact: true }).waitFor({ timeout: 30000 });
    assert.equal(await page.getByRole('button', { name: '退出登录', exact: true }).count(), 0);
    assert.equal(inputs.length, 1);
    await page.getByText('按留存规则保留 0 个正式版本。', { exact: true }).waitFor();
    assert.equal(
      await page.evaluate(() => sessionStorage.getItem('collaboration-report.csrf')),
      null,
    );

    // 原账号已冻结的恢复标签可以继续读进度；另一标签后来登录的真实账号不能被广播清掉。
    const other = await createUser();
    const otherPage = await context.newPage();
    otherPage.setDefaultTimeout(8000);
    await otherPage.goto('https://localhost:' + state().ports.edge);
    await otherPage.getByLabel('登录邮箱', { exact: true }).fill(other.email);
    await otherPage.getByLabel('登录密码', { exact: true }).fill(other.password);
    await otherPage.getByRole('button', { name: '登录', exact: true }).click();
    await otherPage.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    const queried = page.waitForResponse((response) =>
      response.url().endsWith('/closures/' + inputs[0].operation_id),
    );
    await page.getByRole('button', { name: '查询注销进度', exact: true }).click();
    assert.equal((await queried).status(), 200);
    await page.getByText('账号已注销', { exact: true }).waitFor();
    assert.equal(await otherPage.getByRole('button', { name: '退出登录', exact: true }).count(), 1);
    assert.equal(
      await otherPage.evaluate(
        () => JSON.parse(sessionStorage.getItem('collaboration-report.csrf')).user_id,
      ),
      other.id,
    );
    await context.close();
  } finally {
    await browser.close();
  }
});

// 有真实其他成员才需要所有者交接；空组织可自动关闭，不能用空组织制造交接预期。
// 准备期必须保持登录与真实管理入口；往返交接页不得另建注销操作。
test('account closure keeps succession navigation available during preparation', async () => {
  const { owner: user, organization } = await organizationFixture(1);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    const page = await context.newPage();
    await enter(page, user);
    const inputs = [];
    page.on('request', (request) => {
      if (request.method() === 'POST' && request.url().endsWith('/v1/account/closures'))
        inputs.push(request.postDataJSON());
    });
    await page.getByRole('button', { name: '确认申请注销', exact: true }).click();
    const panel = page.getByRole('region', { name: '账号注销', exact: true });
    await panel
      .getByText('组织所有者 · ' + organization, { exact: false })
      .waitFor({ timeout: 20000 });
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await panel.getByRole('button', { name: '前往组织管理交接', exact: true }).first().click();
    await page.getByRole('button', { name: '返回注销进度', exact: true }).click();
    await panel.getByText('原操作编号：' + inputs[0].operation_id, { exact: true }).waitFor();
    assert.equal(inputs.length, 1);
    await context.close();
  } finally {
    await browser.close();
  }
});
