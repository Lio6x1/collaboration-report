# 分步编写 `services/collaboration/src/domain/merge_protocol.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/296-merge-protocol.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

验证三份固定内容及主线cut，并计算合并输入绑定摘要。

调用方说“这三份正文属于同一操作”不够，必须核对目标、版本、字节哈希、来源证明与cut。

写之前先按这条执行路线理解各段的连接：

1. fixedContentBinding保留目标和版本身份，以cut/provenance字节摘要绑定大字段
2. mergeBinding排序来源及rooms，固定attempt/context/base/submitted/main/main_cut
3. fixedBody核字节数、SHA-256、规范正文和provenance schema
4. validateMergeInput拒绝重复room或非法cut，再比较input_digest

本文件需要守住的边界：

- base/submitted/main必须是固定正文，不可恢复时重新取当前版本
- 摘要需绑定资源列表、provenance和cut，不能只hash可见文字

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/merge_protocol.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/merge_protocol.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/merge_protocol.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/merge_protocol.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:276 -->
````javascript
import { readFileSync } from 'node:fs';
import Ajv from 'ajv/dist/2020.js';
import { canonical, normalizeContent, resourceIds } from '../../../../packages/content/index.mjs';
import { hash } from './shared_protocol.mjs';
import { bad, validID } from './shared_protocol.mjs';

````

## 第 2 步：provenanceSchema

插入位置：第 1 步代码之后，紧接追加；完成稿约第 7 行。

本步声明：`const provenanceSchema = JSON.parse(`。

<!-- write-step: 2 bytes=276:414 -->
````javascript
const provenanceSchema = JSON.parse(
  readFileSync(new URL('../../../../contracts/content/provenance.schema.json', import.meta.url)),
);
````

## 第 3 步：validateProvenance

插入位置：第 2 步代码之后，紧接追加；完成稿约第 10 行。

oneOf/not的required引用父层已声明属性；关闭仅该编译告警，实际required/oneOf/未知字段校验不变。

本步声明：`const validateProvenance = new Ajv({ strict: true, strictRequired: false }).compile(`。

<!-- write-step: 3 bytes=414:651 -->
````javascript
// oneOf/not的required引用父层已声明属性；关闭仅该编译告警，实际required/oneOf/未知字段校验不变。
const validateProvenance = new Ajv({ strict: true, strictRequired: false }).compile(
  provenanceSchema,
);
````

## 第 4 步：positive

插入位置：第 3 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`const positive = (value) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof value === &#x27;string&#x27;</code>；<code>/^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(value)</code>；<code>BigInt(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>typeof value === &#x27;string&#x27; &amp;&amp;   /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(value) &amp;&amp;   BigInt(value) &lt;= 18446744073709551615n</code> |

<!-- write-step: 4 bytes=651:790 -->
````javascript
const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;
````

## 第 5 步：binary

插入位置：第 4 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`const binary = (value) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>value ?? &#91;&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>Buffer.from(value ?? &#91;&#93;)</code> |

<!-- write-step: 5 bytes=790:842 -->
````javascript
const binary = (value) => Buffer.from(value ?? []);
````

## 第 6 步：binaryHash

插入位置：第 5 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`const binaryHash = (value) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binary(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>hash(binary(value)).toString(&#x27;base64&#x27;)</code> |

<!-- write-step: 6 bytes=842:912 -->
````javascript
const binaryHash = (value) => hash(binary(value)).toString('base64');
````

## 第 7 步：targetKeys

插入位置：第 6 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`const targetKeys = ['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'];`。

<!-- write-step: 7 bytes=912:1007 -->
````javascript
const targetKeys = ['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'];
````

## 第 8 步：fixedContentBinding

插入位置：第 7 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`export function fixedContentBinding(content) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>content</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>content&#91;key&#93;</code>；<code>content.capture_id</code>；<code>content.operation_id</code>；<code>content.lineage_ref</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     ...Object.fromEntries(targetKeys.map((key) =&gt; &#91;key, content&#91;key&#93; ?? &#x27;0&#x27;&#93;)),     capture_id: content.capture_id ?? &#x27;&#x27;,     operation_id: content.operation_id ?? &#x27;&#x27;,     lineag …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return {     ...Object.fromEntries(targetKeys.map((key) =&gt; &#91;key, content&#91;key&#93; ?? &#x27;0&#x27;&#93;)),     capture_id: content.capture_id ?? &#x27;&#x27;,     operation_id: content.operation_id ?? &#x27;&#x27;,     …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1007:1774 -->
````javascript
export function fixedContentBinding(content) {
  return {
    ...Object.fromEntries(targetKeys.map((key) => [key, content[key] ?? '0'])),
    capture_id: content.capture_id ?? '',
    operation_id: content.operation_id ?? '',
    lineage_ref: content.lineage_ref ?? '',
    schema_version: content.schema_version ?? '',
    canonicalization_version: content.canonicalization_version ?? '',
    hash_algorithm: content.hash_algorithm ?? '',
    byte_size: content.byte_size ?? '0',
    body_hash: binary(content.body_hash).toString('base64'),
    cut_digest: binaryHash(content.cut_json_utf8),
    provenance_digest: binaryHash(content.provenance_json_utf8),
    provenance_schema: content.provenance_schema ?? '',
    resource_ids: content.resource_ids ?? [],
  };
}
````

