import { existsSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import { isAbsolute, resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { command, root, runtime, state } from './runtime-lib.mjs';
import { smtpSettings, withSMTPSettings } from './smtp-configuration.mjs';

// 只调整Account的SMTP段，不生成owner/DSN、不读授权码、不启停服务或试发邮件。
// 在原服务停止时写入，避免把尚未加载的新通道误报为当前运行实例的投递模式。
try {
  if (process.platform !== 'win32') throw Error('SMTP_CONFIGURATION_REQUIRES_WINDOWS');
  if (process.argv.length !== 4 || process.argv[2] !== '--settings' || !isAbsolute(process.argv[3]))
    throw Error('SMTP_SETTINGS_ABSOLUTE_PATH_REQUIRED');
  const settings = smtpSettings(
    JSON.parse(readFileSync(process.argv[3], 'utf8').replace(/^\uFEFF/, '')),
  );
  const environment = state();
  const registry = resolve(runtime, 'processes.json');
  const processes = existsSync(registry) ? JSON.parse(readFileSync(registry)) : [];
  for (const item of processes) {
    if (item.owner !== environment.owner) throw Error('SMTP_PROCESS_OWNER_MISMATCH');
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
    if (actual.state !== 'ABSENT') throw Error('SMTP_CONFIGURATION_REQUIRES_STOPPED_SERVICES');
  }
  const file = resolve(runtime, 'services/account.json');
  const existing = JSON.parse(readFileSync(file));
  if (
    existing.role !== 'account' ||
    resolve(existing.root) !== resolve(root) ||
    existing.rpcPort !== environment.ports.accountRpc ||
    existing.database?.port !== environment.ports.mysql
  )
    throw Error('SMTP_SERVICE_BINDING_INVALID');
  if (settings.credentialsFile) {
    // 只读取路径/ACL元数据。授权码留在用户创建的私密文件中，不进入参数或输出。
    command(
      'powershell',
      [
        '-NoProfile',
        '-NonInteractive',
        '-File',
        resolve(root, 'scripts/protect-smtp-credentials.ps1'),
        '-Path',
        settings.credentialsFile,
      ],
      { failureCode: 'SMTP_CREDENTIAL_PROTECTION_FAILED' },
    );
  }
  const updated = withSMTPSettings(existing, settings);
  const temporary = file + '.pending-' + randomUUID();
  writeFileSync(temporary, JSON.stringify(updated, null, 2) + '\n', { flag: 'wx' });
  renameSync(temporary, file);
  console.log(
    JSON.stringify({ state: 'CONFIGURED', security: settings.security, requiresRestart: true }),
  );
} catch (error) {
  // JSON、文件系统或命令原文可能带绝对路径/服务器信息，输出仅限预定义固定码。
  console.error(
    JSON.stringify({
      state: 'FAILED',
      code: /^SMTP_[A-Z_]+$/.test(error.message) ? error.message : 'SMTP_CONFIGURATION_FAILED',
    }),
  );
  process.exitCode = 1;
}
