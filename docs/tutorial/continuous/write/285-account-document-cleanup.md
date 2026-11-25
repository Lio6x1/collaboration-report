# 分步编写 `services/collaboration/src/domain/account_document_cleanup.mjs`

先按 [C06 · 固定候选清理、账号清理与转移计量](../lessons/collaboration/06-cleanup-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/285-account-document-cleanup.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

验证单文档清理请求及Doc给出的关闭计划摘要。

清理必须被精确文档/原操作/撤权代次约束，不能凭一个document_id删除数据。

写之前先按这条执行路线理解各段的连接：

1. cleanupRequest仅允许operation_id/document_id
2. cleanupAuthority重算ACCOUNT_DOCUMENT_CLEANUP摘要，核doc-acl来源、closed_epoch与revocation_id
3. cleanupReply去掉仅内部路由使用的cleanup_kind，保持既有回执字节语义

本文件需要守住的边界：

- 未知额外请求字段应拒绝，避免清理范围被悄悄扩大
- PURGE类型只能来自已验证Doc authority，不是浏览器选择

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/account_document_cleanup.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/account_document_cleanup.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/account_document_cleanup.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/account_document_cleanup.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:97 -->
````javascript
import { createHash } from 'node:crypto';
import { validID, bad } from './shared_protocol.mjs';

````

## 第 2 步：canonicalID

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

本步声明：`const canonicalID = (value) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof value === &#x27;string&#x27;</code>；<code>/^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(value)</code>；<code>BigInt(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>typeof value === &#x27;string&#x27; &amp;&amp;   /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(value) &amp;&amp;   BigInt(value) &lt;= 18446744073709551615n</code> |

<!-- write-step: 2 bytes=97:240 -->
````javascript
const canonicalID = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;

````

## 第 3 步：cleanupRequest

插入位置：第 2 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`export function cleanupRequest(input) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>!input &#124;&#124;     !validID(input.operation_id)</code>；<code>input.operation_id</code>；<code>input.document_id</code>；<code>Object.keys(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ operation_id: input.operation_id, document_id: input.document_id }</code> |

**跟着本段代码走一遍：**

- 检查 <code>!input &#124;&#124;     !validID(input.operation_id) &#124;&#124;     !canonicalID(input.document_id) &#124;&#124;     Object.keys(input).some((key) =&gt; !&#91;&#x27;operation_id&#x27;, &#x27;document_id&#x27;&#93;.includes(key))</code>。条件成立时执行 <code>throw bad(&#x27;ACCOUNT_DOCUMENT_CLEANUP_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;ACCOUNT_DOCUMENT_CLEANUP_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return { operation_id: input.operation_id, document_id: input.document_id };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=240:598 -->
````javascript
export function cleanupRequest(input) {
  if (
    !input ||
    !validID(input.operation_id) ||
    !canonicalID(input.document_id) ||
    Object.keys(input).some((key) => !['operation_id', 'document_id'].includes(key))
  )
    throw bad('ACCOUNT_DOCUMENT_CLEANUP_INVALID');
  return { operation_id: input.operation_id, document_id: input.document_id };
}

````

## 第 4 步：cleanupAuthority

插入位置：第 3 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`export function cleanupAuthority(input, request) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input?.scope_key</code>；<code>input?.closed_epoch</code>；<code>input?.revocation_id</code>；<code>input?.cleanup_kind</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>request.operation_id</code>；<code>request.document_id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>input</code> |

**跟着本段代码走一遍：**

- 检查 <code>(input?.cleanup_kind !== undefined &amp;&amp; input.cleanup_kind !== &#x27;PURGE&#x27;) &#124;&#124;     input?.operation_id !== request.operation_id &#124;&#124;     input?.document_id !== request.document_id &#124;&#124;     i …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;ACCOUNT_DOCUMENT_CLEANUP_AUTHORITY_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;ACCOUNT_DOCUMENT_CLEANUP_AUTHORITY_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return input;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=598:1486 -->
````javascript
export function cleanupAuthority(input, request) {
  const expected = createHash('sha256')
    .update(
      [
        'ACCOUNT_DOCUMENT_CLEANUP',
        request.operation_id,
        request.document_id,
        input?.scope_key,
        input?.closed_epoch,
        input?.revocation_id,
      ].join('\n'),
    )
    .digest('hex');
  if (
    (input?.cleanup_kind !== undefined && input.cleanup_kind !== 'PURGE') ||
    input?.operation_id !== request.operation_id ||
    input?.document_id !== request.document_id ||
    input?.scope_key !== 'doc-acl:' + request.document_id ||
    !canonicalID(input?.closed_epoch) ||
    !validID(input?.revocation_id) ||
    input?.plan_digest !== expected
  )
    throw bad('ACCOUNT_DOCUMENT_CLEANUP_AUTHORITY_INVALID');
  return input;
}

// cleanup_kind仅是已验Doc authority的内部业务分派，不进入冻结清理回执的字节。
````

## 第 5 步：cleanupReply

插入位置：第 4 步代码之后，紧接追加；完成稿约第 47 行。

Account没有此字段，历史回执保持原形状与原摘要。

本步声明：`export const cleanupReply = (context) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>context</code> | <code>JavaScript 参数</code> | 已验权威清理上下文；去掉仅内部调度使用的cleanup_kind，原固定绑定原样进入CLEANED回执。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ ...fixed, state: &#x27;CLEANED&#x27; }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return { ...fixed, state: &#x27;CLEANED&#x27; };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1486:1696 -->
````javascript
// Account没有此字段，历史回执保持原形状与原摘要。
export const cleanupReply = (context) => {
  const { cleanup_kind: kind, ...fixed } = context;
  return { ...fixed, state: 'CLEANED' };
};
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/account_document_cleanup.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/06-cleanup-storage.md)。
