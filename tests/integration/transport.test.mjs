import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { state, runtime } from '../../scripts/runtime-lib.mjs';
import { resolve } from 'node:path';
import { request } from '../helpers/https.mjs';
import net from 'node:net';
const env = state();
for (const role of ['account', 'document', 'search', 'collaboration']) {
  test(`${role} HTTPS reports actual database readiness`, async () => {
    const response = await request(env.ports[role + 'Http'], '/health/ready');
    assert.equal(response.status, 200);
    assert.equal(JSON.parse(response.body).state, 'READY');
  });
}
test('unfinished API response follows the shared Error retry enum', async () => {
  const spec = JSON.parse(
    readFileSync(new URL('../../contracts/http/openapi.json', import.meta.url)),
  );
  const response = await request(env.ports.edge, '/v1/account/closures', { method: 'POST' });
  assert.equal(response.status, 501);
  const body = JSON.parse(response.body);
  assert(spec.components.schemas.Error.properties.retry_action.enum.includes(body.retry_action));
});
test('healthy listeners belong to the current registered start, including the edge', async () => {
  const list = JSON.parse(readFileSync(resolve(runtime, 'processes.json')));
  assert.equal(list.length, 5 + Number(Boolean(env.ports.smtp)));
  assert.deepEqual(
    list.map((p) => p.role).sort(),
    [
      'account',
      'collaboration',
      'document',
      'edge',
      'search',
      ...(env.ports.smtp ? ['smtp'] : []),
    ].sort(),
  );
  for (const item of list) {
    if (item.role === 'smtp') {
      const banner = await new Promise((done, reject) => {
        const socket = net.connect(env.ports.smtp, '127.0.0.1');
        socket.setTimeout(1000, () => socket.destroy(Error('SMTP_TEST_TIMEOUT')));
        socket.once('data', (value) => {
          socket.end();
          done(value.toString());
        });
        socket.once('error', reject);
      });
      assert.ok(banner.includes('collaboration-dev ' + item.owner));
      continue;
    }
    const r = await request(
      item.role === 'edge' ? env.ports.edge : env.ports[item.role + 'Http'],
      item.role === 'edge' ? '/' : '/health/ready',
    );
    assert.equal(r.headers['x-dev-run-owner'], item.owner);
  }
});
