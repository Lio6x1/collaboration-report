import { WebSocket, WebSocketServer } from 'ws';
import { RPCFailure } from '../domain/shared_protocol.mjs';

// 每连接32帧输入预算、三秒OPEN及原顺序链都在传输层；SAVED/FLUSH由实际用例给出。
export function attachWSS(cfg, http, engine) {
  const wss = new WebSocketServer({
    noServer: true,
    maxPayload: 1500000,
    perMessageDeflate: false,
  });
  http.on('upgrade', (req, socket, head) => {
    if (
      req.url !== '/collaboration' ||
      req.headers.origin !== cfg.origin ||
      !req.socket.authorized ||
      !req.socket
        .getPeerCertificate()
        .subjectaltname?.split(', ')
        .includes('URI:spiffe://collaboration-report/edge') ||
      !req.headers['x-edge-handler']
    ) {
      socket.destroy();
      return;
    }
    wss.handleUpgrade(req, socket, head, (ws) => {
      let b,
        chain = Promise.resolve(),
        pending = 0;
      const timer = setTimeout(() => ws.terminate(), 3000);
      timer.unref();
      ws.once('close', () => clearTimeout(timer));
      ws.on('error', () => ws.terminate());
      ws.on('message', (raw, binary) => {
        // 输入队列有界；串行处理同连接，不让OPEN/UPDATE/FLUSH互相越过。
        if (binary || ++pending > 32) {
          ws.terminate();
          return;
        }
        chain = chain
          .then(async () => {
            let input;
            try {
              input = JSON.parse(raw);
            } catch {
              throw new RPCFailure(3, 'MESSAGE_INVALID');
            }
            if (!b) {
              if (input.type !== 'OPEN') throw new RPCFailure(3, 'OPEN_REQUIRED');
              const opened = await engine.open(input, req.headers['x-edge-handler']);
              b = opened.binding;
              clearTimeout(timer);
              engine.attach(b, ws);
              // OPEN 的依赖返回时连接可能已开始关闭；登记生命周期后等待真实 close，
              // 不能把 CLOSING 当 CLOSED 提前结算，也不能向旧通道补发 READY。
              if (ws.readyState !== WebSocket.OPEN) return;
              await engine.send(b, opened.ready);
              return;
            }
            let result;
            if (input.type === 'UPDATE') result = await engine.update(b, input);
            else if (input.type === 'FLUSH') result = await engine.flush(b, input);
            else if (input.type === 'SYNC') result = await engine.sync(b, input);
            else if (input.type === 'STRUCTURE_READY') {
              engine.structureReady(b, input);
              return;
            } else if (input.type === 'AWARENESS') {
              await engine.awareness(b, input);
              return;
            } else throw new RPCFailure(3, 'MESSAGE_TYPE_INVALID');
            await engine.send(b, result);
          })
          .catch(async (err) => {
            const code =
              err instanceof RPCFailure
                ? err.details
                : err.code === 'INVALID_CONTENT' || err.code === 'DEPENDENCIES_MISSING'
                  ? err.code
                  : 'SERVICE_UNAVAILABLE';
            if (b && !b.stopping) {
              try {
                await engine.send(b, { type: 'ERROR', code });
              } catch {
                ws.terminate();
              }
            } else ws.terminate();
            if (err instanceof RPCFailure && err.code === 7) ws.terminate();
          })
          .finally(() => {
            pending--;
          });
      });
    });
  });

  return wss;
}
