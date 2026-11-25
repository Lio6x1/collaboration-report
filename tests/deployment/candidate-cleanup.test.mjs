import test from 'node:test';
import assert from 'node:assert/strict';
import grpc from '@grpc/grpc-js';
import { createHash } from 'node:crypto';
import { pause } from '../../scripts/runtime-lib.mjs';
import {
  reviewFixture,
  writeDraft,
  submitDraft,
  reviewDatabase,
  collaborationRPC,
} from '../helpers/review.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import { post, json, operation } from '../helpers/workspace.mjs';
import { pollMerge } from '../helpers/merge.mjs';
import { createStore } from '../../services/collaboration/src/bootstrap/store.mjs';
import { Transactions } from '../../services/collaboration/src/repository/sql.mjs';
import { CandidateCleanup } from '../../services/collaboration/src/application/candidate_cleanup.mjs';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';

const sha = (bytes) => createHash('sha256').update(bytes).digest();
// SQL夹具只读本场景原号的服务持久上下文；恢复成protobuf整数/字节，不修改业务成功事实。
function wire(value, key = '') {
  if (value == null) return value;
  if (Array.isArray(value)) return value.map((item) => wire(item));
  if (typeof value === 'object')
    return Object.fromEntries(
      Object.entries(value).map(([name, item]) => [name, wire(item, name)]),
    );
  if (typeof value === 'number') {
    assert.ok(Number.isSafeInteger(value));
    return String(value);
  }
  if (
    [
      'request_fingerprint',
      'cut_json_utf8',
      'body_json_utf8',
      'body_hash',
      'provenance_json_utf8',
      'input_digest',
      'dependency_digest',
    ].includes(key)
  )
    return Buffer.from(value, 'base64');
  return value;
}

