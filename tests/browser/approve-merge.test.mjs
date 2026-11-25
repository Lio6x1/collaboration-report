import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import {
  reviewFixture,
  writeDraft,
  writeDraftBody,
  submitDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import {
  conflictedReviewFixture,
  conflictBody,
  conflictParagraph,
} from '../helpers/conflict-contexts.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

async function open(page, f, user = f.user) {
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
  const panel = page.getByRole('region', { name: '审核记录', exact: true });
  await panel.getByRole('combobox', { name: '审核轮次', exact: true }).waitFor();
  return panel;
}

async function confirm(page) {
  await page.getByRole('button', { name: '通过并合并', exact: true }).click();
  const button = page.getByRole('button', { name: '确认通过并合并', exact: true });
  assert.equal(await button.isDisabled(), true, '必须显式核对整个固定正文');
  await page.getByLabel('我已核对整个固定送审版本，确认通过并合并', { exact: true }).check();
  await button.click();
}

// 真实送审S1先冻结；页面写的S2不能被批准吸收，也不能在刷新主线时丢失。
test('browser manager merges the actual fixed S1 once and safely reopens later S2', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let browser;
  try {
    await writeDraft(f, '本次批准只接纳固定S1');
    const fixed = (await submitDraft(f)).value.result;
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 960 },
      }),
      writes = [],
      errors = [];
    page.on('pageerror', (e) => errors.push(e.message));
    const panel = await open(page, f);
    const begin = panel.getByRole('button', { name: '通过并合并', exact: true });
    assert.equal(await begin.count(), 1, '服务器有资格但审核页面缺少通过并合并入口');
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await editor.waitFor();
    await editor.fill('送审后继续工作的S2仍保留');
    await page.route('**/v1/review-requests/*/approve-and-merge', async (route) => {
      writes.push(route.request().postDataJSON());
      await route.continue();
    });
    await confirm(page);
    await panel.locator('.review-state').filter({ hasText: '已合并' }).waitFor();
    await editor.waitFor({ state: 'detached' });
    assert.equal(writes.length, 1);
    assert.equal(writes[0].round_id, fixed.round_id);
    assert.equal(writes[0].content_version_id, fixed.content_version_id);
    assert.deepEqual(writes[0].own_changes_to_resolve, []);
    await page
      .getByRole('document', { name: '主线正文', exact: true })
      .getByText('本次批准只接纳固定S1', { exact: true })
      .waitFor();
    assert.doesNotMatch(
      await page.getByRole('document', { name: '主线正文', exact: true }).innerText(),
      /继续工作的S2/,
    );
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    await editor.waitFor();
    assert.equal(await editor.innerText(), '送审后继续工作的S2仍保留');
    const request = json(await get(`/v1/review-requests/${fixed.request_id}`, f.user.headers));
    assert.equal(request.status, 'MERGED');
    const [[count]] = await db.query('SELECT COUNT(*) n FROM merge_record WHERE request_id=?', [
      fixed.request_id,
    ]);
    assert.equal(Number(count.n), 1);
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/04/approve-merge-browser.png'),
      fullPage: true,
    });
  } finally {
    f.close();
    await browser?.close();
    await db.end();
  }
});

