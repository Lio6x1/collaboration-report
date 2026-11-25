import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import { root, runtime, state } from '../../scripts/runtime-lib.mjs';
import { get, json, organizationFixture } from '../helpers/workspace.mjs';
import { stopOwnedRole, restartOwnedSearch } from '../helpers/owned-runtime.mjs';
import { until } from '../helpers/authorization-fence.mjs';
import {
  checkedSearchPage,
  pageVisibility,
  publishSearchDocument,
  searchPagePath,
} from '../helpers/search-pages.mjs';

async function waitExistingDocumentConnection() {
  const entries = JSON.parse(readFileSync(resolve(runtime, 'processes.json')));
  const docs = entries.filter((entry) => entry.role === 'document');
  assert.equal(docs.length, 1);
  const doc = docs[0];
  const inspected = spawnSync(
    'powershell',
    [
      '-NoProfile',
      '-NonInteractive',
      '-File',
      resolve(root, 'scripts/process-owner.ps1'),
      '-ProcessId',
      String(doc.pid),
    ],
    { encoding: 'utf8', windowsHide: true, timeout: 10000 },
  );
  assert.equal(inspected.status, 0, 'PAGE_RECOVERY_DOC_IDENTITY_UNKNOWN');
  const facts = JSON.parse(inspected.stdout);
  assert.equal(facts.state, 'PRESENT');
  assert.ok(facts.commandLine.toLowerCase().includes(doc.entry.toLowerCase()));
  assert.ok(facts.commandLine.includes('--run-owner ' + doc.owner));
  const port = state().ports.searchRpc;
  assert.ok(
    Number.isInteger(doc.pid) && doc.pid > 0 && Number.isInteger(port) && port > 0 && port <= 65535,
  );
  // Search自己的HTTP Ready不代表Doc既有gRPC通道已经走完重连退避。
  // 先观察这个真实Doc进程到准确Search端口的Established连接；不循环请求业务页。
  await until(
    () => {
      // netstat读取内核连接表；避免Windows CIM全表枚举本身超过观察预算。
      const observed = spawnSync('netstat', ['-ano', '-p', 'TCP'], {
        encoding: 'utf8',
        windowsHide: true,
        timeout: 3000,
      });
      assert.equal(observed.status, 0, 'PAGE_RECOVERY_CONNECTION_OBSERVATION_UNKNOWN');
      return observed.stdout.split(/\r?\n/).some((line) => {
        const fields = line.trim().split(/\s+/);
        return (
          fields.length === 5 &&
          fields[0] === 'TCP' &&
          fields[2] === `127.0.0.1:${port}` &&
          fields[3] === 'ESTABLISHED' &&
          fields[4] === String(doc.pid)
        );
      });
    },
    'PAGE_RECOVERY_DOC_SEARCH_NOT_RECONNECTED',
    10000,
  );
}

test('真实Search进程不可达时HTTP整页失败，恢复同owner后新链成功', { timeout: 90000 }, async () => {
  const f = await organizationFixture(0);
  f.q = 'pageunavailable' + randomBytes(12).toString('hex');
  await pageVisibility(`/v1/knowledge-bases/${f.kb}`, f.owner, 'PUBLIC');
  f.docs = [
    await publishSearchDocument(f, {
      q: f.q,
      title: `${f.q} 固定标题`,
      body: `${f.q} 正式摘要`,
    }),
  ];
  let stopped = false;
  try {
    // 既有helper核验真实PID、可执行文件、入口路径和随机run-owner后才停止。
    // 只退出Search，不停ES/Doc/Edge，不改SQL或共享配置以制造故障。
    stopOwnedRole('search');
    stopped = true;
    const unavailable = await get(searchPagePath(f.q, { limit: 1 }));
    assert.equal(unavailable.status, 503);
    const error = json(unavailable);
    assert.equal(error.code, 'SEARCH_INCOMPLETE');
    assert.equal(Object.hasOwn(error, 'items'), false);
    assert.equal(Object.hasOwn(error, 'next_cursor'), false);
    // 只允许原owner恢复，等待其真实Ready；不重试失败的业务页或延长HTTP三秒预算。
    await restartOwnedSearch();
    stopped = false;
    await waitExistingDocumentConnection();
    const page = await checkedSearchPage(f, null, { limit: 1 });
    assert.equal(page.items.length, 1);
    assert.equal(page.items[0].document_id, f.docs[0].id);
    assert.equal(page.has_more, false);
  } finally {
    if (stopped) await restartOwnedSearch();
  }
});
