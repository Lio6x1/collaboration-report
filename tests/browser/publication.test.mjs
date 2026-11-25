import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { resolve } from 'node:path';
import { importMarkdown } from '../../packages/content/markdown.mjs';
import { state, root } from '../../scripts/runtime-lib.mjs';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraft,
  writeDraftBody,
} from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';

async function openWorkbench(browser, f) {
  const page = await browser.newPage({
    ignoreHTTPSErrors: true,
    viewport: { width: 1360, height: 960 },
  });
  const errors = [],
    flushes = [];
  page.on('pageerror', (error) => errors.push(error.message));
  page.on('websocket', (socket) =>
    socket.on('framesent', ({ payload }) => {
      if (typeof payload !== 'string') return;
      const message = JSON.parse(payload);
      if (message.type === 'FLUSH') flushes.push(message.operation_id);
    }),
  );
  page.setDefaultTimeout(8000);
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
  await page.locator('a[href="/documents/' + f.d.id + '"]').click();
  await page.getByRole('tab', { name: '版本与发布', exact: true }).click();
  await page.getByRole('button', { name: '发布可靠主线', exact: true }).waitFor();
  assert.equal(
    await page.getByRole('button', { name: '发布可靠主线', exact: true }).count(),
    1,
    '正式发布管理入口必须实际存在',
  );
  return { page, errors, flushes };
}

async function openMainEditor(page) {
  await page.getByRole('tab', { name: '正文', exact: true }).click();
  await page.getByRole('button', { name: '编辑文档', exact: true }).click();
  await page.getByRole('link', { name: '编辑主线', exact: true }).first().click();
  await page.getByText('服务端已保存', { exact: true }).waitFor();
  await expect(page.getByRole('textbox', { name: '主线编辑器', exact: true })).toBeEditable();
}

async function confirmPublication(page) {
  await page.getByRole('button', { name: '保存并返回文档', exact: true }).click();
  await page.getByRole('tab', { name: '版本与发布', exact: true }).click();
  await page.getByRole('button', { name: '发布可靠主线', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '确认发布主线', exact: true });
  await dialog.waitFor();
  assert.match(await dialog.innerText(), /未合入.*支线/);
  await dialog.getByRole('button', { name: '确认发布', exact: true }).click();
}

async function current(f) {
  const response = await get(`/v1/public/documents/${f.d.id}`, f.user.headers);
  assert.equal(response.status, 200, json(response).code);
  return json(response);
}

async function publicAudience(f) {
  for (const path of [`/v1/knowledge-bases/${f.f.kb}`, `/v1/documents/${f.d.id}`]) {
    const value = json(await get(path, f.user.headers));
    await waitAuthorization(
      await post(
        path + '/visibility',
        {
          operation_id: operation('browser-public-audience'),
          expected_visibility_revision: value.visibility_revision,
          policy: 'PUBLIC',
        },
        f.user.headers,
      ),
      f.user,
    );
  }
}

// 发布只捕获可靠MAIN；页面必须先固定本机输入，不能把尚未合入的实际共同支线偷带入正式版。
test('real publication UI saves MAIN, preserves independent draft and switches its own immutable history', async () => {
  const f = await reviewFixture();
  let browser;
  try {
    await writeDraft(f, '尚未合入的独立共同支线');
    await unlockStructurePreview(f);
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const { page, errors, flushes } = await openWorkbench(browser, f);
    await openMainEditor(page);
    const editor = page.getByRole('textbox', { name: '主线编辑器', exact: true });
    await editor.fill('页面正式主线第一版');
    await confirmPublication(page);
    await page.getByRole('status').filter({ hasText: '正式版已发布' }).waitFor();
    const first = await current(f);
    assert.ok(JSON.stringify(first.units).includes('页面正式主线第一版'));
    assert.ok(!JSON.stringify(first.units).includes('独立共同支线'));
    assert.equal(flushes.length, 1);
    await openMainEditor(page);
    await editor.fill('页面正式主线第二版');
    await confirmPublication(page);
    await page.getByRole('status').filter({ hasText: '正式版已发布' }).waitFor();
    const second = await current(f);
    assert.notEqual(second.release_id, first.release_id);
    const history = page.getByRole('region', { name: '正式版本历史', exact: true });
    const row = history.locator('[data-release-id="' + first.release_id + '"]');
    await row.getByRole('button', { name: '查看固定版本', exact: true }).click();
    const preview = page.getByRole('region', { name: '历史正式版阅读', exact: true });
    assert.match(await preview.innerText(), /页面正式主线第一版/);
    assert.doesNotMatch(await preview.innerText(), /第二版|独立共同支线/);
    await row.getByRole('button', { name: '设为当前正式版', exact: true }).click();
    await page
      .getByRole('dialog', { name: '确认切换正式版', exact: true })
      .getByRole('button', { name: '确认切换', exact: true })
      .click();
    await page.getByRole('status').filter({ hasText: '当前正式版已切换' }).waitFor();
    assert.equal((await current(f)).release_id, first.release_id);
    const main = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    const body = json(
      await get(
        `/v1/documents/${f.d.id}/read-views/${main.read_view_id}/units/${main.units[0].unit_id}`,
        f.user.headers,
      ),
    );
    assert.ok(JSON.stringify(body.body_json).includes('页面正式主线第二版'));
    await page.getByRole('button', { name: '撤下当前正式版', exact: true }).click();
    await page
      .getByRole('dialog', { name: '确认撤下正式版', exact: true })
      .getByRole('button', { name: '确认撤下', exact: true })
      .click();
    await page.getByRole('status').filter({ hasText: '当前正式版已撤下' }).waitFor();
    assert.equal((await get(`/v1/public/documents/${f.d.id}`, f.user.headers)).status, 404);
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/06/publication-management-browser.png'),
      fullPage: true,
    });
  } finally {
    f.close();
    await browser?.close();
  }
});

