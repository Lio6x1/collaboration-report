# 分步编写 `services/collaboration/src/client/process_owner.mjs`

先按 [C03 · 撤权门闩、真实输出排空与进程恢复](../lessons/collaboration/03-fence-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/307-process-owner.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按平台核验旧PID是否仍是原Node输出所有者。

PID可复用；仅检查PID存在会误拒绝无关进程，仅检查超时会误放行活着的旧owner。

写之前先按这条执行路线理解各段的连接：

1. Linux比较argv中完整入口和--run-owner后的精确值
2. Windows隐藏启动process-owner.ps1只读探测并解析JSON
3. 匹配原入口和owner则OLD_OUTPUT_PROCESS_STILL_RUNNING
4. 明确ABSENT或身份已变才返回PID_CHECK证明

本文件需要守住的边界：

- 探测超时、JSON损坏或未知平台均OUTPUT_EXIT_PROOF_UNAVAILABLE
- 此类不负责杀进程，旧实例仍活着时应先处理受管进程生命周期

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/client/process_owner.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/client/process_owner.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/client/process_owner.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/client/process_owner.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:149 -->
````javascript
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
import { linuxProcessFacts } from './process_probe_linux.mjs';

````

## 第 2 步：ProcessOwner

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

只读核验指定PID、绝对入口和原启动标记；三秒探测超时继续视为未知，不构成退出证明。

本步声明：`export class ProcessOwner `。

**这一段为什么这样写：** 以pid+入口+随机启动标记识别同一个历史输出进程。

1. 平台探测
2. 拒绝仍在运行的原owner
3. 只返回确定性退出/替换事实


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>root</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.root = root</code> |

**本段进入 ProcessOwner.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>root</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.root = root</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 ProcessOwner.exitProof：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>linuxProcessFacts(facts)</code>；<code>facts.entry</code>；<code>facts.owner</code>；<code>facts.pid</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>&#x27;PID_CHECK:&#x27; + observed.state</code> |

**跟着本段代码走一遍：**

- 检查 <code>process.platform === &#x27;linux&#x27;</code>。
- 检查 <code>observed.state === &#x27;PRESENT&#x27;</code>。
- 检查 <code>entryFound &amp;&amp; ownerFound</code>。条件成立时执行 <code>throw Error(&#x27;OLD_OUTPUT_PROCESS_STILL_RUNNING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;OLD_OUTPUT_PROCESS_STILL_RUNNING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return &#x27;PID_CHECK:&#x27; + observed.state;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>process.platform !== &#x27;win32&#x27;</code>。条件成立时执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>probe.status !== 0</code>。条件成立时执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>observed.state === &#x27;PRESENT&#x27; &amp;&amp;       observed.commandLine?.toLowerCase().includes(String(facts.entry).toLowerCase()) &amp;&amp;       observed.commandLine.includes(&#x27;--run-owner &#x27; + facts.owner)</code>。条件成立时执行 <code>throw Error(&#x27;OLD_OUTPUT_PROCESS_STILL_RUNNING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;OLD_OUTPUT_PROCESS_STILL_RUNNING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!&#91;&#x27;ABSENT&#x27;, &#x27;PRESENT&#x27;&#93;.includes(observed.state)</code>。条件成立时执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 2 bytes=149:1841 -->
````javascript
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
````

## 第 3 步：ProcessOwner（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 50 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return &#x27;PID_CHECK:&#x27; + observed.state;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1841:1889 -->
````javascript
    return 'PID_CHECK:' + observed.state;
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/client/process_owner.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/03-fence-recovery.md)。
