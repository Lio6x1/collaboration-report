import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime } from './runtime-lib.mjs';

export function composeEnvironment(configuration) {
  const { project, owner, ports, passwords } = configuration;
  const identifier = (value) => typeof value === 'string' && /^[A-Za-z0-9_.-]+$/.test(value);
  const port = (value) => Number.isInteger(value) && value >= 1024 && value <= 65535;
  const secret = (value) => typeof value === 'string' && /^[a-f0-9]{24,64}$/.test(value);
  if (
    !identifier(project) ||
    !identifier(owner) ||
    !port(ports?.mysql) ||
    !secret(passwords?.root) ||
    (ports.redis !== undefined && !port(ports.redis))
  )
    throw Error('COMPOSE_ENVIRONMENT_INVALID');
  const lines = [
    `COLLAB_PROJECT=${project}`,
    `COLLAB_OWNER=${owner}`,
    `MYSQL_ROOT_PASSWORD=${passwords.root}`,
    `MYSQL_PORT=${ports.mysql}`,
  ];
  if (ports.redis) lines.push(`REDIS_PORT=${ports.redis}`);
  if (ports.kafka !== undefined || ports.es !== undefined) {
    if (!port(ports.kafka) || !port(ports.es) || !secret(passwords.canal))
      throw Error('COMPOSE_ENVIRONMENT_INVALID');
    lines.push(
      `KAFKA_PORT=${ports.kafka}`,
      `ES_PORT=${ports.es}`,
      `CANAL_PASSWORD=${passwords.canal}`,
    );
  }
  return lines.join('\n') + '\n';
}

// 所有bootstrap共用一份渲染规则；登录依赖重建不能删除已登记搜索端口或更换原Canal秘密。
export function writeComposeEnvironment(configuration) {
  const content = composeEnvironment(configuration);
  writeFileSync(resolve(runtime, 'compose.env'), content);
}
