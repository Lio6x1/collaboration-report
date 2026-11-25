import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
import {
  sourceRoot,
  argumentsForServer,
  deploymentDirectory,
  requirePreparedFiles,
} from './server-configuration.mjs';
import { assertContainerOwner } from './ownership.mjs';

function docker(args, inherit = false) {
  const result = spawnSync('docker', args, {
    cwd: sourceRoot,
    windowsHide: true,
    encoding: 'utf8',
    stdio: inherit ? 'inherit' : 'pipe',
    timeout: 600000,
  });
  if (result.status !== 0) throw Error('SERVER_DOCKER_COMMAND_FAILED');
  return inherit ? '' : result.stdout.trim();
}

function ownership(labels, manifest) {
  assertContainerOwner(
    {
      owner: labels?.['collaboration.report.owner'],
      project: labels?.['com.docker.compose.project'],
    },
    manifest,
  );
}

function verifyResources(manifest) {
  // 容器消失不代表卷已消失。先分别核验同名卷/网络和本project全部容器，
  // 才允许Compose创建、chown或启动，避免接管旁边owner的持久事实。
  const volumes = new Set(docker(['volume', 'ls', '--format', '{{.Name}}']).split(/\r?\n/));
  for (const suffix of ['mysql-data', 'redis-data', 'kafka-data', 'es-data', 'canal-position']) {
    const name = manifest.project + '_' + suffix;
    if (volumes.has(name))
      ownership(
        JSON.parse(docker(['volume', 'inspect', '--format', '{{json .Labels}}', name])),
        manifest,
      );
  }
  const networks = new Set(docker(['network', 'ls', '--format', '{{.Name}}']).split(/\r?\n/));
  const network = manifest.project + '_default';
  if (networks.has(network))
    ownership(
      JSON.parse(docker(['network', 'inspect', '--format', '{{json .Labels}}', network])),
      manifest,
    );
  const ids = docker([
    'ps',
    '-aq',
    '--filter',
    'label=com.docker.compose.project=' + manifest.project,
  ]);
  for (const id of ids.split(/\r?\n/).filter(Boolean))
    ownership(JSON.parse(docker(['inspect', '--format', '{{json .Config.Labels}}', id])), manifest);
}

try {
  const action = process.argv[2];
  if (!['up', 'initialize', 'stop', 'status', 'config', 'logs'].includes(action))
    throw Error('USAGE_SERVER_COMPOSE_UP_INITIALIZE_STOP_STATUS_CONFIG_LOGS');
  const options = argumentsForServer(process.argv.slice(3), ['--directory']);
  const directory = deploymentDirectory(options['--directory']);
  const { manifest } = requirePreparedFiles(directory);
  const base = [
    'compose',
    '--ansi',
    'never',
    '--env-file',
    resolve(directory, 'compose.env'),
    '-p',
    manifest.project,
    '-f',
    resolve(sourceRoot, 'deploy/compose/server.yml'),
    '--profile',
    'initialize',
  ];
  // 普通config会把Canal env_file秘密展开；包装器只给静态有效/无效结果。
  if (action === 'config') {
    docker([...base, 'config', '--quiet']);
    console.log(JSON.stringify({ state: 'COMPOSE_VALID', project: manifest.project }));
  } else {
    verifyResources(manifest);
    const args = {
      up: ['up', '-d', '--wait', '--wait-timeout', '300', 'application'],
      initialize: ['run', '--rm', 'initialize'],
      stop: ['stop', '--timeout', '30'],
      status: ['ps', '--all'],
      logs: ['logs', '--tail', '100', 'application'],
    }[action];
    docker([...base, ...args], true);
    console.log(
      JSON.stringify({
        state: action.toUpperCase() + '_COMPLETED',
        project: manifest.project,
        ...(action === 'up' ? { url: manifest.publicOrigin } : {}),
      }),
    );
  }
} catch (error) {
  console.error(
    JSON.stringify({
      state: 'FAILED',
      code: /^[A-Z0-9_]+$/.test(error.message) ? error.message : 'SERVER_COMPOSE_FAILED',
    }),
  );
  process.exitCode = 1;
}
