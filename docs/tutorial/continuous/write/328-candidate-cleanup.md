# 分步编写 `services/collaboration/src/application/candidate_cleanup.mjs`

先按 [C06 · 固定候选清理、账号清理与转移计量](../lessons/collaboration/06-cleanup-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/328-candidate-cleanup.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

重查Doc CLEANABLE许可后删除冗余送审/合并候选并原子写清理墓碑。

参数里带permit不代表已经获得删除权；未知结果必须保留候选，迟到计算也不能把它重建。

写之前先按这条执行路线理解各段的连接：

1. cleanupBinding核capture/merge引用格式、generation及候选/许可摘要
2. 先查本地成功回执，再锁外AuthorizeCandidateCleanup核真实原许可
3. 与capture/merge同号gate串行，核候选内容hash与已BOUND事实
4. 清正文同时写CANDIDATE_CLEANED固定结果和摘要，重试验证后重放

本文件需要守住的边界：

- RETAIN/UNKNOWN/超时都不能删
- 送审保留BOUND、go_result_ref、cut/hash/lineage元数据供后续合并
- merge计算结束后二次提交必须再次检查墓碑

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/candidate_cleanup.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/candidate_cleanup.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/candidate_cleanup.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/candidate_cleanup.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:452 -->
````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { bad, validID, json } from '../domain/shared_protocol.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import * as repository from '../repository/candidate_artifacts.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';
import * as merges from '../repository/candidate_artifacts.mjs';

````

## 第 2 步：positive

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`const positive = (value) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof value === &#x27;string&#x27;</code>；<code>/^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(value)</code>；<code>BigInt(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>typeof value === &#x27;string&#x27; &amp;&amp;   /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(value) &amp;&amp;   BigInt(value) &lt;= 18446744073709551615n</code> |

<!-- write-step: 2 bytes=452:591 -->
````javascript
const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;
````

## 第 3 步：cleanupBinding

插入位置：第 2 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`export function cleanupBinding(input) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input?.binding</code>；<code>input?.permit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     operation_id: request.operation_id,     generation: request.generation,     candidate_ref: request.candidate_ref,     candidate_digest: Buffer.from(request.candidate_digest). …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>!validID(request?.operation_id) &#124;&#124;     !positive(request?.generation) &#124;&#124;     Buffer.from(request?.candidate_digest ?? &#91;&#93;).length !== 32 &#124;&#124;     !permit &#124;&#124;     permit.operation_id != …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;CANDIDATE_CLEANUP_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;CANDIDATE_CLEANUP_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!(parts.length === 2 &amp;&amp; parts&#91;0&#93; === &#x27;capture&#x27; &amp;&amp; parts&#91;1&#93; === request.operation_id) &amp;&amp;     !(       parts.length === 3 &amp;&amp;       parts&#91;0&#93; === &#x27;merge&#x27; &amp;&amp;       parts&#91;1&#93; === request. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;CANDIDATE_REFERENCE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;CANDIDATE_REFERENCE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {     operation_id: request.operation_id,     generation: request.generation,     candidate_ref: request.candidate_ref,     candidate_digest: Buffer.from(request.candidate_d …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=591:1748 -->
````javascript
export function cleanupBinding(input) {
  const request = input?.binding,
    permit = input?.permit;
  if (
    !validID(request?.operation_id) ||
    !positive(request?.generation) ||
    Buffer.from(request?.candidate_digest ?? []).length !== 32 ||
    !permit ||
    permit.operation_id !== request.operation_id ||
    permit.generation !== request.generation ||
    permit.state !== 'CLEANABLE' ||
    Buffer.from(permit.permit_digest ?? []).length !== 32
  )
    throw bad('CANDIDATE_CLEANUP_INVALID');
  const parts = request.candidate_ref?.split(':') ?? [];
  if (
    !(parts.length === 2 && parts[0] === 'capture' && parts[1] === request.operation_id) &&
    !(
      parts.length === 3 &&
      parts[0] === 'merge' &&
      parts[1] === request.operation_id &&
      parts[2] === request.generation
    )
  )
    throw bad('CANDIDATE_REFERENCE_INVALID');
  return {
    operation_id: request.operation_id,
    generation: request.generation,
    candidate_ref: request.candidate_ref,
    candidate_digest: Buffer.from(request.candidate_digest).toString('base64'),
    permit_digest: Buffer.from(permit.permit_digest).toString('base64'),
  };
}

````

## 第 4 步：replay

插入位置：第 3 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`function replay(row, binding) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>row.payload</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>canonical(binding)</code>；<code>binding.operation_id</code>；<code>binding.generation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>null</code>；<code>{     ...result,     fixed_result_json_utf8: Buffer.from(result.fixed_result_json_utf8, &#x27;base64&#x27;),     result_digest: Buffer.from(result.result_digest, &#x27;base64&#x27;),   }</code> |

**跟着本段代码走一遍：**

- 检查 <code>!row</code>。条件成立时执行 <code>return null;</code>。当前函数在这里结束。
- 检查 <code>canonical(saved.binding) !== canonical(binding)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;CANDIDATE_BINDING_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;CANDIDATE_BINDING_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>result?.operation_id !== binding.operation_id &#124;&#124;     result.generation !== binding.generation &#124;&#124;     result.state !== &#x27;SUCCEEDED&#x27; &#124;&#124;     !result.fixed_result_json_utf8 &#124;&#124;     !resu …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(14, &#x27;CANDIDATE_CLEANUP_RECEIPT_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;CANDIDATE_CLEANUP_RECEIPT_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {     ...result,     fixed_result_json_utf8: Buffer.from(result.fixed_result_json_utf8, &#x27;base64&#x27;),     result_digest: Buffer.from(result.result_digest, &#x27;base64&#x27;),   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1748:2575 -->
````javascript
function replay(row, binding) {
  if (!row) return null;
  const saved = json(row.payload);
  if (canonical(saved.binding) !== canonical(binding))
    throw new RPCFailure(6, 'CANDIDATE_BINDING_CHANGED');
  const result = saved.result;
  if (
    result?.operation_id !== binding.operation_id ||
    result.generation !== binding.generation ||
    result.state !== 'SUCCEEDED' ||
    !result.fixed_result_json_utf8 ||
    !result.result_digest ||
    !hash(Buffer.from(result.fixed_result_json_utf8, 'base64')).equals(
      Buffer.from(result.result_digest, 'base64'),
    )
  )
    throw new RPCFailure(14, 'CANDIDATE_CLEANUP_RECEIPT_UNKNOWN');
  return {
    ...result,
    fixed_result_json_utf8: Buffer.from(result.fixed_result_json_utf8, 'base64'),
    result_digest: Buffer.from(result.result_digest, 'base64'),
  };
}

````

## 第 5 步：CandidateCleanup

插入位置：第 4 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`export class CandidateCleanup `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>peers</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |

**本段进入 CandidateCleanup.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 CandidateCleanup.clean：**

**这一段为什么这样写：** 把许可核验、删除和防重生事实连成可恢复清理。

1. 验证请求并查原回执
2. 查Doc真实许可
3. 锁原候选业务键并核固定字节
4. 删除冗余正文并同事务保存墓碑


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | binding与Doc许可摘要组成的清理请求<br>在此声明中的实际使用：<code>cleanupBinding(input)</code>；<code>input.binding</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>prior</code>；<code>this.transactions.run(async (connection) =&gt; {       const parts = binding.candidate_ref.split(&#x27;:&#x27;);       // 与Capture/Compute相同的原号gate串行，CPU计算期间不持SQL锁；       // 清理墓碑和删除同事务，迟到的第二次计算 …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 等待 <code>this.transactions.run(async (connection) =&gt;       replay(await repository.findCleanup(connection, binding.candidate_ref), binding),     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.findCleanup(connection, binding.candidate_ref)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>prior</code>。条件成立时执行 <code>return prior;</code>。当前函数在这里结束。
- 等待 <code>this.peers.rpc(&#x27;AuthorizeCandidateCleanup&#x27;, input.binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>permit.state !== &#x27;CLEANABLE&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;CANDIDATE_CLEANUP_NOT_PROVEN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;CANDIDATE_CLEANUP_NOT_PROVEN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>permit.operation_id !== binding.operation_id &#124;&#124;       permit.generation !== binding.generation &#124;&#124;       Buffer.from(permit.permit_digest ?? &#91;&#93;).toString(&#x27;base64&#x27;) !== binding.permit_digest</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;CANDIDATE_PERMIT_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;CANDIDATE_PERMIT_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return this.transactions.run(async (connection) =&gt; {       const parts = binding.candidate_ref.split(&#x27;:&#x27;);       // 与Capture/Compute相同的原号gate串行，CPU计算期间不持SQL锁；       // 清理墓碑和删除同事务，迟 …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>captures.lockOperation(connection, binding.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>merges.lockAttempt(connection, binding.operation_id, parts&#91;2&#93;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>repository.findCleanup(connection, binding.candidate_ref)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>completed</code>。条件成立时执行 <code>return completed;</code>。当前函数在这里结束。
- 在本段的内部函数内：检查 <code>parts&#91;0&#93; === &#x27;capture&#x27;</code>。
- 在本段的内部函数内：等待 <code>this.cleanCapture(connection, row, binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>this.cleanMerge(connection, row, binding)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 5 bytes=2575:4631 -->
````javascript
export class CandidateCleanup {
  constructor(transactions, peers) {
    this.transactions = transactions;
    this.peers = peers;
  }

  async clean(input) {
    const binding = cleanupBinding(input);
    const prior = await this.transactions.run(async (connection) =>
      replay(await repository.findCleanup(connection, binding.candidate_ref), binding),
    );
    if (prior) return prior;
    // 调用参数不授予删除权。锁外重查Doc持久原许可，超时/RETAIN/UNKNOWN均保留副本。
    const permit = await this.peers.rpc('AuthorizeCandidateCleanup', input.binding);
    if (permit.state !== 'CLEANABLE') throw new RPCFailure(14, 'CANDIDATE_CLEANUP_NOT_PROVEN');
    if (
      permit.operation_id !== binding.operation_id ||
      permit.generation !== binding.generation ||
      Buffer.from(permit.permit_digest ?? []).toString('base64') !== binding.permit_digest
    )
      throw new RPCFailure(6, 'CANDIDATE_PERMIT_CHANGED');
    return this.transactions.run(async (connection) => {
      const parts = binding.candidate_ref.split(':');
      // 与Capture/Compute相同的原号gate串行，CPU计算期间不持SQL锁；
      // 清理墓碑和删除同事务，迟到的第二次计算提交必须再次读墓碑。
      const row =
        parts[0] === 'capture'
          ? (await captures.lockOperation(connection, binding.operation_id))[0][0]
          : await merges.lockAttempt(connection, binding.operation_id, parts[2]);
      const completed = replay(
        await repository.findCleanup(connection, binding.candidate_ref),
        binding,
      );
      if (completed) return completed;
      if (parts[0] === 'capture') await this.cleanCapture(connection, row, binding);
      else await this.cleanMerge(connection, row, binding);
      const fixed = Buffer.from(
        canonical({
          operation_id: binding.operation_id,
          state: 'SUCCEEDED',
          cleanup_state: 'CLEANED',
          candidate_ref: binding.candidate_ref,
          generation: binding.generation,
        }),
      );
````

## 第 6 步：CandidateCleanup（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 116 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 CandidateCleanup.cleanCapture：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>repository.cleanCaptureBody(connection, binding.operation_id, candidate)</code> |
| <code>row</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>!row &#124;&#124;       row.state !== &#x27;BOUND&#x27;</code>；<code>row.state</code>；<code>row.generation</code>；<code>row.body_hash</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.generation</code>；<code>binding.candidate_digest</code>；<code>binding.operation_id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**本段进入 CandidateCleanup.cleanMerge：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>repository.cleanMergeBody(connection, row.business)</code> |
| <code>row</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>row.context</code>；<code>row.result</code>；<code>row.business</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.generation</code>；<code>binding.operation_id</code>；<code>binding.candidate_digest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>repository.saveCleanup(connection, binding, stored)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return replay({ payload: { binding, result: stored } }, binding);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!row &#124;&#124;       row.state !== &#x27;BOUND&#x27; &#124;&#124;       String(row.generation) !== binding.generation &#124;&#124;       Buffer.from(row.body_hash ?? &#91;&#93;).toString(&#x27;base64&#x27;) !== binding.candidate_digest &#124;&#124;       !row.go_result_ref</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_CLEANUP_NOT_PROVEN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;CAPTURE_CLEANUP_NOT_PROVEN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>candidate.binding?.guard_generation !== binding.generation &#124;&#124;       candidate.content?.body_hash !== binding.candidate_digest &#124;&#124;       !hash(Buffer.from(candidate.content.body_json …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;CAPTURE_CANDIDATE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;CAPTURE_CANDIDATE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>repository.cleanCaptureBody(connection, binding.operation_id, candidate)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!context &#124;&#124;       !result?.candidate &#124;&#124;       context.attempt_id !== binding.generation &#124;&#124;       context.context?.operation_id !== binding.operation_id &#124;&#124;       result.candidate.bo …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(14, &#x27;MERGE_CLEANUP_NOT_PROVEN&#x27;);</code>。转入外层异常处理路径。

<!-- write-step: 6 bytes=4631:6764 -->
````javascript
      const stored = {
        state: 'SUCCEEDED',
        operation_id: binding.operation_id,
        generation: binding.generation,
        terminal_revision: binding.generation,
        fixed_result_json_utf8: fixed.toString('base64'),
        result_digest: hash(fixed).toString('base64'),
      };
      await repository.saveCleanup(connection, binding, stored);
      return replay({ payload: { binding, result: stored } }, binding);
    });
  }

  async cleanCapture(connection, row, binding) {
    if (
      !row ||
      row.state !== 'BOUND' ||
      String(row.generation) !== binding.generation ||
      Buffer.from(row.body_hash ?? []).toString('base64') !== binding.candidate_digest ||
      !row.go_result_ref
    )
      throw new RPCFailure(14, 'CAPTURE_CLEANUP_NOT_PROVEN');
    const candidate = json(row.candidate_json);
    if (
      candidate.binding?.guard_generation !== binding.generation ||
      candidate.content?.body_hash !== binding.candidate_digest ||
      !hash(Buffer.from(candidate.content.body_json_utf8 ?? '', 'base64')).equals(
        Buffer.from(row.body_hash),
      )
    )
      throw new RPCFailure(6, 'CAPTURE_CANDIDATE_CHANGED');
    // 后续合并仍使用原送审cut/hash/lineage元数据；只清冗余正文及完整旧lineage副本。
    // 保留BOUND和go_result_ref，不能把已提交送审变成新的失败业务操作。
    delete candidate.content.body_json_utf8;
    delete candidate.lineage;
    candidate.cleaned = true;
    await repository.cleanCaptureBody(connection, binding.operation_id, candidate);
  }

  async cleanMerge(connection, row, binding) {
    const context = row.context && json(row.context.payload),
      result = row.result && json(row.result.payload);
    if (
      !context ||
      !result?.candidate ||
      context.attempt_id !== binding.generation ||
      context.context?.operation_id !== binding.operation_id ||
      result.candidate.body_hash !== binding.candidate_digest ||
      !hash(Buffer.from(result.candidate.body_json_utf8 ?? '', 'base64')).equals(
        Buffer.from(binding.candidate_digest, 'base64'),
      )
    )
````

## 第 7 步：CandidateCleanup（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 168 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 执行 <code>throw new RPCFailure(14, &#x27;MERGE_CLEANUP_NOT_PROVEN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>repository.cleanMergeBody(connection, row.business)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 7 bytes=6764:6893 -->
````javascript
      throw new RPCFailure(14, 'MERGE_CLEANUP_NOT_PROVEN');
    await repository.cleanMergeBody(connection, row.business);
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/candidate_cleanup.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/06-cleanup-storage.md)。
