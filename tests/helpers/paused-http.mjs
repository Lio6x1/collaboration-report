import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { resolve } from 'node:path';
import { root, state } from '../../scripts/runtime-lib.mjs';
import { until } from './authorization-fence.mjs';

// Python标准TLS验证CA/hostname，小接收窗与暂停读取只控制真实客户端。
// 原3秒总预算、HTTP完整性及实际EOF/错误原因保留；不以客户端超时替代服务端关闭。
export function pausedRead(path, user) {
  const sent = Promise.withResolvers();
  const completed = Promise.withResolvers();
  const child = spawn('python', [resolve(root, 'tests/helpers/paused-http.py')], {
    cwd: root,
    windowsHide: true,
    stdio: 'pipe',
  });
  let buffered = '',
    resumed = false,
    sentObserved = false,
    ended = false,
    exited = false,
    exitCode;
  const failed = () => {
    if (!sentObserved) sent.reject(Error('ACTUAL_HTTP_CLIENT_FAILED'));
    if (!ended) {
      ended = true;
      completed.resolve({ complete: false, failed: true });
    }
  };
  child.stdout.on('data', (chunk) => {
    buffered += chunk.toString();
    const lines = buffered.split(/\r?\n/);
    buffered = lines.pop();
    for (const line of lines) {
      if (!line.trim()) continue;
      let value;
      try {
        value = JSON.parse(line);
      } catch {
        failed();
        continue;
      }
      if (value.state === 'REQUEST_SENT') {
        sentObserved = true;
        sent.resolve(value);
      }
      if (value.state === 'ENDED') {
        ended = true;
        completed.resolve(value);
      }
      if (value.state === 'FAILED') failed();
    }
  });
  // 失败只输出固定状态；不把Python异常或Cookie/正文带入TAP。
  child.stderr.on('data', () => {});
  child.stdin.on('error', () => {});
  child.once('error', failed);
  child.once('close', (code) => {
    exited = true;
    exitCode = code;
    if (!ended || !sentObserved) failed();
  });
  // 私有Cookie通过匿名管道传入，不能放进命令行、进程列表或测试日志。
  child.stdin.write(
    JSON.stringify({ root, port: state().ports.edge, path, headers: user.headers }) + '\n',
  );
  const resume = () => {
    if (resumed || child.stdin.destroyed) return;
    resumed = true;
    child.stdin.end('RESUME\n');
  };
  const close = async () => {
    resume();
    await until(() => exited, 'ACTUAL_HTTP_HELPER_EXIT_UNKNOWN', 5000);
    assert.equal(exitCode, 0, 'ACTUAL_HTTP_HELPER_FAILED');
  };
  return { sent: sent.promise, completed: completed.promise, resume, close };
}
