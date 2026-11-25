import test from 'node:test';
import assert from 'node:assert/strict';
import { organizationFixture, documentFixture, get, post, json } from '../helpers/workspace.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';

const maximum = 18446744073709551615n;

// 只把新建夹具的起始水位设到边界；这不是声称实际重放了2^64条事件。
// 发布、真实旧来源关闭、HTTP重放与最终SQL结果仍必须走产品路径，且不经过Number/int64。
test('actual publication reaches uint64 maximum exactly and refuses a new operation without wrapping', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    db = await reviewDatabase();
  try {
    await db.query('UPDATE document SET publication_epoch=?,event_seq=? WHERE id=?', [
      String(maximum - 1n),
      String(maximum - 1n),
      d.id,
    ]);
    const input = await releaseInput(d.id, f.owner);
    assert.equal(input.expected_publication_revision, String(maximum - 1n));
    const result = await publishRelease(d.id, f.owner, input);
    const metadata = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
    assert.equal(metadata.publication_epoch, String(maximum));
    assert.equal(metadata.current_release_id, result.result.entity_id);
    const [[row]] = await db.query(
      'SELECT CAST(d.event_seq AS CHAR) seq,CAST(o.aggregate_seq AS CHAR) event_seq,o.payload FROM document d JOIN outbox o ON o.aggregate_id=d.id AND o.event_id=? WHERE d.id=?',
      [input.operation_id, d.id],
    );
    assert.equal(row.seq, String(maximum));
    assert.equal(row.event_seq, String(maximum));
    const payload = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
    assert.equal(payload.aggregate_seq, String(maximum));
    assert.equal(payload.publication_epoch, String(maximum));
    assert.equal(payload.release_id, result.result.entity_id);
    const replay = await post(`/v1/documents/${d.id}/releases`, input, f.owner.headers);
    assert.equal(replay.status, 202, json(replay).code);
    assert.ok(JSON.stringify(json(replay)) === JSON.stringify(result));
    const denied = await post(
      `/v1/documents/${d.id}/releases`,
      await releaseInput(d.id, f.owner),
      f.owner.headers,
    );
    assert.equal(denied.status, 409);
    assert.equal(json(denied).code, 'PUBLICATION_SEQUENCE_EXHAUSTED');
    const [[counts]] = await db.query(
      "SELECT (SELECT COUNT(*) FROM `release` WHERE document_id=?) releases,(SELECT COUNT(*) FROM outbox WHERE aggregate_id=? AND aggregate_kind='DOCUMENT') events",
      [d.id, d.id],
    );
    assert.equal(Number(counts.releases), 1);
    assert.equal(Number(counts.events), 1);
  } finally {
    await db.end();
  }
});

test('event sequence exhaustion rolls back release, captured versions, manifest, pointer and outbox together', async () => {
  const f = await organizationFixture(0),
    d = await documentFixture(f),
    db = await reviewDatabase();
  try {
    await db.query('UPDATE document SET event_seq=? WHERE id=?', [String(maximum), d.id]);
    const before = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
    const input = await releaseInput(d.id, f.owner);
    const response = await post(`/v1/documents/${d.id}/releases`, input, f.owner.headers);
    assert.equal(response.status, 202, json(response).code);
    const result = await pollPublication(input.operation_id, f.owner);
    assert.equal(result.state, 'REJECTED');
    assert.equal(result.error.code, 'PUBLICATION_SEQUENCE_EXHAUSTED');
    const after = json(await get(`/v1/documents/${d.id}`, f.owner.headers));
    for (const field of [
      'current_release_id',
      'publication_state',
      'publication_epoch',
      'current_manifest_id',
    ])
      assert.deepEqual(after[field], before[field]);
    const [[counts]] = await db.query(
      'SELECT (SELECT COUNT(*) FROM `release` WHERE operation_id=?) releases,(SELECT COUNT(*) FROM main_manifest WHERE source_operation_id=?) manifests,(SELECT COUNT(*) FROM content_version WHERE source_operation_id=?) versions,(SELECT COUNT(*) FROM outbox WHERE event_id=?) events',
      Array(4).fill(input.operation_id),
    );
    for (const count of Object.values(counts)) assert.equal(Number(count), 0);
    const [[current]] = await db.query(
      'SELECT CAST(event_seq AS CHAR) seq FROM document WHERE id=?',
      [d.id],
    );
    assert.equal(current.seq, String(maximum));
  } finally {
    await db.end();
  }
});
