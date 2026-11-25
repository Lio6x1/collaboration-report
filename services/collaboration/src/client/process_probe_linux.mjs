import { readFileSync, statSync } from 'node:fs';
import { isAbsolute } from 'node:path';

// Linux应用必须共用PID命名空间；权限不足、空cmdline或procfs不可用都继续等待恢复。
export function linuxProcessFacts(facts) {
  const pid = Number(facts.pid);
  if (
    !Number.isSafeInteger(pid) ||
    pid <= 0 ||
    pid > 2147483647 ||
    typeof facts.entry !== 'string' ||
    !isAbsolute(facts.entry) ||
    typeof facts.owner !== 'string' ||
    !facts.owner
  )
    throw Error('OUTPUT_EXIT_PROOF_UNAVAILABLE');
  try {
    // 自身读取用于区分未挂载procfs和指定PID确实消失，不能把任意ENOENT当退出。
    readFileSync('/proc/self/cmdline');
  } catch {
    throw Error('OUTPUT_EXIT_PROOF_UNAVAILABLE');
  }
  const dir = `/proc/${pid}`;
  let raw;
  try {
    raw = readFileSync(`${dir}/cmdline`);
  } catch (error) {
    if (error.code === 'ENOENT') {
      try {
        statSync(dir);
      } catch (statError) {
        if (statError.code === 'ENOENT') return { state: 'ABSENT' };
      }
    }
    throw Error('OUTPUT_EXIT_PROOF_UNAVAILABLE');
  }
  if (!raw.length || raw.at(-1) !== 0) throw Error('OUTPUT_EXIT_PROOF_UNAVAILABLE');
  // NUL分隔的argv保留完整参数边界；禁止用owner前缀或命令行子串证明身份。
  return { state: 'PRESENT', args: raw.subarray(0, -1).toString('utf8').split('\0') };
}
