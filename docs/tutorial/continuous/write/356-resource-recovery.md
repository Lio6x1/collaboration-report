# 分步编写 `services/collaboration/src/worker/resource_recovery.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/356-resource-recovery.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

每秒恢复固定RESOURCE_CONFIRM，并保留未知资源保护。

跨服务确认可丢响应，后台只能沿原任务幂等重试，不能重造释放依据。

写之前先按这条执行路线理解各段的连接：

1. startConfirmationRecovery用recovering防止定时轮次重叠
2. ResourceConfirmations.recover读取原payload调用ConfirmDraftResources
3. CONFIRMED才markResourceConfirmed
4. 权限/状态终止错误7或9记PINNED_AWAITING_CLEANUP；其余记RETRY

本文件需要守住的边界：

- 任务SUCCEEDED且PINNED_AWAITING_CLEANUP不等于资源已释放；pin继续等待正式清理证明
- 调度器catch只是等待下一轮，不能据此对外宣称确认成功

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/worker/resource_recovery.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/worker/resource_recovery.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/worker/resource_recovery.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/worker/resource_recovery.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:274 -->
````javascript
import { json } from '../domain/shared_protocol.mjs';
import * as resourceConfirmations from '../repository/resource_confirmations.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';

// 来源 confirmation_scheduler.mjs：原业务声明与函数体保留。
````

## 第 2 步：startConfirmationRecovery

插入位置：第 1 步代码之后，紧接追加；完成稿约第 6 行。

同一恢复轮不重叠；未知确认保持pin，由既有worker按固定任务继续，不以静默catch冒充成功。

本步声明：`export function startConfirmationRecovery(engine) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>engine</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>engine       .recoverResources</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>() =&gt; clearInterval(timer)</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>recovering</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。
- 此处通过 <code>return () =&gt; clearInterval(timer);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=274:832 -->
````javascript
// 同一恢复轮不重叠；未知确认保持pin，由既有worker按固定任务继续，不以静默catch冒充成功。
export function startConfirmationRecovery(engine) {
  let recovering = false;
  const timer = setInterval(() => {
    if (recovering) return;
    recovering = true;
    engine
      .recoverResources()
      .catch(() => {})
      .finally(() => {
        recovering = false;
      });
  }, 1000);
  timer.unref();
  return () => clearInterval(timer);
}

// 来源 resource_confirmations.mjs：原业务声明与函数体保留。

````

## 第 3 步：ResourceConfirmations

插入位置：第 2 步代码之后，紧接追加；完成稿约第 25 行。

调度与重试不虚构保存依据；未知结果保留pin，闭来源的资源等待可证明的生命周期清理。

本步声明：`export class ResourceConfirmations `。

**这一段为什么这样写：** 消费与日志同事务落库的固定确认请求。

1. 优先按id恢复本次保存，也支持按到期批量恢复
2. 验证远端state为CONFIRMED
3. 按错误类型记录重试或等待受控清理


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.store = store</code> |
| <code>peers</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |

**本段进入 ResourceConfirmations.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.store = store</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 ResourceConfirmations.recover：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>resourceConfirmations.pendingResourceConfirmations(this.store.pool, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>resourceConfirmations.pendingResourceConfirmations(this.store.pool, id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>this.peers.rpc(&#x27;ConfirmDraftResources&#x27;, json(task.payload))</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>receipt.state !== &#x27;CONFIRMED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;RESOURCE_CONFIRM_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;RESOURCE_CONFIRM_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>resourceConfirmations.markResourceConfirmed(this.store.pool, task.business_key)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>resourceConfirmations.retryResourceConfirmation(           this.store.pool,           terminal ? &#x27;SUCCEEDED&#x27; : &#x27;RETRY&#x27;,           terminal ? &#x27;PINNED_AWAITING_CLEANUP&#x27; : &#x27;RESOURCE_C …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=832:2042 -->
````javascript
// 调度与重试不虚构保存依据；未知结果保留pin，闭来源的资源等待可证明的生命周期清理。
export class ResourceConfirmations {
  constructor(store, peers) {
    this.store = store;
    this.peers = peers;
  }

  async recover(id) {
    const [tasks] = await resourceConfirmations.pendingResourceConfirmations(this.store.pool, id);
    for (const task of tasks) {
      try {
        const receipt = await this.peers.rpc('ConfirmDraftResources', json(task.payload));
        if (receipt.state !== 'CONFIRMED') throw new RPCFailure(14, 'RESOURCE_CONFIRM_UNKNOWN');
        await resourceConfirmations.markResourceConfirmed(this.store.pool, task.business_key);
      } catch (err) {
        // 已闭来源不会被后台任务重开。临时引用保持pin，闭代次的清理由后续可证明的生命周期协议接管。
        const terminal = err instanceof RPCFailure && [7, 9].includes(err.code);
        await resourceConfirmations.retryResourceConfirmation(
          this.store.pool,
          terminal ? 'SUCCEEDED' : 'RETRY',
          terminal ? 'PINNED_AWAITING_CLEANUP' : 'RESOURCE_CONFIRM_UNKNOWN',
          task.business_key,
        );
      }
    }
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/worker/resource_recovery.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
