import assert from 'node:assert/strict';
import { cpSync, existsSync, mkdirSync, readFileSync, lstatSync } from 'node:fs';
import { basename, resolve } from 'node:path';
import { command, root, runtime } from '../runtime-lib.mjs';
import { assertNoLinks, assertOriginalProcessesExited } from './inventory.mjs';
import { fingerprint } from './archive.mjs';

// 仅在全部原应用已真实退出、Canal停止且MySQL仍在时调用。云对象逐键
// 完整校验后保存为本地不可变快照，恢复副本不需要也不会写入原Bucket。
export function prepareResourceSnapshot(facts, destination) {
  if (!facts.resourceBackend) return null;
  assertOriginalProcessesExited(facts);
  const item = facts.hosts.find((host) => host.key === 'resources');
  assert.ok(item, 'BACKUP_RESOURCE_SCOPE_MISSING');
  const directory = resolve(destination, 'resource-staging');
  assert.ok(!existsSync(directory), 'BACKUP_RESOURCE_STAGING_EXISTS');
  assertNoLinks(directory);
  // 保留原本地对象、锁和残留临时文件；新快照不继承云地址绑定，防止隔离
  // 实例清理原环境对象。原目录不删除、不覆盖，历史快照说明另行重建。
  cpSync(item.directory, directory, {
    recursive: true,
    dereference: false,
    errorOnExist: true,
    force: false,
    filter: (source) => !['.oss-binding', 'resource-snapshot.json'].includes(basename(source)),
  });
  const executable = resolve(runtime, 'bin', 'resourcesnapshot.exe');
  mkdirSync(resolve(runtime, 'bin'), { recursive: true });
  command(
    'go',
    [
      '-C',
      resolve(root, 'services/document'),
      'build',
      '-mod=readonly',
      '-o',
      executable,
      './cmd/resourcesnapshot',
    ],
    {
      timeout: 300000,
      env: { ...process.env, GOWORK: 'off', GOCACHE: resolve(runtime, 'go-cache') },
      failureCode: 'BACKUP_RESOURCE_TOOL_BUILD_FAILED',
    },
  );
  const result = JSON.parse(
    command(
      executable,
      ['-config', resolve(runtime, 'services/document.json'), '-target', directory],
      { timeout: 960000, failureCode: 'BACKUP_RESOURCE_SNAPSHOT_FAILED' },
    ),
  );
  assert.equal(result.state, 'COMPLETE', 'BACKUP_RESOURCE_SNAPSHOT_INCOMPLETE');
  const manifest = JSON.parse(readFileSync(resolve(directory, 'resource-snapshot.json')));
  assert.equal(manifest.version, 1);
  assert.equal(manifest.state, 'COMPLETE');
  assert.equal(manifest.objects.length, result.objects);
  assert.equal(manifest.missingReady, result.missingReady);
  item.directory = directory;
  return {
    mode: 'OSS_LOCAL_SNAPSHOT',
    provider: facts.resourceBackend,
    objects: result.objects,
    missingReady: result.missingReady,
  };
}

export function restoreResourceConfiguration(config, manifest) {
  if (!manifest.resourceSnapshot) {
    assert.ok(!config.resources?.oss, 'RESTORE_OSS_SNAPSHOT_REQUIRED');
    return;
  }
  assert.equal(
    manifest.resourceSnapshot.mode,
    'OSS_LOCAL_SNAPSHOT',
    'RESTORE_RESOURCE_MODE_INVALID',
  );
  assert.ok(config.resources?.oss, 'RESTORE_OSS_SOURCE_CONFIG_REQUIRED');
  const source = manifest.resourceSnapshot.provider;
  for (const key of ['region', 'endpoint', 'bucket', 'prefix'])
    assert.equal(config.resources.oss[key], source[key], 'RESTORE_OSS_SOURCE_BINDING_INVALID');
  // 还原已经校验的本地副本，不携带原Bucket的可写能力/密钥路径。
  delete config.resources.oss;
}

export async function verifyResourceSnapshotDirectory(directory, backupManifest) {
  if (!backupManifest.resourceSnapshot) return;
  assertNoLinks(directory);
  assert.ok(!existsSync(resolve(directory, '.oss-binding')), 'RESTORE_CLOUD_BINDING_RETAINED');
  const file = resolve(directory, 'resource-snapshot.json');
  const info = lstatSync(file);
  assert.ok(info.isFile() && !info.isSymbolicLink(), 'RESTORE_RESOURCE_MANIFEST_UNSAFE');
  const manifest = JSON.parse(readFileSync(file));
  assert.equal(manifest.version, 1, 'RESTORE_RESOURCE_VERSION_INVALID');
  assert.equal(manifest.state, 'COMPLETE', 'RESTORE_RESOURCE_SNAPSHOT_INCOMPLETE');
  assert.equal(manifest.objects.length, backupManifest.resourceSnapshot.objects);
  const seen = new Set();
  let missingReady = 0;
  for (const item of manifest.objects) {
    if (item.metadataOnly === true) {
      assert.match(item.key, /^[A-Za-z0-9_-]{1,160}$/);
      assert.equal(item.present, false, 'RESTORE_METADATA_ONLY_MUST_BE_ABSENT');
      assert.ok(item.sha256 === '' || /^[a-f0-9]{64}$/.test(item.sha256));
    } else {
      assert.match(item.key, /^[a-f0-9]{64}$/);
      assert.match(item.sha256, /^[a-f0-9]{64}$/);
    }
    assert.ok(!seen.has(item.key), 'RESTORE_RESOURCE_DUPLICATE_KEY');
    seen.add(item.key);
    assert.ok(
      Number.isSafeInteger(item.size) && item.size >= 0 && item.size <= 32 << 20,
      'RESTORE_RESOURCE_SIZE_INVALID',
    );
    assert.ok(['READY', 'UPLOADING', 'DELETING'].includes(item.sourceState));
    assert.equal(typeof item.present, 'boolean');
    const path = resolve(directory, item.key);
    if (!item.present) {
      assert.ok(!existsSync(path), 'RESTORE_RESOURCE_ABSENCE_CHANGED');
      if (item.sourceState === 'READY') missingReady++;
      continue;
    }
    const object = lstatSync(path);
    assert.ok(object.isFile() && !object.isSymbolicLink(), 'RESTORE_RESOURCE_FILE_UNSAFE');
    const actual = await fingerprint(path);
    assert.equal(actual.bytes, item.size, 'RESTORE_RESOURCE_SIZE_CHANGED');
    assert.equal(actual.sha256, item.sha256, 'RESTORE_RESOURCE_DIGEST_CHANGED');
  }
  assert.equal(missingReady, manifest.missingReady, 'RESTORE_RESOURCE_MISSING_COUNT_CHANGED');
  assert.equal(missingReady, backupManifest.resourceSnapshot.missingReady);
}
