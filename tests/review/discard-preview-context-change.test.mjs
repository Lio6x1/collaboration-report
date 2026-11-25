import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, reviewDatabase, writeDraft } from '../helpers/review.mjs';
import { post } from '../helpers/workspace.mjs';
import { discardPreviewPath, discardPreviewInput } from '../helpers/discard-previews.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

test(
  'changed authoritative MAIN context rejects original pending preview while Node is unavailable',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(),
      database = await reviewDatabase();
    let stopped = false,
      control;
    try {
      await writeDraft(f, '原预览不能永远等待已经改变的主线');
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      const input = discardPreviewInput(f);
      const pending = await post(discardPreviewPath(f), input, f.user.headers);
      assert.equal(pending.status, 503, pending.body);
      const [[before]] = await database.query('SELECT control_revision FROM document WHERE id=?', [
        f.d.id,
      ]);
      control = before.control_revision;
      // 仅注入此夹具权威代次改变，不伪造主线成功或切点；finally恢复原字段和实际owner环境。
      await database.query('UPDATE document SET control_revision=control_revision+1 WHERE id=?', [
        f.d.id,
      ]);
      let fact;
      const deadline = Date.now() + 8000;
      do {
        [[fact]] = await database.query(
          'SELECT state,result_json FROM operation_receipt WHERE operation_id=?',
          [input.operation_id],
        );
        if (fact.state !== 'PREPARING') break;
        await new Promise((resolve) => setTimeout(resolve, 100));
      } while (Date.now() < deadline);
      assert.equal(fact.state, 'REJECTED', 'known changed MAIN must not wait for unavailable Node');
      const result =
        typeof fact.result_json === 'string' ? JSON.parse(fact.result_json) : fact.result_json;
      assert.equal(result.error.code, 'MAIN_CONTEXT_CHANGED');
      const [[count]] = await database.query(
        'SELECT COUNT(*) total FROM content_version WHERE source_operation_id=?',
        [input.operation_id],
      );
      assert.equal(Number(count.total), 0);
    } finally {
      if (control)
        await database.query('UPDATE document SET control_revision=? WHERE id=?', [
          control,
          f.d.id,
        ]);
      f.close();
      await database.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
