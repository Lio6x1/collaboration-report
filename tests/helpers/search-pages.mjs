import assert from 'node:assert/strict';
import { createHash, randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
  organizationFixture,
  documentFixture,
  createUser,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from './workspace.mjs';
import { releaseInput, publishRelease } from './publication.mjs';
import { openReviewWorkspace, writeDraft, reviewDatabase } from './review.mjs';
import { unlockStructurePreview } from './structure-previews.mjs';
import {
  searchDatabase,
  searchConfiguration,
  waitSearchReceipt,
  searchProjection,
  assertKafkaCommitted,
} from './search.mjs';
import { pause, root, runtime } from '../../scripts/runtime-lib.mjs';

export async function pageVisibility(path, owner, policy) {
  const current = await get(path, owner.headers);
  assert.equal(current.status, 200, json(current).code);
  if (json(current).visibility_policy === policy) return null;
  const input = {
    operation_id: operation('search-page-visibility'),
    expected_visibility_revision: json(current).visibility_revision,
    policy,
  };
  const changed = await post(path + '/visibility', input, owner.headers);
  assert.equal(changed.status, 202, json(changed).code);
  assert.equal((await waitAuthorization(changed, owner)).state, 'SUCCEEDED');
  return input.operation_id;
}

async function rename(document, owner, title) {
  const response = await get(`/v1/documents/${document}`, owner.headers);
  assert.equal(response.status, 200, json(response).code);
  const changed = await post(
    `/v1/documents/${document}`,
    {
      operation_id: operation('search-page-title'),
      expected_control_revision: json(response).control_revision,
      title,
    },
    owner.headers,
    'PATCH',
  );
  assert.equal(changed.status, 200, json(changed).code);
}

// 消费成功不等于ES refresh已可搜索。只读_count有界等待全部七项可搜索；
// 不主动refresh、不改索引配置，也不重试产品搜索的错误来掩盖业务失败。
export async function waitSearchPagesQueryable(physical, query, count, document) {
  assert.match(physical, /^[a-z0-9][a-z0-9._-]{0,159}$/);
  const { search } = searchConfiguration();
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    const response = await fetch(`${search.elasticEndpoint}/${physical}/_count`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({
        query: {
          bool: {
            filter: [
              { term: { visible: true } },
              ...(document ? [{ term: { document_id: document } }] : []),
            ],
            must: [{ multi_match: { query, fields: ['title', 'text', 'chapters.title'] } }],
          },
        },
      }),
      signal: AbortSignal.timeout(3000),
    });
    assert.equal(response.status, 200, 'SEARCH_PAGE_READINESS_FAILED');
    const reader = response.body.getReader();
    const chunks = [];
    let size = 0;
    try {
      while (true) {
        const part = await reader.read();
        if (part.done) break;
        size += part.value.byteLength;
        if (size > 4096) {
          await reader.cancel();
          assert.fail('SEARCH_PAGE_READINESS_RESPONSE_LIMIT');
        }
        chunks.push(Buffer.from(part.value));
      }
    } finally {
      reader.releaseLock();
    }
    const value = JSON.parse(Buffer.concat(chunks, size).toString('utf8'));
    assert.equal(value._shards.failed, 0);
    assert.equal(value._shards.successful, value._shards.total);
    assert.ok(Number.isInteger(value.count) && value.count >= 0 && value.count <= count);
    if (value.count === count) return;
    await pause(100);
  }
  assert.fail('SEARCH_PAGE_FIXTURE_NOT_SEARCHABLE');
}

