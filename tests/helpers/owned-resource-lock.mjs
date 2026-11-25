import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { root, runtime } from '../../scripts/runtime-lib.mjs';
import { until } from './authorization-fence.mjs';

// 父测试只提供已核对自有resource行的键，helper再次限定原项目默认对象目录。
// 父退出、精确释放信号或15秒看门狗均释放真实Windows文件句柄。
export async function lockOwnedResource(key) {
  assert.equal(process.platform, 'win32');
  assert.match(key, /^[a-f0-9]{64}$/);
  const folder = resolve(runtime, 'faults');
  mkdirSync(folder, { recursive: true });
  const signal = resolve(folder, 'gc-release-' + key + '-' + randomUUID());
  const child = spawn(
    'powershell.exe',
    [
      '-NoProfile',
      '-NonInteractive',
      '-File',
      resolve(root, 'tests/helpers/owned-resource-lock.ps1'),
      '-ProjectRoot',
      root,
      '-StorageKey',
      key,
      '-ReleaseSignal',
      signal,
      '-ParentProcessId',
      String(process.pid),
    ],
    { cwd: root, windowsHide: true, stdio: 'pipe' },
  );
  let buffered = '',
    exited = false,
    exitCode,
    stderrBytes = 0;
  const states = [];
  child.stdout.on('data', (chunk) => {
    buffered += chunk.toString();
    const lines = buffered.split(/\r?\n/);
    buffered = lines.pop();
    for (const line of lines) if (line.trim()) states.push(JSON.parse(line).state);
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
  let released;
  const release = () =>
    (released ??= (async () => {
      writeFileSync(signal, 'release', { flag: 'wx' });
      try {
        await until(() => exited, 'RESOURCE_LOCK_RELEASE_UNKNOWN', 20000);
      } finally {
        // 精确移除本次随机信号，无递归删除；它不属于用户资源或服务持久账本。
        rmSync(signal, { force: true });
      }
      assert.equal(exitCode, 0, 'RESOURCE_LOCK_HELPER_FAILED stderr_bytes=' + stderrBytes);
      assert.ok(states.includes('RELEASED'));
    })());
  try {
    await until(
      () => {
        assert.equal(exited, false, 'RESOURCE_LOCK_START_FAILED stderr_bytes=' + stderrBytes);
        return states.includes('LOCKED');
      },
      'RESOURCE_LOCK_START_TIMEOUT',
      4000,
    );
    return {
      release,
      assertHeld() {
        assert.equal(exited, false, 'RESOURCE_FAULT_RELEASED_EARLY');
        assert.equal(states.includes('RELEASED'), false, 'RESOURCE_FAULT_RELEASED_EARLY');
      },
    };
  } catch (error) {
    await release();
    throw error;
  }
}
