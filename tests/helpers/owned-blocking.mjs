import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { resolve } from 'node:path';
import { root } from '../../scripts/runtime-lib.mjs';
import { until } from './authorization-fence.mjs';

// 独立 Windows 辅助进程核验 owner 后暂停线程；父测试退出和15秒看门狗都自动恢复。
// 不更改服务入口，不打开调试端口，不把模拟等待冒充真实 Node 阻塞。
export async function prepareOwnedBlocking(role) {
  assert.equal(process.platform, 'win32', 'ACTUAL_WINDOWS_REQUIRED');
  assert.ok(['collaboration', 'edge'].includes(role), 'OWNED_BLOCKING_ROLE_INVALID');
  const child = spawn(
    'powershell.exe',
    [
      '-NoProfile',
      '-NonInteractive',
      '-File',
      resolve(root, 'tests/helpers/owned-blocking.ps1'),
      '-ProjectRoot',
      root,
      '-ParentProcessId',
      String(process.pid),
    ],
    {
      cwd: root,
      windowsHide: true,
      stdio: 'pipe',
      env: { ...process.env, COLLAB_TEST_BLOCK_ROLE: role },
    },
  );
  const states = [];
  let buffered = '',
    exited = false,
    exitCode,
    stderrBytes = 0;
  child.stdout.on('data', (chunk) => {
    buffered += chunk.toString();
    const lines = buffered.split(/\r?\n/);
    buffered = lines.pop();
    for (const line of lines) {
      if (!line.trim()) continue;
      try {
        states.push(JSON.parse(line));
      } catch {
        states.push({ state: 'INVALID_HELPER_OUTPUT' });
      }
    }
  });
  child.stderr.on('data', (chunk) => {
    stderrBytes += chunk.length;
  });
  child.on('error', () => {
    exited = true;
    exitCode = -1;
  });
  child.on('close', (code) => {
    exited = true;
    exitCode = code;
  });
  let releaseResult;
  const release = () =>
    (releaseResult ??= (async () => {
      if (!child.stdin.destroyed) child.stdin.end('RESUME\n');
      // 独立辅助进程最多阻塞15秒，再留有限时间收集真实恢复事实；不直接杀掉持有线程句柄的辅助进程。
      await until(() => exited, 'OWNED_BLOCKING_RECOVERY_UNKNOWN', 20000);
      assert.equal(exitCode, 0, 'OWNED_BLOCKING_HELPER_FAILED stderr_bytes=' + stderrBytes);
      assert.ok(
        states.some((value) => value.state === 'RESUMED'),
        'ACTUAL_RESUME_REQUIRED',
      );
    })());
  const block = async () => {
    child.stdin.write('BLOCK\n');
    await until(
      () => {
        if (exited) assert.fail('OWNED_BLOCKING_START_FAILED stderr_bytes=' + stderrBytes);
        return states.some((value) => value.state === 'BLOCKED');
      },
      'OWNED_BLOCKING_START_TIMEOUT',
      2000,
    );
    const blocked = states.find((value) => value.state === 'BLOCKED');
    assert.ok(blocked.threadCount > 0, 'ACTUAL_SUSPENDED_THREADS_REQUIRED');
  };
  try {
    await until(
      () => {
        if (exited) assert.fail('OWNED_BLOCKING_START_FAILED stderr_bytes=' + stderrBytes);
        return states.some((value) => value.state === 'PREPARED');
      },
      'OWNED_BLOCKING_START_TIMEOUT',
      10000,
    );
    return { block, release, states };
  } catch (error) {
    await release();
    throw error;
  }
}

export async function blockOwnedCollaboration() {
  const fault = await prepareOwnedBlocking('collaboration');
  try {
    await fault.block();
    return fault;
  } catch (error) {
    await fault.release();
    throw error;
  }
}
