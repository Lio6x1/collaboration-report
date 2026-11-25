import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const runtimeURL = new URL('../../scripts/runtime-lib.mjs', import.meta.url).href;

function nestedLaunch(flags) {
  // 实际启动两层 Node，验证 Windows 兼容参数不会在监督器边界丢失；不用数组快照替代行为。
  const source = `
    import { command } from ${JSON.stringify(runtimeURL)};
    process.stdout.write(command(process.execPath, [
      '--input-type=module', '--eval',
      'console.log(JSON.stringify({ maglev: process.execArgv.includes("--no-maglev"), warnings: process.execArgv.includes("--no-warnings") }))'
    ]));
  `;
  const result = spawnSync(process.execPath, [...flags, '--input-type=module', '--eval', source], {
    encoding: 'utf8',
    windowsHide: true,
    timeout: 10000,
  });
  assert.equal(result.status, 0, result.stderr);
  return JSON.parse(result.stdout);
}

test('默认启动不隐式关闭 Maglev', () => {
  assert.deepEqual(nestedLaunch([]), { maglev: false, warnings: false });
});

test('显式兼容选项传到真实 Node 子进程，其他调用者参数不传播', () => {
  assert.deepEqual(nestedLaunch(['--no-maglev', '--no-warnings']), {
    maglev: true,
    warnings: false,
  });
});
