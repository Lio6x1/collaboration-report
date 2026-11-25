# 分步编写 `services/collaboration/src/application/bootstrap_fence.mjs`

先按 [C03 · 撤权门闩、真实输出排空与进程恢复](../lessons/collaboration/03-fence-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/327-bootstrap-fence.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保留仅适用于完全空业务库的旧启动撤权边界。

没有房间、grant和delivery才能证明不存在需排空输出；正式服务必须使用完整fenceRooms。

写之前先按这条执行路线理解各段的连接：

1. 检查原绑定及历史终态
2. 逐表COUNT room/collaboration_grant/delivery_session
3. 任一非零抛FULL_OUTPUT_DRAIN_REQUIRED
4. 仅空库事务内写BLOCKED和ACK，COMMIT成功后返回

本文件需要守住的边界：

- 不得删除非空检查来让启动通过
- 驱动异常转换FENCE_COMMIT_UNKNOWN，不能宣布撤权成功

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/bootstrap_fence.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/bootstrap_fence.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/bootstrap_fence.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/bootstrap_fence.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:197 -->
````javascript
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { binding, ack, assertSameBinding } from '../domain/fence_protocol.mjs';
import * as repository from '../repository/fences.mjs';

````

## 第 2 步：bootstrapFence

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

只保留历史空启动协议作为具名安全边界；真实服务使用完整排空器，非空库绝不能空ACK。

本步声明：`export async function bootstrapFence(store, req) `。

**这一段为什么这样写：** 显式拒绝把历史空启动便捷路径用于现有数据。

1. 先验证请求绑定
2. 检查三张业务表全部空
3. 同事务关门并写终态
4. 异常保留未知提交语义


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>JavaScript 参数</code> | 包含pool的私有存储<br>在此声明中的实际使用：<code>store.pool</code> |
| <code>req</code> | <code>JavaScript 参数</code> | 固定撤权请求<br>在此声明中的实际使用：<code>binding(req)</code>；<code>req.closed_scopes</code>；<code>req.revocation_id</code>；<code>ack(req)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>ack(req)</code>；<code>result</code> |

**跟着本段代码走一遍：**

- 等待 <code>store.pool.getConnection()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>repository.startBootstrap(connection)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>repository.lockedClosure(connection, req.revocation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>prior</code>。
- 检查 <code>prior.state === &#x27;SUCCEEDED&#x27;</code>。条件成立时执行 <code>return ack(req);</code>。当前函数在这里结束。
- 检查 <code>String(count) !== &#x27;0&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(9, &#x27;FULL_OUTPUT_DRAIN_REQUIRED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(9, &#x27;FULL_OUTPUT_DRAIN_REQUIRED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!prior</code>。
- 等待 <code>repository.insertClosure(connection, req.revocation_id, scope, current)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>repository.blockSource(connection, req.revocation_id, scope)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>repository.completeClosure(       connection,       req.revocation_id,       scope,       result.result_digest,       true,     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>repository.commitBootstrap(connection)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err instanceof RPCFailure</code>。条件成立时执行 <code>throw err;</code>。转入外层异常处理路径。
- 执行 <code>throw err;</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 执行 <code>throw new RPCFailure(14, &#x27;FENCE_COMMIT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>repository.rollbackBootstrap(connection)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=197:1569 -->
````javascript
// 只保留历史空启动协议作为具名安全边界；真实服务使用完整排空器，非空库绝不能空ACK。
export async function bootstrapFence(store, req) {
  const current = binding(req),
    scope = req.closed_scopes[0],
    connection = await store.pool.getConnection();
  try {
    await repository.startBootstrap(connection);
    const prior = await repository.lockedClosure(connection, req.revocation_id);
    if (prior) {
      assertSameBinding(prior, scope, current);
      if (prior.state === 'SUCCEEDED') return ack(req);
    }
    for await (const count of repository.bootstrapBusinessCounts(connection))
      if (String(count) !== '0') throw new RPCFailure(9, 'FULL_OUTPUT_DRAIN_REQUIRED');
    if (!prior) await repository.insertClosure(connection, req.revocation_id, scope, current);
    await repository.blockSource(connection, req.revocation_id, scope);
    const result = ack(req);
    await repository.completeClosure(
      connection,
      req.revocation_id,
      scope,
      result.result_digest,
      true,
    );
    await repository.commitBootstrap(connection);
    return result;
  } catch (err) {
    if (err instanceof RPCFailure) throw err;
    throw new RPCFailure(14, 'FENCE_COMMIT_UNKNOWN');
  } finally {
    try {
      await repository.rollbackBootstrap(connection);
    } catch {}
    connection.release();
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/bootstrap_fence.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/03-fence-recovery.md)。
