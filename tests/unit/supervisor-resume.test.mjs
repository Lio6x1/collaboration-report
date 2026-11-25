import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve, basename } from 'node:path';
import vm from 'node:vm';

// 直接执行监督器主体，替换其外部适配器；避免测试启动/杀死真实服务或等待业务恢复。
const source = readFileSync(new URL('../../scripts/start.mjs', import.meta.url), 'utf8').replace(
  /^import .*;\r?\n/gm,
  '',
);

test('observation timeout preserves the same process and later start completes missing roles', async () => {
  const root = resolve('supervisor-fixture');
  const runtime = resolve(root, '.runtime');
  const files = new Map();
  const children = new Map();
  const launches = [];
  const kills = [];
  let ready = false;
  let bootstrapped = false;
  let waits = 0;
  let builds = 0;
  const registry = resolve(runtime, 'processes.json');
  const startupFile = resolve(runtime, 'startup.json');
  const fakeProcess = {
    platform: 'win32',
    execPath: 'node.exe',
    env: {},
    argv: ['node', 'start.mjs'],
    kill: (pid) => kills.push(pid),
  };
  const context = {
    root,
    runtime,
    resolve,
    basename,
    process: fakeProcess,
    console: { log() {}, error() {} },
    readFileSync: (path) => files.get(path) ?? 'fixture-ca',
    existsSync: (path) => files.has(path),
    mkdirSync() {},
    openSync: () => 1,
    closeSync() {},
    saveJSON: (path, value) => files.set(path, JSON.stringify(value)),
    randomUUID: () => 'same-start-owner',
    state: () => ({
      ports: { collaborationHttp: 1, accountHttp: 2, documentHttp: 3, searchHttp: 4, edge: 5 },
    }),
    pause: async (ms) => {
      assert.equal(ms, 250);
      waits++;
    },
    nodeArguments: (args) => args,
    checkHTTPS: async (_port, _path, owner) => {
      assert.equal(owner, 'same-start-owner');
      if (!(ready || (bootstrapped && _path === '/health/live'))) throw Error('PROBE_TIMEOUT');
    },
    command: (exe, args) => {
      if (exe === 'go') {
        builds++;
        return '';
      }
      const child = children.get(Number(args.at(-1)));
      return JSON.stringify(child ? { state: 'PRESENT', ...child } : { state: 'ABSENT' });
    },
    spawn: (executable, args) => {
      const pid = 100 + launches.length;
      const commandLine = args.join(' ');
      launches.push({ pid, executable, commandLine });
      children.set(pid, {
        commandLine: `${executable} ${commandLine}`,
        executablePath: executable,
      });
      return { pid, unref() {}, on() {} };
    },
  };
  const run = () => vm.runInNewContext(`(async () => {${source}\n})()`, context);
  await run();
  assert.equal(fakeProcess.exitCode, 1);
  assert.equal(waits, 80, '保留原观察次数，不能靠延长期限通过');
  assert.equal(launches.length, 1);
  assert.deepEqual(kills, [], '观察超时不能杀死仍在恢复的 Node');
  const before = JSON.parse(files.get(registry));
  const pending = JSON.parse(files.get(startupFile));
  assert.equal(pending.state, 'OBSERVATION_TIMEOUT');
  assert.equal(pending.stage, 'COLLABORATION_BOOTSTRAP_POLL');
  assert.equal(pending.owner, before[0].owner);
  assert.equal(pending.roles[0].pid, before[0].pid);

  bootstrapped = true;
  await run();
  assert.equal(launches.length, 5);
  assert.equal(JSON.parse(files.get(startupFile)).stage, 'READY_POLL');
  assert.equal(JSON.parse(files.get(startupFile)).state, 'OBSERVATION_TIMEOUT');
  assert.deepEqual(kills, [], '真实 ready 尚未满足也只能保存观察状态');

  ready = true;
  fakeProcess.exitCode = 0;
  await run();
  const after = JSON.parse(files.get(registry));
  assert.equal(fakeProcess.exitCode, 0);
  assert.equal(builds, 3, '恢复不覆盖运行中的二进制');
  assert.equal(launches.length, 5, '只补齐账户、文档、搜索与 edge');
  assert.deepEqual(
    after.find((item) => item.role === 'collaboration'),
    before[0],
  );
  assert.equal(new Set(after.map((item) => item.owner)).size, 1);
  assert.equal(JSON.parse(files.get(startupFile)).state, 'READY');
  assert.deepEqual(kills, []);

  await run();
  assert.equal(fakeProcess.exitCode, 1, '已就绪进程仍拒绝重复启动');
  assert.equal(launches.length, 5);
});
