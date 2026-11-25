import assert from 'node:assert/strict';
import test from 'node:test';
import { createHash } from 'node:crypto';
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { root } from '../../scripts/runtime-lib.mjs';

test('real OSS cold backup restores complete bytes using an isolated local backend', async () => {
  const target = resolve(process.env.COLLAB_RESTORE_ROOT ?? '');
  assert.ok(process.env.COLLAB_RESTORE_ROOT, 'RESTORE_TARGET_REQUIRED');
  assert.notEqual(target, resolve(root), 'RESTORE_MUST_BE_ISOLATED');
  const runtime = resolve(target, '.runtime');
  const config = JSON.parse(readFileSync(resolve(runtime, 'services/document.json')));
  assert.equal(config.resources.oss, undefined, 'RESTORE_MUST_HAVE_NO_CLOUD_WRITE_CAPABILITY');
  assert.equal(existsSync(resolve(config.resources.root, '.oss-binding')), false);
  const probe = JSON.parse(readFileSync(resolve(runtime, 'services/restore-oss-probe.json')));
  const local = readFileSync(resolve(config.resources.root, probe.key));
  assert.equal(local.length, probe.byteSize);
  assert.equal(createHash('sha256').update(local).digest('hex'), probe.checksum);
  const snapshot = JSON.parse(
    readFileSync(resolve(config.resources.root, 'resource-snapshot.json')),
  );
  assert.equal(snapshot.state, 'COMPLETE');
  assert.ok(snapshot.objects.some((item) => item.key === probe.key && item.present));
  // 导入恢复目录的现有 HTTP 助手，CA/端口来自该副本，不借用原 owner 会话。
  const { post, json } = await import(pathToFileURL(resolve(target, 'tests/helpers/account.mjs')));
  const { binaryResourceGet } = await import(
    pathToFileURL(resolve(target, 'tests/helpers/resource.mjs'))
  );
  const login = await post('/v1/account/login', { email: probe.email, password: probe.password });
  assert.equal(login.status, 200);
  const headers = {
    cookie: login.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
    'x-csrf-token': json(login).csrf_token,
  };
  const downloaded = await binaryResourceGet(probe.path, headers);
  assert.equal(downloaded.status, 200);
  assert.equal(downloaded.headers['content-type'], 'image/png');
  assert.deepEqual(downloaded.body, local);
  console.log(
    'OSS_RESTORE_STAGE_PASS: cloud-only source → local snapshot → real login and reference-bound HTTP bytes',
  );
});
