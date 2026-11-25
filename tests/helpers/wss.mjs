import { WebSocket } from 'ws';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime, state } from '../../scripts/runtime-lib.mjs';

export function wire(session, client, connection) {
  const s = state(),
    socket = new WebSocket(session.wss_url, {
      ca: readFileSync(resolve(runtime, 'pki/ca.pem')),
      origin: 'https://localhost:' + s.ports.edge,
      // READY包含完整可靠检查点及base64开销；接收预算与16MiB正文契约一致。
      // 服务端单次UPDATE的入站预算仍由真实WSS入口独立限制。
      maxPayload: 32 << 20,
    });
  const messages = [],
    waiters = [];
  let closed = false;
  socket.on('message', (data) => {
    const value = JSON.parse(data);
    messages.push(value);
    for (const wake of waiters.splice(0)) wake();
  });
  socket.on('close', () => {
    closed = true;
    for (const wake of waiters.splice(0)) wake();
  });
  socket.on('error', () => {});
  async function next(type, timeout = 4000) {
    const deadline = Date.now() + timeout;
    for (;;) {
      const i = messages.findIndex((m) => m.type === type || m.type === 'ERROR');
      if (i >= 0) {
        const value = messages.splice(i, 1)[0];
        if (value.type === 'ERROR' && type !== 'ERROR')
          throw Object.assign(Error(value.code), { code: value.code });
        return value;
      }
      if (closed) throw Error('SOCKET_CLOSED');
      if (Date.now() >= deadline) throw Error('WIRE_TIMEOUT_' + type);
      await new Promise((r) => {
        const timer = setTimeout(r, 50);
        waiters.push(() => {
          clearTimeout(timer);
          r();
        });
      });
    }
  }
  const ready = new Promise((done, reject) => {
    socket.once('open', () => {
      socket.send(
        JSON.stringify({
          type: 'OPEN',
          protocol: 'collab-wss-v1',
          ticket: session.ticket,
          client_session_id: client,
          connection_id: connection,
          workspace_epoch: session.workspace_epoch,
        }),
      );
      next('READY').then(done, reject);
    });
    socket.once('error', reject);
  });
  return {
    socket,
    messages,
    next,
    ready,
    send: (message) =>
      socket.send(
        JSON.stringify({
          room_id: session.room_id,
          workspace_epoch: session.workspace_epoch,
          client_session_id: client,
          ...message,
        }),
      ),
    close: () => socket.terminate(),
  };
}
