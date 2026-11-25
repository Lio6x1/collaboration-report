import { spawn } from 'node:child_process';
import { copyFileSync, mkdirSync, openSync, closeSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { root, runtime, state, saveJSON, pause, nodeArguments } from './runtime-lib.mjs';
import { checkHTTPS } from './https-readiness.mjs';
import { checkServer } from './server-health.mjs';

// 此入口只管理自身启动的子进程，不调用Docker、不读宿主PID、不接管Windows实例。
if (process.platform !== 'linux') throw Error('SERVER_REQUIRES_LINUX');
const environment = state();
if (environment.deployment !== 'linux-compose' || environment.root !== root)
  throw Error('SERVER_CONFIGURATION_MISMATCH');
// Compose依赖顺序不能替代直接运行入口的约束：只接受同一部署已完成的真实初始化回执。
const initialization = JSON.parse(readFileSync(resolve(runtime, 'initialized.json'), 'utf8'));
if (
  initialization.state !== 'DEPENDENCIES_INITIALIZED' ||
  initialization.owner !== environment.owner ||
  initialization.project !== environment.project
)
  throw Error('SERVER_INITIALIZATION_REQUIRED');
const owner = randomUUID();
const children = [];
let stopping = false;
let failed = false;
let stage = 'STARTING';
let observationEnded = false;
mkdirSync(resolve(runtime, 'bin'), { recursive: true });
mkdirSync(resolve(runtime, 'logs'), { recursive: true });

function record(status) {
  saveJSON(
    resolve(runtime, 'processes.json'),
    children.map(({ facts }) => facts),
  );
  saveJSON(resolve(runtime, 'startup.json'), {
    state: status,
    owner,
    stage,
    roles: children.map(({ facts: { role, pid } }) => ({ role, pid })),
    updatedAt: new Date().toISOString(),
  });
}

async function stop(abnormal = false) {
  failed ||= abnormal;
  if (stopping) return;
  stopping = true;
  record('STOPPING');
  // ChildProcess句柄来自本监督器；终止目标不由可篡改的磁盘PID列表提供。
  for (const { child } of children) {
    if (child.exitCode === null && child.signalCode === null) child.kill('SIGTERM');
  }
  const deadline = setTimeout(() => {
    for (const { child } of children) {
      if (child.exitCode === null && child.signalCode === null) child.kill('SIGKILL');
    }
  }, 10000);
  await Promise.all(children.map(({ exited }) => exited));
  clearTimeout(deadline);
  record(failed ? 'FAILED' : 'STOPPED');
  process.exitCode = failed ? 1 : 0;
}

function launch(role, executable, entry, args) {
  const log = openSync(resolve(runtime, 'logs', role + '.log'), 'a', 0o600);
  const child = spawn(
    executable,
    executable === process.execPath
      ? nodeArguments([...args, '--run-owner', owner])
      : [...args, '--run-owner', owner],
    { cwd: root, stdio: ['ignore', log, log], env: { ...process.env, COLLAB_RUN_OWNER: owner } },
  );
  closeSync(log);
  const exited = new Promise((done) => {
    child.once('error', () => {
      done();
      void stop(true);
    });
    child.once('exit', () => {
      done();
      if (!stopping) void stop(true);
    });
  });
  children.push({ child, exited, facts: { role, pid: child.pid, owner, entry, executable } });
  record('STARTING');
}

// 观察时间结束只说明尚未就绪，存活实例继续恢复，不能据超时重建原业务操作。
async function observe(action) {
  const started = Date.now();
  while (!stopping) {
    try {
      await action();
      return true;
    } catch {
      if (!observationEnded && Date.now() - started >= 20000) {
        observationEnded = true;
        record('OBSERVATION_TIMEOUT');
        console.error(JSON.stringify({ state: 'OBSERVATION_TIMEOUT', stage }));
      }
      await pause(1000);
    }
  }
  return false;
}

process.once('SIGTERM', () => void stop());
process.once('SIGINT', () => void stop());

try {
  // 镜像二进制复制到持久运行目录，保持退出证明所核验的绝对入口一致。
  // 配置/owner/资源不随镜像更新覆盖；完整镜像固定路径，不接受外部任意执行文件。
  for (const role of ['account', 'document', 'search'])
    copyFileSync(`/opt/collab/bin/${role}`, resolve(runtime, 'bin', role));
  const collaboration = resolve(root, 'services/collaboration/src/main.mjs');
  launch('collaboration', process.execPath, collaboration, [
    collaboration,
    '--config',
    resolve(runtime, 'services/collaboration.json'),
  ]);
  stage = 'COLLABORATION_BOOTSTRAP';
  const ca = readFileSync(resolve(runtime, 'pki/ca.pem'));
  if (
    await observe(() =>
      checkHTTPS(environment.ports.collaborationHttp, '/health/live', owner, { ca }),
    )
  ) {
    if (environment.developmentSMTP === true) {
      const smtp = resolve(root, 'scripts/smtp-receiver.mjs');
      launch('smtp', process.execPath, smtp, [smtp]);
    }
    for (const role of ['account', 'document', 'search']) {
      const executable = resolve(runtime, 'bin', role);
      launch(role, executable, executable, ['--config', resolve(runtime, `services/${role}.json`)]);
    }
    const edge = resolve(root, 'scripts/edge.mjs');
    launch('edge', process.execPath, edge, [edge]);
    stage = 'READY';
    observationEnded = false;
    if (await observe(() => checkServer(owner, environment))) {
      record('READY');
      console.log(JSON.stringify({ state: 'READY', url: environment.publicOrigin }));
    }
  }
} catch {
  console.error(JSON.stringify({ code: 'SERVER_START_FAILED', stage }));
  await stop(true);
}
