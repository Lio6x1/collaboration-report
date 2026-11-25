import assert from 'node:assert/strict';
import { existsSync, lstatSync, readFileSync } from 'node:fs';
import { isAbsolute, relative, resolve, sep } from 'node:path';
import { command, compose, docker, root, runtime, state } from '../runtime-lib.mjs';
import { assertContainerOwner } from '../ownership.mjs';

export const archiveImage =
  'golang@sha256:154bd7001b6eb339e88c964442c0ad6ed5e53f09844cc818a41ce4ecb3ce3b43';
const expectedVolumes = {
  mysql: ['mysql-data', '/var/lib/mysql'],
  redis: ['redis-data', '/data'],
  kafka: ['kafka-data', '/tmp/kraft-combined-logs'],
  es: ['es-data', '/usr/share/elasticsearch/data'],
  canal: ['canal-position', '/home/admin/canal-server/meta'],
};

// 私密制品只放忽略目录；拒绝已有目录、链接和跨根路径，防止备份覆盖
// 原环境或把密码/证书落入源码。这里只限制本项目第一版单机恢复入口。
export function privateTarget(path, family) {
  assert.ok(isAbsolute(path), 'BACKUP_ABSOLUTE_PATH_REQUIRED');
  const target = resolve(path),
    base = resolve(runtime, family);
  const offset = relative(base, target);
  assert.ok(
    offset && !offset.startsWith('..' + sep) && offset !== '..' && !isAbsolute(offset),
    'BACKUP_TARGET_OUTSIDE_PRIVATE_ROOT',
  );
  assert.ok(!existsSync(target), 'BACKUP_TARGET_ALREADY_EXISTS');
  assertNoLinks(target);
  return target;
}

export function assertNoLinks(path) {
  for (let part = resolve(path); ; part = resolve(part, '..')) {
    if (existsSync(part))
      assert.ok(
        lstatSync(part).isDirectory() && !lstatSync(part).isSymbolicLink(),
        'BACKUP_DIRECTORY_UNSAFE',
      );
    if (resolve(part, '..') === part) break;
  }
}

export function inventory() {
  const environment = state(),
    containers = [],
    volumes = [];
  for (const id of compose('ps', '--all', '-q').split(/\s+/).filter(Boolean)) {
    const item = JSON.parse(docker('inspect', id))[0],
      labels = item.Config.Labels;
    assertContainerOwner(
      {
        owner: labels['collaboration.report.owner'],
        project: labels['com.docker.compose.project'],
      },
      environment,
    );
    const service = labels['com.docker.compose.service'];
    containers.push({ id, service, image: item.Config.Image, wasRunning: item.State.Running });
    if (!(service in expectedVolumes)) continue;
    const [key, destination] = expectedVolumes[service];
    const mount = item.Mounts.find(
      (value) => value.Type === 'volume' && value.Destination === destination,
    );
    assert.ok(
      mount && mount.Name === environment.project + '_' + key,
      'BACKUP_VOLUME_LAYOUT_MISMATCH',
    );
    const volume = JSON.parse(docker('volume', 'inspect', mount.Name))[0];
    assert.equal(
      volume.Labels['collaboration.report.owner'],
      environment.owner,
      'BACKUP_VOLUME_OWNER_MISMATCH',
    );
    assert.equal(
      volume.Labels['com.docker.compose.project'],
      environment.project,
      'BACKUP_VOLUME_PROJECT_MISMATCH',
    );
    volumes.push({ service, key, name: mount.Name, destination });
  }
  assert.equal(volumes.length, 5, 'BACKUP_FULL_ENVIRONMENT_REQUIRED');
  const document = JSON.parse(readFileSync(resolve(runtime, 'services/document.json')));
  const hosts = [
    {
      key: 'config',
      directory: runtime,
      entries: ['dev.json', 'compose.env', 'services', 'pki', 'redis', 'edge-output-gates.json'],
    },
    {
      key: 'resources',
      directory: document.resources?.root ?? resolve(root, '.runtime/resources/document'),
      entries: ['.'],
    },
    {
      key: 'exports',
      directory: document.exports?.root ?? resolve(root, '.runtime/exports/document'),
      entries: ['.'],
    },
  ];
  for (const host of hosts) {
    assert.ok(isAbsolute(host.directory), 'BACKUP_DATA_PATH_NOT_ABSOLUTE');
    assertNoLinks(host.directory);
    assert.ok(existsSync(host.directory), 'BACKUP_DATA_DIRECTORY_MISSING');
    for (const entry of host.entries)
      assert.ok(existsSync(resolve(host.directory, entry)), 'BACKUP_CONFIG_MISSING');
  }
  const processes = JSON.parse(readFileSync(resolve(runtime, 'processes.json')));
  return {
    owner: environment.owner,
    project: environment.project,
    sourceRoot: root,
    containers,
    volumes,
    hosts,
    processes,
    resourceBackend: document.resources?.oss
      ? Object.fromEntries(
          ['region', 'endpoint', 'bucket', 'prefix'].map((key) => [
            key,
            document.resources.oss[key],
          ]),
        )
      : null,
  };
}

export function assertOriginalProcessesExited(facts) {
  // 清空登记不等于系统已经完成终止。建立冷边界前按原PID、入口/owner
  // 复核；PID被重用的无关进程不当作原写者，也不终止它。
  for (const item of facts.processes) {
    const actual = JSON.parse(
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
        { failureCode: 'BACKUP_PROCESS_INSPECTION_FAILED' },
      ),
    );
    assert.ok(
      actual.state === 'ABSENT' ||
        !actual.commandLine.includes(item.entry) ||
        !actual.commandLine.includes('--run-owner ' + item.owner),
      'BACKUP_ORIGINAL_PROCESS_STILL_RUNNING',
    );
  }
}

export function assertStopped(facts) {
  for (const item of facts.containers) {
    const current = JSON.parse(docker('inspect', item.id))[0];
    assert.equal(current.Config.Labels['collaboration.report.owner'], facts.owner);
    assert.equal(current.State.Running, false, 'BACKUP_CONTAINER_STILL_RUNNING');
    if (['mysql', 'redis'].includes(item.service))
      assert.equal(current.State.ExitCode, 0, 'BACKUP_DATABASE_NOT_CLEANLY_STOPPED');
  }
  const processes = JSON.parse(readFileSync(resolve(runtime, 'processes.json')));
  assert.equal(processes.length, 0, 'BACKUP_APPLICATIONS_NOT_STOPPED');
  for (const volume of facts.volumes) {
    assert.equal(
      docker('ps', '-q', '--filter', 'volume=' + volume.name),
      '',
      'BACKUP_VOLUME_HAS_RUNNING_WRITER',
    );
  }
}
