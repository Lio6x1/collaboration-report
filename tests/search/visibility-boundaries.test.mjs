import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import test from 'node:test';
import https from 'node:https';
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
import { kafkaWindow } from '../helpers/kafka.mjs';

async function destroyIdleHTTPS() {
  // 同步broker CLI阻塞event loop时，已结束HTTP的idle TLS关闭事件可能延迟；
  // 只在确认没有在途请求/活动socket后清空池，不重试POST，不触碰活跃业务输出。
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(
    Object.values(https.globalAgent.requests).flat().length,
    0,
    'BOUNDARY_HTTP_STILL_QUEUED',
  );
  assert.equal(
    Object.values(https.globalAgent.sockets).flat().length,
    0,
    'BOUNDARY_HTTP_STILL_IN_FLIGHT',
  );
  https.globalAgent.destroy();
  await new Promise((resolve) => setImmediate(resolve));
}

test.beforeEach(destroyIdleHTTPS);

function visibilityEventID(operationID, document) {
  assert.match(document, /^[1-9][0-9]*$/);
  return (
    'visibility_index_' +
    createHash('sha256')
      .update(operationID + '\n' + document)
      .digest('hex')
  );
}

function creationInput(fixture) {
  return {
    operation_id: operation('visibility-boundary-create'),
    space_id: fixture.space,
    knowledge_base_id: fixture.kb,
    title: '协作报告',
  };
}

async function createDocument(fixture, input = creationInput(fixture)) {
  const response = await post('/v1/documents', input, fixture.owner.headers);
  assert.equal(response.status, 201, json(response).code);
  const id = json(response).result.entity_id;
  assert.match(id, /^[1-9][0-9]*$/);
  return id;
}

async function visibilityInput(fixture) {
  const response = await get(`/v1/knowledge-bases/${fixture.kb}`, fixture.owner.headers);
  assert.equal(response.status, 200, json(response).code);
  return {
    operation_id: operation('visibility-boundary-kb'),
    expected_visibility_revision: json(response).visibility_revision,
    policy: 'PUBLIC',
  };
}

function visibilityRequest(fixture, input) {
  return post(`/v1/knowledge-bases/${fixture.kb}/visibility`, input, fixture.owner.headers);
}

async function visibilitySucceeded(response, fixture) {
  assert.equal(response.status, 202, json(response).code);
  const result = await waitAuthorization(response, fixture.owner);
  assert.equal(result.state, 'SUCCEEDED');
  return result;
}

async function knowledgeBaseDocuments(database, knowledgeBase) {
  const [rows] = await database.query(
    `SELECT CAST(id AS CHAR) id,CAST(event_seq AS CHAR) sequence,
      CAST(publication_epoch AS CHAR) publication_epoch,
      CAST(current_release_id AS CHAR) release_id,
      CAST(visibility_revision AS CHAR) visibility_revision,
      CAST(acl_revision AS CHAR) acl_revision,visibility_policy,lifecycle
    FROM document WHERE knowledge_base_id=? ORDER BY id`,
    [knowledgeBase],
  );
  for (const row of rows) {
    for (const key of ['id', 'publication_epoch', 'visibility_revision', 'acl_revision'])
      assert.match(row[key], /^[1-9][0-9]*$/);
    assert.match(row.sequence, /^(0|[1-9][0-9]*)$/);
    if (row.release_id !== null) assert.match(row.release_id, /^[1-9][0-9]*$/);
  }
  return rows;
}

async function visibilityRows(database, input, documents) {
  const [rows] = await database.query(
    `SELECT event_id,aggregate_kind,CAST(aggregate_id AS CHAR) document_id,
      CAST(aggregate_seq AS CHAR) sequence,event_type,schema_version,payload
    FROM outbox WHERE event_id IN (?) ORDER BY aggregate_id`,
    [documents.map((document) => visibilityEventID(input.operation_id, document.id))],
  );
  assert.equal(rows.length, documents.length, 'KB_FIXED_RANGE_OUTBOX_INCOMPLETE');
  const expected = new Map(documents.map((document) => [document.id, document]));
  for (const row of rows) {
    const document = expected.get(row.document_id);
    assert.ok(document, 'KB_FIXED_RANGE_HAS_UNEXPECTED_DOCUMENT');
    assert.equal(row.event_id, visibilityEventID(input.operation_id, document.id));
    assert.equal(row.aggregate_kind, 'DOCUMENT');
    assert.equal(row.event_type, 'DOCUMENT_VISIBILITY_CHANGED');
    assert.equal(row.schema_version, '1');
    assert.equal(row.sequence, document.sequence);
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
    assert.equal(payload.event_id, row.event_id);
    assert.equal(payload.document_id, document.id);
    assert.equal(payload.aggregate_seq, document.sequence);
    assert.equal(payload.publication_epoch, document.publication_epoch);
    assert.equal(payload.release_id, document.release_id);
    assert.equal(payload.schema_version, '1');
    assert.equal(payload.event_type, 'DOCUMENT_VISIBILITY_CHANGED');
    assert.match(payload.occurred_at, /Z$/);
    assert.ok(Number.isFinite(Date.parse(payload.occurred_at)));
    row.payload = payload;
    expected.delete(document.id);
  }
  assert.equal(expected.size, 0);
  return rows;
}

