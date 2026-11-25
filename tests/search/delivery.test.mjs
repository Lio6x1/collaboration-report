import assert from 'node:assert/strict';
import { test } from 'node:test';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';
import { kafkaOffsets, kafkaWindow } from '../helpers/kafka.mjs';

test(
  '真实发布outbox经Canal到三分区Kafka，同文档顺序稳定且消息只有正式引用',
  { timeout: 120000 },
  async () => {
    const fixture = await organizationFixture(0);
    const documents = [];
    for (let index = 0; index < 4; index++) documents.push(await documentFixture(fixture));
    const start = kafkaOffsets();
    const events = new Map();
    for (let round = 0; round < 2; round++) {
      for (const document of documents) {
        const input = await releaseInput(document.id, fixture.owner);
        const result = await publishRelease(document.id, fixture.owner, input);
        events.set(input.operation_id, {
          document: document.id,
          sequence: String(round + 1),
          release: result.result.entity_id,
          epoch: String(BigInt(input.expected_publication_revision) + 1n),
        });
      }
    }
    let records = [];
    const deadline = Date.now() + 20000;
    while (Date.now() < deadline) {
      records = kafkaWindow(start, kafkaOffsets());
      const seen = new Set(
        records
          .flatMap((record) => record.value.data ?? [])
          .map((row) => row.event_id)
          .filter((id) => events.has(id)),
      );
      if (seen.size === events.size) break;
      await new Promise((resolve) => setTimeout(resolve, 250));
    }
    const seen = new Set(),
      partitions = new Map();
    for (const record of records) {
      const message = record.value;
      for (const row of message.data ?? []) {
        const expected = events.get(row.event_id);
        if (!expected) continue;
        // Canal flatMessage的row与payload必须绑定；不把JSON里的数字转成JS Number。
        assert.equal(message.database, 'document_db');
        assert.equal(message.table, 'outbox');
        assert.equal(message.type, 'INSERT');
        assert.equal(message.isDdl, false);
        assert.equal(row.aggregate_kind, 'DOCUMENT');
        assert.equal(row.aggregate_id, expected.document);
        assert.equal(row.aggregate_seq, expected.sequence);
        assert.equal(row.event_type, 'DOCUMENT_PUBLISHED');
        assert.equal(row.schema_version, '1');
        const payload = JSON.parse(row.payload);
        assert.deepEqual(Object.keys(payload).sort(), [
          'aggregate_seq',
          'document_id',
          'event_id',
          'event_type',
          'occurred_at',
          'publication_epoch',
          'release_id',
          'schema_version',
        ]);
        assert.equal(payload.event_id, row.event_id);
        assert.equal(payload.document_id, expected.document);
        assert.equal(payload.aggregate_seq, expected.sequence);
        assert.equal(payload.release_id, expected.release);
        assert.equal(payload.schema_version, '1');
        assert.equal(payload.event_type, 'DOCUMENT_PUBLISHED');
        assert.equal(payload.publication_epoch, expected.epoch);
        assert.ok(Number.isFinite(Date.parse(payload.occurred_at)));
        const previous = partitions.get(expected.document);
        if (seen.has(row.event_id)) {
          // Canal至少一次投递允许同事件重读；重复事件仍须原内容和同一分区。
          assert.equal(previous.partition, record.partition, 'DUPLICATE_PARTITION_CHANGED');
          continue;
        }
        if (previous) {
          assert.equal(previous.partition, record.partition, 'SAME_DOCUMENT_PARTITION_CHANGED');
          assert.ok(previous.offset < record.offset, 'DOCUMENT_ORDER_NOT_PRESERVED');
          assert.equal(previous.sequence, '1');
        }
        partitions.set(expected.document, {
          partition: record.partition,
          offset: record.offset,
          sequence: expected.sequence,
        });
        seen.add(row.event_id);
      }
    }
    assert.equal(seen.size, events.size, 'CANAL_DELIVERY_INCOMPLETE');
    assert.equal(partitions.size, documents.length);
    assert.equal(
      new Set([...partitions.values()].map((value) => value.partition)).size,
      3,
      'DOCUMENT_HASH_PARTITION_NOT_ACTIVE',
    );
  },
);
