# 分步编写 `services/collaboration/src/domain/capture_protocol.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/286-capture-protocol.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义送审冻结请求绑定、二进制存储转换和块来源证明。

送审结果要能跨进程重放且追溯编辑来源；Buffer不能直接混入任意JSON编码。

写之前先按这条执行路线理解各段的连接：

1. captureBinding核SUBMISSION目标、操作指纹、FLUSH回执和lineage/guard代次
2. storeContent/wireContent把四个二进制字段在base64与Buffer间转换
3. captureProvenance按块ID比较基线/当前：未变沿用旧来源，改动记EDITED，新增记INSERTED，缺失记DELETED

本文件需要守住的边界：

- 同号不同context/cut/lineage必须CAPTURE_ID_CONFLICT
- 不能只给存活块写来源而遗漏DELETED记录

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/capture_protocol.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/capture_protocol.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/capture_protocol.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/capture_protocol.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:175 -->
````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { validID, bad } from './shared_protocol.mjs';
import { RPCFailure } from './shared_protocol.mjs';

````

## 第 2 步：positive

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

本步声明：`const positive = (value) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof value === &#x27;string&#x27;</code>；<code>/^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(value)</code>；<code>BigInt(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>typeof value === &#x27;string&#x27; &amp;&amp;   /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/.test(value) &amp;&amp;   BigInt(value) &lt;= 18446744073709551615n</code> |

<!-- write-step: 2 bytes=175:314 -->
````javascript
const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;
````

## 第 3 步：captureBinding

插入位置：第 2 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`export function captureBinding(input) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.context</code>；<code>input.flush_receipt_id</code>；<code>input.guard_generation</code>；<code>input.lineage_ref</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     context: {       ...context,       request_fingerprint: Buffer.from(context.request_fingerprint).toString(&#x27;base64&#x27;),     },     flush_receipt_id: input.flush_receipt_id,      …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>!validID(context?.operation_id) &#124;&#124;     !positive(context?.actor_id) &#124;&#124;     !target &#124;&#124;     !&#91;&#x27;document_id&#x27;, &#x27;unit_id&#x27;, &#x27;branch_id&#x27;, &#x27;structure_id&#x27;, &#x27;workspace_epoch&#x27;&#93;.every((key) =&gt; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;CAPTURE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;CAPTURE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {     context: {       ...context,       request_fingerprint: Buffer.from(context.request_fingerprint).toString(&#x27;base64&#x27;),     },     flush_receipt_id: input.flush_receipt_i …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=314:1259 -->
````javascript
export function captureBinding(input) {
  const context = input.context,
    target = context?.target;
  if (
    !validID(context?.operation_id) ||
    !positive(context?.actor_id) ||
    !target ||
    !['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'].every((key) =>
      positive(target[key]),
    ) ||
    target.purpose !== 'SUBMISSION' ||
    !validID(input.flush_receipt_id) ||
    !positive(input.guard_generation) ||
    !positive(input.lineage_ref) ||
    Buffer.from(context.request_fingerprint ?? []).length !== 32 ||
    !context.document_admission_ref ||
    !context.identity_admission_ref
  )
    throw bad('CAPTURE_INVALID');
  return {
    context: {
      ...context,
      request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
    },
    flush_receipt_id: input.flush_receipt_id,
    guard_generation: input.guard_generation,
    lineage_ref: input.lineage_ref,
  };
}

````

## 第 4 步：sameBinding

插入位置：第 3 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`export function sameBinding(expected, actual) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>expected</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>canonical(expected)</code> |
| <code>actual</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>canonical(actual)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**跟着本段代码走一遍：**

- 检查 <code>canonical(expected) !== canonical(actual)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;CAPTURE_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;CAPTURE_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 4 bytes=1259:1407 -->
````javascript
export function sameBinding(expected, actual) {
  if (canonical(expected) !== canonical(actual)) throw new RPCFailure(6, 'CAPTURE_ID_CONFLICT');
}

````

## 第 5 步：matchCaptureQuery

插入位置：第 4 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`export function matchCaptureQuery(row, binding, query) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>row.operation_id</code>；<code>row.request_fingerprint</code> |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.context</code> |
| <code>query</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query.operation_id</code>；<code>query.request_fingerprint</code>；<code>query.target</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**跟着本段代码走一遍：**

- 检查 <code>row.operation_id !== query.operation_id &#124;&#124;     row.request_fingerprint.toString(&#x27;base64&#x27;) !==       Buffer.from(query.request_fingerprint ?? &#91;&#93;).toString(&#x27;base64&#x27;) &#124;&#124;     canonical …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;CAPTURE_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;CAPTURE_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 5 bytes=1407:1767 -->
````javascript
export function matchCaptureQuery(row, binding, query) {
  if (
    row.operation_id !== query.operation_id ||
    row.request_fingerprint.toString('base64') !==
      Buffer.from(query.request_fingerprint ?? []).toString('base64') ||
    canonical(binding.context.target) !== canonical(query.target)
  )
    throw new RPCFailure(6, 'CAPTURE_ID_CONFLICT');
}

````

## 第 6 步：binaryFields

插入位置：第 5 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`const binaryFields = ['cut_json_utf8', 'body_json_utf8', 'body_hash', 'provenance_json_utf8'];`。

<!-- write-step: 6 bytes=1767:1862 -->
````javascript
const binaryFields = ['cut_json_utf8', 'body_json_utf8', 'body_hash', 'provenance_json_utf8'];
````

## 第 7 步：storeContent

插入位置：第 6 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`export function storeContent(content) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>content</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>return {     ...content,     ...Object.fromEntries(       binaryFields.map((key) =&gt; &#91;key, Buffer.from(content&#91;key&#93;).toString(&#x27;base64&#x27;)&#93;),     ),   };</code>；<code>content&#91;key&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     ...content,     ...Object.fromEntries(       binaryFields.map((key) =&gt; &#91;key, Buffer.from(content&#91;key&#93;).toString(&#x27;base64&#x27;)&#93;),     ),   }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return {     ...content,     ...Object.fromEntries(       binaryFields.map((key) =&gt; &#91;key, Buffer.from(content&#91;key&#93;).toString(&#x27;base64&#x27;)&#93;),     ),   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1862:2056 -->
````javascript
export function storeContent(content) {
  return {
    ...content,
    ...Object.fromEntries(
      binaryFields.map((key) => [key, Buffer.from(content[key]).toString('base64')]),
    ),
  };
}
````

## 第 8 步：wireContent

插入位置：第 7 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`export function wireContent(content) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>content</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>return {     ...content,     ...Object.fromEntries(binaryFields.map((key) =&gt; &#91;key, Buffer.from(content&#91;key&#93;, &#x27;base64&#x27;)&#93;)),   };</code>；<code>content&#91;key&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     ...content,     ...Object.fromEntries(binaryFields.map((key) =&gt; &#91;key, Buffer.from(content&#91;key&#93;, &#x27;base64&#x27;)&#93;)),   }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return {     ...content,     ...Object.fromEntries(binaryFields.map((key) =&gt; &#91;key, Buffer.from(content&#91;key&#93;, &#x27;base64&#x27;)&#93;)),   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2056:2228 -->
````javascript
export function wireContent(content) {
  return {
    ...content,
    ...Object.fromEntries(binaryFields.map((key) => [key, Buffer.from(content[key], 'base64')])),
  };
}

````

## 第 9 步：captureProvenance

插入位置：第 8 步代码之后，紧接追加；完成稿约第 69 行。

来源描述保留真实旧基线和已接纳切点；新增块才把当前capture作为来源。

本步声明：`export function captureProvenance(body, lineage, operation) `。

**这一段为什么这样写：** 从固定基线和冻结正文构造可复查的块级来源。

1. 收集两份正文的块ID映射
2. 继承完全未改块原来源
3. 记录编辑/新增/删除关系
4. 保留accepted_captures


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>body</code> | <code>JavaScript 参数</code> | 本次冻结后的正文<br>在此声明中的实际使用：<code>collect(body, currentBlocks)</code> |
| <code>lineage</code> | <code>JavaScript 参数</code> | 固定的基线正文、版本ID及原provenance<br>在此声明中的实际使用：<code>lineage.body</code>；<code>lineage.provenance</code>；<code>lineage.base_version_id</code>；<code>lineage.lineage_id</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>JavaScript 参数</code> | 本次送审操作ID<br>在此声明中的实际使用：<code>blockSources.push({         block_id: id,         origin_block_id: id,         ...(base           ? { origin_content_version_id: lineage.base_version_id }           : { origin_capt …</code>（长表达式，按本步完整代码继续）；<code>return {     schema_version: &#x27;provenance-v1&#x27;,     lineage_id: lineage.lineage_id,     base_manifest_id: lineage.base_manifest_id,     base_content_version_id: lineage.base_version_ …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     schema_version: &#x27;provenance-v1&#x27;,     lineage_id: lineage.lineage_id,     base_manifest_id: lineage.base_manifest_id,     base_content_version_id: lineage.base_version_id,     …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>node.attrs?.id</code>。
- 检查 <code>base &amp;&amp; old &amp;&amp; canonical(base) === canonical(node)</code>。
- 检查 <code>!currentBlocks.has(id)</code>。
- 此处通过 <code>return {     schema_version: &#x27;provenance-v1&#x27;,     lineage_id: lineage.lineage_id,     base_manifest_id: lineage.base_manifest_id,     base_content_version_id: lineage.base_version_ …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2228:3984 -->
````javascript
// 来源描述保留真实旧基线和已接纳切点；新增块才把当前capture作为来源。
export function captureProvenance(body, lineage, operation) {
  const baseBlocks = new Map(),
    currentBlocks = new Map();
  const collect = (node, map) => {
    if (node.attrs?.id) map.set(node.attrs.id, node);
    for (const child of node.content ?? []) collect(child, map);
  };
  collect(lineage.body, baseBlocks);
  collect(body, currentBlocks);
  const previous = new Map(
    lineage.provenance.block_sources.map((source) => [source.block_id, source]),
  );
  const blockSources = [];
  for (const [id, node] of currentBlocks) {
    const base = baseBlocks.get(id),
      old = previous.get(id);
    if (base && old && canonical(base) === canonical(node)) blockSources.push(structuredClone(old));
    else
      blockSources.push({
        block_id: id,
        origin_block_id: id,
        ...(base
          ? { origin_content_version_id: lineage.base_version_id }
          : { origin_capture_ref: operation }),
        relation: base ? 'EDITED' : 'INSERTED',
        source_range: { start: 0, end: 0 },
      });
  }
  for (const [id] of baseBlocks)
    if (!currentBlocks.has(id))
      blockSources.push({
        block_id: id,
        origin_block_id: id,
        origin_content_version_id: lineage.base_version_id,
        relation: 'DELETED',
        source_range: { start: 0, end: 0 },
      });
  return {
    schema_version: 'provenance-v1',
    lineage_id: lineage.lineage_id,
    base_manifest_id: lineage.base_manifest_id,
    base_content_version_id: lineage.base_version_id,
    source_operation_id: operation,
    accepted_captures: structuredClone(lineage.provenance.accepted_captures),
    block_sources: blockSources,
  };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/capture_protocol.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
