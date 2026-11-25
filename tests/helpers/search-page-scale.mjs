import assert from 'node:assert/strict';
import { createHash, randomBytes } from 'node:crypto';
import { canonical, normalizeContent, resourceIds } from '../../packages/content/index.mjs';

const TOTAL = 2305;
const MAX_GROUP = 100;
const MAX_BYTES = 256 << 10;
const RELEASE = '`release`';
const sha = (value) => createHash('sha256').update(value).digest();

function id(value) {
  assert.equal(typeof value, 'string', 'SCALE_ID_MUST_REMAIN_STRING');
  assert.match(value, /^[1-9][0-9]{0,19}$/);
  assert.ok(BigInt(value) <= 18446744073709551615n, 'SCALE_ID_OVERFLOW');
  return value;
}

export function partitionScaleIDs(ids) {
  assert.equal(ids.length, TOTAL);
  assert.equal(new Set(ids).size, TOTAL);
  ids.forEach(id);
  // ES document_id为keyword；数值自增跨位数后不再与字典序一致。
  const sorted = [...ids].sort();
  return { denied: sorted.slice(0, 1200), allowed: sorted.slice(1200) };
}

export function scaleProvenance(row, operation, block, text) {
  ['unit', 'manifest', 'version'].forEach((key) => id(row[key]));
  assert.match(operation, /^[A-Za-z0-9][A-Za-z0-9_-]{0,95}$/);
  assert.match(block, /^[A-Za-z0-9][A-Za-z0-9_-]{0,95}$/);
  return {
    schema_version: 'provenance-v1',
    lineage_id: row.unit,
    base_manifest_id: row.manifest,
    base_content_version_id: row.version,
    source_operation_id: operation,
    accepted_captures: [],
    block_sources: [
      {
        block_id: block,
        origin_content_version_id: row.version,
        origin_block_id: block,
        relation: 'INSERTED',
        source_range: { start: 0, end: [...text].length },
      },
    ],
  };
}

export function boundedScaleGroups(rows, encode, maximum, count = MAX_GROUP) {
  assert.ok(Number.isInteger(maximum) && maximum > 0);
  assert.ok(Number.isInteger(count) && count > 0 && count <= MAX_GROUP);
  const groups = [];
  let group = [];
  for (const row of rows) {
    const candidate = [...group, row];
    if (candidate.length > count || Buffer.byteLength(encode(candidate)) > maximum) {
      assert.ok(group.length, 'SCALE_SINGLE_ROW_EXCEEDS_PACKET');
      groups.push(group);
      group = [row];
      assert.ok(Buffer.byteLength(encode(group)) <= maximum, 'SCALE_SINGLE_ROW_EXCEEDS_PACKET');
    } else group = candidate;
  }
  if (group.length) groups.push(group);
  return groups;
}

function queryShape(q) {
  return {
    bool: {
      filter: [{ term: { visible: true } }],
      must: [{ multi_match: { query: q, fields: ['title', 'text', 'chapters.title'] } }],
    },
  };
}

async function es(endpoint, method, path, value, maximum = 1 << 20, ndjson = false) {
  const response = await fetch(endpoint.replace(/\/$/, '') + path, {
    method,
    headers: { 'content-type': ndjson ? 'application/x-ndjson' : 'application/json' },
    ...(value === undefined ? {} : { body: ndjson ? value : JSON.stringify(value) }),
    signal: AbortSignal.timeout(3000),
  });
  const chunks = [];
  const reader = response.body.getReader();
  let bytes = 0;
  try {
    for (;;) {
      const part = await reader.read();
      if (part.done) break;
      bytes += part.value.length;
      if (bytes > maximum) {
        await reader.cancel();
        assert.fail('SCALE_ES_RESPONSE_LIMIT');
      }
      chunks.push(Buffer.from(part.value));
    }
  } finally {
    reader.releaseLock();
  }
  assert.equal(response.status, 200, 'SCALE_ES_REQUEST_FAILED');
  return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(Buffer.concat(chunks, bytes)));
}

function completeShards(value) {
  assert.ok(value && Number.isInteger(value.total) && value.total > 0, 'SCALE_SHARDS_MISSING');
  assert.equal(value.successful, value.total, 'SCALE_PARTIAL_SHARDS');
  assert.equal(value.failed, 0, 'SCALE_FAILED_SHARD');
}

