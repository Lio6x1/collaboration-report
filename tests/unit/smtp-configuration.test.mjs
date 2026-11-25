import test from 'node:test';
import assert from 'node:assert/strict';
import { resolve } from 'node:path';
import { smtpSettings, retainedSMTP, withSMTPSettings } from '../../scripts/smtp-configuration.mjs';
import { serviceConfiguration } from '../../scripts/server-configuration.mjs';

const remote = () => ({
  address: 'smtp.qq.com:465',
  from: 'sender@qq.com',
  security: 'tls',
  credentialsFile: resolve('smtp-test-private.json'),
});

test('更新SMTP与重跑准备都保留数据库、owner和既有真实投递通道', () => {
  const original = {
    role: 'account',
    dsn: 'owned-dsn',
    owner: 'owned-owner',
    peers: { document: 'owned-peer' },
    smtp: { address: '127.0.0.1:2525', from: 'sender@example.test' },
  };
  const updated = withSMTPSettings(original, remote());
  assert.deepEqual({ ...updated, smtp: original.smtp }, original);
  assert.deepEqual(updated.smtp, remote());
  assert.deepEqual(retainedSMTP(updated.smtp, original.smtp), remote());
  assert.equal(retainedSMTP(original.smtp, remote()), original.smtp);
  assert.equal(smtpSettings(original.smtp).security, 'development');
  const prepared = serviceConfiguration(
    'account',
    { developmentSMTP: false, smtpAddress: 'smtp.qq.com:465', smtpFrom: 'sender@qq.com' },
    {},
    {},
  );
  assert.equal(prepared.smtp.security, 'tls');
  assert.equal(prepared.smtp.credentialsFile, '/app/.runtime/private/smtp-credentials.json');
});

test('远程明文、内联授权码和非本机凭据路径不能成为SMTP设置', () => {
  for (const value of [
    { address: 'smtp.qq.com:25', from: 'sender@qq.com' },
    { ...remote(), password: 'must-never-enter-service-config' },
    { ...remote(), credentialsFile: 'relative-private.json' },
    { ...remote(), credentialsFile: '\\\\server\\credentials.json' },
    { ...remote(), security: 'development' },
    { ...remote(), from: 'sender@qq.com\r\nBcc: other@example.net' },
  ])
    assert.throws(() => smtpSettings(value), /SMTP_SETTINGS_INVALID/);
  assert.throws(
    () => withSMTPSettings({ role: 'document' }, remote()),
    /SMTP_SERVICE_BINDING_INVALID/,
  );
});
