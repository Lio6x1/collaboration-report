import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import test from 'node:test';
import { runtime } from '../../scripts/runtime-lib.mjs';
import { env } from '../helpers/account.mjs';
import { until } from '../helpers/authorization-fence.mjs';
import { request } from '../helpers/https.mjs';
import { prepareOwnedBlocking } from '../helpers/owned-blocking.mjs';
import { pausedRead } from '../helpers/paused-http.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { searchDatabase } from '../helpers/search.mjs';
import { pageVisibility, publishSearchDocument, searchPagePath } from '../helpers/search-pages.mjs';
import {
  get,
  json,
  operation,
  organizationFixture,
  post,
  waitAuthorization,
} from '../helpers/workspace.mjs';

// 小样本全部经实际产品创建、发布及消费。SQL只作为最终锁窗口和持久事实观察，
// 不伪造正文、权限、输出或ACK；它与后续大规模SQL/ES样本构造关口分开。
async function publicFixture(count) {
  const f = await organizationFixture(0);
  f.q = 'pageoutput' + randomUUID().replaceAll('-', '');
  await pageVisibility(`/v1/knowledge-bases/${f.kb}`, f.owner, 'PUBLIC');
  const search = await searchDatabase();
  try {
    f.documents = [];
    for (let index = 0; index < count; index++) {
      f.documents.push(
        await publishSearchDocument(
          f,
          {
            q: f.q,
            title: `${f.q} 固定标题 ${index}`,
            body: `${f.q} 固定摘要 ${index}`,
          },
          search,
        ),
      );
    }
    return f;
  } finally {
    await search.end();
  }
}

function startedRead(path) {
  // 注册拒绝分支，失败清理也等待原请求结束，避免悬挂Promise掩盖原反例。
  return get(path).then(
    (response) => ({ response }),
    () => ({ failed: true }),
  );
}

async function releaseFault(blocker, database, { fault, reading, pending } = {}) {
  // 每一阶段都收集失败，先放锁/恢复实际进程，再等原请求，最后关闭全部连接。
  // close()的客户端断言失败也不能截断后续清理，所有事实未知仍明确报告。
  const results = [];
  results.push(...(await Promise.allSettled([blocker.rollback(), fault?.release()])));
  results.push(...(await Promise.allSettled([reading?.close(), pending])));
  results.push(...(await Promise.allSettled([blocker.end(), database.end()])));
  assert.equal(
    results.filter((result) => result.status === 'rejected').length,
    0,
    'SEARCH_FAULT_CLEANUP_FAILED',
  );
}

async function waitFinalGate(database, scope) {
  await until(
    async () => {
      const [processes] = await database.query('SHOW FULL PROCESSLIST');
      const matching = processes.filter(
        (row) =>
          row.Info?.startsWith('INSERT INTO access_gate') &&
          row.Info.includes(`'${scope}'`) &&
          row.Info.includes('ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)'),
      );
      assert.ok(matching.length <= 1, 'SEARCH_FINAL_CONNECTION_NOT_UNIQUE');
      return matching.length === 1;
    },
    'SEARCH_FINAL_GATE_WAIT_NOT_OBSERVED',
    1000,
  );
}

async function leaseWatermark(database) {
  const [[row]] = await database.query(
    'SELECT CAST(COALESCE(MAX(id),0) AS CHAR) id FROM durable_task',
  );
  assert.match(row.id, /^(0|[1-9][0-9]*)$/);
  return row.id;
}

