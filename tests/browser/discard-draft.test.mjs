import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { resolve } from 'node:path';
import { reviewFixture, openReviewWorkspace, writeDraft, submitDraft } from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';
import { pollDiscardDraft } from '../helpers/discard-previews.mjs';

async function open(page, f) {
  page.setDefaultTimeout(10000);
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
  await page.getByRole('button', { name: '协作报告', exact: true }).click();
  await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
  const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
  await editor.waitFor();
  return editor;
}

// 预览中的S1必须来自保留版本；确认前S2先真实保存，然后由原预览明确拒绝。
test('browser fixed discard preview retains S1 and rejects later saved S2 without recapturing', async () => {
  const f = await reviewFixture();
  let browser, page;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    page = await browser.newPage({ ignoreHTTPSErrors: true });
    const editor = await open(page, f),
      previews = [],
      confirms = [],
      errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await page.route('**/v1/documents/*/units/*/discard-previews', async (route) => {
      previews.push(route.request().postDataJSON());
      await route.continue();
    });
    await page.route('**/v1/documents/*/units/*/discard-draft', async (route) => {
      confirms.push(route.request().postDataJSON());
      await route.continue();
    });
    await editor.fill('固定预览前S1旧稿');
    const preview = page.getByRole('button', { name: '生成丢弃预览', exact: true });
    assert.equal(await preview.count(), 1, '管理者缺少已验证的丢弃预览入口');
    await preview.click();
    const fixed = page.getByRole('document', { name: '待保留旧草稿', exact: true });
    await fixed.waitFor();
    assert.equal(await fixed.innerText(), '固定预览前S1旧稿');
    await editor.fill('预览后S2不能被抹掉');
    await page
      .getByLabel('我确认保留所示旧稿，恢复共同草稿并归档已撤回工作', { exact: true })
      .check();
    await page.getByRole('button', { name: '确认丢弃并恢复主线', exact: true }).click();
    await page.locator('.discard-control .error').filter({ hasText: 'DRAFT_CHANGED' }).waitFor();
    assert.equal(await editor.innerText(), '预览后S2不能被抹掉');
    assert.equal(await fixed.innerText(), '固定预览前S1旧稿');
    assert.equal(previews.length, 1);
    assert.equal(confirms.length, 1);
    assert.equal(confirms[0].preview_cut_ref, previews[0].operation_id);
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/discard-changed-browser.png'),
      fullPage: true,
    });
  } finally {
    if (page)
      await page.screenshot({
        path: resolve(root, 'artifacts/batches/04/discard-browser-current.png'),
        fullPage: true,
      });
    f.close();
    await browser?.close();
  }
});

