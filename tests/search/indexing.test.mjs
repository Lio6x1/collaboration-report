import assert from 'node:assert/strict';
import test from 'node:test';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { releaseInput, publishRelease, pollPublication } from '../helpers/publication.mjs';
import { reviewDatabase } from '../helpers/review.mjs';
import {
  searchDatabase,
  waitSearchReceipt,
  searchProjection,
  assertKafkaCommitted,
  assertOriginalReceipt,
} from '../helpers/search.mjs';

test(
  '产品原Canal主题进入实际Go消费者，切版与撤下保留单调水位和隐藏墓碑',
  { timeout: 90000 },
  async () => {
    const fixture = await organizationFixture(0);
    const document = await documentFixture(fixture);
    const database = await searchDatabase();
    try {
      const first = await releaseInput(document.id, fixture.owner);
      const published = await publishRelease(document.id, fixture.owner, first);
      const firstReceipt = await waitSearchReceipt(database, first.operation_id);
      assertOriginalReceipt(firstReceipt, first.operation_id, document.id);
      const firstProjection = await searchProjection(document.id, firstReceipt.physical);
      assert.equal(firstProjection.sequence, '1');
      assert.equal(firstProjection.sequence_key, '00000000000000000001');
      assert.equal(firstProjection.release_id, published.result.entity_id);
      assert.equal(firstProjection.visible, true);
      assert.equal(firstProjection.title, '协作报告');
      await assertKafkaCommitted(firstReceipt);

      const second = await releaseInput(document.id, fixture.owner);
      await publishRelease(document.id, fixture.owner, second);
      const secondReceipt = await waitSearchReceipt(database, second.operation_id);
      assertOriginalReceipt(secondReceipt, second.operation_id, document.id);
      const current = json(await get(`/v1/documents/${document.id}`, fixture.owner.headers));
      const switched = {
        operation_id: operation('search-switch'),
        expected_publication_revision: current.publication_epoch,
        action: 'SWITCH',
        release_id: published.result.entity_id,
      };
      const switchedResponse = await post(
        `/v1/documents/${document.id}/publication`,
        switched,
        fixture.owner.headers,
      );
      assert.equal(switchedResponse.status, 202, json(switchedResponse).code);
      assert.equal(
        (await pollPublication(switched.operation_id, fixture.owner)).state,
        'SUCCEEDED',
      );
      const switchReceipt = await waitSearchReceipt(database, switched.operation_id);
      const switchProjection = await searchProjection(document.id, switchReceipt.physical);
      assert.equal(switchProjection.sequence, '3');
      assert.equal(switchProjection.release_id, published.result.entity_id);

      const now = json(await get(`/v1/documents/${document.id}`, fixture.owner.headers));
      const hidden = {
        operation_id: operation('search-unpublish'),
        expected_publication_revision: now.publication_epoch,
        action: 'UNPUBLISH',
      };
      const response = await post(
        `/v1/documents/${document.id}/publication`,
        hidden,
        fixture.owner.headers,
      );
      assert.equal(response.status, 202, json(response).code);
      assert.equal((await pollPublication(hidden.operation_id, fixture.owner)).state, 'SUCCEEDED');
      const tombstone = await waitSearchReceipt(database, hidden.operation_id);
      assertOriginalReceipt(tombstone, hidden.operation_id, document.id);
      const projection = await searchProjection(document.id, tombstone.physical);
      assert.equal(projection.sequence, '4');
      assert.equal(projection.sequence_key, '00000000000000000004');
      assert.equal(projection.visible, false);
      assert.equal(projection.release_id, null);
      assert.equal(projection.title, '');
      assert.equal(projection.text, '');
      assert.deepEqual(projection.chapters, []);
      // 业务成功、SQL原事件与实际应用、broker offset各自验真；不以某一项代替其他项。
      await assertKafkaCommitted(tombstone);
    } finally {
      await database.end();
    }
  },
);

test(
  '产品原主题的最大uint64发布及隐藏水位完整到达ES和Search私有SQL',
  { timeout: 60000 },
  async () => {
    const fixture = await organizationFixture(0);
    const document = await documentFixture(fixture);
    const database = await searchDatabase();
    const facts = await reviewDatabase();
    const maximum = 18446744073709551615n;
    try {
      // 只把新夹具放到边界前两步，不声称实际执行了2^64次发布。
      // 后面的发布、旧输出关闭、原Canal投递和所有业务水位均走真实产品路径。
      await facts.query('UPDATE document SET event_seq=?,publication_epoch=? WHERE id=?', [
        String(maximum - 2n),
        String(maximum - 2n),
        document.id,
      ]);
      const input = await releaseInput(document.id, fixture.owner);
      const published = await publishRelease(document.id, fixture.owner, input);
      const applied = await waitSearchReceipt(database, input.operation_id);
      const visible = await searchProjection(document.id, applied.physical);
      assert.equal(applied.sequence, String(maximum - 1n));
      assert.equal(visible.sequence, String(maximum - 1n));
      assert.equal(visible.release_id, published.result.entity_id);
      const current = json(await get(`/v1/documents/${document.id}`, fixture.owner.headers));
      const hidden = {
        operation_id: operation('search-max-unpublish'),
        expected_publication_revision: current.publication_epoch,
        action: 'UNPUBLISH',
      };
      const response = await post(
        `/v1/documents/${document.id}/publication`,
        hidden,
        fixture.owner.headers,
      );
      assert.equal(response.status, 202, json(response).code);
      assert.equal((await pollPublication(hidden.operation_id, fixture.owner)).state, 'SUCCEEDED');
      const receipt = await waitSearchReceipt(database, hidden.operation_id);
      assertOriginalReceipt(receipt, hidden.operation_id, document.id);
      const tombstone = await searchProjection(document.id, receipt.physical);
      assert.equal(receipt.sequence, String(maximum));
      assert.equal(tombstone.sequence, String(maximum));
      assert.equal(tombstone.sequence_key, String(maximum));
      assert.equal(tombstone.visible, false);
      assert.equal(tombstone.text, '');
      assert.equal(tombstone.title, '');
      assert.equal(tombstone.release_id, null);
      await assertKafkaCommitted(receipt);
    } finally {
      await facts.end();
      await database.end();
    }
  },
);
