import https from 'node:https';
import { checkServerIdentity } from 'node:tls';
import {
  readFileSync,
  statSync,
  createReadStream,
  existsSync,
  writeFileSync,
  renameSync,
} from 'node:fs';
import { resolve, extname, sep } from 'node:path';
import { randomUUID, createHash } from 'node:crypto';
import { state, root, runtime } from './runtime-lib.mjs';
import { routeFor, filterProxyHeaders } from './edge-routing.mjs';
import { browserSessionContext, scopedResponseHeaders } from './browser-session-context.mjs';
import { EdgeOutputs } from './edge-outputs.mjs';
import { accountUnleasedResponse } from './account-output-policy.mjs';
import { fail, sendBufferedResponse, anonymousSearchEmptyResponse } from './edge-responses.mjs';
const env = state(),
  ca = readFileSync(resolve(runtime, 'pki/ca.pem'));
// 公网Origin与内部localhost mTLS地址分开；Windows默认地址保持既有行为。
const publicOrigin = env.publicOrigin ?? `https://localhost:${env.ports.edge}`;
const listenHost = env.edgeListenHost ?? '127.0.0.1';
if (
  !['127.0.0.1', '0.0.0.0'].includes(listenHost) ||
  new URL(publicOrigin).origin !== publicOrigin ||
  new URL(publicOrigin).protocol !== 'https:'
)
  throw Error('EDGE_CONFIGURATION_INVALID');