async function indexedRange(searchDB, rows, documents, options = {}) {
  const diagnostic = options.diagnostic ?? (() => {});
  const verificationStarted = options.startedAt ?? Date.now();
  const deadline = verificationStarted + 90000;
  const expected = new Map(documents.map((document) => [document.id, document]));
  const receipts = [];
  for (const row of rows) {
    // 总规模预算不延长任何一条事件的既有20秒消费期限。
    assert.ok(Date.now() < deadline, 'KB_SCALE_SQL_ES_BUDGET_EXHAUSTED');
    const receipt = await waitSearchReceipt(
      searchDB,
      row.event_id,
      Math.min(20000, deadline - Date.now()),
    );
    assertOriginalReceipt(receipt, row.event_id, row.document_id);
    assert.equal(receipt.sequence, row.sequence);
    const original = JSON.parse(receipt.original_row.toString('utf8'));
    assert.equal(original.event_type, 'DOCUMENT_VISIBILITY_CHANGED');
    assert.equal(original.aggregate_seq, row.sequence);
    assert.deepEqual(JSON.parse(original.payload), row.payload);
    receipts.push(receipt);
    const document = expected.get(row.document_id);
    const projection = await searchProjection(document.id, receipt.physical);
    assert.equal(projection.sequence, document.sequence);
    assert.equal(projection.sequence_key, document.sequence.padStart(20, '0'));
    assert.equal(projection.release_id, document.release_id);
    // Doc的COLLABORATORS遮蔽KB的PUBLIC；有效policy不变也必须更新水位。
    assert.equal(projection.visibility_policy, 'COLLABORATORS');
    assert.equal(projection.visible, document.release_id !== null);
    if (document.release_id === null) {
      assert.equal(projection.title, '');
      assert.equal(projection.text, '');
      assert.deepEqual(projection.chapters, []);
    } else assert.equal(projection.title, '协作报告');
  }

  assert.ok(Date.now() < deadline, 'KB_SCALE_SQL_ES_BUDGET_EXHAUSTED');
  diagnostic(
    `SQL/ES阶段完成，共${documents.length}个Doc，耗时${Date.now() - verificationStarted}ms；预算90秒`,
  );

  const brokerStarted = Date.now();
  const brokerDeadline = brokerStarted + 90000;
  let brokerComplete = false;
  try {
    const partitions = new Map();
    for (const receipt of receipts) {
      const group = partitions.get(receipt.partition_id) ?? [];
      group.push(receipt);
      partitions.set(receipt.partition_id, group);
    }
    for (const [partition, group] of partitions) {
      group.sort((a, b) => (BigInt(a.record_offset) < BigInt(b.record_offset) ? -1 : 1));
      const byOffset = new Map();
      for (const receipt of group) {
        const values = byOffset.get(receipt.record_offset) ?? [];
        values.push(receipt);
        byOffset.set(receipt.record_offset, values);
      }
      const first = BigInt(group[0].record_offset);
      const end = BigInt(group.at(-1).record_offset) + 1n;
      // 有限窗口分段读取，保持已有每段128条/1MiB上限；不从topic起点重读历史。
      assert.ok(end - first <= 1024n, 'KB_SCALE_BROKER_WINDOW_OUT_OF_BOUNDS');
      for (let offset = first; offset < end; offset += 128n) {
        assert.ok(Date.now() < brokerDeadline, 'KB_SCALE_BROKER_BUDGET_EXHAUSTED');
        const stop = offset + 128n < end ? offset + 128n : end;
        const records = kafkaWindow(new Map([[partition, offset]]), new Map([[partition, stop]]));
        for (const record of records) {
          for (const receipt of byOffset.get(String(record.offset)) ?? []) {
            const original = JSON.parse(receipt.original_row.toString('utf8'));
            assert.equal(record.value.database, 'document_db');
            assert.equal(record.value.table, 'outbox');
            assert.equal(record.value.type, 'INSERT');
            const brokerRow = record.value.data.find(
              (value) => value.event_id === original.event_id,
            );
            assert.deepEqual(brokerRow, original, 'ORIGINAL_CANAL_VISIBILITY_ROW_MISSING');
          }
          byOffset.delete(String(record.offset));
        }
      }
      assert.equal(byOffset.size, 0, 'KB_FIXED_RANGE_BROKER_RECORD_INCOMPLETE');
      // 分区末条的真实组提交覆盖此前所有offset，不用401次CLI冒充额外业务等待。
      await assertKafkaCommitted(group.at(-1));
    }
    assert.ok(Date.now() < brokerDeadline, 'KB_SCALE_BROKER_BUDGET_EXHAUSTED');
    brokerComplete = true;
  } finally {
    await destroyIdleHTTPS();
    diagnostic(
      `原broker记录及真实组提交阶段退出，耗时${Date.now() - brokerStarted}ms；独立预算90秒，完整核验=${brokerComplete}`,
    );
  }
}

