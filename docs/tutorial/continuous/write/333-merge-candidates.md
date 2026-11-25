# 分步编写 `services/collaboration/src/application/merge_candidates.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/333-merge-candidates.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

固定合并尝试上下文，在线程外计算，再持久化可重放候选或冲突。

计算可能跨撤权或清理时间点，因此前后两次短事务都必须检查来源、绑定和墓碑。

写之前先按这条执行路线理解各段的连接：

1. validateMergeInput核三份固定内容及input_digest
2. 首事务lockSources/lockAttempt，核cleaned并保存MERGE_ATTEMPT_CONTEXT
3. 无结果时executor纯计算，成功正文生成hash/resources/provenance
4. 次事务重验来源/绑定/cleaned后saveResult；并发已有结果则重放

本文件需要守住的边界：

- 不能持SQL锁等待worker
- 上下文已存但结果未存时，由Go MERGE_RECOVER重送同哈希输入；不是把登记成功当结果成功

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/merge_candidates.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/merge_candidates.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/merge_candidates.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/merge_candidates.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:534 -->
````javascript
import { canonical, resourceIds } from '../../../../packages/content/index.mjs';
import * as repository from '../repository/candidate_artifacts.mjs';
import { validateMergeInput } from '../domain/merge_protocol.mjs';
import { captureProvenance, storeContent, wireContent } from '../domain/capture_protocol.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { lockSources } from './admission_gates.mjs';

````

## 第 2 步：MergeCandidates

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`export class MergeCandidates `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>executor</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.executor = executor</code> |

**本段进入 MergeCandidates.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>executor</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.executor = executor</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 MergeCandidates.compute：**

**这一段为什么这样写：** 把可重试计算与不可变持久候选衔接。

1. 固定输入绑定
2. 离开SQL事务做纯计算
3. 重新取得原业务锁并验证资格
4. 保存唯一结果


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 带attempt_id、input_digest、固定三正文和主线cut的请求<br>在此声明中的实际使用：<code>validateMergeInput(input)</code>；<code>input.context</code>；<code>input.attempt_id</code>；<code>input.input_digest</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.reply(prior)</code>；<code>this.reply(committed)</code> |

**跟着本段代码走一遍：**

