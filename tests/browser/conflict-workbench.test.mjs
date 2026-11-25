import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
import { resolve } from 'node:path';
import { writeFileSync, mkdirSync } from 'node:fs';
import {
  conflictedReviewFixture,
  conflictBody,
  conflictParagraph,
} from '../helpers/conflict-contexts.mjs';
import { writeDraftBody } from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { pollPull } from '../helpers/pull-main.mjs';
import { existingTestAccount } from '../helpers/account.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

async function open(page, f, id, { user = f.user, editable = true } = {}) {
  page.setDefaultTimeout(10000);
  await page.goto('https://localhost:' + state().ports.edge + '/documents/' + f.d.id);
  await page.getByLabel('登录邮箱', { exact: true }).fill(user.email);
  await page.getByLabel('登录密码', { exact: true }).fill(user.password);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
  await page.getByRole('tab', { name: '审核', exact: true }).click();
  await page.getByRole('region', { name: '审核记录', exact: true }).waitFor();
  await page.getByRole('button', { name: '处理合并冲突', exact: true }).click();
  const input = page.getByLabel('原冲突操作编号', { exact: true });
  if ((await input.count()) === 0)
    writeFileSync(
      resolve(root, 'artifacts/batches/04/conflict-workbench-entry-diagnostic.txt'),
      await page.locator('body').innerText(),
    );
  await input.waitFor();
  assert.equal(await input.count(), 1, '尚无跨撤回刷新保留的人工修复工作台');
  await input.fill(id);
  await page.getByRole('button', { name: '打开固定冲突工作台', exact: true }).click();
  const workbench = page.getByRole('region', { name: '人工修复工作台', exact: true });
  if (editable) {
    await expect(
      workbench.getByRole('button', { name: '核对撤回状态', exact: true }),
    ).toBeEnabled();
    const candidate = workbench.getByRole('textbox', { name: '人工修复候选编辑器', exact: true });
    await candidate.waitFor();
    // 固定上下文先显示，随后审核状态查询结束才解除全局busy锁。
    // 等真实可编辑状态，不把DOM已出现误当候选已就绪，也不扩大原测试期限。
    await expect(candidate).toBeEditable({ timeout: 10000 });
  } else await workbench.getByRole('region', { name: '本次固定冲突', exact: true }).waitFor();
  return workbench;
}

async function acknowledge(workbench, context) {
  for (const item of context.conflicts)
    await workbench.getByLabel('已处理冲突 ' + item.id, { exact: true }).check();
  await workbench
    .getByLabel('我已核对完整修复正文，保存为新共同草稿后重新送审', { exact: true })
    .check();
}

