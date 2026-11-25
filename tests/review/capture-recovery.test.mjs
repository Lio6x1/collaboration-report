import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';
import {
  reviewFixture,
  writeDraft,
  flushDraft,
  captureInput,
  collaborationRPC,
} from '../helpers/review.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { restartOwnedRuntime } from '../helpers/owned-runtime.mjs';
import { createStore } from '../../services/collaboration/src/bootstrap/store.mjs';
import { Transactions } from '../../services/collaboration/src/repository/sql.mjs';
import { RoomSnapshots as SnapshotRepository } from '../../services/collaboration/src/repository/room_snapshots.mjs';
import { RoomSnapshots } from '../../services/collaboration/src/application/room_snapshots.mjs';
import { SerializedRooms } from '../../services/collaboration/src/application/serialized_rooms.mjs';
import { SubmissionCaptures } from '../../services/collaboration/src/application/submission_captures.mjs';

test('actual committed CUT_FIXED survives a failure before candidate generation and excludes later S2', async () => {
  const f = await reviewFixture(),
    peer = collaborationRPC();
  const store = createStore(
    JSON.parse(readFileSync(resolve(runtime, 'services/collaboration.json'))),
  );
  let lineageOriginal;
  try {
    await writeDraft(f, '固定后等待恢复的S1');
    const input = await captureInput(f, await flushDraft(f)),
      transactions = new Transactions(store.pool);
    let calls = 0;
    // 注入事务依赖故障，不直接INSERT/改写候选状态。前两次仍走实际服务用例和MySQL COMMIT。
    const fault = {
      run(body) {
        if (++calls === 3) throw Error('INJECTED_CANDIDATE_UNAVAILABLE');
        return transactions.run(body);
      },
    };
    const capture = new SubmissionCaptures(
      fault,
      new SerializedRooms(),
      new RoomSnapshots(new SnapshotRepository(store.pool), store),
      null,
    );
    await assert.rejects(() => capture.capture(input), /INJECTED_CANDIDATE_UNAVAILABLE/);
    const [[row]] = await store.pool.query(
      'SELECT state,canonical_body FROM capture_operation WHERE operation_id=?',
      [input.context.operation_id],
    );
    assert.equal(row.state, 'CUT_FIXED');
    assert.equal(row.canonical_body, null);
    // 注入来源上下文在同epoch发生后续修订，模拟接纳切点；原候选仍必须使用首次固定的来源。
    [[lineageOriginal]] = await store.pool.query(
      "SELECT CAST(payload AS CHAR) AS raw FROM durable_task WHERE kind='ROOM_LINEAGE_CONTEXT' AND business_key=?",
      [f.session.room_id + ':' + f.session.workspace_epoch],
    );
    await store.pool.query(
      "UPDATE durable_task SET payload=JSON_SET(payload,'$.lineage_id','987654321','$.base_version_id','987654321') WHERE kind='ROOM_LINEAGE_CONTEXT' AND business_key=?",
      [f.session.room_id + ':' + f.session.workspace_epoch],
    );
    await writeDraft(f, '恢复前到来的S2');
    const restored = await rpcCall(peer, 'GetCapture', {
      operation_id: input.context.operation_id,
      target: input.context.target,
      request_fingerprint: input.context.request_fingerprint,
    });
    assert.equal(restored.state, 'FIXED');
    const provenance = JSON.parse(
      Buffer.from(restored.content.provenance_json_utf8).toString('utf8'),
    );
    assert.equal(provenance.lineage_id, f.session.lineage_id);
    assert.equal(provenance.base_content_version_id, f.session.base_version_id);
    assert.match(
      Buffer.from(restored.content.body_json_utf8).toString('utf8'),
      /固定后等待恢复的S1/,
    );
    assert.doesNotMatch(
      Buffer.from(restored.content.body_json_utf8).toString('utf8'),
      /恢复前到来的S2/,
    );
  } finally {
    peer.close();
    f.close();
    // 只恢复本夹具的可写上下文；故障不能污染后续整个owner重启，原已固定捕获不改写。
    if (lineageOriginal)
      await store.pool.query(
        "UPDATE durable_task SET payload=? WHERE kind='ROOM_LINEAGE_CONTEXT' AND business_key=?",
        [lineageOriginal.raw, f.session.room_id + ':' + f.session.workspace_epoch],
      );
    await store.pool.end();
  }
});

test('actual owned service restart returns identical fixed capture bytes', async () => {
  const f = await reviewFixture();
  let peer = collaborationRPC();
  let restartPending = false;
  try {
    await writeDraft(f, '重启保持固定正文');
    const input = await captureInput(f, await flushDraft(f)),
      original = await rpcCall(peer, 'CaptureSubmission', input);
    peer.close();
    f.w.close();
    restartPending = true;
    restartOwnedRuntime();
    restartPending = false;
    peer = collaborationRPC();
    assert.deepEqual(
      await rpcCall(peer, 'GetCapture', {
        operation_id: input.context.operation_id,
        target: input.context.target,
        request_fingerprint: input.context.request_fingerprint,
      }),
      original,
    );
  } finally {
    peer.close();
    f.close();
    // 原启动失败仍使本例FAIL；恢复环境只是避免所有后续夹具因服务停止而连锁失败。
    if (restartPending) restartOwnedRuntime();
  }
});
