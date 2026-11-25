import assert from 'node:assert/strict';
import { writeFileSync } from 'node:fs';
import { basename } from 'node:path';
import { organizationFixture, documentFixture } from '../helpers/workspace.mjs';
import { releaseInput, publishRelease } from '../helpers/publication.mjs';
import { kafkaOffsets, kafkaWindow } from '../helpers/kafka.mjs';

// Go实际客户端测试通过产品HTTPS创建固定正式版，再读取原Canal窗口；不跨库读取Doc事实。
try {
  const file = process.argv[2];
  assert.equal(basename(file), 'search-index-fixture.json');
  const fixture = await organizationFixture(0);
  fixture.owner.headers = { ...fixture.owner.headers, connection: 'close' };
  const document = await documentFixture(fixture);
  const start = kafkaOffsets();
  const input = await releaseInput(document.id, fixture.owner);
  const result = await publishRelease(document.id, fixture.owner, input);
  let found;
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    found = kafkaWindow(start, kafkaOffsets()).find((record) =>
      record.value.data?.some((row) => row.event_id === input.operation_id),
    );
    if (found) break;
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  assert.ok(found, 'ACTUAL_INDEX_FIXTURE_NOT_DELIVERED');
  writeFileSync(
    file,
    JSON.stringify({
      document: document.id,
      release: result.result.entity_id,
      operation: input.operation_id,
      topic: 'collab-publications-v1',
      partition: found.partition,
      offset: String(found.offset),
      record: found.value,
    }),
  );
  console.log(
    JSON.stringify({ stage: 'ACTUAL_INDEX_FIXTURE_READY', realHTTPS: true, realCanalKafka: true }),
  );
} catch (error) {
  console.error(
    JSON.stringify({
      stage: 'ACTUAL_INDEX_FIXTURE_FAILED',
      code: /^[A-Z0-9_]+$/.test(error.message) ? error.message : 'ACTUAL_FIXTURE_UNKNOWN',
    }),
  );
  process.exitCode = 1;
}
