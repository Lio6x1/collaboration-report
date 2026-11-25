import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import test from 'node:test';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
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
import { kafkaOffsets, kafkaWindow } from '../helpers/kafka.mjs';

const maximum = '18446744073709551615';

function visibilityEventID(operationID, document) {
  assert.match(document, /^[1-9][0-9]*$/);
  return (
    'visibility_index_' +
    createHash('sha256')
      .update(operationID + '\n' + document)
      .digest('hex')
  );
}

async function visibilityInput(path, user, policy) {
  const response = await get(path, user.headers);
  assert.equal(response.status, 200, json(response).code);
  return {
    operation_id: operation('search-visibility'),
    expected_visibility_revision: json(response).visibility_revision,
    policy,
  };
}

async function changeVisibility(path, user, input) {
  const response = await post(path + '/visibility', input, user.headers);
  assert.equal(response.status, 202, json(response).code);
  const result = await waitAuthorization(response, user);
  assert.equal(result.state, 'SUCCEEDED');
  return result;
}

async function documentFacts(database, document) {
  const [rows] = await database.query(
    `SELECT CAST(id AS CHAR) id,visibility_policy policy,
      CAST(visibility_revision AS CHAR) visibility_revision,
      CAST(acl_revision AS CHAR) acl_revision,CAST(event_seq AS CHAR) sequence,
      CAST(publication_epoch AS CHAR) publication_epoch,
      CAST(current_release_id AS CHAR) release_id,publication_state,lifecycle
    FROM document WHERE id=?`,
    [document],
  );
  assert.equal(rows.length, 1);
  const value = rows[0];
  for (const key of ['id', 'visibility_revision', 'acl_revision', 'publication_epoch'])
    assert.match(value[key], /^[1-9][0-9]*$/);
  assert.match(value.sequence, /^(0|[1-9][0-9]*)$/);
  if (value.release_id !== null) assert.match(value.release_id, /^[1-9][0-9]*$/);
  return value;
}

async function indexedVisibility(documentDB, searchDB, input, before, policy) {
  const event = visibilityEventID(input.operation_id, before.id);
  const expectedSequence = String(BigInt(before.sequence) + 1n);
  const [rows] = await documentDB.query(
    `SELECT event_id,aggregate_kind,CAST(aggregate_id AS CHAR) document_id,
      CAST(aggregate_seq AS CHAR) sequence,event_type,schema_version,payload
    FROM outbox WHERE event_id=?`,
    [event],
  );
  assert.equal(rows.length, 1, 'VISIBILITY_OUTBOX_MISSING_OR_DUPLICATED');
  const row = rows[0];
  assert.equal(row.aggregate_kind, 'DOCUMENT');
  assert.equal(row.document_id, before.id);
  assert.equal(row.sequence, expectedSequence);
  assert.equal(row.event_type, 'DOCUMENT_VISIBILITY_CHANGED');
  assert.equal(row.schema_version, '1');
  const payload = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
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
  assert.equal(payload.event_id, event);
  assert.equal(payload.event_type, 'DOCUMENT_VISIBILITY_CHANGED');
  assert.equal(payload.schema_version, '1');
  assert.equal(payload.document_id, before.id);
  assert.equal(payload.aggregate_seq, expectedSequence);
  assert.equal(payload.publication_epoch, before.publication_epoch);
  assert.equal(payload.release_id, before.release_id);
  assert.match(payload.occurred_at, /Z$/);
  assert.ok(Number.isFinite(Date.parse(payload.occurred_at)));

  const receipt = await waitSearchReceipt(searchDB, event);
  assertOriginalReceipt(receipt, event, before.id);
  assert.equal(receipt.sequence, expectedSequence);
  const original = JSON.parse(receipt.original_row.toString('utf8'));
  assert.equal(original.event_type, 'DOCUMENT_VISIBILITY_CHANGED');
  assert.equal(original.aggregate_seq, expectedSequence);
  assert.deepEqual(JSON.parse(original.payload), payload);

  // 读取原主题上账本对应的单条真实broker记录，不能把SQL checkpoint当作Canal/Kafka交付。
  const start = kafkaOffsets();
  const end = new Map(start);
  start.set(receipt.partition_id, BigInt(receipt.record_offset));
  end.set(receipt.partition_id, BigInt(receipt.record_offset) + 1n);
  const records = kafkaWindow(start, end);
  assert.equal(records.length, 1);
  assert.equal(records[0].partition, receipt.partition_id);
  assert.equal(String(records[0].offset), receipt.record_offset);
  assert.equal(records[0].value.database, 'document_db');
  assert.equal(records[0].value.table, 'outbox');
  assert.equal(records[0].value.type, 'INSERT');
  const brokerRow = records[0].value.data.find((value) => value.event_id === event);
  assert.ok(brokerRow, 'ORIGINAL_CANAL_VISIBILITY_ROW_MISSING');
  assert.deepEqual(brokerRow, original);

  const projection = await searchProjection(before.id, receipt.physical);
  assert.equal(projection.sequence, expectedSequence);
  assert.equal(projection.sequence_key, expectedSequence.padStart(20, '0'));
  assert.equal(projection.release_id, before.release_id);
  assert.equal(projection.visibility_policy, policy);
  assert.equal(projection.visible, before.release_id !== null);
  if (before.release_id === null) {
    assert.equal(projection.title, '');
    assert.equal(projection.text, '');
    assert.deepEqual(projection.chapters, []);
  } else assert.equal(projection.title, '协作报告');
  await assertKafkaCommitted(receipt);
}