// 参数依然由mysql2编码。批次预算计算的是最终转义后SQL，包含BLOB的hex膨胀，
// 不是只量原正文；setup不改变max_allowed_packet，也不放宽业务HTTP/链期限。
async function insertRows(db, prefix, rows, maximum) {
  const encode = (part) => {
    const sql = prefix + part.map((row) => '(' + row.map(() => '?').join(',') + ')').join(',');
    return db.format(sql, part.flat());
  };
  for (const group of boundedScaleGroups(rows, encode, maximum)) {
    const sql = prefix + group.map((row) => '(' + row.map(() => '?').join(',') + ')').join(',');
    const [written] = await db.query(sql, group.flat());
    assert.equal(written.affectedRows, group.length, 'SCALE_SQL_INSERT_COUNT');
  }
}

async function ownedRows(db, rows, table, select, predicate, args) {
  const found = [];
  for (let offset = 0; offset < rows.length; offset += MAX_GROUP) {
    const group = rows.slice(offset, offset + MAX_GROUP);
    const [part] = await db.query(
      `SELECT ${select} FROM ${table} WHERE document_id IN (?) AND ${predicate}`,
      [group.map((row) => row.id), ...args],
    );
    assert.equal(part.length, group.length, 'SCALE_OWN_LAYER_COUNT');
    found.push(...part);
  }
  const byDoc = new Map(found.map((row) => [id(row.document), id(row.entity)]));
  assert.equal(byDoc.size, rows.length);
  return byDoc;
}

