# 分步编写 `services/collaboration/src/application/fences.mjs`

先按 [C03 · 撤权门闩、真实输出排空与进程恢复](../lessons/collaboration/03-fence-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/331-fences.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按关门提交、真实输出排空、终态提交三阶段撤权。

持有SQL事务等待socket关闭容易阻塞；提前ACK又会让已撤权连接继续读正文。

写之前先按这条执行路线理解各段的连接：

1. binding核对固定目标
2. 第一事务锁历史计划，关闭gate/grants/receipts并提交
3. engine.stopScope等待真实套接字停止
4. 第二事务completeClosure保存OUTPUT_STOPPED，返回ack

本文件需要守住的边界：

- stopScope失败必须留在FENCING等待恢复
- 同号SUCCEEDED可重放原ACK，但先验证绑定一致

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/fences.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/fences.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/fences.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/fences.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:137 -->
````javascript
import { binding, ack, assertSameBinding } from '../domain/fence_protocol.mjs';
import * as repository from '../repository/fences.mjs';

````

## 第 2 步：fenceRooms

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

撤权分成已提交门闩、真实输出排空、终态三个阶段；ACK永远在socket停止之后持久登记。

本步声明：`export async function fenceRooms(engine, req) `。

**这一段为什么这样写：** 把权限关闭与输出停止排成可恢复协议。

1. 已成功同号直接重放
2. 否则先持久关闭新准入
3. 排空所有受影响输出
4. 最终提交ACK


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>engine</code> | <code>JavaScript 参数</code> | 提供transaction与真实stopScope的房间引擎<br>在此声明中的实际使用：<code>engine.transaction</code>；<code>engine.stopScope</code> |
| <code>req</code> | <code>JavaScript 参数</code> | 原始固定撤权请求<br>在此声明中的实际使用：<code>binding(req)</code>；<code>req.closed_scopes</code>；<code>req.revocation_id</code>；<code>ack(req)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>ack(req)</code>；<code>result</code> |

**跟着本段代码走一遍：**

- 等待 <code>engine.transaction(async (connection) =&gt; {     const prior = await repository.lockedClosure(connection, req.revocation_id);     if (prior) {       assertSameBinding(prior, scope, c …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.lockedClosure(connection, req.revocation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>prior</code>。
- 在本段的内部函数内：检查 <code>prior.state === &#x27;SUCCEEDED&#x27;</code>。条件成立时执行 <code>return true;</code>。当前函数在这里结束。
- 在本段的内部函数内：等待 <code>repository.insertClosure(connection, req.revocation_id, scope, current)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.blockSource(connection, req.revocation_id, scope)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.closeGrants(connection, scope)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.closeReceipts(connection, scope)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return false;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>done</code>。条件成立时执行 <code>return ack(req);</code>。当前函数在这里结束。
- 等待 <code>engine.stopScope(scope)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>engine.transaction((connection) =&gt;     repository.completeClosure(connection, req.revocation_id, scope, result.result_digest),   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=137:1176 -->
````javascript
// 撤权分成已提交门闩、真实输出排空、终态三个阶段；ACK永远在socket停止之后持久登记。
export async function fenceRooms(engine, req) {
  const current = binding(req),
    scope = req.closed_scopes[0];
  const done = await engine.transaction(async (connection) => {
    const prior = await repository.lockedClosure(connection, req.revocation_id);
    if (prior) {
      assertSameBinding(prior, scope, current);
      if (prior.state === 'SUCCEEDED') return true;
    } else await repository.insertClosure(connection, req.revocation_id, scope, current);
    await repository.blockSource(connection, req.revocation_id, scope);
    await repository.closeGrants(connection, scope);
    await repository.closeReceipts(connection, scope);
    return false;
  });
  if (done) return ack(req);
  await engine.stopScope(scope);
  const result = ack(req);
  await engine.transaction((connection) =>
    repository.completeClosure(connection, req.revocation_id, scope, result.result_digest),
  );
  return result;
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/fences.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/03-fence-recovery.md)。