async function knowledgeBaseState(database, knowledgeBase, documents) {
  const [knowledgeBases] = await database.query(
    `SELECT CAST(id AS CHAR) id,visibility_policy policy,
      CAST(visibility_revision AS CHAR) visibility_revision,
      CAST(acl_revision AS CHAR) acl_revision,lifecycle FROM knowledge_base WHERE id=?`,
    [knowledgeBase],
  );
  assert.equal(knowledgeBases.length, 1);
  const facts = [];
  for (const document of documents) facts.push(await documentFacts(database, document));
  const [events] = await database.query(
    `SELECT event_id,CAST(aggregate_id AS CHAR) document_id,
      CAST(aggregate_seq AS CHAR) sequence,event_type,CAST(payload AS CHAR) payload
    FROM outbox WHERE aggregate_kind='DOCUMENT' AND aggregate_id IN (?)
    ORDER BY aggregate_id,aggregate_seq`,
    [documents],
  );
  const scopes = ['kb-acl:' + knowledgeBase, ...documents.map((id) => 'doc-acl:' + id)];
  const [gates] = await database.query(
    `SELECT scope_key,CAST(max_epoch AS CHAR) epoch,state,
      CAST(revision AS CHAR) revision,last_revocation_id
    FROM access_gate WHERE scope_key IN (?) ORDER BY scope_key`,
    [scopes],
  );
  return { knowledgeBase: knowledgeBases[0], documents: facts, events, gates };
}

async function assertNoFailedVisibilityWrites(database, input, documents) {
  const ids = documents.map((document) => visibilityEventID(input.operation_id, document));
  const [[counts]] = await database.query(
    `SELECT
      CAST((SELECT COUNT(*) FROM operation_receipt WHERE operation_id=?) AS CHAR) receipts,
      CAST((SELECT COUNT(*) FROM authorization_change WHERE revocation_id=?) AS CHAR) closures,
      CAST((SELECT COUNT(*) FROM durable_task WHERE operation_id=?) AS CHAR) tasks,
      CAST((SELECT COUNT(*) FROM outbox WHERE event_id IN (?)) AS CHAR) events,
      CAST((SELECT COUNT(*) FROM access_gate WHERE scope_key=?) AS CHAR) operation_gates`,
    [
      input.operation_id,
      input.operation_id,
      input.operation_id,
      ids,
      'operation:' + input.operation_id,
    ],
  );
  for (const [kind, count] of Object.entries(counts))
    assert.equal(count, '0', 'EXHAUSTED_VISIBILITY_LEFT_' + kind.toUpperCase());
}

