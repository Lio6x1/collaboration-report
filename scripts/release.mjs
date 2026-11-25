import assert from 'node:assert/strict';
import { readFileSync, existsSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { createHash } from 'node:crypto';
import { state, stateFile, root, runtime, saveJSON, compose, command } from './runtime-lib.mjs';

const roles = ['account', 'document', 'search', 'collaboration'];
const digest = (path) => createHash('sha256').update(readFileSync(path)).digest('hex');

// 发行记录只包含源码/锁文件/迁移/镜像摘要，不拷贝服务配置或数据库口令。
// enable只选择持久发行层；显式apply才重建已核验本owner依赖，且不会迁移数据。
function inspect(environment) {
  assert.equal(process.platform, 'win32', 'RELEASE_FIRST_EDITION_REQUIRES_WINDOWS');
  assert.ok(existsSync(resolve(root, 'web/dist/index.html')), 'RELEASE_WEB_BUILD_REQUIRED');
  for (const role of roles) {
    const cfg = JSON.parse(readFileSync(resolve(runtime, 'services', role + '.json')));
    assert.equal(cfg.role, role);
    assert.equal(resolve(cfg.root), resolve(root));
    assert.equal(cfg.database.host, '127.0.0.1');
    assert.equal(cfg.database.database, role + '_db');
  }
  const model = JSON.parse(compose('config', '--format', 'json'));
  const services = ['mysql', 'redis', 'kafka', 'es', 'canal'].map((name) => {
    const service = model.services[name];
    assert.ok(service, 'RELEASE_DEPENDENCIES_REQUIRED');
    assert.equal(service.labels['collaboration.report.owner'], environment.owner);
    assert.match(service.image, /@sha256:[a-f0-9]{64}$/);
    for (const port of service.ports ?? []) assert.equal(port.host_ip, '127.0.0.1');
    if (environment.deployment === 'release') {
      assert.equal(service.restart, 'unless-stopped');
      assert.equal(service.logging.driver, 'local');
    }
    return {
      role: name,
      image: service.image,
      memoryLimitBytes: service.mem_limit,
      logging: service.logging?.driver ?? 'default',
    };
  });
  return {
    version: 1,
    state: 'CONFIG_VERIFIED',
    deployment: environment.deployment ?? 'development',
    commit: command('git', ['rev-parse', 'HEAD']),
    dirty: Boolean(command('git', ['status', '--porcelain'])),
    lockDigest: digest(resolve(root, 'package-lock.json')),
    schemaManifestDigest: digest(resolve(root, 'db/migrations/schema-manifest.json')),
    migrations: readdirSync(resolve(root, 'db/migrations'))
      .filter((name) => /^\d+\.up\.sql$/.test(name))
      .sort()
      .map((file) => ({ file, sha256: digest(resolve(root, 'db/migrations', file)) })),
    goModules: roles
      .filter((role) => role !== 'collaboration')
      .map((role) => ({
        role,
        moduleDigest: digest(resolve(root, 'services', role, 'go.mod')),
        sumDigest: digest(resolve(root, 'services', role, 'go.sum')),
      })),
    services,
    generatedAt: new Date().toISOString(),
  };
}

try {
  const action = process.argv[2] ?? 'check';
  assert.ok(['check', 'enable', 'apply'].includes(action), 'USAGE_RELEASE_CHECK_ENABLE_APPLY');
  const environment = state();
  if (action !== 'check') {
    const registry = JSON.parse(readFileSync(resolve(runtime, 'processes.json')));
    assert.equal(registry.length, 0, 'RELEASE_REQUIRES_STOPPED_APPLICATIONS');
    // 原容器的实际标签也须核验；config里的声明不能充当运行环境归属证明。
    const { ownedContainer } = await import('./runtime-lib.mjs');
    for (const name of ['mysql', 'redis', 'kafka', 'es', 'canal']) ownedContainer(name);
    inspect(environment);
    environment.deployment = 'release';
    saveJSON(stateFile, environment);
  }
  const evidence = inspect(environment);
  saveJSON(resolve(root, 'artifacts/batches/08/release-config.json'), evidence);
  if (action === 'apply') {
    compose('up', '-d', 'mysql', 'redis', 'kafka', 'es', 'canal');
    evidence.state = 'DEPENDENCIES_STARTED';
    saveJSON(resolve(root, 'artifacts/batches/08/release-config.json'), evidence);
  }
  console.log(
    JSON.stringify({
      state: evidence.state,
      deployment: evidence.deployment,
      dependencies: evidence.services.length,
      applicationReadyVerified: false,
    }),
  );
} catch (error) {
  const candidate = error.code ?? error.message;
  console.error(
    JSON.stringify({
      state: 'FAILED',
      code: /^[A-Z][A-Z0-9_]{1,95}$/.test(candidate) ? candidate : 'RELEASE_CHECK_FAILED',
    }),
  );
  process.exitCode = 1;
}
