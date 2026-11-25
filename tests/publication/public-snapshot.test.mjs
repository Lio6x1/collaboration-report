import test from 'node:test';
import assert from 'node:assert/strict';
import {
  reviewFixture,
  openReviewWorkspace,
  writeDraft,
  reviewDatabase,
} from '../helpers/review.mjs';
import { get, post, json, operation } from '../helpers/workspace.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';
import { documentRPC, rpcCall } from '../helpers/collaboration.mjs';
import { publicSnapshotRPC } from '../helpers/public-snapshot.mjs';

// 事件的旧release/seq只是提示；当前正文、标题和水位只能由Doc同一事务的真实正式指针提供。
test('actual Search mTLS snapshots use current authority and never disclose MAIN or fall back to old releases', async () => {
  const f = await reviewFixture(),
    peer = publicSnapshotRPC(),
    wrong = documentRPC(),
    db = await reviewDatabase();
  let main;
  try {
    const input = { document_id: f.d.id, expected_release_id: '0', aggregate_seq: '0' };
    await assert.rejects(
      () => rpcCall(wrong, 'GetPublicSnapshot', input),
      (error) => error.code === 7,
    );
    const empty = await rpcCall(peer, 'GetPublicSnapshot', input);
    assert.equal(empty.document_id, f.d.id);
    assert.equal(empty.visible, false);
    assert.equal(empty.release_id, '0');
    assert.deepEqual(empty.units, []);
    await unlockStructurePreview(f);
    main = await openReviewWorkspace(f.f, f.d, f.user, 'MAIN');
    await writeDraft(main, 'Search只取得固定正式S1');
    const first = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
    const original = await rpcCall(peer, 'GetPublicSnapshot', input);
    assert.equal(original.visible, true);
    assert.equal(original.release_id, first.result.entity_id);
    assert.equal(original.title, '协作报告');
    assert.equal(original.units.length, 1);
    assert.ok(original.units[0].content.body_json_utf8.toString().includes('固定正式S1'));
    const [[facts]] = await db.query(
      'SELECT CAST(event_seq AS CHAR) seq,CAST(publication_epoch AS CHAR) epoch FROM document WHERE id=?',
      [f.d.id],
    );
    assert.equal(original.aggregate_seq, facts.seq);
    assert.equal(original.publication_epoch, facts.epoch);
    await writeDraft(main, '未发布的MAIN S2不得索引');
    const current = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const rename = await post(
      `/v1/documents/${f.d.id}`,
      {
        operation_id: operation('snapshot-rename'),
        expected_control_revision: current.control_revision,
        title: '下一版标题',
      },
      f.user.headers,
      'PATCH',
    );
    assert.equal(rename.status, 200, json(rename).code);
    assert.deepEqual(await rpcCall(peer, 'GetPublicSnapshot', input), original);
    const second = await publishRelease(f.d.id, f.user, await releaseInput(f.d.id, f.user));
    const latest = await rpcCall(peer, 'GetPublicSnapshot', {
      ...input,
      expected_release_id: original.release_id,
      aggregate_seq: original.aggregate_seq,
    });
    assert.equal(latest.release_id, second.result.entity_id);
    assert.equal(latest.title, '下一版标题');
    assert.ok(BigInt(latest.aggregate_seq) > BigInt(original.aggregate_seq));
    assert.ok(latest.units[0].content.body_json_utf8.toString().includes('MAIN S2'));
    const now = json(await get(`/v1/documents/${f.d.id}`, f.user.headers));
    const unpublish = {
      operation_id: operation('snapshot-unpublish'),
      expected_publication_revision: now.publication_epoch,
      action: 'UNPUBLISH',
    };
    const response = await post(`/v1/documents/${f.d.id}/publication`, unpublish, f.user.headers);
    assert.equal(response.status, 202, json(response).code);
    assert.equal((await pollPublication(unpublish.operation_id, f.user)).state, 'SUCCEEDED');
    const hidden = await rpcCall(peer, 'GetPublicSnapshot', {
      ...input,
      expected_release_id: original.release_id,
      aggregate_seq: original.aggregate_seq,
    });
    assert.equal(hidden.visible, false);
    assert.equal(hidden.release_id, '0');
    assert.deepEqual(hidden.units, []);
    assert.ok(BigInt(hidden.aggregate_seq) > BigInt(latest.aggregate_seq));
    await assert.rejects(
      () => rpcCall(peer, 'GetPublicSnapshot', { ...input, document_id: '18446744073709551615' }),
      (error) => error.code === 5,
    );
  } finally {
    main?.close();
    f.close();
    peer.close();
    wrong.close();
    await db.end();
  }
});