// REVIEW不需要EDIT；拟本人解除必须完整读取实际两页并逐项明确，不能只处理第一页。
test('browser reviewer without EDIT confirms every own opinion from real pages and merges atomically', async () => {
  const f = await reviewFixture(1),
    db = await reviewDatabase();
  let browser;
  try {
    await writeDraft(f, '只有审核权也能接纳这个固定版本');
    const fixed = (await submitDraft(f)).value.result,
      reviewer = f.f.members[0];
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await waitAuthorization(
      await post(
        `/v1/documents/${f.d.id}/authorization-changes`,
        {
          operation_id: operation('browser-reviewer'),
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
    const ids = [];
    for (const reason of ['第一项本人审核要求', '第二页本人审核要求']) {
      const response = await post(
        `/v1/review-requests/${fixed.request_id}/actions`,
        {
          operation_id: operation('browser-own-opinion'),
          round_id: fixed.round_id,
          content_version_id: fixed.content_version_id,
          type: 'REQUEST_CHANGES',
          reason,
        },
        reviewer.headers,
      );
      assert.equal(response.status, 200, json(response).code);
      ids.push(json(response).result.entity_id);
    }
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true }),
      pages = [],
      writes = [];
    await page.route('**/v1/review-requests/*/change-requests?**', async (route) => {
      const url = new URL(route.request().url());
      url.searchParams.set('limit', '1');
      pages.push(url.searchParams.get('cursor'));
      await route.fulfill({ response: await route.fetch({ url: url.href }) });
    });
    const panel = await open(page, f, reviewer);
    assert.equal(await page.getByRole('button', { name: '编辑共同支线', exact: true }).count(), 0);
    assert.equal(await page.getByRole('button', { name: '撤回当前轮次', exact: true }).count(), 0);
    pages.length = 0;
    await panel.getByRole('button', { name: '通过并合并', exact: true }).click();
    const confirmation = page.getByRole('region', { name: '合并确认', exact: true });
    await confirmation.getByText('第二页本人审核要求', { exact: true }).waitFor();
    assert.equal(pages.length, 2);
    assert.equal(pages[0], null);
    assert.ok(pages[1]);
    await confirmation
      .getByLabel('我已核对整个固定送审版本，确认通过并合并', { exact: true })
      .check();
    const button = confirmation.getByRole('button', { name: '确认通过并合并', exact: true });
    assert.equal(await button.isDisabled(), true);
    await confirmation.getByLabel('一并解除本人要求 ' + ids[0], { exact: true }).check();
    assert.equal(await button.isDisabled(), true);
    await confirmation.getByLabel('一并解除本人要求 ' + ids[1], { exact: true }).check();
    assert.equal(await button.isDisabled(), false);
    await page.route('**/v1/review-requests/*/approve-and-merge', async (route) => {
      writes.push(route.request().postDataJSON());
      await route.continue();
    });
    await button.click();
    await panel.locator('.review-state').filter({ hasText: '已合并' }).waitFor();
    assert.equal(writes.length, 1);
    assert.deepEqual(
      writes[0].own_changes_to_resolve.map((x) => x.id),
      ids,
    );
    assert.ok(writes[0].own_changes_to_resolve.every((x) => x.revision === '1'));
    const [opinions] = await db.query(
      'SELECT status FROM change_request WHERE request_id=? ORDER BY id',
      [fixed.request_id],
    );
    assert.deepEqual(
      opinions.map((x) => x.status),
      ['RESOLVED', 'RESOLVED'],
    );
    const [actions] = await db.query(
      'SELECT kind,item_no FROM review_action WHERE operation_id=? ORDER BY item_no',
      [writes[0].operation_id],
    );
    assert.deepEqual(
      actions.map((x) => [x.kind, Number(x.item_no)]),
      [
        ['APPROVE_AND_MERGE', 0],
        ['RESOLVE_CHANGES', 1],
        ['RESOLVE_CHANGES', 2],
      ],
    );
  } finally {
    f.close();
    await browser?.close();
    await db.end();
  }
});

// 丢失登记回执时保存原号与固定轮次，不因重试将共同草稿重新送审或重复批准。
test('lost real merge response resumes the original fixed operation and saves new S3 before refreshing', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  let browser;
  try {
    await writeDraft(f, '丢响应的原固定S1');
    const fixed = (await submitDraft(f)).value.result;
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true }),
      writes = [],
      flushes = [];
    page.on('websocket', (socket) =>
      socket.on('framesent', ({ payload }) => {
        const value = JSON.parse(String(payload));
        if (value.type === 'FLUSH') flushes.push(value.operation_id);
      }),
    );
    const panel = await open(page, f);
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await editor.waitFor();
    await editor.fill('未知操作期间仍保留原S2');
    await page.route('**/v1/review-requests/*/approve-and-merge', async (route) => {
      writes.push(route.request().postDataJSON());
      const response = await route.fetch();
      assert.equal(response.status(), 202);
      if (writes.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    await confirm(page);
    const retry = page.getByRole('button', { name: '确认操作结果', exact: true });
    await retry.waitFor();
    assert.equal(flushes.length, 1);
    // 未知结果期间输入仍由共同支线可靠承载；成功刷新前必须保存这段更晚的S3。
    await editor.fill('未知操作期间仍保留原S2，等待期间又写入S3');
    assert.equal((await pollMerge(writes[0].operation_id, f.user.headers, db)).state, 'SUCCEEDED');
    await retry.click();
    await panel.locator('.review-state').filter({ hasText: '已合并' }).waitFor();
    assert.equal(writes.length, 2);
    assert.deepEqual(writes[0], writes[1]);
    assert.equal(writes[0].round_id, fixed.round_id);
    assert.equal(flushes.length, 2, '登记前和最终刷新前各保存一次，未知重POST不能另截一次');
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    await editor.waitFor();
    assert.equal(await editor.innerText(), '未知操作期间仍保留原S2，等待期间又写入S3');
  } finally {
    f.close();
    await browser?.close();
    await db.end();
  }
});

// 已确认登记后丢进度响应只续接原查询，不能另POST批准或重新取得首次保存边界。
test('lost real merge progress response resumes only the original query and then refreshes installed MAIN', async () => {
  const f = await reviewFixture();
  let browser;
  try {
    await writeDraft(f, '进度响应丢失仍只有本次固定S1');
    const fixed = (await submitDraft(f)).value.result;
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true }),
      writes = [],
      queries = [],
      flushes = [];
    page.on('websocket', (socket) =>
      socket.on('framesent', ({ payload }) => {
        const value = JSON.parse(String(payload));
        if (value.type === 'FLUSH') flushes.push(value.operation_id);
      }),
    );
    const panel = await open(page, f);
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await editor.waitFor();
    await editor.fill('原查询恢复后仍保留共同S2');
    await page.route('**/v1/review-requests/*/approve-and-merge', async (route) => {
      writes.push(route.request().postDataJSON());
      await route.continue();
    });
    await page.route('**/v1/merge-operations/*', async (route) => {
      queries.push(new URL(route.request().url()).pathname);
      const response = await route.fetch();
      assert.equal(response.status(), 200);
      if (queries.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    await confirm(page);
    const retry = page.getByRole('button', { name: '确认操作结果', exact: true });
    await retry.waitFor();
    assert.equal(flushes.length, 1);
    await retry.click();
    await panel.locator('.review-state').filter({ hasText: '已合并' }).waitFor();
    assert.equal(writes.length, 1);
    assert.equal(writes[0].round_id, fixed.round_id);
    assert.ok(queries.length >= 2);
    assert.ok(queries.every((path) => path === '/v1/merge-operations/' + writes[0].operation_id));
    assert.equal(flushes.length, 2);
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    await editor.waitFor();
    assert.equal(await editor.innerText(), '原查询恢复后仍保留共同S2');
  } finally {
    f.close();
    await browser?.close();
  }
});

// 拒绝后只读取这次原attempt，晚到MAIN/S2不替换原计算输入，不因GET失败另开合并。
test('browser rejected merge reads its original fixed conflict after later reliable changes and retries only that read', async () => {
  const { f, db, main, fixed, block } = await conflictedReviewFixture();
  let browser;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true }),
      writes = [],
      reads = [];
    await open(page, f);
    await page.route('**/v1/review-requests/*/approve-and-merge', async (route) => {
      writes.push(route.request().postDataJSON());
      await route.continue();
    });
    await confirm(page);
    const read = page.getByRole('button', { name: '查看本次固定冲突', exact: true });
    await read.waitFor();
    await writeDraftBody(main, conflictBody(conflictParagraph(block, '后来主线输入不替换原冲突')));
    await writeDraftBody(
      f,
      conflictBody(
        conflictParagraph(block, '原计算的固定提交S1'),
        conflictParagraph('approval-later', '后来S2不替换原冲突'),
      ),
    );
    await page.route('**/v1/conflict-contexts/*', async (route) => {
      reads.push(new URL(route.request().url()).pathname);
      const response = await route.fetch();
      assert.equal(response.status(), 200);
      if (reads.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    await read.click();
    await page.locator('.merge-control .error').waitFor();
    await read.click();
    const context = page.getByRole('region', { name: '本次固定冲突', exact: true });
    await context
      .getByRole('document', { name: '冲突原送审正文', exact: true })
      .getByText('原计算的固定提交S1', { exact: true })
      .waitFor();
    await context
      .getByRole('document', { name: '冲突原主线正文', exact: true })
      .getByText('原计算的可靠MAIN修改', { exact: true })
      .waitFor();
    assert.doesNotMatch(await context.innerText(), /后来主线输入|后来S2/);
    assert.equal(writes.length, 1);
    assert.equal(reads.length, 2);
    assert.equal(reads[0], reads[1]);
    assert.equal(reads[0], '/v1/conflict-contexts/' + writes[0].operation_id);
    assert.equal(writes[0].round_id, fixed.round_id);
    const [[approval]] = await db.query(
      "SELECT COUNT(*) n FROM review_action WHERE operation_id=? AND kind='APPROVE_AND_MERGE'",
      [writes[0].operation_id],
    );
    assert.equal(Number(approval.n), 0);
    await context.screenshot({
      path: resolve(root, 'artifacts/batches/04/approval-fixed-conflict-browser.png'),
    });
    // 原结果可直接带入独立候选；随后审核面板撤回刷新也不能销毁工作台。
    await page.getByRole('button', { name: '打开修复工作台', exact: true }).click();
    const workbench = page.getByRole('region', { name: '人工修复工作台', exact: true });
    const candidate = workbench.getByRole('textbox', { name: '人工修复候选编辑器', exact: true });
    await candidate.waitFor();
    assert.equal(
      await workbench.getByLabel('原冲突操作编号', { exact: true }).inputValue(),
      writes[0].operation_id,
    );
    assert.match(await candidate.innerText(), /原计算的固定提交S1/);
    const panel = page.getByRole('region', { name: '审核记录', exact: true });
    await panel.getByRole('button', { name: '撤回当前轮次', exact: true }).click();
    await panel.locator('.review-state').filter({ hasText: '已撤回' }).waitFor();
    assert.match(await candidate.innerText(), /原计算的固定提交S1/);
  } finally {
    main.close();
    f.close();
    await browser?.close();
    await db.end();
  }
});