test('browser requires withdrawal and one explicit reason per blocking opinion, then archives and starts a new review', async () => {
  const f = await reviewFixture();
  let browser, next;
  try {
    await writeDraft(f, '需要保留并归档的旧稿');
    const round = (await submitDraft(f)).value.result;
    for (const reason of ['补充实验来源', '修正比较结论']) {
      const response = await post(
        `/v1/review-requests/${round.request_id}/actions`,
        {
          operation_id: operation('discard-browser-opinion'),
          round_id: round.round_id,
          content_version_id: round.content_version_id,
          type: 'REQUEST_CHANGES',
          reason,
        },
        f.user.headers,
      );
      assert.equal(response.status, 200, response.body);
    }
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true });
    const editor = await open(page, f),
      errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await page.getByRole('button', { name: '生成丢弃预览', exact: true }).click();
    await page.locator('.discard-control .error').filter({ hasText: '请先撤回' }).waitFor();
    const panel = page.getByRole('region', { name: '审核记录', exact: true });
    await panel.getByRole('button', { name: '撤回当前轮次', exact: true }).click();
    await panel.locator('.review-state').filter({ hasText: '已撤回，可编辑后重新送审' }).waitFor();
    await page.getByRole('button', { name: '生成丢弃预览', exact: true }).click();
    const fixed = page.getByRole('region', { name: '丢弃确认', exact: true });
    await fixed.getByRole('document', { name: '待保留旧草稿', exact: true }).waitFor();
    const opinions = fixed.locator('.discard-opinion');
    assert.equal(await opinions.count(), 2);
    const confirm = fixed.getByRole('button', { name: '确认丢弃并恢复主线', exact: true });
    await fixed
      .getByLabel('我确认保留所示旧稿，恢复共同草稿并归档已撤回工作', { exact: true })
      .check();
    assert.equal(await confirm.isDisabled(), true);
    await opinions
      .nth(0)
      .getByRole('textbox', { name: /处理理由/ })
      .fill('保留来源，终止此项旧工作');
    assert.equal(await confirm.isDisabled(), true);
    await opinions
      .nth(1)
      .getByRole('textbox', { name: /处理理由/ })
      .fill('结论留在旧稿，重新整理主线');
    await confirm.click();
    await editor.waitFor({ state: 'detached' });
    const archived = json(await get(`/v1/review-requests/${round.request_id}`, f.user.headers));
    assert.ok(archived.archived_at);
    const changes = json(
      await get(`/v1/review-requests/${round.request_id}/change-requests`, f.user.headers),
    );
    assert.ok(changes.items.every((change) => change.status === 'RESOLVED'));
    assert.deepEqual(
      changes.items.map((change) => change.resolution_reason),
      ['保留来源，终止此项旧工作', '结论留在旧稿，重新整理主线'],
    );
    next = await openReviewWorkspace(f.f, f.d);
    assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    await editor.waitFor();
    assert.doesNotMatch(await editor.innerText(), /需要保留并归档的旧稿/);
    await editor.fill('恢复后新的工作需要新审核');
    await page.getByRole('button', { name: '送审当前草稿', exact: true }).click();
    await page.getByText('固定送审已完成，后续草稿可继续编辑', { exact: true }).waitFor();
    const currentList = json(await get(`/v1/documents/${f.d.id}/review-requests`, f.user.headers));
    const current = currentList.items.filter(
      (request) => !request.archived_at && request.status === 'REVIEWING',
    );
    assert.equal(current.length, 1, '恢复后必须实际创建唯一的新审核');
    assert.notEqual(current[0].id, round.request_id);
    await expect(panel.getByRole('combobox', { name: '审核请求', exact: true })).toHaveValue(
      current[0].id,
    );
    await panel.getByText('恢复后新的工作需要新审核', { exact: true }).waitFor();
    // 真正两页服务端列表：第一页只有归档旧请求，当前审核在第二页，不能据第一页猜NEW。
    const pages = [];
    await page.route('**/v1/documents/*/review-requests?**', async (route) => {
      const url = new URL(route.request().url());
      url.searchParams.set('limit', '1');
      pages.push(url.searchParams.get('cursor'));
      await route.fulfill({ response: await route.fetch({ url: url.href }) });
    });
    await page.getByRole('button', { name: '生成丢弃预览', exact: true }).click();
    await page.locator('.discard-control .error').filter({ hasText: '请先撤回' }).waitFor();
    assert.equal(pages.length, 2);
    assert.equal(pages[0], null);
    assert.ok(pages[1], '第二页必须用实际服务端游标');
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/discard-installed-browser.png'),
      fullPage: true,
    });
  } finally {
    next?.close();
    f.close();
    await browser?.close();
  }
});