async function resumeScaleFixture(database, knowledgeBase, diagnostic) {
  const resumedAt = Date.now();
  // 显式指定才允许复用。非法/空/超uint64环境值在任何HTTP创建之前拒绝，绝不回退新seed。
  assert.match(knowledgeBase, /^[1-9][0-9]*$/);
  assert.ok(BigInt(knowledgeBase) <= 18446744073709551615n, 'KB_SCALE_RESUME_ID_OUT_OF_RANGE');
  let kb;
  let original;
  await database.beginTransaction();
  try {
    const [knowledgeBases] = await database.query(
      `SELECT CAST(id AS CHAR) id,CAST(space_id AS CHAR) space_id,lifecycle,visibility_policy,
        CAST(visibility_revision AS CHAR) visibility_revision,CAST(acl_revision AS CHAR) acl_revision
      FROM knowledge_base WHERE id=?`,
      [knowledgeBase],
    );
    assert.equal(knowledgeBases.length, 1, 'KB_SCALE_RESUME_KB_NOT_FOUND');
    kb = knowledgeBases[0];
    assert.equal(kb.id, knowledgeBase);
    assert.match(kb.space_id, /^[1-9][0-9]*$/);
    assert.equal(kb.lifecycle, 'ACTIVE');
    assert.equal(kb.visibility_policy, 'PUBLIC');
    assert.equal(kb.visibility_revision, '2');
    assert.equal(kb.acl_revision, '2');
    const [authorizations] = await database.query(
      `SELECT c.revocation_id operation_id,CAST(c.actor_id AS CHAR) actor_id,c.state,
        CAST(c.epoch AS CHAR) epoch,CAST(r.actor_id AS CHAR) receipt_actor,r.kind,
        r.state receipt_state,HEX(r.request_fingerprint) fingerprint
      FROM authorization_change c LEFT JOIN operation_receipt r ON r.operation_id=c.revocation_id
      WHERE c.scope_key=?`,
      ['kb-acl:' + knowledgeBase],
    );
    assert.equal(authorizations.length, 1, 'KB_SCALE_RESUME_AUTHORIZATION_NOT_UNIQUE');
    original = authorizations[0];
    const uuid = '[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}';
    assert.match(original.operation_id, new RegExp('^visibility-boundary-kb-' + uuid + '$'));
    assert.match(original.actor_id, /^[1-9][0-9]*$/);
    assert.equal(original.receipt_actor, original.actor_id);
    assert.equal(original.state, 'SUCCEEDED');
    assert.equal(original.receipt_state, 'SUCCEEDED');
    assert.equal(original.kind, 'KB_VISIBILITY');
    assert.equal(original.epoch, '1');
    const documents = await knowledgeBaseDocuments(database, knowledgeBase);
    assert.equal(documents.length, 401, 'KB_SCALE_RESUME_DOCUMENT_RANGE_CHANGED');
    for (const document of documents) {
      assert.equal(document.lifecycle, 'ACTIVE');
      assert.equal(document.sequence, '1');
      assert.equal(document.publication_epoch, '1');
      assert.equal(document.release_id, null);
      assert.equal(document.visibility_policy, 'COLLABORATORS');
      assert.equal(document.visibility_revision, '1');
      assert.equal(document.acl_revision, '1');
    }
    const [[unpublished]] = await database.query(
      `SELECT CAST(COUNT(*) AS CHAR) count FROM document
      WHERE knowledge_base_id=? AND publication_state='UNPUBLISHED' AND responsible_user_id=?`,
      [knowledgeBase, original.actor_id],
    );
    assert.equal(unpublished.count, '401', 'KB_SCALE_RESUME_UNPUBLISHED_ACTOR_BINDING_CHANGED');
    await database.commit();
  } finally {
    await database.rollback();
  }
  const input = {
    operation_id: original.operation_id,
    expected_visibility_revision: '1',
    policy: 'PUBLIC',
  };
  // 核对冻结DTO编码的原请求指纹，复用不能换号/换目标/把当前revision2冒充原请求。
  const fingerprint = createHash('sha256')
    .update(
      JSON.stringify({
        Path: `/v1/knowledge-bases/${knowledgeBase}/visibility`,
        Input: input,
      }),
    )
    .digest('hex')
    .toUpperCase();
  assert.equal(original.fingerprint, fingerprint, 'KB_SCALE_RESUME_ORIGINAL_INPUT_CHANGED');
  let account;
  let email;
  try {
    account = await reviewDatabase('account');
    const [users] = await account.query('SELECT email,state FROM `user` WHERE id=?', [
      original.actor_id,
    ]);
    assert.equal(users.length, 1, 'KB_SCALE_RESUME_ACTOR_NOT_FOUND');
    assert.equal(users[0].state, 'ACTIVE');
    // 只登录createUser约定的本项目fixture账号，不尝试真实用户或读取密码hash。
    assert.ok(
      /^fixture-[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}@example\.test$/.test(
        users[0].email,
      ),
      'KB_SCALE_RESUME_ACTOR_IS_NOT_FIXTURE',
    );
    email = users[0].email;
  } finally {
    await account?.end();
  }
  const login = await post('/v1/account/login', { email, password: 'fixture-password-123' });
  assert.equal(login.status, 200, json(login).code);
  assert.ok(Array.isArray(login.headers['set-cookie']), 'KB_SCALE_RESUME_LOGIN_COOKIE_MISSING');
  const owner = {
    id: original.actor_id,
    headers: {
      cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
      'x-csrf-token': json(login).csrf_token,
    },
  };
  const me = await get('/v1/account/me', owner.headers);
  assert.equal(me.status, 200, json(me).code);
  assert.equal(json(me).id, original.actor_id, 'KB_SCALE_RESUME_LOGIN_ACTOR_CHANGED');
  diagnostic(
    `显式复用原产品fixture KB=${knowledgeBase}，资格核验及真实登录耗时${Date.now() - resumedAt}ms：401个Doc及原SUCCEEDED政策操作已核验；跳过seed，不重做政策，不声称旧FAIL已PASS`,
  );
  return { fixture: { kb: knowledgeBase, space: kb.space_id, owner }, input };
}

