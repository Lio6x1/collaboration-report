import assert from 'node:assert/strict';
import { test } from 'node:test';
import { compose, docker, ownedContainer, pause } from '../../scripts/runtime-lib.mjs';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';
import { kafkaOffsets, kafkaWindow, kafkaTool } from '../helpers/kafka.mjs';

function kraftIdentity() {
  const contents = docker(
    'exec',
    ownedContainer('kafka'),
    'cat',
    '/tmp/kraft-combined-logs/meta.properties',
  );
  // Java Properties可转义cluster.id中的等号；保留原序列化值比较，不猜UUID编码。
  const cluster = /^cluster\.id=(.{1,100})$/m.exec(contents)?.[1];
  const node = /^node\.id=(\d+)$/m.exec(contents)?.[1];
  assert.ok(cluster && node, 'KRAFT_PERSISTENT_IDENTITY_MISSING');
  return { cluster, node };
}
async function waitEvent(start, id) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    const records = kafkaWindow(start, kafkaOffsets());
    const matches = records.flatMap((record) =>
      (record.value.data ?? [])
        .filter((row) => row.event_id === id)
        .map((row) => ({ record, row })),
    );
    if (matches.length) return matches;
    await pause(250);
  }
  assert.fail('CANAL_OFFLINE_EVENT_NOT_REPLAYED');
}
test(
  'Kafka与Canal保留原卷重建，停机期间提交的正式事件仍从原复制位置补取',
  { timeout: 180000 },
  async () => {
    const fixture = await organizationFixture(0);
    // 停机演练可能跨过Edge的keep-alive期限；每次请求独立TLS连接，避免复用已关闭socket。
    fixture.owner.headers = { ...fixture.owner.headers, connection: 'close' };
    const document = await documentFixture(fixture);
    const start = kafkaOffsets();
    const first = await releaseInput(document.id, fixture.owner);
    await publishRelease(document.id, fixture.owner, first);
    await waitEvent(start, first.operation_id);
    const identity = kraftIdentity();
    const oldKafka = ownedContainer('kafka'),
      oldCanal = ownedContainer('canal');
    const before = kafkaOffsets();
    let stopped = false;
    try {
      // 正常停机让Canal刷出复制位置；不清卷。第二次发布发生在两个依赖均停机之后。
      compose('stop', 'canal', 'kafka');
      stopped = true;
      const second = await releaseInput(document.id, fixture.owner);
      const published = await publishRelease(document.id, fixture.owner, second);
      compose('up', '-d', '--force-recreate', 'kafka');
      let ready = false;
      for (let attempt = 0; attempt < 80; attempt++) {
        try {
          kafkaTool('kafka-topics.sh', '--list');
          ready = true;
          break;
        } catch {}
        await pause(500);
      }
      assert.ok(ready, 'KAFKA_RECREATE_NOT_READY');
      assert.notEqual(ownedContainer('kafka'), oldKafka, 'KAFKA_WAS_NOT_RECREATED');
      assert.deepEqual(kraftIdentity(), identity, 'KRAFT_IDENTITY_CHANGED');
      const retained = kafkaOffsets();
      for (const [partition, offset] of before)
        assert.ok(retained.get(partition) >= offset, 'KAFKA_OFFSET_REWOUND');
      compose('up', '-d', '--force-recreate', 'canal');
      assert.notEqual(ownedContainer('canal'), oldCanal, 'CANAL_WAS_NOT_RECREATED');
      const messages = await waitEvent(before, second.operation_id);
      for (const { row } of messages) {
        const payload = JSON.parse(row.payload);
        assert.equal(row.aggregate_id, document.id);
        assert.equal(row.aggregate_seq, '2');
        assert.equal(payload.release_id, published.result.entity_id);
      }
      stopped = false;
    } finally {
      // 测试失败也恢复本owner依赖；保留全部已提交业务和失败证据，不能通过删卷制造通过。
      if (stopped) compose('up', '-d', 'kafka', 'canal');
    }
  },
);
