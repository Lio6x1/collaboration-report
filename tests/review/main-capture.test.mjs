import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { reviewFixture, collaborationRPC, reviewDatabase } from '../helpers/review.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { operation } from '../helpers/workspace.mjs';

// mTLS服务身份并不允许伪造Doc操作上下文；没有原号事实就不能固定可安装的全篇主线。
test('main capture requires an actual registered document merge context', async () => {
  const f = await reviewFixture(),
    peer = collaborationRPC(),
    db = await reviewDatabase('collaboration');
  const id = operation('unknown-main-capture');
  try {
    const input = {
      context: {
        ...f.grant.context,
        operation_id: id,
        identity_admission_ref: operation('unknown-identity'),
        target: { ...f.grant.context.target, purpose: 'MERGE' },
        request_fingerprint: createHash('sha256').update(id).digest(),
      },
      expected_manifest_id: f.d.main.manifest_id,
      expected_control_revision: '1',
      expected_structure_id: f.grant.context.target.structure_id,
      expected_style_id: f.d.main.style.id,
      usage: 'MERGE_ATTEMPT:1',
    };
    await assert.rejects(rpcCall(peer, 'CaptureMainView', input), (err) => err.code === 14);
    const [[fact]] = await db.query(
      "SELECT COUNT(*) AS captures FROM durable_task WHERE kind='MAIN_VIEW_FIXED' AND operation_id=?",
      [id],
    );
    assert.equal(Number(fact.captures), 0);
  } finally {
    peer.close();
    f.close();
    await db.end();
  }
});
