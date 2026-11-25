import test from 'node:test';
import assert from 'node:assert/strict';
import https from 'node:https';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from '../../scripts/runtime-lib.mjs';
import {
  sendBufferedResponse,
  anonymousSearchEmptyResponse,
} from '../../scripts/edge-responses.mjs';
import { request } from '../helpers/https.mjs';

async function throughTLS({ status, body, canRelease }) {
  const tls = {
    key: readFileSync(resolve(runtime, 'pki/edge-key.pem')),
    cert: readFileSync(resolve(runtime, 'pki/edge.pem')),
    minVersion: 'TLSv1.3',
  };
  const ca = readFileSync(resolve(runtime, 'pki/ca.pem'));
  const upstream = https.createServer(tls, (_req, res) => {
    res.writeHead(status, {
      'content-type': 'application/json',
      'content-length': Buffer.byteLength(body),
      'x-test-original': 'yes',
    });
    res.end(body);
  });
  await new Promise((resolve) => upstream.listen(0, '127.0.0.1', resolve));
  const gateway = https.createServer(tls, (_req, res) => {
    const proxy = https.get(
      { hostname: 'localhost', port: upstream.address().port, ca },
      (received) => {
        const chunks = [];
        received.on('data', (chunk) => chunks.push(chunk));
        // 只搭建真实 TLS 适配器，响应决策调用产品函数；不在测试实现第二份规则。
        received.on('end', () => {
          sendBufferedResponse(res, {
            status: received.statusCode,
            headers: received.headers,
            body: Buffer.concat(chunks),
            canRelease,
          });
        });
        received.on('error', () => res.destroy());
      },
    );
    proxy.on('error', () => res.destroy());
  });
  await new Promise((resolve) => gateway.listen(0, '127.0.0.1', resolve));
  try {
    return await request(gateway.address().port, '/buffered');
  } finally {
    for (const server of [gateway, upstream]) {
      server.closeAllConnections();
      await new Promise((resolve) => server.close(resolve));
    }
  }
}

test('a complete empty upstream 503 becomes a retryable public Error DTO', async () => {
  const response = await throughTLS({
    status: 503,
    body: '',
    canRelease: () => assert.fail('错误响应不使用成功正文准入'),
  });
  assert.equal(response.status, 503);
  assert.ok(response.body.length > 0, '完整空 503 必须具有冻结错误正文');
  const error = JSON.parse(response.body);
  assert.equal(error.code, 'SERVICE_UNAVAILABLE');
  assert.equal(error.retry_action, 'RETRY_SAME_OPERATION');
  assert.match(error.trace_id, /^[0-9a-f-]{36}$/);
  assert.equal(response.headers['cache-control'], 'no-store');
  assert.notEqual(response.headers['content-length'], '0');
  assert.equal(response.headers['x-test-original'], undefined);
});

test('a nonempty upstream error keeps its exact original bytes', async () => {
  const body = JSON.stringify({ code: 'DEPENDENCY_TIMEOUT', retry_action: 'RETRY_SAME_OPERATION' });
  const response = await throughTLS({
    status: 503,
    body,
    canRelease: () => assert.fail('错误响应不使用成功正文准入'),
  });
  assert.equal(response.status, 503);
  assert.equal(response.body, body);
  assert.equal(response.headers['x-test-original'], 'yes');
});

test('a buffered success without live admission never releases its private body', async () => {
  const response = await throughTLS({
    status: 200,
    body: 'private fixture',
    canRelease: () => false,
  });
  assert.equal(response.status, 503);
  assert.equal(JSON.parse(response.body).code, 'OUTPUT_LEASE_REQUIRED');
  assert.equal(response.body.includes('private fixture'), false);
});

test('a fully buffered admitted success keeps its actual status and body', async () => {
  const body = '{"text":"完整中文正文"}';
  const response = await throughTLS({ status: 200, body, canRelease: () => true });
  assert.equal(response.status, 200);
  assert.equal(response.body, body);
});

const emptySearch = '{"items":[],"next_cursor":null,"has_more":false}';
const emptySearchInput = (overrides = {}) => ({
  authority: 'document',
  method: 'GET',
  path: '/v1/search',
  status: 200,
  body: Buffer.from(emptySearch + '\n'),
  ...overrides,
});

test('anonymous search-empty accepts only the two fixed Go output byte forms', () => {
  // WriteJSON使用json.Encoder会加一次换行；已登记输出Marshal没有换行。
  // 比较原bytes，不允许JSON.parse覆盖重复键后再放行携带私密内容的原响应。
  for (const body of [Buffer.from(emptySearch), Buffer.from(emptySearch + '\n')]) {
    assert.equal(anonymousSearchEmptyResponse(emptySearchInput({ body })), true);
    assert.equal(
      anonymousSearchEmptyResponse(emptySearchInput({ body, cookie: 'theme=dark; other=ok' })),
      true,
    );
  }
});

test('anonymous search-empty never exempts a session-bearing or differently routed response', () => {
  for (const overrides of [
    { authority: 'account' },
    { authority: 'search' },
    { method: 'POST' },
    { method: 'HEAD' },
    { method: 'get' },
    { path: '/v1/search/' },
    { path: '/v1/search?q=x' },
    { path: '/v1/documents' },
    { status: 201 },
    { status: 204 },
    { status: 404 },
    { cookie: '__Host-collabSession=actual-session' },
    { cookie: '__Host-collabSession=' },
    { cookie: '__Host-collabSession' },
    { cookie: 'other=x; __Host-collabSession = broken' },
    { cookie: '__Host-collabSession=one; __Host-collabSession=two' },
    // Go strings.TrimSpace包含NEL，不能仅用JS trim漏掉这个畸形会话名。
    { cookie: '\u0085__Host-collabSession=x' },
    { cookie: ['__Host-collabSession=x'] },
  ]) {
    assert.equal(anonymousSearchEmptyResponse(emptySearchInput(overrides)), false);
  }
});

test('anonymous search-empty rejects private, duplicate, extra and noncanonical original bytes', () => {
  for (const body of [
    emptySearch.replace('[]', '[{"title":"private fixture"}]'),
    emptySearch.replace('null', '"private-cursor"'),
    emptySearch.replace('false', 'true'),
    emptySearch.replace('}', ',"private":"fixture"}'),
    '{"items":[{"title":"private fixture"}],"items":[],"next_cursor":null,"has_more":false}',
    '{"items":[],"next_cursor":"private-cursor","next_cursor":null,"has_more":false}',
    '{"has_more":false,"next_cursor":null,"items":[]}',
    emptySearch + '\n\n',
    emptySearch + '\r\n',
    ' ' + emptySearch,
    '\ufeff' + emptySearch,
    emptySearch.replace('[]', 'null'),
    '{}',
  ]) {
    assert.equal(
      anonymousSearchEmptyResponse(emptySearchInput({ body: Buffer.from(body) })),
      false,
    );
  }
  for (const body of [
    Buffer.concat([Buffer.from(emptySearch), Buffer.from([0xff])]),
    Buffer.from(emptySearch, 'utf16le'),
    emptySearch,
    undefined,
  ]) {
    assert.equal(anonymousSearchEmptyResponse(emptySearchInput({ body })), false);
  }
});
