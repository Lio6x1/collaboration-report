import https from 'node:https';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// 隔离目标必须使用自己的端口和证书；正文保持Buffer，PDF不能按UTF-8解码。
// 原3秒总预算覆盖握手及完整正文，不因持续少量字节重置。
export function restoredRequest(target, port, path, options = {}) {
  return new Promise((done, reject) => {
    const body = options.body === undefined ? undefined : JSON.stringify(options.body);
    const req = https.request(
      {
        hostname: 'localhost',
        port,
        path,
        ca: readFileSync(resolve(target, '.runtime/pki/ca.pem')),
        method: options.method ?? 'GET',
        headers: {
          ...options.headers,
          ...(body
            ? { 'content-type': 'application/json', 'content-length': Buffer.byteLength(body) }
            : {}),
        },
      },
      (response) => {
        const chunks = [];
        let size = 0;
        response.on('data', (chunk) => {
          size += chunk.length;
          if (size > 32 << 20) req.destroy(Error('RESTORE_RESPONSE_TOO_LARGE'));
          else chunks.push(chunk);
        });
        response.once('error', reject);
        response.once('aborted', () => reject(Error('RESTORE_RESPONSE_INCOMPLETE')));
        response.once('end', () => {
          clearTimeout(timer);
          done({
            status: response.statusCode,
            headers: response.headers,
            body: Buffer.concat(chunks),
          });
        });
      },
    );
    const timer = setTimeout(() => req.destroy(Error('RESTORE_REQUEST_TIMEOUT')), 3000);
    req.once('error', (error) => {
      clearTimeout(timer);
      reject(error);
    });
    req.end(body);
  });
}