// 两个独立业务动作各自保留原号；原预览和原确认的恢复都不能重新FLUSH或覆盖新epoch。
test('lost real preview and confirmation responses resume their respective original operations and save boundaries', async () => {
  const f = await reviewFixture();
  let browser, next;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true });
    const previews = [],
      confirms = [],
      flushes = [],
      errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('websocket', (socket) =>
      socket.on('framesent', ({ payload }) => {
        const value = JSON.parse(String(payload));
        if (value.type === 'FLUSH') flushes.push(value.operation_id);
      }),
    );
    const editor = await open(page, f);
    await page.route('**/v1/documents/*/units/*/discard-previews', async (route) => {
      previews.push(route.request().postDataJSON());
      const response = await route.fetch();
      assert.equal(response.status(), 200);
      if (previews.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    await editor.fill('原预览回执丢失仍固定旧稿');
    await page.getByRole('button', { name: '生成丢弃预览', exact: true }).click();
    const retry = page.getByRole('button', { name: '确认操作结果', exact: true });
    await retry.waitFor();
    await retry.click();
    const fixed = page.getByRole('document', { name: '待保留旧草稿', exact: true });
    await fixed.waitFor();
    assert.equal(await fixed.innerText(), '原预览回执丢失仍固定旧稿');
    assert.equal(previews.length, 2);
    assert.deepEqual(previews[0], previews[1]);
    assert.equal(flushes.length, 1);
    await page.route('**/v1/documents/*/units/*/discard-draft', async (route) => {
      confirms.push(route.request().postDataJSON());
      const response = await route.fetch();
      assert.equal(response.status(), 202);
      if (confirms.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    await page
      .getByLabel('我确认保留所示旧稿，恢复共同草稿并归档已撤回工作', { exact: true })
      .check();
    await page.getByRole('button', { name: '确认丢弃并恢复主线', exact: true }).click();
    await retry.waitFor();
    assert.equal(
      (await pollDiscardDraft(confirms[0].operation_id, f.user.headers)).state,
      'SUCCEEDED',
    );
    await retry.click();
    await editor.waitFor({ state: 'detached' });
    assert.equal(confirms.length, 2);
    assert.deepEqual(confirms[0], confirms[1]);
    assert.equal(confirms[0].preview_cut_ref, previews[0].operation_id);
    assert.notEqual(confirms[0].operation_id, previews[0].operation_id);
    assert.equal(flushes.length, 2, '未知续接只使用两个首次取得的保存边界');
    next = await openReviewWorkspace(f.f, f.d);
    assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
    assert.deepEqual(errors, []);
  } finally {
    next?.close();
    f.close();
    await browser?.close();
  }
});

test('browser preview expiry disables a new confirmation and an explicit new preview uses a new operation', async () => {
  const f = await reviewFixture();
  let browser;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true }),
      previews = [],
      confirms = [];
    let expires;
    const editor = await open(page, f);
    await page.route('**/v1/documents/*/units/*/discard-previews', async (route) => {
      previews.push(route.request().postDataJSON());
      const response = await route.fetch();
      expires = (await response.json()).expires_at;
      await route.fulfill({ response });
    });
    await page.route('**/v1/documents/*/units/*/discard-draft', async (route) => {
      confirms.push(route.request().postDataJSON());
      await route.continue();
    });
    await editor.fill('展示时钟到期仍保留本次旧稿');
    await page.getByRole('button', { name: '生成丢弃预览', exact: true }).click();
    const fixed = page.getByRole('document', { name: '待保留旧草稿', exact: true });
    await fixed.waitFor();
    await page
      .getByLabel('我确认保留所示旧稿，恢复共同草稿并归档已撤回工作', { exact: true })
      .check();
    const confirm = page.getByRole('button', { name: '确认丢弃并恢复主线', exact: true });
    assert.equal(await confirm.isDisabled(), false);
    // 正式预览有效十分钟。本例按真实expires_at推进浏览器展示时钟，验证UI禁止另发新确认；
    // 不修改服务端/SQL时间，也不把此用例作为后台自然到期的验收证据。
    await page.clock.setSystemTime(new Date(Date.parse(expires) + 2000));
    await page.getByText('预览已到期，请关闭后重新核对。', { exact: true }).waitFor();
    assert.equal(await confirm.isDisabled(), true);
    assert.equal(confirms.length, 0);
    assert.equal(previews.length, 1);
    assert.equal(await fixed.innerText(), '展示时钟到期仍保留本次旧稿');
    await page.getByRole('button', { name: '关闭此预览', exact: true }).click();
    await page.clock.setSystemTime(new Date());
    await page.getByRole('button', { name: '生成丢弃预览', exact: true }).click();
    await fixed.waitFor();
    assert.equal(previews.length, 2);
    assert.notEqual(previews[0].operation_id, previews[1].operation_id);
    assert.equal(await fixed.innerText(), '展示时钟到期仍保留本次旧稿');
  } finally {
    f.close();
    await browser?.close();
  }
});
