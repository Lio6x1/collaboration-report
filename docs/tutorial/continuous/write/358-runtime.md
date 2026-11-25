# 分步编写 `services/collaboration/src/bootstrap/runtime.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/358-runtime.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按恢复先于监听的顺序启动协作服务，并集中停止资源。

旧输出、房间日志或撤权未恢复就开放端口，会暴露不一致正文和失效授权。

写之前先按这条执行路线理解各段的连接：

1. 核--config和role，创建store/TLS/peers/engine
2. 先engine.initialize，再recoverFences
3. 创建HTTPS/RPC/WSS并强制RPC客户端证书，绑定端口后启动每秒资源确认
4. SIGINT/SIGTERM停恢复定时器、终止socket、关HTTP/RPC/peers/pool再退出

本文件需要守住的边界：

- service_started只表示已完成该启动路径，就绪请求仍须查数据库状态
- 端口错误只给SERVICE_PORT_UNAVAILABLE，不打印私有配置
- 不能把监听步骤移到恢复之前

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/bootstrap' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/bootstrap/runtime.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/bootstrap/runtime.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/bootstrap/runtime.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/bootstrap/runtime.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:729 -->
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

````

## 第 2 步：run

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`export async function run() `。

**这一段为什么这样写：** 规定整个进程安全启动和关闭顺序。

1. 装配配置与依赖
2. 完成输出/日志/确认/撤权恢复
3. 开放私有监听
4. 登记有序停止处理


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === &#x27;1&#x27;</code>。
- 检查 <code>process.argv&#91;2&#93; !== &#x27;--config&#x27;</code>。条件成立时执行 <code>throw Error(&#x27;SERVICE_CONFIG_REQUIRED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;SERVICE_CONFIG_REQUIRED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>cfg.role !== &#x27;collaboration&#x27;</code>。条件成立时执行 <code>throw Error(&#x27;SERVICE_CONFIG_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;SERVICE_CONFIG_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>engine.initialize()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>recoverFences(store.pool, engine)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>new Promise((done, reject) =&gt;     rpc.bindAsync(       `${cfg.host}:${cfg.rpcPort}`,       grpc.ServerCredentials.createSsl(         tls.ca,         &#91;{ private_key: tls.key, cert_c …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=729:2658 -->
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
````

## 第 3 步：run（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 70 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 等待 <code>new Promise((done, reject) =&gt; {     http.once(&#x27;error&#x27;, () =&gt; reject(Error(&#x27;SERVICE_PORT_UNAVAILABLE&#x27;)));     http.listen(cfg.httpPort, cfg.host, done);   })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=2658:3307 -->
````javascript
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

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/bootstrap/runtime.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
