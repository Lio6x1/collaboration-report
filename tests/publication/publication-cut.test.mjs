import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  reviewDatabase,
  openReviewWorkspace,
  writeDraft,
} from '../helpers/review.mjs';
import { get, post, json, operation, waitAuthorization } from '../helpers/workspace.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { prepareOwnedBlocking } from '../helpers/owned-blocking.mjs';
import { until } from '../helpers/authorization-fence.mjs';
import { releaseInput, pollPublication } from '../helpers/publication.mjs';

// 暂停真实Node只用于确定任务先登记，再锁住Doc原operation gate。
// 捕获RPC不使用这把锁；Node固定S1后，最终发布事务才等待它，避免用改写结果冒充真实竞争。
async function fixedCut(f, db, node, blocker) {
  const input = await releaseInput(f.d.id, f.user);
  await blocker.block();
  const response = await post(`/v1/documents/${f.d.id}/releases`, input, f.user.headers);
  assert.equal(response.status, 202, json(response).code);
  await until(async () => {
    const [[task]] = await db.query(
      "SELECT state FROM durable_task WHERE kind='PUBLICATION_RECOVER' AND operation_id=?",
      [input.operation_id],
    );
    return task?.state === 'RUNNING';
  }, 'ACTUAL_PUBLICATION_CLAIM_REQUIRED');
  await db.beginTransaction();
  const [[gate]] = await db.query('SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE', [
    'operation:' + input.operation_id,
  ]);
  assert.ok(gate, 'ORIGINAL_OPERATION_GATE_REQUIRED');
  await blocker.release();
  const captured = await until(async () => {
    const [[row]] = await node.query(
      "SELECT state,payload FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND business_key=?",
      [input.operation_id + ':RELEASE_CREATE:1'],
    );
    if (row?.state !== 'SUCCEEDED') return false;
    return typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
  }, 'ACTUAL_MAIN_CUT_REQUIRED');
  assert.ok(captured.units.length, 'COMPLETE_CAPTURE_REQUIRED');
  assert.ok(
    Buffer.from(captured.units[0].body_json_utf8, 'base64').toString().includes('固定切点S1'),
    'ACTUAL_S1_CAPTURE_REQUIRED',
  );
  return input;
}

async function releaseBody(db, document, release) {
  const [[row]] = await db.query(
    'SELECT v.body_json FROM `release` r JOIN manifest_entry e ON e.manifest_id=r.manifest_id JOIN content_version v ON v.id=e.content_version_id WHERE r.document_id=? AND r.id=?',
    [document, release],
  );
  assert.ok(row, 'ACTUAL_RELEASE_CONTENT_REQUIRED');
  return JSON.stringify(row.body_json);
}

test(
  'publication uses the real fixed S1 cut while later SAVED S2 remains in subsequent MAIN',
  { timeout: 60000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase(),
      node = await reviewDatabase('collaboration');
    let main, blocker;
    try {
      await unlockStructurePreview(f);
      main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      await writeDraft(main, '可靠主线固定切点S1');
      blocker = await prepareOwnedBlocking('collaboration');
      const input = await fixedCut(f, db, node, blocker);
      await writeDraft(main, '固定切点之后已保存的S2');
      await db.rollback();
      const result = await pollPublication(input.operation_id, f.user);
      assert.equal(result.state, 'SUCCEEDED', result.error?.code);
      const body = await releaseBody(db, f.d.id, result.result.entity_id);
      assert.ok(body.includes('固定切点S1'), 'RELEASE_MUST_RETAIN_S1');
      assert.ok(!body.includes('已保存的S2'), 'LATER_INPUT_MUST_NOT_ENTER_OLD_CUT');
      const response = await get(`/v1/documents/${f.d.id}/main`, f.user.headers);
      assert.equal(response.status, 200, json(response).code);
      const view = json(response);
      const content = await get(
        `/v1/documents/${f.d.id}/read-views/${view.read_view_id}/units/${view.units[0].unit_id}`,
        f.user.headers,
      );
      assert.equal(content.status, 200, json(content).code);
      assert.ok(JSON.stringify(json(content).body_json).includes('已保存的S2'));
      assert.ok(
        (await releaseBody(db, f.d.id, result.result.entity_id)) === body,
        'READING_LATER_MAIN_MUST_NOT_REWRITE_RELEASE',
      );
    } finally {
      await blocker?.release();
      await db.rollback();
      main?.close();
      f.close();
      await node.end();
      await db.end();
    }
  },
);

