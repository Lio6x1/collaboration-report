import test from 'node:test';
import assert from 'node:assert/strict';
import { reviewFixture, reviewDatabase, writeDraft } from '../helpers/review.mjs';
import { operation, post } from '../helpers/workspace.mjs';
import { pullInput, pullPath, pollPull, persistedPull } from '../helpers/pull-main.mjs';
import { request } from '../helpers/https.mjs';
import { state } from '../../scripts/runtime-lib.mjs';

test('pending pull readiness verifies the actual original admission actor and target', async () => {
  const f = await reviewFixture(),
    holder = await reviewDatabase(),
    editor = await reviewDatabase(),
    roomHolder = await reviewDatabase('collaboration');
  const input = pullInput(f, operation('admission-binding-pull'));
  let locked = false,
    admission,
    actor,
    target,
    roomLocked = false;
  const ready = () => request(state().ports.documentHttp, '/health/ready');
  try {
    await writeDraft(f, '原准入必须与这一份可靠草稿一致');
    // 房间锁挡住实际Node预览完成；登记本身不访问Node，后台仍可真实领取任务。
    await roomHolder.beginTransaction();
    roomLocked = true;
    await roomHolder.query('SELECT id FROM room WHERE id=? FOR UPDATE', [f.session.room_id]);
    assert.equal((await post(pullPath(f), input, f.user.headers)).status, 202);
    let observed;
    for (let n = 0; n < 30; n++) {
      const [[current]] = await editor.query(
        "SELECT state FROM durable_task WHERE kind='PULL_MAIN_RECOVER' AND business_key=?",
        [input.operation_id],
      );
      observed = current.state;
      if (observed === 'RUNNING') break;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    assert.equal(observed, 'RUNNING', '必须真实领取且仍被房间锁阻止完成，不能猜调度时机');
    await holder.beginTransaction();
    locked = true;
    const [[task]] = await holder.query(
      "SELECT state FROM durable_task WHERE kind='PULL_MAIN_RECOVER' AND business_key=? FOR UPDATE",
      [input.operation_id],
    );
    // 领取后的RUNNING同样是未决状态；房间锁与此任务行锁共同阻止它越过本次篡改核验。
    assert.ok(['QUEUED', 'RETRY', 'RUNNING'].includes(task.state), JSON.stringify(task));
    const [[context]] = await holder.query(
      "SELECT CAST(payload AS CHAR) AS raw FROM durable_task WHERE kind='PULL_MAIN_CONTEXT' AND business_key=?",
      [input.operation_id],
    );
    admission = persistedPull(context.raw).context.document_admission_ref;
    const [[original]] = await editor.query(
      'SELECT actor_id,CAST(target_json AS CHAR) AS target FROM document_admission WHERE admission_id=?',
      [admission],
    );
    actor = String(original.actor_id);
    target = original.target;
    assert.equal((await ready()).status, 200);
    // 只篡改本测试新建操作的事实；工作线程仍被真实SQL行锁阻挡，finally恢复原记录。
    await editor.query('UPDATE document_admission SET actor_id=actor_id+1 WHERE admission_id=?', [
      admission,
    ]);
    assert.equal((await ready()).status, 503, 'substituted original actor must fail readiness');
    await editor.query('UPDATE document_admission SET actor_id=? WHERE admission_id=?', [
      actor,
      admission,
    ]);
    assert.equal((await ready()).status, 200);
    await editor.query(
      "UPDATE document_admission SET target_json=JSON_SET(target_json,'$.purpose','ANOTHER_OPERATION') WHERE admission_id=?",
      [admission],
    );
    assert.equal((await ready()).status, 503, 'substituted original target must fail readiness');
    await editor.query('UPDATE document_admission SET target_json=? WHERE admission_id=?', [
      target,
      admission,
    ]);
    assert.equal((await ready()).status, 200);
    await holder.commit();
    locked = false;
    await roomHolder.rollback();
    roomLocked = false;
    assert.equal((await pollPull(input.operation_id, f.user.headers)).state, 'SUCCEEDED');
  } finally {
    try {
      if (admission && actor && target)
        await editor.query(
          'UPDATE document_admission SET actor_id=?,target_json=? WHERE admission_id=?',
          [actor, target, admission],
        );
    } finally {
      if (locked) await holder.rollback();
      if (roomLocked) await roomHolder.rollback();
      f.close();
      await holder.end();
      await editor.end();
      await roomHolder.end();
    }
  }
});
