import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { organizationFixture, documentFixture, get, json } from '../helpers/workspace.mjs';
import { state, root } from '../../scripts/runtime-lib.mjs';
import { resolve } from 'node:path';
import { writeFileSync } from 'node:fs';

function diagnostics(page, name) {
  const responses = [],
    errors = [],
    consoleErrors = [],
    pending = [];
  page.on('pageerror', (e) => errors.push(e.message));
  page.on('console', (message) => {
    if (['error', 'warning'].includes(message.type())) consoleErrors.push(message.text());
  });
  page.on('response', (response) => {
    const url = new URL(response.url());
    if (!url.pathname.startsWith('/v1/')) return;
    const record = { path: url.pathname, status: response.status() };
    responses.push(record);
    pending.push(
      response
        .json()
        .then((value) => {
          // 只保留稳定状态，不记录登录令牌、请求头、配置或整个账号响应。
          record.code = value.code;
          record.state = value.state;
          if (value.current_round) record.fixedBody = value.current_round.content.body_json;
        })
        .catch(() => {}),
    );
  });
  return async () => {
    await Promise.allSettled(pending);
    writeFileSync(
      resolve(root, 'artifacts/batches/04/' + name + '.json'),
      JSON.stringify(
        {
          responses,
          errors,
          consoleErrors,
          fixedMarkup: await page
            .locator('.fixed-review-body')
            .evaluateAll((nodes) => nodes.map((node) => node.outerHTML)),
          text: await page.locator('body').innerText(),
        },
        null,
        2,
      ),
    );
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/' + name + '.png'),
      fullPage: true,
    });
  };
}

async function open(page, f) {
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
  await page.locator('a[href="/documents/' + f.d.id + '"]').click();
  await openBranch(page);
  await page.getByRole('textbox', { name: '共同支线编辑器', exact: true }).waitFor();
}

async function openBranch(page) {
  await page.getByRole('tab', { name: '正文', exact: true }).click();
  await page.getByRole('button', { name: '编辑文档', exact: true }).click();
  await page.getByRole('link', { name: '编辑共同支线', exact: true }).first().click();
  await page.getByText('服务端已保存', { exact: true }).waitFor();
  await expect(page.getByRole('region', { name: '共同支线编辑区', exact: true })).toHaveAttribute(
    'aria-busy',
    'false',
  );
  await expect(page.getByRole('textbox', { name: '共同支线编辑器', exact: true })).toBeEditable();
}

async function showSubmittedReview(page) {
  await page.getByRole('button', { name: '保存并查看审核记录', exact: true }).click();
  await page
    .getByRole('region', { name: '审核记录', exact: true })
    .getByText('展开送审正文', { exact: true })
    .click();
  await page
    .getByRole('region', { name: '审核记录', exact: true })
    .getByRole('button', { name: '完整送审稿', exact: true })
    .click();
}

