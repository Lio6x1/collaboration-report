import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  writeDraft,
  reviewDatabase,
  collaborationRPC,
  openReviewWorkspace,
} from '../helpers/review.mjs';
import { post, get, json, operation } from '../helpers/workspace.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { pullInput, pullPath, pollPull, persistedPull } from '../helpers/pull-main.mjs';

test('reliable input after the fixed pull preview rejects that original operation and preserves the later draft', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase(),
    peer = collaborationRPC();
  let locked = false,
    next;
  const input = pullInput(f, operation('stale-preview-pull'));
  try {
    await writeDraft(f, '固定预览之前的正文');
    // 202的输出准入也会等待网络；先观察真正登记的领取行，不能等响应结束后再与worker竞速。
    // READ COMMITTED不锁尚不存在的唯一键间隙，所以登记事务仍可插入；发现后只锁这个原号。
    await db.query('SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED');
    await db.beginTransaction();
    locked = true;
    const accepted = post(pullPath(f), input, f.user.headers);
    let task;
    const registrationDeadline = Date.now() + 10000;
    do {
      [[task]] = await db.query(
        "SELECT state FROM durable_task WHERE kind='PULL_MAIN_RECOVER' AND business_key=? FOR UPDATE",
        [input.operation_id],
      );
      if (task) break;
      await new Promise((resolve) => setTimeout(resolve, 1));
    } while (Date.now() < registrationDeadline);
    assert.ok(task, 'fixture must observe the actual registered recovery task');
    assert.ok(
      ['QUEUED', 'RETRY'].includes(task.state),
      'fixture must hold the worker before it is claimed',
    );
    assert.equal((await accepted).status, 202);
    const [[row]] = await db.query(
      "SELECT CAST(payload AS CHAR) AS raw FROM durable_task WHERE kind='PULL_MAIN_CONTEXT' AND business_key=?",
      [input.operation_id],
    );
    const fixed = persistedPull(row.raw),
      context = {
        ...fixed.context,
        request_fingerprint: Buffer.from(fixed.context.request_fingerprint, 'base64'),
      };
    const request = {
      context,
      expected_branch_id: fixed.workspace.Workspace,
      expected_structure_id: fixed.Structure,
      expected_lineage_id: fixed.workspace.Lineage,
    };
    const first = await rpcCall(peer, 'CaptureDraftPreview', request);
    assert.equal(first.state, 'FIXED');
    await writeDraft(f, '预览之后可靠保存的新输入必须保留');
    assert.deepEqual(await rpcCall(peer, 'CaptureDraftPreview', request), first);
    await db.commit();
    locked = false;
    const result = await pollPull(input.operation_id, f.user.headers);
    assert.equal(result.state, 'REJECTED', result.error?.code);
    assert.equal(result.error.code, 'DRAFT_CHANGED');
    // 先等待原系统收尾完成，再申请新编辑会话；不会重POST修复旧操作。
    const deadline = Date.now() + 10000;
    let settled = false;
    do {
      const [[row]] = await db.query(
        "SELECT COUNT(*) AS n FROM durable_task WHERE kind='PULL_MAIN_INSTALLED' AND business_key=? AND state='SUCCEEDED'",
        [input.operation_id],
      );
      settled = String(row.n) === '1';
      if (settled) break;
      await new Promise((r) => setTimeout(r, 100));
    } while (Date.now() < deadline);
    assert.equal(settled, true);
    next = await openReviewWorkspace(f.f, f.d);
    assert.equal(next.session.workspace_epoch, f.session.workspace_epoch);
    assert.equal(next.session.lineage_id, f.session.lineage_id);
    await writeDraft(next, '拒绝旧预览后支线仍可可靠保存');
  } finally {
    if (locked) await db.rollback();
    next?.close();
    peer.close();
    f.close();
    await db.end();
  }
});

test('a pull conflict keeps both fixed inputs and changes no branch epoch or formal main pointer', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase(),
    node = await reviewDatabase('collaboration');
  let main;
  try {
    const initial = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    await completedMainControl(
      await post(
        `/v1/documents/${f.d.id}/main-lock`,
        {
          operation_id: operation('conflict-unlock'),
          expected_control_revision: initial.control_revision,
          locked: false,
        },
        f.user.headers,
      ),
      f.user,
    );
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    await writeDraft(main, '主线在同一块新增A');
    await writeDraft(f, '支线在同一块新增B');
    const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const input = pullInput(f, operation('conflicting-pull'));
    assert.equal((await post(pullPath(f), input, f.user.headers)).status, 202);
    const result = await pollPull(input.operation_id, f.user.headers);
    assert.equal(result.state, 'REJECTED');
    assert.equal(result.error.code, 'PULL_MAIN_CONFLICT');
    const [[fixed]] = await node.query(
      "SELECT payload FROM durable_task WHERE kind='DRAFT_PREVIEW_FIXED' AND business_key=?",
      [input.operation_id],
    );
    const preview = typeof fixed.payload === 'string' ? JSON.parse(fixed.payload) : fixed.payload;
    assert.ok(preview.conflicts.length > 0);
    assert.equal(preview.candidate, null);
    assert.match(
      Buffer.from(preview.main.body_json_utf8, 'base64').toString(),
      /主线在同一块新增A/,
    );
    assert.match(
      Buffer.from(preview.draft.body_json_utf8, 'base64').toString(),
      /支线在同一块新增B/,
    );
    const [[branch]] = await db.query(
      'SELECT workspace_epoch,current_lineage_id FROM branch WHERE id=?',
      [f.session.branch_id],
    );
    assert.equal(String(branch.workspace_epoch), f.session.workspace_epoch);
    assert.equal(String(branch.current_lineage_id), f.session.lineage_id);
    const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    assert.equal(after.current_manifest_id, before.current_manifest_id);
    assert.equal(after.control_revision, before.control_revision);
    await writeDraft(f, '冲突后原草稿仍可以保存');
  } finally {
    main?.close();
    f.close();
    await db.end();
    await node.end();
  }
});
