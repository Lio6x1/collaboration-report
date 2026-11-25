import test from 'node:test';
import assert from 'node:assert/strict';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { pollMainControl, completedMainControl } from '../helpers/main-controls.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

// 真实准入自然过期以后，后台只能续接原管理会话/来源；不得靠浏览器再次POST取得新权力。
test(
  'original main control resumes after natural identity expiry and owned restart',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(0),
      d = await documentFixture(f),
      db = await reviewDatabase();
    let stopped = false;
    try {
      const current = json(await get(`/v1/documents/${d.id}`, f.owner.headers)),
        id = operation('recover-main-control');
      stopOwnedRole('collaboration');
      stopped = true;
      const made = await post(
        `/v1/documents/${d.id}/main-lock`,
        { operation_id: id, expected_control_revision: current.control_revision, locked: false },
        f.owner.headers,
      );
      assert.equal(made.status, 202, json(made).code);
      await new Promise((resolve) => setTimeout(resolve, 31000));
      const pending = await get(`/v1/operations/${id}`, f.owner.headers);
      assert.equal(pending.status, 200, json(pending).code);
      assert.equal(json(pending).state, 'PREPARING');
      const [[row]] = await db.query(
        "SELECT payload FROM durable_task WHERE kind='MAIN_CONTROL_CONTEXT' AND business_key=?",
        [id],
      );
      const original = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
      assert.ok(original.access.Proofs.every((proof) => proof.expires_unix_ms < Date.now()));
      assert.equal(JSON.stringify(original).includes('__Host-collabSession'), false);
      restartOwnedRuntime();
      stopped = false;
      const result = await pollMainControl(id, f.owner);
      assert.equal(result.state, 'SUCCEEDED', result.error?.code);
      assert.equal(result.result.main_sync_state, 'READY');
      const after = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
      assert.equal(after.main_locked, false);
      assert.equal(BigInt(after.control_revision), BigInt(current.control_revision) + 1n);
      const [[same]] = await db.query(
        "SELECT payload FROM durable_task WHERE kind='MAIN_CONTROL_CONTEXT' AND business_key=?",
        [id],
      );
      assert.deepEqual(
        typeof same.payload === 'string' ? JSON.parse(same.payload) : same.payload,
        original,
      );
    } finally {
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);

// 对实际安装回执加空隙锁，使Go成功与Node安装明确分离；停机后必须继续同一固定结果。
test(
  'committed style stays successful and syncing through interrupted Node installation',
  { timeout: 180000 },
  async () => {
    const f = await organizationFixture(0),
      d = await documentFixture(f),
      node = await reviewDatabase('collaboration');
    let stopped = false,
      locked = false;
    try {
      const current = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
      await completedMainControl(
        await post(
          `/v1/documents/${d.id}/main-lock`,
          {
            operation_id: operation('recovery-style-unlock'),
            expected_control_revision: current.control_revision,
            locked: false,
          },
          f.owner.headers,
        ),
        f.owner,
      );
      const before = json(await get(`/v1/documents/${d.id}/main`, f.owner.headers)),
        id = operation('recover-style-install');
      await node.beginTransaction();
      locked = true;
      await node.query('SELECT operation_id FROM install_receipt WHERE operation_id=? FOR UPDATE', [
        id,
      ]);
      const made = await post(
        `/v1/documents/${d.id}/styles`,
        {
          operation_id: id,
          expected_style_id: before.style.id,
          expected_control_revision: before.document.control_revision,
          style: { ...before.style.style, body_font: 'FangSong' },
        },
        f.owner.headers,
      );
      assert.equal(made.status, 202, json(made).code);
      const syncing = await pollMainControl(id, f.owner, { ready: false });
      assert.equal(syncing.state, 'SUCCEEDED', syncing.error?.code);
      assert.equal(syncing.result.main_sync_state, 'SYNCING');
      const [[missing]] = await node.query(
        'SELECT COUNT(*) AS n FROM install_receipt WHERE operation_id=?',
        [id],
      );
      assert.equal(Number(missing.n), 0);
      stopOwnedRole('collaboration');
      stopped = true;
      await node.rollback();
      locked = false;
      const unchanged = await get(`/v1/operations/${id}`, f.owner.headers);
      assert.equal(unchanged.status, 200, json(unchanged).code);
      assert.deepEqual(json(unchanged).result, syncing.result);
      restartOwnedRuntime();
      stopped = false;
      const ready = await pollMainControl(id, f.owner);
      assert.equal(ready.state, 'SUCCEEDED');
      assert.equal(ready.result.main_sync_state, 'READY');
      assert.equal(ready.result.manifest_id, syncing.result.manifest_id);
      assert.equal(ready.result.entity_id, syncing.result.entity_id);
      const after = json(await get(`/v1/documents/${d.id}/main`, f.owner.headers));
      assert.equal(after.style.id, ready.result.entity_id);
      assert.equal(after.style.style.body_font, 'FangSong');
      const [[installed]] = await node.query(
        'SELECT COUNT(*) AS n FROM install_receipt WHERE operation_id=?',
        [id],
      );
      assert.equal(Number(installed.n), 1);
    } finally {
      if (locked) await node.rollback();
      await node.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