test('real browser immediately submits Chinese composition, keeps later draft separate and withdraws into a new round', async () => {
  const f = await organizationFixture(0);
  f.d = await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  let saveDiagnostics;
  try {
    const context = await browser.newContext({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 960 },
      }),
      page = await context.newPage(),
      errors = [];
    saveDiagnostics = diagnostics(page, 'review-browser-current');
    page.setDefaultTimeout(8000);
    page.on('pageerror', (e) => errors.push(e.message));
    await open(page, f);
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true }),
      panel = page.getByRole('region', { name: '审核记录', exact: true });
    await editor.focus();
    const cdp = await context.newCDPSession(page);
    await cdp.send('Input.imeSetComposition', {
      text: '中文候选',
      selectionStart: 4,
      selectionEnd: 4,
    });
    await cdp.send('Input.insertText', { text: '立即送审中文正文' });
    await page.getByRole('button', { name: '送审当前草稿', exact: true }).click();
    await page.getByText('固定送审已完成，后续草稿可继续编辑', { exact: true }).waitFor();
    await editor.fill('送审后继续写的内容');
    await showSubmittedReview(page);
    await panel
      .getByRole('document', { name: '固定送审正文', exact: true })
      .getByText('立即送审中文正文', { exact: true })
      .waitFor();
    assert.equal(
      await panel
        .getByRole('document', { name: '固定送审正文', exact: true })
        .getByText('送审后继续写的内容', { exact: true })
        .count(),
      0,
    );
    await panel.getByLabel('审核理由', { exact: true }).fill('请补充来源');
    await panel.getByRole('button', { name: '要求修改', exact: true }).click();
    await panel.getByText('请补充来源', { exact: true }).waitFor();
    await panel.getByRole('button', { name: '撤回当前轮次', exact: true }).click();
    await panel.locator('.review-state').filter({ hasText: '已撤回，可编辑后重新送审' }).waitFor();
    await openBranch(page);
    await page.getByRole('button', { name: '送审当前草稿', exact: true }).click();
    await page.getByText('固定送审已完成，后续草稿可继续编辑', { exact: true }).waitFor();
    await showSubmittedReview(page);
    await panel
      .getByRole('document', { name: '固定送审正文', exact: true })
      .getByText('送审后继续写的内容', { exact: true })
      .waitFor();
    await panel.getByLabel('解除理由', { exact: true }).fill('已重新检查内容');
    await panel.getByRole('button', { name: '解除修改要求', exact: true }).click();
    await panel.getByText('已解除', { exact: true }).waitFor();
    assert.equal(await panel.getByRole('button', { name: '独立批准', exact: true }).count(), 0);
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/review-desktop.png'),
      fullPage: true,
    });
  } finally {
    try {
      await saveDiagnostics?.();
    } finally {
      await browser.close();
    }
  }
});

test('a lost real submission response retries the same operation and the same FLUSH without recapturing later edits', async () => {
  const f = await organizationFixture(0);
  f.d = await documentFixture(f);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  let saveDiagnostics;
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await context.newPage(),
      inputs = [];
    saveDiagnostics = diagnostics(page, 'review-browser-retry-current');
    page.setDefaultTimeout(8000);
    await open(page, f);
    await page.route('**/v1/documents/*/units/*/submissions', async (route) => {
      inputs.push(route.request().postDataJSON());
      const response = await route.fetch();
      assert.equal(response.status(), 202);
      if (inputs.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await editor.fill('响应丢失前固定的正文');
    await page.getByRole('button', { name: '送审当前草稿', exact: true }).click();
    await page.getByRole('button', { name: '确认操作结果', exact: true }).waitFor();
    // 202 只是登记意图。本例先由独立真实读取确认 cut 已固定，再注入 S2，不能把未固定前编辑误称为“后续草稿”。
    let fixed = false;
    for (let n = 0; n < 100; n++) {
      const progress = await get('/v1/operations/' + inputs[0].operation_id, f.owner.headers);
      assert.equal(progress.status, 200);
      if (json(progress).state === 'SUCCEEDED') {
        fixed = true;
        break;
      }
      await new Promise((done) => setTimeout(done, 100));
    }
    assert.ok(fixed, '必须实际完成原操作固定后再编辑 S2');
    await editor.fill('响应丢失之后的草稿');
    await page.getByRole('button', { name: '确认操作结果', exact: true }).click();
    await page.getByText('固定送审已完成，后续草稿可继续编辑', { exact: true }).waitFor();
    await showSubmittedReview(page);
    const panel = page.getByRole('region', { name: '审核记录', exact: true });
    await panel
      .getByRole('document', { name: '固定送审正文', exact: true })
      .getByText('响应丢失前固定的正文', { exact: true })
      .waitFor();
    assert.equal(inputs.length, 2);
    assert.deepEqual(inputs[0], inputs[1]);
    await openBranch(page);
    assert.equal(await editor.innerText(), '响应丢失之后的草稿');
  } finally {
    try {
      await saveDiagnostics?.();
    } finally {
      await browser.close();
    }
  }
});
