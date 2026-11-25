# 分步编写 `services/collaboration/src/domain/draft_protocol.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/289-draft-protocol.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义草稿预览、拉取主线、丢弃候选和安装绑定规则。

草稿变换会换房间代次，必须证明安装正文正是用户预览的候选，lineage也与新基线一致。

写之前先按这条执行路线理解各段的连接：

1. draftPreviewBinding核用途、目标、lineage和请求指纹；draftBody去除font后按BRANCH校验
2. fixedDraft把正文、cut、资源、provenance打包为固定内容
3. computeDraftPreview分PULL_MAIN三方合并与DISCARD_PREVIEW取main
4. validateDraftInstall逐项核旧room、新epoch=旧+1、新room、候选字节/版本/资源、seq0 cut和provenance

本文件需要守住的边界：

- 不能只校验body_hash而忽略lineage/provenance
- 新room不得复用旧room ID，安装必须从seq0建立新检查点
- draftPreparedResult超过24MiB明确失败，不截断正文

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/draft_protocol.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/draft_protocol.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/draft_protocol.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/draft_protocol.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:378 -->
````javascript
import { canonical, normalizeContent, resourceIds } from '../../../../packages/content/index.mjs';
import { validID, bad } from './shared_protocol.mjs';
import { hash } from './shared_protocol.mjs';
import { captureProvenance, storeContent } from './capture_protocol.mjs';
import { mergeContent } from './content_merge.mjs';
import { RPCFailure } from './shared_protocol.mjs';

````

## 第 2 步：draftPreviewBinding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 8 行。

本步声明：`export function draftPreviewBinding(input) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.context</code>；<code>input.expected_branch_id</code>；<code>input.expected_structure_id</code>；<code>input.expected_lineage_id</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     ...input,     context: {       ...context,       request_fingerprint: fingerprint.toString(&#x27;base64&#x27;),     },   }</code> |

**跟着本段代码走一遍：**

- 检查 <code>!validID(context?.operation_id) &#124;&#124;     !positive(context?.actor_id) &#124;&#124;     !&#91;&#x27;PULL_MAIN&#x27;, &#x27;DISCARD_PREVIEW&#x27;, &#x27;REVERSE&#x27;, &#x27;CONFLICT_RESOLUTION&#x27;&#93;.includes(target?.purpose) &#124;&#124;     !&#91;&#x27;d …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;DRAFT_PREVIEW_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;DRAFT_PREVIEW_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {     ...input,     context: {       ...context,       request_fingerprint: fingerprint.toString(&#x27;base64&#x27;),     },   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=378:1691 -->
````javascript
export function draftPreviewBinding(input) {
  const context = input.context,
    target = context?.target;
  const rawFingerprint = context?.request_fingerprint;
  const fingerprint =
    typeof rawFingerprint === 'string'
      ? Buffer.from(rawFingerprint, 'base64')
      : Buffer.from(rawFingerprint ?? []);
  const positive = (value) =>
    typeof value === 'string' &&
    /^[1-9][0-9]{0,19}$/.test(value) &&
    BigInt(value) <= 18446744073709551615n;
  if (
    !validID(context?.operation_id) ||
    !positive(context?.actor_id) ||
    !['PULL_MAIN', 'DISCARD_PREVIEW', 'REVERSE', 'CONFLICT_RESOLUTION'].includes(target?.purpose) ||
    !['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'].every((key) =>
      positive(target[key]),
    ) ||
    input.expected_branch_id !== target.branch_id ||
    input.expected_structure_id !== target.structure_id ||
    !positive(input.expected_lineage_id) ||
    fingerprint.length !== 32 ||
    (typeof rawFingerprint === 'string' && fingerprint.toString('base64') !== rawFingerprint) ||
    !context.identity_admission_ref ||
    !context.document_admission_ref
  )
    throw bad('DRAFT_PREVIEW_INVALID');
  return {
    ...input,
    context: {
      ...context,
      request_fingerprint: fingerprint.toString('base64'),
    },
  };
}

````

## 第 3 步：draftBody

插入位置：第 2 步代码之后，紧接追加；完成稿约第 45 行。

支线由样式修订控制字体；拉取正文不能把 MAIN 独立字体标记注入禁止该能力的草稿。

本步声明：`export function draftBody(body) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>body</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>structuredClone(body)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>normalizeContent(value)</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>node.marks</code>。
- 在本段的内部函数内：检查 <code>!node.marks.length</code>。
- 此处通过 <code>return normalizeContent(value);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1691:2294 -->
````javascript
// 支线由样式修订控制字体；拉取正文不能把 MAIN 独立字体标记注入禁止该能力的草稿。
export function draftBody(body) {
  const value = structuredClone(body);
  const visit = (node) => {
    if (node.marks) {
      node.marks = node.marks.filter((mark) => mark.type !== 'font');
      if (!node.marks.length) delete node.marks;
    }
    for (const child of node.content ?? []) visit(child);
  };
  visit(value);
  return normalizeContent(value);
}