const instance = {
  instance_id: (process.env.COLLAB_RUN_OWNER ?? '') + '-edge',
  pid: process.pid,
  boot_digest: createHash('sha256')
    .update('edge\n' + process.env.COLLAB_RUN_OWNER)
    .digest('hex'),
};
const gateFile = resolve(runtime, 'edge-output-gates.json');
const outputs = new EdgeOutputs({
  load: () => (existsSync(gateFile) ? JSON.parse(readFileSync(gateFile)) : Object.create(null)),
  save: (value) => {
    writeFileSync(gateFile + '.tmp', JSON.stringify(value));
    renameSync(gateFile + '.tmp', gateFile);
  },
});
function peer(req, role) {
  return (
    req.socket.authorized &&
    req.socket
      .getPeerCertificate()
      .subjectaltname?.split(', ')
      .includes('URI:spiffe://collaboration-report/' + role)
  );
}
async function control(req, res, path) {
  if (!peer(req, 'document')) {
    fail(res, 403, 'CONTROL_IDENTITY_REQUIRED');
    return;
  }
  try {
    if (path === '/infra/output/info') {
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify(instance));
      return;
    }
    let size = 0,
      chunks = [];
    for await (const chunk of req) {
      size += chunk.length;
      if (size > 65536) throw Error('CONTROL_TOO_LARGE');
      chunks.push(chunk);
    }
    const input = JSON.parse(Buffer.concat(chunks));
    if (input.instance_id !== instance.instance_id) throw Error('OUTPUT_INSTANCE_CHANGED');
    let result;
    if (path === '/infra/output/open') result = outputs.open(input);
    else if (path === '/infra/output/fence') result = await outputs.fence(input);
    else throw Error('CONTROL_INVALID');
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(
      JSON.stringify({
        ...result,
        ...instance,
        ...(path === '/infra/output/fence'
          ? {
              scope_key: input.scope_key,
              epoch: input.epoch,
              revocation_id: input.revocation_id,
              target_digest: input.target_digest,
            }
          : {}),
      }),
    );
  } catch {
    fail(res, 409, 'OUTPUT_CONTROL_REJECTED');
  }
}
function settled(lease) {
  if (!lease) return;
  const body = JSON.stringify({ lease_id: lease, instance_id: instance.instance_id });
  const send = (n) => {
    const r = https.request(
      {
        hostname: '127.0.0.1',
        servername: 'localhost',
        port: env.ports.documentHttp,
        path: '/infra/output/settled',
        method: 'POST',
        ca,
        key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
        cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
        headers: { 'content-type': 'application/json', 'content-length': Buffer.byteLength(body) },
      },
      (res) => {
        res.resume();
        res.on('end', () => {
          if (res.statusCode !== 200 && n < 3) setTimeout(() => send(n + 1), 100);
        });
      },
    );
    r.setTimeout(1000, () => r.destroy());
    r.on('error', () => {
      if (n < 3) setTimeout(() => send(n + 1), 100);
    });
    r.end(body);
  };
  send(0);
}
const mime = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
};
const server = https.createServer(
  {
    key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
    cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
    ca,
    requestCert: true,
    rejectUnauthorized: false,
    minVersion: 'TLSv1.3',
  },
  (req, res) => {
    res.setHeader('X-Dev-Run-Owner', process.env.COLLAB_RUN_OWNER ?? '');
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Referrer-Policy', 'no-referrer');
    res.setHeader(
      'Content-Security-Policy',
      "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'",
    );
    let url;
    try {
      url = new URL(req.url, publicOrigin);
    } catch {
      fail(res, 400, 'REQUEST_INVALID');
      return;
    }
    if (url.pathname.startsWith('/infra/')) {
      void control(req, res, url.pathname);
      return;
    }
    const health = /^\/ops\/health\/(account|document|search|collaboration)$/.exec(url.pathname);
    const route = routeFor(req.method, url.pathname),
      authority = health?.[1] ?? route?.authority;
    if (authority) {
      let session;
      try {
        session = browserSessionContext(req.headers, url, req.method);
      } catch (error) {
        fail(res, error.message === 'BROWSER_SESSION_LIMIT' ? 429 : 400, error.message);
        return;
      }
      // 不向浏览器开放RPC；HTTP转发验证后端CA和hostname，身份自报头先清除。
      const protectedOutput = ['document', 'account'].includes(authority) && !health,
        // 固定单元/正式整篇正文使用32MiB完整响应上限，容纳既有16MiB单元和24MiB全篇。
        // 只扩大这些已登记输出的固定正文路由；普通工作台JSON仍为2MiB，放行继续核验原lease。
        responseBudget = [
          'get__v1_documents__documentId__read_views__viewId__units__unitId_',
          'get__v1_public_documents__documentId_',
          'get__v1_documents__documentId__releases__releaseId_',
          'get__v1_resources__resourceId__content',
        ].includes(route?.operationId)
          ? 32 * 1024 * 1024
          : 2 * 1024 * 1024,
        handlerId = randomUUID(),
        headers = filterProxyHeaders(req.headers);
      delete headers['x-collab-session-context'];
      delete headers.cookie;
      if (session.cookie) headers.cookie = session.cookie;
      let proxy;
      if (protectedOutput) {
        headers['x-edge-handler'] = handlerId;
        let close;
        const closed = new Promise((resolve) => {
          close = resolve;
        });
        outputs.attach(handlerId, {
          stop: () => {
            proxy?.destroy();
            res.destroy();
          },
          closed,
        });
        let finished = false;
        const finish = () => {
          if (finished) return;
          finished = true;
          const lease = outputs.handlers.get(handlerId)?.lease;
          close();
          outputs.detach(handlerId);
          if (lease) settled(JSON.parse(lease).lease_id);
        };
        res.once('finish', finish);
        res.once('close', finish);
      }
      proxy = https.request(
        {
          hostname: '127.0.0.1',
          servername: 'localhost',
          port: env.ports[authority + 'Http'],
          path: health ? '/health/ready' : url.pathname + url.search,
          method: req.method,
          headers,
          ca,
          checkServerIdentity: (name, cert) => {
            const err = checkServerIdentity(name, cert);
            if (err) return err;
            return cert.subjectaltname
              ?.split(', ')
              .includes('URI:spiffe://collaboration-report/' + authority)
              ? undefined
              : Error('HTTP_PEER_IDENTITY_MISMATCH');
          },
        },
        (up) => {
          const responseHeaders = scopedResponseHeaders(up.headers, session.context);
          if (!protectedOutput) {
            res.writeHead(up.statusCode, responseHeaders);
            up.pipe(res);
            return;
          }
          // 工作台短JSON响应先完整缓冲；注册失败、撤权或超预算时不向浏览器释放已构造正文。
          let size = 0,
            chunks = [];
          up.on('data', (chunk) => {
            size += chunk.length;
            if (size > responseBudget) {
              up.destroy();
              fail(res, 503, 'OUTPUT_BUDGET_EXCEEDED');
            } else chunks.push(chunk);
          });
          up.on('end', () => {
            const body = Buffer.concat(chunks);
            sendBufferedResponse(res, {
              status: up.statusCode,
              headers: responseHeaders,
              body,
              canRelease: () =>
                outputs.allowed(handlerId) ||
                anonymousSearchEmptyResponse({
                  authority,
                  method: req.method,
                  path: url.pathname,
                  status: up.statusCode,
                  cookie: session.cookie,
                  body,
                }) ||
                (authority === 'account' && accountUnleasedResponse(url.pathname, body)),
            });
          });
          up.on('error', () => fail(res, 503, 'DEPENDENCY_UNAVAILABLE'));
        },
      );
      const uploadBytes = route?.operationId === 'put__v1_resources__resourceId__upload';
      proxy.setTimeout(uploadBytes ? 35000 : 4000, () =>
        proxy.destroy(Error('DEPENDENCY_TIMEOUT')),
      );
      proxy.on('error', () => fail(res, 503, 'DEPENDENCY_UNAVAILABLE'));
      req.on('aborted', () => proxy.destroy());
      req.pipe(proxy);
      return;
    }
    if (
      url.pathname.startsWith('/v1/') ||
      url.pathname.startsWith('/ops/') ||
      url.pathname.startsWith('/collaboration.')
    ) {
      fail(res, 404, 'ROUTE_NOT_FOUND');
      return;
    }
    if (req.method !== 'GET' && req.method !== 'HEAD') {
      fail(res, 405, 'METHOD_NOT_ALLOWED');
      return;
    }
    try {
      const folder = resolve(root, 'web/dist');
      let file = resolve(folder, '.' + decodeURIComponent(url.pathname));
      // 规范化后的绝对路径必须留在dist中；非法路径不能被SPA回退掩盖。
      if (file !== folder && !file.startsWith(folder + sep)) {
        fail(res, 400, 'PATH_INVALID');
        return;
      }
      try {
        if (!statSync(file).isFile()) file = resolve(folder, 'index.html');
      } catch {
        file = resolve(folder, 'index.html');
      }
      res.writeHead(200, {
        'content-type': mime[extname(file)] ?? 'application/octet-stream',
        'cache-control': extname(file) === '.html' ? 'no-store' : 'public,max-age=3600',
      });
      if (req.method === 'HEAD') res.end();
      else
        createReadStream(file)
          .on('error', () => res.destroy())
          .pipe(res);
    } catch {
      fail(res, 503, 'FRONTEND_UNAVAILABLE');
    }
  },
);
// 协作入口只代理固定路径；票据在首个OPEN帧传输，避免出现在URL和访问日志中。
// Edge持有浏览器socket并参与真实关闭，不能把上游已断开误当作下游已停止。
server.on('upgrade', (req, socket, head) => {
  if (req.url !== '/collaboration' || req.headers.origin !== publicOrigin) {
    socket.end('HTTP/1.1 403 Forbidden\r\nConnection: close\r\n\r\n');
    return;
  }
  const handlerId = randomUUID(),
    headers = filterProxyHeaders(req.headers);
  // WSS以本标签HTTP取得的一次性OPEN票据识别身份，不把其他标签的Cookie带入协作服务。
  delete headers.cookie;
  delete headers['x-collab-session-context'];
  headers['x-edge-handler'] = handlerId;
  headers.connection = 'Upgrade';
  headers.upgrade = 'websocket';
  let upstream, proxy;
  let close;
  const closed = new Promise((resolve) => {
    close = resolve;
  });
  outputs.attach(handlerId, {
    stop: () => {
      proxy?.destroy();
      upstream?.destroy();
      socket.destroy();
    },
    closed,
  });
  socket.once('close', () => {
    proxy?.destroy();
    upstream?.destroy();
    const lease = outputs.handlers.get(handlerId)?.lease;
    close();
    outputs.detach(handlerId);
    if (lease) settled(JSON.parse(lease).lease_id);
  });
  socket.on('error', () => socket.destroy());
  proxy = https.request({
    hostname: '127.0.0.1',
    servername: 'localhost',
    port: env.ports.collaborationHttp,
    path: '/collaboration',
    method: 'GET',
    headers,
    ca,
    key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
    cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
    checkServerIdentity: (name, cert) =>
      checkServerIdentity(name, cert) ??
      (cert.subjectaltname?.split(', ').includes('URI:spiffe://collaboration-report/collaboration')
        ? undefined
        : Error('PEER_IDENTITY_INVALID')),
  });
  proxy.setTimeout(3000, () => proxy.destroy());
  proxy.on('error', () => socket.destroy());
  proxy.on('response', () => socket.destroy());
  proxy.on('upgrade', (res, up, upHead) => {
    upstream = up;
    up.on('error', () => socket.destroy());
    up.once('close', () => socket.destroy());
    socket.write(
      'HTTP/1.1 101 Switching Protocols\r\n' +
        Object.entries(res.headers)
          .map(([k, v]) => k + ': ' + v)
          .join('\r\n') +
        '\r\n\r\n',
    );
    if (head.length) up.write(head);
    if (upHead.length) {
      if (outputs.allowed(handlerId)) socket.write(upHead);
      else socket.destroy();
    }
    socket.pipe(up);
    up.on('data', (data) => {
      if (!outputs.allowed(handlerId)) {
        socket.destroy();
        up.destroy();
        return;
      }
      if (!socket.write(data)) up.pause();
    });
    socket.on('drain', () => up.resume());
  });
  proxy.end();
});
// 二进制上传的总期限由Doc的30秒控制；普通API仍有4秒上游超时。
server.requestTimeout = 35000;
server.headersTimeout = 5000;
server.listen(env.ports.edge, listenHost, () =>
  console.log(JSON.stringify({ event: 'edge_started', url: publicOrigin })),
);
server.on('error', () => {
  console.error(JSON.stringify({ code: 'EDGE_START_FAILED' }));
  process.exit(1);
});
process.on('SIGINT', () => server.close());
process.on('SIGTERM', () => server.close());
