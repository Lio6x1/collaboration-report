import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { canonical, normalizeContent, resourceIds } from '../../packages/content/index.mjs';
import { operation, get, json } from '../helpers/workspace.mjs';
import {
  reviewFixture,
  reviewDatabase,
  writeDraft,
  flushDraft,
  captureInput,
  collaborationRPC,
} from '../helpers/review.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { mergeInputDigest } from '../../services/collaboration/src/domain/merge_protocol.mjs';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';
import { createStore } from '../../services/collaboration/src/bootstrap/store.mjs';
import { Transactions } from '../../services/collaboration/src/repository/sql.mjs';
import { MergeCandidates } from '../../services/collaboration/src/application/merge_candidates.mjs';
import { MergeExecutor } from '../../services/collaboration/src/application/merge_executor.mjs';
import { restartOwnedRuntime } from '../helpers/owned-runtime.mjs';

const sha = (value) => createHash('sha256').update(value).digest();
const p = (id, value) => ({
  type: 'paragraph',
  attrs: { id },
  content: [{ type: 'text', text: value }],
});

test('an actual fixed attempt recovers after result transaction failure without using later draft', async () => {
  const value = await fixture();
  const store = createStore(
    JSON.parse(readFileSync(resolve(runtime, 'services/collaboration.json'))),
  );
  try {
    const transactions = new Transactions(store.pool);
    let calls = 0;
    // 只替换第二事务依赖；原生产仓储和第一次真实MySQL提交保持，不能直接伪造FIXED状态。
    const faulty = {
      run(body) {
        if (++calls === 2) throw Error('INJECTED_MERGE_RESULT_UNAVAILABLE');
        return transactions.run(body);
      },
    };
    value.request.input_digest = mergeInputDigest(value.request);
    await assert.rejects(
      () => new MergeCandidates(faulty, new MergeExecutor()).compute(value.request),
      /INJECTED_MERGE_RESULT_UNAVAILABLE/,
    );
    const business = value.request.context.operation_id + ':' + value.request.attempt_id;
    const [rows] = await store.pool.query(
      "SELECT kind FROM durable_task WHERE business_key=? AND kind IN ('MERGE_ATTEMPT_CONTEXT','MERGE_CANDIDATE_RESULT')",
      [business],
    );
    assert.deepEqual(
      rows.map((row) => row.kind),
      ['MERGE_ATTEMPT_CONTEXT'],
    );
    await writeDraft(value.f, '结果失败后到来的草稿S2');
    const reply = await rpcCall(value.peer, 'ComputeMergeCandidate', value.request);
    assert.equal(reply.state, 'FIXED');
    const body = Buffer.from(reply.candidate.body_json_utf8).toString('utf8');
    assert.match(body, /主线后来修改/);
    assert.match(body, /本轮新增/);
    assert.doesNotMatch(body, /结果失败后到来的草稿S2/);
    assert.deepEqual(await rpcCall(value.peer, 'ComputeMergeCandidate', value.request), reply);
  } finally {
    value.close();
    await store.pool.end();
  }
});

test('actual conflicting candidate stays fixed and never supplies installable content', async () => {
  const value = await fixture();
  try {
    const bytes = Buffer.from(
      canonical(normalizeContent({ type: 'doc', content: [p('base-p', '送审不同替换')] })),
    );
    value.request.submitted = {
      ...value.request.submitted,
      body_json_utf8: bytes,
      body_hash: sha(bytes),
      byte_size: String(bytes.length),
    };
    value.request.input_digest = mergeInputDigest(value.request);
    const result = await rpcCall(value.peer, 'ComputeMergeCandidate', value.request);
    assert.equal(result.state, 'FIXED');
    assert.equal(result.candidate, null);
    assert.ok(result.conflicts.some((conflict) => conflict.kind === 'TEXT_OVERLAP'));
    assert.deepEqual(await rpcCall(value.peer, 'ComputeMergeCandidate', value.request), result);
  } finally {
    value.close();
  }
});

