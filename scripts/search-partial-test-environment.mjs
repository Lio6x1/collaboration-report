import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { execFile, spawn } from 'node:child_process';
import { promisify } from 'node:util';
import { createServer } from 'node:net';
import { mkdir, readFile, writeFile, access } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { assertContainerOwner } from './ownership.mjs';

const root = fileURLToPath(new URL('../', import.meta.url));
const execute = promisify(execFile);
const delay = (ms) => new Promise((done) => setTimeout(done, ms));
const events = [];
const stage = (name) => {
  const event = JSON.stringify({ stage: name });
  events.push(event);
  console.log(event);
};
const composeFile = resolve(root, 'deploy/compose/search-partial-test.yml');
const fixturePattern = /^[a-f0-9]{32}$/;
const ownerPattern = /^[A-Za-z0-9_.-]{1,64}$/;

function validateIdentity(identity) {
  if (
    typeof identity.fixture !== 'string' ||
    typeof identity.owner !== 'string' ||
    !fixturePattern.test(identity.fixture) ||
    !ownerPattern.test(identity.owner) ||
    identity.project !== `collab-search-partial-${identity.fixture}`
  )
    throw Error('PARTIAL_IDENTITY_INVALID');
}

function composeArguments(identity, environment, args) {
  validateIdentity(identity);
  // 明确使用隔离文件和精确project，不使用runtime-lib的产品compose()/compose.env。
  return ['compose', '--env-file', environment, '-p', identity.project, '-f', composeFile, ...args];
}

function validateContainer(labels, identity, service) {
  validateIdentity(identity);
  if (service !== 'partial-a' && service !== 'partial-b') throw Error('PARTIAL_SERVICE_INVALID');
  assertContainerOwner(
    { owner: labels['collaboration.report.owner'], project: labels['com.docker.compose.project'] },
    identity,
  );
  if (
    labels['collaboration.report.partial-fixture'] !== identity.fixture ||
    labels['com.docker.compose.service'] !== service
  )
    throw Error('PARTIAL_CONTAINER_IDENTITY_INVALID');
}

function checkControlRules() {
  const fixture = '0123456789abcdef0123456789abcdef';
  const identity = { fixture, owner: 'test-owner', project: `collab-search-partial-${fixture}` };
  const args = composeArguments(identity, 'isolated.env', ['up', '-d']);
  assert.equal(args[args.indexOf('-p') + 1], identity.project);
  assert.equal(args[args.indexOf('-f') + 1], composeFile);
  assert.equal(args.includes('deploy/compose/search.yml'), false);
  assert.throws(
    () => composeArguments({ ...identity, project: 'production' }, 'isolated.env', ['down']),
    /PARTIAL_IDENTITY_INVALID/,
  );
  const labels = {
    'collaboration.report.owner': identity.owner,
    'com.docker.compose.project': identity.project,
    'collaboration.report.partial-fixture': fixture,
    'com.docker.compose.service': 'partial-b',
  };
  validateContainer(labels, identity, 'partial-b');
  for (const key of Object.keys(labels)) {
    assert.throws(() => validateContainer({ ...labels, [key]: 'wrong' }, identity, 'partial-b'));
  }
  assert.throws(() => validateContainer(labels, identity, 'es'), /PARTIAL_SERVICE_INVALID/);
  stage('PARTIAL_CONTROL_RULES_PASS_NO_DOCKER');
}

async function docker(args, timeout = 30000) {
  try {
    const result = await execute('docker', args, {
      cwd: root,
      windowsHide: true,
      timeout,
      maxBuffer: 1 << 20,
    });
    return result.stdout.trim();
  } catch {
    // 不泄漏Docker错误原文、环境或潜在命令参数，只记录固定阶段错误。
    throw Error('PARTIAL_DOCKER_COMMAND_FAILED');
  }
}

async function freePort() {
  const server = createServer();
  await new Promise((done, failed) => {
    server.once('error', failed);
    server.listen(0, '127.0.0.1', done);
  });
  const port = server.address().port;
  await new Promise((done, failed) => server.close((error) => (error ? failed(error) : done())));
  return port;
}

