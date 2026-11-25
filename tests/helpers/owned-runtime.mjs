import assert from 'node:assert/strict';
import { readFileSync, openSync, closeSync } from 'node:fs';
import { basename, resolve } from 'node:path';
import { spawnSync, spawn } from 'node:child_process';
import https from 'node:https';
import {
  root,
  runtime,
  nodeArguments,
  command,
  saveJSON,
  state,
  pause,
} from '../../scripts/runtime-lib.mjs';
import { checkHTTPS } from '../../scripts/https-readiness.mjs';

export function stopOwnedRole(role) {
  assert.ok(['account', 'collaboration', 'document', 'search'].includes(role));
  const item = JSON.parse(readFileSync(resolve(runtime, 'processes.json'))).find(
    (p) => p.role === role,
  );
  assert.ok(item, 'OWNED_ROLE_MISSING');
  const result = spawnSync(
    'powershell',
    [
      '-NoProfile',
      '-NonInteractive',
      '-File',
      resolve(root, 'scripts/process-owner.ps1'),
      '-ProcessId',
      String(item.pid),
    ],
    { cwd: root, windowsHide: true, encoding: 'utf8', timeout: 10000 },
  );
  assert.equal(result.status, 0, 'PROCESS_IDENTITY_UNAVAILABLE');
  const facts = JSON.parse(result.stdout);
  assert.equal(facts.state, 'PRESENT');
  assert.ok(facts.commandLine.toLowerCase().includes(item.entry.toLowerCase()));
  assert.ok(facts.commandLine.includes('--run-owner ' + item.owner));
  assert.equal(
    basename(facts.executablePath).toLowerCase(),
    basename(item.executable).toLowerCase(),
  );
  // 三项实际归属证据齐全后才注入指定服务退出，保留其全部数据库事实与其他项目进程。
  process.kill(item.pid);
  return item.pid;
}

export async function restartOwnedSearch() {
  const registry = resolve(runtime, 'processes.json');
  const items = JSON.parse(readFileSync(registry));
  const item = items.find((entry) => entry.role === 'search');
  assert.ok(item, 'OWNED_SEARCH_MISSING');
  const inspected = command('powershell', [
    '-NoProfile',
    '-NonInteractive',
    '-File',
    resolve(root, 'scripts/process-owner.ps1'),
    '-ProcessId',
    String(item.pid),
  ]);
  const facts = JSON.parse(inspected);
  if (facts.state === 'PRESENT') {
    // 首次启动已登记新PID但Ready未知时，finally只能核验并等待原实例；
    // 不能启动第二个消费者，也不能用PROCESS_STILL_PRESENT覆盖原失败。
    assert.ok(facts.commandLine.toLowerCase().includes(item.entry.toLowerCase()));
    assert.ok(facts.commandLine.includes('--run-owner ' + item.owner));
    assert.equal(
      basename(facts.executablePath).toLowerCase(),
      basename(item.executable).toLowerCase(),
    );
    return waitOwnedSearchReady(item.owner);
  }
  assert.equal(facts.state, 'ABSENT', 'SEARCH_PROCESS_IDENTITY_UNKNOWN');
  // 只恢复已确认退出的本owner Search。其余五个进程和数据库保持，避免无关全站重启。
  command(
    'D:/go1.25.10/bin/go.exe',
    ['-C', resolve(root, 'services/search'), 'build', '-mod=readonly', '-o', item.executable, '.'],
    {
      timeout: 180000,
      env: { ...process.env, GOWORK: 'off', GOCACHE: resolve(runtime, 'go-cache') },
    },
  );
  const output = openSync(resolve(runtime, 'logs/search.log'), 'a');
  const child = spawn(
    item.executable,
    ['--config', resolve(runtime, 'services/search.json'), '--run-owner', item.owner],
    {
      cwd: root,
      detached: true,
      windowsHide: true,
      stdio: ['ignore', output, output],
      env: { ...process.env, COLLAB_RUN_OWNER: item.owner },
    },
  );
  closeSync(output);
  child.unref();
  child.on('error', () => {});
  assert.ok(child.pid, 'SEARCH_PROCESS_NOT_STARTED');
  item.pid = child.pid;
  saveJSON(registry, items);
  return waitOwnedSearchReady(item.owner);
}

async function waitOwnedSearchReady(owner) {
  const ca = readFileSync(resolve(runtime, 'pki/ca.pem'));
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    try {
      await checkHTTPS(state().ports.searchHttp, '/health/ready', owner, { ca });
      return;
    } catch {
      await pause(250);
    }
  }
  assert.fail('SEARCH_RESTART_NOT_READY');
}

export function restartOwnedRuntime() {
  let stages = [];
  for (const args of [['stop'], []]) {
    const result = spawnSync(
      process.execPath,
      nodeArguments([resolve(root, 'scripts/start.mjs'), ...args]),
      {
        cwd: root,
        windowsHide: true,
        encoding: 'utf8',
        timeout: 90000,
      },
    );
    assert.equal(
      result.status,
      0,
      'OWNED_RESTART_FAILED ' +
        JSON.stringify({
          phase: args.length ? 'stop' : 'start',
          exit: result.status,
          signal: result.signal,
          error_code: result.error?.code ?? null,
          declared_ready: /"state":"READY"/.test(result.stdout ?? ''),
          native_assertion:
            /Assertion failed:[^\r\n]{0,400}/.exec(result.stderr ?? '')?.[0] ?? null,
          stdout_bytes: Buffer.byteLength(result.stdout ?? ''),
          stderr_bytes: Buffer.byteLength(result.stderr ?? ''),
          last_stage:
            [...(result.stderr ?? '').matchAll(/"stage":"([A-Z_]+)"/g)].at(-1)?.[1] ?? null,
          supervisor_code:
            [...(result.stderr ?? '').matchAll(/"code":"([A-Z0-9_]+)"/g)].at(-1)?.[1] ?? null,
          probe_failures: [
            ...(result.stderr ?? '').matchAll(
              /"role":"(account|document|search|collaboration|edge|smtp)","code":"([A-Z0-9_]+)"/g,
            ),
          ].map(([, role, code]) => ({ role, code })),
        }),
    );
    if (!args.length)
      stages = [...(result.stderr ?? '').matchAll(/"stage":"([A-Z_]+)"/g)].map(
        ([, stage]) => stage,
      );
  }
  https.globalAgent.destroy();
  return { stages };
}
