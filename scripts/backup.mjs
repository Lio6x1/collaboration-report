import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { command, compose, root, nodeArguments } from './runtime-lib.mjs';
import {
  inventory,
  privateTarget,
  assertStopped,
  assertNoLinks,
  assertOriginalProcessesExited,
} from './backup/inventory.mjs';
import { archive, fingerprint, inspectBackup } from './backup/archive.mjs';
import { restore } from './backup/restore.mjs';
import { prepareResourceSnapshot } from './backup/resources.mjs';

function argument(name) {
  const index = process.argv.indexOf(name);
  assert.ok(index >= 3 && process.argv[index + 1], 'BACKUP_ARGUMENT_REQUIRED');
  return process.argv[index + 1];
}
function protect(directory) {
  if (process.platform !== 'win32') throw Error('BACKUP_FIRST_RELEASE_REQUIRES_WINDOWS');
  const account = command('whoami', []);
  command('icacls', [directory, '/inheritance:r', '/grant:r', account + ':(OI)(CI)F']);
}

async function backup() {
  const destination = privateTarget(argument('--destination'), 'backups');
  const facts = inventory();
  mkdirSync(destination, { recursive: true });
  protect(destination);
  const manifest = {
    version: 1,
    state: 'PREPARING',
    owner: facts.owner,
    project: facts.project,
    sourceRoot: facts.sourceRoot,
    commit: command('git', ['rev-parse', 'HEAD']),
    dirty: Boolean(command('git', ['status', '--porcelain'])),
    startedAt: new Date().toISOString(),
    volumes: facts.volumes,
    hosts: facts.hosts,
    containers: facts.containers.map(({ service, image, wasRunning }) => ({
      service,
      image,
      wasRunning,
    })),
    archives: [],
  };
  const save = () =>
    writeFileSync(resolve(destination, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
  save();
  try {
    command(process.execPath, nodeArguments([resolve(root, 'scripts/start.mjs'), 'stop']));
    // 先冻结应用，再按消息生产者→broker/索引→数据库停止；原号与未知任务
    // 留在真实持久事实中。只有所有写者停止才建立冷备份边界。
    compose('stop', '-t', '30', 'canal');
    compose('stop', '-t', '30', 'kafka', 'es', 'redis');
    // 云快照的只读SQL清单必须在MySQL停止前取得，所有应用写者已经退出。
    // 失败停留在原事实上，绝不继续生成缺对象的 COMPLETE。
    manifest.resourceSnapshot = prepareResourceSnapshot(facts, destination);
    save();
    compose('stop', '-t', '30', 'mysql');
    assertOriginalProcessesExited(facts);
    assertStopped(facts);
    manifest.stoppedAt = new Date().toISOString();
    manifest.state = 'ARCHIVING';
    save();
    for (const item of [...facts.volumes, ...facts.hosts]) {
      assertStopped(facts);
      console.log(JSON.stringify({ state: 'ARCHIVING', scope: item.key }));
      const path = archive(destination, facts.owner, item, Boolean(item.name));
      manifest.archives.push({ file: item.key + '.tar.gz', ...(await fingerprint(path)) });
      save();
    }
    const source = resolve(destination, 'source.tar');
    command('git', ['archive', '--format=tar', '--output', source, manifest.commit]);
    manifest.archives.push({ file: 'source.tar', ...(await fingerprint(source)) });
    assertStopped(facts);
    manifest.state = 'COMPLETE';
    manifest.completedAt = new Date().toISOString();
    save();
    await inspectBackup(destination);
    console.log(JSON.stringify({ state: 'COMPLETE', archives: manifest.archives.length }));
  } catch (error) {
    manifest.state = 'FAILED';
    manifest.failure = /^[A-Z0-9_]+$/.test(error.message) ? error.message : 'BACKUP_FAILED';
    save();
    throw error;
  }
  // 不在finally里启动：备份成功后保持明确停止状态，恢复演练/运维可决定
  // 下一步；失败也保留现场，不能悄悄启动写者并改写所需诊断事实。
}

try {
  if (process.argv[2] === 'backup') await backup();
  else if (process.argv[2] === 'inspect') {
    const source = resolve(argument('--source'));
    assertNoLinks(source);
    const manifest = await inspectBackup(source);
    console.log(
      JSON.stringify({
        state: 'VERIFIED',
        commit: manifest.commit,
        archives: manifest.archives.length,
      }),
    );
  } else if (process.argv[2] === 'restore') {
    const result = await restore(resolve(argument('--source')), argument('--target'));
    console.log(
      JSON.stringify({ state: result.state, target: result.target, project: result.project }),
    );
  } else throw Error('USAGE_BACKUP_INSPECT_OR_RESTORE');
} catch (error) {
  console.error(
    JSON.stringify({
      state: 'FAILED',
      code: /^[A-Z0-9_]+$/.test(error.message) ? error.message : 'BACKUP_COMMAND_FAILED',
    }),
  );
  process.exitCode = 1;
}
