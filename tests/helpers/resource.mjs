import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import https from 'node:https';
import { get, post, json, operation, env, origin } from './account.mjs';
import { request } from './https.mjs';
import { runtime } from '../../scripts/runtime-lib.mjs';

export const resourcePNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jC1kAAAAASUVORK5CYII=',
  'base64',
);

export async function uploadPNG(f, bytes = resourcePNG) {
  const checksum = createHash('sha256').update(bytes).digest('hex');
  const quota = await get(`/v1/spaces/${f.f.space}/quota`, f.user.headers);
  assert.equal(quota.status, 200, quota.body);
  const prepared = await post(
    `/v1/documents/${f.d.id}/resources`,
    {
      operation_id: operation('conflict-image-upload'),
      expected_quota_revision: json(quota).revision,
      byte_size: String(bytes.length),
      media_type: 'image/png',
      checksum,
    },
    f.user.headers,
  );
  assert.equal(prepared.status, 201, prepared.body);
  const handle = json(prepared);
  const uploaded = await request(env.ports.edge, new URL(handle.upload_url).pathname, {
    method: 'PUT',
    headers: { origin, 'content-type': 'application/octet-stream', ...f.user.headers },
    body: bytes,
    timeoutMs: 10000,
  });
  assert.equal(uploaded.status, 200, uploaded.body);
  const completed = await post(
    `/v1/resources/${handle.resource_id}/complete`,
    {
      operation_id: operation('conflict-image-complete'),
      byte_size: String(bytes.length),
      checksum,
    },
    f.user.headers,
  );
  assert.equal(completed.status, 200, completed.body);
  assert.equal(json(completed).state, 'READY');
  return handle.resource_id;
}

// 下载断言保留完整原始Buffer；UTF-8字符串会损坏PNG并掩盖字节不一致。
export function binaryResourceGet(path, headers = {}) {
  return new Promise((resolveResponse, reject) => {
    let deadline;
    let settled = false;
    const finish = (error, response) => {
      if (settled) return;
      settled = true;
      clearTimeout(deadline);
      if (error) reject(error);
      else resolveResponse(response);
    };
    const req = https.request(
      {
        hostname: 'localhost',
        port: env.ports.edge,
        path,
        method: 'GET',
        ca: readFileSync(resolve(runtime, 'pki/ca.pem')),
        headers,
      },
      (response) => {
        const chunks = [];
        response.on('data', (chunk) => chunks.push(chunk));
        response.once('error', (error) => finish(error));
        response.once('aborted', () => finish(Error('RESOURCE_BODY_INCOMPLETE')));
        response.once('close', () => {
          if (!response.complete) finish(Error('RESOURCE_BODY_INCOMPLETE'));
        });
        response.once('end', () => {
          if (!response.complete) return finish(Error('RESOURCE_BODY_INCOMPLETE'));
          finish(null, {
            status: response.statusCode,
            headers: response.headers,
            body: Buffer.concat(chunks),
          });
        });
      },
    );
    deadline = setTimeout(() => req.destroy(Error('RESOURCE_DOWNLOAD_TIMEOUT')), 10000);
    req.once('error', (error) => finish(error));
    req.end();
  });
}
