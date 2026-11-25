import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve, relative, isAbsolute, sep } from 'node:path';
import net from 'node:net';
import { command, docker } from '../runtime-lib.mjs';
import { archiveImage, assertNoLinks, privateTarget } from './inventory.mjs';
import { inspectBackup } from './archive.mjs';
import { composeEnvironment } from '../compose-environment.mjs';
import { restoreResourceConfiguration, verifyResourceSnapshotDirectory } from './resources.mjs';

function safeListing(archive) {
  const names = command('tar', ['-tf', archive]).split(/\r?\n/).filter(Boolean);
  assert.ok(names.length > 0, 'RESTORE_ARCHIVE_EMPTY');
  for (const name of names) {
    const normalized = name.replaceAll('\\', '/');
    assert.ok(
      !normalized.startsWith('/') &&
        !/^[A-Za-z]:/.test(normalized) &&
        !normalized.split('/').includes('..'),
      'RESTORE_ARCHIVE_PATH_UNSAFE',
    );
  }
  return names;
}

async function freePort(port) {
  const server = net.createServer();
  await new Promise((done, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', done);
  });
  await new Promise((done) => server.close(done));
  return port;
}

export async function allocateRestorePorts(oldPorts) {
  // listen(0)选择的是临时端口；数据库连接/启动探针可能在服务绑定前
  // 抢到同一个端口。本机动态范围也可能被用户改过，不能只猜49152。
  const ranges = ['ipv4', 'ipv6'].map((family) => {
    const output = command('netsh', ['int', family, 'show', 'dynamicport', 'tcp']);
    const values = [...output.matchAll(/:\s*(\d+)/g)].map((match) => Number(match[1]));
    assert.equal(values.length, 2, 'RESTORE_DYNAMIC_PORT_RANGE_UNAVAILABLE');
    return { first: values[0], last: values[0] + values[1] - 1 };
  });
  const reserved = new Set(Object.values(oldPorts)),
    ports = {};
  let candidate = 21000;
  for (const key of Object.keys(oldPorts)) {
    let selected = false;
    for (; candidate <= 49000; candidate++) {
      if (
        reserved.has(candidate) ||
        ranges.some((range) => candidate >= range.first && candidate <= range.last)
      )
        continue;
      try {
        ports[key] = await freePort(candidate);
        selected = true;
        candidate++;
        break;
      } catch (error) {
        if (!['EADDRINUSE', 'EACCES'].includes(error.code)) throw error;
      }
    }
    assert.ok(selected, 'RESTORE_SERVICE_PORTS_UNAVAILABLE');
  }
  return ports;
}

export function rebase(value, sourceRoot, target, oldPorts, ports) {
  if (Array.isArray(value))
    return value.map((item) => rebase(item, sourceRoot, target, oldPorts, ports));
  if (value && typeof value === 'object')
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [
        key,
        rebase(item, sourceRoot, target, oldPorts, ports),
      ]),
    );
  if (typeof value === 'number') {
    const key = Object.keys(oldPorts).find((key) => oldPorts[key] === value);
    return key ? ports[key] : value;
  }
  if (typeof value !== 'string') return value;
  let result = value;
  if (isAbsolute(value)) {
    const offset = relative(sourceRoot, value);
    if (!offset.startsWith('..' + sep) && offset !== '..' && !isAbsolute(offset))
      result = resolve(target, offset);
  }
  for (const key of Object.keys(oldPorts)) {
    result = result.replaceAll('127.0.0.1:' + oldPorts[key], '127.0.0.1:' + ports[key]);
    result = result.replaceAll('localhost:' + oldPorts[key], 'localhost:' + ports[key]);
  }
  return result;
}

