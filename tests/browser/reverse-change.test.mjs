import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import { mergedReverseFixture, pollReverse } from '../helpers/reverse-changes.mjs';
import { writeDraftBody } from '../helpers/review.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

async function open(page, f, user = f.user) {
  page.setDefaultTimeout(12000);
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

async function inspect(panel) {
  const begin = panel.getByRole('button', { name: '反向创建新审核', exact: true });
  assert.equal(await begin.count(), 1, '已合并管理者缺少反向创建新审核入口');
  await begin.click();
  const confirmation = panel.getByRole('region', { name: '反向修改确认', exact: true });
  await confirmation.waitFor();
  await confirmation
    .getByLabel('反向修改理由', { exact: true })
    .fill('保留历史，重新审核这项反向修改');
  const save = confirmation.getByRole('button', { name: '确认创建反向审核', exact: true });
  assert.equal(await save.isDisabled(), true);
  await confirmation.getByLabel('我确认创建新审核，再次批准后才更新主线', { exact: true }).check();
  return { confirmation, save };
}

async function history({ f, fixed }) {
  // 真实动作页按ID正序；批准位于后页，不能从第一页猜不存在合并记录。
  const made = await post(
    `/v1/review-requests/${fixed.request_id}/actions`,
    {
      operation_id: operation('reverse-ui-old-opinion'),
      round_id: fixed.round_id,
      content_version_id: fixed.content_version_id,
      type: 'REQUEST_CHANGES',
      reason: '真实历史意见',
    },
    f.user.headers,
  );
  assert.equal(made.status, 200, made.body);
  const id = json(made).result.entity_id;
  const resolved = await post(
    `/v1/review-requests/${fixed.request_id}/change-requests/${id}/resolve`,
    {
      operation_id: operation('reverse-ui-old-resolution'),
      expected_round_id: fixed.round_id,
      expected_change_revision: '1',
      reason: '核对后明确解除',
    },
    f.user.headers,
  );
  assert.equal(resolved.status, 200, resolved.body);
}

test('browser reverse reads the complete real merge history, preserves S2 and unseen MAIN and requires ordinary approval', async () => {
  const { f, db, fixed, merged, record, originalBlockId } = await mergedReverseFixture({
    mainTail: true,
    beforeMerge: history,
  });
  let browser;
  try {
    await writeDraftBody(f, {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'reverse-ui-s2' },
          content: [{ type: 'text', text: '新反向审核保留独立S2' }],
        },
        {
          type: 'paragraph',
          attrs: { id: originalBlockId },
          content: [{ type: 'text', text: '这项已经合并的修改需要反向' }],
        },
      ],
    });
    const control = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const reading = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
    assert.notEqual(reading.manifest_id, control.current_manifest_id);
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 960 },
      }),
      pages = [],
      writes = [],
      errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await page.route('**/v1/review-requests/*/actions?**', async (route) => {
      const url = new URL(route.request().url());
      url.searchParams.set('limit', '1');
      pages.push(url.searchParams.get('cursor'));
      await route.fulfill({ response: await route.fetch({ url: url.href }) });
    });
    await page.route('**/v1/documents/*/review-requests?**', async (route) => {
      // 实际后页的新请求应由原结果精确读取；不扩大首页或把旧已合并请求当作新审核。
      const url = new URL(route.request().url());
      url.searchParams.set('limit', '1');
      await route.fulfill({ response: await route.fetch({ url: url.href }) });
    });
    const panel = await open(page, f);
    await page.getByRole('button', { name: '编辑共同支线', exact: true }).click();
    const editor = page.getByRole('textbox', { name: '共同支线编辑器', exact: true });
    await editor.waitFor();
    assert.match(await editor.innerText(), /独立S2/);
    const { confirmation, save } = await inspect(panel);
    assert.ok(pages.length >= 3 && pages[0] === null && pages[1], '批准必须来自真实后页');
    assert.match(await confirmation.innerText(), new RegExp(String(record.id)));
    await page.route('**/v1/documents/*/reverse-changes', async (route) => {
      writes.push(route.request().postDataJSON());
      await route.continue();
    });
    await confirmation.screenshot({
      path: resolve(root, 'artifacts/batches/04/reverse-change-browser.png'),
    });
    await save.click();
    await editor.waitFor({ state: 'detached' });
    try {
      await panel.locator('.review-state').filter({ hasText: '审核中' }).waitFor();
    } catch (error) {
      console.error(
        JSON.stringify({
          panel: (await panel.count()) ? await panel.innerText() : null,
          alerts: await page.getByRole('alert').allTextContents(),
          writes,
          actual: writes[0]
            ? json(await get('/v1/operations/' + writes[0].operation_id, f.user.headers))
            : null,
        }),
      );
      await page.screenshot({
        path: resolve(root, 'artifacts/batches/04/reverse-change-browser-failure.png'),
        fullPage: true,
      });
      throw error;
    }
    assert.equal(writes.length, 1);
    assert.equal(writes[0].merge_record_id, String(record.id));
    assert.equal(writes[0].expected_manifest_id, control.current_manifest_id);
    const result = await pollReverse(writes[0].operation_id, f.user.headers);
    assert.equal(result.state, 'SUCCEEDED');
    assert.notEqual(result.result.request_id, fixed.request_id);
    assert.equal(
      await panel.getByRole('combobox', { name: '审核请求', exact: true }).inputValue(),
      result.result.request_id,
    );
    assert.match(await panel.innerText(), /独立S2/);
    assert.match(await panel.innerText(), /可靠保存但未物化的MAIN新增/);
    assert.doesNotMatch(
      await panel.getByRole('document', { name: '固定送审正文', exact: true }).innerText(),
      /已经合并的修改需要反向/,
    );
    const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    assert.equal(
      after.current_manifest_id,
      merged.result.manifest_id,
      '创建反向审核不能立即改变MAIN',
    );
    assert.equal(
      json(await get(`/v1/review-requests/${fixed.request_id}`, f.user.headers)).status,
      'MERGED',
    );
    const [[old]] = await db.query(
      "SELECT CAST(JSON_OBJECT('operation',operation_id,'round',round_id,'input',input_manifest_id,'output',output_manifest_id,'version',result_version_id,'capture',accepted_capture_ref,'lineage',lineage_id) AS CHAR) raw FROM merge_record WHERE id=?",
      [record.id],
    );
    assert.equal(old.raw, record.raw);
    await panel.getByRole('button', { name: '通过并合并', exact: true }).click();
    await panel.getByLabel('我已核对整个固定送审版本，确认通过并合并', { exact: true }).check();
    await panel.getByRole('button', { name: '确认通过并合并', exact: true }).click();
    await panel.locator('.review-state').filter({ hasText: '已合并' }).waitFor();
    const main = page.getByRole('document', { name: '主线正文', exact: true });
    assert.match(await main.innerText(), /独立S2/);
    assert.match(await main.innerText(), /可靠保存但未物化的MAIN新增/);
    assert.doesNotMatch(await main.innerText(), /已经合并的修改需要反向/);
    assert.deepEqual(errors, []);
  } finally {
    f.close();
    await browser?.close();
    await db.end();
  }
});

