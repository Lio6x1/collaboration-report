# services/collaboration/src/transport/http.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/348-http.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：接入HTTP/RPC/WSS；只在业务确认后返回相应成功事实。

主要声明：`createHTTP`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import https from 'node:https';
import { randomUUID } from 'node:crypto';

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

UTF-8/LF全文SHA-256：`d2e57ac37090ec1e38192d7562006403c0cca08ab3997e400bbf069d8a80b338`。[参考实现](../../../../services/collaboration/src/transport/http.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
