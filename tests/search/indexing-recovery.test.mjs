import assert from 'node:assert/strict';
import test from 'node:test';
import { compose, pause } from '../../scripts/runtime-lib.mjs';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';
import { stopOwnedRole, restartOwnedSearch } from '../helpers/owned-runtime.mjs';
import { kafkaOffsets, kafkaWindow } from '../helpers/kafka.mjs';
import {
  searchDatabase,
  searchConfiguration,
  waitSearchReceipt,
  waitSearchState,
  searchProjection,
  assertOriginalReceipt,
  assertKafkaCommitted,
  kafkaCommittedOffset,
  searchReceipt,
} from '../helpers/search.mjs';

test('实际ES停止时原消息未提交，Search进程退出后从同一消息恢复', { timeout: 90000 }, async () => {
  const fixture = await organizationFixture(0);
  const document = await documentFixture(fixture);
  const database = await searchDatabase();
  const initialOffsets = kafkaOffsets();
  let paused = false,
    stopped = false;
  try {
    compose('pause', 'es');
    paused = true;
    const input = await releaseInput(document.id, fixture.owner);
    const published = await publishRelease(document.id, fixture.owner, input);
    let original;
    const deadline = Date.now() + 20000;
    while (Date.now() < deadline) {
      original = kafkaWindow(initialOffsets, kafkaOffsets()).find((record) =>
        record.value.data?.some((row) => row.event_id === input.operation_id),
      );
      if (original) break;
      await pause(100);
    }
    assert.ok(original, 'ES_OFFLINE_ORIGINAL_CANAL_RECORD_NOT_DELIVERED');
    // 真实别名检查在原事件事务之前：ES不可达时可以尚未接收到账本。
    // 验证的是原消息未成功/未提交，不能错误要求它必须已进入APPLYING。
    const pending = await searchReceipt(database, input.operation_id);
    assert.notEqual(pending?.state, 'SUCCEEDED');
    assert.ok(kafkaCommittedOffset(original.partition) <= original.offset);
    stopOwnedRole('search');
    stopped = true;
    compose('unpause', 'es');
    paused = false;
    await restartOwnedSearch();
    stopped = false;
    const completed = await waitSearchReceipt(database, input.operation_id);
    assertOriginalReceipt(completed, input.operation_id, document.id);
    const projection = await searchProjection(document.id, completed.physical);
    assert.equal(projection.release_id, published.result.entity_id);
    await assertKafkaCommitted(completed);
  } finally {
    // 失败也恢复实际依赖/原owner进程，不通过删卷或推进组offset消除反例。
    try {
      if (paused) compose('unpause', 'es');
      if (stopped) await restartOwnedSearch();
    } finally {
      await database.end();
    }
  }
});

test(
  '实际SQL检查点阻塞后Search退出，已成功原事件重读不重增应用修订，再同步Kafka',
  { timeout: 90000 },
  async () => {
    const fixture = await organizationFixture(0);
    const document = await documentFixture(fixture);
    const database = await searchDatabase();
    const blocker = await searchDatabase();
    const { search } = searchConfiguration();
    let held = false,
      stopped = false;
    try {
      await blocker.beginTransaction();
      held = true;
      const [locked] = await blocker.query(
        'SELECT partition_id FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? ORDER BY partition_id FOR UPDATE',
        [search.group, search.topic],
      );
      assert.equal(locked.length, 3, 'ORIGINAL_PARTITION_CHECKPOINTS_NOT_READY');
      const input = await releaseInput(document.id, fixture.owner);
      await publishRelease(document.id, fixture.owner, input);
      // 阻塞的是之后独立的SQL检查点事务；真实ES及原事件/应用事务已经成功。
      // 明确观察边界后才退出进程，不能用sleep猜测是否已走到这一阶段。
      const applied = await waitSearchState(
        database,
        input.operation_id,
        (receipt) => receipt.state === 'SUCCEEDED',
      );
      assert.ok(BigInt(applied.next_offset) <= BigInt(applied.record_offset));
      assert.ok(kafkaCommittedOffset(applied.partition_id) <= BigInt(applied.record_offset));
      assert.equal((await searchProjection(document.id, applied.physical)).sequence, '1');
      stopOwnedRole('search');
      stopped = true;
      await blocker.rollback();
      held = false;
      await restartOwnedSearch();
      stopped = false;
      const recovered = await waitSearchReceipt(database, input.operation_id);
      assertOriginalReceipt(recovered, input.operation_id, document.id);
      assert.equal(recovered.application_revision, applied.application_revision);
      assert.equal(recovered.raw_digest, applied.raw_digest);
      assert.equal(recovered.authority_digest, applied.authority_digest);
      await assertKafkaCommitted(recovered);
    } finally {
      try {
        if (held) await blocker.rollback();
        if (stopped) await restartOwnedSearch();
      } finally {
        await blocker.end();
        await database.end();
      }
    }
  },
);