test(
  'Doc可见性收紧和放开保留正式指针与发布代次，新水位经原Canal及broker到达ES',
  { timeout: 120000 },
  async () => {
    const fixture = await organizationFixture(0);
    const facts = await reviewDatabase();
    const search = await searchDatabase();
    try {
      const kbPath = `/v1/knowledge-bases/${fixture.kb}`;
      await changeVisibility(
        kbPath,
        fixture.owner,
        await visibilityInput(kbPath, fixture.owner, 'PUBLIC'),
      );
      const document = await documentFixture(fixture);
      const publication = await releaseInput(document.id, fixture.owner);
      await publishRelease(document.id, fixture.owner, publication);
      await waitSearchReceipt(search, publication.operation_id);
      const path = `/v1/documents/${document.id}`;
      for (const policy of ['PUBLIC', 'COLLABORATORS']) {
        const before = await documentFacts(facts, document.id);
        const input = await visibilityInput(path, fixture.owner, policy);
        await changeVisibility(path, fixture.owner, input);
        const after = await documentFacts(facts, document.id);
        assert.equal(after.sequence, String(BigInt(before.sequence) + 1n));
        assert.equal(after.policy, policy);
        assert.equal(after.visibility_revision, String(BigInt(before.visibility_revision) + 1n));
        assert.equal(after.acl_revision, String(BigInt(before.acl_revision) + 1n));
        assert.equal(after.release_id, before.release_id);
        assert.equal(after.publication_epoch, before.publication_epoch);
        assert.equal(after.publication_state, before.publication_state);
        await indexedVisibility(facts, search, input, before, policy);
      }
    } finally {
      await facts.end();
      await search.end();
    }
  },
);

test(
  'KB固定范围包含已发布未发布撤下及回收站文档，被Doc策略遮蔽也推进每条水位，原号重放不重复',
  { timeout: 120000 },
  async () => {
    const fixture = await organizationFixture(0);
    const documents = [];
    for (let index = 0; index < 4; index++) documents.push(await documentFixture(fixture));
    const facts = await reviewDatabase();
    const search = await searchDatabase();
    try {
      for (const document of [documents[0], documents[2]]) {
        const input = await releaseInput(document.id, fixture.owner);
        await publishRelease(document.id, fixture.owner, input);
        await waitSearchReceipt(search, input.operation_id);
      }
      const current = await documentFacts(facts, documents[2].id);
      const unpublish = {
        operation_id: operation('visibility-unpublish'),
        expected_publication_revision: current.publication_epoch,
        action: 'UNPUBLISH',
      };
      const response = await post(
        `/v1/documents/${documents[2].id}/publication`,
        unpublish,
        fixture.owner.headers,
      );
      assert.equal(response.status, 202, json(response).code);
      assert.equal(
        (await pollPublication(unpublish.operation_id, fixture.owner)).state,
        'SUCCEEDED',
      );
      await waitSearchReceipt(search, unpublish.operation_id);

      // 经原DELETE/真实关闭ACK进入回收站，保留KB归属；不能因非ACTIVE漏掉其KB摘要修订。
      const deletedPath = `/v1/documents/${documents[3].id}`;
      const deletedCurrent = json(await get(deletedPath, fixture.owner.headers));
      const beforeTrash = await documentFacts(facts, documents[3].id);
      const trashInput = {
        operation_id: operation('visibility-trash'),
        expected_revision: deletedCurrent.control_revision,
      };
      await waitAuthorization(
        await post(deletedPath, trashInput, fixture.owner.headers, 'DELETE'),
        fixture.owner,
      );

      const ids = documents.map((document) => document.id);
      const before = await knowledgeBaseState(facts, fixture.kb, ids);
      assert.notEqual(before.documents[0].release_id, null);
      assert.equal(before.documents[1].sequence, '0');
      assert.equal(before.documents[1].release_id, null);
      assert.equal(before.documents[2].sequence, '2');
      assert.equal(before.documents[2].release_id, null);
      assert.equal(before.documents[3].lifecycle, 'TRASHED');
      // 当前DELETE在原事务中推进隐藏事件；即使从未发布也不能保留旧零水位。
      // 先核原删除事件真实经过Canal/Kafka/ES，再以这一事实作KB变更的基线。
      assert.equal(before.documents[3].sequence, String(BigInt(beforeTrash.sequence) + 1n));
      assert.equal(
        before.documents[3].publication_epoch,
        String(BigInt(beforeTrash.publication_epoch) + 1n),
      );
      await indexedVisibility(
        facts,
        search,
        trashInput,
        { ...beforeTrash, publication_epoch: before.documents[3].publication_epoch },
        'COLLABORATORS',
      );
      assert.ok(before.documents.every((value) => value.policy === 'COLLABORATORS'));
      const path = `/v1/knowledge-bases/${fixture.kb}`;
      const input = await visibilityInput(path, fixture.owner, 'PUBLIC');
      await changeVisibility(path, fixture.owner, input);
      const after = await knowledgeBaseState(facts, fixture.kb, ids);
      assert.equal(after.knowledgeBase.policy, 'PUBLIC');
      assert.equal(
        after.knowledgeBase.visibility_revision,
        String(BigInt(before.knowledgeBase.visibility_revision) + 1n),
      );
      assert.equal(
        after.knowledgeBase.acl_revision,
        String(BigInt(before.knowledgeBase.acl_revision) + 1n),
      );
      for (let index = 0; index < documents.length; index++) {
        const original = before.documents[index];
        const changed = after.documents[index];
        assert.equal(changed.sequence, String(BigInt(original.sequence) + 1n));
        for (const key of [
          'policy',
          'visibility_revision',
          'acl_revision',
          'release_id',
          'publication_epoch',
          'publication_state',
        ])
          assert.equal(changed[key], original[key], 'KB_VISIBILITY_CHANGED_DOCUMENT_' + key);
        // 即使有效policy仍被Doc限制，KB修订已进入权威摘要；每个子文档都必须获新seq，不能制造同号异摘要。
        await indexedVisibility(facts, search, input, original, 'COLLABORATORS');
      }
      assert.equal(after.events.length, before.events.length + documents.length);
      // Search快照可把新KB来源gate重开；先等全部实际消费完成，再固定重放前事实，避免把合法恢复误判为重放副作用。
      const beforeReplay = await knowledgeBaseState(facts, fixture.kb, ids);
      const replay = await changeVisibility(path, fixture.owner, input);
      assert.equal(replay.operation_id, input.operation_id);
      assert.deepEqual(await knowledgeBaseState(facts, fixture.kb, ids), beforeReplay);
    } finally {
      await facts.end();
      await search.end();
    }
  },
);

