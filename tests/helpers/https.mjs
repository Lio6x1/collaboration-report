import https from 'node:https';
import { readFileSync } from 'node:fs';
import { runtime } from '../../scripts/runtime-lib.mjs';
import { resolve } from 'node:path';
import { routeFor } from '../../scripts/edge-routing.mjs';
const ca = readFileSync(resolve(runtime, 'pki/ca.pem'));
export function request(port, path, options = {}) {
  const { timeoutMs = 3000, ...httpOptions } = options;
  options = httpOptions;
  return new Promise((done, reject) => {
    const started = Date.now();
    let receivedHeaders = false;
    let finished = false;
    let deadline;
    const finish = (error, result) => {
      if (finished) return;
      finished = true;
      clearTimeout(deadline);
      if (error) {
        // 只记录冻结操作名和传输阶段，路径可能含邀请凭据，不能连同Cookie/正文输出。
        error.cause ??= {
          operationId:
            routeFor(options.method ?? 'GET', path.split('?')[0])?.operationId ?? 'UNMAPPED',
          method: options.method ?? 'GET',
          port,
          receivedHeaders,
          elapsedMs: Date.now() - started,
          timeoutMs,
          tlsAuthorized: req.socket?.authorized === true,
          reusedSocket: req.reusedSocket,
        };
        reject(error);
      } else done(result);
    };
    // Node低层DELETE请求不默认采用chunked；明确正文长度，不能测试成空JSON。
    if (options.body !== undefined)
      options = {
        ...options,
        headers: { 'content-length': Buffer.byteLength(options.body), ...options.headers },
      };
    const req = https.request(
      { hostname: 'localhost', port, path, ca, method: 'GET', ...options },
      (res) => {
        receivedHeaders = true;
        let body = '';
        res.setEncoding('utf8');
        res.on('data', (x) => (body += x));
        // 响应头之后的TLS截断属于IncomingMessage，ClientRequest不一定再次报错。
        // 不完整正文既不能成功，也不能留下等不到end的Promise。
        const incomplete = () => finish(Error('TEST_INCOMPLETE_BODY'));
        res.once('aborted', incomplete);
        res.once('error', (error) => finish(error));
        res.once('close', () => {
          if (!res.complete) incomplete();
        });
        res.once('end', () => {
          if (!res.complete) incomplete();
          else finish(null, { status: res.statusCode, headers: res.headers, body });
        });
      },
    );
    // 总预算覆盖握手、响应头和全部正文；持续少量字节不能重置原定结束时间。
    deadline = setTimeout(() => req.destroy(Error('TEST_TIMEOUT')), timeoutMs);
    req.once('error', (error) => finish(error));
    req.end(options.body);
  });
}
