import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
import { linuxProcessFacts } from './process_probe_linux.mjs';

// 只读核验指定PID、绝对入口和原启动标记；三秒探测超时继续视为未知，不构成退出证明。
export class ProcessOwner {
  constructor(root) {
    this.root = root;
  }
  exitProof(facts) {
    if (process.platform === 'linux') {
      const observed = linuxProcessFacts(facts);
      if (observed.state === 'PRESENT') {
        const entryFound = observed.args.includes(facts.entry);
        const ownerFound = observed.args.some(
          (arg, index) => arg === '--run-owner' && observed.args[index + 1] === facts.owner,
        );
        if (entryFound && ownerFound) throw Error('OLD_OUTPUT_PROCESS_STILL_RUNNING');
      }
      return 'PID_CHECK:' + observed.state;
    }
    if (process.platform !== 'win32') throw Error('OUTPUT_EXIT_PROOF_UNAVAILABLE');
    const probe = spawnSync(
      'powershell',
      [
        '-NoProfile',
        '-NonInteractive',
        '-File',
        resolve(this.root, 'scripts/process-owner.ps1'),
        '-ProcessId',
        String(facts.pid),
      ],
      { windowsHide: true, encoding: 'utf8', timeout: 3000 },
    );
    if (probe.status !== 0) throw Error('OUTPUT_EXIT_PROOF_UNAVAILABLE');
    let observed;
    try {
      observed = JSON.parse(probe.stdout);
    } catch {
      throw Error('OUTPUT_EXIT_PROOF_UNAVAILABLE');
    }
    if (
      observed.state === 'PRESENT' &&
      observed.commandLine?.toLowerCase().includes(String(facts.entry).toLowerCase()) &&
      observed.commandLine.includes('--run-owner ' + facts.owner)
    )
      throw Error('OLD_OUTPUT_PROCESS_STILL_RUNNING');
    if (!['ABSENT', 'PRESENT'].includes(observed.state))
      throw Error('OUTPUT_EXIT_PROOF_UNAVAILABLE');
    return 'PID_CHECK:' + observed.state;
  }
}