test('Doc最大event_seq拒绝整笔可见性更新，不留outbox关闭任务或操作回执', async () => {
  const fixture = await organizationFixture(0);
  const document = await documentFixture(fixture);
  const facts = await reviewDatabase();
  try {
    // 只为新夹具注入边界事实；实际变更仍经HTTP原事务，不把注入当成执行了2^64条事件。
    await facts.query('UPDATE document SET event_seq=? WHERE id=?', [maximum, document.id]);
    const before = await knowledgeBaseState(facts, fixture.kb, [document.id]);
    const path = `/v1/documents/${document.id}`;
    const input = await visibilityInput(path, fixture.owner, 'PUBLIC');
    const response = await post(path + '/visibility', input, fixture.owner.headers);
    assert.equal(response.status, 409, json(response).code);
    assert.equal(json(response).code, 'EVENT_SEQUENCE_EXHAUSTED');
    assert.deepEqual(await knowledgeBaseState(facts, fixture.kb, [document.id]), before);
    await assertNoFailedVisibilityWrites(facts, input, [document.id]);
  } finally {
    await facts.end();
  }
});

test('KB数值锁序中间文档event_seq耗尽使政策及前后子文档全部回滚', async () => {
  const fixture = await organizationFixture(0);
  const documents = [];
  for (let index = 0; index < 3; index++) documents.push(await documentFixture(fixture));
  const ids = documents
    .map((document) => document.id)
    .sort((left, right) =>
      BigInt(left) < BigInt(right) ? -1 : BigInt(left) > BigInt(right) ? 1 : 0,
    );
  const facts = await reviewDatabase();
  try {
    // 耗尽放在数值顺序中间，捕获先提交政策或先落前一个事件再失败的错误事务边界。
    await facts.query('UPDATE document SET event_seq=? WHERE id=?', [maximum, ids[1]]);
    const before = await knowledgeBaseState(facts, fixture.kb, ids);
    const path = `/v1/knowledge-bases/${fixture.kb}`;
    const input = await visibilityInput(path, fixture.owner, 'PUBLIC');
    const response = await post(path + '/visibility', input, fixture.owner.headers);
    assert.equal(response.status, 409, json(response).code);
    assert.equal(json(response).code, 'EVENT_SEQUENCE_EXHAUSTED');
    assert.deepEqual(await knowledgeBaseState(facts, fixture.kb, ids), before);
    await assertNoFailedVisibilityWrites(facts, input, ids);
  } finally {
    await facts.end();
  }
});
