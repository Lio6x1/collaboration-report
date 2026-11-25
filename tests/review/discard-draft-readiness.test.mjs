import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, reviewDatabase, writeDraft } from '../helpers/review.mjs';
import { post, json } from '../helpers/workspace.mjs';
import {
  discardPreviewPath,
  discardPreviewInput,
  discardDraftPath,
  discardDraftInput,
  pollDiscardDraft,
} from '../helpers/discard-previews.mjs';
import { request } from '../helpers/https.mjs';
import { state } from '../../scripts/runtime-lib.mjs';
import { stopOwnedRole, restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

test('Node readiness rejects substituted persistent draft MAIN gate binding', async () => {
  const f = await reviewFixture(),
    node = await reviewDatabase('collaboration');
  let original;
  try {
    await writeDraft(f, '检查持久屏障原绑定');
    const response = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
    assert.equal(response.status, 200, response.body);
    const preview = json(response),
      input = discardDraftInput(preview);
    assert.equal((await post(discardDraftPath(preview), input, f.user.headers)).status, 202);
    assert.equal((await pollDiscardDraft(input.operation_id, f.user.headers)).state, 'SUCCEEDED');
    const [[row]] = await node.query(
      "SELECT CAST(payload AS CHAR) raw FROM durable_task WHERE kind='DRAFT_MAIN_GATE' AND business_key=?",
      [f.d.id],
    );
    original = row.raw;
    assert.equal((await request(state().ports.collaborationHttp, '/health/ready')).status, 200);
    await node.query(
      "UPDATE durable_task SET payload=JSON_SET(payload,'$.binding.context.actor_id','99999999') WHERE kind='DRAFT_MAIN_GATE' AND business_key=?",
      [f.d.id],
    );
    const invalid = await request(state().ports.collaborationHttp, '/health/ready');
    assert.equal(invalid.status, 503, 'substituted installed gate must fail readiness');
  } finally {
    if (original)
      await node.query(
        "UPDATE durable_task SET payload=? WHERE kind='DRAFT_MAIN_GATE' AND business_key=?",
        [original, f.d.id],
      );
    f.close();
    await node.end();
  }
});

test(
  'Go readiness binds pending confirmation to its actual unit admission',
  { timeout: 180000 },
  async () => {
    const f = await reviewFixture(),
      holder = await reviewDatabase(),
      editor = await reviewDatabase();
    let held = false,
      stopped = false,
      admission;
    const ready = () => request(state().ports.documentHttp, '/health/ready');
    try {
      await writeDraft(f, '待确认原准入核验');
      const response = await post(discardPreviewPath(f), discardPreviewInput(f), f.user.headers);
      assert.equal(response.status, 200, response.body);
      const preview = json(response),
        input = discardDraftInput(preview);
      f.close();
      stopOwnedRole('collaboration');
      stopped = true;
      assert.equal((await post(discardDraftPath(preview), input, f.user.headers)).status, 202);
      await holder.beginTransaction();
      held = true;
      await holder.query(
        "SELECT id FROM durable_task WHERE kind='DISCARD_DRAFT_RECOVER' AND business_key=? FOR UPDATE",
        [input.operation_id],
      );
      const [[context]] = await holder.query(
        "SELECT payload FROM durable_task WHERE kind='DISCARD_DRAFT_CONTEXT' AND business_key=?",
        [input.operation_id],
      );
      const fixed =
        typeof context.payload === 'string' ? JSON.parse(context.payload) : context.payload;
      const id = fixed.Context.document_admission_ref;
      const [[original]] = await editor.query(
        'SELECT actor_id,unit_id,CAST(target_json AS CHAR) target FROM document_admission WHERE admission_id=?',
        [id],
      );
      admission = { id, original };
      assert.equal((await ready()).status, 200);
      for (const altered of [
        'UPDATE document_admission SET actor_id=actor_id+1 WHERE admission_id=?',
        'UPDATE document_admission SET unit_id=NULL WHERE admission_id=?',
        "UPDATE document_admission SET target_json=JSON_SET(target_json,'$.purpose','OTHER_OPERATION') WHERE admission_id=?",
      ]) {
        await editor.query(altered, [id]);
        assert.equal(
          (await ready()).status,
          503,
          'changed confirmation admission must fail readiness',
        );
        await editor.query(
          'UPDATE document_admission SET actor_id=?,unit_id=?,target_json=? WHERE admission_id=?',
          [original.actor_id, original.unit_id, original.target, id],
        );
        assert.equal((await ready()).status, 200);
      }
      await holder.rollback();
      held = false;
      restartOwnedRuntime();
      stopped = false;
      assert.equal((await pollDiscardDraft(input.operation_id, f.user.headers)).state, 'SUCCEEDED');
    } finally {
      if (admission)
        await editor.query(
          'UPDATE document_admission SET actor_id=?,unit_id=?,target_json=? WHERE admission_id=?',
          [
            admission.original.actor_id,
            admission.original.unit_id,
            admission.original.target,
            admission.id,
          ],
        );
      if (held) await holder.rollback();
      f.close();
      await holder.end();
      await editor.end();
      if (stopped) restartOwnedRuntime();
    }
  },
);
