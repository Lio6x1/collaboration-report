import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  writeDraft,
  submitDraft,
  reviewDatabase,
  collaborationRPC,
} from '../helpers/review.mjs';
import { post, json, operation } from '../helpers/workspace.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import { pollMerge } from '../helpers/merge.mjs';

// 私有夹具ID必须仍是安全整数；把Go持久JSON转换为固定protobuf wire，字节字段保持原样。
function persistedRPC(value, key = '') {
  if (value == null) return value;
  if (Array.isArray(value)) return value.map((item) => persistedRPC(item));
  if (typeof value === 'object')
    return Object.fromEntries(
      Object.entries(value).map(([name, item]) => [name, persistedRPC(item, name)]),
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
    ].includes(key)
  )
    return Buffer.from(value, 'base64');
  return value;
}

// 验证真实完整事务与Node安装，不能用纯合并引擎或登记202代替业务成功。
test('actual approve-and-merge commits fixed S1, installs main and preserves S2 for the next submission', async () => {
  const f = await reviewFixture(),
    db = await reviewDatabase();
  try {
    await writeDraft(f, '本轮固定送审S1');
    const submitted = await submitDraft(f),
      fixed = submitted.value.result;
    await writeDraft(f, '可靠保存但未送审的S2');
    const id = operation('complete-merge'),
      path = `/v1/review-requests/${fixed.request_id}/approve-and-merge`;
    const input = {
      operation_id: id,
      round_id: fixed.round_id,
      content_version_id: fixed.content_version_id,
      own_changes_to_resolve: [],
    };
    const made = await post(path, input, f.f.owner.headers);
    assert.equal(made.status, 202, json(made).code);
    const value = await pollMerge(id, f.f.owner.headers, db);
    assert.equal(value.state, 'SUCCEEDED', value.error?.code);
    assert.equal(value.result.main_sync_state, 'READY');
    const [[facts]] = await db.query(
      "SELECT r.status,(SELECT COUNT(*) FROM review_action a WHERE a.operation_id=? AND a.kind='APPROVE_AND_MERGE') AS approvals,(SELECT COUNT(*) FROM merge_record m WHERE m.operation_id=?) AS records FROM review_request r WHERE r.id=?",
      [id, id, fixed.request_id],
    );
    assert.equal(facts.status, 'MERGED');
    assert.equal(Number(facts.approvals), 1);
    assert.equal(Number(facts.records), 1);
    const [[body]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      value.result.content_version_id,
    ]);
    assert.match(body.canonical_body.toString(), /本轮固定送审S1/);
    assert.doesNotMatch(body.canonical_body.toString(), /可靠保存但未送审的S2/);
    const repeated = await post(path, input, f.f.owner.headers);
    assert.equal(repeated.status, 202, json(repeated).code);
    assert.deepEqual(json(repeated).result, value.result);
    // 原WSS工作区仍能送审S2，接纳来源不重置共同草稿，也不沿用已合并请求。
    const next = (await submitDraft(f)).value;
    assert.equal(next.state, 'SUCCEEDED', next.error?.code);
    assert.notEqual(next.result.request_id, fixed.request_id);
    const [[lineage]] = await db.query('SELECT base_version_id FROM review_round WHERE id=?', [
      next.result.round_id,
    ]);
    assert.equal(String(lineage.base_version_id), fixed.content_version_id);
    const [[second]] = await db.query('SELECT canonical_body FROM content_version WHERE id=?', [
      next.result.content_version_id,
    ]);
    assert.match(second.canonical_body.toString(), /可靠保存但未送审的S2/);
    // 后续S2也真实合并到更高epoch，再重送原S1安装，证明丢ACK重试只读旧receipt而不回滚新主线。
    const secondID = operation('second-merge');
    assert.equal(
      (
        await post(
          `/v1/review-requests/${next.result.request_id}/approve-and-merge`,
          {
            operation_id: secondID,
            round_id: next.result.round_id,
            content_version_id: next.result.content_version_id,
            own_changes_to_resolve: [],
          },
          f.f.owner.headers,
        )
      ).status,
      202,
    );
    const later = await pollMerge(secondID, f.f.owner.headers, db);
    assert.equal(later.state, 'SUCCEEDED', later.error?.code);
    assert.notEqual(later.result.manifest_id, value.result.manifest_id);
    const [[row]] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='MAIN_COMMIT_FIXED' AND business_key=?",
      [id],
    );
    const installed = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
    const context = persistedRPC(installed.context),
      documentPeer = documentRPC(),
      nodePeer = collaborationRPC();
    try {
      const authority = await rpcCall(documentPeer, 'GetOperationResult', {
        operation_id: id,
        target: context.target,
        request_fingerprint: context.request_fingerprint,
      });
      const replay = await rpcCall(nodePeer, 'ReconcileMainCommit', {
        context,
        barrier_generation: String(installed.generation),
        authoritative_result: authority,
        manifest_id: String(installed.manifest),
        manifest_digest: Buffer.from(installed.digest, 'base64'),
        units: persistedRPC(installed.units),
      });
      assert.equal(replay.state, 'READY');
      assert.equal(replay.manifest_id, value.result.manifest_id);
    } finally {
      documentPeer.close();
      nodePeer.close();
    }
    const [[currentMain]] = await db.query('SELECT current_manifest_id FROM document WHERE id=?', [
      f.d.id,
    ]);
    assert.equal(String(currentMain.current_manifest_id), later.result.manifest_id);
  } finally {
    f.close();
    await db.end();
  }
});
