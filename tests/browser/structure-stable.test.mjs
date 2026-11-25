import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import { reviewFixture, writeDraft, submitDraft } from '../helpers/review.mjs';
import { get, json } from '../helpers/workspace.mjs';
import { state, root } from '../../scripts/runtime-lib.mjs';
import { pollMainControl } from '../helpers/main-controls.mjs';

// 缺少目录入口、把改名写入旧结构或移动时重建原单元，都会破坏本例的可见结果。
test('real browser adds, renames and moves chapters while preserving the original fixed review and unit', async () => {
  const f = await reviewFixture();
  let browser;
  let page;
  const errors = [];
  const responses = [];
  try {
    await writeDraft(f, '目录页面改动前的固定正文');
    const submission = (await submitDraft(f)).value.result;
    const original = json(
      await get(
        `/v1/review-requests/${submission.request_id}/rounds/${submission.round_id}`,
        f.user.headers,
      ),
    );
    const originalUnit = f.d.main.units[0].unit_id;
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    page = await browser.newPage({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 960 },
    });
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('response', async (response) => {
      const path = new URL(response.url()).pathname;
      if (
        !path.includes('/chapters') &&
        !path.startsWith('/v1/operations/') &&
        !path.includes('/structure-changes')
      )
        return;
      const record = { path, status: response.status() };
      responses.push(record);
      try {
        const value = await response.json();
        Object.assign(record, {
          state: value.state,
          code: value.code,
          error: value.error?.code,
          sync: value.result?.main_sync_state,
        });
      } catch {}
    });
    page.setDefaultTimeout(8000);
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(f.user.email);
    await page.getByLabel('登录密码', { exact: true }).fill(f.user.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page.goto('https://localhost:' + state().ports.edge + '/documents/' + f.d.id);
    await page.getByRole('tab', { name: '文档设置', exact: true }).click();
    await page.getByRole('region', { name: '主线控制', exact: true }).waitFor();
    const panel = page.getByRole('region', { name: '目录管理', exact: true });
    await panel.waitFor();
    const chapterInputs = [];
    await page.route('**/v1/documents/*/chapters', async (route) => {
      chapterInputs.push(route.request().postDataJSON());
      const response = await route.fetch();
      if (chapterInputs.length === 1) {
        const accepted = await response.json();
        const completed = await pollMainControl(accepted.operation_id, f.user);
        assert.equal(completed.state, 'SUCCEEDED');
        await route.abort('failed');
      } else await route.fulfill({ response });
    });
    await panel.getByRole('button', { name: '新增章节', exact: true }).click();
    const addDialog = page.getByRole('dialog', { name: '新增章节', exact: true });
    await addDialog.getByLabel('新增章节标题', { exact: true }).fill('第二章：浏览器验证');
    await addDialog.getByRole('button', { name: '新增章节', exact: true }).click();
    await addDialog.getByRole('button', { name: '确认操作结果', exact: true }).click();
    await addDialog.waitFor({ state: 'hidden' });
    await panel.getByText('第二章：浏览器验证', { exact: true }).waitFor();
    const created = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    assert.equal(created.units.length, 2);
    assert.ok(chapterInputs.length >= 1);
    assert.ok(chapterInputs.every((input) => input.operation_id === chapterInputs[0].operation_id));
    assert.equal(created.units[0].unit_id, originalUnit);
    const first = created.structure.entries[0].chapter_id;
    const second = created.structure.entries[1].chapter_id;
    await page.getByRole('button', { name: '解锁主线', exact: true }).click();
    await page.getByRole('button', { name: '锁定主线', exact: true }).waitFor();
    await panel.getByRole('button', { name: '章节操作：第二章：浏览器验证', exact: true }).click();
    await panel
      .getByRole('button', { name: '重命名', exact: true })
      .filter({ visible: true })
      .click();
    const renameDialog = page.getByRole('dialog', { name: '重命名章节', exact: true });
    await renameDialog.getByLabel('章节标题', { exact: true }).fill('第二章：正式目录标题');
    await renameDialog.getByRole('button', { name: '更新章节标题', exact: true }).click();
    await panel.getByText('第二章：正式目录标题', { exact: true }).waitFor();
    await panel.getByRole('button', { name: '章节操作：第一章', exact: true }).click();
    await panel
      .getByRole('button', { name: '移动章节', exact: true })
      .filter({ visible: true })
      .click();
    const moveDialog = page.getByRole('dialog', { name: '移动章节', exact: true });
    await moveDialog.getByLabel('移动到章节之后', { exact: true }).selectOption(second);
    await moveDialog.getByRole('button', { name: '移动章节', exact: true }).click();
    await page.waitForFunction(
      () =>
        document.querySelector('.chapter-management-row strong')?.textContent ===
        '第二章：正式目录标题',
    );
    const current = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    assert.deepEqual(
      current.structure.entries.map((entry) => entry.chapter_id),
      [second, first],
    );
    assert.equal(current.structure.entries[0].title, '第二章：正式目录标题');
    assert.equal(current.structure.entries[1].unit_id, originalUnit);
    assert.notEqual(current.structure.id, original.structure.id);
    assert.deepEqual(
      json(
        await get(
          `/v1/review-requests/${submission.request_id}/rounds/${submission.round_id}`,
          f.user.headers,
        ),
      ),
      original,
    );
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/structure-stable-browser.png'),
      fullPage: true,
    });
  } catch (error) {
    if (page) {
      console.error(
        JSON.stringify({
          pageErrors: errors,
          responses,
          feedback: await page.locator('.feedback').allTextContents(),
        }),
      );
      await page.screenshot({
        path: resolve(root, 'artifacts/batches/04/structure-stable-browser-failure.png'),
        fullPage: true,
      });
    }
    throw error;
  } finally {
    f.close();
    await browser?.close();
  }
});
