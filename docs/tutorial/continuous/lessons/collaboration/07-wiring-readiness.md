# C07｜协作完整依赖装配、传输与就绪

## 这节课要实现什么

前六课已有保存、撤权和结构迁移逻辑，现在要让浏览器真正连上共同编辑服务。本课把依赖、网络入口、恢复任务和进程生命周期接起来。

## 做完后能看到什么

本课结束可运行完整协作离线规则；06中服务只有迁移和必要恢复完成才报告就绪，浏览器才能获得真实房间连接。停止进程时要收束连接和任务，不留下假成功。

## 先想清楚怎样实现

1. 先只创建一份房间队列及共享依赖。
2. 注册内部RPC、HTTP和浏览器WSS入口。
3. 在监听与就绪前处理必要恢复，退出时按依赖顺序停止输出并释放资源。

## 怎样判断这一课完成

本课跑齐协作离线规则；数据库就绪检查和真实WSS按06执行，实际输入后再重连确认内容仍在。

## 开始写之前

从上一节已经写入的文件继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/collaboration/src/repository/readiness.mjs` | 检查migration原校验和、持久任务和屏障事实，避免只连上MySQL就报告READY。 | [逐段带写](../../write/317-readiness.md) |
| 2 | `services/collaboration/src/application/readiness.mjs` | 组合schema、恢复标记及房间完整性检查，读真实恢复结果。 | [逐段带写](../../write/337-readiness.md) |
| 3 | `services/collaboration/src/application/runtime_metrics.mjs` | 导出房间队列聚合指标供Doc运维接口读取，不泄露正文/身份。 | [逐段带写](../../write/342-runtime-metrics.md) |
| 4 | `services/collaboration/src/application/room_engine.mjs` | 提供唯一用例调用面；UPDATE清除旧lastFlush，FLUSH记录本连接回执，结构握手调用实际输出器。 | [逐段带写](../../write/339-room-engine.md) |
| 5 | `services/collaboration/src/bootstrap/room_components.mjs` | 创建唯一事务器、房间队列、准入、输出及所有用例；防止各用例另建队列越序。 | [逐段带写](../../write/357-room-components.md) |
| 6 | `services/collaboration/src/rooms.mjs` | 兼容RoomEngine构造入口，将装配对象交给应用引擎并重导出既有工具。 | [逐段带写](../../write/365-rooms.md) |
| 7 | `services/collaboration/src/transport/rpc_authorization.mjs` | 按mTLS URI角色、方法白名单及有限deadline授权，证书CN不能代替URI。 | [逐段带写](../../write/351-rpc-authorization.md) |
| 8 | `services/collaboration/src/transport/rpc.mjs` | 从internal.proto加载生成契约，注册撤权、capture、merge、MAIN与草稿RPC；保留UNIMPLEMENTED边界。 | [逐段带写](../../write/350-rpc.md) |
| 9 | `services/collaboration/src/transport/http.mjs` | 提供Doc专用持久结果、预览、准备、清理与计量私有HTTPS路由及健康检查。 | [逐段带写](../../write/348-http.md) |
| 10 | `services/collaboration/src/transport/wss.mjs` | 只接受Edge mTLS和固定Origin的/collaboration，三秒OPEN、32帧有界链，并分派实际用例。 | [逐段带写](../../write/352-wss.md) |
| 11 | `services/collaboration/src/bootstrap/store.mjs` | 创建mysql2连接池和readiness；队列上限32与连接期限有界。 | [逐段带写](../../write/359-store.md) |
| 12 | `services/collaboration/src/bootstrap/runtime.mjs` | 读本服务配置，恢复房间/输出和撤权后才监听RPC/HTTPS/WSS，启动资源恢复定时器。 | [逐段带写](../../write/358-runtime.md) |
| 13 | `services/collaboration/src/main.mjs` | 进程入口只调用run并输出稳定启动错误，不重复装配业务。 | [逐段带写](../../write/364-main.md) |

## 2. 先装配唯一的依赖对象

先完成readiness仓储和应用、metrics，再写RoomEngine，最后createRoomComponents。`RoomEngine` 只是已分工用例的调用面，不重新解释正文、不直接写SQL；update先 `assertStructureWrite` 并清该delivery.lastFlush，flush拿真实FLUSHED后 `recordFlush`，保证结构ACK不能沿旧回执继续写。

`createRoomComponents(store,cfg,peers,{startupStage})` 按以下顺序创建：Transactions→SerializedRooms→SnapshotRepository/RoomSnapshots→RoomAdmissions→OutputSessions→ResourceConfirmations→RoomUpdates/RoomReading/RoomAwareness→capture/merge/MAIN/草稿/结构/清理用例→OutputRecovery。所有房间动作必须共用前三个实例，输出授权复用同一admissions。merge计算器只负责worker线程；OutputRecovery拿ProcessOwner和同一snapshots/confirmations。返回对象交给应用RoomEngine；rooms.mjs兼容构造也调用这个组合函数。

<!-- lesson-code: services/collaboration/src/bootstrap/room_components.mjs :: createRoomComponents -->
````javascript
// 显式组合唯一队列/事务/授权/输出，避免每个用例各建一份状态导致UPDATE与FLUSH互相越过。
export function createRoomComponents(store, cfg, peers, { startupStage } = {}) {
  const instance = (process.env.COLLAB_RUN_OWNER ?? '') + '-collaboration';
  const transactions = new Transactions(store.pool);
  const queue = new SerializedRooms();
  const snapshots = new RoomSnapshots(new SnapshotRepository(store.pool), store);
  const admissions = new RoomAdmissions(store, peers, instance, transactions, queue, snapshots);
  const outputs = new OutputSessions(store, instance, transactions, admissions);
  const confirmations = new ResourceConfirmations(store, peers);
  const accountDocumentCleanup = new AccountDocumentCleanup(transactions, peers);
  const personalTransferStorage = new PersonalTransferStorage(transactions);
  const candidateCleanup = new CandidateCleanup(transactions, peers);
  const updates = new RoomUpdates(
    peers,
    transactions,
    queue,
    snapshots,
    admissions,
    outputs,
    confirmations,
  );
  const reading = new RoomReading(transactions, queue, snapshots, admissions);
  const awareness = new RoomAwareness(transactions, admissions, outputs);
  const captures = new SubmissionCaptures(transactions, queue, snapshots, peers);
  const mergeCandidates = new MergeCandidates(transactions, new MergeExecutor());
  const structurePreparations = new StructurePreparations();
  const mainCommits = new MainCommits(transactions, peers, structurePreparations, outputs);
  const mainCaptures = new MainCaptures(transactions, snapshots, peers);
  const mainInstallations = new MainInstallations(
    transactions,
    snapshots,
    peers,
    outputs,
    structurePreparations,
    new StructureInstallations(snapshots),
  );
  const draftPreviews = new DraftPreviews(transactions, queue, snapshots, peers);
  const structurePreviews = new StructurePreviews(transactions, snapshots, peers, mainCaptures);
  const draftTransitions = new DraftTransitions(transactions, queue, snapshots, peers, outputs);
  const recovery = new OutputRecovery(
    store,
    cfg,
    instance,
    transactions,
    snapshots,
    confirmations,
    new ProcessOwner(cfg.root),
    startupStage,
  );
  return {
    store,
    cfg,
    peers,
    instance,
    transactions,
    queue,
    snapshots,
    admissions,
    updates,
    reading,
    awareness,
    captures,
    mergeCandidates,
    mainCommits,
    mainCaptures,
    mainInstallations,
    draftPreviews,
    structurePreviews,
    draftTransitions,
    outputs,
    confirmations,
    recovery,
    accountDocumentCleanup,
    personalTransferStorage,
    candidateCleanup,
  };
}
````
<!-- /lesson-code -->

`createStore(config)` 建mysql2 pool（等待连接、queueLimit=32、connectTimeout=2000）并创建readiness；实际数据库配置由06生成，讲义不硬编码DSN。Readiness除了迁移SQL原checksum，还核持久任务、屏障和 `roomsRecovered`，不能以 `SELECT 1` 代替房间恢复。RuntimeMetrics输出uptime、CPU/内存、事件循环耗时及pending/active/waiting/peakPending/rooms队列聚合值，私有运维调用不包含正文/房间号/身份。

## 3. 注册实际RPC与私有HTTP

先写 `authorizeCall(auth,path,deadline)`：必须有受信URI身份，方法在该角色白名单内，deadline存在且不超资源预算。证书common name即使写document也不代表Doc。随后createRPC从现成 `contracts/proto/internal.proto` 和health.proto加载契约，longs保持String、bytes保持Buffer，不手抄另一份DTO。接收64MiB预算容纳三份固定输入，发送32MiB容纳候选；不能沿默认4MiB截断送审正文。

| 实际RPC方法 | 调用目标 | 本步连接的用户动作 |
| --- | --- | --- |
| FenceAccess/GetFenceResult | fenceRooms | 退出或撤权等待真实旧输出排空 |
| CaptureSubmission/GetCapture/FinalizeCapture | engine.captures | FLUSH后送审固定正文、查询与终结原capture |
| ComputeMergeCandidate/CleanupCandidate | mergeCandidates/candidateCleanup | 审核合并固定三方输入及终结后的候选清理 |
| CaptureMainView/PrepareMainCommit/ReconcileMainCommit | mainCaptures/mainCommits/mainInstallations | 可靠MAIN读取、准备与Go提交后安装 |
| CaptureDraftPreview | structurePreviews或draftPreviews，按purpose分派 | 整文档结构预览或草稿PULL/弃稿/修复预览 |
| ApplyDraftResolution | draftTransitions.apply | 原固定草稿变换安装 |

<!-- lesson-code: services/collaboration/src/transport/rpc.mjs :: createRPC -->
````javascript
// 生成契约和服务身份在传输边界处理；未实现方法保持UNIMPLEMENTED。
export function createRPC(cfg, store, engine) {
  const rpc = new grpc.Server({
    // 三份固定输入每份最多16MiB；响应候选仍最多16MiB，不能沿用单份接收预算。
    'grpc.max_receive_message_length': 64 << 20,
    'grpc.max_send_message_length': 32 << 20,
  });
  const options = { longs: String, enums: String, defaults: true, keepCase: true };
  const business = grpc.loadPackageDefinition(
    loader.loadSync(resolve(cfg.root, 'contracts/proto/internal.proto'), options),
  ).collaboration.v1.CollaborationInternal;
  const health = grpc.loadPackageDefinition(
    loader.loadSync(resolve(cfg.root, 'contracts/proto/health.proto'), options),
  ).grpc.health.v1.Health;
  const protect = (handler) => async (call, callback) => {
    const error = authorizeCall(call.getAuthContext(), call.getPath(), call.getDeadline());
    if (error) {
      callback(error);
      return;
    }
    try {
      await handler(call, callback);
    } catch (err) {
      callback(
        err instanceof RPCFailure
          ? { code: err.code, details: err.details }
          : { code: grpc.status.UNAVAILABLE, details: 'SERVICE_UNAVAILABLE' },
      );
    }
  };
  const handlers = Object.fromEntries(
    Object.keys(business.service).map((method) => [
      method,
      protect(async (_call, callback) =>
        callback({ code: grpc.status.UNIMPLEMENTED, details: 'FEATURE_NOT_IMPLEMENTED' }),
      ),
    ]),
  );
  handlers.FenceAccess = protect(async (call, callback) =>
    callback(null, await fenceRooms(engine, call.request)),
  );
  handlers.GetFenceResult = handlers.FenceAccess;
  handlers.CaptureSubmission = protect(async (call, callback) =>
    callback(null, await engine.captures.capture(call.request)),
  );
  handlers.GetCapture = protect(async (call, callback) =>
    callback(null, await engine.captures.get(call.request)),
  );
  handlers.FinalizeCapture = protect(async (call, callback) =>
    callback(null, await engine.captures.finalize(call.request)),
  );
  handlers.ComputeMergeCandidate = protect(async (call, callback) =>
    callback(null, await engine.mergeCandidates.compute(call.request)),
  );
  handlers.CleanupCandidate = protect(async (call, callback) =>
    callback(null, await engine.candidateCleanup.clean(call.request)),
  );
  handlers.CaptureMainView = protect(async (call, callback) =>
    callback(null, await engine.mainCaptures.capture(call.request)),
  );
  handlers.ReconcileMainCommit = protect(async (call, callback) =>
    callback(null, await engine.mainInstallations.reconcile(call.request)),
  );
  handlers.PrepareMainCommit = protect(async (call, callback) =>
    callback(null, await engine.mainCommits.prepare(call.request)),
  );
  handlers.CaptureDraftPreview = protect(async (call, callback) =>
    callback(
      null,
      await (
        call.request.context?.target?.purpose === 'STRUCTURE_PREVIEW'
          ? engine.structurePreviews
          : engine.draftPreviews
      ).capture(call.request),
    ),
  );
  handlers.ApplyDraftResolution = protect(async (call, callback) =>
    callback(null, await engine.draftTransitions.apply(call.request)),
  );
  rpc.addService(business.service, handlers);
  rpc.addService(health.service, {
    Check: protect(async (_call, callback) => {
      try {
        await store.checkReady();
        callback(null, { status: 'SERVING' });
      } catch {
        callback(null, { status: 'NOT_SERVING' });
      }
    }),
  });

  return rpc;
}
````
<!-- /lesson-code -->

HTTP由TLSv1.3建私有HTTPS。`/health/live` 说明进程活着，`/health/ready` 调真实store检查。Doc证书才能POST `/infra/persistence/update`、`/infra/main/revision`、`/infra/persistence/submission-cut`、`/infra/draft/prepare`、`/infra/structure/fixed`、`/infra/draft/fixed`、两条lifecycle路由；请求8192字节上限，正文固定结果是响应。GET `/infra/metrics` 同样只给Doc。异常不能输出原MySQL消息；返回稳定503和RETRY_SAME_OPERATION，草稿传输超限413按真实业务码处理。

## 4. WSS接线必须保持输入与输出生命周期

写attachWSS时先设置noServer、maxPayload=1500000、禁压缩；upgrade只认 `/collaboration`、配置Origin、Edge的mTLS URI和x-edge-handler。三秒内必须OPEN，二进制或超过32个待处理帧立即终止。每连接chain串行消费，OPEN没完成UPDATE不能越过它；OPEN返回后先attach生命周期，连接若已CLOSING就不发READY，仍等实际close结算。

UPDATE/FLUSH/SYNC调用engine对应方法；STRUCTURE_READY和AWARENESS分别调用结构确认和当前权限在线状态。返回的SAVED/FLUSHED由应用与数据库产生，传输层不编造确认。RPCFailure 7发送ERROR后终止；未知原始错误只输出SERVICE_UNAVAILABLE。输出 `send` 还执行admissions.authorized，不能凭第一次OPEN长期广播。

<!-- lesson-code: services/collaboration/src/transport/wss.mjs :: attachWSS -->
````javascript
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
````
<!-- /lesson-code -->

具体用户路径：浏览器复用现成前端向Doc申请edit-session，Edge代理WSS并带实际handler，Node兑换ticket并回READY；浏览器输入发UPDATE，应用SQL提交后回SAVED；点击送审先FLUSH得到receipt，再走Doc HTTP→内部RPC CaptureSubmission。任何一步只有本地Yjs显示文字不算可靠保存，只有202/PROCESSING也不等于业务终态。

## 5. main与恢复worker的启动/停止次序

最后写runtime.run与main入口。run要求 `--config <本服务配置>` 且role=collaboration；创建store、读取本服务TLS、建peers/engine，先await initialize（实例/房间/资源恢复），再await recoverFences。全部成立后构造HTTP/RPC/WSS，bind mTLS RPC，listen HTTPS，再输出service_started并启动每秒资源确认恢复。恢复失败时不能先开放端口再假READY。

<!-- lesson-code: services/collaboration/src/bootstrap/runtime.mjs :: run -->
````javascript
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
<!-- /lesson-code -->

SIGINT/SIGTERM先停止恢复timer，terminate实际WSS，关闭HTTP/RPC/peers，等待pool.end后退出。不能遗留后台恢复任务继续用已关闭pool；正常停止仍不删持久数据。main只调run，错误用稳定启动码退出1；服务端口由06自己的owner配置分配，浏览器不能直连私有RPC或数据库。

## 6. 全协作包写齐后的离线检查

```powershell
# 工作目录：自己的lessonRoot，C01-C07全部完成；00已安装锁定Node依赖。
Set-Location -LiteralPath $lessonRoot
$lessonRuleFiles = @(Get-ChildItem -LiteralPath 'services/collaboration/tests' -Filter '*.test.mjs' |
    Where-Object { $_.Name -ne 'readiness.test.mjs' } | ForEach-Object { $_.FullName })
