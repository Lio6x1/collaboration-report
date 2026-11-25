import { spawn } from 'node:child_process';
import { readFileSync, existsSync, openSync, closeSync, mkdirSync } from 'node:fs';
import { resolve, basename } from 'node:path';
import { randomUUID } from 'node:crypto';
import net from 'node:net';
import { root, runtime, state, saveJSON, pause, command, nodeArguments } from './runtime-lib.mjs';
import { checkHTTPS } from './https-readiness.mjs';
const registry = resolve(runtime, 'processes.json'),
  startupFile = resolve(runtime, 'startup.json'),
  roles = ['account', 'document', 'search', 'collaboration'];
function diagnostic(stage, facts = {}) {
  // 故障定位只输出固定阶段名；不打印启动参数、私密配置或服务响应正文。
  if (process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === '1')
    console.error(JSON.stringify({ event: 'supervisor_probe', stage, ...facts }));
}
const probeFailures = new Map();
async function probe(role, action) {
  try {
    await action();
    probeFailures.delete(role);
  } catch (error) {
    // 只记录固定角色与稳定错误码；相同失败不逐轮刷屏，也不暴露响应正文或配置。
    const code = /^[A-Z][A-Z0-9_]{1,63}$/.test(error.code ?? error.message)
      ? (error.code ?? error.message)
      : 'PROBE_FAILED';
    if (probeFailures.get(role) !== code) diagnostic('PROBE_FAILED', { role, code });
    probeFailures.set(role, code);
    throw error;
  }
}
function ownedProcess(item) {
  if (process.platform !== 'win32') throw Error('PROCESS_INSPECTION_REQUIRES_WINDOWS');
  const facts = JSON.parse(
    command(
      'powershell',
      [
        '-NoProfile',
        '-NonInteractive',
        '-File',
        resolve(root, 'scripts/process-owner.ps1'),
        '-ProcessId',
        String(item.pid),
      ],
      { failureCode: 'PROCESS_IDENTITY_UNAVAILABLE' },
    ),
  );
  if (facts.state === 'ABSENT') return false;
  // PID可被重复分配，必须同时匹配本项目的绝对入口和随机启动标记。
  if (
    !facts.commandLine.includes(item.entry) ||
    !facts.commandLine.includes('--run-owner ' + item.owner)
  )
    throw Error('PROCESS_OWNER_MISMATCH');
  if (basename(facts.executablePath).toLowerCase() !== basename(item.executable).toLowerCase())
    throw Error('PROCESS_OWNER_MISMATCH');
  return true;
}
async function stop() {
  if (!existsSync(registry)) return;
  const list = JSON.parse(readFileSync(registry));
  // 先核验整组，再终止；遇到外来进程就停止清理。
  const owned = list.filter(ownedProcess);
  for (const item of owned) process.kill(item.pid);
  saveJSON(registry, []);
  saveJSON(startupFile, { state: 'STOPPED' });
  console.log(JSON.stringify({ state: 'STOPPED', databaseVolumeRetained: true }));
}
function check(port, path, owner) {
  return checkHTTPS(port, path, owner, { ca: readFileSync(resolve(runtime, 'pki/ca.pem')) });
}
async function start() {
  diagnostic('START');
  const recorded = existsSync(registry) ? JSON.parse(readFileSync(registry)) : [];
  const live = recorded.filter(ownedProcess);
  const previous = existsSync(startupFile) ? JSON.parse(readFileSync(startupFile)) : null;
  // 观察窗口结束不等于服务失败。只恢复本项目、同一 owner 且整组仍存活的未完成启动。
  // 已退出的部分角色不自动重启，避免把未知结果或崩溃隐蔽地当作恢复成功。
  const resuming = live.length > 0;
  if (resuming) {
    if (!['STARTING', 'OBSERVATION_TIMEOUT'].includes(previous?.state))
      throw Error('SERVICES_ALREADY_RUNNING');
    if (live.length !== recorded.length) throw Error('PROCESS_EXITED');
    if (live.some((item) => item.owner !== previous.owner)) throw Error('PROCESS_OWNER_MISMATCH');
    if (new Set(live.map((item) => item.role)).size !== live.length)
      throw Error('PROCESS_REGISTRY_INVALID');
  }
  const env = state();
  mkdirSync(resolve(runtime, 'bin'), { recursive: true });
  mkdirSync(resolve(runtime, 'logs'), { recursive: true });
  for (const role of resuming ? [] : roles.filter((x) => x !== 'collaboration')) {
    diagnostic('BUILD_' + role.toUpperCase());
    command(
      'go',
      [
        '-C',
        resolve(root, 'services', role),
        'build',
        '-mod=readonly',
        '-o',
        resolve(runtime, 'bin', role + '.exe'),
        '.',
      ],
      {
        timeout: 180000,
        env: { ...process.env, GOWORK: 'off', GOCACHE: resolve(runtime, 'go-cache') },
        failureCode: 'GO_BUILD_FAILED',
      },
    );
  }
  const owner = resuming ? previous.owner : randomUUID(),
    list = resuming ? live : [];
  let stage = 'COLLABORATION_BOOTSTRAP_POLL';
  function recordStartup(status) {
    saveJSON(startupFile, {
      state: status,
      stage,
      owner,
      roles: list.map(({ role, pid }) => ({ role, pid })),
      updatedAt: new Date().toISOString(),
    });
  }
  recordStartup('STARTING');
  function launch(role, executable, entry, args) {
    // 恢复只补齐缺失角色；尤其不能再次启动仍在恢复持久房间的 Node。
    if (list.some((item) => item.role === role)) return;
    diagnostic('LAUNCH_' + role.toUpperCase());
    const out = openSync(resolve(runtime, 'logs', role + '.log'), 'a');
    const entryArgs = args.concat('--run-owner', owner);
    const child = spawn(
      executable,
      executable === process.execPath ? nodeArguments(entryArgs) : entryArgs,
      {
        cwd: root,
        windowsHide: true,
        detached: true,
        stdio: ['ignore', out, out],
        env: { ...process.env, COLLAB_RUN_OWNER: owner },
      },
    );
    closeSync(out);
    child.unref();
    child.on('error', () => {});
    list.push({ role, pid: child.pid, owner, entry, executable });
    saveJSON(registry, list);
    recordStartup('STARTING');
  }
  async function waitForStage(action, failureCode) {
    for (let n = 0; n < 80; n++) {
      try {
        await action();
        return;
      } catch {
        await pause(250);
      }
    }
    throw Error(failureCode);
  }
  try {
    // 先完成Node持久房间/旧输出的本地bootstrap，再启动依赖它的账户关闭worker。
    // 否则积压关闭在Node尚未监听时反复访问SQL，与整篇恢复争用I/O。
    // live只证明监听开放前的bootstrap已完成；下面仍逐角色校验真实ready/gate和owner。
    launch(
      'collaboration',
      process.execPath,
      resolve(root, 'services/collaboration/src/main.mjs'),
      [
        resolve(root, 'services/collaboration/src/main.mjs'),
        '--config',
        resolve(runtime, 'services/collaboration.json'),
      ],
    );
    diagnostic('COLLABORATION_BOOTSTRAP_POLL');
    await waitForStage(async () => {
      await probe('collaboration', () => check(env.ports.collaborationHttp, '/health/live', owner));
      if (!ownedProcess(list.find((item) => item.role === 'collaboration')))
        throw Error('PROCESS_EXITED');
    }, 'COLLABORATION_BOOTSTRAP_TIMEOUT');
    diagnostic('COLLABORATION_BOOTSTRAPPED');
    if (env.ports.smtp)
      launch('smtp', process.execPath, resolve(root, 'scripts/smtp-receiver.mjs'), [
        resolve(root, 'scripts/smtp-receiver.mjs'),
      ]);
    for (const role of roles.filter((item) => item !== 'collaboration')) {
      const cfg = resolve(runtime, 'services', role + '.json');
      const exe = resolve(runtime, 'bin', role + '.exe');
      launch(role, exe, exe, ['--config', cfg]);
    }
    launch('edge', process.execPath, resolve(root, 'scripts/edge.mjs'), [
      resolve(root, 'scripts/edge.mjs'),
    ]);
    stage = 'READY_POLL';
    recordStartup('STARTING');
    diagnostic(stage);
    await waitForStage(async () => {
      await Promise.all(
        roles.map((role) =>
          probe(role, () => check(env.ports[role + 'Http'], '/health/ready', owner)),
        ),
      );
      await probe('edge', () => check(env.ports.edge, '/', owner));
      if (env.ports.smtp) await probe('smtp', () => checkSMTP(env.ports.smtp, owner));
      diagnostic('OUTPUTS_READY');
      if (!list.every(ownedProcess)) throw Error('PROCESS_EXITED');
      diagnostic('OWNER_VERIFIED');
    }, 'SERVICE_READINESS_TIMEOUT');
    saveJSON(resolve(root, 'artifacts/batches/01/services.json'), {
      state: 'READY',
      url: `https://localhost:${env.ports.edge}`,
      roles,
      startedAt: new Date().toISOString(),
    });
    recordStartup('READY');
    console.log(JSON.stringify({ state: 'READY', url: `https://localhost:${env.ports.edge}` }));
    diagnostic('START_COMPLETED');
  } catch (err) {
    if (['COLLABORATION_BOOTSTRAP_TIMEOUT', 'SERVICE_READINESS_TIMEOUT'].includes(err.message)) {
      // 保留有身份凭据的整组存活进程，返回非零表示尚未就绪；后续 start 继续观察。
      // 身份检查失败或进程退出仍走原失败清理，不扩大业务或探针期限。
      if (list.length > 0 && list.every(ownedProcess)) {
        recordStartup('OBSERVATION_TIMEOUT');
        console.error(
          JSON.stringify({
            state: 'OBSERVATION_TIMEOUT',
            stage,
            owner,
            roles: list.map(({ role, pid }) => ({ role, pid })),
          }),
        );
        throw err;
      }
    }
    await stop();
    throw err;
  }
}
function checkSMTP(port, owner) {
  return new Promise((done, reject) => {
    const socket = net.connect(port, '127.0.0.1');
    socket.setTimeout(1000, () => socket.destroy(Error('SMTP_NOT_READY')));
    socket.once('data', (data) => {
      socket.end();
      data.toString().includes('collaboration-dev ' + owner)
        ? done()
        : reject(Error('SMTP_OWNER_MISMATCH'));
    });
    socket.once('error', reject);
  });
}
try {
  if (process.argv[2] === 'stop') await stop();
  else await start();
} catch (err) {
  console.error(
    JSON.stringify({
      code: /^[A-Z0-9_]+$/.test(err.message) ? err.message : 'SERVICE_SUPERVISOR_FAILED',
    }),
  );
  process.exitCode = 1;
}
