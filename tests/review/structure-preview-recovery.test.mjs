import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, writeDraft, reviewDatabase } from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { completedMainControl } from '../helpers/main-controls.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import { request } from '../helpers/https.mjs';
import { state } from '../../scripts/runtime-lib.mjs';
import { origin } from '../helpers/account.mjs';

test(
  'structure preview survives natural proof expiry and owner restart under its original operation',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase();
    let stopped = false;
    try {
      const locked = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      await completedMainControl(
        await post(
          `/v1/documents/${f.d.id}/main-lock`,
          {
            operation_id: operation('preview-recovery-unlock'),
            expected_control_revision: locked.control_revision,
            locked: false,
          },
          f.user.headers,
        ),
        f.user,
      );
      const before = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      await writeDraft(f, '原号预览跨过自然期限也要保留可靠草稿');
      const input = {
        operation_id: operation('recover-structure-preview'),
        expected_structure_id: before.current_structure_id,
        expected_control_revision: before.control_revision,
        changes: [{ type: 'REMOVE', chapter_id: f.d.main.structure.entries[0].chapter_id }],
      };
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      const pending = await request(
        state().ports.edge,
        `/v1/documents/${f.d.id}/structure-previews`,
        {
          method: 'POST',
          headers: { origin, ...f.user.headers, 'content-type': 'application/json' },
          body: JSON.stringify(input),
          timeoutMs: 6000,
        },
      );
      assert.equal(pending.status, 503, pending.body);
      assert.equal(json(pending).code, 'STRUCTURE_PREVIEW_PREPARING');
      await new Promise((resolve) => setTimeout(resolve, 31000));
      const progress = await get(`/v1/operations/${input.operation_id}`, f.user.headers);
      assert.equal(progress.status, 200, progress.body);
      assert.equal(json(progress).state, 'PREPARING');
      const [[row]] = await db.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='STRUCTURE_PREVIEW_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      const original = JSON.parse(row.raw);
      assert.ok(
        original.Access.Proofs.every((proof) => Number(proof.expires_unix_ms) < Date.now()),
      );
      restartOwnedRuntime();
      stopped = false;
      const deadline = Date.now() + 20000;
      let result;
      do {
        result = json(await get(`/v1/operations/${input.operation_id}`, f.user.headers));
        if (result.state !== 'PREPARING') break;
        await new Promise((resolve) => setTimeout(resolve, 100));
      } while (Date.now() < deadline);
      assert.equal(result.state, 'SUCCEEDED', result.error?.code);
      // 这里只读取原号：重启之前已登记，恢复不依赖浏览器第二次POST重新取得准入。
      const [[same]] = await db.query(
        "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='STRUCTURE_PREVIEW_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      assert.equal(same.raw, row.raw);
      const [[fixed]] = await db.query(
        "SELECT payload FROM durable_task WHERE kind='STRUCTURE_PREVIEW_FIXED' AND business_key=?",
        [input.operation_id],
      );
      const value = typeof fixed.payload === 'string' ? JSON.parse(fixed.payload) : fixed.payload;
      assert.match(
        Buffer.from(value.drafts[0].body_json_utf8, 'base64').toString(),
        /原号预览跨过自然期限/,
      );
      const after = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
      assert.equal(after.current_structure_id, before.current_structure_id);
      assert.equal(after.current_manifest_id, before.current_manifest_id);
      assert.equal(after.control_revision, before.control_revision);
    } finally {
      f.close();
      await db.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