test(
  'KB超过400文档固定范围：401个产品创建Doc全部同事务推进并经原Canal和broker到ES，原号不重复',
  { timeout: 420000 },
  async (t) => {
    const resume = process.env.COLLAB_TEST_VISIBILITY_RESUME_KB;
    if (resume !== undefined) {
      assert.match(resume, /^[1-9][0-9]*$/);
      assert.ok(BigInt(resume) <= 18446744073709551615n, 'KB_SCALE_RESUME_ID_OUT_OF_RANGE');
    }
    let facts;
    let search;
    try {
      facts = await reviewDatabase();
      search = await searchDatabase();
      let fixture;
      let input;
      let after;
      let created;
      if (resume !== undefined) {
        ({ fixture, input } = await resumeScaleFixture(facts, resume, (message) =>
          t.diagnostic(message),
        ));
        // 复用直接核对真实seq1/epoch1事实；没有虚构before，更不再执行新可见性操作。
        after = await knowledgeBaseDocuments(facts, fixture.kb);
        created = after.map((document) => document.id);
      } else {
        fixture = await organizationFixture(0);
        // 只做401次真实创建，不用SQL伪造正文/文档；4个请求是固定并发上限。
        // 实测4并发90秒仍未种完401项，因此150秒只覆盖规模种数据，不是业务延时。
        // 单次HTTP仍为原3秒；失败后不再派发新创建，全范围验证仍单独限90秒。
        const fixtureStarted = Date.now();
        const fixtureDeadline = fixtureStarted + 150000;
        created = [];
        let next = 0;
        let fixtureFailed = false;
        const worker = async () => {
          try {
            while (next < 401 && !fixtureFailed) {
              assert.ok(Date.now() < fixtureDeadline, 'KB_SCALE_FIXTURE_BUDGET_EXHAUSTED');
              next++;
              created.push(await createDocument(fixture));
              if (created.length % 100 === 0)
                t.diagnostic(`已通过产品HTTP创建${created.length}/401个Doc`);
            }
          } catch (error) {
            fixtureFailed = true;
            throw error;
          }
        };
        const workers = await Promise.allSettled(Array.from({ length: 4 }, worker));
        for (const result of workers) if (result.status === 'rejected') throw result.reason;
        t.diagnostic(
          `401个Doc产品fixture创建完成，耗时${Date.now() - fixtureStarted}ms；此为种数据耗时，尚未执行KB可见性业务`,
        );
        created.sort((left, right) => (BigInt(left) < BigInt(right) ? -1 : 1));
        const before = await knowledgeBaseDocuments(facts, fixture.kb);
        assert.equal(before.length, 401);
        assert.deepEqual(
          before.map((document) => document.id),
          created,
        );
        for (const document of before) {
          assert.equal(document.sequence, '0');
          assert.equal(document.publication_epoch, '1');
          assert.equal(document.release_id, null);
          assert.equal(document.lifecycle, 'ACTIVE');
        }
        input = await visibilityInput(fixture);
        await visibilitySucceeded(await visibilityRequest(fixture, input), fixture);
        after = await knowledgeBaseDocuments(facts, fixture.kb);
        assert.deepEqual(
          after,
          before.map((document) => ({ ...document, sequence: '1' })),
        );
      }
      const verificationStarted = Date.now();
      const checked = await knowledgeBaseDocuments(facts, fixture.kb);
      assert.equal(checked.length, 401);
      assert.deepEqual(checked, after);
      const rows = await visibilityRows(facts, input, after);
      await indexedRange(search, rows, after, {
        startedAt: verificationStarted,
        diagnostic: (message) => t.diagnostic(message),
      });
      const replay = await visibilityRequest(fixture, input);
      // 原号重放沿冻结的政策POST状态码202，SUCCEEDED与无重复事件另行核验。
      assert.equal(replay.status, 202, json(replay).code);
      assert.equal(json(replay).state, 'SUCCEEDED');
      assert.deepEqual(await knowledgeBaseDocuments(facts, fixture.kb), after);
      assert.deepEqual(await visibilityRows(facts, input, after), rows);
      const [[count]] = await facts.query(
        `SELECT CAST(COUNT(*) AS CHAR) count FROM outbox WHERE aggregate_kind='DOCUMENT'
       AND aggregate_id IN (?)`,
        [created],
      );
      assert.equal(count.count, '401', 'KB_SCALE_REPLAY_DUPLICATED_EVENTS');
    } finally {
      const ended = await Promise.allSettled([facts?.end(), search?.end()]);
      for (const result of ended) if (result.status === 'rejected') throw result.reason;
    }
  },
);

