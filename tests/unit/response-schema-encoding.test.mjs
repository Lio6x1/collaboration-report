import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
import test from 'node:test';
import { root } from '../../scripts/runtime-lib.mjs';

function validateSnippet(snippet) {
  // 真实Node→Python匿名管道发送UTF-8；Windows默认GBK不能改写Unicode内容，
  // 否则合法emoji边界会误报、正文语义也会被验收工具悄悄改变。
  return spawnSync('python', [resolve(root, 'scripts/validate-responses.py')], {
    cwd: root,
    input: JSON.stringify([
      {
        schema: 'SearchResultPage',
        value: {
          items: [{ document_id: '1', release_id: '2', title: '中😀'.repeat(120), snippet }],
          next_cursor: null,
          has_more: false,
        },
      },
    ]),
    encoding: 'utf8',
    windowsHide: true,
    timeout: 10000,
    maxBuffer: 4096,
  });
}

test('real UTF-8 response pipe preserves exactly 2000 Unicode snippet characters', () => {
  const result = validateSnippet('中😀'.repeat(1000));
  assert.equal(result.status, 0, 'UTF8_RESPONSE_PIPE_CORRUPTED_VALID_UNICODE');
  assert.deepEqual(JSON.parse(result.stdout), { state: 'PASS', responses: 1 });
});

test('real UTF-8 response pipe still rejects 2001 Unicode snippet characters', () => {
  const result = validateSnippet('中😀'.repeat(1000) + '尾');
  assert.equal(result.status, 1);
  assert.deepEqual(JSON.parse(result.stdout), {
    code: 'RESPONSE_SCHEMA_INVALID',
    model: 'SearchResultPage',
    path: ['items', 0, 'snippet'],
  });
});