node --no-maglev --test --test-reporter=tap --test-concurrency=1 @lessonRuleFiles
if ($LASTEXITCODE -ne 0) { throw '协作离线规则或用例失败' }
python $coursePrepare --target $lessonRoot --check 04
if ($LASTEXITCODE -ne 0) { throw 'phase04手写文件未齐或字节不符' }
```

预期所有被选测试执行、fail/skip=0、进程退出0。精确安全测试 `Node RPC rejects a caller without TLS identity`、`Node RPC rejects a missing deadline`、`Node RPC denies a trusted but unauthorized role`、`Node RPC accepts an authorized bounded service call`；`readiness accepts every configured migration with its original checksum` 使用schema替身，证明规则而非真实库。`draft prepare measures the encoded candidate and MAIN baseline before blocking the room` 核预算先于冻结。

唯一暂不选 `readiness.test.mjs`：它读取06的真实服务配置/MySQL，不是漏测。06完整READY以后执行：

```powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 services/collaboration/tests/readiness.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实Node恢复就绪失败' }
```

精确测试 `Node ready accepts the current baseline only after actual durable room verification` 与 `Node ready restores a terminal persisted closure without reopening its gate` 核真恢复和门闩。其后沿06运行WSS/审核/结构/生命周期代表流程及浏览器；本章离线成功不代表这些真实流程已执行。

找不到模块按本课返回对象与前课文件表核每条相对import；NOT_SERVING查migration原checksum、roomsRecovered、原UNKNOWN任务/屏障；RPC UNIMPLEMENTED核handlers名称与proto；WSS秒断核Origin/EdgeURI/handler/OPEN。只调整真实漏接或输入错误，不能删任务、放宽validator、延长保存期限来制造READY。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/collaboration/src/repository/readiness.mjs`：[本课完成文件](../../source/317-readiness.md)。
- `services/collaboration/src/application/readiness.mjs`：[本课完成文件](../../source/337-readiness.md)。
- `services/collaboration/src/application/runtime_metrics.mjs`：[本课完成文件](../../source/342-runtime-metrics.md)。
- `services/collaboration/src/application/room_engine.mjs`：[本课完成文件](../../source/339-room-engine.md)。
- `services/collaboration/src/bootstrap/room_components.mjs`：[本课完成文件](../../source/357-room-components.md)。
- `services/collaboration/src/rooms.mjs`：[本课完成文件](../../source/365-rooms.md)。
- `services/collaboration/src/transport/rpc_authorization.mjs`：[本课完成文件](../../source/351-rpc-authorization.md)。
- `services/collaboration/src/transport/rpc.mjs`：[本课完成文件](../../source/350-rpc.md)。
- `services/collaboration/src/transport/http.mjs`：[本课完成文件](../../source/348-http.md)。
- `services/collaboration/src/transport/wss.mjs`：[本课完成文件](../../source/352-wss.md)。
- `services/collaboration/src/bootstrap/store.mjs`：[本课完成文件](../../source/359-store.md)。
- `services/collaboration/src/bootstrap/runtime.mjs`：[本课完成文件](../../source/358-runtime.md)。
- `services/collaboration/src/main.mjs`：[本课完成文件](../../source/364-main.md)。

继续：[05 搜索服务](../../05-search.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/collaboration/src/repository/readiness.mjs',
    'services/collaboration/src/application/readiness.mjs',
    'services/collaboration/src/application/runtime_metrics.mjs',
    'services/collaboration/src/application/room_engine.mjs',
    'services/collaboration/src/bootstrap/room_components.mjs',
    'services/collaboration/src/rooms.mjs',
    'services/collaboration/src/transport/rpc_authorization.mjs',
    'services/collaboration/src/transport/rpc.mjs',
    'services/collaboration/src/transport/http.mjs',
    'services/collaboration/src/transport/wss.mjs',
    'services/collaboration/src/bootstrap/store.mjs',
    'services/collaboration/src/bootstrap/runtime.mjs',
    'services/collaboration/src/main.mjs'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
