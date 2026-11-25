import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import { writeFileSync } from 'node:fs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import { fromPM } from '../../packages/content/index.mjs';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraftBody,
  submitDraft,
} from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { pollPull, pullMain } from '../helpers/pull-main.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

const paragraph = (id, text) => ({
  type: 'paragraph',
  attrs: { id },
  content: [{ type: 'text', text }],
});

async function openEditor(page, f, user = f.user) {
  page.setDefaultTimeout(10000);
  await page.goto('https://localhost:' + state().ports.edge);
  await page.getByLabel('登录邮箱', { exact: true }).fill(user.email);
  await page.getByLabel('登录密码', { exact: true }).fill(user.password);
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

function track(page) {
  const errors = [],
    flushes = [];
  page.on('pageerror', (error) => errors.push(error.message));
  page.on('websocket', (socket) =>
    socket.on('framesent', ({ payload }) => {
      const frame = JSON.parse(String(payload));
      if (frame.type === 'FLUSH') flushes.push(frame.operation_id);
    }),
  );
  return { errors, flushes };
}

// 不是仅检查按钮文字：实际拉取必须同时保留MAIN新增与共同S2，并安装新支线谱系。
test('real browser pulls reliable MAIN into a new draft epoch while preserving compatible S2', async () => {
  const f = await reviewFixture();
  let main, next, browser, page;
  const errors = [];
  try {
    const base = fromPM(yDocToProsemirrorJSON(f.doc));
    const pending = structuredClone(base);
    // S2改已有块，MAIN新增另一块。两方同一位置插入不同新块按正式协议会发生顺序冲突。
    pending.content[0].content = [{ type: 'text', text: '兼容的共同草稿S2' }];
    await writeDraftBody(f, pending);
    await unlockStructurePreview(f);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    const accepted = structuredClone(base);
    accepted.content.push(paragraph('pull-browser-main', '拉取前可靠主线新增'));
    await writeDraftBody(main, accepted);
    const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    page = await browser.newPage({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 960 },
    });
    page.on('pageerror', (error) => errors.push(error.message));
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
    await page.getByRole('button', { name: '协作报告', exact: true }).click();
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await editor.waitFor();
    assert.match(await editor.innerText(), /兼容的共同草稿S2/);
    const pull = page.getByRole('button', { name: '拉取主线到草稿', exact: true });
    assert.equal(await pull.count(), 1, '共同支线缺少已验证的显式PULL入口');
    await pull.click();
    await editor.waitFor({ state: 'detached' });
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    await editor.waitFor();
    assert.match(await editor.innerText(), /兼容的共同草稿S2/);
    assert.match(await editor.innerText(), /拉取前可靠主线新增/);
    next = await openReviewWorkspace(f.f, f.d, f.user);
    assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
    assert.notEqual(next.session.lineage_id, f.session.lineage_id);
    const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    assert.equal(after.current_manifest_id, before.current_manifest_id);
    assert.equal(after.control_revision, before.control_revision);
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/pull-main-browser.png'),
      fullPage: true,
    });
  } catch (error) {
    // 诊断不能再等待一个已经卸载的控件、覆盖原失败；只保存固定阶段和控件数量。
    if (page) {
      try {
        const controls = await Promise.allSettled(
          ['.draft-control', '[role="alert"]', '[role="textbox"]'].map((selector) =>
            page.locator(selector).count(),
          ),
        );
        writeFileSync(
          resolve(root, 'artifacts/batches/04/pull-main-browser-failure.json'),
          JSON.stringify(
            {
              errorName: error.name,
              errorCode: error.code ?? null,
              pageErrorCount: errors.length,
              controls: controls.map((value) =>
                value.status === 'fulfilled' ? value.value : null,
              ),
            },
            null,
            2,
          ),
        );
        await page.screenshot({
          path: resolve(root, 'artifacts/batches/04/pull-main-browser-failure.png'),
          fullPage: true,
        });
      } catch {
        // 即使页面已关闭或诊断文件无法写入，也保留触发本用例的原异常。
      }
    }
    throw error;
  } finally {
    next?.close();
    main?.close();
    f.close();
    await browser?.close();
  }
});

// 原POST确已登记但浏览器收不到回执；重试只认首次lineage和编号，不能再保存到后来epoch。
test('lost real PULL response retries the original operation with one IME FLUSH and reopens installed draft', async () => {
  const f = await reviewFixture();
  let browser, next;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true });
    const { errors, flushes } = track(page),
      inputs = [];
    const editor = await openEditor(page, f);
    await page.route('**/v1/documents/*/units/*/pull-main', async (route) => {
      inputs.push(route.request().postDataJSON());
      const response = await route.fetch();
      assert.equal(response.status(), 202);
      if (inputs.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    await editor.focus();
    const cdp = await page.context().newCDPSession(page);
    await cdp.send('Input.imeSetComposition', {
      text: '拉取中文候选',
      selectionStart: 6,
      selectionEnd: 6,
    });
    await cdp.send('Input.insertText', { text: '立即拉取前提交的中文输入' });
    await page.getByRole('button', { name: '拉取主线到草稿', exact: true }).click();
    const retry = page.getByRole('button', { name: '确认操作结果', exact: true });
    await retry.waitFor();
    assert.equal((await pollPull(inputs[0].operation_id, f.user.headers)).state, 'SUCCEEDED');
    await retry.click();
    await editor.waitFor({ state: 'detached' });
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    await editor.waitFor();
    assert.match(await editor.innerText(), /立即拉取前提交的中文输入/);
    assert.equal(inputs.length, 2);
    assert.deepEqual(inputs[0], inputs[1]);
    assert.equal(flushes.length, 1, '恢复原PULL不应重新FLUSH新epoch');
    next = await openReviewWorkspace(f.f, f.d);
    assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
    assert.deepEqual(errors, []);
  } finally {
    next?.close();
    f.close();
    await browser?.close();
  }
});

