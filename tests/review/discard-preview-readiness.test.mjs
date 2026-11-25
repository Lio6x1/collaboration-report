import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, reviewDatabase, writeDraft } from '../helpers/review.mjs';
import { post, json } from '../helpers/workspace.mjs';
import {
  discardPreviewPath,
  discardPreviewInput,
  pollDiscardPreview,
} from '../helpers/discard-previews.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import { request } from '../helpers/https.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

test(
  'pending discard preview readiness checks actual original admission and preserves immutable sources',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(),
      holder = await reviewDatabase(),
      editor = await reviewDatabase();
    let stopped = false,
      held = false,
      admission;
    const ready = () => request(state().ports.documentHttp, '/health/ready');
    try {
      await writeDraft(f, '启动核验不能接受换掉的原准入');
      const input = discardPreviewInput(f);
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      const pending = await post(discardPreviewPath(f), input, f.user.headers);
      assert.equal(pending.status, 503, pending.body);
      await holder.beginTransaction();
      held = true;
      const [[task]] = await holder.query(
        "SELECT state FROM durable_task WHERE kind='DISCARD_PREVIEW_RECOVER' AND business_key=? FOR UPDATE",
        [input.operation_id],
      );
      assert.ok(['QUEUED', 'RETRY', 'RUNNING'].includes(task.state));
      const [[context]] = await holder.query(
        "SELECT payload FROM durable_task WHERE kind='DISCARD_PREVIEW_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      const fixed =
        typeof context.payload === 'string' ? JSON.parse(context.payload) : context.payload;
      const id = fixed.Context.document_admission_ref;
      const [[original]] = await editor.query(
        'SELECT actor_id,unit_id,CAST(target_json AS CHAR) target FROM document_admission WHERE admission_id=?',
        [id],
      );
      const [sources] = await editor.query(
        'SELECT scope_key,epoch FROM admission_source WHERE admission_id=? ORDER BY scope_key',
        [id],
      );
      assert.ok(sources.length > 0);
      admission = { id, original, source: sources[0] };
      assert.equal((await ready()).status, 200);
      // 工作线程被本测试原任务行锁隔离；仅改自己的准入，全部变动在finally恢复。
      for (const altered of [
        'UPDATE document_admission SET actor_id=actor_id+1 WHERE admission_id=?',
        'UPDATE document_admission SET unit_id=NULL WHERE admission_id=?',
        "UPDATE document_admission SET target_json=JSON_SET(target_json,'$.purpose','OTHER_OPERATION') WHERE admission_id=?",
      ]) {
        await editor.query(altered, [id]);
        assert.equal(
          (await ready()).status,
          503,
          'substituted original admission must fail readiness',
        );
        await editor.query(
          'UPDATE document_admission SET actor_id=?,unit_id=?,target_json=? WHERE admission_id=?',
          [original.actor_id, original.unit_id, original.target, id],
        );
        assert.equal((await ready()).status, 200);
      }
      // 原来源事实受真实数据库权限保护；故障注入不能扩大应用账号的UPDATE权限。
      await assert.rejects(
        () =>
          editor.query(
            'UPDATE admission_source SET epoch=epoch+1 WHERE admission_id=? AND scope_key=?',
            [id, sources[0].scope_key],
          ),
        (error) => error.code === 'ER_TABLEACCESS_DENIED_ERROR',
      );
      assert.equal((await ready()).status, 200);
      await holder.commit();
      held = false;
      restartOwnedRuntime();
      stopped = false;
      assert.equal(
        (await pollDiscardPreview(input.operation_id, f.user.headers)).state,
        'SUCCEEDED',
      );
    } finally {
      try {
        if (admission) {
          const { id, original } = admission;
          await editor.query(
            'UPDATE document_admission SET actor_id=?,unit_id=?,target_json=? WHERE admission_id=?',
            [original.actor_id, original.unit_id, original.target, id],
          );
        }
      } finally {
        if (held) await holder.rollback();
        f.close();
        await holder.end();
        await editor.end();
        if (stopped) restartOwnedRuntime();
      }
    }
  },
);
