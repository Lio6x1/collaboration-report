# 分步编写 `services/collaboration/src/domain/discard_confirmation.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/288-discard-confirmation.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将丢弃预览绑定到新的确认操作，并验证可靠MAIN cut未变。

预览不是最终丢弃授权；确认要有独立操作号，同时只能消费原预览和原轮次。

写之前先按这条执行路线理解各段的连接：

1. deriveDiscardConfirmation核DISCARD_PREVIEW→DISCARD_DRAFT、同actor/目标/lineage及权威preview_context
2. 为draft/main/candidate重绑operation/capture/provenance引用
3. 保留preview_binding/input_digest并计算确认固定内容新摘要
4. sameReliableMainCut要求所有房间OPEN且actual向量与fixed.main_cut相同

本文件需要守住的边界：

- 不能复用预览操作ID提交丢弃
- 预览后主线/草稿变了必须重新预览，而不是默默丢弃最新内容

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/discard_confirmation.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/discard_confirmation.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/discard_confirmation.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/discard_confirmation.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:216 -->
````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { hash } from './shared_protocol.mjs';
import { RPCFailure } from './shared_protocol.mjs';
import { validID } from './shared_protocol.mjs';

````

## 第 2 步：deriveDiscardConfirmation

插入位置：第 1 步代码之后，紧接追加；完成稿约第 6 行。

只派生新确认身份；正文、旧cut与历史来源仍来自原固定预览，原包不改写。

本步声明：`export function deriveDiscardConfirmation(preview, binding, authority) `。

**这一段为什么这样写：** 把用户确认与被确认的准确损失范围关联。

1. 比较权威预览绑定
2. 重绑可证明属于本预览的操作引用
3. 固定expected_round_id和新input_digest


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>preview</code> | <code>JavaScript 参数</code> | 已持久DISCARD_PREVIEW完整结果<br>在此声明中的实际使用：<code>preview.binding</code>；<code>preview.preview_cut_ref</code>；<code>preview.input_digest</code>；<code>structuredClone(preview)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>JavaScript 参数</code> | 新DISCARD_DRAFT操作context与lineage期望<br>在此声明中的实际使用：<code>binding.context</code>；<code>binding&#91;key&#93;</code> |
| <code>authority</code> | <code>JavaScript 参数</code> | Doc对原预览及轮次的核验结果<br>在此声明中的实际使用：<code>authority.purpose</code>；<code>authority.preview_cut_ref</code>；<code>authority.preview_input_digest</code>；<code>authority.preview_context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>fixed</code> |

**跟着本段代码走一遍：**

- 检查 <code>original?.target?.purpose !== &#x27;DISCARD_PREVIEW&#x27; &#124;&#124;     context?.target?.purpose !== &#x27;DISCARD_DRAFT&#x27; &#124;&#124;     authority.purpose !== &#x27;DISCARD_DRAFT&#x27; &#124;&#124;     !validID(context.operation_i …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DISCARD_CONFIRMATION_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;DISCARD_CONFIRMATION_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>source.origin_capture_ref === original.operation_id</code>。
- 在本段的内部函数内：此处通过 <code>return value;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=216:2516 -->
````javascript
// 只派生新确认身份；正文、旧cut与历史来源仍来自原固定预览，原包不改写。
export function deriveDiscardConfirmation(preview, binding, authority) {
  const original = preview.binding?.context;
  const context = binding.context;
  if (
    original?.target?.purpose !== 'DISCARD_PREVIEW' ||
    context?.target?.purpose !== 'DISCARD_DRAFT' ||
    authority.purpose !== 'DISCARD_DRAFT' ||
    !validID(context.operation_id) ||
    context.operation_id === original.operation_id ||
    context.actor_id !== original.actor_id ||
    preview.preview_cut_ref !== authority.preview_cut_ref ||
    preview.input_digest !== authority.preview_input_digest ||
    canonical(original) !== canonical(authority.preview_context) ||
    !['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'].every(
      (key) => context.target[key] === original.target[key],
    ) ||
    !['expected_branch_id', 'expected_structure_id', 'expected_lineage_id'].every(
      (key) => binding[key] === preview.binding[key],
    ) ||
    Buffer.from(context.request_fingerprint ?? '', 'base64').length !== 32 ||
    !context.identity_admission_ref ||
    !context.document_admission_ref ||
    !/^(0|[1-9][0-9]{0,19})$/.test(authority.expected_round_id)
  )
    throw new RPCFailure(6, 'DISCARD_CONFIRMATION_CONFLICT');
  const rebind = (content, suffix) => {
    const value = structuredClone(content);
    const provenance = JSON.parse(Buffer.from(value.provenance_json_utf8, 'base64'));
    provenance.source_operation_id = context.operation_id;
    for (const source of provenance.block_sources) {
      if (source.origin_capture_ref === original.operation_id)
        source.origin_capture_ref = context.operation_id;
    }
    value.operation_id = context.operation_id;
    value.capture_id = context.operation_id + suffix;
    value.provenance_json_utf8 = Buffer.from(canonical(provenance)).toString('base64');
    return value;
  };
  const fixed = {
    ...structuredClone(preview),
    binding,
    preview_input_digest: preview.input_digest,
    preview_binding: preview.binding,
    expected_round_id: authority.expected_round_id,
    draft: rebind(preview.draft, ''),
    main: rebind(preview.main, ':main'),
    candidate: rebind(preview.candidate, ':candidate'),
  };
````

## 第 3 步：deriveDiscardConfirmation（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 55 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return fixed;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2516:2630 -->
````javascript
  delete fixed.input_digest;
  fixed.input_digest = hash(canonical(fixed)).toString('base64');
  return fixed;
}

````

## 第 4 步：sameReliableMainCut

插入位置：第 3 步代码之后，紧接追加；完成稿约第 60 行。

迟到的零尾MAIN房间也属于变化；不能仅比较已有房间的序号。

本步声明：`export function sameReliableMainCut(fixed, rows) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>fixed.main_cut</code> |
| <code>rows</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>rows.map</code>；<code>rows.every</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>rows.every((room) =&gt; room.state === &#x27;OPEN&#x27;) &amp;&amp; canonical(actual) === canonical(fixed.main_cut)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return (     rows.every((room) =&gt; room.state === &#x27;OPEN&#x27;) &amp;&amp; canonical(actual) === canonical(fixed.main_cut)   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2630:3025 -->
````javascript
// 迟到的零尾MAIN房间也属于变化；不能仅比较已有房间的序号。
export function sameReliableMainCut(fixed, rows) {
  const actual = rows.map((room) => ({
    room_id: room.id,
    workspace_epoch: room.epoch,
    durable_seq: room.last_durable_seq,
  }));
  return (
    rows.every((room) => room.state === 'OPEN') && canonical(actual) === canonical(fixed.main_cut)
  );
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/discard_confirmation.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
