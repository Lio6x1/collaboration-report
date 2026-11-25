# 分步编写 `services/collaboration/src/client/process_probe_linux.mjs`

先按 [C03 · 撤权门闩、真实输出排空与进程恢复](../lessons/collaboration/03-fence-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/308-process-probe-linux.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在共享PID命名空间下读取Linux指定PID的完整argv事实。

/proc未挂载或权限不足与进程不存在不同；错误当ABSENT会放过仍在输出的旧进程。

写之前先按这条执行路线理解各段的连接：

1. 验证pid范围及绝对entry和owner
2. 先读/proc/self/cmdline证明procfs可用
3. 目标cmdline ENOENT后再stat进程目录，双重不存在才ABSENT
4. 非空NUL结尾argv返回PRESENT与精确参数数组

本文件需要守住的边界：

- 空cmdline、无权限或不完整argv都必须UNKNOWN失败
- 不能拿owner前缀或命令行子串作完整Linux身份证明

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/client/process_probe_linux.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/client/process_probe_linux.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/client/process_probe_linux.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/client/process_probe_linux.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:91 -->
````javascript
import { readFileSync, statSync } from 'node:fs';
import { isAbsolute } from 'node:path';

````

## 第 2 步：linuxProcessFacts

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

Linux应用必须共用PID命名空间；权限不足、空cmdline或procfs不可用都继续等待恢复。

本步声明：`export function linuxProcessFacts(facts) `。

**这一段为什么这样写：** 提供只读进程事实，供ProcessOwner判断原owner是否仍存活。

1. 验证探测输入
2. 验证procfs
3. 区分ABSENT与未知
4. 保留NUL分隔参数边界


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>JavaScript 参数</code> | 原启动记录中的pid、绝对entry和随机owner<br>在此声明中的实际使用：<code>facts.pid</code>；<code>facts.entry</code>；<code>facts.owner</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ state: &#x27;ABSENT&#x27; }</code>；<code>{ state: &#x27;PRESENT&#x27;, args: raw.subarray(0, -1).toString(&#x27;utf8&#x27;).split(&#x27;\0&#x27;) }</code> |

**跟着本段代码走一遍：**

- 检查 <code>!Number.isSafeInteger(pid) &#124;&#124;     pid &lt;= 0 &#124;&#124;     pid &gt; 2147483647 &#124;&#124;     typeof facts.entry !== &#x27;string&#x27; &#124;&#124;     !isAbsolute(facts.entry) &#124;&#124;     typeof facts.owner !== &#x27;string&#x27; &#124;&#124;     !facts.owner</code>。条件成立时执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>error.code === &#x27;ENOENT&#x27;</code>。
- 检查 <code>statError.code === &#x27;ENOENT&#x27;</code>。条件成立时执行 <code>return { state: &#x27;ABSENT&#x27; };</code>。当前函数在这里结束。
- 执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!raw.length &#124;&#124; raw.at(-1) !== 0</code>。条件成立时执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;OUTPUT_EXIT_PROOF_UNAVAILABLE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return { state: &#x27;PRESENT&#x27;, args: raw.subarray(0, -1).toString(&#x27;utf8&#x27;).split(&#x27;\0&#x27;) };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=91:1384 -->
````javascript
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

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/client/process_probe_linux.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/03-fence-recovery.md)。
