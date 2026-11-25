# services/collaboration/src/client/process_owner.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/307-process-owner.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`ProcessOwner`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`d06478d4684e601277fd4f5ea823a5cabaf0a73bd9f5f367171905a792cff85a`。[参考实现](../../../../services/collaboration/src/client/process_owner.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