async function buildGraph(f, db, template) {
  const [[packet]] = await db.query('SELECT @@max_allowed_packet maximum,@@foreign_key_checks fk');
  assert.equal(Number(packet.fk), 1, 'SCALE_FOREIGN_KEYS_REQUIRED');
  const maximum = Math.min(MAX_BYTES, Math.floor(Number(packet.maximum) / 4));
  assert.ok(maximum >= 16384, 'SCALE_PACKET_TOO_SMALL');
  const body = normalizeContent(JSON.parse(template.canonical_body.toString('utf8')), {
    workspaceKind: 'MAIN',
  });
  assert.equal(canonical(body), template.canonical_body.toString('utf8'));
  assert.deepEqual(resourceIds(body), []);
  assert.equal(body.type, 'doc');
  assert.equal(body.content.length, 1);
  const paragraph = body.content[0];
  assert.equal(paragraph.type, 'paragraph');
  assert.equal(paragraph.content.length, 1);
  assert.equal(paragraph.content[0].type, 'text');
  assert.equal(paragraph.content[0].text, f.templateText);
  paragraph.content[0].text = f.text;
  const normalized = normalizeContent(body, { workspaceKind: 'MAIN' });
  f.body = Buffer.from(canonical(normalized));
  f.block = paragraph.attrs.id;
  f.bodyHash = sha(f.body);
  assert.ok(f.body.length < 4096);
  f.style = canonical(JSON.parse(template.style_json));
  f.chapterTitle = template.chapter_title;

  await db.beginTransaction();
  f.transaction = true;
  await insertRows(
    db,
    'INSERT INTO document(space_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES',
    Array.from({ length: TOTAL }, () => [
      f.space,
      f.owner.id,
      f.marker,
      'COLLABORATORS',
      'INITIALIZING',
      true,
      'UNPUBLISHED',
    ]),
    maximum,
  );
  const [documents] = await db.query(
    "SELECT CAST(id AS CHAR) id FROM document WHERE space_id=? AND responsible_user_id=? AND title=? AND lifecycle='INITIALIZING' LIMIT 2306",
    [f.space, f.owner.id, f.marker],
  );
  const divided = partitionScaleIDs(documents.map((row) => row.id));
  f.denied = divided.denied;
  f.allowed = divided.allowed;
  const denied = new Set(f.denied);
  f.rows = [...f.denied, ...f.allowed].map((document) => ({
    id: document,
    policy: denied.has(document) ? 'COLLABORATORS' : 'PUBLIC',
  }));

  await insertRows(
    db,
    'INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES',
    f.rows.map((row) => [row.id, f.operation, 'structure-v1']),
    maximum,
  );
  await insertRows(
    db,
    'INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES',
    f.rows.map((row) => [row.id, f.style, 'style-v1', f.operation]),
    maximum,
  );
  for (const [table, key] of [
    ['structure_revision', 'structure'],
    ['style_revision', 'style'],
  ]) {
    const mapping = await ownedRows(
      db,
      f.rows,
      table,
      'CAST(document_id AS CHAR) document,CAST(id AS CHAR) entity',
      'source_operation_id=?',
      [f.operation],
    );
    f.rows.forEach((row) => {
      row[key] = mapping.get(row.id);
    });
  }
  await insertRows(
    db,
    'INSERT INTO chapter(document_id,created_structure_id) VALUES',
    f.rows.map((row) => [row.id, row.structure]),
    maximum,
  );
  const chapters = await ownedRows(
    db,
    f.rows,
    'chapter',
    'CAST(document_id AS CHAR) document,CAST(id AS CHAR) entity',
    'retired_structure_id IS NULL',
    [],
  );
  f.rows.forEach((row) => {
    row.chapter = chapters.get(row.id);
  });
  await insertRows(
    db,
    'INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES',
    f.rows.map((row) => [row.id, row.chapter, 'CHAPTER', row.structure]),
    maximum,
  );
  const units = await ownedRows(
    db,
    f.rows,
    'collaboration_unit',
    'CAST(document_id AS CHAR) document,CAST(id AS CHAR) entity',
    "kind='CHAPTER' AND active_branch_id IS NULL",
    [],
  );
  f.rows.forEach((row) => {
    row.unit = units.get(row.id);
  });
  await insertRows(
    db,
    'INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES',
    f.rows.map((row) => [
      row.structure,
      row.id,
      row.chapter,
      row.chapter,
      'CHAPTER',
      f.chapterTitle,
      0,
      row.unit,
    ]),
    maximum,
  );
  await insertRows(
    db,
    'INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES',
    f.rows.map((row) => [
      row.id,
      row.structure,
      row.style,
      f.operation,
      'SEARCH_PAGE_SCALE',
      f.operation,
    ]),
    maximum,
  );
  const manifests = await ownedRows(
    db,
    f.rows,
    'main_manifest',
    'CAST(document_id AS CHAR) document,CAST(id AS CHAR) entity',
    'source_operation_id=?',
    [f.operation],
  );
  f.rows.forEach((row) => {
    row.manifest = manifests.get(row.id);
  });
  const versionColumns =
    'document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_kind,source_ref,provenance_json,provenance_schema';
  const versionValues = (row, provenance) => [
    row.id,
    row.unit,
    row.structure,
    f.body.toString('utf8'),
    f.body,
    f.bodyHash,
    'content-v1',
    'canonical-v1',
    f.body.length,
    f.operation,
    'INITIAL',
    'scale-' + row.id,
    provenance,
    'provenance-v1',
  ];
  // 先在尚未提交且没有清单引用的原事务内取得AUTO_INCREMENT完整字符串ID。
  // immutable账号无UPDATE；精确删除/重插原自有版本后才安装清单/正式指针。
  await insertRows(
    db,
    `INSERT INTO content_version(${versionColumns}) VALUES`,
    f.rows.map((row) => versionValues(row, '{}')),
    maximum,
  );
  const versions = await ownedRows(
    db,
    f.rows,
    'content_version',
    'CAST(document_id AS CHAR) document,CAST(id AS CHAR) entity',
    'source_operation_id=?',
    [f.operation],
  );
  f.rows.forEach((row) => {
    row.version = versions.get(row.id);
    row.provenance = canonical(scaleProvenance(row, f.operation, f.block, f.text));
  });
  for (let offset = 0; offset < f.rows.length; offset += MAX_GROUP) {
    const group = f.rows.slice(offset, offset + MAX_GROUP);
    const [removed] = await db.query(
      'DELETE FROM content_version WHERE id IN (?) AND document_id IN (?) AND source_operation_id=?',
      [group.map((row) => row.version), group.map((row) => row.id), f.operation],
    );
    assert.equal(removed.affectedRows, group.length);
  }
  await insertRows(
    db,
    `INSERT INTO content_version(id,${versionColumns}) VALUES`,
    f.rows.map((row) => [row.version, ...versionValues(row, row.provenance)]),
    maximum,
  );
  await insertRows(
    db,
    'INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES',
    f.rows.map((row) => [row.manifest, row.id, row.unit, row.version]),
    maximum,
  );
  await insertRows(
    db,
    'INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES',
    f.rows.map((row) => [row.id, row.manifest, f.title, f.owner.id, f.operation + '-' + row.id]),
    maximum,
  );
  const releases = await ownedRows(
    db,
    f.rows,
    '`release`',
    'CAST(document_id AS CHAR) document,CAST(id AS CHAR) entity',
    'publisher_id=?',
    [f.owner.id],
  );
  f.rows.forEach((row) => {
    row.release = releases.get(row.id);
  });
  for (let offset = 0; offset < f.rows.length; offset += MAX_GROUP) {
    const group = f.rows.slice(offset, offset + MAX_GROUP);
    const [installed] = await db.query(
      `UPDATE document d
      JOIN structure_revision s ON s.document_id=d.id AND s.source_operation_id=?
      JOIN style_revision y ON y.document_id=d.id AND y.source_operation_id=?
      JOIN main_manifest m ON m.document_id=d.id AND m.source_operation_id=?
      JOIN ${RELEASE} r ON r.document_id=d.id AND r.manifest_id=m.id AND r.operation_id=CONCAT(?,'-',CAST(d.id AS CHAR))
      SET d.lifecycle='ACTIVE',d.current_structure_id=s.id,d.current_style_id=y.id,
        d.current_manifest_id=m.id,d.current_release_id=r.id,d.publication_state='PUBLISHED',d.event_seq=1,
        d.visibility_policy=CASE WHEN d.id IN (?) THEN 'COLLABORATORS' ELSE 'PUBLIC' END
      WHERE d.id IN (?) AND d.space_id=? AND d.responsible_user_id=? AND d.title=? AND d.lifecycle='INITIALIZING'`,
      [
        f.operation,
        f.operation,
        f.operation,
        f.operation,
        group
          .filter((row) => row.policy === 'COLLABORATORS')
          .map((row) => row.id)
          .concat('0'),
        group.map((row) => row.id),
        f.space,
        f.owner.id,
        f.marker,
      ],
    );
    assert.equal(installed.affectedRows, group.length, 'SCALE_POINTER_INSTALL_COUNT');
  }
  await verifyGraph(f, db);
  f.commitAttempted = true;
  await db.commit();
  f.transaction = false;
  f.docs = f.rows.map((row) => ({
    id: row.id,
    release: row.release,
    title: f.title,
    snippet: f.text,
    chapter: row.chapter,
    physical: f.physical,
    policy: row.policy,
  }));
}