// 新增单候选GC的主要关口：真实Doc复制/收尾后才许可、Node原号清理，
// 清送审正文后原cut仍可供后续真正合并，迟到Capture/Compute不能复活已清副本。
test(
  'candidate cleanup retains unknown work and removes only proven duplicate bodies with durable replay',
  { timeout: 60000 },
  async () => {
    const f = await reviewFixture(),
      db = await reviewDatabase(),
      nodeDB = await reviewDatabase('collaboration');
    const doc = documentRPC(),
      node = collaborationRPC();
    try {
      const unknown = {
        operation_id: operation('unknown-candidate'),
        generation: '1',
        candidate_ref: '',
        candidate_digest: Buffer.alloc(32),
      };
      unknown.candidate_ref = 'capture:' + unknown.operation_id;
      assert.equal((await rpcCall(doc, 'AuthorizeCandidateCleanup', unknown)).state, 'UNKNOWN');
      await assert.rejects(
        rpcCall(node, 'CleanupCandidate', {
          binding: unknown,
          permit: {
            operation_id: unknown.operation_id,
            generation: '1',
            state: 'CLEANABLE',
            permit_digest: Buffer.alloc(32),
          },
        }),
        (error) => error.code === grpc.status.UNAVAILABLE,
      );

      await writeDraft(f, '候选清理仍能通过原切点合并');
      const submitted = await submitDraft(f),
        id = submitted.input.operation_id;
      let captured;
      const deadline = Date.now() + 10000;
      do {
        const [[row]] = await nodeDB.query(
          'SELECT state,CAST(generation AS CHAR) generation,body_hash,candidate_json FROM capture_operation WHERE operation_id=?',
          [id],
        );
        captured = row;
        const [[finalize]] = await db.query(
          "SELECT state FROM durable_task WHERE kind='CAPTURE_FINALIZE' AND business_key=?",
          [id],
        );
        if (row?.state === 'BOUND' && finalize?.state === 'SUCCEEDED') break;
        await pause(100);
      } while (Date.now() < deadline);
      assert.equal(captured.state, 'BOUND');
      const capture = {
        operation_id: id,
        generation: captured.generation,
        candidate_ref: 'capture:' + id,
        candidate_digest: captured.body_hash,
      };
      const permit = await rpcCall(doc, 'AuthorizeCandidateCleanup', capture);
      assert.equal(permit.state, 'CLEANABLE');
      assert.deepEqual(await rpcCall(doc, 'AuthorizeCandidateCleanup', capture), permit);
      await assert.rejects(
        rpcCall(node, 'CleanupCandidate', {
          binding: capture,
          permit: { ...permit, permit_digest: Buffer.alloc(32) },
        }),
        (error) => error.code === grpc.status.ALREADY_EXISTS,
      );
      const clean = await rpcCall(node, 'CleanupCandidate', { binding: capture, permit });
      assert.equal(clean.state, 'SUCCEEDED');
      assert.deepEqual(clean.result_digest, sha(clean.fixed_result_json_utf8));
      assert.deepEqual(
        await rpcCall(node, 'CleanupCandidate', { binding: capture, permit }),
        clean,
      );
      await assert.rejects(
        rpcCall(doc, 'AuthorizeCandidateCleanup', {
          ...capture,
          candidate_digest: Buffer.alloc(32),
        }),
        (error) => error.code === grpc.status.ALREADY_EXISTS,
      );
      const [[after]] = await nodeDB.query(
        'SELECT state,canonical_body,candidate_json,cut_ref FROM capture_operation WHERE operation_id=?',
        [id],
      );
      assert.equal(after.state, 'BOUND');
      assert.equal(after.canonical_body, null);
      assert.equal(after.candidate_json.cleaned, true);
      assert.equal(after.candidate_json.content.body_json_utf8, undefined);
      assert.equal(after.candidate_json.lineage, undefined);
      assert.deepEqual(
        after.candidate_json.content.cut_json_utf8,
        captured.candidate_json.content.cut_json_utf8,
      );
      const [[saved]] = await db.query(
        "SELECT payload FROM durable_task WHERE kind='SUBMISSION_RECOVER' AND business_key=?",
        [id],
      );
      await assert.rejects(
        rpcCall(node, 'CaptureSubmission', wire(saved.payload.capture)),
        (error) => error.code === grpc.status.FAILED_PRECONDITION,
      );

      // 这是受影响行为回归：清理不删committedCut依赖的原hash/cut/lineage，真正合并仍READY。
      const mergeID = operation('candidate-merge'),
        fixed = submitted.value.result;
      const response = await post(
        `/v1/review-requests/${fixed.request_id}/approve-and-merge`,
        {
          operation_id: mergeID,
          round_id: fixed.round_id,
          content_version_id: fixed.content_version_id,
          own_changes_to_resolve: [],
        },
        f.f.owner.headers,
      );
      assert.equal(response.status, 202, response.body);
      const merged = await pollMerge(mergeID, f.f.owner.headers, db);
      assert.equal(merged.state, 'SUCCEEDED', merged.error?.code);
      assert.equal(merged.result.main_sync_state, 'READY');
      const [[attempt]] = await db.query(
        'SELECT CAST(attempt_id AS CHAR) id,result_version_id FROM merge_record WHERE operation_id=?',
        [mergeID],
      );
      const [[candidate]] = await nodeDB.query(
        "SELECT payload FROM durable_task WHERE kind='MERGE_CANDIDATE_RESULT' AND business_key=?",
        [mergeID + ':' + attempt.id],
      );
      const merge = {
        operation_id: mergeID,
        generation: attempt.id,
        candidate_ref: 'merge:' + mergeID + ':' + attempt.id,
        candidate_digest: Buffer.from(candidate.payload.candidate.body_hash, 'base64'),
      };
      const mergePermit = await rpcCall(doc, 'AuthorizeCandidateCleanup', merge);
      assert.equal(mergePermit.state, 'CLEANABLE');
      // 真正提交删除与墓碑后丢弃一次调用回复；不手写SQL成功或假许可。
      // 正常RPC重试必须从同原号墓碑恢复结果，而不是重新选候选或再次执行删除。
      const store = createStore(
        JSON.parse(readFileSync(resolve(runtime, 'services/collaboration.json'))),
      );
      try {
        const transactions = new Transactions(store.pool);
        let calls = 0;
        const unknownReply = {
          async run(body) {
            const result = await transactions.run(body);
            if (++calls === 2) throw Error('INJECTED_CLEANUP_COMMIT_REPLY_UNKNOWN');
            return result;
          },
        };
        await assert.rejects(
          new CandidateCleanup(unknownReply, {
            rpc: (method, input) => rpcCall(doc, method, input),
          }).clean({ binding: merge, permit: mergePermit }),
          /INJECTED_CLEANUP_COMMIT_REPLY_UNKNOWN/,
        );
        assert.equal(calls, 2);
      } finally {
        await store.pool.end();
      }
      const mergeClean = await rpcCall(node, 'CleanupCandidate', {
        binding: merge,
        permit: mergePermit,
      });
      assert.equal(mergeClean.state, 'SUCCEEDED');
      assert.deepEqual(
        await rpcCall(node, 'CleanupCandidate', { binding: merge, permit: mergePermit }),
        mergeClean,
      );
      const [[remaining]] = await nodeDB.query(
        "SELECT COUNT(*) n FROM durable_task WHERE kind='MERGE_CANDIDATE_RESULT' AND business_key=?",
        [mergeID + ':' + attempt.id],
      );
      assert.equal(Number(remaining.n), 0);
      const [[inputs]] = await db.query(
        "SELECT payload FROM durable_task WHERE kind='MERGE_ATTEMPT_FIXED' AND business_key=?",
        [mergeID + ':' + attempt.id],
      );
      await assert.rejects(
        rpcCall(node, 'ComputeMergeCandidate', wire(inputs.payload.Input)),
        (error) => error.code === grpc.status.FAILED_PRECONDITION,
      );
      const [[original]] = await db.query(
        'SELECT canonical_body,body_hash FROM content_version WHERE id=?',
        [attempt.result_version_id],
      );
      assert.match(original.canonical_body.toString(), /候选清理仍能通过原切点合并/);
      assert.deepEqual(original.body_hash, sha(original.canonical_body));
      console.log(
        'CANDIDATE_CLEANUP_REAL_PASS: unknown retained; actual submitted and installed copies proven; original receipts; metadata supports later merge; late capture/compute cannot resurrect bodies',
      );
    } finally {
      f.close();
      doc.close();
      node.close();
      await db.end();
      await nodeDB.end();
    }
  },
);
