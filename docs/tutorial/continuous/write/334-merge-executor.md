# 分步编写 `services/collaboration/src/application/merge_executor.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/334-merge-executor.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

以最多两个受限worker执行三方合并，保护Node事件循环。

CPU密集diff若堵住事件循环，会延迟WSS授权、撤权和自然到期处理。

写之前先按这条执行路线理解各段的连接：

1. active>=2返回MERGE_COMPUTE_BUSY供持久任务重试
2. Worker限制256MiB堆和4MiB栈
3. 等待结果，1.5秒预算到则给MERGE_COMPLEXITY_LIMIT冲突
4. finally清timer、terminate worker并减active

本文件需要守住的边界：

- 不能无限增加线程或计算期限来掩盖复杂度
- worker被终止没有数据库事务，因此不会产生未知提交

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/merge_executor.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/merge_executor.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/merge_executor.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/merge_executor.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:107 -->
````javascript
import { Worker } from 'node:worker_threads';
import { RPCFailure } from '../domain/shared_protocol.mjs';

````

## 第 2 步：MergeExecutor

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

有限计算线程保护WSS、来源gate和自然到期调度；队列满时由原Go任务重试。

本步声明：`export class MergeExecutor `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>active</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.active = 0</code> |

**本段进入 MergeExecutor.constructor：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 MergeExecutor.compute：**

**这一段为什么这样写：** 把长计算隔离并给出有限资源边界。

1. 检查并发名额
2. 启动纯计算worker
3. 等待结果/错误/预算
4. 始终终止线程回收名额


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 已经验证的base/submitted/main正文对象 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>await new Promise((resolve, reject) =&gt; {           worker.once(&#x27;message&#x27;, (value) =&gt;             value.result               ? resolve(value.result)               : reject(new RPCFa …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>this.active &gt;= 2</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MERGE_COMPUTE_BUSY&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;MERGE_COMPUTE_BUSY&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return await new Promise((resolve, reject) =&gt; {           worker.once(&#x27;message&#x27;, (value) =&gt;             value.result               ? resolve(value.result)               : reject(ne …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>new Promise((resolve, reject) =&gt; {           worker.once(&#x27;message&#x27;, (value) =&gt;             value.result               ? resolve(value.result)               : reject(new RPCFailure( …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>code !== 0</code>。
- 等待 <code>worker.terminate()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=107:1685 -->
````javascript
// 有限计算线程保护WSS、来源gate和自然到期调度；队列满时由原Go任务重试。
export class MergeExecutor {
  constructor() {
    this.active = 0;
  }

  async compute(input) {
    if (this.active >= 2) throw new RPCFailure(14, 'MERGE_COMPUTE_BUSY');
    this.active++;
    try {
      const worker = new Worker(new URL('../worker/merge_thread.mjs', import.meta.url), {
        workerData: input,
        resourceLimits: { maxOldGenerationSizeMb: 256, stackSizeMb: 4 },
      });
      let timer;
      try {
        return await new Promise((resolve, reject) => {
          worker.once('message', (value) =>
            value.result
              ? resolve(value.result)
              : reject(new RPCFailure(14, 'MERGE_COMPUTE_INVALID')),
          );
          worker.once('error', () => reject(new RPCFailure(14, 'MERGE_WORKER_FAILED')));
          worker.once('exit', (code) => {
            if (code !== 0) reject(new RPCFailure(14, 'MERGE_WORKER_FAILED'));
          });
          timer = setTimeout(
            () =>
              resolve({
                body: null,
                conflicts: [
                  {
                    block_id: '',
                    node_path: '$root',
                    kind: 'MERGE_COMPLEXITY_LIMIT',
                    reason: '固定输入超出自动计算时间预算，请分段处理。',
                  },
                ],
              }),
            1500,
          );
        });
      } finally {
        clearTimeout(timer);
        await worker.terminate();
      }
    } finally {
````

## 第 3 步：MergeExecutor（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 51 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

<!-- write-step: 3 bytes=1685:1718 -->
````javascript
      this.active--;
    }
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/merge_executor.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
