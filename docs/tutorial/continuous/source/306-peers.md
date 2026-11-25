# services/collaboration/src/client/peers.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/306-peers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`createPeers`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import https from 'node:https';
import { checkServerIdentity } from 'node:tls';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { RPCFailure } from '../domain/shared_protocol.mjs';

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

UTF-8/LF全文SHA-256：`caf22aad42f2ca4b17cc7629f79c756a628750e6abb29dd7a279e5634e7c23b4`。[参考实现](../../../../services/collaboration/src/client/peers.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
