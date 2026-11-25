import test from 'node:test';
import assert from 'node:assert/strict';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';
import {
  searchDatabase,
  searchConfiguration,
  waitSearchReceipt,
  searchProjection,
  assertKafkaCommitted,
} from '../helpers/search.mjs';

test('持久日志身份消费者完成真实发布，SQL落定后同步提交普通组', { timeout: 60000 }, async () => {
  const db = await searchDatabase();
  try {
    const { search } = searchConfiguration();
    const [bindings] = await db.query(
      'SELECT kafka_topic,cluster_id,HEX(topic_id) uuid,revision FROM consumer_log_identity WHERE consumer_name=?',
      [search.group],
    );
    assert.equal(bindings.length, 1);
    assert.equal(bindings[0].kafka_topic, search.topic);
    assert.match(bindings[0].uuid, /^[0-9A-F]{32}$/);
    const fixture = await organizationFixture(0);
    const document = await documentFixture(fixture);
    const input = await releaseInput(document.id, fixture.owner);
    const published = await publishRelease(document.id, fixture.owner, input);
    const receipt = await waitSearchReceipt(db, input.operation_id);
    const [[active]] = await db.query(
      'SELECT physical_index FROM index_generation WHERE alias_name=?',
      [search.alias],
    );
    assert.equal(receipt.physical, active.physical_index);
    assert.equal(
      (await searchProjection(document.id, receipt.physical)).release_id,
      published.result.entity_id,
    );
    await assertKafkaCommitted(receipt);
    const [after] = await db.query(
      'SELECT kafka_topic,cluster_id,HEX(topic_id) uuid,revision FROM consumer_log_identity WHERE consumer_name=?',
      [search.group],
    );
    assert.deepEqual(after, bindings);
  } finally {
    await db.end();
  }
});
