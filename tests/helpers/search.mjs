import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createHash } from 'node:crypto';
import mysql from 'mysql2/promise';
import { runtime, pause } from '../../scripts/runtime-lib.mjs';
import { kafkaTool } from './kafka.mjs';

// 测试使用Search自己的私有账号；不能借Doc连接读取Search账本，也不输出配置。
export function searchConfiguration() {
  return JSON.parse(readFileSync(resolve(runtime, 'services/search.json')));
}

export async function searchDatabase() {
  return mysql.createConnection(searchConfiguration().database);
}

export async function searchReceipt(database, event) {
  const { search } = searchConfiguration();
  const [rows] = await database.query(
    `SELECT e.state,o.kafka_topic topic,o.kafka_partition partition_id,
      CAST(o.kafka_offset AS CHAR) record_offset,o.original_row,
      HEX(o.raw_digest) raw_digest,o.applied_index physical,
      CAST(o.applied_seq AS CHAR) sequence,HEX(o.authority_digest) authority_digest,
      CAST(p.next_offset AS CHAR) next_offset,
      CAST(a.revision AS CHAR) application_revision
    FROM consumed_event e JOIN event_origin o
      ON o.consumer_name=e.consumer_name AND o.event_id=e.event_id
    LEFT JOIN consumer_checkpoint p ON p.consumer_name=o.consumer_name
      AND p.kafka_topic=o.kafka_topic AND p.partition_id=o.kafka_partition
    LEFT JOIN index_application a ON a.physical_index=o.applied_index
      AND a.document_id=e.document_id
    WHERE e.consumer_name=? AND e.event_id=?`,
    [search.group, event],
  );
  assert.ok(rows.length <= 1, 'SEARCH_RECEIPT_NOT_UNIQUE');
  return rows[0];
}

export async function waitSearchReceipt(database, event, timeout = 20000) {
  const deadline = Date.now() + timeout;
  let receipt;
  while (Date.now() < deadline) {
    receipt = await searchReceipt(database, event);
    if (
      receipt?.state === 'SUCCEEDED' &&
      receipt.next_offset !== null &&
      BigInt(receipt.next_offset) > BigInt(receipt.record_offset)
    )
      return receipt;
    await pause(100);
  }
  assert.fail(`SEARCH_PRODUCT_CONSUMPTION_INCOMPLETE ${receipt?.state ?? 'NOT_RECEIVED'}`);
}

export async function searchProjection(document, physical) {
  const { search } = searchConfiguration();
  assert.match(document, /^[1-9][0-9]*$/);
  assert.match(physical, /^[a-z0-9][a-z0-9._-]{0,159}$/);
  const response = await fetch(`${search.elasticEndpoint}/${physical}/_doc/${document}`, {
    signal: AbortSignal.timeout(3000),
  });
  assert.equal(response.status, 200, 'ACTUAL_SEARCH_PROJECTION_UNAVAILABLE');
  // 实际投影含整篇文本和逐章文本，允许既定64MiB ES响应预算；
  // 容量测试仍完整读到末尾，但不能无界缓冲，也不能只比截断前缀。
  const reader = response.body.getReader();
  const chunks = [];
  let bytes = 0;
  try {
    while (true) {
      const part = await reader.read();
      if (part.done) break;
      bytes += part.value.byteLength;
      if (bytes > 64 << 20) {
        await reader.cancel();
        assert.fail('ACTUAL_SEARCH_PROJECTION_RESPONSE_LIMIT');
      }
      chunks.push(Buffer.from(part.value));
    }
  } finally {
    reader.releaseLock();
  }
  const value = JSON.parse(Buffer.concat(chunks, bytes).toString('utf8'));
  assert.equal(value.found, true);
  assert.equal(value._source.document_id, document);
  return value._source;
}

// SQL成功只证明本地事实；另向真实broker查询组offset，不能拿SQL检查点冒充Kafka提交。
export async function assertKafkaCommitted(receipt) {
  const { search } = searchConfiguration();
  const deadline = Date.now() + 15000;
  while (Date.now() < deadline) {
    const output = kafkaTool('kafka-consumer-groups.sh', '--describe', '--group', search.group);
    for (const line of output.split(/\r?\n/)) {
      const columns = line.trim().split(/\s+/);
      if (
        columns[0] === search.group &&
        columns[1] === search.topic &&
        columns[2] === String(receipt.partition_id) &&
        /^(0|[1-9][0-9]*)$/.test(columns[3]) &&
        BigInt(columns[3]) > BigInt(receipt.record_offset)
      )
        return;
    }
    await pause(100);
  }
  assert.fail('ACTUAL_KAFKA_COMMIT_NOT_OBSERVED');
}

export function kafkaCommittedOffset(partition) {
  const { search } = searchConfiguration();
  const output = kafkaTool('kafka-consumer-groups.sh', '--describe', '--group', search.group);
  for (const line of output.split(/\r?\n/)) {
    const columns = line.trim().split(/\s+/);
    if (
      columns[0] === search.group &&
      columns[1] === search.topic &&
      columns[2] === String(partition)
    ) {
      assert.match(columns[3], /^(0|[1-9][0-9]*)$/);
      return BigInt(columns[3]);
    }
  }
  assert.fail('ACTUAL_KAFKA_GROUP_POSITION_UNAVAILABLE');
}

export async function waitSearchState(database, event, predicate, timeout = 15000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const receipt = await searchReceipt(database, event);
    if (receipt && predicate(receipt)) return receipt;
    await pause(100);
  }
  assert.fail('ACTUAL_SEARCH_PHASE_NOT_OBSERVED');
}

export function assertOriginalReceipt(receipt, event, document) {
  const { search } = searchConfiguration();
  assert.equal(receipt.topic, search.topic);
  assert.equal(receipt.physical, search.initialIndex);
  const original = JSON.parse(receipt.original_row.toString('utf8'));
  assert.equal(original.event_id, event);
  assert.equal(original.aggregate_id, document);
  assert.equal(
    createHash('sha256').update(receipt.original_row).digest('hex').toUpperCase(),
    receipt.raw_digest,
    'ORIGINAL_CANAL_ROW_DIGEST_CHANGED',
  );
  assert.match(receipt.authority_digest, /^[0-9A-F]{64}$/);
}
