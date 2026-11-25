import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import test from 'node:test';
import { root, state, pause } from '../../scripts/runtime-lib.mjs';

const image = 'golang@sha256:154bd7001b6eb339e88c964442c0ad6ed5e53f09844cc818a41ce4ecb3ce3b43';
function docker(args, timeout = 120000) {
  const result = spawnSync('docker', args, {
    cwd: root,
    windowsHide: true,
    encoding: 'utf8',
    timeout,
  });
  assert.equal(
    result.status,
    0,
    `LINUX_EXPORT_DOCKER_FAILED ${args[0]} ${result.stdout.slice(-3000)} ${result.stderr}`,
  );
  return result.stdout.trim();
}

test(
  'real Linux cancellation kills converter group and PID1 exit ends its container descendants',
  { timeout: 180000 },
  async () => {
    const owner = state().owner;
    const attempt = randomUUID(),
      name = 'collab-export-exit-' + attempt;
    const evidence = resolve(root, 'artifacts/batches/08/linux-process', attempt);
    mkdirSync(evidence, { recursive: true });
    // 仅挂载转换器源码与独立证据目录；没有网络、业务卷或正式服务配置。
    const source = resolve(root, 'services/document/internal/exporter');
    const common = [
      '--network',
      'none',
      '--read-only',
      '--memory',
      '1g',
      '--cap-drop',
      'ALL',
      '--security-opt',
      'no-new-privileges',
      '--tmpfs',
      '/tmp:rw,exec,size=512m',
      '--label',
      'collaboration.report.owner=' + owner,
      '--mount',
      `type=bind,source=${source},target=/source,readonly`,
      '--mount',
      `type=bind,source=${evidence},target=/evidence`,
    ];
    const compilation = docker([
      'run',
      '--rm',
      ...common,
      '-e',
      'GO111MODULE=off',
      '-e',
      'GOCACHE=/tmp/go-cache',
      image,
      'sh',
      '-ec',
      'go test -v /source/process_linux.go /source/process_linux_test.go; go test -c -o /evidence/process-probe /source/process_linux.go /source/process_linux_test.go',
    ]);
    assert.match(compilation, /--- PASS: TestRunProgramCancelsConverterGroup/);
    writeFileSync(resolve(evidence, 'linux-test.log'), compilation + '\n');
    let created = false;
    try {
      docker([
        'run',
        '-d',
        '--name',
        name,
        ...common,
        '-e',
        'EXPORT_NAMESPACE_EVIDENCE=/evidence',
        '--entrypoint',
        '/evidence/process-probe',
        image,
      ]);
      created = true;
      const deadline = Date.now() + 10000;
      while (!existsSync(resolve(evidence, 'child.pid')) && Date.now() < deadline) await pause(50);
      assert.ok(existsSync(resolve(evidence, 'child.pid')), 'NAMESPACE_CONVERTER_NOT_STARTED');
      const childPID = Number(readFileSync(resolve(evidence, 'child.pid'), 'utf8').trim());
      assert.ok(Number.isInteger(childPID) && childPID > 1, 'CONVERTER_PID_INVALID');
      const before = JSON.parse(docker(['inspect', name]))[0];
      assert.equal(before.Config.Labels['collaboration.report.owner'], owner);
      assert.equal(before.State.Running, true);
      assert.ok(before.State.Pid > 0);
      // 命名空间内进程不能强杀自己的PID1；由Docker所在的祖先空间
      // 向服务PID1送SIGKILL。不是先执行正常停止再声称覆盖了崩溃。
      docker(['kill', '--signal', 'KILL', name]);
      assert.equal(docker(['wait', name], 10000), '137');
      const after = JSON.parse(docker(['inspect', name]))[0];
      assert.equal(after.State.Running, false);
      assert.equal(after.State.Pid, 0);
      writeFileSync(
        resolve(evidence, 'result.json'),
        JSON.stringify(
          {
            state: 'PASS',
            image,
            owner,
            childPID,
            servicePID1ExitCode: 137,
            hostPIDAfterExit: 0,
            completedAt: new Date().toISOString(),
          },
          null,
          2,
        ) + '\n',
      );
      console.log(
        'LINUX_EXPORT_PROCESS_ACTUAL_PASS: context group kill; real service PID1 SIGKILL; isolated PID namespace stopped',
      );
    } finally {
      if (created) {
        const facts = JSON.parse(docker(['inspect', name]))[0];
        assert.equal(facts.Config.Labels['collaboration.report.owner'], owner);
        docker(['rm', '-f', name]);
      }
    }
  },
);
