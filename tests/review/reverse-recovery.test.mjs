import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { reviewDatabase, openReviewWorkspace } from '../helpers/review.mjs';
import { mergedReverseFixture, pollReverse } from '../helpers/reverse-changes.mjs';
import { post, get, json, operation } from '../helpers/workspace.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import { request } from '../helpers/https.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

const digest = (raw) => createHash('sha256').update(raw).digest('hex');
const valueOf = (value) => (typeof value === 'string' ? JSON.parse(value) : value);
const reverseInput = (merged, record, id) => ({
  operation_id: id,
  expected_manifest_id: merged.result.manifest_id,
  merge_record_id: String(record.id),
  reason: '以原服务器来源恢复新审核',
});

test(
  'reverse verifies its real admission, then resumes the original server source after natural proof expiry and owner restart',
  { timeout: 180000 },
  async () => {
    const { f, db, merged, record } = await mergedReverseFixture(),
      holder = await reviewDatabase(),
      editor = await reviewDatabase();
    let stopped = false,
      held = false,
      admission,
      next;
    const ready = () => request(state().ports.documentHttp, '/health/ready');
    try {
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      const id = operation('reverse-original-expiry'),
        input = reverseInput(merged, record, id);
      const accepted = await post(`/v1/documents/${f.d.id}/reverse-changes`, input, f.user.headers);
      assert.equal(accepted.status, 202, accepted.body);
      await holder.beginTransaction();
      held = true;
      await holder.query(
        "SELECT id FROM durable_task WHERE kind='REVERSE_RECOVER' AND business_key=? FOR UPDATE",
        [id],
      );
      const [[original]] = await holder.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='REVERSE_CONTEXT' AND business_key=?",
        [id],
      );
      const fixed = JSON.parse(original.raw),
        admissionId = fixed.Context.document_admission_ref;
      const [[binding]] = await editor.query(
        'SELECT actor_id,unit_id,CAST(target_json AS CHAR) target FROM document_admission WHERE admission_id=?',
        [admissionId],
      );
      admission = { id: admissionId, binding };
      assert.equal((await ready()).status, 200);
      for (const alteration of [
        'UPDATE document_admission SET actor_id=actor_id+1 WHERE admission_id=?',
        'UPDATE document_admission SET unit_id=NULL WHERE admission_id=?',
        "UPDATE document_admission SET target_json=JSON_SET(target_json,'$.purpose','SUBMISSION') WHERE admission_id=?",
      ]) {
        await editor.query(alteration, [admissionId]);
        assert.equal(
          (await ready()).status,
          503,
          'substituted server-source admission must fail readiness',
        );
        await editor.query(
          'UPDATE document_admission SET actor_id=?,unit_id=?,target_json=? WHERE admission_id=?',
          [binding.actor_id, binding.unit_id, binding.target, admissionId],
        );
        assert.equal((await ready()).status, 200);
      }
      // 等真实短证明自然失效，不回写过期时间、不伪造SUBMISSION或FLUSH。
      await new Promise((resolve) => setTimeout(resolve, 31000));
      assert.ok(fixed.Access.Proofs.every((proof) => Number(proof.expires_unix_ms) < Date.now()));
      assert.equal(json(await get(`/v1/operations/${id}`, f.user.headers)).state, 'PREPARING');
      await holder.rollback();
      held = false;
      restartOwnedRuntime();
      stopped = false;
      const result = await pollReverse(id, f.user.headers);
      assert.equal(result.state, 'SUCCEEDED', JSON.stringify(result));
      const [[same]] = await db.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='REVERSE_CONTEXT' AND business_key=?",
        [id],
      );
      assert.equal(digest(same.raw), digest(original.raw));
      next = await openReviewWorkspace(f.f, f.d);
      assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
      assert.equal(
        json(await get(`/v1/documents/${f.d.id}`, f.user.headers)).current_manifest_id,
        merged.result.manifest_id,
      );
    } finally {
      if (admission)
        await editor.query(
          'UPDATE document_admission SET actor_id=?,unit_id=?,target_json=? WHERE admission_id=?',
          [
            admission.binding.actor_id,
            admission.binding.unit_id,
            admission.binding.target,
            admission.id,
          ],
        );
      if (held) await holder.rollback();
      next?.close();
      f.close();
      await holder.end();
      await editor.end();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);