function settledRequest(start) {
  return start().then(
    (response) => ({ response }),
    (error) => ({ error }),
  );
}

async function requestResult(pending) {
  const result = await pending;
  if (result.error) throw result.error;
  return result.response;
}

async function waitQuery(observer, predicate, connection, timeout = 1000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const [rows] = await observer.query('SHOW FULL PROCESSLIST');
    const matches = rows.filter(
      (row) =>
        (connection === undefined || String(row.Id) === connection) && predicate(row.Info ?? ''),
    );
    assert.ok(matches.length <= 1, 'BOUNDARY_LOCK_CONNECTION_NOT_UNIQUE');
    if (matches.length === 1) {
      // mysql2按BIGINT字符串返回连接ID；它也只能按完整十进制绑定，不能转Number。
      const connectionID = String(matches[0].Id);
      assert.match(connectionID, /^[1-9][0-9]*$/);
      assert.ok(BigInt(connectionID) <= 18446744073709551615n);
      return connectionID;
    }
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  assert.fail('BOUNDARY_SPECIFIC_OPERATION_LOCK_NOT_OBSERVED');
}

async function holdOperation(connection, operationID) {
  // 仅种测试互斥gate，不写receipt、policy、正文或水位。服务仍走真实业务事务。
  await connection.query("INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN')", [
    'operation:' + operationID,
  ]);
  await connection.beginTransaction();
  await connection.query('SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE', [
    'operation:' + operationID,
  ]);
}

function operationQuery(operationID) {
  return (sql) =>
    sql.includes('INSERT INTO access_gate') && sql.includes('operation:' + operationID);
}

function businessQuery(table, id) {
  assert.ok(['space', 'knowledge_base', 'document'].includes(table));
  assert.match(id, /^[1-9][0-9]*$/);
  const pattern = new RegExp(`FROM ${table} WHERE id='?${id}'? FOR UPDATE`);
  return (sql) => pattern.test(sql);
}