## 第 9 步：mergeBinding

插入位置：第 8 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`export function mergeBinding(input) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.context</code>；<code>input.main_cut</code>；<code>input.attempt_id</code>；<code>input.base</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     attempt_id: input.attempt_id,     context: {       operation_id: context.operation_id,       actor_id: context.actor_id,       identity_admission_ref: context.identity_admiss …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return {     attempt_id: input.attempt_id,     context: {       operation_id: context.operation_id,       actor_id: context.actor_id,       identity_admission_ref: context.identity …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=1774:3068 -->
````javascript
export function mergeBinding(input) {
  const context = input.context,
    cut = input.main_cut;
  return {
    attempt_id: input.attempt_id,
    context: {
      operation_id: context.operation_id,
      actor_id: context.actor_id,
      identity_admission_ref: context.identity_admission_ref,
      document_admission_ref: context.document_admission_ref,
      target: {
        ...Object.fromEntries(targetKeys.map((key) => [key, context.target[key] ?? '0'])),
        purpose: context.target.purpose,
      },
      sources: [...context.sources].sort((a, b) =>
        Buffer.compare(Buffer.from(a.scope_key), Buffer.from(b.scope_key)),
      ),
      request_fingerprint: binary(context.request_fingerprint).toString('base64'),
    },
    base: fixedContentBinding(input.base),
    submitted: fixedContentBinding(input.submitted),
    main: fixedContentBinding(input.main),
    main_cut: {
      document_id: cut.document_id,
      structure_id: cut.structure_id,
      style_id: cut.style_id,
      control_revision: cut.control_revision,
      rooms: [...(cut.rooms ?? [])].sort((a, b) =>
        BigInt(a.room_id) < BigInt(b.room_id) ? -1 : BigInt(a.room_id) > BigInt(b.room_id) ? 1 : 0,
      ),
      dependency_digest: binary(cut.dependency_digest).toString('base64'),
    },
  };
}
````

## 第 10 步：mergeInputDigest

插入位置：第 9 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`export const mergeInputDigest = (input) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>mergeBinding(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>hash(Buffer.from(canonical(mergeBinding(input))))</code> |

<!-- write-step: 10 bytes=3068:3163 -->
````javascript
export const mergeInputDigest = (input) => hash(Buffer.from(canonical(mergeBinding(input))));

````

## 第 11 步：fixedBody

插入位置：第 10 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`function fixedBody(content, target) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>content</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>content.body_json_utf8</code>；<code>content.document_id</code>；<code>content.unit_id</code>；<code>content.structure_id</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>target</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>target.document_id</code>；<code>target.unit_id</code>；<code>target.structure_id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ body, provenance }</code> |

**跟着本段代码走一遍：**

- 检查 <code>content.document_id !== target.document_id &#124;&#124;     content.unit_id !== target.unit_id &#124;&#124;     content.structure_id !== target.structure_id &#124;&#124;     content.schema_version !== &#x27;content- …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;MERGE_FIXED_CONTENT_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MERGE_FIXED_CONTENT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>canonical(body) !== bytes.toString(&#x27;utf8&#x27;) &#124;&#124;       !validateProvenance(provenance) &#124;&#124;       canonical(provenance) !== rawProvenance.toString(&#x27;utf8&#x27;) &#124;&#124;       canonical(resourceIds …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw Error(&#x27;INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return { body, provenance };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>throw bad(&#x27;MERGE_FIXED_CONTENT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 11 bytes=3163:4363 -->
````javascript
function fixedBody(content, target) {
  const bytes = binary(content.body_json_utf8);
  if (
    content.document_id !== target.document_id ||
    content.unit_id !== target.unit_id ||
    content.structure_id !== target.structure_id ||
    content.schema_version !== 'content-v1' ||
    content.canonicalization_version !== 'canonical-v1' ||
    content.hash_algorithm !== 'SHA-256' ||
    content.provenance_schema !== 'provenance-v1' ||
    bytes.length > 16 << 20 ||
    content.byte_size !== String(bytes.length) ||
    !hash(bytes).equals(binary(content.body_hash))
  )
    throw bad('MERGE_FIXED_CONTENT_INVALID');
  try {
    const body = normalizeContent(JSON.parse(bytes), { workspaceKind: 'MAIN' });
    const rawProvenance = binary(content.provenance_json_utf8),
      provenance = JSON.parse(rawProvenance);
    if (
      canonical(body) !== bytes.toString('utf8') ||
      !validateProvenance(provenance) ||
      canonical(provenance) !== rawProvenance.toString('utf8') ||
      canonical(resourceIds(body)) !== canonical(content.resource_ids ?? [])
    )
      throw Error('INVALID');
    return { body, provenance };
  } catch {
    throw bad('MERGE_FIXED_CONTENT_INVALID');
  }
}

````

## 第 12 步：validateMergeInput

插入位置：第 11 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`export function validateMergeInput(input) `。

**这一段为什么这样写：** 把协议输入变为已验证body和稳定binding。

1. 校验身份与cut形状
2. 分别校验三份内容
3. 重算mergeInputDigest
4. 返回binding及解码正文供纯计算


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 包含context、attempt_id、三份固定内容、main_cut和input_digest的请求<br>在此声明中的实际使用：<code>input.context</code>；<code>input.main_cut</code>；<code>input.attempt_id</code>；<code>input.input_digest</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ binding, content }</code> |

**跟着本段代码走一遍：**

- 检查 <code>!validID(context?.operation_id) &#124;&#124;     !positive(context?.actor_id) &#124;&#124;     !positive(input.attempt_id) &#124;&#124;     !target &#124;&#124;     !targetKeys.every((key) =&gt; positive(target&#91;key&#93;)) &#124;&#124;    …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;MERGE_INPUT_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MERGE_INPUT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!positive(room.room_id) &#124;&#124;       !positive(room.workspace_epoch) &#124;&#124;       !(room.durable_seq === &#x27;0&#x27; &#124;&#124; positive(room.durable_seq))</code>。条件成立时执行 <code>throw bad(&#x27;MERGE_CUT_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MERGE_CUT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>new Set((cut.rooms ?? &#91;&#93;).map((room) =&gt; room.room_id)).size !== (cut.rooms ?? &#91;&#93;).length</code>。条件成立时执行 <code>throw bad(&#x27;MERGE_CUT_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MERGE_CUT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!mergeInputDigest(input).equals(binary(input.input_digest))</code>。条件成立时执行 <code>throw bad(&#x27;MERGE_INPUT_DIGEST_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MERGE_INPUT_DIGEST_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return { binding, content };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4363:5974 -->
````javascript
export function validateMergeInput(input) {
  const context = input.context,
    target = context?.target,
    cut = input.main_cut;
  if (
    !validID(context?.operation_id) ||
    !positive(context?.actor_id) ||
    !positive(input.attempt_id) ||
    !target ||
    !targetKeys.every((key) => positive(target[key])) ||
    target.purpose !== 'MERGE' ||
    !context.identity_admission_ref ||
    !context.document_admission_ref ||
    binary(context.request_fingerprint).length !== 32 ||
    binary(input.input_digest).length !== 32 ||
    !Array.isArray(context.sources) ||
    !context.sources.length ||
    !cut ||
    cut.document_id !== target.document_id ||
    cut.structure_id !== target.structure_id ||
    !positive(cut.style_id) ||
    !positive(cut.control_revision) ||
    binary(cut.dependency_digest).length !== 32 ||
    !input.base ||
    !input.submitted ||
    !input.main
  )
    throw bad('MERGE_INPUT_INVALID');
  for (const room of cut.rooms ?? [])
    if (
      !positive(room.room_id) ||
      !positive(room.workspace_epoch) ||
      !(room.durable_seq === '0' || positive(room.durable_seq))
    )
      throw bad('MERGE_CUT_INVALID');
  if (new Set((cut.rooms ?? []).map((room) => room.room_id)).size !== (cut.rooms ?? []).length)
    throw bad('MERGE_CUT_INVALID');
  const content = Object.fromEntries(
    ['base', 'submitted', 'main'].map((key) => [key, fixedBody(input[key], target)]),
  );
  const binding = mergeBinding(input);
  if (!mergeInputDigest(input).equals(binary(input.input_digest)))
    throw bad('MERGE_INPUT_DIGEST_INVALID');
  return { binding, content };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/merge_protocol.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