- 等待 <code>this.transactions.run(async (connection) =&gt; {       await lockSources(connection, key.sources);       const row = await repository.lockAttempt(connection, key.operation_id, input.a …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>lockSources(connection, key.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.lockAttempt(connection, key.operation_id, input.attempt_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>row.cleaned</code>。条件成立时执行 <code>throw new RPCFailure(9, &#x27;MERGE_CANDIDATE_CLEANED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(9, &#x27;MERGE_CANDIDATE_CLEANED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>row.context</code>。
- 在本段的内部函数内：检查 <code>canonical(json(row.context.payload)) !== canonical(fixed.binding)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MERGE_ATTEMPT_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;MERGE_ATTEMPT_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>repository.saveContext(connection, row.business, fixed.binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return row.result ? json(row.result.payload) : null;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>prior</code>。条件成立时执行 <code>return this.reply(prior);</code>。当前函数在这里结束。
- 等待 <code>this.executor.compute(       Object.fromEntries(Object.entries(fixed.content).map((&#91;key, value&#93;) =&gt; &#91;key, value.body&#93;)),     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>result.body</code>。

<!-- write-step: 2 bytes=534:2917 -->
````javascript
export class MergeCandidates {
  constructor(transactions, executor) {
    this.transactions = transactions;
    this.executor = executor;
  }

  async compute(input) {
    const fixed = validateMergeInput(input),
      key = input.context;
    const prior = await this.transactions.run(async (connection) => {
      await lockSources(connection, key.sources);
      const row = await repository.lockAttempt(connection, key.operation_id, input.attempt_id);
      if (row.cleaned) throw new RPCFailure(9, 'MERGE_CANDIDATE_CLEANED');
      if (row.context) {
        if (canonical(json(row.context.payload)) !== canonical(fixed.binding))
          throw new RPCFailure(6, 'MERGE_ATTEMPT_CONFLICT');
      } else await repository.saveContext(connection, row.business, fixed.binding);
      return row.result ? json(row.result.payload) : null;
    });
    if (prior) return this.reply(prior);
    // FIXED上下文登记已完成，任务state=SUCCEEDED表达登记事实。只有RESULT表示计算完成；
    // 此处崩溃由Go的持久MERGE_RECOVER重送相同哈希的三份输入，不需要Node保存三份大正文。
    const result = await this.executor.compute(
      Object.fromEntries(Object.entries(fixed.content).map(([key, value]) => [key, value.body])),
    );
    const stored = {
      state: 'FIXED',
      input_digest: Buffer.from(input.input_digest).toString('base64'),
      conflicts: result.conflicts,
    };
    if (result.body) {
      const bytes = Buffer.from(canonical(result.body)),
        provenance = fixed.content.submitted.provenance;
      const content = {
        ...input.submitted,
        capture_id: key.operation_id + '-merge-' + input.attempt_id,
        operation_id: key.operation_id,
        body_json_utf8: bytes,
        body_hash: hash(bytes),
        byte_size: String(bytes.length),
        resource_ids: resourceIds(result.body),
        provenance_json_utf8: Buffer.from(
          canonical(
            captureProvenance(
              result.body,
              {
                body: fixed.content.base.body,
                lineage_id: input.submitted.lineage_ref,
                base_manifest_id: provenance.base_manifest_id,
                base_version_id: provenance.base_content_version_id,
                provenance,
              },
              key.operation_id,
            ),
          ),
        ),
      };
````

## 第 3 步：MergeCandidates（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 67 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 MergeCandidates.reply：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>stored</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>stored.state</code>；<code>stored.input_digest</code>；<code>stored.conflicts</code>；<code>stored.candidate</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{       state: stored.state,       input_digest: Buffer.from(stored.input_digest, &#x27;base64&#x27;),       conflicts: stored.conflicts,       ...(stored.candidate ? { candidate: wireContent(stored.candidate) } : {}),     }</code> |

**跟着本段代码走一遍：**

- 等待 <code>this.transactions.run(async (connection) =&gt; {       await lockSources(connection, key.sources);       const row = await repository.lockAttempt(connection, key.operation_id, input.a …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>lockSources(connection, key.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.lockAttempt(connection, key.operation_id, input.attempt_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>row.cleaned</code>。条件成立时执行 <code>throw new RPCFailure(9, &#x27;MERGE_CANDIDATE_CLEANED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(9, &#x27;MERGE_CANDIDATE_CLEANED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>canonical(json(row.context.payload)) !== canonical(fixed.binding)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;MERGE_ATTEMPT_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;MERGE_ATTEMPT_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>row.result</code>。条件成立时执行 <code>return json(row.result.payload);</code>。当前函数在这里结束。
- 在本段的内部函数内：等待 <code>repository.saveResult(connection, row.business, stored)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return stored;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.reply(committed);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return {       state: stored.state,       input_digest: Buffer.from(stored.input_digest, &#x27;base64&#x27;),       conflicts: stored.conflicts,       ...(stored.candidate ? { candidate: wir …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2917:3845 -->
````javascript
      stored.candidate = storeContent(content);
    }
    const committed = await this.transactions.run(async (connection) => {
      await lockSources(connection, key.sources);
      const row = await repository.lockAttempt(connection, key.operation_id, input.attempt_id);
      if (row.cleaned) throw new RPCFailure(9, 'MERGE_CANDIDATE_CLEANED');
      if (canonical(json(row.context.payload)) !== canonical(fixed.binding))
        throw new RPCFailure(6, 'MERGE_ATTEMPT_CONFLICT');
      if (row.result) return json(row.result.payload);
      await repository.saveResult(connection, row.business, stored);
      return stored;
    });
    return this.reply(committed);
  }

  reply(stored) {
    return {
      state: stored.state,
      input_digest: Buffer.from(stored.input_digest, 'base64'),
      conflicts: stored.conflicts,
      ...(stored.candidate ? { candidate: wireContent(stored.candidate) } : {}),
    };
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/merge_candidates.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
