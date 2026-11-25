import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
import { root } from '../../scripts/runtime-lib.mjs';
test('Windows PowerShell 5 reads Chinese-comment script and finds a live PID', () => {
  // PS5把无BOM的UTF-8当ANSI，中文注释可能让下一行被吞掉，误判为进程不存在。
  const r = spawnSync(
    'powershell',
    [
      '-NoProfile',
      '-NonInteractive',
      '-File',
      resolve(root, 'scripts/process-owner.ps1'),
      '-ProcessId',
      String(process.pid),
    ],
    { windowsHide: true, encoding: 'utf8' },
  );
  assert.equal(r.status, 0);
  const facts = JSON.parse(r.stdout);
  assert.equal(facts.state, 'PRESENT');
  assert.equal(facts.pid, process.pid);
});

test('Windows absent PID proof fits the existing three-second recovery budget', () => {
  // 已核验此PID不存在；真实内核缺失证据不需要加载CIM，原退出证明期限保持三秒。
  const result = spawnSync(
    'powershell',
    [
      '-NoProfile',
      '-NonInteractive',
      '-File',
      resolve(root, 'scripts/process-owner.ps1'),
      '-ProcessId',
      '2147483647',
    ],
    { windowsHide: true, encoding: 'utf8', timeout: 3000 },
  );
  assert.equal(result.status, 0, 'ABSENT_PROCESS_PROOF_EXCEEDED_ORIGINAL_BUDGET');
  assert.deepEqual(JSON.parse(result.stdout), { state: 'ABSENT' });
});

test('Windows invalid PID is rejected rather than returned as an exit proof', () => {
  const result = spawnSync(
    'powershell',
    [
      '-NoProfile',
      '-NonInteractive',
      '-File',
      resolve(root, 'scripts/process-owner.ps1'),
      '-ProcessId',
      '0',
    ],
    { windowsHide: true, encoding: 'utf8' },
  );
  assert.notEqual(result.status, 0);
  assert.ok(!result.stdout.includes('ABSENT'));
});
