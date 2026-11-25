import assert from 'node:assert/strict';
import { existsSync, lstatSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import { isAbsolute, resolve, dirname } from 'node:path';
import { randomUUID } from 'node:crypto';
import { command, root, runtime, state } from './runtime-lib.mjs';

// 配置只在真实停止后改写，不在服务运行中切换对象身份。原资源目录、
// 容量和上传期限保留；此脚本不读/显示 AK、SK，也不启动或删除任何环境。
try {
  assert.equal(process.platform, 'win32', 'OSS_CONFIGURATION_REQUIRES_WINDOWS');
  const argument = process.argv.indexOf('--settings');
  assert.ok(argument >= 2 && process.argv[argument + 1], 'OSS_SETTINGS_REQUIRED');
  const settingsFile = process.argv[argument + 1];
  assert.ok(isAbsolute(settingsFile), 'OSS_SETTINGS_ABSOLUTE_PATH_REQUIRED');
  const settings = JSON.parse(readFileSync(settingsFile, 'utf8').replace(/^\uFEFF/, ''));
  assert.match(settings.region, /^[a-z0-9]+(?:-[a-z0-9]+)+$/);
  assert.equal(
    settings.endpoint,
    `https://oss-${settings.region}.aliyuncs.com`,
    'OSS_ENDPOINT_INVALID',
  );
  assert.match(settings.bucket, /^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$/);
  assert.match(settings.prefix, /^(?:[a-zA-Z0-9_-]+\/)+$/);
  assert.ok(settings.prefix.length <= 160, 'OSS_PREFIX_INVALID');
  assert.ok(isAbsolute(settings.credentialsFile), 'OSS_CREDENTIAL_PATH_INVALID');
  for (let path = resolve(settings.credentialsFile); ; path = dirname(path)) {
    const info = lstatSync(path);
    assert.ok(!info.isSymbolicLink(), 'OSS_CREDENTIAL_LINK_REJECTED');
    if (dirname(path) === path) break;
  }
  assert.ok(lstatSync(settings.credentialsFile).isFile(), 'OSS_CREDENTIAL_FILE_INVALID');
  const environment = state();
  const registry = resolve(runtime, 'processes.json');
  const processes = existsSync(registry) ? JSON.parse(readFileSync(registry)) : [];
  for (const item of processes) {
    assert.equal(item.owner, environment.owner, 'OSS_PROCESS_OWNER_MISMATCH');
    const actual = JSON.parse(
      command('powershell', [
        '-NoProfile',
        '-NonInteractive',
        '-File',
        resolve(root, 'scripts/process-owner.ps1'),
        '-ProcessId',
        String(item.pid),
      ]),
    );
    assert.equal(actual.state, 'ABSENT', 'OSS_CONFIGURATION_REQUIRES_STOPPED_SERVICES');
  }
  // Windows 的 FileMode 不是 ACL。替换权限规则，只给当前运行账号访问，
  // 不把凭据值放入命令行、服务配置或结果。
  command(
    'powershell',
    [
      '-NoProfile',
      '-NonInteractive',
      '-File',
      resolve(root, 'scripts/protect-oss-credentials.ps1'),
      '-Path',
      settings.credentialsFile,
    ],
    { failureCode: 'OSS_CREDENTIAL_PROTECTION_FAILED' },
  );
  const file = resolve(runtime, 'services/document.json');
  const cfg = JSON.parse(readFileSync(file));
  assert.equal(cfg.role, 'document');
  assert.equal(resolve(cfg.root), resolve(root));
  assert.equal(cfg.rpcPort, environment.ports.documentRpc);
  assert.equal(cfg.database.port, environment.ports.mysql);
  const oss = {
    region: settings.region,
    endpoint: settings.endpoint,
    bucket: settings.bucket,
    prefix: settings.prefix,
    credentialsFile: resolve(settings.credentialsFile),
  };
  if (cfg.resources?.oss) {
    const existing = cfg.resources.oss;
    for (const key of ['region', 'endpoint', 'bucket', 'prefix'])
      assert.equal(oss[key], existing[key], 'OSS_BACKEND_CHANGE_REJECTED');
  }
  cfg.resources = {
    root: resolve(runtime, 'resources/document'),
    spaceBytes: 1 << 30,
    uploadBytes: 32 << 20,
    temporaryBytes: 256 << 20,
    uploadTTLSeconds: 3600,
    ...cfg.resources,
    oss,
  };
  const temporary = file + '.pending-' + randomUUID();
  writeFileSync(temporary, JSON.stringify(cfg, null, 2) + '\n', { flag: 'wx' });
  renameSync(temporary, file);
  console.log(
    JSON.stringify({
      state: 'CONFIGURED',
      role: 'document',
      provider: 'aliyun-oss',
      bucket: oss.bucket,
      prefix: oss.prefix,
      servicesRemainStopped: true,
    }),
  );
} catch (error) {
  console.error(
    JSON.stringify({
      state: 'FAILED',
      code: /^[A-Z0-9_]+$/.test(error.message) ? error.message : 'OSS_CONFIGURATION_FAILED',
    }),
  );
  process.exitCode = 1;
}
