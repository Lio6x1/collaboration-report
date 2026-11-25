# 分步编写 `services/collaboration/src/domain/structure_preview_protocol.mjs`

先按 [C05 · 结构拆并与每个在线编辑器保存握手](../lessons/collaboration/05-structure-online.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/305-structure-preview-protocol.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

验证全篇结构预览专用准入目标。

单元级草稿权限不能通过把unit参数改大而升级成全篇管理范围。

写之前先按这条执行路线理解各段的连接：

1. structurePreviewBinding要求purpose=STRUCTURE_PREVIEW且unit/branch/lineage期望均为0
2. 核document/structure/workspace与32字节指纹
3. base64指纹必须规范编码，保留来源与双重准入引用

本文件需要守住的边界：

- uint64必须十进制字符串；不接受负数、前导零或Number舍入
- workspace_epoch允许0不代表unit可任意指定

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/structure_preview_protocol.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/structure_preview_protocol.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/structure_preview_protocol.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/structure_preview_protocol.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:55 -->
````javascript
import { validID, bad } from './shared_protocol.mjs';

````

## 第 2 步：unsigned

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`const unsigned = (value, positive = false) =>`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>typeof value === &#x27;string&#x27;</code>；<code>(positive ? /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/ : /^(0&#124;&#91;1-9&#93;&#91;0-9&#93;{0,19})$/).test(value)</code>；<code>BigInt(value)</code> |
| <code>positive = false</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>(positive ? /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/ : /^(0&#124;&#91;1-9&#93;&#91;0-9&#93;{0,19})$/).test</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>typeof value === &#x27;string&#x27; &amp;&amp;   (positive ? /^&#91;1-9&#93;&#91;0-9&#93;{0,19}$/ : /^(0&#124;&#91;1-9&#93;&#91;0-9&#93;{0,19})$/).test(value) &amp;&amp;   BigInt(value) &lt;= 18446744073709551615n</code> |

<!-- write-step: 2 bytes=55:253 -->
````javascript
const unsigned = (value, positive = false) =>
  typeof value === 'string' &&
  (positive ? /^[1-9][0-9]{0,19}$/ : /^(0|[1-9][0-9]{0,19})$/).test(value) &&
  BigInt(value) <= 18446744073709551615n;

````

## 第 3 步：structurePreviewBinding

插入位置：第 2 步代码之后，紧接追加；完成稿约第 8 行。

全篇预览与单支线拉取是不同的准入目标；不能把浏览器传来的任意unit扩大成全篇范围。

本步声明：`export function structurePreviewBinding(input) `。

**这一段为什么这样写：** 在读取任何全篇正文前确认请求确实是全篇管理预览。

1. 验证用途及零占位字段
2. 核指纹编码和来源
3. 返回JSON可存的固定binding


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 带context和expected_branch/lineage/structure的请求<br>在此声明中的实际使用：<code>input?.context</code>；<code>input.expected_branch_id</code>；<code>input.expected_lineage_id</code>；<code>input.expected_structure_id</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     ...input,     context: {       ...context,       request_fingerprint: fingerprint.toString(&#x27;base64&#x27;),     },   }</code> |

**跟着本段代码走一遍：**

- 检查 <code>typeof value === &#x27;string&#x27; &amp;&amp; fingerprint.toString(&#x27;base64&#x27;) !== value</code>。条件成立时执行 <code>throw bad(&#x27;STRUCTURE_PREVIEW_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;STRUCTURE_PREVIEW_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!validID(context?.operation_id) &#124;&#124;     !unsigned(context?.actor_id, true) &#124;&#124;     !target &#124;&#124;     target.purpose !== &#x27;STRUCTURE_PREVIEW&#x27; &#124;&#124;     !unsigned(target.document_id, true) &#124;&#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;STRUCTURE_PREVIEW_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;STRUCTURE_PREVIEW_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {     ...input,     context: {       ...context,       request_fingerprint: fingerprint.toString(&#x27;base64&#x27;),     },   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=253:1731 -->
````javascript
// 全篇预览与单支线拉取是不同的准入目标；不能把浏览器传来的任意unit扩大成全篇范围。
export function structurePreviewBinding(input) {
  const context = input?.context,
    target = context?.target,
    value = context?.request_fingerprint,
    fingerprint =
      typeof value === 'string' ? Buffer.from(value, 'base64') : Buffer.from(value ?? []);
  // RPC bytes和protojson base64必须表示同一个32字节摘要；拒绝宽松解码接受的尾部空白或非规范字符串。
  if (typeof value === 'string' && fingerprint.toString('base64') !== value)
    throw bad('STRUCTURE_PREVIEW_INVALID');
  if (
    !validID(context?.operation_id) ||
    !unsigned(context?.actor_id, true) ||
    !target ||
    target.purpose !== 'STRUCTURE_PREVIEW' ||
    !unsigned(target.document_id, true) ||
    !unsigned(target.structure_id, true) ||
    !unsigned(target.workspace_epoch) ||
    target.unit_id !== '0' ||
    target.branch_id !== '0' ||
    input.expected_branch_id !== '0' ||
    input.expected_lineage_id !== '0' ||
    input.expected_structure_id !== target.structure_id ||
    !context.identity_admission_ref ||
    !context.document_admission_ref ||
    !Array.isArray(context.sources) ||
    !context.sources.length ||
    fingerprint.length !== 32
  )
    throw bad('STRUCTURE_PREVIEW_INVALID');
  return {
    ...input,
    context: {
      ...context,
      request_fingerprint: fingerprint.toString('base64'),
    },
  };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/structure_preview_protocol.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/05-structure-online.md)。