async function verifyGraph(f, db) {
  for (let offset = 0; offset < f.rows.length; offset += MAX_GROUP) {
    const group = f.rows.slice(offset, offset + MAX_GROUP);
    const [facts] = await db.query(
      `SELECT CAST(d.id AS CHAR) id,d.visibility_policy policy,d.lifecycle,d.publication_state,
      CAST(d.event_seq AS CHAR) seq,CAST(d.acl_revision AS CHAR) acl,CAST(d.publication_epoch AS CHAR) epoch,
      CAST(s.id AS CHAR) structure,CAST(y.id AS CHAR) style,CAST(m.id AS CHAR) manifest,
      CAST(r.id AS CHAR) release_id,CAST(e.chapter_id AS CHAR) chapter,CAST(e.unit_id AS CHAR) unit,
      CAST(v.id AS CHAR) version,r.title,e.title chapter_title,y.style_json,
      v.body_json,v.canonical_body,v.body_hash,CAST(v.byte_size AS CHAR) byte_size,v.provenance_json,
      s.schema_version structure_schema,y.schema_version style_schema,v.schema_version body_schema,
      v.canonicalization_version,v.provenance_schema
      FROM document d JOIN structure_revision s ON s.id=d.current_structure_id AND s.document_id=d.id
      JOIN style_revision y ON y.id=d.current_style_id AND y.document_id=d.id
      JOIN main_manifest m ON m.id=d.current_manifest_id AND m.document_id=d.id AND m.structure_id=s.id AND m.style_id=y.id
      JOIN ${RELEASE} r ON r.id=d.current_release_id AND r.document_id=d.id AND r.manifest_id=m.id
      JOIN structure_entry e ON e.structure_id=s.id AND e.document_id=d.id
      JOIN chapter c ON c.id=e.chapter_id AND c.document_id=d.id AND c.created_structure_id=s.id
      JOIN collaboration_unit u ON u.id=e.unit_id AND u.document_id=d.id AND u.chapter_id=c.id AND u.created_structure_id=s.id
      JOIN manifest_entry me ON me.manifest_id=m.id AND me.document_id=d.id AND me.unit_id=u.id
      JOIN content_version v ON v.id=me.content_version_id AND v.unit_id=u.id AND v.document_id=d.id AND v.structure_id=s.id
      WHERE d.id IN (?) AND d.space_id=? AND d.responsible_user_id=? AND d.title=?
        AND s.source_operation_id=? AND y.source_operation_id=? AND m.source_operation_id=? AND v.source_operation_id=?`,
      [group.map((row) => row.id), f.space, f.owner.id, f.marker, ...Array(4).fill(f.operation)],
    );
    assert.equal(facts.length, group.length, 'SCALE_FULL_GRAPH_COUNT');
    for (const fact of facts) {
      const row = group.find((item) => item.id === fact.id);
      assert.ok(row);
      for (const key of ['structure', 'style', 'manifest', 'chapter', 'unit', 'version'])
        assert.equal(fact[key], row[key]);
      assert.equal(fact.release_id, row.release);
      assert.equal(fact.policy, row.policy);
      assert.equal(fact.lifecycle, 'ACTIVE');
      assert.equal(fact.publication_state, 'PUBLISHED');
      for (const key of ['seq', 'acl', 'epoch']) assert.equal(fact[key], '1');
      assert.equal(fact.title, f.title);
      assert.equal(fact.chapter_title, f.chapterTitle);
      assert.equal(fact.structure_schema, 'structure-v1');
      assert.equal(fact.style_schema, 'style-v1');
      assert.equal(fact.body_schema, 'content-v1');
      assert.equal(fact.canonicalization_version, 'canonical-v1');
      assert.equal(fact.provenance_schema, 'provenance-v1');
      assert.deepEqual(fact.canonical_body, f.body);
      assert.deepEqual(fact.body_hash, f.bodyHash);
      assert.equal(fact.byte_size, String(f.body.length));
      for (const [key, expected] of [
        ['body_json', f.body.toString('utf8')],
        ['provenance_json', row.provenance],
        ['style_json', f.style],
      ]) {
        const value = typeof fact[key] === 'string' ? JSON.parse(fact[key]) : fact[key];
        assert.equal(canonical(value), expected, 'SCALE_JSON_SEMANTICS_CHANGED');
      }
    }
  }
}

