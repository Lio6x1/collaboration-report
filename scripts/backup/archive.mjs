import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { createReadStream, lstatSync } from 'node:fs';
import { stat } from 'node:fs/promises';
import { resolve } from 'node:path';
import { command } from '../runtime-lib.mjs';
import { archiveImage } from './inventory.mjs';

export async function fingerprint(path) {
  const hash = createHash('sha256');
  for await (const chunk of createReadStream(path)) hash.update(chunk);
  return { bytes: (await stat(path)).size, sha256: hash.digest('hex') };
}

export function archive(directory, owner, item, volume) {
  const file = item.key + '.tar.gz';
  // GNU tar在Linux内保留真实卷的UID/GID/权限与链接；宿主机复制Docker
  // 卷目录不能保证这些事实。只读挂载源，唯一可写挂载是新私密目录。
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
      '--label',
      'collaboration.report.owner=' + owner,
      '--mount',
      volume
        ? `type=volume,source=${item.name},target=/source,readonly`
        : `type=bind,source=${item.directory},target=/source,readonly`,
      '--mount',
      `type=bind,source=${directory},target=/backup`,
      archiveImage,
      'tar',
      '--numeric-owner',
      '-czf',
      '/backup/' + file,
      '-C',
      '/source',
      '--',
      ...(item.entries ?? ['.']),
    ],
    { timeout: 900000, failureCode: 'BACKUP_ARCHIVE_FAILED' },
  );
  return resolve(directory, file);
}

export async function inspectBackup(directory) {
  const { readFile } = await import('node:fs/promises');
  const manifest = JSON.parse(await readFile(resolve(directory, 'manifest.json'), 'utf8'));
  assert.equal(manifest.version, 1, 'BACKUP_VERSION_UNSUPPORTED');
  assert.equal(manifest.state, 'COMPLETE', 'BACKUP_NOT_COMPLETE');
  assert.match(manifest.commit, /^[a-f0-9]{40}$/);
  assert.equal(manifest.archives.length, 9, 'BACKUP_ARCHIVES_INCOMPLETE');
  const names = new Set();
  for (const item of manifest.archives) {
    assert.match(item.file, /^[a-z0-9-]+\.tar(?:\.gz)?$/);
    assert.ok(!names.has(item.file), 'BACKUP_ARCHIVE_DUPLICATE');
    names.add(item.file);
    assert.match(item.sha256, /^[a-f0-9]{64}$/);
    const path = resolve(directory, item.file);
    const type = lstatSync(path);
    assert.ok(type.isFile() && !type.isSymbolicLink(), 'BACKUP_ARCHIVE_FILE_UNSAFE');
    const actual = await fingerprint(path);
    assert.equal(actual.bytes, item.bytes, 'BACKUP_SIZE_MISMATCH');
    assert.equal(actual.sha256, item.sha256, 'BACKUP_DIGEST_MISMATCH');
  }
  const expected = [
    'mysql-data.tar.gz',
    'redis-data.tar.gz',
    'kafka-data.tar.gz',
    'es-data.tar.gz',
    'canal-position.tar.gz',
    'config.tar.gz',
    'resources.tar.gz',
    'exports.tar.gz',
    'source.tar',
  ];
  assert.deepEqual([...names].sort(), expected.sort(), 'BACKUP_ARCHIVE_SCOPE_MISMATCH');
  if (manifest.resourceSnapshot) {
    assert.equal(
      manifest.resourceSnapshot.mode,
      'OSS_LOCAL_SNAPSHOT',
      'BACKUP_RESOURCE_MODE_INVALID',
    );
    const resourceEntries = command('tar', ['-tf', resolve(directory, 'resources.tar.gz')]).split(
      /\r?\n/,
    );
    assert.ok(
      resourceEntries.includes('./resource-snapshot.json'),
      'BACKUP_RESOURCE_SNAPSHOT_MISSING',
    );
    assert.ok(
      !resourceEntries.some((name) => name === '.oss-binding' || name === './.oss-binding'),
      'BACKUP_RESOURCE_CLOUD_BINDING_RETAINED',
    );
  }
  const configEntries = command('tar', ['-tf', resolve(directory, 'config.tar.gz')]).split(/\r?\n/);
  assert.ok(configEntries.includes('edge-output-gates.json'), 'BACKUP_GATE_STATE_REQUIRED');
  return manifest;
}
