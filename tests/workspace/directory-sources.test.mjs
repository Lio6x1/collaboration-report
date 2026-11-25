import test from 'node:test';
import assert from 'node:assert/strict';
import mysql from 'mysql2/promise';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
  waitAuthorization,
} from '../helpers/workspace.mjs';
test('knowledge-base visibility derived from one document records that document scope on both outputs', async () => {
  const f = await organizationFixture(1),
    u = f.members[0],
    d = await documentFixture(f),
    grant = {
      user_id: u.id,
      scope_kind: 'UNIT',
      scope_id: d.main.units[0].unit_id,
      capability: 'EDIT',
    };
  await waitAuthorization(
    await post(
      '/v1/documents/' + d.id + '/authorization-changes',
      {
        operation_id: operation('grant'),
        expected_acl_revision: '1',
        changes: [{ action: 'GRANT', grant }],
      },
      f.owner.headers,
    ),
    f.owner,
  );
  const token = 'scope-check-' + operation('request'),
    r = await get('/v1/knowledge-bases/' + f.kb, { ...u.headers, 'x-trace-id': token });
  assert.equal(r.status, 200);
  const cfg = JSON.parse(readFileSync(resolve(runtime, 'services/document.json'))),
    db = await mysql.createConnection(cfg.database);
  try {
    const [leases] = await db.query(
      "SELECT payload FROM durable_task WHERE kind='OUTPUT_LEASE' AND JSON_CONTAINS(payload,JSON_OBJECT('scope_key',?),'$.sources') ORDER BY id DESC LIMIT 2",
      [
        'session:' +
          (await import('node:crypto'))
            .createHash('sha256')
            .update(
              Buffer.from(/__Host-collabSession=([^;]+)/.exec(u.headers.cookie)[1], 'base64url'),
            )
            .digest('hex'),
      ],
    );
    assert.equal(leases.length, 2);
    for (const l of leases) {
      const payload = typeof l.payload === 'string' ? JSON.parse(l.payload) : l.payload;
      assert.ok(payload.sources.some((s) => s.scope_key === 'doc-acl:' + d.id));
    }
  } finally {
    await db.end();
  }
});