function projection(f, row) {
  // 明确是样本指纹，不伪造Doc认证快照digest或Search的消费者/应用账本。
  return {
    document_id: row.id,
    release_id: row.release,
    sequence: '1',
    sequence_key: '00000000000000000001',
    authority_digest: sha(
      'SEARCH_PAGE_SCALE_SAMPLE\n' + f.operation + '\n' + row.id + '\n' + row.release,
    ).toString('hex'),
    visible: true,
    visibility_policy: row.policy,
    title: f.title,
    text: f.text + '\n',
    chapters: [{ unit_id: row.unit, title: f.chapterTitle, text: f.text }],
  };
}

async function seedES(f) {
  const mapping = await es(f.endpoint, 'GET', '/' + f.physical + '/_mapping', undefined);
  assert.deepEqual(Object.keys(mapping), [f.physical]);
  assert.equal(mapping[f.physical].mappings._meta.application, 'collaboration-report');
  assert.equal(mapping[f.physical].mappings._meta.owner, f.elasticOwner);
  assert.equal(mapping[f.physical].mappings._meta.schema, 'publication-index-v1-uint64');
  assert.equal(mapping[f.physical].mappings.dynamic, 'strict');
  const encode = (group) =>
    group
      .map(
        (row) =>
          JSON.stringify({ create: { _index: f.physical, _id: row.id } }) +
          '\n' +
          JSON.stringify(projection(f, row)) +
          '\n',
      )
      .join('');
  for (const group of boundedScaleGroups(f.rows, encode, MAX_BYTES)) {
    // 请求返回未知也记住全部尝试ID；finally只能核验原指纹后删已确定自有记录。
    f.esAttempted.push(...group);
    const result = await es(f.endpoint, 'POST', '/_bulk', encode(group), 1 << 20, true);
    assert.equal(result.errors, false, 'SCALE_ES_BULK_FAILED');
    assert.equal(result.items.length, group.length);
    result.items.forEach((item, index) => {
      assert.deepEqual(Object.keys(item), ['create']);
      assert.equal(item.create.status, 201);
      assert.equal(item.create._index, f.physical);
      assert.equal(item.create._id, group[index].id);
      assert.equal(item.create.error, undefined);
    });
  }
  const { waitSearchPagesQueryable } = await import('./search-pages.mjs');
  await waitSearchPagesQueryable(f.physical, f.q, TOTAL);
  const sorted = await es(
    f.endpoint,
    'POST',
    '/' + f.physical + '/_search?allow_partial_search_results=false',
    {
      size: 1201,
      track_total_hits: true,
      _source: ['document_id', 'release_id', 'sequence'],
      // document_id唯一，前两级已经决定全部顺序；_shard_doc只属于实际PIT，
      // 准备阶段不额外开PIT/游标。真实入口仍使用冻结的完整三元组排序。
      query: queryShape(f.q),
      sort: [{ _score: 'desc' }, { document_id: 'asc' }],
    },
  );
  completeShards(sorted._shards);
  assert.equal(sorted.timed_out, false);
  assert.notEqual(sorted.terminated_early, true);
  assert.equal(sorted.hits.total.relation, 'eq');
  assert.equal(sorted.hits.total.value, TOTAL);
  assert.equal(sorted.hits.hits.length, 1201);
  const expected = [...f.denied, f.allowed[0]];
  sorted.hits.hits.forEach((hit, index) => {
    const row = f.rows[index];
    assert.equal(hit._index, f.physical);
    assert.equal(hit._id, expected[index]);
    assert.equal(hit._source.document_id, row.id);
    assert.equal(hit._source.release_id, row.release);
    assert.equal(hit._source.sequence, '1');
    assert.equal(hit._score, sorted.hits.hits[0]._score, 'SCALE_QUERY_SCORES_MUST_MATCH');
    assert.equal(hit.sort[1], row.id);
  });
}