async function exists(file) {
  try {
    await access(file);
    return true;
  } catch {
    return false;
  }
}

async function clusterReady(endpoint, budget = 90000) {
  const until = Date.now() + budget;
  while (Date.now() < until) {
    try {
      const response = await fetch(
        `${endpoint}/_cluster/health?wait_for_nodes=2&wait_for_status=yellow&timeout=500ms`,
        { signal: AbortSignal.timeout(2000) },
      );
      const health = await response.json();
      const versionResponse = await fetch(endpoint, { signal: AbortSignal.timeout(2000) });
      const version = await versionResponse.json();
      if (
        response.ok &&
        !health.timed_out &&
        health.number_of_nodes === 2 &&
        ['green', 'yellow'].includes(health.status) &&
        versionResponse.ok &&
        version.version?.number === '8.19.4'
      )
        return;
    } catch {}
    await delay(100);
  }
  throw Error('PARTIAL_CLUSTER_RESTORE_OR_START_BUDGET_EXHAUSTED');
}

async function run() {
  // 只读取本项目公开owner状态，不读取任一私有service配置/DSN，不写产品state。
  const current = JSON.parse(await readFile(resolve(root, '.runtime/dev.json'), 'utf8'));
  const fixture = randomBytes(16).toString('hex');
  const identity = { fixture, owner: current.owner, project: `collab-search-partial-${fixture}` };
  validateIdentity(identity);
  if (identity.project === current.project) throw Error('PARTIAL_PROJECT_MUST_BE_ISOLATED');
  const directory = resolve(root, '.runtime/search-partial-test', fixture);
  const environment = resolve(directory, 'topology.env');
  const port = await freePort();
  const endpoint = `http://127.0.0.1:${port}`;
  await mkdir(directory, { recursive: true });
  await writeFile(
    environment,
    `COLLAB_PARTIAL_OWNER=${identity.owner}\nCOLLAB_PARTIAL_FIXTURE=${fixture}\nCOLLAB_PARTIAL_PORT=${port}\n`,
  );
  await writeFile(
    resolve(directory, 'topology.json'),
    JSON.stringify({ version: 1, ...identity, endpoint }) + '\n',
  );
  const compose = (args, timeout) => docker(composeArguments(identity, environment, args), timeout);
  let child;
  let childDone = false;
  let failed = false;
  let started = false;
  let restored = false;
  let output = '';
  let faultApplied = false;
  let restoreApplied = false;
  async function owned(service) {
    const id = await compose(['ps', '--all', '-q', service]);
    if (!/^[a-f0-9]{12,64}$/.test(id)) throw Error('PARTIAL_OWNED_CONTAINER_MISSING');
    const labels = JSON.parse(await docker(['inspect', '--format', '{{json .Config.Labels}}', id]));
    validateContainer(labels, identity, service);
    return id;
  }
  async function restore() {
    // 无论测试何处失败，都重新核验精确B后恢复；绝不启动/停止原es服务。
    const id = await owned('partial-b');
    await docker(['start', id]);
    await clusterReady(endpoint);
    restored = true;
    await writeFile(resolve(directory, 'restore.applied'), '1\n');
    stage('PARTIAL_EXACT_B_RESTORED');
  }
  try {
    started = true;
    // --pull never要求主控先核验/准备固定镜像，不在故障测试中隐式下载其它镜像。
    await compose(['up', '-d', '--pull', 'never'], 60000);
    await owned('partial-a');
    await owned('partial-b');
    await clusterReady(endpoint);
    stage('PARTIAL_TWO_NODE_FIXTURE_READY');
    child = spawn(
      'D:/go1.25.10/bin/go.exe',
      [
        'test',
        '-mod=readonly',
        '-tags=search_partial',
        './internal/client/elastic',
        '-run',
        '^TestActualPITRejectsRealPartialShardTopology$',
        '-count=1',
        '-v',
        '-timeout=300s',
      ],
      {
        cwd: resolve(root, 'services/search'),
        windowsHide: true,
        env: {
          ...process.env,
          GOWORK: 'off',
          GOCACHE: resolve(root, '.runtime/go-cache'),
          COLLAB_TEST_SEARCH_PARTIAL_DIRECTORY: directory,
        },
        stdio: ['ignore', 'pipe', 'pipe'],
      },
    );
    const append = (chunk) => {
      if (output.length + chunk.length > 1 << 20) {
        child.kill();
        failed = true;
        return;
      }
      output += chunk.toString();
      process.stdout.write(chunk);
    };
    child.stdout.on('data', append);
    child.stderr.on('data', append);
    const completion = new Promise((done) => {
      child.once('error', () => {
        childDone = true;
        failed = true;
        done(-1);
      });
      child.once('close', (code) => {
        childDone = true;
        done(code);
      });
    });
    const until = Date.now() + 310000;
    while (!childDone) {
      if (Date.now() >= until) throw Error('PARTIAL_TEST_PROCESS_BUDGET_EXHAUSTED');
      if (!faultApplied && (await exists(resolve(directory, 'fault.request')))) {
        const id = await owned('partial-b');
        await docker(['stop', '--time', '2', id], 10000);
        faultApplied = true;
        restored = false;
        await writeFile(resolve(directory, 'fault.applied'), '1\n');
        stage('PARTIAL_EXACT_B_STOPPED');
      }
      if (!restoreApplied && (await exists(resolve(directory, 'restore.request')))) {
        await restore();
        restoreApplied = true;
      }
      await delay(25);
    }
    if ((await completion) !== 0 || failed || !faultApplied || !restoreApplied)
      throw Error('PARTIAL_ACTUAL_TEST_FAILED');
  } catch (error) {
    failed = true;
    stage(error.message.startsWith('PARTIAL_') ? error.message : 'PARTIAL_HARNESS_FAILED');
    await writeFile(resolve(directory, 'controller.failed'), '1\n');
  } finally {
    if (child && !childDone) {
      child.kill();
      const until = Date.now() + 5000;
      while (!childDone && Date.now() < until) await delay(25);
      if (!childDone) {
        failed = true;
        stage('PARTIAL_TEST_PROCESS_EXIT_UNKNOWN');
      }
    }
    if (started) {
      try {
        await restore();
      } catch {
        restored = false;
        failed = true;
        stage('PARTIAL_FINAL_RESTORE_FAILED_RESOURCES_RETAINED');
      }
      if (restored && (!child || childDone)) {
        try {
          await owned('partial-a');
          await owned('partial-b');
          await compose(['down', '--volumes'], 60000);
          stage('PARTIAL_EXACT_PROJECT_REMOVED');
        } catch {
          failed = true;
          stage('PARTIAL_CLEANUP_FAILED_RESOURCES_RETAINED');
        }
      }
    }
    // Go仅输出固定错误/计数，不输出PIT、正文或配置。失败证据也保留，不能覆写为PASS。
    const evidenceDirectory = resolve(root, 'artifacts/batches/06');
    await mkdir(evidenceDirectory, { recursive: true });
    const evidence = resolve(evidenceDirectory, `search-partial-shards-actual-${fixture}.txt`);
    await writeFile(
      evidence,
      output +
        '\n' +
        events.join('\n') +
        `\nharness_result=${failed ? 'FAIL' : 'PASS'}\nrestored=${restored}\nproject=${identity.project}\nfixture=${fixture}\n`,
    );
    console.log(
      JSON.stringify({
        stage: 'PARTIAL_EVIDENCE_SAVED',
        evidence,
        project: identity.project,
        fixture,
      }),
    );
  }
  if (failed) process.exitCode = 1;
}

if (process.argv.length === 3 && process.argv[2] === '--check') checkControlRules();
else if (process.argv.length === 3 && process.argv[2] === '--run') {
  await run().catch(() => {
    stage('PARTIAL_HARNESS_FAILED');
    process.exitCode = 1;
  });
} else {
  console.error('Usage: node scripts/search-partial-test-environment.mjs --check|--run');
  process.exitCode = 2;
}