// 候选是固定S1副本，跨明确撤回保留；修复不批准，正常重提/批准再接纳S2和未展示MAIN。
test('browser manual repair keeps its candidate across withdrawal, preserves S2 and unseen MAIN, and follows normal resubmission', async () => {
  const { f, db, main, fixed, block, id } = await conflictedReviewFixture({
    owner: await existingTestAccount(),
  });
  let browser;
  try {
    const context = json(await get('/v1/conflict-contexts/' + id, f.user.headers));
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 960 },
      }),
      writes = [],
      updates = [];
    page.on('websocket', (socket) =>
      socket.on('framesent', ({ payload }) => {
        const value = JSON.parse(String(payload));
        if (value.type === 'UPDATE') updates.push(value);
      }),
    );
    const workbench = await open(page, f, id),
      candidate = workbench.getByRole('textbox', { name: '人工修复候选编辑器', exact: true });
    await candidate.fill('人工处理固定冲突的完整正文');
    assert.equal(updates.length, 0, '纯候选不能写入共同草稿或同步到服务器');
    const save = workbench.getByRole('button', { name: '保存修复为新草稿', exact: true });
    await acknowledge(workbench, context);
    assert.equal(await save.isDisabled(), true, '审核中必须先明确撤回');
    const panel = page.getByRole('region', { name: '审核记录', exact: true });
    await panel.getByRole('button', { name: '撤回当前轮次', exact: true }).click();
    await panel.locator('.review-state').filter({ hasText: '已撤回' }).waitFor();
    assert.equal(await candidate.innerText(), '人工处理固定冲突的完整正文');
    await workbench.getByRole('button', { name: '核对撤回状态', exact: true }).click();
    await page.getByRole('button', { name: '编辑文档', exact: true }).click();
    await page.getByRole('link', { name: '编辑共同支线', exact: true }).first().click();
    const draft = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await draft.waitFor();
    await draft.press('Control+End');
    await draft.press('Enter');
    await draft.pressSequentially('工作台之外的独立S2');
    await writeDraftBody(
      main,
      conflictBody(
        conflictParagraph('workbench-unseen-main', '原工作台未展示的MAIN新增'),
        conflictParagraph(block, '原计算的可靠MAIN修改'),
      ),
    );
    await page.route('**/v1/conflict-contexts/*/resolutions', async (route) => {
      writes.push(route.request().postDataJSON());
      await route.continue();
    });
    await workbench.screenshot({
      path: resolve(root, 'artifacts/batches/04/conflict-workbench-browser.png'),
    });
    await save.click();
    await draft.waitFor({ state: 'detached' });
    assert.equal(writes.length, 1);
    assert.equal(writes[0].expected_attempt_id, context.attempt_id);
    assert.equal(writes[0].expected_input_digest, context.input_digest);
    assert.equal(writes[0].expected_round_id, fixed.round_id);
    assert.equal(writes[0].resolution_content.content[0].attrs.id, block);
    assert.deepEqual(
      writes[0].resolved_conflict_ids,
      context.conflicts.map((x) => x.id),
    );
    const request = json(await get(`/v1/review-requests/${fixed.request_id}`, f.user.headers));
    assert.equal(request.status, 'WITHDRAWN');
    assert.equal(request.current_round_id, fixed.round_id);
    const [[records]] = await db.query('SELECT COUNT(*) n FROM merge_record WHERE request_id=?', [
      fixed.request_id,
    ]);
    assert.equal(Number(records.n), 0, '人工修复本身不产生批准');
    const originalMain = page.getByRole('document', { name: '主线正文', exact: true });
    assert.match(await originalMain.innerText(), /未展示的MAIN新增/);
    assert.doesNotMatch(await originalMain.innerText(), /人工处理固定冲突/);
    await page.getByRole('button', { name: '编辑文档', exact: true }).click();
    await page.getByRole('link', { name: '编辑共同支线', exact: true }).first().click();
    await draft.waitFor();
    assert.match(await draft.innerText(), /人工处理固定冲突的完整正文/);
    assert.match(await draft.innerText(), /工作台之外的独立S2/);
    await page.getByRole('button', { name: '送审当前草稿', exact: true }).click();
    await panel.locator('.review-state').filter({ hasText: '审核中' }).waitFor();
    await panel.getByText('人工处理固定冲突的完整正文', { exact: true }).waitFor();
    await panel.getByRole('button', { name: '通过并合并', exact: true }).click();
    await panel.getByLabel('我已核对整个固定送审版本，确认通过并合并', { exact: true }).check();
    await panel.getByRole('button', { name: '确认通过并合并', exact: true }).click();
    await panel.locator('.review-state').filter({ hasText: '已合并' }).waitFor();
    assert.match(await originalMain.innerText(), /人工处理固定冲突的完整正文/);
    assert.match(await originalMain.innerText(), /工作台之外的独立S2/);
    assert.match(await originalMain.innerText(), /未展示的MAIN新增/);
    assert.deepEqual(json(await get('/v1/conflict-contexts/' + id, f.user.headers)), context);
  } finally {
    main.close();
    f.close();
    await browser?.close();
    await db.end();
  }
});

