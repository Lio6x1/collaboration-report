import test from 'node:test';
import assert from 'node:assert/strict';
import https from 'node:https';
import net from 'node:net';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';
import { checkHTTPS } from '../../scripts/https-readiness.mjs';

const owner = 'probe-original-owner';
const ca = readFileSync(resolve(runtime, 'pki/ca.pem'));
const credentials = {
  key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
  cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
};

async function withServer(handler, run) {
  const server = https.createServer(credentials, handler);
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    await run(server.address().port);
  } finally {
    https.globalAgent.destroy();
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
}

test('supervisor refuses truncated output after actual ready headers within its original budget', async () => {
  let reached = false;
  await withServer(
    (_req, res) => {
      reached = true;
      res.writeHead(200, { 'content-length': '1000', 'x-dev-run-owner': owner });
      res.flushHeaders();
      res.write('{"partial":');
      setTimeout(() => res.socket?.destroy(), 25);
    },
    async (port) => {
      // 请求确实拿到原owner与200；剩余不完整正文不能被误判成READY，也不能永远等end。
      await assert.rejects(
        () => checkHTTPS(port, '/health/ready', owner, { ca }),
        /PROBE_INCOMPLETE_BODY/,
      );
      assert.equal(reached, true);
    },
  );
});

test('supervisor keeps the original total budget even while ready response bytes arrive', async () => {
  await withServer(
    (_req, res) => {
      res.writeHead(200, { 'x-dev-run-owner': owner });
      res.flushHeaders();
      const timer = setInterval(() => res.write(' '), 15);
      res.once('close', () => clearInterval(timer));
    },
    async (port) => {
      await assert.rejects(
        () => checkHTTPS(port, '/health/ready', owner, { ca, timeoutMs: 100 }),
        /PROBE_TIMEOUT/,
      );
    },
  );
});

test('supervisor settles when the actual TCP peer never completes TLS', async () => {
  const sockets = new Set();
  const server = net.createServer((socket) => {
    sockets.add(socket);
    socket.once('close', () => sockets.delete(socket));
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    await assert.rejects(
      () => checkHTTPS(server.address().port, '/health/ready', owner, { ca, timeoutMs: 100 }),
      /PROBE_TIMEOUT/,
    );
  } finally {
    for (const socket of sockets) socket.destroy();
    await new Promise((resolve) => server.close(resolve));
  }
});

test('supervisor accepts only a complete 200 response from the exact original owner', async () => {
  for (const [status, actualOwner, accepted] of [
    [200, owner, true],
    [503, owner, false],
    [200, 'another-owner', false],
  ]) {
    await withServer(
      (_req, res) => {
        res.writeHead(status, { 'x-dev-run-owner': actualOwner });
        res.end('complete');
      },
      async (port) => {
        const result = checkHTTPS(port, '/health/ready', owner, { ca });
        if (accepted) await result;
        else await assert.rejects(() => result, /NOT_READY/);
      },
    );
  }
});
