# 分步编写 `services/collaboration/src/worker/fence_recovery.mjs`

先按 [C03 · 撤权门闩、真实输出排空与进程恢复](../lessons/collaboration/03-fence-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/353-fence-recovery.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

监听开放前续接所有FENCING撤权计划。

进程退出不会取消已经提交的撤权；恢复时改用当前权限会更换历史目标。

写之前先按这条执行路线理解各段的连接：

1. pendingClosures读取原scope/epoch和target_permission
2. 还原generation及十六进制target_digest
3. 逐个调用完整fenceRooms直至真实排空和终态提交

本文件需要守住的边界：

- 任何恢复失败都应阻止READY
- 不能跳过UNKNOWN或为旧计划换新revocation_id

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/worker/fence_recovery.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/worker/fence_recovery.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/worker/fence_recovery.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/worker/fence_recovery.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:117 -->
````javascript
import { pendingClosures } from '../repository/fences.mjs';
import { fenceRooms } from '../application/fences.mjs';

````

## 第 2 步：recoverFences

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

启动前消费原持久目标；监听开放前完成所有未知撤权，不能根据当前权限重写旧计划。

本步声明：`export async function recoverFences(pool, engine) `。

**这一段为什么这样写：** 从持久计划恢复协议，而非重新推算当前要撤谁。

1. 读取待完成记录
2. 组装原请求
3. 等待完整排空用例返回


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pool</code> | <code>JavaScript 参数</code> | collaboration私有库连接池<br>在此声明中的实际使用：<code>pendingClosures(pool)</code> |
| <code>engine</code> | <code>JavaScript 参数</code> | 当前进程真实输出管理器所在引擎<br>在此声明中的实际使用：<code>fenceRooms(engine, {       revocation_id: pending.revocation_id,       closed_scopes: &#91;{ scope_key: pending.scope_key, epoch: String(pending.epoch) }&#93;,       generation: p.generati …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>pendingClosures(pool)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>fenceRooms(engine, {       revocation_id: pending.revocation_id,       closed_scopes: &#91;{ scope_key: pending.scope_key, epoch: String(pending.epoch) }&#93;,       generation: p.generati …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=117:772 -->
````javascript
// 启动前消费原持久目标；监听开放前完成所有未知撤权，不能根据当前权限重写旧计划。
export async function recoverFences(pool, engine) {
  for (const pending of await pendingClosures(pool)) {
    const p =
      typeof pending.target_permission === 'string'
        ? JSON.parse(pending.target_permission)
        : pending.target_permission;
    await fenceRooms(engine, {
      revocation_id: pending.revocation_id,
      closed_scopes: [{ scope_key: pending.scope_key, epoch: String(pending.epoch) }],
      generation: p.generation,
      target_digest: Buffer.from(p.bootstrap_target_digest, 'hex'),
    });
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/worker/fence_recovery.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/03-fence-recovery.md)。
