# services/collaboration/src/client/process_probe_linux.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/308-process-probe-linux.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`linuxProcessFacts`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`447c8afc8356b5ce0eec74c3d733598c4e455e7fec7f510c12da43ccce723bfb`。[参考实现](../../../../services/collaboration/src/client/process_probe_linux.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