// 可供最终复核/输出测试复用两篇PUBLIC夹具。可选连接仍是Search私有连接；
// 不跨服务用Doc账号查询消费账本，也不隐式持有一整组SQL事务。
export async function publishSearchDocument(
  f,
  { q, title, body, policy = 'PUBLIC', mutateMain = false },
  searchDB,
) {
  assert.equal(typeof q, 'string');
  assert.ok(q && title.includes(q) && body.includes(q));
  const d = await documentFixture(f);
  await rename(d.id, f.owner, title);
  await pageVisibility(`/v1/documents/${d.id}`, f.owner, policy);
  await unlockStructurePreview({ f, d, user: f.owner });
  const main = await openReviewWorkspace(f, d, f.owner, 'MAIN');
  let ownedDB;
  try {
    await writeDraft(main, body);
    const input = await releaseInput(d.id, f.owner);
    const published = await publishRelease(d.id, f.owner, input);
    if (!searchDB) ownedDB = await searchDatabase();
    const receipt = await waitSearchReceipt(searchDB ?? ownedDB, input.operation_id);
    const projection = await searchProjection(d.id, receipt.physical);
    assert.equal(projection.release_id, published.result.entity_id);
    assert.equal(projection.title, title);
    assert.equal(projection.visible, true);
    await waitSearchPagesQueryable(receipt.physical, q, 1, d.id);
    if (mutateMain) {
      await writeDraft(main, '未发布MAIN S2：不允许泄漏');
      await rename(d.id, f.owner, '管理标题S2：不允许覆盖正式标题');
    }
    return {
      id: d.id,
      release: published.result.entity_id,
      title,
      snippet: [...body].slice(0, 2000).join(''),
      chapter: d.main.structure.entries[0].chapter_id,
      space: f.space,
      kb: f.kb,
      policy,
      physical: receipt.physical,
      event: input.operation_id,
    };
  } finally {
    main.close();
    if (ownedDB) await ownedDB.end();
  }
}

export async function searchPageFixture() {
  const f = await organizationFixture(1);
  const outsider = await createUser();
  const directory = await get('/v1/me/spaces', f.owner.headers);
  assert.equal(directory.status, 200, json(directory).code);
  const personal = json(directory).items.find((space) => space.kind === 'PERSONAL');
  assert.ok(personal);
  await pageVisibility(`/v1/knowledge-bases/${f.kb}`, f.owner, 'PUBLIC');
  const second = await post(
    `/v1/spaces/${f.space}/knowledge-bases`,
    { operation_id: operation('search-page-kb'), name: '搜索页交集库' },
    f.owner.headers,
  );
  assert.equal(second.status, 201, json(second).code);
  const restrictedKB = json(second).result.entity_id;
  await pageVisibility(`/v1/knowledge-bases/${restrictedKB}`, f.owner, 'SPACE_MEMBERS');
  const q = 'searchpages' + randomBytes(12).toString('hex');
  const searchDB = await searchDatabase();
  const docs = [];
  try {
    const definitions = [
      ...Array.from({ length: 3 }, () => ({ space: f.space, kb: f.kb, policy: 'PUBLIC' })),
      { space: personal.id, kb: undefined, policy: 'PUBLIC' },
      { space: f.space, kb: f.kb, policy: 'SPACE_MEMBERS' },
      { space: f.space, kb: f.kb, policy: 'COLLABORATORS' },
      { space: f.space, kb: restrictedKB, policy: 'PUBLIC' },
    ];
    let physical;
    for (const [index, definition] of definitions.entries()) {
      const location = { ...f, space: definition.space, kb: definition.kb };
      const title = `固定正式标题${index + 1} ${q}`;
      const body =
        `正式正文${index + 1} ${q} ` + (index === 0 ? '中😀'.repeat(1100) : '当前正式版');
      const doc = await publishSearchDocument(
        location,
        { q, title, body, policy: definition.policy, mutateMain: index === 0 },
        searchDB,
      );
      assert.ok(!physical || physical === doc.physical, 'FIXTURE_GENERATION_CHANGED');
      physical = doc.physical;
      docs.push(doc);
    }
    // 原提交边界已有独立actual验收；只在整组末尾确认一次broker组位置，
    // 可复用的单篇helper不执行同步Docker CLI，以免阻塞最终变化测试的事件循环。
    await assertKafkaCommitted(await waitSearchReceipt(searchDB, docs.at(-1).event));
    const collaborator = docs[5];
    const current = await get(`/v1/documents/${collaborator.id}`, f.owner.headers);
    assert.equal(current.status, 200, json(current).code);
    await waitAuthorization(
      await post(
        `/v1/documents/${collaborator.id}/authorization-changes`,
        {
          operation_id: operation('search-page-read'),
          expected_acl_revision: json(current).acl_revision,
          changes: [
            {
              action: 'GRANT',
              grant: {
                user_id: f.members[0].id,
                scope_kind: 'DOCUMENT',
                scope_id: collaborator.id,
                capability: 'REVIEW',
              },
            },
          ],
        },
        f.owner.headers,
      ),
      f.owner,
    );
    await waitSearchPagesQueryable(physical, q, 7);
    return { ...f, outsider, personal: personal.id, restrictedKB, q, docs };
  } finally {
    await searchDB.end();
  }
}

