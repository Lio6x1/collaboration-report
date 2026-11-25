# 分步编写 `services/collaboration/src/worker/output_recovery.mjs`

先按 [C03 · 撤权门闩、真实输出排空与进程恢复](../lessons/collaboration/03-fence-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/355-output-recovery.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

启动时依次核验旧输出退出、验证全部房间和恢复资源确认。

监听先开放会让旧输出和新输出重叠，也会在发现持久日志损坏之前发出READY。

写之前先按这条执行路线理解各段的连接：

1. initialize读NODE_INSTANCE，跳过已有exit_proof的记录
2. ProcessOwner取得证明后，同事务结算旧delivery并记录证明
3. 登记当前pid/entry/COLLAB_RUN_OWNER
4. snapshots.verifyRooms实际恢复全部房间，confirmations.recover后设置recovered

本文件需要守住的边界：

- 任一旧owner仍活着、日志损坏或恢复异常都不能置recovered
- 启动阶段日志只记录固定阶段，不泄漏正文/凭据

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/worker/output_recovery.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/worker/output_recovery.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/worker/output_recovery.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/worker/output_recovery.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:163 -->
````javascript
import { resolve } from 'node:path';
import * as outputInstances from '../repository/output_instances.mjs';
import { json } from '../domain/shared_protocol.mjs';

````

## 第 2 步：OutputRecovery

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

恢复旧实例输出、全部连续房间日志及资源任务后才允许就绪，不能把旧持久房间当作空启动。

本步声明：`export class OutputRecovery `。

**这一段为什么这样写：** 为服务就绪建立可核查的前置条件。

1. OLD_OUTPUTS_RECOVERED→ROOMS_VERIFIED→CONFIRMATIONS_RECOVERED
2. 全部完成才更新recovered和roomsRecovered


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.store = store</code> |
| <code>cfg</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.cfg = cfg</code> |
| <code>instance</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.instance = instance</code> |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>snapshots</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>confirmations</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.confirmations = confirmations</code> |
| <code>processOwner</code> | <code>实例字段</code> | 实际进程身份探测器<br>在此声明中的实际使用：<code>this.processOwner = processOwner</code> |
| <code>startupStage</code> | <code>实例字段</code> | 仅输出稳定阶段名的诊断回调<br>在此声明中的实际使用：<code>this.startupStage = startupStage</code> |
| <code>recovered</code> | <code>实例字段</code> | 只有initialize所有步骤完成后为true<br>在此声明中的实际使用：<code>this.recovered = false</code> |

**本段进入 OutputRecovery.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.store = store</code> |
| <code>cfg</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.cfg = cfg</code> |
| <code>instance</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.instance = instance</code> |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>confirmations</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.confirmations = confirmations</code> |
| <code>processOwner</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.processOwner = processOwner</code> |
| <code>startupStage = () =&gt; {}</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>this.startupStage = startupStage</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 OutputRecovery.initialize：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>outputInstances.listOutputInstances(this.store.pool)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>old.business_key === this.instance &#124;&#124; facts.exit_proof</code>。条件成立时执行 <code>continue;</code>。跳过当前循环本轮剩余语句。
- 等待 <code>this.transactions.run(async (c) =&gt; {         await outputInstances.settleExitedOutputs(c, old.business_key);         await outputInstances.recordOutputExit(c, JSON.stringify(facts), old.business_key);       })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>outputInstances.settleExitedOutputs(c, old.business_key)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>outputInstances.recordOutputExit(c, JSON.stringify(facts), old.business_key)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>outputInstances.insertOutputInstance(       this.store.pool,       this.instance,       JSON.stringify(facts),     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=163:1740 -->
````javascript
// 恢复旧实例输出、全部连续房间日志及资源任务后才允许就绪，不能把旧持久房间当作空启动。
export class OutputRecovery {
  constructor(
    store,
    cfg,
    instance,
    transactions,
    snapshots,
    confirmations,
    processOwner,
    startupStage = () => {},
  ) {
    this.store = store;
    this.cfg = cfg;
    this.instance = instance;
    this.transactions = transactions;
    this.snapshots = snapshots;
    this.confirmations = confirmations;
    this.processOwner = processOwner;
    this.startupStage = startupStage;
    this.recovered = false;
  }

  async initialize() {
    // 超时不是进程退出证据。指定PID、绝对入口和随机启动标记三者共同确定旧输出所有者。
    const [instances] = await outputInstances.listOutputInstances(this.store.pool);
    for (const old of instances) {
      const facts = json(old.payload);
      if (old.business_key === this.instance || facts.exit_proof) continue;
      facts.exit_proof = this.processOwner.exitProof(facts);
      await this.transactions.run(async (c) => {
        await outputInstances.settleExitedOutputs(c, old.business_key);
        await outputInstances.recordOutputExit(c, JSON.stringify(facts), old.business_key);
      });
    }
    const facts = {
      pid: process.pid,
      entry: resolve(this.cfg.root, 'services/collaboration/src/main.mjs'),
      owner: process.env.COLLAB_RUN_OWNER,
    };
    await outputInstances.insertOutputInstance(
      this.store.pool,
      this.instance,
      JSON.stringify(facts),
    );
````

## 第 3 步：OutputRecovery（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 50 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 等待 <code>this.snapshots.verifyRooms()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>this.confirmations.recover()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=1740:2030 -->
````javascript
    this.startupStage('OLD_OUTPUTS_RECOVERED');
    await this.snapshots.verifyRooms();
    this.startupStage('ROOMS_VERIFIED');
    await this.confirmations.recover();
    this.startupStage('CONFIRMATIONS_RECOVERED');
    this.recovered = true;
    this.store.roomsRecovered = true;
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/worker/output_recovery.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/03-fence-recovery.md)。