// 真实MAIN基线可能带管理字体；旧固定来源只在读取比较时投影，不改历史artifact的字节。
````

## 第 4 步：draftLineage

插入位置：第 3 步代码之后，紧接追加；完成稿约第 60 行。

版本/谱系/已接纳切点保持原身份。客户端输入仍必须经过严格BRANCH正文校验。

本步声明：`export function draftLineage(lineage) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>lineage</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>return { ...lineage, body: draftBody(lineage.body) };</code>；<code>lineage.body</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ ...lineage, body: draftBody(lineage.body) }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return { ...lineage, body: draftBody(lineage.body) };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2294:2501 -->
````javascript
// 版本/谱系/已接纳切点保持原身份。客户端输入仍必须经过严格BRANCH正文校验。
export function draftLineage(lineage) {
  return { ...lineage, body: draftBody(lineage.body) };
}

````

## 第 5 步：fixedDraft

插入位置：第 4 步代码之后，紧接追加；完成稿约第 65 行。

本步声明：`export function fixedDraft(body, context, cut, lineage, ref) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>body</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>canonical(body)</code>；<code>resourceIds(body)</code>；<code>captureProvenance(body, lineage, context.operation_id)</code> |
| <code>context</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>context.target</code>；<code>context.operation_id</code> |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>canonical(cut)</code> |
| <code>lineage</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>lineage.lineage_id</code>；<code>captureProvenance(body, lineage, context.operation_id)</code> |
| <code>ref</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>storeContent({     document_id: context.target.document_id,     unit_id: context.target.unit_id,     branch_id: context.target.branch_id,     structure_id: context.target.structure …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>storeContent({     document_id: context.target.document_id,     unit_id: context.target.unit_id,     branch_id: context.target.branch_id,     structure_id: context.target.structure …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return storeContent({     document_id: context.target.document_id,     unit_id: context.target.unit_id,     branch_id: context.target.branch_id,     structure_id: context.target.st …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2501:3411 -->
````javascript
export function fixedDraft(body, context, cut, lineage, ref) {
  const bytes = Buffer.from(canonical(body));
  return storeContent({
    document_id: context.target.document_id,
    unit_id: context.target.unit_id,
    branch_id: context.target.branch_id,
    structure_id: context.target.structure_id,
    workspace_epoch: context.target.workspace_epoch,
    capture_id: ref,
    operation_id: context.operation_id,
    lineage_ref: lineage.lineage_id,
    cut_json_utf8: Buffer.from(canonical(cut)),
    body_json_utf8: bytes,
    schema_version: 'content-v1',
    canonicalization_version: 'canonical-v1',
    hash_algorithm: 'SHA-256',
    body_hash: hash(bytes),
    byte_size: String(bytes.length),
    resource_ids: resourceIds(body),
    provenance_json_utf8: Buffer.from(
      canonical(captureProvenance(body, lineage, context.operation_id)),
    ),
    provenance_schema: 'provenance-v1',
  });
}

````

## 第 6 步：computePull

插入位置：第 5 步代码之后，紧接追加；完成稿约第 91 行。

本步声明：`export function computePull(base, draft, main) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>draftBody(base)</code> |
| <code>draft</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>draftBody(draft)</code> |
| <code>main</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>draftBody(main)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ ...result, body: result.body &amp;&amp; draftBody(result.body) }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return { ...result, body: result.body &amp;&amp; draftBody(result.body) };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3411:3657 -->
````javascript
export function computePull(base, draft, main) {
  const result = mergeContent({
    base: draftBody(base),
    submitted: draftBody(draft),
    main: draftBody(main),
  });
  return { ...result, body: result.body && draftBody(result.body) };
}

````

## 第 7 步：computeDraftPreview

插入位置：第 6 步代码之后，紧接追加；完成稿约第 100 行。

策略由Doc原操作用途决定。丢弃预览保留旧稿，候选仅为同一可靠MAIN；拉取仍沿原三方算法。

本步声明：`export function computeDraftPreview(purpose, base, draft, main) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>purpose</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>purpose === &#x27;PULL_MAIN&#x27;</code>；<code>purpose === &#x27;DISCARD_PREVIEW&#x27;</code> |
| <code>base</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>computePull(base, draft, main)</code> |
| <code>draft</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>computePull(base, draft, main)</code> |
| <code>main</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>computePull(base, draft, main)</code>；<code>draftBody(main)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>computePull(base, draft, main)</code>；<code>{ body: draftBody(main), conflicts: &#91;&#93; }</code> |

**跟着本段代码走一遍：**

- 检查 <code>purpose === &#x27;PULL_MAIN&#x27;</code>。条件成立时执行 <code>return computePull(base, draft, main);</code>。当前函数在这里结束。
- 检查 <code>purpose === &#x27;DISCARD_PREVIEW&#x27;</code>。条件成立时执行 <code>return { body: draftBody(main), conflicts: &#91;&#93; };</code>。当前函数在这里结束。
- 执行 <code>throw bad(&#x27;DRAFT_PREVIEW_PURPOSE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 7 bytes=3657:4178 -->
````javascript
// 策略由Doc原操作用途决定。丢弃预览保留旧稿，候选仅为同一可靠MAIN；拉取仍沿原三方算法。
export function computeDraftPreview(purpose, base, draft, main) {
  if (purpose === 'PULL_MAIN') return computePull(base, draft, main);
  if (purpose === 'DISCARD_PREVIEW') return { body: draftBody(main), conflicts: [] };
  throw bad('DRAFT_PREVIEW_PURPOSE_INVALID');
}

// Go的CutBinding按数值room_id计算依赖摘要。房间ID不是创建时间，不能假设BRANCH总排在MAIN前。
````

## 第 8 步：draftPreviewCutDigest

插入位置：第 7 步代码之后，紧接追加；完成稿约第 108 行。

原预览的cut数组/完整input_digest不改写；旧PULL协议保留它原来的摘要排列。

本步声明：`export function draftPreviewCutDigest(purpose, cuts) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>purpose</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>purpose === &#x27;DISCARD_PREVIEW&#x27;</code> |
| <code>cuts</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>&#91;...cuts&#93;.sort</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>hash(canonical(values)).toString(&#x27;base64&#x27;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return hash(canonical(values)).toString(&#x27;base64&#x27;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4178:4674 -->
````javascript
// 原预览的cut数组/完整input_digest不改写；旧PULL协议保留它原来的摘要排列。
export function draftPreviewCutDigest(purpose, cuts) {
  const values =
    purpose === 'DISCARD_PREVIEW'
      ? [...cuts].sort((left, right) =>
          BigInt(left.room_id) < BigInt(right.room_id)
            ? -1
            : BigInt(left.room_id) > BigInt(right.room_id)
              ? 1
              : 0,
        )
      : cuts;
  return hash(canonical(values)).toString('base64');
}

````

## 第 9 步：draftPreparedResult

插入位置：第 8 步代码之后，紧接追加；完成稿约第 123 行。

本步声明：`export function draftPreparedResult(fixed) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>fixed.preview_cut_ref</code>；<code>fixed.input_digest</code>；<code>fixed.draft_cut</code>；<code>fixed.candidate</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>result</code> |

**跟着本段代码走一遍：**

- 检查 <code>Buffer.byteLength(JSON.stringify(result)) &gt; 24 &lt;&lt; 20</code>。条件成立时执行 <code>throw new RPCFailure(8, &#x27;DRAFT_TRANSFER_LIMIT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(8, &#x27;DRAFT_TRANSFER_LIMIT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return result;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4674:5181 -->
````javascript
export function draftPreparedResult(fixed) {
  const result = {
    state: 'PREPARED',
    preview_cut_ref: fixed.preview_cut_ref,
    input_digest: fixed.input_digest,
    draft_cut: fixed.draft_cut,
    candidate: fixed.candidate,
    main: fixed.main,
  };
  // 预算按真实base64 JSON传输测量，并预留其余协议字段；超限发生在房间BLOCKED之前。
  if (Buffer.byteLength(JSON.stringify(result)) > 24 << 20)
    throw new RPCFailure(8, 'DRAFT_TRANSFER_LIMIT');
  return result;
}

````

## 第 10 步：validateDraftInstall

插入位置：第 9 步代码之后，紧接追加；完成稿约第 138 行。

新代次只允许重编码原固定候选与真实MAIN基线，不能因两个Go输入彼此一致就替换可靠正文。

本步声明：`export function validateDraftInstall(fixed, source) `。

**这一段为什么这样写：** 阻止系统安装阶段换候选、换基线或偷用旧房间。

1. 核目标与新代次
2. 逐字段比较candidate
3. 重算规范正文hash/byte_size
4. 比较新lineage基线与provenance
5. 核新room seq0 cut


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>JavaScript 参数</code> | 原预览/准备阶段已持久冻结结果<br>在此声明中的实际使用：<code>fixed.binding</code>；<code>fixed.candidate</code>；<code>fixed.room_id</code>；<code>fixed.source</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>source</code> | <code>JavaScript 参数</code> | Doc已提交的草稿安装事实<br>在此声明中的实际使用：<code>source.content</code>；<code>source.lineage</code>；<code>source.old_room_id</code>；<code>source.new_epoch</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**跟着本段代码走一遍：**

- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;DRAFT_INSTALL_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!content &#124;&#124;     !candidate &#124;&#124;     !source.lineage &#124;&#124;     source.old_room_id !== fixed.room_id &#124;&#124;     source.new_epoch !== (BigInt(target.workspace_epoch) + 1n).toString() &#124;&#124;     co …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>canonical(content&#91;key&#93;) !== canonical(candidate&#91;key&#93;)</code>。
- 检查 <code>hash(bytes).toString(&#x27;base64&#x27;) !== content.body_hash &#124;&#124;     String(bytes.length) !== content.byte_size &#124;&#124;     bytes.length &gt; 16 &lt;&lt; 20</code>。

<!-- write-step: 10 bytes=5181:6822 -->
````javascript
// 新代次只允许重编码原固定候选与真实MAIN基线，不能因两个Go输入彼此一致就替换可靠正文。
export function validateDraftInstall(fixed, source) {
  const conflict = () => {
    throw new RPCFailure(6, 'DRAFT_INSTALL_CONFLICT');
  };
  const target = fixed.binding.context.target,
    content = source.content,
    candidate = fixed.candidate;
  if (
    !content ||
    !candidate ||
    !source.lineage ||
    source.old_room_id !== fixed.room_id ||
    source.new_epoch !== (BigInt(target.workspace_epoch) + 1n).toString() ||
    content.workspace_epoch !== source.new_epoch ||
    source.new_room_id === fixed.room_id ||
    !['document_id', 'unit_id', 'branch_id', 'structure_id'].every(
      (key) => content[key] === target[key],
    ) ||
    content.operation_id !== candidate.operation_id ||
    content.capture_id !== candidate.capture_id ||
    content.lineage_ref !== source.lineage.lineage_id ||
    source.lineage.base_manifest_id !==
      (target.purpose === 'CONFLICT_RESOLUTION'
        ? fixed.source.resolution_base_manifest_id
        : fixed.source.main_manifest_id)
  )
    conflict();
  for (const key of [
    'body_json_utf8',
    'body_hash',
    'byte_size',
    'resource_ids',
    'schema_version',
    'canonicalization_version',
    'hash_algorithm',
    'provenance_schema',
  ]) {
    if (canonical(content[key]) !== canonical(candidate[key])) conflict();
  }
  const bytes = Buffer.from(content.body_json_utf8, 'base64');
  if (
    hash(bytes).toString('base64') !== content.body_hash ||
    String(bytes.length) !== content.byte_size ||
    bytes.length > 16 << 20
  )
````

## 第 11 步：validateDraftInstall（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 184 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>canonical(body) !== bytes.toString() &#124;&#124;     canonical(source.lineage.body) !==       (target.purpose === &#x27;CONFLICT_RESOLUTION&#x27;         ? canonical(draftBody(JSON.parse(wireResoluti …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>Buffer.from(content.cut_json_utf8, &#x27;base64&#x27;).toString() !== canonical(expectedCut)</code>。
- 检查 <code>canonical(provenance) !== canonical(source.lineage.provenance) &#124;&#124;     provenance.lineage_id !== content.lineage_ref &#124;&#124;     provenance.base_manifest_id !== source.lineage.base_manif …</code>（长表达式，按本步完整代码继续）。

<!-- write-step: 11 bytes=6822:8261 -->
````javascript
    conflict();
  const body = normalizeContent(JSON.parse(bytes));
  if (
    canonical(body) !== bytes.toString() ||
    canonical(source.lineage.body) !==
      (target.purpose === 'CONFLICT_RESOLUTION'
        ? canonical(draftBody(JSON.parse(wireResolutionBaseline(fixed))))
        : Buffer.from(fixed.main.body_json_utf8, 'base64').toString()) ||
    (target.purpose === 'CONFLICT_RESOLUTION' &&
      source.lineage.base_version_id !== fixed.source.resolution_base_version_id)
  )
    conflict();
  const expectedCut = {
    room_id: source.new_room_id,
    workspace_epoch: source.new_epoch,
    durable_seq: '0',
  };
  if (Buffer.from(content.cut_json_utf8, 'base64').toString() !== canonical(expectedCut))
    conflict();
  const original = JSON.parse(Buffer.from(candidate.provenance_json_utf8, 'base64'));
  const provenance = JSON.parse(Buffer.from(content.provenance_json_utf8, 'base64'));
  if (
    canonical(provenance) !== canonical(source.lineage.provenance) ||
    provenance.lineage_id !== content.lineage_ref ||
    provenance.base_manifest_id !== source.lineage.base_manifest_id ||
    provenance.base_content_version_id !== source.lineage.base_version_id ||
    provenance.source_operation_id !== candidate.operation_id ||
    canonical(provenance.accepted_captures) !== canonical(original.accepted_captures) ||
    canonical(provenance.block_sources) !== canonical(original.block_sources)
  )
    conflict();
}

````

## 第 12 步：wireResolutionBaseline

插入位置：第 11 步代码之后，紧接追加；完成稿约第 217 行。

本步声明：`function wireResolutionBaseline(fixed) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>fixed.source</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>Buffer.from(raw, &#x27;base64&#x27;).toString()</code> |

**跟着本段代码走一遍：**

- 检查 <code>typeof raw !== &#x27;string&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DRAFT_INSTALL_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;DRAFT_INSTALL_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return Buffer.from(raw, &#x27;base64&#x27;).toString();</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=8261:8498 -->
````javascript
function wireResolutionBaseline(fixed) {
  const raw = fixed.source.resolution_baseline?.body_json_utf8;
  if (typeof raw !== 'string') throw new RPCFailure(6, 'DRAFT_INSTALL_CONFLICT');
  return Buffer.from(raw, 'base64').toString();
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/draft_protocol.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