async function retireChain(f) {
  const { searchDatabase } = await import('./search.mjs');
  const { pause } = await import('../../scripts/runtime-lib.mjs');
  const db = await searchDatabase();
  try {
    const [rows] = await db.query(
      'SELECT cursor_hash,state,CAST(UNIX_TIMESTAMP(expires_at)*1000 AS UNSIGNED) expiry FROM cursor_registry WHERE query_hash=? AND session_ref IS NULL AND physical_index=? AND page_size=100',
      [sha(f.q), f.physical],
    );
    assert.ok(rows.length <= 1, 'SCALE_ONLY_ONE_PRODUCT_CHAIN');
    if (!rows.length && f.chain) {
      assert.ok(Date.now() >= f.chain.expiry, 'SCALE_ORIGINAL_CHAIN_REMOVED_EARLY');
    }
    for (const row of rows) {
      const expiry = Number(row.expiry);
      if (f.chain) {
        assert.deepEqual(row.cursor_hash, f.chain.hash, 'SCALE_CHAIN_ID_CHANGED');
        assert.equal(expiry, f.chain.expiry, 'SCALE_CHAIN_EXPIRY_MUST_NOT_EXTEND');
      }
      assert.ok(
        Number.isSafeInteger(expiry) && expiry <= Date.now() + 30000,
        'SCALE_ORIGINAL_EXPIRY_INVALID',
      );
      const deadline = expiry + 3000;
      let released = false;
      while (Date.now() < deadline) {
        const [current] = await db.query(
          'SELECT state,CAST(UNIX_TIMESTAMP(expires_at)*1000 AS UNSIGNED) expiry FROM cursor_registry WHERE cursor_hash=? AND query_hash=?',
          [row.cursor_hash, sha(f.q)],
        );
        if (current.length === 0) {
          // worker可以回收已到原期限的CLOSED行；不通过测试DELETE制造容量释放。
          assert.ok(Date.now() >= expiry, 'SCALE_CHAIN_REMOVED_BEFORE_ORIGINAL_EXPIRY');
          released = true;
          f.diagnostic({ kind: 'SCALE_CHAIN_NATURALLY_REMOVED', original_expiry: expiry });
          break;
        }
        assert.equal(Number(current[0].expiry), expiry, 'SCALE_CHAIN_EXPIRY_MUST_NOT_EXTEND');
        if (current[0].state === 'CLOSED') {
          released = true;
          f.diagnostic({ kind: 'SCALE_CHAIN_CLOSED', original_expiry: expiry });
          break;
        }
        await pause(50);
      }
      assert.equal(released, true, 'SCALE_CHAIN_CLOSE_UNKNOWN_KEEP_OWN_FIXTURE');
    }
  } finally {
    await db.end();
  }
}

async function removeES(f) {
  for (let offset = 0; offset < f.esAttempted.length; offset += MAX_GROUP) {
    const group = f.esAttempted.slice(offset, offset + MAX_GROUP);
    const found = await es(f.endpoint, 'POST', '/' + f.physical + '/_mget', {
      ids: group.map((row) => row.id),
    });
    assert.equal(found.docs.length, group.length);
    const owned = [];
    found.docs.forEach((entry, index) => {
      assert.equal(entry._index, f.physical);
      assert.equal(entry._id, group[index].id);
      assert.equal(entry.error, undefined);
      if (entry.found === false) return;
      assert.equal(entry.found, true);
      assert.deepEqual(
        entry._source,
        projection(f, group[index]),
        'SCALE_ES_OWNERSHIP_CHANGED_KEEP_RECORD',
      );
      assert.ok(Number.isSafeInteger(entry._seq_no) && entry._seq_no >= 0);
      assert.ok(Number.isSafeInteger(entry._primary_term) && entry._primary_term > 0);
      owned.push({ ...group[index], seq: entry._seq_no, term: entry._primary_term });
    });
    if (!owned.length) continue;
    const body = owned
      .map(
        (row) =>
          JSON.stringify({
            delete: {
              _index: f.physical,
              _id: row.id,
              if_seq_no: row.seq,
              if_primary_term: row.term,
            },
          }) + '\n',
      )
      .join('');
    const result = await es(f.endpoint, 'POST', '/_bulk', body, 1 << 20, true);
    assert.equal(result.errors, false);
    assert.equal(result.items.length, owned.length);
    result.items.forEach((item, index) => {
      assert.deepEqual(Object.keys(item), ['delete']);
      assert.equal(item.delete.status, 200);
      assert.equal(item.delete.result, 'deleted');
      assert.equal(item.delete._index, f.physical);
      assert.equal(item.delete._id, owned[index].id);
      assert.equal(item.delete.error, undefined);
    });
  }
}

