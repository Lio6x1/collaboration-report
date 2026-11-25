import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { importMarkdown } from '../../packages/content/markdown.mjs';
import { canonical, fromPM } from '../../packages/content/index.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';
import {
  reviewFixture,
  reviewDatabase,
  openReviewWorkspace,
  writeDraftBody,
} from '../helpers/review.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { unlockStructurePreview, structurePreviewInput } from '../helpers/structure-previews.mjs';
import { pollMainControl } from '../helpers/main-controls.mjs';
import { waitOperation } from '../helpers/account.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import { wire } from '../helpers/wss.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import { root, runtime } from '../../scripts/runtime-lib.mjs';
import { createStore } from '../../services/collaboration/src/bootstrap/store.mjs';
import { createPeers } from '../../services/collaboration/src/client/peers.mjs';
import { Transactions } from '../../services/collaboration/src/repository/sql.mjs';
import { SerializedRooms } from '../../services/collaboration/src/application/serialized_rooms.mjs';
import { RoomAdmissions } from '../../services/collaboration/src/application/room_admissions.mjs';

async function confirmation(f, type) {
  const document = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
  const input = structurePreviewInput(f, document, type);
  const response = await post(`/v1/documents/${f.d.id}/structure-previews`, input, f.user.headers);
  assert.equal(response.status, 200, json(response).code);
  return {
    ...input,
    operation_id: operation('structure-recovery-confirm'),
    changes: input.changes.map((change) => ({
      ...change,
      preview_cut_ref: json(response).preview_cut_ref,
    })),
  };
}

async function waitPrepared(node, document, id) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    const [[row]] = await node.query(
      'SELECT operation_id,state FROM main_barrier WHERE document_id=?',
      [document],
    );
    if (row?.operation_id === id && row.state === 'PREPARED') return;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.fail('ACTUAL_STRUCTURE_PREPARED_NOT_OBSERVED');
}

async function fixedPayload(database, kind, id) {
  const [[row]] = await database.query(
    'SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind=? AND business_key=? AND operation_id=?',
    [kind, id, id],
  );
  assert.ok(row, 'ACTUAL_FIXED_ARTIFACT_MISSING');
  return row.raw;
}

async function fixedControlResult(reader, peer, id, expected = 'SUCCEEDED') {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    const [[row]] = await reader.query('SELECT state FROM operation_receipt WHERE operation_id=?', [
      id,
    ]);
    if (row?.state === expected) {
      const original = JSON.parse(await fixedPayload(reader, 'MAIN_CONTROL_CONTEXT', id));
      const context = original.context;
      const reply = await rpcCall(peer, 'GetOperationResult', {
        operation_id: id,
        target: context.target,
        request_fingerprint: Buffer.from(context.request_fingerprint, 'base64'),
      });
      assert.equal(reply.state, expected);
      return { reply, value: JSON.parse(Buffer.from(reply.fixed_result_json_utf8).toString()) };
    }
    assert.ok(!row || row.state === 'PREPARING', 'STRUCTURE_CONTROL_REJECTED');
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.fail('ACTUAL_STRUCTURE_COMMIT_NOT_OBSERVED');
}

async function waitOriginalSessionClosure(reader, original) {
  const context = JSON.parse(original).context;
  const [sources] = await reader.query(
    "SELECT scope_key,epoch FROM admission_source WHERE admission_id=? AND scope_key LIKE 'session:%'",
    [context.document_admission_ref],
  );
  assert.equal(sources.length, 1, 'ACTUAL_ORIGINAL_SESSION_SOURCE_REQUIRED');
  const source = sources[0];
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    const [[gate]] = await reader.query(
      'SELECT state,max_epoch FROM access_gate WHERE scope_key=?',
      [source.scope_key],
    );
    if (
      gate &&
      (BigInt(gate.max_epoch) > BigInt(source.epoch) ||
        (BigInt(gate.max_epoch) === BigInt(source.epoch) && gate.state === 'BLOCKED'))
    )
      return;
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  assert.fail('ACTUAL_DOCUMENT_SESSION_CLOSURE_NOT_OBSERVED');
}

