import { readFileSync } from 'node:fs';
import grpc from '@grpc/grpc-js';
import { createStore } from './store.mjs';
import { createRoomComponents } from './room_components.mjs';
import { RoomEngine } from '../application/room_engine.mjs';
import { PersistenceOutcome } from '../application/persistence_outcome.mjs';
import { createPeers } from '../client/peers.mjs';
import { createHTTP } from '../transport/http.mjs';
import { createRPC } from '../transport/rpc.mjs';
import { attachWSS } from '../transport/wss.mjs';
import { recoverFences } from '../worker/fence_recovery.mjs';
import { startConfirmationRecovery } from '../worker/resource_recovery.mjs';
import { RuntimeMetrics } from '../application/runtime_metrics.mjs';

export async function run() {
  const started = performance.now();
  const startupStage = (stage) => {
    // 仅输出固定恢复阶段与耗时，定位启动慢点；不打印房间正文、任务输入或私密配置。
    if (process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === '1')
      console.error(
        JSON.stringify({
          event: 'collaboration_startup',
          stage,
          elapsed_ms: Math.round(performance.now() - started),
        }),
      );
  };
  if (process.argv[2] !== '--config') throw Error('SERVICE_CONFIG_REQUIRED');
  const cfg = JSON.parse(readFileSync(process.argv[3]));
  if (cfg.role !== 'collaboration') throw Error('SERVICE_CONFIG_INVALID');
  const store = createStore(cfg);
  const tls = {
    key: readFileSync(cfg.keyFile),
    cert: readFileSync(cfg.certFile),
    ca: readFileSync(cfg.caFile),
  };
  const peers = createPeers(cfg);
  const engine = new RoomEngine(createRoomComponents(store, cfg, peers, { startupStage }));
  await engine.initialize();
  await recoverFences(store.pool, engine);
  startupStage('FENCES_RECOVERED');
  const http = createHTTP(
    cfg,
    store,
    tls,
    new PersistenceOutcome(store.pool),
    engine.captures,
    engine.draftTransitions,
    engine.structurePreviews,
    engine.draftPreviews,
    engine.accountDocumentCleanup,
    engine.personalTransferStorage,
    new RuntimeMetrics(engine.queue),
    engine.mainCaptures,
  );
  const rpc = createRPC(cfg, store, engine);
  const wss = attachWSS(cfg, http, engine);
  // RPC握手强制客户端证书；具体URI/方法/有限deadline仍由传输适配核验。
  await new Promise((done, reject) =>
    rpc.bindAsync(
      `${cfg.host}:${cfg.rpcPort}`,
      grpc.ServerCredentials.createSsl(
        tls.ca,
        [{ private_key: tls.key, cert_chain: tls.cert }],
        true,
      ),
      (err) => (err ? reject(Error('SERVICE_PORT_UNAVAILABLE')) : done()),
    ),
  );
  startupStage('RPC_BOUND');
  await new Promise((done, reject) => {
    http.once('error', () => reject(Error('SERVICE_PORT_UNAVAILABLE')));
    http.listen(cfg.httpPort, cfg.host, done);
  });
  startupStage('HTTPS_LISTENING');
  console.log(JSON.stringify({ event: 'service_started', role: cfg.role }));
  const stopRecovery = startConfirmationRecovery(engine);
  const stop = () => {
    stopRecovery();
    for (const socket of wss.clients) socket.terminate();
    http.close();
    rpc.forceShutdown();
    peers.close();
    store.pool.end().finally(() => process.exit(0));
  };
  process.on('SIGINT', stop);
  process.on('SIGTERM', stop);
}
