# 分步编写 `services/collaboration/src/client/peers.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/306-peers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

创建Node到Doc的mTLS RPC与HTTPS客户端。

资源保护、票据兑换和结果恢复依赖Doc权威判断，连接本身必须验证对端服务身份。

写之前先按这条执行路线理解各段的连接：

1. 读取配置指定证书，加载私有protobuf并将longs保留为String
2. rpc调用固定2.5秒deadline并净化错误
3. http POST JSON，校验TLS和Doc SPIFFE身份，限制响应32MiB
4. 请求超时destroy，HTTP403/409映射权限/状态冲突

本文件需要守住的边界：

- 不能改大保存保护的期限来隐藏未知结果
- 响应不是200或JSON无效时不可当作保护成功

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/client/peers.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/client/peers.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/client/peers.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/client/peers.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:293 -->
````javascript
import https from 'node:https';
import { checkServerIdentity } from 'node:tls';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { RPCFailure } from '../domain/shared_protocol.mjs';

````

## 第 2 步：createPeers

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`export function createPeers(cfg) `。

**这一段为什么这样写：** 把可信私有传输注入业务用例，业务不自行拼TLS连接。

1. 加载证书和契约
2. 构造DocumentInternal
3. 返回rpc/http/close三个能力


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>JavaScript 参数</code> | 公开配置结构，含证书文件路径、根目录和Doc RPC/HTTP地址<br>在此声明中的实际使用：<code>cfg.keyFile</code>；<code>cfg.certFile</code>；<code>cfg.caFile</code>；<code>cfg.root</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     rpc: (method, input) =&gt;       new Promise((done, reject) =&gt;         client&#91;method&#93;(input, { deadline: Date.now() + 2500 }, (err, v) =&gt;           err ? reject(new RPCFailure(e …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return {     rpc: (method, input) =&gt;       new Promise((done, reject) =&gt;         client&#91;method&#93;(input, { deadline: Date.now() + 2500 }, (err, v) =&gt;           err ? reject(new RPCFa …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：此处通过 <code>return new Promise((done, reject) =&gt; {         const url = new URL(cfg.peers.documentHttp + path),           body = JSON.stringify(input);         const req = https.request(        …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=293:1932 -->
````javascript
export function createPeers(cfg) {
  const key = readFileSync(cfg.keyFile),
    cert = readFileSync(cfg.certFile),
    ca = readFileSync(cfg.caFile);
  const api = grpc.loadPackageDefinition(
    loader.loadSync(resolve(cfg.root, 'contracts/proto/internal.proto'), {
      longs: String,
      enums: String,
      defaults: true,
      keepCase: true,
    }),
  ).collaboration.v1;
  const client = new api.DocumentInternal(
    cfg.peers.document,
    grpc.credentials.createSsl(ca, key, cert),
  );
  return {
    rpc: (method, input) =>
      new Promise((done, reject) =>
        client[method](input, { deadline: Date.now() + 2500 }, (err, v) =>
          err ? reject(new RPCFailure(err.code, err.details)) : done(v),
        ),
      ),
    close: () => client.close(),
    http(path, input, { deadlineMs } = {}) {
      return new Promise((done, reject) => {
        const url = new URL(cfg.peers.documentHttp + path),
          body = JSON.stringify(input);
        const req = https.request(
          url,
          {
            servername: 'localhost',
            ca,
            key,
            cert,
            method: 'POST',
            headers: {
              'content-type': 'application/json',
              'content-length': Buffer.byteLength(body),
            },
            checkServerIdentity: (name, peer) =>
              checkServerIdentity(name, peer) ??
              (peer.subjectaltname
                ?.split(', ')
                .includes('URI:spiffe://collaboration-report/document')
                ? undefined
                : Error('PEER_IDENTITY_INVALID')),
          },
          (res) => {
````

## 第 3 步：createPeers（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 58 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>size &gt; 32 &lt;&lt; 20</code>。
- 在本段的内部函数内：检查 <code>res.statusCode !== 200</code>。条件成立时执行 <code>throw new RPCFailure(                     res.statusCode === 403 ? 7 : res.statusCode === 409 ? 9 : 14,                     value.code ?? &#x27;DEPENDENCY_UNAVAILABLE&#x27;,                   );</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(                     res.statusCode === 403 ? 7 : res.statusCode === 409 ? 9 : 14,                     value.code ?? &#x27;DEPENDENCY_UNAVAILABLE&#x27;,                   );</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 3 bytes=1932:3349 -->
````javascript
            let size = 0,
              chunks = [];
            res.on('data', (data) => {
              size += data.length;
              if (size > 32 << 20) res.destroy();
              else chunks.push(data);
            });
            res.on('error', () => reject(new RPCFailure(14, 'DEPENDENCY_UNAVAILABLE')));
            res.on('end', () => {
              try {
                const value = JSON.parse(Buffer.concat(chunks));
                if (res.statusCode !== 200)
                  throw new RPCFailure(
                    res.statusCode === 403 ? 7 : res.statusCode === 409 ? 9 : 14,
                    value.code ?? 'DEPENDENCY_UNAVAILABLE',
                  );
                done(value);
              } catch (err) {
                reject(err instanceof RPCFailure ? err : new RPCFailure(14, 'DEPENDENCY_INVALID'));
              }
            });
          },
        );
        // 正文保护沿用原 ProtectDraftResources 的总计2.5秒期限；
        // 增加额度侧账不能以延长原保存依赖期限掩盖吞吐问题。
        const deadline = deadlineMs ? setTimeout(() => req.destroy(), deadlineMs) : undefined;
        req.once('close', () => clearTimeout(deadline));
        req.setTimeout(deadlineMs ?? 3000, () => req.destroy());
        req.on('error', () => reject(new RPCFailure(14, 'DEPENDENCY_UNAVAILABLE')));
        req.end(body);
      });
    },
  };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/client/peers.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
