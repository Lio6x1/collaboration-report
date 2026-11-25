import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, pause } from '../../scripts/runtime-lib.mjs';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';
import {
  searchDatabase,
  searchConfiguration,
  waitSearchReceipt,
  searchReceipt,
  searchProjection,
} from '../helpers/search.mjs';

test('封存后的真实发布独立重放到目标，普通消费事实与当前代次保持', { timeout: 90000 }, async () => {
  const original = JSON.parse(
    readFileSync(resolve(root, 'artifacts/batches/06/rebuild-prepare-operation.json')),
  );
  const db = await searchDatabase();
  try {
    const [[job]] = await db.query(
      `SELECT CAST(j.id AS CHAR) id,j.state,j.source_index,j.target_index FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.operation_id=?`,
      [original.operation],
    );
    assert.equal(job.state, 'REPLAYING');
    // 文档在S封存之后才创建；目标中出现它必须依靠真实增量，不能由种子覆盖冒充。
    const fixture = await organizationFixture(0);
    const document = await documentFixture(fixture);
    const input = await releaseInput(document.id, fixture.owner);
    const published = await publishRelease(document.id, fixture.owner, input);
    const normal = await waitSearchReceipt(db, input.operation_id);
    let replay;
    const deadline = Date.now() + 30000;
    while (Date.now() < deadline) {
      [[replay]] = await db.query(
        `SELECT e.state,o.applied_index physical,HEX(o.raw_digest) digest,
          o.kafka_partition partition_id,CAST(o.kafka_offset AS CHAR) record_offset,
          CAST(p.next_offset AS CHAR) next_offset
        FROM consumed_event e JOIN event_origin o ON o.consumer_name=e.consumer_name AND o.event_id=e.event_id
        JOIN consumer_checkpoint p ON p.consumer_name=o.consumer_name AND p.kafka_topic=o.kafka_topic AND p.partition_id=o.kafka_partition
        WHERE e.consumer_name=? AND e.event_id=?`,
        ['rebuild-' + job.id, input.operation_id],
      );
      if (
        replay?.state === 'SUCCEEDED' &&
        BigInt(replay.next_offset) > BigInt(replay.record_offset)
      )
        break;
      await pause(100);
    }
    assert.equal(replay?.state, 'SUCCEEDED', 'REBUILD_REAL_PUBLICATION_NOT_REPLAYED');
    assert.ok(BigInt(replay.next_offset) > BigInt(replay.record_offset));
    assert.equal(replay.physical, job.target_index);
    assert.equal(replay.digest, normal.raw_digest);
    assert.equal(replay.partition_id, normal.partition_id);
    assert.equal(replay.record_offset, normal.record_offset);
    const projected = await searchProjection(document.id, job.target_index);
    assert.equal(projected.visible, true);
    assert.equal(projected.release_id, published.result.entity_id);
    // 两套消费账本共享原消息身份，但不能覆盖普通消费者的原物理索引事实。
    const unchanged = await searchReceipt(db, input.operation_id);
    assert.equal(unchanged.physical, job.source_index);
    assert.equal(unchanged.raw_digest, normal.raw_digest);
    const [[active]] = await db.query(
      'SELECT physical_index FROM index_generation WHERE alias_name=?',
      [searchConfiguration().search.alias],
    );
    assert.equal(active.physical_index, job.source_index);
  } finally {
    await db.end();
  }
});
