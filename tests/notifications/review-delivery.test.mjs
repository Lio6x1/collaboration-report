import test from 'node:test';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';
import { resolve } from 'node:path';
import { mkdir } from 'node:fs/promises';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraft,
  submitDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { root, state } from '../../scripts/runtime-lib.mjs';

// 真实送审与合并贯穿同一请求：仅COMMENT不能收到审核提醒，旧提醒不能随重提漂移到新轮次。
test('审核通知按真实事件精确投递，重提保留原轮次且撤权后隐藏', { timeout: 120000 }, async () => {
  let f = await reviewFixture(2);
  const [reviewer, commenter] = f.f.members;
  let db, browser;
  try {
    db = await reviewDatabase();
    const grant = (user, capability) => ({
      user_id: user.id,
      scope_kind: 'DOCUMENT',
      scope_id: f.d.id,
      capability,
    });
    async function permission(changes) {
      const current = await get(`/v1/documents/${f.d.id}`, f.user.headers);
      assert.equal(current.status, 200, current.body);
      const result = await waitAuthorization(
        await post(
          `/v1/documents/${f.d.id}/authorization-changes`,
          {
            operation_id: operation('review-notification-permission'),
            expected_acl_revision: json(current).acl_revision,
            changes,
          },
          f.user.headers,
        ),
        f.user,
      );
      assert.equal(result.state, 'SUCCEEDED', JSON.stringify(result));
    }
    const events = [];
    async function notifications(user) {
      const response = await get('/v1/notifications', user.headers);
      assert.equal(response.status, 200, response.body);
      return json(response).items;
    }
    async function delivered(id, taskKind, kind, recipient, fixed) {
      const deadline = Date.now() + 12000;
      for (;;) {
        const [[task]] = await db.query(
          'SELECT state,attempts,last_error FROM durable_task WHERE kind=? AND operation_id=?',
          [taskKind, id],
        );
        if (task?.state === 'SUCCEEDED') break;
        assert.ok(
          task?.state !== 'REJECTED' && Date.now() < deadline,
          'REVIEW_NOTIFICATION_NOT_FINISHED ' +
            JSON.stringify({ id, taskKind, task: task ?? null }),
        );
        await new Promise((done) => setTimeout(done, 200));
      }
      const [rows] = await db.query(
        'SELECT CAST(recipient_id AS CHAR) recipient,kind,target_json FROM notification WHERE event_id=? ORDER BY recipient_id,kind',
        [id],
      );
      // 精确集合同时证明操作者排除、管理身份与显式REVIEW重叠时去重、COMMENT旁观者未获提醒。
      assert.deepEqual(
        rows.map((row) => ({ recipient: row.recipient, kind: row.kind })),
        [{ recipient: recipient.id, kind }],
      );
      const target =
        typeof rows[0].target_json === 'string'
          ? JSON.parse(rows[0].target_json)
          : rows[0].target_json;
      assert.equal(target.request_id, fixed.request_id);
      assert.equal(target.round_id, fixed.round_id);
      const received = (await notifications(recipient)).filter((item) => item.event_id === id);
      assert.equal(received.length, 1);
      assert.equal(received[0].kind, kind);
      assert.equal(received[0].target_id, fixed.request_id);
      events.push(id);
      return received[0];
    }

    await permission([
      { action: 'GRANT', grant: grant(reviewer, 'REVIEW') },
      { action: 'GRANT', grant: grant(commenter, 'COMMENT') },
    ]);
    // ACL变化真实关闭旧协作准入；之后的编辑必须重新取得当前代次工作区。
    f.close();
    f = await openReviewWorkspace(f.f, f.d);
    const originalText = '审核通知需要固定的真实送审正文';
    const resubmittedText = '重新提交的新版正文不得替换旧通知内容';
    await writeDraft(f, originalText);
    const first = await submitDraft(f);
    assert.equal(first.value.state, 'SUCCEEDED', JSON.stringify(first.value));
    const fixed = first.value.result;
    const path = `/v1/review-requests/${fixed.request_id}`;
    const firstNotice = await delivered(
      first.input.operation_id,
      'REVIEW_NOTIFICATION',
      'REVIEW_SUBMITTED',
      reviewer,
      fixed,
    );

    const change = {
      operation_id: operation('notification-request-changes'),
      round_id: fixed.round_id,
      content_version_id: fixed.content_version_id,
      type: 'REQUEST_CHANGES',
      reason: '补充来源后进行最终审核',
    };
    const changed = await post(path + '/actions', change, reviewer.headers);
    assert.equal(changed.status, 200, changed.body);
    const changeID = json(changed).result.entity_id;
    assert.ok(changeID);
    await delivered(
      change.operation_id,
      'REVIEW_ACTION_NOTIFICATION',
      'REVIEW_ACTION',
      f.user,
      fixed,
    );

    const withdraw = {
      operation_id: operation('notification-withdraw'),
      expected_round_id: fixed.round_id,
      reason: '原请求撤回后重提',
    };
    const withdrawn = await post(path + '/withdrawals', withdraw, f.user.headers);
    assert.equal(withdrawn.status, 200, withdrawn.body);
    await delivered(
      withdraw.operation_id,
      'REVIEW_ACTION_NOTIFICATION',
      'REVIEW_ACTION',
      reviewer,
      fixed,
    );

    await writeDraft(f, resubmittedText);
    const second = await submitDraft(f, {
      mode: 'RESUBMIT',
      request_id: fixed.request_id,
      expected_round_id: fixed.round_id,
    });
    assert.equal(second.value.state, 'SUCCEEDED', JSON.stringify(second.value));
    const next = second.value.result;
    assert.equal(next.request_id, fixed.request_id);
    assert.notEqual(next.round_id, fixed.round_id);
    await delivered(
      second.input.operation_id,
      'REVIEW_NOTIFICATION',
      'REVIEW_RESUBMITTED',
      reviewer,
      next,
    );
    const [[original]] = await db.query(
      "SELECT JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.round_id')) round_id FROM notification WHERE event_id=? AND recipient_id=?",
      [first.input.operation_id, reviewer.id],
    );
    assert.equal(original.round_id, fixed.round_id);

    // 同请求的新轮次已有不同正文，原通知读取仍须精确绑定第一轮，不能偷换current_round。
    const originalPath = `/v1/notifications/${firstNotice.id}/review-round`;
    const originalResponse = await get(originalPath, reviewer.headers);
    assert.equal(originalResponse.status, 200, originalResponse.body);
    const originalRound = json(originalResponse);
    assert.equal(originalRound.id, fixed.round_id);
    assert.equal(originalRound.request_id, fixed.request_id);
    assert.equal(originalRound.content_version_id, fixed.content_version_id);
    assert.equal(originalRound.content.id, fixed.content_version_id);
    const originalBody = JSON.stringify(originalRound.content.body_json);
    assert.ok(originalBody.includes(originalText), originalBody);
    assert.equal(originalBody.includes(resubmittedText), false);
    const commenterReview = await get(path, commenter.headers);
    assert.equal(commenterReview.status, 200, commenterReview.body);
    const otherNotice = await get(originalPath, commenter.headers);
    assert.equal(otherNotice.status, 404, otherNotice.body);

    // 使用真实登录与通知入口验证渲染，不注入页面状态或复用HTTP测试的Cookie。
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1280, height: 900 },
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const pageErrors = [];
    page.on('pageerror', (error) => pageErrors.push(error.message));
    await page.goto('https://localhost:' + state().ports.edge);
    await page.getByLabel('登录邮箱', { exact: true }).fill(reviewer.email);
    await page.getByLabel('登录密码', { exact: true }).fill(reviewer.password);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).waitFor();
    await page.getByRole('button', { name: '通知', exact: true }).click();
    const panel = page.getByRole('region', { name: '通知', exact: true });
    const notice = panel.getByRole('listitem').filter({ hasText: '有新的审核提交' });
    await notice.waitFor();
    assert.equal(await notice.count(), 1);
    await notice.getByRole('button', { name: '打开相关内容', exact: true }).click();
    const destination = page.getByRole('region', { name: '通知原始审核轮次', exact: true });
    const renderedBody = destination.locator('.content-view .ProseMirror');
    await renderedBody.waitFor();
    assert.equal((await renderedBody.innerText()).trim(), originalText);
    assert.equal((await renderedBody.innerText()).includes(resubmittedText), false);
    const identifiers = destination.locator('p.hint').filter({ hasText: '原审核请求' });
    assert.equal(
      (await identifiers.innerText()).replace(/\s+/g, ' ').trim(),
      `原审核请求 ${fixed.request_id} · 原轮次 ${fixed.round_id} · 固定正文版本 ${fixed.content_version_id}`,
    );
    await mkdir(resolve(root, 'artifacts/batches/06'), { recursive: true });
    await page.screenshot({
      path: resolve(root, 'artifacts/batches/06/review-notification-browser.png'),
      fullPage: true,
    });
    assert.deepEqual(pageErrors, []);
    await context.close();

    const [[pending]] = await db.query('SELECT status,revision FROM change_request WHERE id=?', [
      changeID,
    ]);
    assert.equal(pending.status, 'OPEN');
    const merge = {
      operation_id: operation('notification-merge'),
      round_id: next.round_id,
      content_version_id: next.content_version_id,
      own_changes_to_resolve: [{ id: changeID, revision: String(pending.revision) }],
    };
    const merged = await post(path + '/approve-and-merge', merge, reviewer.headers);
    assert.equal(merged.status, 202, merged.body);
    const completed = await pollMerge(merge.operation_id, reviewer.headers, db);
    assert.equal(completed.state, 'SUCCEEDED', JSON.stringify(completed));
    assert.equal(completed.result.main_sync_state, 'READY');
    await delivered(merge.operation_id, 'MERGE_NOTIFICATION', 'REVIEW_MERGED', f.user, next);

    const replay = await post(path + '/approve-and-merge', merge, reviewer.headers);
    assert.equal(replay.status, 202, replay.body);
    assert.deepEqual(json(replay).result, completed.result);
    const [[count]] = await db.query('SELECT COUNT(*) n FROM notification WHERE event_id=?', [
      merge.operation_id,
    ]);
    assert.equal(Number(count.n), 1);
    assert.equal(
      (await notifications(commenter)).some((item) => events.includes(item.event_id)),
      false,
    );

    // 撤去唯一REVIEW来源后，已持久化的旧提醒也必须重新核验可见性，不能泄露审核目标。
    await permission([{ action: 'REVOKE', grant: grant(reviewer, 'REVIEW') }]);
    assert.equal(
      (await notifications(reviewer)).some((item) => events.includes(item.event_id)),
      false,
    );
    const revoked = await get(originalPath, reviewer.headers);
    assert.equal(revoked.status, 403, revoked.body);
  } finally {
    await browser?.close();
    await db?.end();
    f.close();
  }
});