async function withLocks(run) {
  const connections = [];
  const pending = [];
  const acquire = async () => {
    const connection = await reviewDatabase();
    connections.push(connection);
    return connection;
  };
  const start = (request) => {
    const result = settledRequest(request);
    pending.push(result);
    return result;
  };
  try {
    return await run({ acquire, start });
  } finally {
    // 失败也先释放全部SQL屏障，再等待原请求退出并关闭连接；不留悬挂服务事务。
    const released = await Promise.allSettled(
      connections.map((connection) => connection.rollback()),
    );
    await Promise.allSettled(pending);
    const ended = await Promise.allSettled(connections.map((connection) => connection.end()));
    for (const result of [...released, ...ended])
      if (result.status === 'rejected') throw result.reason;
  }
}

async function orderedRequests({ firstOperation, secondOperation, table, id, first, second }) {
  return withLocks(async ({ acquire, start }) => {
    const business = await acquire();
    const firstGate = await acquire();
    const secondGate = await acquire();
    const observer = await acquire();
    await holdOperation(firstGate, firstOperation);
    await holdOperation(secondGate, secondOperation);
    await business.beginTransaction();
    await business.query(`SELECT id FROM ${table} WHERE id=? FOR UPDATE`, [id]);
    const firstPending = start(first);
    const firstConnection = await waitQuery(observer, operationQuery(firstOperation));
    await firstGate.rollback();
    // 先识别原operation对应连接，再确认其取得来源gate后等唯一fixture业务行。
    await waitQuery(observer, businessQuery(table, id), firstConnection);
    const secondPending = start(second);
    await waitQuery(observer, operationQuery(secondOperation));
    await business.rollback();
    const firstResponse = await requestResult(firstPending);
    // 第一笔业务已提交即可放第二笔，不把异步关闭/消费的等待加进3秒HTTP预算。
    await secondGate.rollback();
    const secondResponse = await requestResult(secondPending);
    return { firstResponse, secondResponse };
  });
}

function assertOldScopeRejected(response) {
  const code = json(response).code;
  // 第一笔已关闭旧代。参与者ACK未齐只允许过渡拒绝，齐后只能旧来源拒绝。
  assert.ok(
    (response.status === 403 && code === 'SOURCE_CLOSED') ||
      (response.status === 409 && code === 'SCOPE_TRANSITION_PENDING'),
    `STALE_KB_SCOPE_WAS_NOT_REJECTED ${response.status} ${code}`,
  );
}

async function assertNoOperationWrites(database, operationID, publication = false) {
  const [[counts]] = await database.query(
    `SELECT
      CAST((SELECT COUNT(*) FROM operation_receipt WHERE operation_id=?) AS CHAR) receipts,
      CAST((SELECT COUNT(*) FROM durable_task WHERE operation_id=?) AS CHAR) tasks,
      CAST((SELECT COUNT(*) FROM outbox WHERE event_id=?) AS CHAR) events,
      CAST((SELECT COUNT(*) FROM authorization_change WHERE revocation_id=?) AS CHAR) closures`,
    [operationID, operationID, operationID, operationID],
  );
  for (const [kind, count] of Object.entries(counts))
    assert.equal(count, '0', 'STALE_OPERATION_LEFT_' + kind.toUpperCase());
  if (publication) {
    const [[releases]] = await database.query(
      'SELECT CAST(COUNT(*) AS CHAR) count FROM `release` WHERE operation_id=?',
      [operationID],
    );
    assert.equal(releases.count, '0', 'STALE_PUBLICATION_LEFT_RELEASE');
  }
}

async function withFacts(run) {
  let facts;
  let search;
  try {
    facts = await reviewDatabase();
    search = await searchDatabase();
    return await run(facts, search);
  } finally {
    const ended = await Promise.allSettled([facts?.end(), search?.end()]);
    for (const result of ended) if (result.status === 'rejected') throw result.reason;
  }
}

