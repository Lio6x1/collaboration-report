import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import mysql from 'mysql2/promise';
import { createUser, get, env } from '../helpers/account.mjs';
import { runtime } from '../../scripts/runtime-lib.mjs';
test('expired sessions are durably fenced even when the browser never sends logout', async () => {
  const user = await createUser(),
    token = /__Host-collabSession=([^;]+)/.exec(user.headers.cookie)[1],
    hash = createHash('sha256').update(Buffer.from(token, 'base64url')).digest();
  const cfg = JSON.parse(readFileSync(resolve(runtime, 'services/account.json'))),
    db = await mysql.createConnection(cfg.database);
  try {
    await db.query(
      'UPDATE session SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE id=?',
      [hash],
    );
    assert.equal((await get('/v1/account/me', user.headers)).status, 401);
    let state;
    for (let n = 0; n < 40; n++) {
      const [[row]] = await db.query('SELECT state FROM session WHERE id=?', [hash]);
      state = row.state;
      if (state === 'CLOSED') break;
      await new Promise((r) => setTimeout(r, 100));
    }
    assert.equal(state, 'CLOSED');
    const [[ack]] = await db.query(
      "SELECT COUNT(*) AS n FROM authorization_ack WHERE scope_key=? AND participant='document' AND phase='OUTPUT_STOPPED'",
      ['session:' + hash.toString('hex')],
    );
    assert.equal(String(ack.n), '1');
  } finally {
    await db.end();
  }
});
