# 分步编写 `services/collaboration/src/application/personal_transfer_storage.mjs`

先按 [C06 · 固定候选清理、账号清理与转移计量](../lessons/collaboration/06-cleanup-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/336-personal-transfer-storage.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

核验Doc ACL和publication关闭终态后冻结Node侧存储用量。

转移不能在仍有写入和公开输出时采样变化中的容量，原号重试也不应重新计价。

写之前先按这条执行路线理解各段的连接：

1. 逐closure核BLOCKED来源与SUCCEEDED原authorization_change及目标摘要
2. 锁文档入口后读取原计量回执
3. 有回执规范化saved.request比较再重放；否则storageFacts→measurement→saveStorageReceipt

本文件需要守住的边界：

- MySQL JSON可能重排键，比较前先transferStorageRequest规范化
- 此接口只返回计量，不能重开来源或签发grant

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/personal_transfer_storage.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/personal_transfer_storage.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/personal_transfer_storage.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/personal_transfer_storage.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:415 -->
````javascript
import { createHash } from 'node:crypto';
import {
  transferStorageRequest,
  transferStorageMeasurement,
} from '../domain/personal_transfer_storage.mjs';
import { bad, json } from '../domain/shared_protocol.mjs';
import { lockedClosure } from '../repository/fences.mjs';
import { lockDocument } from '../repository/main_workspace.mjs';
import * as repository from '../repository/personal_transfer_storage.mjs';

````

## 第 2 步：PersonalTransferStorage

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

本步声明：`export class PersonalTransferStorage `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |

**本段进入 PersonalTransferStorage.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 PersonalTransferStorage.measure：**

**这一段为什么这样写：** 以已关闭来源和文档锁取得稳定容量切片。

1. 证明两条原关闭事实
2. 锁文档后原号去重
3. 采样并保存固定回执


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | operation_id/document_id/generation及两个closures<br>在此声明中的实际使用：<code>transferStorageRequest(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.transactions.run(async (c) =&gt; {       // 只接受真实终结的两条原关闭事实。mTLS限制调用方为Doc；此接口返回计量，       // 不发grant、不重新开放来源，也不能用另一个文档的ACK换取存储证明。       for (const scope of request.closures) {     …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return this.transactions.run(async (c) =&gt; {       // 只接受真实终结的两条原关闭事实。mTLS限制调用方为Doc；此接口返回计量，       // 不发grant、不重新开放来源，也不能用另一个文档的ACK换取存储证明。       for (const scope of request.closures …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>repository.lockStorageSource(c, scope.scope_key)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>lockedClosure(c, scope.revocation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!source &#124;&#124;           source.state !== &#x27;BLOCKED&#x27; &#124;&#124;           BigInt(source.max_epoch) &lt; BigInt(scope.epoch) &#124;&#124;           !closed &#124;&#124;           closed.state !== &#x27;SUCCEEDED&#x27; &#124;&#124;        …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_DRAIN_PENDING&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_DRAIN_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>lockDocument(c, request.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.storageReceipt(c, request.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>original</code>。
- 在本段的内部函数内：检查 <code>original.state !== &#x27;SUCCEEDED&#x27; &#124;&#124;           JSON.stringify(transferStorageRequest(saved.request)) !== JSON.stringify(request)</code>。条件成立时执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_BINDING_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_BINDING_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return saved.result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>repository.storageFacts(c, request.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=415:2553 -->
````javascript
export class PersonalTransferStorage {
  constructor(transactions) {
    this.transactions = transactions;
  }

  async measure(input) {
    const request = transferStorageRequest(input);
    return this.transactions.run(async (c) => {
      // 只接受真实终结的两条原关闭事实。mTLS限制调用方为Doc；此接口返回计量，
      // 不发grant、不重新开放来源，也不能用另一个文档的ACK换取存储证明。
      for (const scope of request.closures) {
        const source = await repository.lockStorageSource(c, scope.scope_key);
        const closed = await lockedClosure(c, scope.revocation_id);
        const digest = createHash('sha256')
          .update('AUTHORIZATION_CHANGE\n' + scope.scope_key + '\n' + scope.epoch)
          .digest('hex');
        const binding = closed && json(closed.target_permission);
        if (
          !source ||
          source.state !== 'BLOCKED' ||
          BigInt(source.max_epoch) < BigInt(scope.epoch) ||
          !closed ||
          closed.state !== 'SUCCEEDED' ||
          closed.scope_key !== scope.scope_key ||
          String(closed.epoch) !== scope.epoch ||
          binding?.generation !== '1' ||
          binding?.bootstrap_target_digest !== digest
        )
          throw bad('PERSONAL_TRANSFER_STORAGE_DRAIN_PENDING');
      }
      await lockDocument(c, request.document_id);
      const original = await repository.storageReceipt(c, request.operation_id);
      if (original) {
        const saved = json(original.payload);
        // MySQL JSON 会规范化对象字段顺序；重放比较严格解析后的绑定，
        // 而不是数据库读回对象的序列化顺序，否则真实原回执永远被误判冲突。
        if (
          original.state !== 'SUCCEEDED' ||
          JSON.stringify(transferStorageRequest(saved.request)) !== JSON.stringify(request)
        )
          throw bad('PERSONAL_TRANSFER_STORAGE_BINDING_CHANGED');
        return saved.result;
      }
      const facts = await repository.storageFacts(c, request.document_id);
      const result = transferStorageMeasurement(request, facts);
````

## 第 3 步：PersonalTransferStorage（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 56 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>repository.saveStorageReceipt(c, request, result)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2553:2651 -->
````javascript
      await repository.saveStorageReceipt(c, request, result);
      return result;
    });
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/personal_transfer_storage.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/06-cleanup-storage.md)。