test(
  'a title and control change after real MAIN capture rejects the original publication atomically',
  { timeout: 60000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase(),
      node = await reviewDatabase('collaboration');
    let main, blocker;
    try {
      await unlockStructurePreview(f);
      main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      await writeDraft(main, '标题竞争前的固定切点S1');
      blocker = await prepareOwnedBlocking('collaboration');
      const input = await fixedCut(f, db, node, blocker);
      const metadata = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      const renamed = await post(
        `/v1/documents/${f.d.id}`,
        {
          operation_id: operation('rename-after-cut'),
          expected_control_revision: metadata.control_revision,
          title: '可靠捕获之后的新标题',
        },
        f.user.headers,
        'PATCH',
      );
      assert.equal(renamed.status, 200, json(renamed).code);
      await db.rollback();
      const result = await pollPublication(input.operation_id, f.user);
      assert.equal(result.state, 'REJECTED');
      assert.ok(['MAIN_CONTROL_CHANGED', 'MAIN_SOURCE_CHANGED'].includes(result.error.code));
      const [[counts]] = await db.query(
        'SELECT (SELECT COUNT(*) FROM `release` WHERE operation_id=?) releases,(SELECT COUNT(*) FROM outbox WHERE event_id=?) events',
        [input.operation_id, input.operation_id],
      );
      assert.equal(Number(counts.releases), 0);
      assert.equal(Number(counts.events), 0);
      const [[capture]] = await node.query(
        "SELECT state FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND business_key=?",
        [input.operation_id + ':RELEASE_CREATE:1'],
      );
      assert.equal(capture.state, 'SUCCEEDED', 'REJECTION_MUST_RETAIN_ORIGINAL_NODE_FACT');
      const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      assert.equal(current.title, '可靠捕获之后的新标题');
      assert.ok(current.current_release_id == null);
    } finally {
      await blocker?.release();
      await db.rollback();
      main?.close();
      f.close();
      await node.end();
      await db.end();
    }
  },
);

test(
  'REVIEW cannot publish and loss of actual MANAGE after fixed capture rejects the original pending operation',
  { timeout: 90000 },
  async () => {
    const f = await reviewFixture(2),
      db = await reviewDatabase(),
      node = await reviewDatabase('collaboration'),
      peer = documentRPC();
    const [manager, reviewer] = f.f.members;
    let main, blocker;
    try {
      await unlockStructurePreview(f);
      const grant = (user, capability) => ({
        user_id: user.id,
        scope_kind: 'DOCUMENT',
        scope_id: f.d.id,
        capability,
      });
      const before = json(await get(`/v1/documents/${f.d.id}`, f.f.owner.headers));
      await waitAuthorization(
        await post(
          `/v1/documents/${f.d.id}/authorization-changes`,
          {
            operation_id: operation('publication-roles'),
            expected_acl_revision: before.acl_revision,
            changes: [
              { action: 'GRANT', grant: grant(manager, 'MANAGE') },
              { action: 'GRANT', grant: grant(reviewer, 'REVIEW') },
            ],
          },
          f.f.owner.headers,
        ),
        f.f.owner,
      );
      const denied = await post(
        `/v1/documents/${f.d.id}/releases`,
        await releaseInput(f.d.id, f.f.owner),
        reviewer.headers,
      );
      assert.equal(denied.status, 403);
      main = await openReviewWorkspace(f.f, f.d, f.f.owner, 'MAIN');
      await writeDraft(main, '撤销管理之前的固定切点S1');
      f.user = manager;
      blocker = await prepareOwnedBlocking('collaboration');
      const input = await fixedCut(f, db, node, blocker);
      const metadata = json(await get(`/v1/documents/${f.d.id}`, f.f.owner.headers));
      const [[receipt]] = await node.query(
        "SELECT FROM_BASE64(JSON_UNQUOTE(JSON_EXTRACT(payload,'$.binding.context.request_fingerprint'))) fingerprint FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND business_key=?",
        [input.operation_id + ':RELEASE_CREATE:1'],
      );
      const query = {
        operation_id: input.operation_id,
        request_fingerprint: receipt.fingerprint,
        target: {
          document_id: f.d.id,
          unit_id: '0',
          branch_id: '0',
          structure_id: metadata.current_structure_id,
          workspace_epoch: metadata.control_revision,
          purpose: 'PUBLICATION',
        },
      };
      assert.equal((await rpcCall(peer, 'GetOperationResult', query)).state, 'PROCESSING');
      await waitAuthorization(
        await post(
          `/v1/documents/${f.d.id}/authorization-changes`,
          {
            operation_id: operation('publication-revoke-manage'),
            expected_acl_revision: metadata.acl_revision,
            changes: [{ action: 'REVOKE', grant: grant(manager, 'MANAGE') }],
          },
          f.f.owner.headers,
        ),
        f.f.owner,
      );
      await db.rollback();
      const result = await until(
        async () => {
          const value = await rpcCall(peer, 'GetOperationResult', query);
          return value.state === 'REJECTED' && value;
        },
        'LOST_MANAGEMENT_MUST_REJECT_ORIGINAL_PUBLICATION',
        20000,
      );
      assert.ok(result.business_code, 'EXPLICIT_REJECTION_REQUIRED');
      assert.deepEqual(result.result_ids, []);
      assert.equal(
        (await get(`/v1/operations/${input.operation_id}`, manager.headers)).status,
        403,
      );
      const [[counts]] = await db.query(
        'SELECT (SELECT COUNT(*) FROM `release` WHERE operation_id=?) releases,(SELECT COUNT(*) FROM outbox WHERE event_id=?) events',
        [input.operation_id, input.operation_id],
      );
      assert.equal(Number(counts.releases), 0);
      assert.equal(Number(counts.events), 0);
    } finally {
      await blocker?.release();
      await db.rollback();
      main?.close();
      f.close();
      peer.close();
      await node.end();
      await db.end();
    }
  },
);
