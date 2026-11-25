import test from 'node:test';
import assert from 'node:assert/strict';
import https from 'node:https';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';
import { request } from '../../tests/helpers/https.mjs';

test('a real HTTPS truncated response terminates the test client within its original budget', async () => {
  let sawRequest = false;
  const server = https.createServer(
    {
      key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
      cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
    },
    (_req, res) => {
      sawRequest = true;
      res.writeHead(200, { 'content-type': 'application/json', 'content-length': '1000' });
      res.flushHeaders();
      res.write('{"incomplete":');
      // 确实先交付响应头和部分正文，再终止TLS输出；不把握手前失败冒充截断。
      setTimeout(() => res.socket?.destroy(), 25);
    },
  );
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  let guard;
  try {
    const outcome = await Promise.race([
      request(server.address().port, '/truncated', { timeoutMs: 250 }).then(
        () => 'INCORRECT_SUCCESS',
        (error) => error.message,
      ),
      new Promise((resolve) => {
        guard = setTimeout(() => resolve('CLIENT_STILL_WAITING'), 750);
      }),
    ]);
    assert.equal(sawRequest, true);
    assert.notEqual(outcome, 'INCORRECT_SUCCESS', '不完整正文不能算成功');
    assert.notEqual(outcome, 'CLIENT_STILL_WAITING', 'socket已经截断，不能继续永久等待end');
  } finally {
    clearTimeout(guard);
    https.globalAgent.destroy();
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
});

test('steady response bytes cannot extend the original whole-request budget', async () => {
  const timers = new Set();
  const server = https.createServer(
    {
      key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
      cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
    },
    (_req, res) => {
      res.writeHead(200, { 'content-type': 'application/json' });
      res.flushHeaders();
      const body = '{"slow":true}';
      let position = 0;
      // 字节持续到达，旧inactivity timeout不断重新计时；整个请求仍必须有原定结束预算。
      const timer = setInterval(() => {
        if (position < body.length) res.write(body[position++]);
        else {
          clearInterval(timer);
          timers.delete(timer);
          res.end();
        }
      }, 30);
      timers.add(timer);
      res.once('close', () => {
        clearInterval(timer);
        timers.delete(timer);
      });
    },
  );
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    await assert.rejects(
      () => request(server.address().port, '/slow', { timeoutMs: 100 }),
      /TEST_TIMEOUT/,
    );
  } finally {
    for (const timer of timers) clearInterval(timer);
    https.globalAgent.destroy();
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
});

test('a complete HTTPS response preserves its actual status headers and full body', async () => {
  const body = '{"complete":true,"text":"完整中文响应"}';
  const server = https.createServer(
    {
      key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
      cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
    },
    (_req, res) => {
      res.writeHead(201, { 'content-type': 'application/json', 'x-test-complete': 'yes' });
      res.end(body);
    },
  );
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const result = await request(server.address().port, '/complete');
    assert.equal(result.status, 201);
    assert.equal(result.headers['x-test-complete'], 'yes');
    assert.equal(result.body, body);
  } finally {
    https.globalAgent.destroy();
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
});
