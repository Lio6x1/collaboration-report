import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, reviewDatabase, writeDraft } from '../helpers/review.mjs';
import { post } from '../helpers/workspace.mjs';
import { discardPreviewPath, discardPreviewInput } from '../helpers/discard-previews.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import { request } from '../helpers/https.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

test(
  'registration uses one final authoritative MAIN snapshot after waiting for the document lock',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(),
      holder = await reviewDatabase(),
      editor = await reviewDatabase(),
      account = await reviewDatabase('account');
    let stopped = false,
      held = false,
      control,
      input;
    try {
      await writeDraft(f, '固定原操作的注册快照');
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      await holder.beginTransaction();
      held = true;
      const [[before]] = await holder.query(
        'SELECT control_revision FROM document WHERE id=? FOR UPDATE',
        [f.d.id],
      );
      control = before.control_revision;
      input = discardPreviewInput(f);
      const responsePromise = post(discardPreviewPath(f), input, f.user.headers);
      // Account真实原号准入已取得才释放Doc行锁，确定授权快照和最终注册之间的竞争窗口。
      const deadline = Date.now() + 1500;
      let acquired;
      do {
        [[acquired]] = await account.query(
          "SELECT COUNT(*) total FROM identity_admission WHERE operation_id=? AND purpose='DISCARD_PREVIEW'",
          [input.operation_id],
        );
        if (Number(acquired.total) >= 2) break;
        await new Promise((resolve) => setTimeout(resolve, 20));
      } while (Date.now() < deadline);
      assert.ok(Number(acquired.total) >= 2, 'actual original identity proof was not acquired');
      await holder.query('UPDATE document SET control_revision=control_revision+1 WHERE id=?', [
        f.d.id,
      ]);
      await holder.commit();
      held = false;
      const pending = await responsePromise;
      assert.equal(pending.status, 503, pending.body);
      const [[row]] = await editor.query(
        "SELECT payload FROM durable_task WHERE kind='DISCARD_PREVIEW_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      const fixed = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
      assert.equal(
        fixed.Access.Document.control_revision,
        String(fixed.Scope.Control),
        'final registration must not mix two MAIN snapshots',
      );
      assert.equal((await request(state().ports.documentHttp, '/health/ready')).status, 200);
    } finally {
      try {
        if (held) await holder.rollback();
        if (control)
          await editor.query('UPDATE document SET control_revision=? WHERE id=?', [
            control,
            f.d.id,
          ]);
        // 恢复真实字段后固定预期已确定过时；Go自行拒绝原号，不能改写原上下文让测试通过。
        if (input) {
          const deadline = Date.now() + 12000;
          let fact;
          do {
            [[fact]] = await editor.query(
              'SELECT state FROM operation_receipt WHERE operation_id=?',
              [input.operation_id],
            );
            if (!fact || fact.state !== 'PREPARING') break;
            await new Promise((resolve) => setTimeout(resolve, 100));
          } while (Date.now() < deadline);
          assert.equal(
            fact?.state,
            'REJECTED',
            'race fixture must be closed under its original operation',
          );
        }
      } finally {
        f.close();
        await holder.end();
        await editor.end();
        await account.end();
        if (stopped) restartOwnedRuntime();
      }
    }
  },
);
