import assert from 'node:assert/strict';
import { docker, ownedContainer } from '../../scripts/runtime-lib.mjs';

export const publicationTopic = 'collab-publications-v1';
export function kafkaTool(tool, ...arguments_) {
  return docker(
    'exec',
    ownedContainer('kafka'),
    '/opt/kafka/bin/' + tool,
    '--bootstrap-server',
    'localhost:29092',
    ...arguments_,
  );
}
export function kafkaOffsets() {
  const output = kafkaTool('kafka-get-offsets.sh', '--topic', publicationTopic, '--time', '-1');
  const offsets = new Map();
  for (const line of output.split(/\r?\n/)) {
    const match = /^collab-publications-v1:([0-2]):(0|[1-9][0-9]*)$/.exec(line);
    assert.ok(match, 'KAFKA_OFFSET_SHAPE_INVALID');
    offsets.set(Number(match[1]), BigInt(match[2]));
  }
  assert.equal(offsets.size, 3, 'KAFKA_PARTITION_COUNT_INVALID');
  return offsets;
}

// 只读本次操作前后固定offset窗口；不从topic起点读取历史业务，也不修改消费者组位置。
export function kafkaWindow(start, end) {
  const result = [];
  for (const [partition, offset] of start) {
    const count = end.get(partition) - offset;
    assert.ok(count >= 0n && count <= 128n, 'KAFKA_TEST_WINDOW_OUT_OF_BOUNDS');
    if (!count) continue;
    const output = kafkaTool(
      'kafka-console-consumer.sh',
      '--topic',
      publicationTopic,
      '--partition',
      String(partition),
      '--offset',
      String(offset),
      '--max-messages',
      String(count),
      '--timeout-ms',
      '5000',
      '--property',
      'print.partition=true',
      '--property',
      'print.offset=true',
      '--property',
      'print.value=true',
    );
    assert.ok(Buffer.byteLength(output) <= 1024 * 1024, 'KAFKA_TEST_BODY_OUT_OF_BOUNDS');
    const lines = output.split(/\r?\n/);
    assert.equal(BigInt(lines.length), count, 'KAFKA_TEST_WINDOW_INCOMPLETE');
    for (const line of lines) {
      const match = /^Partition:([0-2])\tOffset:(0|[1-9][0-9]*)\t(.+)$/.exec(line);
      assert.ok(match, 'KAFKA_TEST_RECORD_SHAPE_INVALID');
      result.push({
        partition: Number(match[1]),
        offset: BigInt(match[2]),
        value: JSON.parse(match[3]),
      });
    }
  }
  return result;
}