async function pageLeases(database, f, after) {
  const registry = JSON.parse(readFileSync(resolve(runtime, 'processes.json')));
  const pids = ['document', 'edge'].map((role) => {
    const process = registry.find((value) => value.role === role);
    assert.ok(process, 'SEARCH_REAL_OUTPUT_OWNER_REQUIRED');
    return process.pid;
  });
  const clauses = f.documents.map(
    () => "JSON_CONTAINS(t.payload,JSON_OBJECT('scope_key',?),'$.sources')",
  );
  // 起始水位+本例完整publication并集+真实进程，不能从全库猜最近两条记录。
  const [leases] = await database.query(
    `SELECT CAST(t.id AS CHAR) id,t.state,t.instance_id,t.payload
     FROM durable_task t JOIN output_instance i ON i.instance_id=t.instance_id
     WHERE t.kind='OUTPUT_LEASE' AND t.id>? AND i.pid IN (?,?)
       AND ${clauses.join(' AND ')} ORDER BY t.id`,
    [after, ...pids, ...f.documents.map((document) => `publication:${document.id}`)],
  );
  return leases.map((lease) => ({
    ...lease,
    binding: typeof lease.payload === 'string' ? JSON.parse(lease.payload) : lease.payload,
  }));
}

function assertLeasePair(leases, f) {
  assert.equal(leases.length, 2, 'EXACT_ACTUAL_SEARCH_OUTPUT_PAIR_REQUIRED');
  const local = leases.find((lease) => lease.instance_id.endsWith('-document'));
  const edge = leases.find((lease) => lease.instance_id.endsWith('-edge'));
  assert.ok(local && edge, 'REAL_GO_AND_EDGE_INSTANCES_REQUIRED');
  assert.deepEqual(local.binding.sources, edge.binding.sources);
  for (const lease of leases) {
    assert.equal(lease.binding.document_id, undefined);
    assert.equal(lease.binding.admission_id, undefined);
    for (const document of f.documents) {
      assert.ok(
        lease.binding.sources.some((source) => source.scope_key === `doc-acl:${document.id}`),
      );
      assert.ok(
        lease.binding.sources.some((source) => source.scope_key === `publication:${document.id}`),
      );
    }
  }
  return { local, edge };
}

test(
  'actual search return and lookahead policy changes reject the whole final page',
  { timeout: 180000 },
  async (t) => {
    for (const position of ['RETURNED', 'LOOKAHEAD']) {
      await t.test(position, async () => {
        const f = await publicFixture(3);
        const database = await reviewDatabase();
        const blocker = await reviewDatabase();
        let pending;
        try {
          const path = searchPagePath(f.q, { limit: 2 });
          const baseline = await get(path);
          assert.equal(baseline.status, 200, json(baseline).code);
          const page = json(baseline);
          assert.equal(page.items.length, 2);
          assert.equal(page.has_more, true);
          const firstScope = f.documents.map((document) => `doc-acl:${document.id}`).sort()[0];
          const returned = new Set(page.items.map((item) => item.document_id));
          const target = f.documents.find(
            (document) =>
              `doc-acl:${document.id}` !== firstScope &&
              returned.has(document.id) === (position === 'RETURNED'),
          );
          assert.ok(target, 'TARGET_MUST_FOLLOW_THE_FIRST_FINAL_GATE');
          const current = await get(`/v1/documents/${target.id}`, f.owner.headers);
          assert.equal(current.status, 200, json(current).code);
          const input = {
            operation_id: operation('search-final-policy'),
            expected_visibility_revision: json(current).visibility_revision,
            policy: 'COLLABORATORS',
          };
          const after = await leaseWatermark(database);
          await blocker.beginTransaction();
          await blocker.query('SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE', [
            firstScope,
          ]);
          pending = startedRead(path);
          await waitFinalGate(database, firstScope);
          // 另一来源尚未被页面持锁，真实政策原号可以完成。单Doc同锁范围的变更
          // 会被页面挡住，不能用它冒充“最终复核前事实已经改变”的反例。
          const changed = await post(
            `/v1/documents/${target.id}/visibility`,
            input,
            f.owner.headers,
          );
          assert.equal(changed.status, 202, json(changed).code);
          await waitAuthorization(changed, f.owner);
          await blocker.rollback();
          const result = await pending;
          assert.equal(result.failed, undefined, 'FINAL_CHANGE_MUST_RETURN_A_CONTRACT_ERROR');
          assert.equal(result.response.status, 409);
          assert.equal(json(result.response).code, 'PAGE_CHANGED_RETRY');
          assert.equal(result.response.body.includes(target.title), false);
          assert.equal((await pageLeases(database, f, after)).length, 0);
        } finally {
          await releaseFault(blocker, database, { pending });
        }
      });
    }
  },
);

