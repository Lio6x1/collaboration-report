import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { once } from 'node:events';
import { ProcessOwner } from '../../services/collaboration/src/client/process_owner.mjs';

test('Linux原PID与owner必须真实存活，回收后才接受退出证明', async (t) => {
  assert.equal(process.platform, 'linux', '此项应在真实Linux容器运行');
  const root = await mkdtemp(join(tmpdir(), 'collab-owner-'));
  const entry = join(root, 'child.mjs');
  await writeFile(
    entry,
    "console.log('READY'); process.stdin.resume(); process.stdin.on('end', () => process.exit(0));\n",
  );
  const owner = 'linux-original-owner';
  const child = spawn(process.execPath, [entry, '--run-owner', owner], {
    stdio: ['pipe', 'pipe', 'pipe'],
  });
  const exited = once(child, 'exit');
  t.after(async () => {
    if (child.exitCode === null && child.signalCode === null) {
      child.kill('SIGKILL');
      await exited;
    }
    await rm(root, { recursive: true, force: true });
  });
  await Promise.race([
    once(child.stdout, 'data').then(([raw]) => assert.match(String(raw), /READY/)),
    exited.then(() => {
      throw Error('原实例未就绪便退出');
    }),
  ]);
  const facts = { pid: child.pid, entry, owner };
  const probe = new ProcessOwner(root);
  assert.throws(() => probe.exitProof(facts), /OLD_OUTPUT_PROCESS_STILL_RUNNING/);
  assert.equal(probe.exitProof({ ...facts, owner: 'linux-original' }), 'PID_CHECK:PRESENT');
  assert.throws(() => probe.exitProof({ ...facts, pid: 0 }), /OUTPUT_EXIT_PROOF_UNAVAILABLE/);
  child.stdin.end();
  const [code, signal] = await exited;
  assert.equal(code, 0);
  assert.equal(signal, null);
  assert.equal(probe.exitProof(facts), 'PID_CHECK:ABSENT');
});
