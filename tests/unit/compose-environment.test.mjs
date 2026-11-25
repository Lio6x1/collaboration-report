import test from 'node:test';
import assert from 'node:assert/strict';
import { composeEnvironment } from '../../scripts/compose-environment.mjs';
const base = () => ({
  project: 'collab-unit-test',
  owner: 'collab-test-owner',
  ports: { mysql: 20600, redis: 20610 },
  passwords: { root: 'a'.repeat(48) },
});

// 环境被登录/bootstrap重写后若丢失搜索字段，下次compose会接错端口或丢复制身份，不能用旧临时文件掩盖。
test('shared compose environment keeps existing owner and optional Redis settings', () => {
  const result = composeEnvironment(base());
  assert.equal(
    result,
    `COLLAB_PROJECT=collab-unit-test\nCOLLAB_OWNER=collab-test-owner\nMYSQL_ROOT_PASSWORD=${'a'.repeat(48)}\nMYSQL_PORT=20600\nREDIS_PORT=20610\n`,
  );
});
test('shared compose environment preserves Kafka, ES and the original Canal credential', () => {
  const configuration = base();
  Object.assign(configuration.ports, { kafka: 20612, es: 20613 });
  configuration.passwords.canal = 'b'.repeat(24);
  const first = composeEnvironment(configuration),
    second = composeEnvironment(structuredClone(configuration));
  assert.equal(first, second);
  assert.match(first, /KAFKA_PORT=20612\nES_PORT=20613\nCANAL_PASSWORD=b{24}\n$/);
  assert.match(first, /REDIS_PORT=20610\n/);
});
test('partial search state and unsafe environment fields fail before writing an env file', () => {
  const incomplete = base();
  incomplete.ports.kafka = 20612;
  assert.throws(() => composeEnvironment(incomplete), /COMPOSE_ENVIRONMENT_INVALID/);
  const injected = base();
  injected.owner = 'owner\nCOLLAB_PROJECT=other';
  assert.throws(() => composeEnvironment(injected), /COMPOSE_ENVIRONMENT_INVALID/);
});