test('actual owner service restart replays the same fixed merge result', async () => {
  const value = await fixture();
  let peer = value.peer;
  try {
    value.request.input_digest = mergeInputDigest(value.request);
    const before = await rpcCall(peer, 'ComputeMergeCandidate', value.request);
    peer.close();
    value.f.w.close();
    restartOwnedRuntime();
    peer = collaborationRPC();
    assert.deepEqual(await rpcCall(peer, 'ComputeMergeCandidate', value.request), before);
  } finally {
    peer.close();
    value.close();
  }
});
// 候选RPC属于可信Doc计算适配；它不代表用户已经通过审核，实际最终事务另有产品集成用例。
async function fixture() {
  const f = await reviewFixture(),
    peer = collaborationRPC();
  await writeDraft(f, '固定送审内容');
  const input = await captureInput(f, await flushDraft(f));
  const { content: submitted } = await rpcCall(peer, 'CaptureSubmission', input);
  const baseBody = { type: 'doc', content: [p('base-p', '共同基线')] };
  const mainBody = { type: 'doc', content: [p('base-p', '主线后来修改')] };
  const fixed = (body) => {
    const bytes = Buffer.from(canonical(normalizeContent(body)));
    return {
      ...submitted,
      body_json_utf8: bytes,
      body_hash: sha(bytes),
      byte_size: String(bytes.length),
      resource_ids: resourceIds(body),
    };
  };
  // 三份正文固定输入由Doc服务给出；实际权限准入/版本身份的最终核对属于Go合并入口。
  const request = {
    context: {
      ...input.context,
      operation_id: operation('merge'),
      target: { ...input.context.target, purpose: 'MERGE' },
    },
    attempt_id: '1',
    base: fixed(baseBody),
    submitted: fixed({ type: 'doc', content: [p('base-p', '共同基线'), p('new-p', '本轮新增')] }),
    main: fixed(mainBody),
    main_cut: {
      document_id: f.d.id,
      structure_id: submitted.structure_id,
      style_id: f.d.main.style.id,
      control_revision: json(await get('/v1/documents/' + f.d.id, f.user.headers)).control_revision,
      rooms: [],
      dependency_digest: sha('no-main-rooms'),
    },
    input_digest: sha('initial-red-input'),
  };
  return {
    f,
    peer,
    request,
    close() {
      peer.close();
      f.close();
    },
  };
}
test('actual document mTLS merge RPC fixes one candidate and rejects an altered attempt', async () => {
  const fixtureValue = await fixture();
  try {
    // 输入摘要使用生产协议；候选正文预期独立写出，不调用合并算法生成答案。
    const request = fixtureValue.request;
    request.input_digest = mergeInputDigest(request);
    const reply = await rpcCall(fixtureValue.peer, 'ComputeMergeCandidate', request);
    assert.equal(reply.state, 'FIXED');
    assert.deepEqual(reply.conflicts, []);
    assert.deepEqual(JSON.parse(Buffer.from(reply.candidate.body_json_utf8)), {
      type: 'doc',
      content: [p('base-p', '主线后来修改'), p('new-p', '本轮新增')],
    });
    await writeDraft(fixtureValue.f, '送审后继续起草');
    assert.deepEqual(await rpcCall(fixtureValue.peer, 'ComputeMergeCandidate', request), reply);
    const changed = { ...request, main: request.submitted };
    changed.input_digest = mergeInputDigest(changed);
    await assert.rejects(
      () => rpcCall(fixtureValue.peer, 'ComputeMergeCandidate', changed),
      (error) => error.code === 6,
    );
    const invalid = {
      ...request,
      attempt_id: '2',
      main: { ...request.main, body_hash: Buffer.alloc(32, 1) },
    };
    invalid.input_digest = mergeInputDigest(invalid);
    await assert.rejects(
      () => rpcCall(fixtureValue.peer, 'ComputeMergeCandidate', invalid),
      (error) => error.code === 3,
    );
    const db = await reviewDatabase('collaboration');
    try {
      const [[row]] = await db.query(
        "SELECT COUNT(*) AS n FROM durable_task WHERE kind='MERGE_CANDIDATE_RESULT' AND business_key=?",
        [request.context.operation_id + ':' + request.attempt_id],
      );
      assert.equal(String(row.n), '1');
    } finally {
      await db.end();
    }
  } finally {
    fixtureValue.close();
  }
});
