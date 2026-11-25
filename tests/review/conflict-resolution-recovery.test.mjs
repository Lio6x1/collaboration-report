import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { conflictedReviewFixture } from '../helpers/conflict-contexts.mjs';
import { resolutionInput, withdrawConflict } from '../helpers/conflict-resolutions.mjs';
import { reviewDatabase, openReviewWorkspace } from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { persistedPull, pollPull } from '../helpers/pull-main.mjs';
import { request } from '../helpers/https.mjs';
import { state } from '../../scripts/runtime-lib.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

const digest = (raw) => createHash('sha256').update(raw).digest('hex');
const documentReady = () => request(state().ports.documentHttp, '/health/ready');

async function assertDocumentReady(expected = 200, reason = '') {
  const response = await documentReady();
  if (response.status !== expected) {
    const diagnostic = await reviewDatabase();
    try {
      const [pending] = await diagnostic.query(
        "SELECT kind,state,COUNT(*) count FROM durable_task WHERE state NOT IN ('SUCCEEDED','REJECTED') GROUP BY kind,state",
      );
      reason += ` pending=${JSON.stringify(pending)}`;
    } finally {
      await diagnostic.end();
    }
  }
  assert.equal(response.status, expected, `${reason} ${response.body}`);
}

test(
  'manual repair renews its original admission after natural proof expiry and owner restart',
  { timeout: 180000 },
  async () => {
    const { f, db, main, fixed, block, id } = await conflictedReviewFixture();
    const holder = await reviewDatabase();
    let stopped = false;
    let held = false;
    let reopened;
    try {
      const context = json(await get(`/v1/conflict-contexts/${id}`, f.user.headers));
      await withdrawConflict(f, fixed);
      f.close();
      main.close();
      stopOwnedRole('collaboration');
      stopped = true;
      const input = resolutionInput(context, block);
      const accepted = await post(`/v1/conflict-contexts/${id}/resolutions`, input, f.user.headers);
      assert.equal(accepted.status, 202, accepted.body);
      await holder.beginTransaction();
      held = true;
      await holder.query(
        "SELECT id FROM durable_task WHERE kind='RESOLUTION_RECOVER' AND business_key=? FOR UPDATE",
        [input.operation_id],
      );
      const [[original]] = await holder.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='RESOLUTION_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      const binding = persistedPull(original.raw);
      await assertDocumentReady();
      // 短证明真实自然失效；持久原上下文不能为了续接而改写原证明、来源或会话。
      await new Promise((resolve) => setTimeout(resolve, 31000));
      assert.ok(binding.Access.Proofs.every((proof) => Number(proof.expires_unix_ms) < Date.now()));
      assert.equal(
        json(await get(`/v1/operations/${input.operation_id}`, f.user.headers)).state,
        'PREPARING',
      );
      await holder.rollback();
      held = false;
      restartOwnedRuntime();
      stopped = false;
      const result = await pollPull(input.operation_id, f.user.headers);
      assert.equal(result.state, 'SUCCEEDED', JSON.stringify(result));
      const [[same]] = await db.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='RESOLUTION_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      assert.equal(digest(same.raw), digest(original.raw));
      reopened = await openReviewWorkspace(f.f, f.d, f.user);
      assert.equal(
        BigInt(reopened.session.workspace_epoch),
        BigInt(f.session.workspace_epoch) + 1n,
      );
      assert.deepEqual(json(await get(`/v1/conflict-contexts/${id}`, f.user.headers)), context);
      const current = json(await get(`/v1/review-requests/${fixed.request_id}`, f.user.headers));
      assert.equal(current.status, 'WITHDRAWN');
      assert.equal(current.current_round_id, fixed.round_id);
    } finally {
      if (held) await holder.rollback();
      reopened?.close();
      main.close();
      f.close();
      await holder.end();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);

test(
  'manual repair keeps WAITING until real installation and completes original responsibility after logout',
  { timeout: 180000 },
  async () => {
    const { f, db, main, fixed, block, id } = await conflictedReviewFixture();
    const holder = await reviewDatabase('collaboration');
    let held = false;
    let stopped = false;
    try {
      const context = json(await get(`/v1/conflict-contexts/${id}`, f.user.headers));
      await withdrawConflict(f, fixed);
      const input = resolutionInput(context, block);
      // 锁实际安装回执唯一键，让Go提交成功事实后停在真实Node安装边界。
      await holder.beginTransaction();
      held = true;
      await holder.query(
        "SELECT id FROM durable_task WHERE kind='DRAFT_TRANSITION_INSTALLED' AND business_key=? FOR UPDATE",
        [input.operation_id],
      );
      const accepted = await post(`/v1/conflict-contexts/${id}/resolutions`, input, f.user.headers);
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
      assert.equal(fact.state, 'SUCCEEDED');
      assert.equal(
        json(await get(`/v1/operations/${input.operation_id}`, f.user.headers)).state,
        'WAITING',
      );
      await assert.rejects(() => openReviewWorkspace(f.f, f.d, f.user), /EDIT_SESSION_FAILED/);
      const [installation] = await db.query(
        "SELECT kind,CAST(payload AS CHAR) raw FROM durable_task WHERE operation_id=? AND kind IN ('RESOLUTION_COMMITTED','RESOLUTION_RECONCILE')",
        [input.operation_id],
      );
      assert.equal(installation.length, 2);
      await assertDocumentReady();
      try {
        // 两份恢复payload一起换成另一真实历史版本也不能绕过不可变版本与原固定候选的核对。
        await db.query(
          "UPDATE durable_task SET payload=JSON_SET(payload,'$.Version',CAST(? AS UNSIGNED)) WHERE operation_id=? AND kind IN ('RESOLUTION_COMMITTED','RESOLUTION_RECONCILE')",
          [fixed.content_version_id, input.operation_id],
        );
        await assertDocumentReady(503);
      } finally {
        for (const row of installation)
          await db.query('UPDATE durable_task SET payload=? WHERE kind=? AND business_key=?', [
            row.raw,
            row.kind,
            input.operation_id,
          ]);
      }
      await assertDocumentReady();
      try {
        // 权威结果中的版本也必须与实际安装来源对应，不能返回一份已成功但指向旧送审稿的结果。
        await db.query(
          "UPDATE operation_receipt SET result_json=JSON_SET(result_json,'$.result.content_version_id',?) WHERE operation_id=?",
          [fixed.content_version_id, input.operation_id],
        );
        await assertDocumentReady(503, 'fixed result must match the actual installation version');
      } finally {
        await db.query('UPDATE operation_receipt SET result_json=? WHERE operation_id=?', [
          fact.raw,
          input.operation_id,
        ]);
      }
      await assertDocumentReady();
      f.close();
      main.close();
      stopOwnedRole('collaboration');
      stopped = true;
      await holder.rollback();
      held = false;
      const logout = await post(
        '/v1/account/logout',
        { operation_id: operation('resolution-author-logout') },
        f.user.headers,
      );
      assert.ok([200, 202].includes(logout.status), logout.body);
      restartOwnedRuntime();
      stopped = false;
      let installed = false;
      const settle = Date.now() + 20000;
      do {
        const [[row]] = await db.query(
          "SELECT COUNT(*) count FROM durable_task WHERE kind='RESOLUTION_INSTALLED' AND business_key=? AND state='SUCCEEDED'",
          [input.operation_id],
        );
        installed = Number(row.count) === 1;
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
      const [[branch]] = await db.query(
        "SELECT lifecycle,JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.operation_id')) owner FROM branch b JOIN durable_task t ON t.kind='BRANCH_CONTROL_GATE' AND t.business_key=CAST(b.id AS CHAR) WHERE b.id=?",
        [f.session.branch_id],
      );
      assert.equal(branch.lifecycle, 'ACTIVE');
      assert.equal(branch.owner, '');
      const [[review]] = await db.query(
        'SELECT status,CAST(current_round_id AS CHAR) round_id FROM review_request WHERE id=?',
        [fixed.request_id],
      );
      assert.equal(review.status, 'WITHDRAWN');
      assert.equal(review.round_id, fixed.round_id);
    } finally {
      if (held) await holder.rollback();
      main.close();
      f.close();
      await holder.end();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