test(
  '创建先提交：KB请求在新Doc出生前准备，锁内固定范围仍包含该Doc',
  { timeout: 120000 },
  async () => {
    const fixture = await organizationFixture(0);
    const original = await documentFixture(fixture);
    const creation = creationInput(fixture);
    const visibility = await visibilityInput(fixture);
    const { firstResponse, secondResponse } = await orderedRequests({
      firstOperation: creation.operation_id,
      secondOperation: visibility.operation_id,
      table: 'space',
      id: fixture.space,
      first: () => post('/v1/documents', creation, fixture.owner.headers),
      second: () => visibilityRequest(fixture, visibility),
    });
    assert.equal(firstResponse.status, 201, json(firstResponse).code);
    const born = json(firstResponse).result.entity_id;
    assert.match(born, /^[1-9][0-9]*$/);
    await visibilitySucceeded(secondResponse, fixture);
    await withFacts(async (facts, search) => {
      const documents = await knowledgeBaseDocuments(facts, fixture.kb);
      assert.deepEqual(
        documents.map((document) => document.id),
        [original.id, born],
      );
      for (const document of documents) {
        assert.equal(document.sequence, '1');
        assert.equal(document.publication_epoch, '1');
        assert.equal(document.release_id, null);
      }
      await indexedRange(search, await visibilityRows(facts, visibility, documents), documents);
    });
  },
);

test(
  'KB先提交：旧范围准备的创建拒绝且整笔无落地，重新取得资格的创建不补旧事件',
  { timeout: 120000 },
  async () => {
    const fixture = await organizationFixture(0);
    const original = await documentFixture(fixture);
    const creation = creationInput(fixture);
    const visibility = await visibilityInput(fixture);
    const { firstResponse, secondResponse } = await orderedRequests({
      firstOperation: visibility.operation_id,
      secondOperation: creation.operation_id,
      table: 'knowledge_base',
      id: fixture.kb,
      first: () => visibilityRequest(fixture, visibility),
      second: () => post('/v1/documents', creation, fixture.owner.headers),
    });
    assertOldScopeRejected(secondResponse);
    await visibilitySucceeded(firstResponse, fixture);
    await withFacts(async (facts, search) => {
      const fixed = await knowledgeBaseDocuments(facts, fixture.kb);
      assert.equal(fixed.length, 1, 'STALE_CREATE_LEFT_DOCUMENT');
      assert.equal(fixed[0].id, original.id);
      assert.equal(fixed[0].sequence, '1');
      await assertNoOperationWrites(facts, creation.operation_id);
      await indexedRange(search, await visibilityRows(facts, visibility, fixed), fixed);
      const born = await createDocument(fixture);
      const documents = await knowledgeBaseDocuments(facts, fixture.kb);
      assert.equal(documents.length, 2);
      assert.deepEqual(documents[0], fixed[0]);
      assert.equal(documents[1].id, born);
      assert.equal(documents[1].sequence, '0');
      assert.equal(documents[1].publication_epoch, '1');
      assert.equal(documents[1].release_id, null);
      const [[events]] = await facts.query(
        'SELECT CAST(COUNT(*) AS CHAR) count FROM outbox WHERE event_id=?',
        [visibilityEventID(visibility.operation_id, born)],
      );
      assert.equal(events.count, '0', 'NEW_SCOPE_CREATE_WAS_ADDED_TO_OLD_FIXED_RANGE');
    });
  },
);