test(
  'reverse success remains WAITING until real Node installation and completes the same review after logout',
  { timeout: 180000 },
  async () => {
    const { f, db, fixed: originalReview, merged, record } = await mergedReverseFixture(),
      holder = await reviewDatabase('collaboration'),
      editor = await reviewDatabase('collaboration');
    let held = false,
      stopped = false,
      gate;
    try {
      const id = operation('reverse-install-logout'),
        input = reverseInput(merged, record, id);
      // 锁真实安装回执唯一键，仅阻挡Node安装提交；Go固定新审核与成功事实仍可提交。
      await holder.beginTransaction();
      held = true;
      await holder.query(
        "SELECT id FROM durable_task WHERE kind='DRAFT_TRANSITION_INSTALLED' AND business_key=? FOR UPDATE",
        [id],
      );
      assert.equal(
        (await post(`/v1/documents/${f.d.id}/reverse-changes`, input, f.user.headers)).status,
        202,
      );
      let fact;
      const deadline = Date.now() + 20000;
      do {
        [[fact]] = await db.query(
          'SELECT state,CAST(result_json AS CHAR) raw FROM operation_receipt WHERE operation_id=?',
          [id],
        );
        if (fact.state === 'SUCCEEDED') break;
        await new Promise((resolve) => setTimeout(resolve, 100));
      } while (Date.now() < deadline);
      assert.equal(fact.state, 'SUCCEEDED');
      const result = JSON.parse(fact.raw).result;
      assert.equal(json(await get(`/v1/operations/${id}`, f.user.headers)).state, 'WAITING');
      const documentReady = () => request(state().ports.documentHttp, '/health/ready');
      assert.equal((await documentReady()).status, 200);
      const [installationFacts] = await db.query(
        "SELECT kind,CAST(payload AS CHAR) raw FROM durable_task WHERE operation_id=? AND kind IN ('REVERSE_COMMITTED','REVERSE_RECONCILE')",
        [id],
      );
      assert.equal(installationFacts.length, 2);
      // 轮次表禁止UPDATE；可写恢复元数据也不能把新审核的版本引用替换成另一真实历史版本。
      try {
        await db.query(
          "UPDATE durable_task SET payload=JSON_SET(payload,'$.Version',CAST(? AS UNSIGNED)) WHERE operation_id=? AND kind IN ('REVERSE_COMMITTED','REVERSE_RECONCILE')",
          [originalReview.content_version_id, id],
        );
        assert.equal(
          (await documentReady()).status,
          503,
          'substituted review source must fail readiness',
        );
      } finally {
        for (const row of installationFacts)
          await db.query('UPDATE durable_task SET payload=? WHERE kind=? AND business_key=?', [
            row.raw,
            row.kind,
            id,
          ]);
      }
      assert.equal((await documentReady()).status, 200);
      await assert.rejects(() => openReviewWorkspace(f.f, f.d), /EDIT_SESSION_FAILED/);
      const pendingApprove = await post(
        `/v1/review-requests/${result.request_id}/approve-and-merge`,
        {
          operation_id: operation('too-early-reverse-approval'),
          round_id: result.round_id,
          content_version_id: result.content_version_id,
          own_changes_to_resolve: [],
        },
        f.user.headers,
      );
      assert.equal(pendingApprove.status, 409, pendingApprove.body);
      assert.equal(json(pendingApprove).code, 'BRANCH_NOT_READY');
      const [[row]] = await editor.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='DRAFT_MAIN_GATE' AND business_key=?",
        [f.d.id],
      );
      gate = { document: f.d.id, raw: row.raw };
      assert.equal(valueOf(row.raw).binding.context.target.purpose, 'REVERSE');
      assert.equal(valueOf(row.raw).state, 'PREPARED');
      const nodeReady = () => request(state().ports.collaborationHttp, '/health/ready');
      assert.equal((await nodeReady()).status, 200);
      await editor.query(
        "UPDATE durable_task SET payload=JSON_SET(payload,'$.binding.context.actor_id','another-actor') WHERE kind='DRAFT_MAIN_GATE' AND business_key=?",
        [f.d.id],
      );
      assert.equal(
        (await nodeReady()).status,
        503,
        'reverse gate must retain the original fixed source binding',
      );
      await editor.query(
        "UPDATE durable_task SET payload=? WHERE kind='DRAFT_MAIN_GATE' AND business_key=?",
        [row.raw, f.d.id],
      );
      gate = null;
      assert.equal((await nodeReady()).status, 200);
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      await holder.rollback();
      held = false;
      const logout = await post(
        '/v1/account/logout',
        { operation_id: operation('reverse-author-logout') },
        f.user.headers,
      );
      assert.ok([200, 202].includes(logout.status), logout.body);
      restartOwnedRuntime();
      stopped = false;
      let installed = false;
      const settle = Date.now() + 20000;
      do {
        const [[row]] = await db.query(
          "SELECT COUNT(*) n FROM durable_task WHERE kind='REVERSE_INSTALLED' AND business_key=? AND state='SUCCEEDED'",
          [id],
        );
        installed = Number(row.n) === 1;
        if (installed) break;
        await new Promise((resolve) => setTimeout(resolve, 100));
      } while (Date.now() < settle);
      assert.equal(installed, true);
      const [[after]] = await db.query(
        'SELECT state,CAST(result_json AS CHAR) raw FROM operation_receipt WHERE operation_id=?',
        [id],
      );
      assert.equal(after.state, 'SUCCEEDED');
      assert.equal(digest(after.raw), digest(fact.raw));
      const [[review]] = await db.query(
        'SELECT status,current_round_id FROM review_request WHERE id=?',
        [result.request_id],
      );
      assert.equal(review.status, 'REVIEWING');
      assert.equal(String(review.current_round_id), result.round_id);
      const [[main]] = await db.query('SELECT current_manifest_id FROM document WHERE id=?', [
        f.d.id,
      ]);
      assert.equal(String(main.current_manifest_id), merged.result.manifest_id);
      const [[rounds]] = await db.query(
        'SELECT COUNT(*) n FROM review_round WHERE submission_operation_id=?',
        [id],
      );
      assert.equal(Number(rounds.n), 1);
    } finally {
      if (gate)
        await editor.query(
          "UPDATE durable_task SET payload=? WHERE kind='DRAFT_MAIN_GATE' AND business_key=?",
          [gate.raw, gate.document],
        );
      if (held) await holder.rollback();
      f.close();
      await holder.end();
      await editor.end();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
