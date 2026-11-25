import assert from 'node:assert/strict';
import test from 'node:test';
import {
  restoreResourceConfiguration,
  verifyResourceSnapshotDirectory,
} from '../../scripts/backup/resources.mjs';
import { createHash } from 'node:crypto';
import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

// 配置转换决定隔离副本是否能误删原Bucket；不执行云网络，也不冒称恢复演练。
test('OSS snapshot restoration removes original cloud write ability while preserving resource limits', () => {
  const source = {
    region: 'cn-shenzhen',
    endpoint: 'https://oss-cn-shenzhen.aliyuncs.com',
    bucket: 'collaboration-report',
    prefix: 'collabreport-tests/dev-fc4e4e3b/',
  };
  const config = {
    resources: {
      root: 'D:/private/restore/resources',
      spaceBytes: 1073741824,
      uploadBytes: 33554432,
      temporaryBytes: 268435456,
      uploadTTLSeconds: 3600,
      oss: { ...source, credentialsFile: 'D:/original/private/credentials.json' },
    },
  };
  assert.throws(
    () => restoreResourceConfiguration(structuredClone(config), {}),
    /RESTORE_OSS_SNAPSHOT_REQUIRED/,
  );
  assert.throws(
    () =>
      restoreResourceConfiguration(structuredClone(config), {
        resourceSnapshot: { mode: 'OSS_LOCAL_SNAPSHOT', provider: { ...source, prefix: 'other/' } },
      }),
    /RESTORE_OSS_SOURCE_BINDING_INVALID/,
  );
  const expected = structuredClone(config.resources);
  delete expected.oss;
  restoreResourceConfiguration(config, {
    resourceSnapshot: { mode: 'OSS_LOCAL_SNAPSHOT', provider: source },
  });
  assert.deepEqual(config.resources, expected);
  assert.doesNotThrow(() => restoreResourceConfiguration({ resources: expected }, {}));
});

test('restoration verifies actual snapshot object bytes and original absence before enabling applications', async () => {
  const directory = mkdtempSync(join(tmpdir(), 'collabreport-oss-snapshot-'));
  const body = Buffer.from('完整云对象快照'.repeat(100));
  const sha256 = createHash('sha256').update(body).digest('hex');
  const key = 'a'.repeat(64),
    missing = 'b'.repeat(64);
  const manifest = {
    version: 1,
    state: 'COMPLETE',
    missingReady: 1,
    objects: [
      { key, size: body.length, sha256, sourceState: 'READY', present: true },
      { key: missing, size: 1, sha256: '0'.repeat(64), sourceState: 'READY', present: false },
    ],
  };
  const backup = { resourceSnapshot: { mode: 'OSS_LOCAL_SNAPSHOT', objects: 2, missingReady: 1 } };
  writeFileSync(join(directory, key), body);
  writeFileSync(join(directory, 'resource-snapshot.json'), JSON.stringify(manifest));
  await verifyResourceSnapshotDirectory(directory, backup);
  const damaged = Buffer.from(body);
  damaged[0] ^= 1;
  writeFileSync(join(directory, key), damaged);
  await assert.rejects(
    verifyResourceSnapshotDirectory(directory, backup),
    /RESTORE_RESOURCE_DIGEST_CHANGED/,
  );
  writeFileSync(join(directory, key), body);
  writeFileSync(join(directory, missing), 'x');
  await assert.rejects(
    verifyResourceSnapshotDirectory(directory, backup),
    /RESTORE_RESOURCE_ABSENCE_CHANGED/,
  );
});

test('legacy metadata-only entries preserve null checksum and prove local absence', async () => {
  const directory = mkdtempSync(join(tmpdir(), 'collabreport-legacy-snapshot-'));
  const key = 'fixture-storage-original';
  const manifest = {
    version: 1,
    state: 'COMPLETE',
    missingReady: 1,
    objects: [
      { key, size: 1, sha256: '', sourceState: 'READY', present: false, metadataOnly: true },
    ],
  };
  const backup = { resourceSnapshot: { mode: 'OSS_LOCAL_SNAPSHOT', objects: 1, missingReady: 1 } };
  writeFileSync(join(directory, 'resource-snapshot.json'), JSON.stringify(manifest));
  await verifyResourceSnapshotDirectory(directory, backup);
  writeFileSync(join(directory, key), 'x');
  await assert.rejects(
    verifyResourceSnapshotDirectory(directory, backup),
    /RESTORE_RESOURCE_ABSENCE_CHANGED/,
  );
});