export function searchPagePath(q, { limit = 20, cursor, space } = {}) {
  const params = new URLSearchParams({ q, limit: String(limit) });
  if (cursor !== undefined) params.set('cursor', cursor);
  if (space !== undefined) params.set('space_id', space);
  return '/v1/search?' + params;
}

export async function checkedSearchPage(f, user, options = {}) {
  const response = await get(searchPagePath(f.q, options), user?.headers ?? {});
  assert.equal(response.status, 200, json(response).code ?? 'SEARCH_HTTP_NOT_OK');
  const page = json(response);
  assert.deepEqual(Object.keys(page).sort(), ['has_more', 'items', 'next_cursor']);
  assert.ok(Array.isArray(page.items) && page.items.length <= (options.limit ?? 20));
  assert.equal(typeof page.has_more, 'boolean');
  if (page.has_more) {
    assert.equal(typeof page.next_cursor, 'string');
    assert.match(page.next_cursor, /^[A-Za-z0-9_-]+$/);
    assert.ok(page.next_cursor.length <= 2048);
  } else assert.equal(page.next_cursor, null);
  const seen = new Set();
  for (const item of page.items) {
    assert.deepEqual(Object.keys(item).sort(), [
      'chapter_id',
      'document_id',
      'release_id',
      'snippet',
      'title',
    ]);
    const fixed = f.docs.find((d) => d.id === item.document_id);
    assert.ok(fixed, 'SEARCH_RETURNED_FOREIGN_DOCUMENT');
    assert.ok(!seen.has(item.document_id), 'SEARCH_PAGE_DUPLICATE_DOCUMENT');
    seen.add(item.document_id);
    assert.equal(item.release_id, fixed.release);
    assert.equal(item.title, fixed.title);
    assert.equal(item.snippet, fixed.snippet);
    assert.equal(item.chapter_id, fixed.chapter);
    assert.ok([...item.snippet].length <= 2000);
  }
  return page;
}

export function assertPageIDs(page, documents) {
  assert.deepEqual(
    page.items.map((item) => item.document_id).sort(),
    documents.map((d) => d.id).sort(),
  );
  assert.equal(page.has_more, false);
  assert.equal(page.next_cursor, null);
}