export async function restore(source, targetArgument) {
  assertNoLinks(source);
  const manifest = await inspectBackup(source);
  const target = privateTarget(targetArgument, 'restores');
  const configEntries = safeListing(resolve(source, 'config.tar.gz'));
  assert.ok(configEntries.includes('edge-output-gates.json'), 'BACKUP_GATE_STATE_REQUIRED');
  safeListing(resolve(source, 'source.tar'));
  const expectedKeys = ['mysql-data', 'redis-data', 'kafka-data', 'es-data', 'canal-position'];
  assert.deepEqual(
    manifest.volumes.map((item) => item.key).sort(),
    [...expectedKeys].sort(),
    'RESTORE_VOLUME_SCOPE_MISMATCH',
  );
  // 全部摘要与路径检查通过后才创建目标。原source卷名不会传入写挂载，
  // 每个恢复卷使用新的project/owner，恢复失败也保留隔离现场，不删除原卷。
  mkdirSync(target, { recursive: true });
  const account = command('whoami', []);
  command('icacls', [target, '/inheritance:r', '/grant:r', account + ':(OI)(CI)F']);
  command('tar', ['-xf', resolve(source, 'source.tar'), '-C', target]);
  const localRuntime = resolve(target, '.runtime');
  mkdirSync(localRuntime);
  command('tar', ['-xf', resolve(source, 'config.tar.gz'), '-C', localRuntime]);
  for (const key of ['resources', 'exports']) {
    const directory = resolve(localRuntime, key, 'document');
    mkdirSync(directory, { recursive: true });
    safeListing(resolve(source, key + '.tar.gz'));
    command('tar', ['-xf', resolve(source, key + '.tar.gz'), '-C', directory], { timeout: 300000 });
    if (key === 'resources') await verifyResourceSnapshotDirectory(directory, manifest);
  }
  const environment = JSON.parse(readFileSync(resolve(localRuntime, 'dev.json')));
  const oldPorts = environment.ports,
    ports = await allocateRestorePorts(oldPorts);
  const suffix = randomUUID().replaceAll('-', '');
  environment.owner = 'collaboration-report-restore-' + suffix;
  environment.project = 'collabrestore' + suffix;
  environment.ports = ports;
  writeFileSync(resolve(localRuntime, 'dev.json'), JSON.stringify(environment, null, 2) + '\n');
  writeFileSync(resolve(localRuntime, 'compose.env'), composeEnvironment(environment));
  for (const role of ['account', 'document', 'search', 'collaboration']) {
    const path = resolve(localRuntime, 'services', role + '.json');
    const config = rebase(
      JSON.parse(readFileSync(path)),
      manifest.sourceRoot,
      target,
      oldPorts,
      ports,
    );
    config.root = target;
    if (config.resources) config.resources.root = resolve(localRuntime, 'resources/document');
    if (role === 'document') restoreResourceConfiguration(config, manifest);
    if (config.exports) config.exports.root = resolve(localRuntime, 'exports/document');
    // 索引owner/消费group是已备份数据的持久身份，保留原值。新owner只
    // 标记隔离部署的容器/卷；改索引元数据或跳过校验都会破坏来源证明。
    writeFileSync(path, JSON.stringify(config, null, 2) + '\n');
  }
  writeFileSync(resolve(localRuntime, 'processes.json'), '[]\n');
  writeFileSync(resolve(localRuntime, 'startup.json'), '{"state":"STOPPED"}\n');
  const result = {
    state: 'RESTORING',
    sourceCommit: manifest.commit,
    target,
    owner: environment.owner,
    project: environment.project,
    ports,
    volumes: [],
  };
  const save = () =>
    writeFileSync(resolve(localRuntime, 'restore.json'), JSON.stringify(result, null, 2) + '\n');
  save();
  const existing = new Set(docker('volume', 'ls', '--format', '{{.Name}}').split(/\r?\n/));
  try {
    for (const key of expectedKeys) {
      const name = environment.project + '_' + key;
      assert.ok(!existing.has(name), 'RESTORE_TARGET_VOLUME_EXISTS');
      docker(
        'volume',
        'create',
        '--label',
        'collaboration.report.owner=' + environment.owner,
        '--label',
        'com.docker.compose.project=' + environment.project,
        '--label',
        'com.docker.compose.volume=' + key,
        name,
      );
      const volume = JSON.parse(docker('volume', 'inspect', name))[0];
      assert.equal(volume.Labels['collaboration.report.owner'], environment.owner);
      assert.equal(docker('ps', '-q', '--filter', 'volume=' + name), '', 'RESTORE_VOLUME_IN_USE');
      command(
        'docker',
        [
          'run',
          '--rm',
          '--network',
          'none',
          '--read-only',
          '--memory',
          '256m',
          '--cap-drop',
          'ALL',
          '--cap-add',
          'DAC_OVERRIDE',
          '--cap-add',
          'CHOWN',
          '--cap-add',
          'FOWNER',
          '--label',
          'collaboration.report.owner=' + environment.owner,
          '--mount',
          `type=bind,source=${source},target=/backup,readonly`,
          '--mount',
          `type=volume,source=${name},target=/target`,
          archiveImage,
          'tar',
          '--numeric-owner',
          '-xzf',
          '/backup/' + key + '.tar.gz',
          '-C',
          '/target',
        ],
        { timeout: 900000, failureCode: 'RESTORE_VOLUME_EXTRACTION_FAILED' },
      );
      result.volumes.push({ key, name });
      save();
      console.log(JSON.stringify({ state: 'RESTORING', scope: key }));
    }
    result.state = 'RESTORED_STOPPED';
    result.completedAt = new Date().toISOString();
    save();
    return result;
  } catch (error) {
    result.state = 'FAILED';
    save();
    throw error;
  }
}