// 两种未知结果分别验证：登记丢响应只复用同号POST，进度丢响应只GET原号；二者均不再次FLUSH。
for (const loss of ['registration', 'progress'])
  test(`real publication ${loss} response loss resumes the original operation and saves only once`, async () => {
    const f = await reviewFixture();
    let browser;
    try {
      await unlockStructurePreview(f);
      browser = await chromium.launch({ channel: 'msedge', headless: true });
      const { page, errors, flushes } = await openWorkbench(browser, f);
      const inputs = [],
        queries = [];
      await page.route('**/v1/documents/*/releases', async (route) => {
        if (route.request().method() !== 'POST') return route.continue();
        inputs.push(route.request().postDataJSON());
        const response = await route.fetch();
        assert.equal(response.status(), 202);
        if (loss === 'registration' && inputs.length === 1) await route.abort('failed');
        else await route.fulfill({ response });
      });
      await page.route('**/v1/operations/*', async (route) => {
        queries.push(route.request().url().split('/').at(-1));
        const response = await route.fetch();
        if (loss === 'progress' && queries.length === 1) await route.abort('failed');
        else await route.fulfill({ response });
      });
      await openMainEditor(page);
      const editor = page.getByRole('textbox', { name: '主线编辑器', exact: true });
      await editor.fill('原号发布的主线');
      const cdp = await page.context().newCDPSession(page);
      await editor.focus();
      await cdp.send('Input.imeSetComposition', {
        text: '发布中文候选',
        selectionStart: 6,
        selectionEnd: 6,
      });
      await cdp.send('Input.insertText', { text: '发布前提交中文输入' });
      await confirmPublication(page);
      const retry = page.getByRole('button', { name: '确认操作结果', exact: true });
      await retry.waitFor();
      assert.equal((await pollPublication(inputs[0].operation_id, f.user)).state, 'SUCCEEDED');
      await retry.click();
      await page.getByRole('status').filter({ hasText: '正式版已发布' }).waitFor();
      assert.equal(inputs.length, loss === 'registration' ? 2 : 1);
      assert.ok(inputs.every((value) => JSON.stringify(value) === JSON.stringify(inputs[0])));
      assert.ok(queries.every((id) => id === inputs[0].operation_id));
      assert.equal(flushes.length, 1);
      assert.ok(JSON.stringify((await current(f)).units).includes('发布前提交中文输入'));
      assert.deepEqual(errors, []);
    } finally {
      f.close();
      await browser?.close();
    }
  });

// 匿名页面实际加载固定表格/公式/格式；旧URL只重新取当前正式版，撤下后清空正文且没有内部历史入口。
test('real anonymous page renders complete fixed rich content and clears old content after unpublish', async () => {
  const f = await reviewFixture();
  let main, browser;
  try {
    await unlockStructurePreview(f);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    let next = 0;
    const body = importMarkdown(
      '**正式加粗**与 $x^2$\n\n| 字段 | 值 |\n| --- | --- |\n| 正式 | 可靠正文 |',
      { newId: () => 'public-browser-' + ++next },
    ).body;
    await writeDraftBody(main, body);
    const first = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
    await publicAudience(f);
    main.close();
    const fresh = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    main = await openReviewWorkspace(f.f, { id: f.d.id, main: fresh }, f.user, 'MAIN');
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 960 },
    });
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    page.setDefaultTimeout(8000);
    await page.goto(
      `https://localhost:${state().ports.edge}/public/documents/${f.d.id}?release_id=${first.result.entity_id}`,
    );
    const content = page.getByRole('document', { name: '正式正文', exact: true });
    await content.waitFor();
    assert.equal(await content.getAttribute('contenteditable'), 'false');
    assert.equal(await content.locator('strong').innerText(), '正式加粗');
    assert.equal(await content.locator('table').count(), 1);
    assert.equal(await content.locator('[data-formula="x^2"] .katex').count(), 1);
    assert.equal(await page.getByRole('button', { name: '发布可靠主线', exact: true }).count(), 0);
    assert.equal(await page.getByRole('region', { name: '正式版本历史', exact: true }).count(), 0);
    assert.equal(await page.getByLabel('登录邮箱', { exact: true }).count(), 0);
    assert.equal(await page.getByRole('region', { name: '服务状态', exact: true }).count(), 0);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/06/public-reading-browser.png'),
      fullPage: true,
    });
    await writeDraft(main, '新当前正式版');
    const second = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
    await page.getByRole('button', { name: '刷新正式版', exact: true }).click();
    await content.filter({ hasText: '新当前正式版' }).waitFor();
    assert.equal(
      await page.locator('[data-current-release]').getAttribute('data-current-release'),
      second.result.entity_id,
    );
    const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const unpublish = operation('browser-unpublish');
    const response = await post(
      `/v1/documents/${f.d.id}/publication`,
      {
        operation_id: unpublish,
        expected_publication_revision: document.publication_epoch,
        action: 'UNPUBLISH',
      },
      f.user.headers,
    );
    assert.equal(response.status, 202);
    assert.equal((await pollPublication(unpublish, f.user)).state, 'SUCCEEDED');
    await page.getByRole('button', { name: '刷新正式版', exact: true }).click();
    await page.getByRole('alert').filter({ hasText: '当前没有可阅读的正式版' }).waitFor();
    assert.equal(await content.count(), 0);
    assert.deepEqual(errors, []);
  } finally {
    main?.close();
    f.close();
    await browser?.close();
  }
});