async function removeGraph(f) {
  const { reviewDatabase } = await import('./review.mjs');
  const db = await reviewDatabase();
  try {
    await db.beginTransaction();
    // 原模板、账号、个人空间仍有真实产品依赖；仅处理记录在内存中且归属重新核验的样本图。
    await verifyGraph(f, db);
    for (let offset = 0; offset < f.rows.length; offset += MAX_GROUP) {
      const group = f.rows.slice(offset, offset + MAX_GROUP);
      const ids = group.map((row) => row.id);
      const [reset] = await db.query(
        `UPDATE document SET current_release_id=NULL,publication_state='UNPUBLISHED',
        current_manifest_id=NULL,current_structure_id=NULL,current_style_id=NULL WHERE id IN (?) AND space_id=? AND responsible_user_id=? AND title=?`,
        [ids, f.space, f.owner.id, f.marker],
      );
      assert.equal(reset.affectedRows, group.length);
      for (const table of [
        '`release`',
        'manifest_entry',
        'content_version',
        'structure_entry',
        'main_manifest',
        'collaboration_unit',
        'chapter',
        'style_revision',
        'structure_revision',
      ]) {
        const [deleted] = await db.query(`DELETE FROM ${table} WHERE document_id IN (?)`, [ids]);
        assert.equal(deleted.affectedRows, group.length, 'SCALE_GRAPH_DELETE_COUNT');
      }
      const [deleted] = await db.query(
        'DELETE FROM document WHERE id IN (?) AND space_id=? AND responsible_user_id=? AND title=?',
        [ids, f.space, f.owner.id, f.marker],
      );
      assert.equal(deleted.affectedRows, group.length);
    }
    // HTTP生成的输出租约/来源与access_gate保留作为真实证据，不删除其历史来伪装清理。
    await db.commit();
  } catch (error) {
    await db.rollback();
    throw error;
  } finally {
    await db.end();
  }
}

