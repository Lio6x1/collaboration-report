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
