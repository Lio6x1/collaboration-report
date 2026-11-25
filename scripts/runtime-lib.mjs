import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { assertContainerOwner } from './ownership.mjs';
export const root = fileURLToPath(new URL('../', import.meta.url));
export const runtime = resolve(root, '.runtime');
export const stateFile = resolve(runtime, 'dev.json');
export const state = () => JSON.parse(readFileSync(stateFile));
export const pause = (ms) => new Promise((done) => setTimeout(done, ms));
export function saveJSON(file, value) {
  mkdirSync(resolve(file, '..'), { recursive: true });
  writeFileSync(file, JSON.stringify(value, null, 2) + '\n');
}
export function nodeArguments(args) {
  // Node 不会自动把 CLI 优化器选项传给子进程。只继承显式兼容选项，
  // 不转发 --test/--import/--eval 等调用者参数，保持服务入口与默认行为不变。
  return process.execArgv.includes('--no-maglev') && !args.includes('--no-maglev')
    ? ['--no-maglev', ...args]
    : args;
}
export function command(executable, args, options = {}) {
  const result = spawnSync(
    executable,
    executable === process.execPath ? nodeArguments(args) : args,
    {
      cwd: root,
      windowsHide: true,
      encoding: 'utf8',
      timeout: 60000,
      ...options,
    },
  );
  if (result.status !== 0) throw Error(options.failureCode ?? 'COMMAND_FAILED');
  return result.stdout.trim();
}
export function docker(...args) {
  return command('docker', args, { failureCode: 'DOCKER_COMMAND_FAILED' });
}
export function compose(...args) {
  const c = state();
  return docker(
    'compose',
    '--env-file',
    resolve(runtime, 'compose.env'),
    '-p',
    c.project,
    '-f',
    resolve(root, 'deploy/compose/dev.yml'),
    ...(c.ports.redis ? ['-f', resolve(root, 'deploy/compose/login.yml')] : []),
    ...(c.ports.kafka && c.ports.es ? ['-f', resolve(root, 'deploy/compose/search.yml')] : []),
    ...(c.deployment === 'release' ? ['-f', resolve(root, 'deploy/compose/release.yml')] : []),
    ...args,
  );
}
export function ownedContainer(service) {
  const c = state(),
    id = compose('ps', '--all', '-q', service);
  if (!id) throw Error('OWNED_CONTAINER_MISSING');
  const labels = JSON.parse(docker('inspect', '--format', '{{json .Config.Labels}}', id));
  assertContainerOwner(
    { owner: labels['collaboration.report.owner'], project: labels['com.docker.compose.project'] },
    c,
  );
  return id;
}
