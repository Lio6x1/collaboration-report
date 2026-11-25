import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  writeDraft,
  openReviewWorkspace,
  collaborationRPC,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { rpcCall } from '../helpers/collaboration.mjs';
import { fromPM } from '../../packages/content/index.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';

test('explicit pull preserves reliable draft and installs a new branch epoch under the original operation', async () => {
  const f = await reviewFixture();
  let next;
  try {
    await writeDraft(f, '拉取仍须保留这份可靠草稿');
    const input = {
      operation_id: operation('pull-main'),
      expected_branch_id: f.session.branch_id,
      expected_lineage_id: f.session.lineage_id,
      expected_structure_id: f.grant.context.target.structure_id,
    };
    const path = `/v1/documents/${f.d.id}/units/${f.grant.context.target.unit_id}/pull-main`;
    const accepted = await post(path, input, f.user.headers);
    assert.equal(accepted.status, 202, accepted.body);
    const deadline = Date.now() + 20000;
    let value;
    do {
      const response = await get('/v1/operations/' + input.operation_id, f.user.headers);
      assert.equal(response.status, 200, response.body);
      value = json(response);
      if (!['PREPARING', 'WAITING'].includes(value.state)) break;
      await new Promise((resolve) => setTimeout(resolve, 100));
    } while (Date.now() < deadline);
    assert.equal(value.state, 'SUCCEEDED', JSON.stringify(value));
    assert.equal(typeof value.result.entity_id, 'string');
    next = await openReviewWorkspace(f.f, f.d);
    assert.equal(BigInt(next.session.workspace_epoch), BigInt(f.session.workspace_epoch) + 1n);
    assert.notEqual(next.session.lineage_id, f.session.lineage_id);
    assert.deepEqual(fromPM(yDocToProsemirrorJSON(next.doc)), fromPM(yDocToProsemirrorJSON(f.doc)));
    assert.equal((await post(path, input, f.user.headers)).status, 202);
    // 相同原号的期待值不能换成新工作区；重试必须返回原结果，不能再拉一次。
    const changed = await post(
      path,
      { ...input, expected_lineage_id: next.session.lineage_id },
      f.user.headers,
    );
    assert.equal(changed.status, 409);
    assert.equal(json(changed).code, 'OPERATION_ID_CONFLICT');
    const stale = await post(
      path,
      { ...input, operation_id: operation('stale-pull') },
      f.user.headers,
    );
    assert.equal(stale.status, 409);
    assert.equal(json(stale).code, 'LINEAGE_CHANGED');
  } finally {
    next?.close();
    f.close();
  }
});

test('draft preview requires a registered Go pull operation even with a real document certificate', async () => {
  const f = await reviewFixture(),
    peer = collaborationRPC();
  try {
    await assert.rejects(
      rpcCall(peer, 'CaptureDraftPreview', {
        context: {
          ...f.grant.context,
          operation_id: operation('unregistered-pull'),
          identity_admission_ref: operation('unknown-identity'),
          target: { ...f.grant.context.target, purpose: 'PULL_MAIN' },
          request_fingerprint: Buffer.alloc(32, 1),
        },
        expected_branch_id: f.session.branch_id,
        expected_structure_id: f.grant.context.target.structure_id,
        expected_lineage_id: f.session.lineage_id,
      }),
      (error) => error.code === 14,
    );
  } finally {
    peer.close();
    f.close();
  }
});