// 富文本候选保留整份PM正文和稳定身份，不能用innerText把表格、公式或中文候选压成纯文本。
test('browser Chinese manual candidate retains stable IDs, table cells, formula fields and strong formatting in the installed draft', async () => {
  const { f, db, main, fixed, block, id } = await conflictedReviewFixture({
    owner: await existingTestAccount(),
  });
  let browser;
  try {
    const withdrawn = await post(
      `/v1/review-requests/${fixed.request_id}/withdrawals`,
      { operation_id: operation('rich-workbench-withdraw'), expected_round_id: fixed.round_id },
      f.user.headers,
    );
    assert.equal(withdrawn.status, 200, json(withdrawn).code);
    const context = json(await get('/v1/conflict-contexts/' + id, f.user.headers));
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const browserContext = await browser.newContext({ ignoreHTTPSErrors: true }),
      page = await browserContext.newPage(),
      writes = [],
      errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    const inFlight = new Set();
    page.on('request', (request) => inFlight.add(new URL(request.url()).pathname));
    page.on('requestfinished', (request) => inFlight.delete(new URL(request.url()).pathname));
    const workbench = await open(page, f, id),
      candidate = workbench.getByRole('textbox', { name: '人工修复候选编辑器', exact: true });
    try {
      await candidate.fill('保留原块身份的富文本修复');
    } catch (error) {
      writeFileSync(
        resolve(root, 'artifacts/batches/04/conflict-navigation-diagnostic.txt'),
        (await page.locator('body').innerText()) + '\nIN_FLIGHT\n' + JSON.stringify([...inFlight]),
      );
      throw error;
    }
    await candidate.press('Control+a');
    await workbench.getByRole('button', { name: '粗体', exact: true }).click();
    await candidate.press('ArrowRight');
    await workbench.getByRole('button', { name: '表格工具', exact: true }).click();
    await workbench.getByRole('button', { name: '插入表格', exact: true }).click();
    await expect(
      workbench.getByRole('dialog', { name: '表格工具', exact: true }),
    ).not.toBeVisible();
    await candidate.locator('td').first().click();
    await page.keyboard.insertText('真实表格单元格');
    await candidate.press('Control+End');
    await workbench.getByRole('button', { name: '插入公式', exact: true }).click();
    await workbench.getByLabel('公式 LaTeX', { exact: true }).fill('x^2');
    await workbench.getByRole('button', { name: '插入行内公式', exact: true }).click();
    await candidate.press('Control+End');
    const cdp = await browserContext.newCDPSession(page);
    await cdp.send('Input.imeSetComposition', {
      text: '中文候选',
      selectionStart: 4,
      selectionEnd: 4,
    });
    await cdp.send('Input.insertText', { text: '立即保存的中文修复' });
    // 返回列表、再进入工作区不销毁整份富文本候选。
    await page.getByRole('button', { name: '返回审核列表', exact: true }).click();
    await expect(workbench).not.toBeVisible();
    await page.getByRole('button', { name: '处理合并冲突', exact: true }).click();
    await expect(candidate).toContainText('真实表格单元格');
    await workbench
      .getByRole('navigation', { name: '固定冲突输入' })
      .getByRole('button', { name: '原基线', exact: true })
      .click();
    await expect(
      workbench.getByRole('document', { name: '冲突原基线正文', exact: true }),
    ).toBeVisible();
    await workbench
      .getByRole('navigation', { name: '固定冲突输入' })
      .getByRole('button', { name: '原送审', exact: true })
      .click();
    mkdirSync('artifacts/frontend-navigation', { recursive: true });
    await page.screenshot({
      path: 'artifacts/frontend-navigation/conflict-desktop.png',
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await page.screenshot({
      path: 'artifacts/frontend-navigation/conflict-mobile.png',
      fullPage: true,
    });
    await page.setViewportSize({ width: 1280, height: 960 });
    await acknowledge(workbench, context);
    await page.route('**/v1/conflict-contexts/*/resolutions', async (route) => {
      writes.push(route.request().postDataJSON());
      await route.continue();
    });
    await workbench.screenshot({
      path: resolve(root, 'artifacts/batches/04/conflict-workbench-rich-browser.png'),
    });
    await workbench.getByRole('button', { name: '保存修复为新草稿', exact: true }).click();
    await candidate.waitFor({ state: 'detached' });
    assert.equal(writes.length, 1);
    const body = writes[0].resolution_content,
      nodes = [];
    const visit = (node) => {
      nodes.push(node);
      for (const child of node.content ?? []) visit(child);
    };
    visit(body);
    assert.equal(body.content[0].attrs.id, block);
    assert.ok(nodes.some((node) => node.marks?.some((mark) => mark.type === 'strong')));
    assert.ok(nodes.some((node) => node.type === 'table'));
    assert.ok(nodes.some((node) => node.type === 'text' && node.text.includes('真实表格单元格')));
    assert.ok(
      nodes.some((node) => node.type === 'text' && node.text.includes('立即保存的中文修复')),
    );
    const formula = nodes.find((node) => node.type === 'formula');
    assert.equal(formula.attrs.latex, 'x^2');
    assert.equal(formula.attrs.atomLink, null);
    await page.getByRole('button', { name: '编辑文档', exact: true }).click();
    await page.getByRole('link', { name: '编辑共同支线', exact: true }).first().click();
    const draft = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await draft.waitFor();
    assert.match(await draft.innerText(), /立即保存的中文修复/);
    assert.match(await draft.locator('td').first().innerText(), /真实表格单元格/);
    assert.equal(await draft.locator('.formula-atom').getAttribute('data-formula'), 'x^2');
    assert.deepEqual(errors, []);
  } finally {
    main.close();
    f.close();
    await browser?.close();
    await db.end();
  }
});

// 审核者能核对原固定输入，但不会因此取得EDIT或新草稿写入资格。
test('browser current REVIEW without EDIT reads fixed inputs and cannot save a manual candidate', async () => {
  const { f, db, main, id } = await conflictedReviewFixture({ members: 1 });
  let browser;
  try {
    const reviewer = f.f.members[0],
      current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await waitAuthorization(
      await post(
        `/v1/documents/${f.d.id}/authorization-changes`,
        {
          operation_id: operation('read-workbench-reviewer'),
          expected_acl_revision: current.acl_revision,
          changes: [
            {
              action: 'GRANT',
              grant: {
                user_id: reviewer.id,
                scope_kind: 'DOCUMENT',
                scope_id: f.d.id,
                capability: 'REVIEW',
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
    const workbench = await open(page, f, id, { user: reviewer, editable: false });
    await workbench
      .getByRole('document', { name: '冲突原送审正文', exact: true })
      .getByText('原计算的固定提交S1', { exact: true })
      .waitFor();
    assert.equal(
      await workbench.getByRole('textbox', { name: '人工修复候选编辑器', exact: true }).count(),
      0,
    );
    assert.equal(
      await workbench.getByRole('button', { name: '保存修复为新草稿', exact: true }).count(),
      0,
    );
    const context = json(await get('/v1/conflict-contexts/' + id, reviewer.headers));
    const denied = await post(
      `/v1/conflict-contexts/${id}/resolutions`,
      {
        operation_id: operation('reviewer-cannot-repair'),
        expected_attempt_id: context.attempt_id,
        expected_input_digest: context.input_digest,
        expected_round_id: context.round_id,
        resolution_content: context.submitted.body_json,
        resolved_conflict_ids: context.conflicts.map((item) => item.id),
      },
      reviewer.headers,
    );
    assert.equal(denied.status, 403);
    assert.equal(json(denied).code, 'UNIT_EDIT_REQUIRED');
  } finally {
    main.close();
    f.close();
    await browser?.close();
    await db.end();
  }
});

// 候选及原固定绑定一次确定，未知登记复用原号，不因结果已经安装而另写一次新草稿。
test('lost real manual repair response retries the same candidate and original context after the new epoch installs', async () => {
  const { f, db, main, fixed, block, id } = await conflictedReviewFixture({
    owner: await existingTestAccount(),
  });
  let browser;
  try {
    const withdrawn = await post(
      `/v1/review-requests/${fixed.request_id}/withdrawals`,
      { operation_id: operation('workbench-withdraw'), expected_round_id: fixed.round_id },
      f.user.headers,
    );
    assert.equal(withdrawn.status, 200, json(withdrawn).code);
    const context = json(await get('/v1/conflict-contexts/' + id, f.user.headers));
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true }),
      writes = [];
    const workbench = await open(page, f, id),
      candidate = workbench.getByRole('textbox', { name: '人工修复候选编辑器', exact: true });
    await candidate.fill('回执未知仍沿原修复候选');
    await acknowledge(workbench, context);
    await page.route('**/v1/conflict-contexts/*/resolutions', async (route) => {
      writes.push(route.request().postDataJSON());
      const response = await route.fetch();
      assert.equal(response.status(), 202);
      if (writes.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    await workbench.getByRole('button', { name: '保存修复为新草稿', exact: true }).click();
    const retry = page.getByRole('button', { name: '确认操作结果', exact: true });
    await retry.waitFor();
    assert.equal(await candidate.getAttribute('contenteditable'), 'false');
    assert.equal((await pollPull(writes[0].operation_id, f.user.headers)).state, 'SUCCEEDED');
    await retry.click();
    await candidate.waitFor({ state: 'detached' });
    assert.equal(writes.length, 2);
    assert.deepEqual(writes[0], writes[1]);
    assert.equal(writes[0].resolution_content.content[0].attrs.id, block);
    await page.getByRole('button', { name: '编辑文档', exact: true }).click();
    await page.getByRole('link', { name: '编辑共同支线', exact: true }).first().click();
    const draft = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await draft.waitFor();
    assert.equal(await draft.innerText(), '回执未知仍沿原修复候选');
    assert.deepEqual(json(await get('/v1/conflict-contexts/' + id, f.user.headers)), context);
  } finally {
    main.close();
    f.close();
    await browser?.close();
    await db.end();
  }
});