test('EDIT member can pull while the original review keeps its fixed round and cannot manage the document', async () => {
  const f = await reviewFixture(1),
    member = f.f.members[0];
  let browser;
  try {
    const base = fromPM(yDocToProsemirrorJSON(f.doc));
    base.content[0].content = [{ type: 'text', text: '保持原审核固定正文' }];
    await writeDraftBody(f, base);
    const round = (await submitDraft(f)).value.result;
    const before = json(
      await get(`/v1/review-requests/${round.request_id}/rounds/${round.round_id}`, f.user.headers),
    );
    // 授权变更真实关闭旧ACL连接；先固定审核，成员页面随后取得当前准入。
    await waitAuthorization(
      await post(
        `/v1/documents/${f.d.id}/authorization-changes`,
        {
          operation_id: operation('pull-member-grant'),
          expected_acl_revision: '1',
          changes: [
            {
              action: 'GRANT',
              grant: {
                user_id: member.id,
                scope_kind: 'UNIT',
                scope_id: f.d.main.units[0].unit_id,
                capability: 'EDIT',
              },
            },
          ],
        },
        f.user.headers,
      ),
      f.user,
    );
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true });
    const { errors } = track(page);
    const editor = await openEditor(page, f, member);
    assert.equal(await page.getByRole('button', { name: '解锁主线', exact: true }).count(), 0);
    assert.equal(await page.getByRole('button', { name: '生成丢弃预览', exact: true }).count(), 0);
    await editor.fill('成员已编辑的后来共同S2');
    await page.getByRole('button', { name: '拉取主线到草稿', exact: true }).click();
    await editor.waitFor({ state: 'detached' });
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    await editor.waitFor();
    assert.equal(await editor.innerText(), '成员已编辑的后来共同S2');
    assert.deepEqual(
      json(
        await get(
          `/v1/review-requests/${round.request_id}/rounds/${round.round_id}`,
          f.user.headers,
        ),
      ),
      before,
    );
    assert.deepEqual(errors, []);
  } finally {
    f.close();
    await browser?.close();
  }
});

test('a concurrently replaced lineage rejects the browser original expectation without retrying a fresh pull', async () => {
  const f = await reviewFixture();
  let browser;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true });
    const { errors, flushes } = track(page),
      inputs = [];
    const editor = await openEditor(page, f);
    await page.route('**/v1/documents/*/units/*/pull-main', async (route) => {
      inputs.push(route.request().postDataJSON());
      assert.equal((await pullMain(f)).value.state, 'SUCCEEDED');
      await route.fulfill({ response: await route.fetch() });
    });
    await page.getByRole('button', { name: '拉取主线到草稿', exact: true }).click();
    await page.locator('.draft-control .error').filter({ hasText: 'LINEAGE_CHANGED' }).waitFor();
    assert.equal(inputs.length, 1);
    assert.equal(inputs[0].expected_lineage_id, f.session.lineage_id);
    assert.equal(flushes.length, 1);
    assert.equal(await page.getByRole('button', { name: '确认操作结果', exact: true }).count(), 0);
    assert.equal(await editor.count(), 1);
    assert.deepEqual(errors, []);
  } finally {
    f.close();
    await browser?.close();
  }
});

test('a lost real PULL progress response resumes only the original query and one save boundary', async () => {
  const f = await reviewFixture();
  let browser;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true });
    const { errors, flushes } = track(page),
      inputs = [],
      queries = [];
    const editor = await openEditor(page, f);
    await page.route('**/v1/documents/*/units/*/pull-main', async (route) => {
      inputs.push(route.request().postDataJSON());
      await route.continue();
    });
    await page.route('**/v1/operations/*', async (route) => {
      queries.push(new URL(route.request().url()).pathname.split('/').at(-1));
      const response = await route.fetch();
      if (queries.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    await editor.fill('进度回执丢失仍保留原草稿');
    await page.getByRole('button', { name: '拉取主线到草稿', exact: true }).click();
    const retry = page.getByRole('button', { name: '确认操作结果', exact: true });
    await retry.waitFor();
    assert.equal((await pollPull(inputs[0].operation_id, f.user.headers)).state, 'SUCCEEDED');
    await retry.click();
    await editor.waitFor({ state: 'detached' });
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    await editor.waitFor();
    assert.equal(await editor.innerText(), '进度回执丢失仍保留原草稿');
    assert.equal(inputs.length, 1, '已登记后的进度未知不重POST');
    assert.ok(queries.length >= 2);
    assert.ok(queries.every((id) => id === inputs[0].operation_id));
    assert.equal(flushes.length, 1);
    assert.deepEqual(errors, []);
  } finally {
    f.close();
    await browser?.close();
  }
});
