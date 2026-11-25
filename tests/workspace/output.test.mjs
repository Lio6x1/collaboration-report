import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import mysql from 'mysql2/promise';
import { runtime, root } from '../../scripts/runtime-lib.mjs';
import { request } from '../helpers/https.mjs';
import { createUser, get, post, json, operation, waitOperation, env } from '../helpers/account.mjs';

test('runtime document content, structure and provenance match fixed schemas and actual lineage rows', async () => {
  const user = await createUser(),
    spaces = await get('/v1/me/spaces', user.headers);
  assert.equal(spaces.status, 200);
  const space = json(spaces).items.find((x) => x.kind === 'PERSONAL');
  const created = await post(
    '/v1/documents',
    { operation_id: operation('document'), space_id: space.id, title: '完整基线' },
    user.headers,
  );
  assert.equal(created.status, 201);
  const id = json(created).result.entity_id,
    main = await get('/v1/documents/' + id + '/main', user.headers);
  assert.equal(main.status, 200);
  const v = json(main),
    unit = v.units[0].unit_id;
  const body = await get(
    '/v1/documents/' + id + '/read-views/' + v.read_view_id + '/units/' + unit,
    user.headers,
  );
  assert.equal(body.status, 200);
  const fixed = json(body);
  const checks = [
    { schema: 'SpacePage', value: json(spaces) },
    { schema: 'Operation', value: json(created) },
    { schema: 'MainView', value: v },
    { schema: 'FixedContent', value: fixed },
    { schema: 'content', value: fixed.body_json },
    { schema: 'provenance', value: fixed.provenance },
  ];
  const result = execFileSync('python', [resolve(root, 'scripts/validate-responses.py')], {
    input: JSON.stringify(checks),
    encoding: 'utf8',
    windowsHide: true,
  });
  assert.equal(JSON.parse(result).state, 'PASS');
  const cfg = JSON.parse(readFileSync(resolve(runtime, 'services/document.json'))),
    db = await mysql.createConnection(cfg.database);
  try {
    const [[lineage]] = await db.query(
      'SELECT unit_id,base_manifest_id,base_version_id FROM branch_lineage WHERE id=?',
      [fixed.provenance.lineage_id],
    );
    assert.equal(String(lineage.unit_id), unit);
    assert.equal(String(lineage.base_manifest_id), v.manifest_id);
    assert.equal(String(lineage.base_version_id), fixed.id);
  } finally {
    await db.end();
  }
});
test('logout closes real document admissions, read views and both persisted output segments', async () => {
  const user = await createUser(),
    spaces = await get('/v1/me/spaces', user.headers),
    space = json(spaces).items.find((x) => x.kind === 'PERSONAL');
  const c = await post(
    '/v1/documents',
    { operation_id: operation('doc'), space_id: space.id, title: '注销关闭' },
    user.headers,
  );
  assert.equal(c.status, 201);
  const id = json(c).result.entity_id,
    main = await get('/v1/documents/' + id + '/main', user.headers);
  assert.equal(main.status, 200);
  const v = json(main);
  await waitOperation(
    await post('/v1/account/logout', { operation_id: operation('logout') }, user.headers),
    user,
  );
  assert.equal((await get('/v1/documents/' + id + '/main', user.headers)).status, 401);
  const cfg = JSON.parse(readFileSync(resolve(runtime, 'services/document.json'))),
    db = await mysql.createConnection(cfg.database);
  try {
    const [[row]] = await db.query(
      'SELECT v.state AS view_state,a.state AS admission_state FROM read_view v JOIN document_admission a ON a.admission_id=v.admission_id WHERE v.view_id=?',
      [v.read_view_id],
    );
    assert.equal(row.view_state, 'CLOSED');
    assert.equal(row.admission_state, 'CLOSED');
    const [deliveries] = await db.query(
      'SELECT state,instance_id FROM delivery_session WHERE document_id=?',
      [id],
    );
    assert.ok(deliveries.some((x) => x.instance_id.endsWith('-edge')));
    assert.ok(deliveries.some((x) => x.instance_id.endsWith('-document')));
    assert.ok(deliveries.every((x) => x.state === 'SETTLED'));
  } finally {
    await db.end();
  }
});
test('untrusted browser proxy identities are stripped and an invented direct handler cannot release content', async () => {
  const user = await createUser();
  const through = await get('/v1/me/spaces', {
    ...user.headers,
    'x-edge-handler': 'invented-handler',
  });
  assert.equal(through.status, 200);
  const direct = await request(env.ports.documentHttp, '/v1/me/spaces', {
    headers: { ...user.headers, 'x-edge-handler': 'invented-handler' },
  });
  assert.equal(direct.status, 503);
  assert.ok(!direct.body.includes('"owner_user_id"'));
  assert.equal(
    (
      await post(
        '/v1/documents',
        { operation_id: operation('csrf'), space_id: json(through).items[0].id, title: 'CSRF越权' },
        { ...user.headers, 'x-csrf-token': 'invented' },
      )
    ).status,
    403,
  );
});
