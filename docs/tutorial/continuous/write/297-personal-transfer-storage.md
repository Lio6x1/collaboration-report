# 分步编写 `services/collaboration/src/domain/personal_transfer_storage.mjs`

先按 [C06 · 固定候选清理、账号清理与转移计量](../lessons/collaboration/06-cleanup-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/297-personal-transfer-storage.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

验证个人空间转移的两条关闭来源，并用持久行字节统计生成计量证明。

转移容量必须基于关闭后的稳定存储事实；JS Number求和会丢uint64精度。

写之前先按这条执行路线理解各段的连接：

1. transferStorageRequest只接受该文档doc-acl/publication两条独立撤权，排序形成固定请求
2. transferStorageMeasurement累加checkpoints/updates/captures bytes为BigInt
3. 核uint64溢出后返回storage_bytes和request+facts的snapshot_digest

本文件需要守住的边界：

- 不能统计可见正文长度冒充数据库已占用字节
- 两条来源不得重复或共用revocation_id

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/personal_transfer_storage.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/personal_transfer_storage.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/personal_transfer_storage.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/personal_transfer_storage.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:97 -->
````javascript
import { createHash } from 'node:crypto';
import { bad, validID } from './shared_protocol.mjs';

````

## 第 2 步：uint64

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

本步声明：`const uint64 = (value, zero = false) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof value === &#x27;string&#x27;</code>；<code>(zero ? /^(0&#124;&#91;1-9&#93;&#91;0-9&#93;{0,19})$/ : /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/).test(value)</code>；<code>BigInt(value)</code> |
| <code>zero = false</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>(zero ? /^(0&#124;&#91;1-9&#93;&#91;0-9&#93;{0,19})$/ : /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/).test</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>typeof value === &#x27;string&#x27; &amp;&amp;   (zero ? /^(0&#124;&#91;1-9&#93;&#91;0-9&#93;{0,19})$/ : /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/).test(value) &amp;&amp;   BigInt(value) &lt;= 18446744073709551615n</code> |

<!-- write-step: 2 bytes=97:285 -->
````javascript
const uint64 = (value, zero = false) =>
  typeof value === 'string' &&
  (zero ? /^(0|[1-9][0-9]{0,19})$/ : /^[1-9][0-9]{0,19}$/).test(value) &&
  BigInt(value) <= 18446744073709551615n;

````

## 第 3 步：transferStorageRequest

插入位置：第 2 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`export function transferStorageRequest(input) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>!input &#124;&#124;     !validID(input.operation_id)</code>；<code>input.operation_id</code>；<code>input.document_id</code>；<code>input.generation</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     operation_id: input.operation_id,     document_id: input.document_id,     generation: &#x27;1&#x27;,     closures,   }</code> |

**跟着本段代码走一遍：**

- 检查 <code>!input &#124;&#124;     !validID(input.operation_id) &#124;&#124;     !uint64(input.document_id) &#124;&#124;     input.generation !== &#x27;1&#x27; &#124;&#124;     !Array.isArray(input.closures) &#124;&#124;     input.closures.length !==  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>!s &#124;&#124;         !validID(s.revocation_id) &#124;&#124;         !uint64(s.epoch) &#124;&#124;         !&#91;&#x27;doc-acl:&#x27; + input.document_id, &#x27;publication:&#x27; + input.document_id&#93;.includes(           s.scope_key …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return { revocation_id: s.revocation_id, scope_key: s.scope_key, epoch: s.epoch };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>closures&#91;0&#93;.scope_key === closures&#91;1&#93;.scope_key &#124;&#124;     closures&#91;0&#93;.revocation_id === closures&#91;1&#93;.revocation_id</code>。条件成立时执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {     operation_id: input.operation_id,     document_id: input.document_id,     generation: &#x27;1&#x27;,     closures,   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=285:1595 -->
````javascript
export function transferStorageRequest(input) {
  if (
    !input ||
    !validID(input.operation_id) ||
    !uint64(input.document_id) ||
    input.generation !== '1' ||
    !Array.isArray(input.closures) ||
    input.closures.length !== 2 ||
    Object.keys(input).some(
      (key) => !['operation_id', 'document_id', 'generation', 'closures'].includes(key),
    )
  )
    throw bad('PERSONAL_TRANSFER_STORAGE_INVALID');
  const closures = input.closures
    .map((s) => {
      if (
        !s ||
        !validID(s.revocation_id) ||
        !uint64(s.epoch) ||
        !['doc-acl:' + input.document_id, 'publication:' + input.document_id].includes(
          s.scope_key,
        ) ||
        Object.keys(s).some((key) => !['revocation_id', 'scope_key', 'epoch'].includes(key))
      )
        throw bad('PERSONAL_TRANSFER_STORAGE_INVALID');
      return { revocation_id: s.revocation_id, scope_key: s.scope_key, epoch: s.epoch };
    })
    .sort((a, b) => a.scope_key.localeCompare(b.scope_key));
  if (
    closures[0].scope_key === closures[1].scope_key ||
    closures[0].revocation_id === closures[1].revocation_id
  )
    throw bad('PERSONAL_TRANSFER_STORAGE_INVALID');
  return {
    operation_id: input.operation_id,
    document_id: input.document_id,
    generation: '1',
    closures,
  };
}

````

## 第 4 步：transferStorageMeasurement

插入位置：第 3 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`export function transferStorageMeasurement(request, facts) `。

**这一段为什么这样写：** 把稳定SQL聚合结果变为可幂等重放的容量回执。

1. 核各组bytes规范整数
2. BigInt累加并检查上界
3. 摘要绑定请求与全部计量事实


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>request</code> | <code>JavaScript 参数</code> | 规范化双来源关闭请求<br>在此声明中的实际使用：<code>request.operation_id</code>；<code>request.document_id</code>；<code>JSON.stringify({ request, facts })</code> |
| <code>facts</code> | <code>JavaScript 参数</code> | 按room排序的房间、检查点、更新和候选SQL聚合<br>在此声明中的实际使用：<code>facts.checkpoints</code>；<code>facts.updates</code>；<code>facts.captures</code>；<code>JSON.stringify({ request, facts })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     operation_id: request.operation_id,     document_id: request.document_id,     generation: &#x27;1&#x27;,     storage_bytes: bytes.toString(),     snapshot_digest: createHash(&#x27;sha256&#x27;). …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>!uint64(String(row.bytes), true)</code>。条件成立时执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_OVERFLOW&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_OVERFLOW&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>bytes &gt; 18446744073709551615n</code>。条件成立时执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_OVERFLOW&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;PERSONAL_TRANSFER_STORAGE_OVERFLOW&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {     operation_id: request.operation_id,     document_id: request.document_id,     generation: &#x27;1&#x27;,     storage_bytes: bytes.toString(),     snapshot_digest: createHash(&#x27;sh …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1595:2390 -->
````javascript
export function transferStorageMeasurement(request, facts) {
  let bytes = 0n;
  // 只累加真实持久行的字节；编号/容量均保持uint64字符串，不把SUM或room ID转Number。
  for (const group of [facts.checkpoints, facts.updates, facts.captures])
    for (const row of group) {
      if (!uint64(String(row.bytes), true)) throw bad('PERSONAL_TRANSFER_STORAGE_OVERFLOW');
      bytes += BigInt(row.bytes);
    }
  if (bytes > 18446744073709551615n) throw bad('PERSONAL_TRANSFER_STORAGE_OVERFLOW');
  return {
    operation_id: request.operation_id,
    document_id: request.document_id,
    generation: '1',
    storage_bytes: bytes.toString(),
    snapshot_digest: createHash('sha256').update(JSON.stringify({ request, facts })).digest('hex'),
    source_closed: true,
  };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/personal_transfer_storage.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/06-cleanup-storage.md)。
