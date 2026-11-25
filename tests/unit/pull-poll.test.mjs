import test from 'node:test';
import assert from 'node:assert/strict';
import https from 'node:https';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';
import { env } from '../../tests/helpers/account.mjs';
import { pollPull } from '../../tests/helpers/pull-main.mjs';

const id = 'poll-original-operation-123456';
const unavailable = (code, retry_action = 'RETRY_SAME_OPERATION') => ({
  code,
  message: '原操作进度暂时不可核验',
  trace_id: 'poll-test-trace',
  retry_action,
});

// 隔离端口仅重放测试所观察到的HTTP边界；不写实际业务库，也不替代正文/安装集成断言。
async function replay(responses, run) {
  const calls = [];
  const server = https.createServer(
    {
      key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
      cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
    },
    (req, res) => {
      calls.push({ method: req.method, path: req.url });
      const [status, value] = responses[Math.min(calls.length - 1, responses.length - 1)];
      res.writeHead(status, { 'content-type': 'application/json' });
      res.end(JSON.stringify(value));
    },
  );
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const originalPort = env.ports.edge;
  env.ports.edge = server.address().port;
  try {
    await run(calls);
  } finally {
    env.ports.edge = originalPort;
    https.globalAgent.destroy();
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
}

test('explicit capacity recovery polls the original operation after SERVICE_UNAVAILABLE', async () => {
  await replay(
    [
      [503, unavailable('SERVICE_UNAVAILABLE')],
      [
        200,
        {
          operation_id: id,
          state: 'SUCCEEDED',
          result: { content_version_id: '123', main_sync_state: 'READY' },
        },
      ],
    ],
    async (calls) => {
      const result = await pollPull(id, {}, { retryUnavailable: true, timeout: 1500 });
      assert.equal(result.state, 'SUCCEEDED');
      assert.equal(result.operation_id, id);
      assert.deepEqual(calls, Array(2).fill({ method: 'GET', path: '/v1/operations/' + id }));
    },
  );
});

test('default polling and a response without retry authorization still fail immediately', async () => {
  for (const options of [{}, { retryUnavailable: true }]) {
    const retry = options.retryUnavailable ? 'NONE' : 'RETRY_SAME_OPERATION';
    await replay([[503, unavailable('SERVICE_UNAVAILABLE', retry)]], async (calls) => {
      await assert.rejects(() => pollPull(id, {}, options), { code: 'ERR_ASSERTION' });
      assert.equal(calls.length, 1);
    });
  }
});

test('capacity recovery never retries denied access or a real conflict', async () => {
  for (const status of [403, 409]) {
    await replay([[status, unavailable('SOURCE_CLOSED')]], async (calls) => {
      await assert.rejects(() => pollPull(id, {}, { retryUnavailable: true }), {
        code: 'ERR_ASSERTION',
      });
      assert.equal(calls.length, 1);
    });
  }
});