test('lost reverse registration resumes the same record, manifest, reason and single reliable save', async () => {
  const { f, db, record, merged } = await mergedReverseFixture();
  let browser;
  try {
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
    await editor.press('Control+End');
    await editor.press('Enter');
    await editor.pressSequentially('反向丢响应也保留本机S2');
    const { save } = await inspect(panel);
    await page.route('**/v1/documents/*/reverse-changes', async (route) => {
      writes.push(route.request().postDataJSON());
      const response = await route.fetch();
      assert.equal(response.status(), 202);
      if (writes.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    await save.click();
    const retry = page.getByRole('button', { name: '确认操作结果', exact: true });
    await retry.waitFor();
    assert.equal(flushes.length, 1);
    assert.equal((await pollReverse(writes[0].operation_id, f.user.headers)).state, 'SUCCEEDED');
    await retry.click();
    await editor.waitFor({ state: 'detached' });
    await panel.locator('.review-state').filter({ hasText: '审核中' }).waitFor();
    assert.equal(writes.length, 2);
    assert.deepEqual(writes[0], writes[1]);
    assert.equal(flushes.length, 1);
    assert.equal(writes[0].merge_record_id, String(record.id));
    assert.equal(writes[0].expected_manifest_id, merged.result.manifest_id);
    assert.match(await panel.innerText(), /丢响应也保留本机S2/);
  } finally {
    f.close();
    await browser?.close();
    await db.end();
  }
});

test('review-only reader sees the merged record without a reverse-management entry', async () => {
  const { f, db, record, merged } = await mergedReverseFixture({ members: 1 });
  let browser;
  try {
    const reviewer = f.f.members[0],
      current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await waitAuthorization(
      await post(
        `/v1/documents/${f.d.id}/authorization-changes`,
        {
          operation_id: operation('reverse-ui-review-grant'),
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
    const page = await browser.newPage({ ignoreHTTPSErrors: true }),
      panel = await open(page, f, reviewer);
    await panel.locator('.review-state').filter({ hasText: '已合并' }).waitFor();
    assert.equal(
      await panel.getByRole('button', { name: '反向创建新审核', exact: true }).count(),
      0,
    );
    const denied = await post(
      `/v1/documents/${f.d.id}/reverse-changes`,
      {
        operation_id: operation('reverse-ui-review-denied'),
        expected_manifest_id: merged.result.manifest_id,
        merge_record_id: String(record.id),
        reason: '不能用审核资格反向',
      },
      reviewer.headers,
    );
    assert.equal(denied.status, 403, denied.body);
    assert.equal(json(denied).code, 'DRAFT_MANAGE_REQUIRED');
  } finally {
    f.close();
    await browser?.close();
    await db.end();
  }
});

test('lost reverse progress queries only the original operation without another POST or FLUSH', async () => {
  const { f, db } = await mergedReverseFixture();
  let browser;
  try {
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
    await page.route('**/v1/documents/*/reverse-changes', async (route) => {
      writes.push(route.request().postDataJSON());
      await route.continue();
    });
    await page.route('**/v1/operations/*', async (route) => {
      queries.push(new URL(route.request().url()).pathname);
      const response = await route.fetch();
      assert.equal(response.status(), 200);
      if (queries.length === 1) await route.abort('failed');
      else await route.fulfill({ response });
    });
    const { save } = await inspect(panel);
    await save.click();
    const retry = page.getByRole('button', { name: '确认操作结果', exact: true });
    await retry.waitFor();
    assert.equal(flushes.length, 1);
    await retry.click();
    await editor.waitFor({ state: 'detached' });
    await panel.locator('.review-state').filter({ hasText: '审核中' }).waitFor();
    assert.equal(writes.length, 1);
    assert.equal(flushes.length, 1);
    assert.ok(
      queries.length >= 2 &&
        queries.every((path) => path === '/v1/operations/' + writes[0].operation_id),
    );
  } finally {
    f.close();
    await browser?.close();
    await db.end();
  }
});

test('a changed business MAIN after reverse confirmation rejects the original expectation without silently refreshing it', async () => {
  const { f, db, merged } = await mergedReverseFixture({ mainTail: true });
  let browser;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const page = await browser.newPage({ ignoreHTTPSErrors: true }),
      writes = [];
    const panel = await open(page, f),
      { save } = await inspect(panel);
    const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/chapters`,
        {
          operation_id: operation('reverse-ui-concurrent-chapter'),
          expected_structure_id: document.current_structure_id,
          title: '确认之后其他管理者新增的章节',
        },
        f.user.headers,
      ),
      f.user,
    );
    assert.notEqual(
      json(await get(`/v1/documents/${f.d.id}`, f.user.headers)).current_manifest_id,
      merged.result.manifest_id,
    );
    await page.route('**/v1/documents/*/reverse-changes', async (route) => {
      writes.push(route.request().postDataJSON());
      await route.continue();
    });
    await save.click();
    await panel.getByRole('alert').filter({ hasText: 'MAIN_CONTEXT_CHANGED' }).waitFor();
    assert.equal(writes.length, 1);
    assert.equal(writes[0].expected_manifest_id, merged.result.manifest_id);
    const [[registered]] = await db.query(
      'SELECT COUNT(*) n FROM operation_receipt WHERE operation_id=?',
      [writes[0].operation_id],
    );
    assert.equal(Number(registered.n), 0, '已知旧期待应在登记前拒绝');
    const [[requests]] = await db.query(
      'SELECT COUNT(*) n FROM review_request WHERE document_id=?',
      [f.d.id],
    );
    assert.equal(Number(requests.n), 1, '不能自动换新清单重发反向操作');
  } finally {
    f.close();
    await browser?.close();
    await db.end();
  }
});