test(
  'actual PREPARED structure abort after logout restores old gates and draft for a current administrator',
  { timeout: 240000 },
  async () => {
    const f = await reviewFixture(1);
    const administrator = f.f.members[0];
    const locker = await reviewDatabase();
    const node = await reviewDatabase('collaboration');
    const reader = await reviewDatabase();
    const peer = documentRPC();
    let locked = false,
      stopped = false,
      restored;
    try {
      await waitOperation(
        await post(
          `/v1/organizations/${f.f.organization}/members/${administrator.id}/role`,
          { operation_id: operation('abort-administrator'), expected_revision: '1', role: 'ADMIN' },
          f.user.headers,
        ),
        f.user,
      );
      const before = await unlockStructurePreview(f);
      await writeDraftBody(f, {
        type: 'doc',
        content: [
          {
            type: 'paragraph',
            attrs: { id: 'structure-abort-draft' },
            content: [{ type: 'text', text: '撤销准备仍保留原可靠草稿' }],
          },
        ],
      });
      const input = await confirmation(f, 'REMOVE');
      stopOwnedRole('collaboration');
      stopped = true;
      const made = await post(`/v1/documents/${f.d.id}/structure-changes`, input, f.user.headers);
      assert.equal(made.status, 202, json(made).code);
      // 让真实准备先完成，仅把原号最终提交停在来源锁之前；不修改操作/会话状态。
      await locker.beginTransaction();
      locked = true;
      const [[gate]] = await locker.query(
        'SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE',
        ['operation:' + input.operation_id],
      );
      assert.ok(gate);
      const original = await fixedPayload(reader, 'MAIN_CONTROL_CONTEXT', input.operation_id);
      restartOwnedRuntime();
      stopped = false;
      await waitPrepared(node, f.d.id, input.operation_id);
      const preparation = await fixedPayload(node, 'STRUCTURE_PREPARATION', input.operation_id);
      const branches = JSON.parse(preparation).branches;
      assert.equal(branches.length, 1);
      assert.equal(branches[0].present, true);
      const [[blocked]] = await node.query('SELECT state FROM room WHERE id=?', [
        branches[0].room_id,
      ]);
      assert.equal(blocked.state, 'BLOCKED');
      const logout = await post(
        '/v1/account/logout',
        { operation_id: operation('structure-prepared-logout') },
        f.user.headers,
      );
      assert.equal(logout.status, 202, json(logout).code);
      // 202只是账号服务登记关闭；协议允许Doc本地关闭之前合法最终提交。
      // 本例要证明关闭先发生，必须由独立读连接观察原会话来源的实际关闭点，
      // 再放开operation锁；不能让原持锁事务的旧快照或调度快慢决定预期。
      await waitOriginalSessionClosure(reader, original);
      await locker.rollback();
      locked = false;
      // 系统读原固定终态不依赖已注销的Cookie，撤销只根据真实REJECTED而不是超时猜测。
      const rejected = await fixedControlResult(reader, peer, input.operation_id, 'REJECTED');
      assert.equal(rejected.value.state, 'REJECTED');
      const deadline = Date.now() + 30000;
      let aborted = false;
      while (Date.now() < deadline) {
        const [[barrier]] = await node.query(
          'SELECT operation_id,state FROM main_barrier WHERE document_id=?',
          [f.d.id],
        );
        const [[branchGate]] = await node.query(
          "SELECT JSON_UNQUOTE(JSON_EXTRACT(payload,'$.state')) state FROM durable_task WHERE kind='STRUCTURE_BRANCH_GATE' AND business_key=? AND operation_id=?",
          [branches[0].room_id + ':' + branches[0].workspace_epoch, input.operation_id],
        );
        if (
          barrier?.operation_id === input.operation_id &&
          barrier.state === 'ABORTED' &&
          branchGate?.state === 'ABORTED'
        ) {
          aborted = true;
          break;
        }
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
      assert.ok(aborted, 'ACTUAL_STRUCTURE_ABORT_NOT_OBSERVED');
      assert.equal(
        await fixedPayload(node, 'STRUCTURE_PREPARATION', input.operation_id),
        preparation,
      );
      assert.equal(
        await fixedPayload(reader, 'MAIN_CONTROL_CONTEXT', input.operation_id),
        original,
      );
      const [[installation]] = await node.query(
        'SELECT COUNT(*) n FROM install_receipt WHERE operation_id=?',
        [input.operation_id],
      );
      assert.equal(Number(installation.n), 0);
      assert.equal((await get(`/v1/documents/${f.d.id}`, f.user.headers)).status, 401);
      const actual = await get(`/v1/documents/${f.d.id}`, administrator.headers);
      assert.equal(actual.status, 200, json(actual).code);
      assert.equal(json(actual).current_structure_id, before.current_structure_id);
      assert.equal(json(actual).current_manifest_id, before.current_manifest_id);
      restored = await openReviewWorkspace(f.f, f.d, administrator);
      assert.equal(restored.session.branch_id, f.session.branch_id);
      assert.equal(restored.session.workspace_epoch, f.session.workspace_epoch);
      assert.match(
        canonical(fromPM(yDocToProsemirrorJSON(restored.doc))),
        /撤销准备仍保留原可靠草稿/,
      );
    } finally {
      restored?.close();
      f.close();
      peer.close();
      if (locked) await locker.rollback();
      await locker.end();
      await node.end();
      await reader.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);

test(
  'actual PREPARED virtual branch denies OPEN across owned restart and resumes the same confirmation',
  { timeout: 240000 },
  async () => {
    const organization = await organizationFixture(0);
    const d = await documentFixture(organization);
    const f = { f: organization, d, user: organization.owner };
    const locker = await reviewDatabase();
    const node = await reviewDatabase('collaboration');
    const reader = await reviewDatabase();
    const config = JSON.parse(readFileSync(resolve(runtime, 'services/collaboration.json')));
    const probeStore = createStore(config),
      probePeers = createPeers({ ...config, root });
    let locked = false,
      stopped = false,
      late;
    try {
      const before = await unlockStructurePreview(f);
      const input = await confirmation(f, 'REMOVE');
      stopOwnedRole('collaboration');
      stopped = true;
      const made = await post(`/v1/documents/${d.id}/structure-changes`, input, f.user.headers);
      assert.equal(made.status, 202, json(made).code);
      // 只持有本原号已有互斥行；真实Go最终事务在取得来源/Doc锁前等待，不篡改任何业务事实。
      await locker.beginTransaction();
      locked = true;
      const [[gate]] = await locker.query(
        'SELECT scope_key FROM access_gate WHERE scope_key=? FOR UPDATE',
        ['operation:' + input.operation_id],
      );
      assert.ok(gate);
      const original = await fixedPayload(reader, 'MAIN_CONTROL_CONTEXT', input.operation_id);
      restartOwnedRuntime();
      stopped = false;
      await waitPrepared(node, d.id, input.operation_id);
      const preparation = await fixedPayload(node, 'STRUCTURE_PREPARATION', input.operation_id);
      assert.ok(JSON.parse(preparation).branches.every((branch) => !branch.present));
      const client = operation('virtual-prepared-client');
      const session = await post(
        `/v1/documents/${d.id}/units/${d.main.units[0].unit_id}/edit-sessions`,
        {
          operation_id: operation('virtual-prepared-edit'),
          workspace_kind: 'BRANCH',
          client_session_id: client,
          expected_structure_id: d.main.structure.id,
        },
        f.user.headers,
      );
      assert.equal(session.status, 200, json(session).code);
      const connection = operation('virtual-prepared-connection');
      late = wire(json(session), client, connection);
      // 未登记输出的OPEN失败直接关闭socket，不能要求在未受控输出上另发ERROR帧。
      await assert.rejects(late.ready, /SOCKET_CLOSED/);
      // 再用相同真实票据/连接与真实mTLS Doc服务、同一个Node库运行生产准入用例，精确证明关闭门槛。
      const admission = new RoomAdmissions(
        probeStore,
        probePeers,
        operation('blocked-probe-instance'),
        new Transactions(probeStore.pool),
        new SerializedRooms(),
        null,
      );
      await assert.rejects(
        () =>
          admission.open(
            {
              protocol: 'collab-wss-v1',
              client_session_id: client,
              connection_id: connection,
              ticket: json(session).ticket,
              workspace_epoch: json(session).workspace_epoch,
            },
            operation('blocked-probe-handler'),
          ),
        (error) => error.code === 14 && error.details === 'STRUCTURE_BRANCH_GATE_PENDING',
      );
      const [[missing]] = await node.query('SELECT COUNT(*) n FROM room WHERE id=?', [
        json(session).room_id,
      ]);
      assert.equal(Number(missing.n), 0);
      const pending = await get(`/v1/operations/${input.operation_id}`, f.user.headers);
      assert.equal(pending.status, 200, json(pending).code);
      assert.equal(json(pending).state, 'PREPARING');
      const unchanged = json(await get(`/v1/documents/${d.id}`, f.user.headers));
      assert.equal(unchanged.current_structure_id, before.current_structure_id);
      restartOwnedRuntime();
      await waitPrepared(node, d.id, input.operation_id);
      assert.equal(
        await fixedPayload(node, 'STRUCTURE_PREPARATION', input.operation_id),
        preparation,
      );
      assert.equal(
        await fixedPayload(reader, 'MAIN_CONTROL_CONTEXT', input.operation_id),
        original,
      );
      await locker.rollback();
      locked = false;
      const result = await pollMainControl(input.operation_id, f.user);
      assert.equal(result.state, 'SUCCEEDED', result.error?.code);
      assert.equal(result.result.main_sync_state, 'READY');
      const main = await get(`/v1/documents/${d.id}/main`, f.user.headers);
      assert.equal(main.status, 200, json(main).code);
      assert.equal(json(main).units.length, 0);
      assert.equal(
        await fixedPayload(reader, 'MAIN_CONTROL_CONTEXT', input.operation_id),
        original,
      );
    } finally {
      late?.close();
      if (locked) await locker.rollback();
      await locker.end();
      await node.end();
      await reader.end();
      probePeers.close();
      await probeStore.pool.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);

test(
  'actual split installation interruption keeps fixed success and RESTORING branches until complete recovery',
  { timeout: 240000 },
  async () => {
    const f = await reviewFixture();
    const node = await reviewDatabase('collaboration');
    const reader = await reviewDatabase();
    let locked = false,
      stopped = false,
      main;
    const opened = [];
    const peer = documentRPC();
    try {
      await unlockStructurePreview(f);
      main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
      let next = 0;
      const accepted = importMarkdown('## 第一节\n\n已接纳一\n\n## 第二节\n\n已接纳二', {
        newId: () => `structure-recovery-${++next}`,
      }).body;
      const draft = structuredClone(accepted);
      draft.content.push({
        type: 'paragraph',
        attrs: { id: 'structure-recovery-S2' },
        content: [{ type: 'text', text: '安装中断仍保留草稿S2' }],
      });
      await writeDraftBody(main, accepted);
      await writeDraftBody(f, draft);
      const input = await confirmation(f, 'SPLIT_LEVEL2');
      await node.beginTransaction();
      locked = true;
      // 真实安装回执空隙锁使整个新房间/旧房间退役TX等待，不伪造成功，也不提前开放新支线。
      await node.query('SELECT operation_id FROM install_receipt WHERE operation_id=? FOR UPDATE', [
        input.operation_id,
      ]);
      const made = await post(`/v1/documents/${f.d.id}/structure-changes`, input, f.user.headers);
      assert.equal(made.status, 202, json(made).code);
      // ACL正在真实关闭时公开GET必须拒绝过渡来源；系统权威查询仍能读取实际不可变成功事实。
      const committed = await fixedControlResult(reader, peer, input.operation_id);
      const syncing = committed.value;
      assert.equal(syncing.state, 'SUCCEEDED', syncing.error?.code);
      assert.equal(syncing.result.main_sync_state, 'SYNCING');
      const [[receipt]] = await reader.query(
        'SELECT CAST(result_json AS CHAR) raw FROM operation_receipt WHERE operation_id=?',
        [input.operation_id],
      );
      const fixed = await fixedPayload(reader, 'MAIN_COMMIT_FIXED', input.operation_id);
      const branches = JSON.parse(fixed).structure_transition.branches;
      assert.equal(branches.length, 2);
      const [restoring] = await reader.query(
        "SELECT id,lifecycle FROM branch WHERE document_id=? AND lifecycle='RESTORING' ORDER BY id",
        [f.d.id],
      );
      assert.equal(restoring.length, 2);
      const denied = await post(
        `/v1/documents/${f.d.id}/units/${branches[0].Unit}/edit-sessions`,
        {
          operation_id: operation('restoring-edit-denied'),
          workspace_kind: 'BRANCH',
          client_session_id: operation('restoring-client'),
          expected_structure_id: String(JSON.parse(fixed).Structure),
        },
        f.user.headers,
      );
      assert.equal(denied.status, 409, json(denied).code);
      assert.ok(
        ['SCOPE_TRANSITION_PENDING', 'BRANCH_NOT_READY', 'MAIN_COMMIT_PENDING'].includes(
          json(denied).code,
        ),
      );
      stopOwnedRole('collaboration');
      stopped = true;
      await node.rollback();
      locked = false;
      const unchanged = await fixedControlResult(reader, peer, input.operation_id);
      assert.ok(
        Buffer.from(unchanged.reply.fixed_result_json_utf8).equals(
          Buffer.from(committed.reply.fixed_result_json_utf8),
        ),
      );
      restartOwnedRuntime();
      stopped = false;
      const ready = await pollMainControl(input.operation_id, f.user);
      assert.equal(ready.state, 'SUCCEEDED', ready.error?.code);
      assert.equal(ready.result.main_sync_state, 'READY');
      assert.equal(ready.result.manifest_id, syncing.result.manifest_id);
      assert.equal(await fixedPayload(reader, 'MAIN_COMMIT_FIXED', input.operation_id), fixed);
      const [[sameReceipt]] = await reader.query(
        'SELECT CAST(result_json AS CHAR) raw FROM operation_receipt WHERE operation_id=?',
        [input.operation_id],
      );
      assert.equal(sameReceipt.raw, receipt.raw);
      const after = json(await get(`/v1/documents/${f.d.id}/main`, f.user.headers));
      for (const unit of after.units) {
        const branch = await openReviewWorkspace(f.f, {
          ...f.d,
          main: { ...after, units: [unit] },
        });
        opened.push(branch);
        const content = canonical(fromPM(yDocToProsemirrorJSON(branch.doc)));
        assert.ok(content.includes('已接纳'));
        if (after.structure.entries.find((v) => v.unit_id === unit.unit_id).title === '第二节')
          assert.match(content, /安装中断仍保留草稿S2/);
      }
      const [active] = await reader.query(
        "SELECT id FROM branch WHERE document_id=? AND lifecycle='ACTIVE' ORDER BY id",
        [f.d.id],
      );
      assert.equal(active.length, 2);
    } finally {
      for (const branch of opened) branch.close();
      main?.close();
      f.close();
      peer.close();
      if (locked) await node.rollback();
      await node.end();
      await reader.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
