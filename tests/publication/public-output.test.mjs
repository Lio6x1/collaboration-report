import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';
import { reviewFixture, reviewDatabase } from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { env } from '../helpers/account.mjs';
import { request } from '../helpers/https.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';
import { populatePublicUnit } from '../helpers/public-capacity.mjs';
import { prepareOwnedBlocking } from '../helpers/owned-blocking.mjs';
import { pausedRead } from '../helpers/paused-http.mjs';
import { until } from '../helpers/authorization-fence.mjs';

// 实际匿名读取在Doc当前读行锁处等待，再暂停真实Edge。避免“客户端尚未读完”
// 被误当作服务端仍未结束；原持久settlement另以真实行锁验证，任何期限都不能充当ACK。
test(
  'actual anonymous publication output remains pending while its real Edge owner is blocked and settles before original publication ACK',
  { timeout: 120000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase(),
      blocker = await reviewDatabase();
    let fault,
      reading,
      locked = false;
    try {
      await unlockStructurePreview(f);
      const viewResponse = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
      assert.equal(viewResponse.status, 200, json(viewResponse).code);
      const view = json(viewResponse);
      await populatePublicUnit(f, view, view.units[0], 8);
      await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
      for (const path of [`/v1/knowledge-bases/${f.f.kb}`, `/v1/documents/${f.d.id}`]) {
        const current = json(await get(path, f.user.headers));
        await waitAuthorization(
          await post(
            path + '/visibility',
            {
              operation_id: operation('anonymous-output-visibility'),
              expected_visibility_revision: current.visibility_revision,
              policy: 'PUBLIC',
            },
            f.user.headers,
          ),
          f.user,
        );
      }
      const path = `/v1/public/documents/${f.d.id}`;
      const direct = await request(env.ports.documentHttp, path);
      assert.equal(direct.status, 503, 'PRIVATE_HTTP_CANNOT_BYPASS_CONTROLLED_PUBLIC_OUTPUT');
      const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      const scope = `publication:${f.d.id}`;
      const [[originalGate]] = await db.query(
        'SELECT CAST(revision AS CHAR) revision FROM access_gate WHERE scope_key=?',
        [scope],
      );
      assert.ok(originalGate, 'ACTUAL_PUBLICATION_GATE_REQUIRED');
      fault = await prepareOwnedBlocking('edge');
      // 锁住本例Doc当前读行，直到实际匿名读取持有publication来源锁；不插入假输出记录。
      const input = await releaseInput(f.d.id, f.user);
      await blocker.beginTransaction();
      locked = true;
      await blocker.query('SELECT id FROM document WHERE id=? FOR UPDATE', [f.d.id]);
      reading = pausedRead(path, { headers: {} });
      assert.equal((await reading.sent).tlsVerified, true);
      await until(async () => {
        try {
          await db.query('SELECT state FROM access_gate WHERE scope_key=? FOR UPDATE NOWAIT', [
            `publication:${f.d.id}`,
          ]);
          return false;
        } catch (error) {
          if (error.errno === 3572) return true;
          throw Error('ACTUAL_ANONYMOUS_SOURCE_PROBE_FAILED');
        }
      }, 'ACTUAL_ANONYMOUS_TRANSACTION_NOT_WAITING');
      await fault.block();
      // 网关被暂停时仅使用Doc真实发布处理器登记原号；Cookie/Origin/CSRF仍正常核验。
      // 匿名敏感正文始终走原Edge，不能为避开暂停改走私有正文端口。
      const publication = request(env.ports.documentHttp, `/v1/documents/${f.d.id}/releases`, {
        method: 'POST',
        headers: {
          ...f.user.headers,
          origin: `https://localhost:${env.ports.edge}`,
          'content-type': 'application/json',
        },
        body: JSON.stringify(input),
      });
      await blocker.rollback();
      locked = false;
      const made = await publication;
      assert.equal(made.status, 202, json(made).code);
      await until(async () => {
        const [[fact]] = await db.query(
          "SELECT COUNT(*) n FROM delivery_session WHERE document_id=? AND publication_epoch=? AND release_id=? AND instance_id LIKE '%-edge'",
          [f.d.id, before.publication_epoch, before.current_release_id],
        );
        return String(fact.n) === '1';
      }, 'ACTUAL_ANONYMOUS_DELIVERY_NOT_REGISTERED');
      const [[lease]] = await db.query(
        "SELECT t.id,d.delivery_id,d.instance_id,d.admission_id,t.payload FROM delivery_session d JOIN durable_task t ON t.kind='OUTPUT_LEASE' AND t.business_key=d.delivery_id WHERE d.document_id=? AND d.publication_epoch=? AND d.release_id=? AND d.instance_id LIKE '%-edge' AND d.state='OPEN' ORDER BY t.id DESC LIMIT 1",
        [f.d.id, before.publication_epoch, before.current_release_id],
      );
      assert.ok(lease, 'ACTUAL_OPEN_ANONYMOUS_EDGE_DELIVERY_REQUIRED');
      assert.equal(lease.admission_id, null);
      const binding = typeof lease.payload === 'string' ? JSON.parse(lease.payload) : lease.payload;
      assert.ok(
        binding.sources.some(
          (source) =>
            source.scope_key === `publication:${f.d.id}` &&
            source.epoch === before.publication_epoch,
        ),
      );
      await blocker.beginTransaction();
      locked = true;
      await blocker.query('SELECT id FROM durable_task WHERE id=? FOR UPDATE', [lease.id]);
      await until(
        async () => {
          const [[fact]] = await db.query(
            "SELECT o.state receipt_state,a.state closure_state,c.state closure_task_state,CAST(g.revision AS CHAR) gate_revision,(SELECT COUNT(*) FROM `release` r WHERE r.operation_id=o.operation_id) releases FROM operation_receipt o JOIN authorization_change a ON a.revocation_id=o.operation_id JOIN durable_task c ON c.operation_id=a.revocation_id AND c.kind='AUTHORIZATION_CHANGE' JOIN access_gate g ON g.scope_key=a.scope_key AND g.last_revocation_id=a.revocation_id WHERE o.operation_id=?",
            [input.operation_id],
          );
          return (
            fact?.receipt_state === 'PREPARING' &&
            fact.closure_state === 'FENCING' &&
            fact.closure_task_state === 'RUNNING' &&
            BigInt(fact.gate_revision) >= BigInt(originalGate.revision) + 2n &&
            String(fact.releases) === '1'
          );
        },
        'ORIGINAL_PUBLICATION_MUST_WAIT_FOR_ACTUAL_EDGE',
        10000,
      );
      // 暂停的实际参与者尚未确认旧输出，原持久结算也被锁住；不能按客户端期限补ACK。
      const [[pending]] = await db.query(
        "SELECT d.state delivery_state,t.state task_state,o.state receipt_state,(SELECT COUNT(*) FROM authorization_ack a WHERE a.revocation_id=o.operation_id AND a.participant='edge') edge_acks FROM delivery_session d JOIN durable_task t ON t.kind='OUTPUT_LEASE' AND t.business_key=d.delivery_id JOIN operation_receipt o ON o.operation_id=? WHERE d.delivery_id=?",
        [input.operation_id, lease.delivery_id],
      );
      assert.equal(pending.delivery_state, 'OPEN');
      assert.equal(pending.task_state, 'RUNNING');
      assert.equal(pending.receipt_state, 'PREPARING');
      assert.equal(String(pending.edge_acks), '0');
      await fault.release();
      fault = null;
      // 登记FENCING先于关闭器执行，不能据此恢复Edge。上面核原worker准备提交，
      // 这里再核真正Edge fence写出的持久水位；客户端期限本身不充当停止证明。
      await until(
        () => {
          const gates = JSON.parse(readFileSync(resolve(runtime, 'edge-output-gates.json')));
          return (
            gates[scope]?.state === 'BLOCKED' && gates[scope].epoch === before.publication_epoch
          );
        },
        'ACTUAL_PUBLICATION_EDGE_FENCE_NOT_PERSISTED',
        1000,
      );
      reading.resume();
      const ended = await reading.completed;
      assert.notEqual(ended.failed, true, 'ACTUAL_TLS_REQUEST_END_REQUIRED');
      assert.ok(ended.status !== 200 || !ended.complete, 'OLD_ANONYMOUS_BODY_MUST_NOT_COMPLETE');
      assert.notEqual(ended.reason, 'TIMEOUT', 'CLIENT_DEADLINE_CANNOT_PROVE_ACTUAL_OUTPUT_STOP');
      const [[held]] = await db.query(
        "SELECT COUNT(*) n FROM authorization_ack WHERE revocation_id=? AND participant='edge'",
        [input.operation_id],
      );
      assert.equal(String(held.n), '0', 'ACTUAL_SETTLEMENT_LOCK_MUST_PREVENT_PREMATURE_ACK');
      await blocker.rollback();
      locked = false;
      const result = await pollPublication(input.operation_id, f.user);
      assert.equal(result.state, 'SUCCEEDED', result.error?.code);
      const [[settled]] = await db.query(
        "SELECT d.state delivery_state,t.state task_state,(SELECT COUNT(*) FROM authorization_ack a WHERE a.revocation_id=? AND a.participant='edge' AND a.phase='OUTPUT_STOPPED' AND OCTET_LENGTH(a.result_digest)=32) edge_acks FROM delivery_session d JOIN durable_task t ON t.kind='OUTPUT_LEASE' AND t.business_key=d.delivery_id WHERE d.delivery_id=?",
        [input.operation_id, lease.delivery_id],
      );
      assert.equal(settled.delivery_state, 'SETTLED');
      assert.equal(settled.task_state, 'SUCCEEDED');
      assert.equal(String(settled.edge_acks), '1');
      const current = await get(path);
      assert.equal(current.status, 200, json(current).code);
      assert.equal(json(current).release_id, result.result.entity_id);
    } finally {
      // 客户端关闭断言失败也必须恢复实际参与者并关闭全部SQL连接。
      const cleanup = [];
      cleanup.push(
        ...(await Promise.allSettled([locked ? blocker.rollback() : undefined, fault?.release()])),
      );
      cleanup.push(...(await Promise.allSettled([reading?.close()])));
      f.close();
      cleanup.push(...(await Promise.allSettled([blocker.end(), db.end()])));
      assert.equal(
        cleanup.filter((result) => result.status === 'rejected').length,
        0,
        'PUBLIC_OUTPUT_CLEANUP_FAILED',
      );
    }
  },
);
