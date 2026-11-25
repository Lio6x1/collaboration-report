import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import {
  reviewFixture,
  reviewDatabase,
  writeDraft,
  submitDraft,
  openReviewWorkspace,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import {
  discardPreviewPath,
  discardPreviewInput,
  discardDraftPath,
  discardDraftInput,
  pollDiscardDraft,
} from '../helpers/discard-previews.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

const digest = (raw) => createHash('sha256').update(raw).digest('hex');
const valueOf = (value) => (typeof value === 'string' ? JSON.parse(value) : value);

test(
  'discard resumes original confirmation after natural proof expiry and owner restart',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase();
    let stopped = false,
      next;
    try {
      await writeDraft(f, '原会话自然过期边界前的旧稿');
      const response = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
      assert.equal(response.status, 200, response.body);
      const preview = json(response),
        input = discardDraftInput(preview, operation('discard-expired-original'));
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      const pending = await post(discardDraftPath(preview), input, f.user.headers);
      assert.equal(pending.status, 202, pending.body);
      const [[original]] = await db.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='DISCARD_DRAFT_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      await new Promise((resolve) => setTimeout(resolve, 31000));
      const fixed = JSON.parse(original.raw);
      assert.ok(fixed.Access.Proofs.every((proof) => Number(proof.expires_unix_ms) < Date.now()));
      assert.equal(
        json(await get(`/v1/operations/${input.operation_id}`, f.user.headers)).state,
        'PREPARING',
      );
      restartOwnedRuntime();
      stopped = false;
      assert.equal((await pollDiscardDraft(input.operation_id, f.user.headers)).state, 'SUCCEEDED');
      const [[same]] = await db.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='DISCARD_DRAFT_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      assert.equal(digest(same.raw), digest(original.raw));
      next = await openReviewWorkspace(f.f, f.d);
      assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
    } finally {
      next?.close();
      f.close();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);

test(
  'Go success retains opinions and resumable until actual Node install, then system archives after logout',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase(),
      node = await reviewDatabase('collaboration');
    let locked = false,
      stopped = false;
    try {
      await writeDraft(f, '安装之前必须保留的撤回工作');
      const submitted = (await submitDraft(f)).value;
      assert.equal(submitted.state, 'SUCCEEDED');
      const review = submitted.result,
        path = `/v1/review-requests/${review.request_id}`;
      assert.equal(
        (
          await post(
            `${path}/actions`,
            {
              operation_id: operation('install-block-change'),
              round_id: review.round_id,
              content_version_id: review.content_version_id,
              type: 'REQUEST_CHANGES',
              reason: '此意见必须待真实恢复后处理',
            },
            f.user.headers,
          )
        ).status,
        200,
      );
      assert.equal(
        (
          await post(
            `${path}/withdrawals`,
            {
              operation_id: operation('install-block-withdraw'),
              expected_round_id: review.round_id,
              reason: '明确结束旧工作',
            },
            f.user.headers,
          )
        ).status,
        200,
      );
      const previewResponse = await post(
        discardPreviewPath(f),
        discardPreviewInput(f),
        f.user.headers,
      );
      assert.equal(previewResponse.status, 200, previewResponse.body);
      const preview = json(previewResponse),
        input = {
          ...discardDraftInput(preview, operation('discard-install-recovery')),
          expected_round_id: review.round_id,
        };
      // 对Node安装回执的真实唯一键加锁；不改任务状态、不伪造成功、不阻止Go私库提交。
      await node.beginTransaction();
      locked = true;
      await node.query(
        "SELECT id FROM durable_task WHERE kind='DRAFT_TRANSITION_INSTALLED' AND business_key=? FOR UPDATE",
        [input.operation_id],
      );
      const accepted = await post(discardDraftPath(preview), input, f.user.headers);
      assert.equal(accepted.status, 202, accepted.body);
      let fact;
      const deadline = Date.now() + 20000;
      do {
        [[fact]] = await db.query(
          'SELECT state,CAST(result_json AS CHAR) raw FROM operation_receipt WHERE operation_id=?',
          [input.operation_id],
        );
        if (fact.state === 'SUCCEEDED') break;
        await new Promise((resolve) => setTimeout(resolve, 100));
      } while (Date.now() < deadline);
      assert.equal(fact.state, 'SUCCEEDED', JSON.stringify({ state: fact.state }));
      assert.equal(
        json(await get(`/v1/operations/${input.operation_id}`, f.user.headers)).state,
        'WAITING',
      );
      const [[request]] = await db.query(
        'SELECT status,archived_at FROM review_request WHERE id=?',
        [review.request_id],
      );
      assert.equal(request.status, 'WITHDRAWN');
      assert.equal(request.archived_at, null);
      const [[guard]] = await db.query(
        'SELECT resumable_request_id FROM unit_review_guard WHERE unit_id=?',
        [preview.unit_id],
      );
      assert.equal(String(guard.resumable_request_id), review.request_id);
      const [[change]] = await db.query('SELECT status FROM change_request WHERE id=?', [
        preview.blocking_changes[0].id,
      ]);
      assert.equal(change.status, 'OPEN');
      const [[branch]] = await db.query('SELECT lifecycle FROM branch WHERE id=?', [
        preview.branch_id,
      ]);
      assert.equal(branch.lifecycle, 'RESTORING');
      await assert.rejects(() => openReviewWorkspace(f.f, f.d), /EDIT_SESSION_FAILED/);
      const [[gate]] = await node.query(
        "SELECT payload FROM durable_task WHERE kind='DRAFT_MAIN_GATE' AND business_key=?",
        [f.d.id],
      );
      assert.equal(valueOf(gate.payload).state, 'PREPARED');
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      await node.rollback();
      locked = false;
      const logout = await post(
        '/v1/account/logout',
        { operation_id: operation('successful-discard-logout') },
        f.user.headers,
      );
      assert.ok([200, 202].includes(logout.status), logout.body);
      restartOwnedRuntime();
      stopped = false;
      let installed = false;
      const settle = Date.now() + 20000;
      do {
        const [[row]] = await db.query(
          "SELECT COUNT(*) total FROM durable_task WHERE kind='DISCARD_DRAFT_INSTALLED' AND business_key=? AND state='SUCCEEDED'",
          [input.operation_id],
        );
        installed = Number(row.total) === 1;
        if (installed) break;
        await new Promise((resolve) => setTimeout(resolve, 100));
      } while (Date.now() < settle);
      assert.equal(installed, true);
      const [[after]] = await db.query(
        'SELECT state,CAST(result_json AS CHAR) raw FROM operation_receipt WHERE operation_id=?',
        [input.operation_id],
      );
      assert.equal(after.state, 'SUCCEEDED');
      assert.equal(digest(after.raw), digest(fact.raw));
      const [[archived]] = await db.query(
        'SELECT status,archived_at FROM review_request WHERE id=?',
        [review.request_id],
      );
      assert.equal(archived.status, 'WITHDRAWN');
      assert.ok(archived.archived_at);
      const [[resolved]] = await db.query('SELECT status FROM change_request WHERE id=?', [
        preview.blocking_changes[0].id,
      ]);
      assert.equal(resolved.status, 'RESOLVED');
      const [[released]] = await node.query(
        "SELECT payload FROM durable_task WHERE kind='DRAFT_MAIN_GATE' AND business_key=?",
        [f.d.id],
      );
      assert.equal(valueOf(released.payload).state, 'INSTALLED');
    } finally {
      if (locked) await node.rollback();
      f.close();
      await node.end();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