export async function searchPageScaleFixture(diagnostic = () => {}) {
  const [
    { createUser, get, json },
    { reviewDatabase },
    { publishSearchDocument },
    { searchConfiguration },
  ] = await Promise.all([
    import('./workspace.mjs'),
    import('./review.mjs'),
    import('./search-pages.mjs'),
    import('./search.mjs'),
  ]);
  const operation = 'search-scale-' + randomBytes(16).toString('hex');
  const f = {
    operation,
    marker: operation + '-management',
    q: operation.replaceAll('-', '') + 'sample',
    rows: [],
    esAttempted: [],
    diagnostic,
    transaction: false,
    commitAttempted: false,
  };
  f.title = '规模正式标题 ' + f.q;
  f.text = '规模固定正文 ' + f.q;
  f.templateQuery = operation.replaceAll('-', '') + 'template';
  f.templateText = '规模固定正文 ' + f.templateQuery;
  const db = await reviewDatabase();
  f.checkChain = async () => {
    const { searchDatabase } = await import('./search.mjs');
    const connection = await searchDatabase();
    try {
      const [rows] = await connection.query(
        `SELECT cursor_hash,CAST(UNIX_TIMESTAMP(expires_at)*1000 AS UNSIGNED) expiry
        FROM cursor_registry WHERE query_hash=? AND session_ref IS NULL AND physical_index=? AND page_size=100`,
        [sha(f.q), f.physical],
      );
      assert.equal(rows.length, 1, 'SCALE_SINGLE_ORIGINAL_CHAIN_REQUIRED');
      const row = rows[0];
      const expiry = Number(row.expiry);
      assert.ok(Number.isSafeInteger(expiry));
      if (!f.chain) {
        assert.ok(
          expiry > Date.now() && expiry <= Date.now() + 30000,
          'SCALE_FIXED_THIRTY_SECOND_TTL_REQUIRED',
        );
        f.chain = { hash: Buffer.from(row.cursor_hash), expiry };
      } else {
        assert.deepEqual(row.cursor_hash, f.chain.hash);
        assert.equal(expiry, f.chain.expiry, 'SCALE_ORIGINAL_ABSOLUTE_EXPIRY_CHANGED');
      }
    } finally {
      await connection.end();
    }
  };
  let closed;
  f.close = () =>
    (closed ??= (async () => {
      const errors = [];
      try {
        // COMMIT传输未知也先结束原连接上的潜在事务，避免新清理连接与自己互锁。
        // 已提交时ROLLBACK无副作用；其后仍靠新连接精确核验真实图，不把未知当未提交。
        if (f.transaction) await db.rollback();
        if (f.commitAttempted) {
          await retireChain(f);
          await removeES(f);
          await removeGraph(f);
        }
      } catch (error) {
        errors.push(error);
      }
      try {
        await db.end();
      } catch (error) {
        errors.push(error);
      }
      if (errors.length)
        throw new AggregateError(errors, 'SCALE_CLEANUP_INCOMPLETE_KEEP_OWN_FACTS');
    })());
  try {
    f.owner = await createUser();
    const directory = await get('/v1/me/spaces', f.owner.headers);
    assert.equal(directory.status, 200);
    const personal = json(directory).items.filter((space) => space.kind === 'PERSONAL');
    assert.equal(personal.length, 1, 'SCALE_ONE_OWN_PERSONAL_SPACE_REQUIRED');
    const space = personal[0];
    f.space = id(space.id);
    const [[ownSpace]] = await db.query(
      "SELECT CAST(account_owner_id AS CHAR) owner,lifecycle FROM space WHERE id=? AND kind='PERSONAL' AND organization_id IS NULL",
      [f.space],
    );
    assert.equal(ownSpace.owner, f.owner.id);
    assert.equal(ownSpace.lifecycle, 'ACTIVE');
    const templateDoc = await publishSearchDocument(
      { owner: f.owner, space: f.space },
      {
        q: f.templateQuery,
        title: '规模正式标题 ' + f.templateQuery,
        body: f.templateText,
      },
    );
    f.template = templateDoc.id;
    f.physical = templateDoc.physical;
    assert.match(f.physical, /^[a-z0-9][a-z0-9._-]{0,159}$/);
    const { search } = searchConfiguration();
    f.endpoint = search.elasticEndpoint;
    f.elasticOwner = search.owner;
    const [templates] = await db.query(
      `SELECT v.canonical_body,y.style_json,e.title chapter_title,
      v.schema_version,v.canonicalization_version,v.body_hash,CAST(v.byte_size AS CHAR) byte_size
      FROM document d JOIN ${RELEASE} r ON r.id=d.current_release_id AND r.document_id=d.id
      JOIN main_manifest m ON m.id=r.manifest_id AND m.document_id=d.id
      JOIN style_revision y ON y.id=m.style_id AND y.document_id=d.id
      JOIN structure_entry e ON e.structure_id=m.structure_id AND e.document_id=d.id
      JOIN manifest_entry me ON me.manifest_id=m.id AND me.document_id=d.id AND me.unit_id=e.unit_id
      JOIN content_version v ON v.id=me.content_version_id AND v.document_id=d.id AND v.unit_id=e.unit_id
      WHERE d.id=? AND d.space_id=? AND d.responsible_user_id=? AND d.knowledge_base_id IS NULL AND r.id=?`,
      [f.template, f.space, f.owner.id, templateDoc.release],
    );
    assert.equal(templates.length, 1, 'SCALE_TEMPLATE_MUST_HAVE_ONE_UNIT');
    const template = templates[0];
    assert.equal(template.schema_version, 'content-v1');
    assert.equal(template.canonicalization_version, 'canonical-v1');
    assert.equal(template.byte_size, String(template.canonical_body.length));
    assert.deepEqual(template.body_hash, sha(template.canonical_body));
    if (typeof template.style_json !== 'string')
      template.style_json = JSON.stringify(template.style_json);
    await buildGraph(f, db, template);
    await seedES(f);
    diagnostic({
      kind: 'SCALE_OWN_SQL_ES_SAMPLE_READY',
      operation: f.operation,
      owner: f.owner.id,
      space: f.space,
      retained_product_template: f.template,
      physical: f.physical,
      rejected: 1200,
      allowed: 1105,
      sample_consumer_receipts_claimed: 0,
    });
    return f;
  } catch (error) {
    try {
      await f.close();
    } catch (cleanup) {
      throw new AggregateError([error, cleanup], 'SCALE_SETUP_AND_CLEANUP_FAILED');
    }
    throw error;
  }
}