test(
  'actual lookahead revocation waits for real search Edge settlement before ACK',
  { timeout: 120000 },
  async () => {
    const f = await publicFixture(2);
    const database = await reviewDatabase();
    const blocker = await reviewDatabase();
    let fault;
    let reading;
    let mutation;
    try {
      const path = searchPagePath(f.q, { limit: 1 });
      const baseline = await get(path);
      assert.equal(baseline.status, 200, json(baseline).code);
      const page = json(baseline);
      assert.equal(page.items.length, 1);
      assert.equal(page.has_more, true);
      const target = f.documents.find((document) => document.id !== page.items[0].document_id);
      assert.ok(target, 'EXACT_UNRETURNED_LOOKAHEAD_REQUIRED');
      const current = await get(`/v1/documents/${target.id}`, f.owner.headers);
      assert.equal(current.status, 200, json(current).code);
      const input = {
        operation_id: operation('search-lookahead-output-fence'),
        expected_visibility_revision: json(current).visibility_revision,
        policy: 'COLLABORATORS',
      };
      const scope = `doc-acl:${target.id}`;
      const closedEpoch = json(current).acl_revision;
      const [[originalGate]] = await database.query(
        'SELECT CAST(revision AS CHAR) revision FROM access_gate WHERE scope_key=?',
        [scope],
      );
      assert.ok(originalGate, 'REAL_LOOKAHEAD_GATE_REQUIRED');
      const after = await leaseWatermark(database);
      fault = await prepareOwnedBlocking('edge');
      const first = [...f.documents].sort((left, right) =>
        BigInt(left.id) < BigInt(right.id) ? -1 : 1,
      )[0];
      await blocker.beginTransaction();
      await blocker.query('SELECT id FROM document WHERE id=? FOR UPDATE', [first.id]);
      reading = pausedRead(path, { headers: {} });
      assert.equal((await reading.sent).tlsVerified, true);
      await until(
        async () => {
          try {
            await database.query(
              'SELECT state FROM access_gate WHERE scope_key=? FOR UPDATE NOWAIT',
              [`publication:${target.id}`],
            );
            return false;
          } catch (error) {
            if (error.errno === 3572) return true;
            throw Error('SEARCH_REAL_FINAL_SOURCE_PROBE_FAILED');
          }
        },
        'SEARCH_REAL_FINAL_SOURCES_NOT_HELD',
        1000,
      );
      await fault.block();
      await blocker.rollback();
      const leases = await until(
        async () => {
          const observed = await pageLeases(database, f, after);
          return observed.length === 2 ? observed : false;
        },
        'SEARCH_ACTUAL_OUTPUT_PAIR_NOT_COMMITTED',
        1000,
      );
      const pair = assertLeasePair(leases, f);
      assert.equal(pair.edge.state, 'RUNNING');
      assert.equal(pair.local.state, 'RUNNING');
      await blocker.beginTransaction();
      await blocker.query('SELECT id FROM durable_task WHERE id=? FOR UPDATE', [pair.edge.id]);
      // 暂停Edge时只用原Doc业务端口登记变更；敏感搜索仍走原真实Edge，
      // Cookie/Origin/CSRF正常验证，不重新POST推进或造关闭确认。
      mutation = request(env.ports.documentHttp, `/v1/documents/${target.id}/visibility`, {
        method: 'POST',
        headers: {
          ...f.owner.headers,
          origin: `https://localhost:${env.ports.edge}`,
          'content-type': 'application/json',
        },
        body: JSON.stringify(input),
      }).then(
        (response) => ({ response }),
        () => ({ failed: true }),
      );
      await until(
        async () => {
          const [[row]] = await database.query(
            "SELECT a.state,t.state task_state,CAST(g.revision AS CHAR) gate_revision,(SELECT COUNT(*) FROM authorization_ack k WHERE k.revocation_id=a.revocation_id AND k.participant='edge') edge_acks FROM authorization_change a JOIN durable_task t ON t.kind='AUTHORIZATION_CHANGE' AND t.operation_id=a.revocation_id JOIN access_gate g ON g.scope_key=a.scope_key WHERE a.revocation_id=? AND a.scope_key=? AND a.epoch=? AND g.last_revocation_id=a.revocation_id AND g.state='BLOCKED'",
            [input.operation_id, scope, closedEpoch],
          );
          // FENCING在业务登记事务中就存在，不能证明worker已经发出实际停止。
          // 第二次gate修订来自真实Close.prepare；只在原任务被领取且这个提交已完成后恢复Edge。
          return (
            row?.state === 'FENCING' &&
            row.task_state === 'RUNNING' &&
            BigInt(row.gate_revision) >= BigInt(originalGate.revision) + 2n &&
            String(row.edge_acks) === '0'
          );
        },
        'SEARCH_LOOKAHEAD_ORIGINAL_CLOSURE_NOT_WAITING',
        1000,
      );
      await fault.release();
      fault = null;
      await until(
        () => {
          const gates = JSON.parse(readFileSync(resolve(runtime, 'edge-output-gates.json')));
          return gates[scope]?.state === 'BLOCKED' && gates[scope].epoch === closedEpoch;
        },
        'ACTUAL_LOOKAHEAD_EDGE_FENCE_NOT_PERSISTED',
        1000,
      );
      reading.resume();
      const ended = await reading.completed;
      assert.notEqual(ended.failed, true, 'REAL_SEARCH_TLS_END_REQUIRED');
      assert.ok(ended.status !== 200 || !ended.complete, 'OLD_SEARCH_BODY_MUST_NOT_COMPLETE');
      assert.notEqual(ended.reason, 'TIMEOUT', 'CLIENT_DEADLINE_CANNOT_STAND_FOR_OUTPUT_ACK');
      const [[held]] = await database.query(
        "SELECT COUNT(*) n FROM authorization_ack WHERE revocation_id=? AND participant='edge'",
        [input.operation_id],
      );
      assert.equal(String(held.n), '0', 'ORIGINAL_DURABLE_SETTLEMENT_MUST_PRECEDE_ACK');
      await blocker.rollback();
      const made = await mutation;
      assert.equal(made.failed, undefined, 'ORIGINAL_VISIBILITY_REQUEST_MUST_RETURN');
      assert.equal(made.response.status, 202, json(made.response).code);
      await waitAuthorization(made.response, f.owner);
      await until(async () => {
        const settled = await pageLeases(database, f, after);
        return settled.length === 2 && settled.every((lease) => lease.state === 'SUCCEEDED');
      }, 'REAL_SEARCH_OUTPUT_PAIR_NOT_SETTLED');
      const [acks] = await database.query(
        "SELECT participant,result_digest FROM authorization_ack WHERE revocation_id=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED' ORDER BY participant",
        [input.operation_id, scope, closedEpoch],
      );
      assert.deepEqual(
        acks.map((ack) => ack.participant),
        ['collaboration', 'edge'],
      );
      // 原关闭ACK必须绑定完整范围、旧代次、原号、generation及目标摘要，
      // 只看32字节长度不足以排除另一个关闭结果被拼到本请求。
      const targetDigest = createHash('sha256')
        .update(['AUTHORIZATION_CHANGE', scope, closedEpoch].join('\n'))
        .digest('hex');
      for (const ack of acks) {
        const expected = createHash('sha256')
          .update(
            [
              'bootstrap-fence-v1',
              ack.participant,
              input.operation_id,
              scope,
              closedEpoch,
              '1',
              targetDigest,
            ].join('\n'),
          )
          .digest();
        assert.ok(
          Buffer.isBuffer(ack.result_digest) && ack.result_digest.equals(expected),
          'EXACT_ORIGINAL_LOOKAHEAD_ACK_DIGEST_REQUIRED',
        );
      }
    } finally {
      await releaseFault(blocker, database, { fault, reading, pending: mutation });
    }
  },
);
