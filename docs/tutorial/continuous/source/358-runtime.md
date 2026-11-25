# services/collaboration/src/bootstrap/runtime.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/358-runtime.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：注册路由、RPC及恢复器，连接入口和完整业务。

主要声明：`run`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`1ed7fcc4119e4edafa301b390107aae2bcea40ff7ae04047f638333dd4a4addf`。[参考实现](../../../../services/collaboration/src/bootstrap/runtime.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
