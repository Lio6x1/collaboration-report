# 分步编写 `services/collaboration/src/transport/http.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/348-http.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供Doc专用的提交证明、固定预览、清理计量和聚合指标私有HTTPS入口。

系统恢复读取也要证书身份约束，浏览器或Edge不能直接查询内部正文事实。

写之前先按这条执行路线理解各段的连接：

1. HTTPS要求TLS1.3；具体infra入口检查authorized与Doc SPIFFE URI
2. POST只允许固定路由且请求体<=8192字节，分派outcome/captures/transitions等用例
3. health/live只报存活，health/ready调用真实恢复检查
4. 错误只暴露稳定SERVICE_UNAVAILABLE和原号重试动作，特定传输超限映射413

本文件需要守住的边界：

- TLS服务器rejectUnauthorized=false不代表开放业务；路由里的authorized检查不能删
- 缓存头no-store，私有结果不能被中间缓存
- 原始SQL异常不得进入响应

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/transport' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/transport/http.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/transport/http.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/transport/http.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/transport/http.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:75 -->
````javascript
import https from 'node:https';
import { randomUUID } from 'node:crypto';

````

## 第 2 步：createHTTP

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

HTTP适配只处理证书身份、请求预算及响应；固定持久事实由应用用例读取。

本步声明：`export function createHTTP(`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>cfg.role</code> |
| <code>store</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>store.checkReady</code> |
| <code>tls</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>https.createServer(     { ...tls, requestCert: true, rejectUnauthorized: false, minVersion: &#x27;TLSv1.3&#x27; },     async (req, res) =&gt; {       res.setHeader(&#x27;X-Dev-Run-Owner&#x27;, process.en …</code>（长表达式，按本步完整代码继续） |
| <code>outcome</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>outcome.update</code> |
| <code>captures</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>captures.committedCut</code> |
| <code>transitions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>transitions.prepare</code> |
| <code>structurePreviews</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>structurePreviews.fixed</code> |
| <code>draftPreviews</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>draftPreviews.fixed</code> |
| <code>accountDocumentCleanup</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>accountDocumentCleanup.clean</code> |
| <code>personalTransferStorage</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>personalTransferStorage.measure</code> |
| <code>metrics</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>metrics.snapshot</code> |
| <code>mainCaptures</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>mainCaptures.revision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>http</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>req.method === &#x27;GET&#x27; &amp;&amp; req.url === &#x27;/infra/metrics&#x27;</code>。
- 在本段的内部函数内：检查 <code>!req.socket.authorized &#124;&#124;             !req.socket               .getPeerCertificate()               .subjectaltname?.split(&#x27;, &#x27;)               .includes(&#x27;URI:spiffe://collaboration-report/document&#x27;)</code>。
- 在本段的内部函数内：此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>req.method === &#x27;POST&#x27; &amp;&amp;           &#91;             &#x27;/infra/persistence/update&#x27;,             &#x27;/infra/main/revision&#x27;,             &#x27;/infra/persistence/submission-cut&#x27;,             &#x27;/inf …</code>（长表达式，按本步完整代码继续）。

<!-- write-step: 2 bytes=75:1968 -->
````javascript
// HTTP适配只处理证书身份、请求预算及响应；固定持久事实由应用用例读取。
export function createHTTP(
  cfg,
  store,
  tls,
  outcome,
  captures,
  transitions,
  structurePreviews,
  draftPreviews,
  accountDocumentCleanup,
  personalTransferStorage,
  metrics,
  mainCaptures,
) {
  const http = https.createServer(
    { ...tls, requestCert: true, rejectUnauthorized: false, minVersion: 'TLSv1.3' },
    async (req, res) => {
      res.setHeader('X-Dev-Run-Owner', process.env.COLLAB_RUN_OWNER ?? '');
      res.setHeader('Content-Type', 'application/json; charset=utf-8');
      res.setHeader('Cache-Control', 'no-store');
      res.setHeader('X-Content-Type-Options', 'nosniff');
      try {
        if (req.method === 'GET' && req.url === '/infra/metrics') {
          // 仅Doc服务证书可取得运维聚合指标；Edge及匿名浏览器不能经
          // 私有端口读取。无需凭用户口令，也不放宽原业务输出来源。
          if (
            !req.socket.authorized ||
            !req.socket
              .getPeerCertificate()
              .subjectaltname?.split(', ')
              .includes('URI:spiffe://collaboration-report/document')
          ) {
            res.statusCode = 403;
            res.end(JSON.stringify({ code: 'CONTROL_IDENTITY_REQUIRED' }));
            return;
          }
          res.end(JSON.stringify(metrics.snapshot()));
          return;
        }
        if (
          req.method === 'POST' &&
          [
            '/infra/persistence/update',
            '/infra/main/revision',
            '/infra/persistence/submission-cut',
            '/infra/draft/prepare',
            '/infra/structure/fixed',
            '/infra/draft/fixed',
            '/infra/lifecycle/document-cleanup',
            '/infra/lifecycle/personal-transfer-storage',
          ].includes(req.url)
        ) {
````

## 第 3 步：createHTTP（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 57 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>!req.socket.authorized &#124;&#124;             !req.socket               .getPeerCertificate()               .subjectaltname?.split(&#x27;, &#x27;)               .includes(&#x27;URI:spiffe://collaboration-report/document&#x27;)</code>。
- 在本段的内部函数内：此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>size &gt; 8192</code>。条件成立时执行 <code>throw Error(&#x27;REQUEST_TOO_LARGE&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw Error(&#x27;REQUEST_TOO_LARGE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>mainCaptures.revision(input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>personalTransferStorage.measure(input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>accountDocumentCleanup.clean(input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>draftPreviews.fixed(input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>structurePreviews.fixed(input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>transitions.prepare(input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>captures.committedCut(input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>outcome.update(input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>req.method === &#x27;GET&#x27; &amp;&amp; req.url === &#x27;/health/live&#x27;</code>。
- 在本段的内部函数内：此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>req.method === &#x27;GET&#x27; &amp;&amp; req.url === &#x27;/health/ready&#x27;</code>。
- 在本段的内部函数内：等待 <code>store.checkReady()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=1968:3975 -->
````javascript
          if (
            !req.socket.authorized ||
            !req.socket
              .getPeerCertificate()
              .subjectaltname?.split(', ')
              .includes('URI:spiffe://collaboration-report/document')
          ) {
            res.statusCode = 403;
            res.end(JSON.stringify({ code: 'CONTROL_IDENTITY_REQUIRED' }));
            return;
          }
          let size = 0,
            chunks = [];
          for await (const data of req) {
            size += data.length;
            if (size > 8192) throw Error('REQUEST_TOO_LARGE');
            chunks.push(data);
          }
          const input = JSON.parse(Buffer.concat(chunks));
          const result =
            req.url === '/infra/main/revision'
              ? await mainCaptures.revision(input)
              : req.url === '/infra/lifecycle/personal-transfer-storage'
                ? await personalTransferStorage.measure(input)
                : req.url === '/infra/lifecycle/document-cleanup'
                  ? await accountDocumentCleanup.clean(input)
                  : req.url === '/infra/draft/fixed'
                    ? await draftPreviews.fixed(input)
                    : req.url === '/infra/structure/fixed'
                      ? await structurePreviews.fixed(input)
                      : req.url === '/infra/draft/prepare'
                        ? await transitions.prepare(input)
                        : req.url === '/infra/persistence/submission-cut'
                          ? await captures.committedCut(input)
                          : await outcome.update(input);
          res.end(JSON.stringify(result));
          return;
        }
        if (req.method === 'GET' && req.url === '/health/live') {
          res.end(JSON.stringify({ state: 'LIVE', role: cfg.role }));
          return;
        }
        if (req.method === 'GET' && req.url === '/health/ready') {
          await store.checkReady();
          res.end(JSON.stringify({ state: 'READY', role: cfg.role }));
````

## 第 4 步：createHTTP（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 102 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>req.url === &#x27;/infra/draft/fixed&#x27; &amp;&amp; error.details === &#x27;DISCARD_TRANSFER_LIMIT&#x27;</code>。
- 在本段的内部函数内：此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>req.url === &#x27;/infra/draft/prepare&#x27; &amp;&amp; error.details === &#x27;DRAFT_TRANSFER_LIMIT&#x27;</code>。
- 在本段的内部函数内：此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return http;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=3975:5143 -->
````javascript
          return;
        }
        res.statusCode = 501;
        res.end(
          JSON.stringify({
            code: 'FEATURE_NOT_IMPLEMENTED',
            message: '该功能将在后续批次实现',
            trace_id: randomUUID(),
            retry_action: 'NONE',
          }),
        );
      } catch (error) {
        if (req.url === '/infra/draft/fixed' && error.details === 'DISCARD_TRANSFER_LIMIT') {
          res.statusCode = 413;
          res.end(JSON.stringify({ code: 'DISCARD_TRANSFER_LIMIT' }));
          return;
        }
        if (req.url === '/infra/draft/prepare' && error.details === 'DRAFT_TRANSFER_LIMIT') {
          res.statusCode = 413;
          res.end(JSON.stringify({ code: 'DRAFT_TRANSFER_LIMIT' }));
          return;
        }
        // 原始MySQL错误不能跨出服务边界，也不能直接写入日志。
        res.statusCode = 503;
        res.end(
          JSON.stringify({
            code: 'SERVICE_UNAVAILABLE',
            message: '服务暂时不可用',
            trace_id: randomUUID(),
            retry_action: 'RETRY_SAME_OPERATION',
          }),
        );
      }
    },
  );

  return http;
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/transport/http.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