test(
  '发布先提交：真实RELEASE_CREATE最终事务先落定，KB事件保留新正式指针和发布代次',
  { timeout: 120000 },
  async () => {
    const fixture = await organizationFixture(0);
    const document = await documentFixture(fixture);
    const publication = await releaseInput(document.id, fixture.owner);
    const visibility = await visibilityInput(fixture);
    const response = await withLocks(async ({ acquire, start }) => {
      const releaseLock = await acquire();
      const kbGate = await acquire();
      const observer = await acquire();
      await holdOperation(kbGate, visibility.operation_id);
      await releaseLock.beginTransaction();
      // release_history是冻结索引；空范围next-key锁只挡该真实发布插入，登记仍可202。
      await releaseLock.query(
        'SELECT id FROM `release` FORCE INDEX (release_history) WHERE document_id=? FOR UPDATE',
        [document.id],
      );
      const registering = start(() =>
        post(`/v1/documents/${document.id}/releases`, publication, fixture.owner.headers),
      );
      const registered = await requestResult(registering);
      assert.equal(registered.status, 202, json(registered).code);
      // 这是后台最终提交的INSERT，operation和Doc同时绑定，不能把登记当成已发布。
      await waitQuery(
        observer,
        (sql) =>
          sql.includes('INSERT INTO `release`') &&
          sql.includes(publication.operation_id) &&
          new RegExp(`VALUES\\('?${document.id}'?,`).test(sql),
        undefined,
        20000,
      );
      const changing = start(() => visibilityRequest(fixture, visibility));
      await waitQuery(observer, operationQuery(visibility.operation_id));
      await releaseLock.rollback();
      const deadline = Date.now() + 1000;
      let committed = false;
      while (Date.now() < deadline) {
        const documents = await knowledgeBaseDocuments(observer, fixture.kb);
        if (documents[0].release_id !== null && documents[0].sequence === '1') {
          assert.equal(documents[0].publication_epoch, '2');
          committed = true;
          break;
        }
        await new Promise((resolve) => setTimeout(resolve, 10));
      }
      assert.equal(committed, true, 'REAL_PUBLICATION_COMMIT_NOT_OBSERVED');
      await kbGate.rollback();
      return requestResult(changing);
    });
    await visibilitySucceeded(response, fixture);
    const published = await pollPublication(publication.operation_id, fixture.owner);
    assert.equal(published.state, 'SUCCEEDED', published.error?.code);
    await withFacts(async (facts, search) => {
      const documents = await knowledgeBaseDocuments(facts, fixture.kb);
      assert.equal(documents.length, 1);
      assert.equal(documents[0].id, document.id);
      assert.equal(documents[0].sequence, '2');
      assert.equal(documents[0].publication_epoch, '2');
      assert.equal(documents[0].release_id, published.result.entity_id);
      const [[publishedEvent]] = await facts.query(
        'SELECT CAST(aggregate_seq AS CHAR) sequence,event_type,payload FROM outbox WHERE event_id=?',
        [publication.operation_id],
      );
      assert.equal(publishedEvent.sequence, '1');
      assert.equal(publishedEvent.event_type, 'DOCUMENT_PUBLISHED');
      const payload =
        typeof publishedEvent.payload === 'string'
          ? JSON.parse(publishedEvent.payload)
          : publishedEvent.payload;
      assert.equal(payload.release_id, documents[0].release_id);
      assert.equal(payload.publication_epoch, '2');
      const receipt = await waitSearchReceipt(search, publication.operation_id);
      assertOriginalReceipt(receipt, publication.operation_id, document.id);
      await indexedRange(search, await visibilityRows(facts, visibility, documents), documents);
    });
  },
);

test(
  'KB先提交：旧范围准备的RELEASE_CREATE拒绝无版本和事件，新资格发布从KB水位续接',
  { timeout: 120000 },
  async () => {
    const fixture = await organizationFixture(0);
    const document = await documentFixture(fixture);
    const publication = await releaseInput(document.id, fixture.owner);
    const visibility = await visibilityInput(fixture);
    const { firstResponse, secondResponse } = await orderedRequests({
      firstOperation: visibility.operation_id,
      secondOperation: publication.operation_id,
      table: 'knowledge_base',
      id: fixture.kb,
      first: () => visibilityRequest(fixture, visibility),
      second: () =>
        post(`/v1/documents/${document.id}/releases`, publication, fixture.owner.headers),
    });
    assertOldScopeRejected(secondResponse);
    await visibilitySucceeded(firstResponse, fixture);
    await withFacts(async (facts, search) => {
      const before = await knowledgeBaseDocuments(facts, fixture.kb);
      assert.equal(before.length, 1);
      assert.equal(before[0].sequence, '1');
      assert.equal(before[0].publication_epoch, '1');
      assert.equal(before[0].release_id, null);
      await assertNoOperationWrites(facts, publication.operation_id, true);
      await indexedRange(search, await visibilityRows(facts, visibility, before), before);
      const fresh = await releaseInput(document.id, fixture.owner);
      const published = await publishRelease(document.id, fixture.owner, fresh);
      const after = await knowledgeBaseDocuments(facts, fixture.kb);
      assert.equal(after[0].sequence, '2');
      assert.equal(after[0].publication_epoch, '2');
      assert.equal(after[0].release_id, published.result.entity_id);
      const receipt = await waitSearchReceipt(search, fresh.operation_id);
      assertOriginalReceipt(receipt, fresh.operation_id, document.id);
      assert.equal(receipt.sequence, '2');
      const projection = await searchProjection(document.id, receipt.physical);
      assert.equal(projection.sequence, '2');
      assert.equal(projection.release_id, after[0].release_id);
      assert.equal(projection.visible, true);
      assert.equal(projection.visibility_policy, 'COLLABORATORS');
      await assertKafkaCommitted(receipt);
      const [[count]] = await facts.query(
        "SELECT CAST(COUNT(*) AS CHAR) count FROM outbox WHERE aggregate_kind='DOCUMENT' AND aggregate_id=?",
        [document.id],
      );
      assert.equal(count.count, '2', 'STALE_PUBLICATION_OR_KB_EVENT_DUPLICATED');
    });
  },
);
