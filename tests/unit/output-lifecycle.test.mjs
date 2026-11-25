import assert from 'node:assert/strict';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import https from 'node:https';
import { resolve } from 'node:path';
import test from 'node:test';
import { WebSocket, WebSocketServer } from 'ws';
import { runtime, state } from '../../scripts/runtime-lib.mjs';
import { OutputSessions } from '../../services/collaboration/src/transport/output_sessions.mjs';
import { attachWSS } from '../../services/collaboration/src/transport/wss.mjs';

function fixtureBinding() {
  return {
    delivery: 'isolated-transport-delivery',
    room: 'isolated-room',
    expires: Date.now() + 10000,
    sources: [{ scope_key: 'isolated-source', epoch: '1' }],
  };
}

function deliveryAdapter(observed) {
  // 隔离单元只替代 SQL 适配器：真实 TLS/close 是本用例的对象，不能冒称私有库 ACK。
  const instance = 'isolated-transport-instance';
  return {
    query: async (statement) => {
      const sql = typeof statement === 'string' ? statement : statement.sql;
      if (sql.startsWith('UPDATE')) {
        observed.settlements.push({ closed: observed.closed });
        return [{ affectedRows: 1 }];
      }
      return [
        observed.settlements.length
          ? []
          : [
              {
                delivery_id: 'isolated-transport-delivery',
                instance_id: instance,
              },
            ],
      ];
    },
  };
}

async function pair() {
  const server = https.createServer({
    key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
    cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
  });
  const wss = new WebSocketServer({ server });
  await new Promise((done) => server.listen(0, '127.0.0.1', done));
  const accepted = once(wss, 'connection');
  const client = new WebSocket('wss://localhost:' + server.address().port, {
    ca: readFileSync(resolve(runtime, 'pki/ca.pem')),
  });
  const [[socket]] = await Promise.all([accepted, once(client, 'open')]);
  return {
    client,
    socket,
    async dispose() {
      client.terminate();
      for (const peer of wss.clients) peer.terminate();
      await new Promise((done) => wss.close(done));
      server.closeAllConnections();
      await new Promise((done) => server.close(done));
    },
  };
}

for (const entryState of ['OPEN', 'CLOSING', 'CLOSED']) {
  test(`transport unit: actual ${entryState} WSS only settles after close evidence`, async () => {
    const connection = await pair();
    const observed = { closed: false, settlements: [] };
    connection.socket.once('close', () => {
      observed.closed = true;
    });
    const outputs = new OutputSessions(
      { pool: deliveryAdapter(observed) },
      'isolated-transport-instance',
      null,
      null,
    );
    try {
      if (entryState === 'CLOSING') {
        connection.client._socket.pause();
        connection.socket.close();
        assert.equal(connection.socket.readyState, WebSocket.CLOSING);
        assert.equal(observed.closed, false);
      } else if (entryState === 'CLOSED') {
        const closed = once(connection.socket, 'close');
        connection.socket.terminate();
        await closed;
        assert.equal(connection.socket.readyState, WebSocket.CLOSED);
      }
      outputs.attach(fixtureBinding(), connection.socket);
      await outputs.stopScope({ scope_key: 'isolated-source', epoch: '1' });
      assert.equal(observed.closed, true, '关闭范围完成必须有真实 close');
      assert.ok(observed.settlements.length > 0);
      assert.ok(
        observed.settlements.every((item) => item.closed),
        '不能提前持久结算',
      );
      assert.equal(outputs.outputs.size, 0);
    } finally {
      await connection.dispose();
    }
  });
}

test('transport unit: OPEN result resolving during real CLOSING does not bypass output lifetime', async () => {
  const tls = {
    key: readFileSync(resolve(runtime, 'pki/collaboration-key.pem')),
    cert: readFileSync(resolve(runtime, 'pki/collaboration.pem')),
    ca: readFileSync(resolve(runtime, 'pki/ca.pem')),
  };
  const server = https.createServer({ ...tls, requestCert: true, rejectUnauthorized: true });
  const observed = { closed: false, settlements: [] };
  const outputs = new OutputSessions(
    { pool: deliveryAdapter(observed) },
    'isolated-transport-instance',
    null,
    null,
  );
  const opened = Promise.withResolvers();
  const entered = Promise.withResolvers();
  const attached = Promise.withResolvers();
  const origin = 'https://localhost:' + state().ports.edge;
  const wss = attachWSS({ origin }, server, {
    open: () => {
      entered.resolve();
      return opened.promise;
    },
    attach: (binding, socket) => {
      outputs.attach(binding, socket);
      attached.resolve();
    },
    settle: (binding, reason) => outputs.settle(binding, reason),
    send: () => assert.fail('关闭通道不得补发 READY'),
  });
  await new Promise((done) => server.listen(0, '127.0.0.1', done));
  const client = new WebSocket('wss://localhost:' + server.address().port + '/collaboration', {
    ca: tls.ca,
    key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
    cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
    origin,
    headers: { 'x-edge-handler': 'isolated-real-transport-handler' },
  });
  try {
    await once(client, 'open');
    client.send(JSON.stringify({ type: 'OPEN' }));
    await entered.promise;
    const [socket] = wss.clients;
    assert.ok(socket);
    socket.once('close', () => {
      observed.closed = true;
    });
    client._socket.pause();
    socket.close();
    assert.equal(socket.readyState, WebSocket.CLOSING);
    opened.resolve({ binding: fixtureBinding(), ready: { type: 'READY' } });
    await attached.promise;
    await new Promise((done) => setImmediate(done));
    assert.equal(observed.closed, false);
    assert.equal(observed.settlements.length, 0, 'OPEN 入口不能重复提前 settle');
    await outputs.stopScope({ scope_key: 'isolated-source', epoch: '1' });
    assert.equal(observed.closed, true);
    assert.ok(observed.settlements.every((item) => item.closed));
  } finally {
    opened.resolve({ binding: fixtureBinding(), ready: { type: 'READY' } });
    client.terminate();
    for (const peer of wss.clients) peer.terminate();
    await new Promise((done) => wss.close(done));
    server.closeAllConnections();
    await new Promise((done) => server.close(done));
  }
});