export async function outputLeaseEvidence(db, watermark, upper, returned, lookahead) {
  // state().owner属于持久容器环境，不是每轮start生成的进程owner。只接受本项目
  // 当前登记的两个精确实例，再与Doc私有output_instance的PID/boot/state交叉绑定；
  // 不能从全库最新实例或另一次运行的同名角色推断本次响应属于谁。
  const registry = JSON.parse(readFileSync(resolve(runtime, 'processes.json'), 'utf8'));
  assert.ok(Array.isArray(registry), 'SEARCH_PAGE_PROCESS_REGISTRY_INVALID');
  const expected = ['document', 'edge'].map((role) => {
    const matches = registry.filter((process) => process.role === role);
    assert.equal(matches.length, 1, 'SEARCH_PAGE_CURRENT_PROCESS_NOT_UNIQUE');
    const process = matches[0];
    assert.ok(Number.isSafeInteger(process.pid) && process.pid > 0);
    assert.match(process.owner, /^[a-f0-9]{8}-(?:[a-f0-9]{4}-){3}[a-f0-9]{12}$/);
    const entry =
      role === 'document'
        ? resolve(runtime, 'bin/document.exe')
        : resolve(root, 'scripts/edge.mjs');
    assert.equal(
      resolve(process.entry).toLowerCase(),
      entry.toLowerCase(),
      'SEARCH_PAGE_FOREIGN_PROCESS_ENTRY',
    );
    return {
      id: process.owner + '-' + role,
      pid: String(process.pid),
      boot: createHash('sha256')
        .update(role + '\n' + process.owner)
        .digest('hex'),
    };
  });
  const [instances] = await db.query(
    'SELECT instance_id,CAST(pid AS CHAR) pid,LOWER(HEX(boot_digest)) boot,state FROM output_instance WHERE instance_id IN (?,?)',
    expected.map((process) => process.id),
  );
  assert.equal(instances.length, 2, 'SEARCH_PAGE_CURRENT_OUTPUT_INSTANCES_REQUIRED');
  for (const process of expected) {
    const instance = instances.find((row) => row.instance_id === process.id);
    assert.ok(instance, 'SEARCH_PAGE_CURRENT_OUTPUT_INSTANCE_MISSING');
    assert.equal(instance.pid, process.pid, 'SEARCH_PAGE_OUTPUT_PROCESS_PID_CHANGED');
    assert.equal(instance.boot, process.boot, 'SEARCH_PAGE_OUTPUT_PROCESS_BOOT_CHANGED');
    assert.equal(instance.state, 'RUNNING', 'SEARCH_PAGE_OUTPUT_PROCESS_NOT_RUNNING');
  }
  const [rows] = await db.query(
    "SELECT t.instance_id,t.business_key,t.payload FROM durable_task t JOIN output_instance i ON i.instance_id=t.instance_id WHERE t.kind='OUTPUT_LEASE' AND t.id>? AND t.id<=? AND ((i.instance_id=? AND i.pid=?) OR (i.instance_id=? AND i.pid=?)) AND i.state='RUNNING' ORDER BY t.id LIMIT 32",
    [watermark, upper, expected[0].id, expected[0].pid, expected[1].id, expected[1].pid],
  );
  const payloads = rows.map((row) => {
    const payload = typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload;
    assert.equal(
      payload.instance_id,
      row.instance_id,
      'SEARCH_PAGE_LEASE_INSTANCE_BINDING_CHANGED',
    );
    assert.equal(payload.lease_id, row.business_key, 'SEARCH_PAGE_LEASE_ID_BINDING_CHANGED');
    return payload;
  });
  const leases = payloads.filter((p) =>
    p.sources?.some((s) => s.scope_key === `publication:${returned.id}`),
  );
  assert.equal(leases.length, 2, 'SEARCH_PAGE_REAL_OUTPUT_PAIR_MISSING');
  assert.deepEqual(
    leases.map((p) => p.instance_id).sort(),
    expected.map((process) => process.id).sort(),
  );
  assert.deepEqual(leases[0].sources, leases[1].sources);
  for (const lease of leases) {
    assert.ok(
      lease.document_id === undefined || lease.document_id === 0 || lease.document_id === '0',
    );
    assert.ok(lease.admission_id === undefined || lease.admission_id === '');
    assert.ok(lease.handler_id && lease.lease_id);
    assert.equal(new Set(lease.sources.map((s) => s.scope_key)).size, lease.sources.length);
    for (const source of lease.sources) assert.match(source.epoch, /^[1-9][0-9]*$/);
    assert.deepEqual(
      lease.sources
        .filter((s) => s.scope_key.startsWith('publication:'))
        .map((s) => s.scope_key)
        .sort(),
      [returned, lookahead].map((d) => `publication:${d.id}`).sort(),
    );
    for (const doc of [returned, lookahead]) {
      const [[current]] = await db.query(
        'SELECT CAST(acl_revision AS CHAR) acl,CAST(publication_epoch AS CHAR) publication FROM document WHERE id=?',
        [doc.id],
      );
      assert.ok(
        lease.sources.some((s) => s.scope_key === `doc-acl:${doc.id}` && s.epoch === current.acl),
      );
      assert.ok(
        lease.sources.some(
          (s) => s.scope_key === `publication:${doc.id}` && s.epoch === current.publication,
        ),
      );
      if (doc.kb) {
        const [[kb]] = await db.query(
          'SELECT CAST(acl_revision AS CHAR) acl FROM knowledge_base WHERE id=?',
          [doc.kb],
        );
        assert.ok(
          lease.sources.some((s) => s.scope_key === `kb-acl:${doc.kb}` && s.epoch === kb.acl),
        );
      }
    }
  }
  // 此处只证明Go/Edge联合来源真实持久登记。请求已结束，不能冒充在途排空ACK验收。
  return leases[0].sources.length;
}

export { reviewDatabase };

export async function waitKBVisibilityConsumption(f, event) {
  assert.ok(event);
  const db = await searchDatabase();
  try {
    const id =
      'visibility_index_' +
      createHash('sha256')
        .update(event + '\n' + f.docs[6].id)
        .digest('hex');
    const receipt = await waitSearchReceipt(db, id);
    assert.equal((await searchProjection(f.docs[6].id, receipt.physical)).visible, true);
  } finally {
    await db.end();
  }
}
