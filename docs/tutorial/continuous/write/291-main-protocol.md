# 分步编写 `services/collaboration/src/domain/main_protocol.mjs`

先按 [C04 · 固定送审、三方合并与草稿变换](../lessons/collaboration/04-fixed-workflows.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/291-main-protocol.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

规范MAIN屏障请求和跨服务cut向量表示。

全篇主线由多个房间组成，必须冻结一组room/epoch/seq而非单一“最新”标记。

写之前先按这条执行路线理解各段的连接：

1. storedCut按room_id数值排序并将dependency_digest转base64
2. prepareBinding核operation/actor/来源与文档结构匹配，rooms最多10000且无重复
3. wireCut恢复Buffer给RPC使用；cutDigest散列规范room数组

本文件需要守住的边界：

- 序号0是合法初始cut，room与epoch必须正整数
- 排序应按BigInt，字符串字典顺序会让10排在2前

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/main_protocol.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/main_protocol.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/main_protocol.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/main_protocol.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:169 -->
````javascript
import { canonical } from '../../../../packages/content/index.mjs';
import { validID, bad } from './shared_protocol.mjs';
import { hash } from './shared_protocol.mjs';

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

<!-- write-step: 2 bytes=169:308 -->
````javascript
const positive = (value) =>
  typeof value === 'string' &&
  /^[1-9][0-9]{0,19}$/.test(value) &&
  BigInt(value) <= 18446744073709551615n;
````

## 第 3 步：cutDigest

插入位置：第 2 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`export const cutDigest = (rooms) => `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>rooms</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>canonical(rooms)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>hash(Buffer.from(canonical(rooms))).toString(&#x27;base64&#x27;)</code> |

<!-- write-step: 3 bytes=308:400 -->
````javascript
export const cutDigest = (rooms) => hash(Buffer.from(canonical(rooms))).toString('base64');
````

## 第 4 步：storedCut

插入位置：第 3 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`export function storedCut(cut) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>cut.document_id</code>；<code>cut.structure_id</code>；<code>cut.style_id</code>；<code>cut.control_revision</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     document_id: cut.document_id,     structure_id: cut.structure_id,     style_id: cut.style_id,     control_revision: cut.control_revision,     rooms: &#91;...cut.rooms&#93;.sort((a, b …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return {     document_id: cut.document_id,     structure_id: cut.structure_id,     style_id: cut.style_id,     control_revision: cut.control_revision,     rooms: &#91;...cut.rooms&#93;.sor …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=400:823 -->
````javascript
export function storedCut(cut) {
  return {
    document_id: cut.document_id,
    structure_id: cut.structure_id,
    style_id: cut.style_id,
    control_revision: cut.control_revision,
    rooms: [...cut.rooms].sort((a, b) =>
      BigInt(a.room_id) < BigInt(b.room_id) ? -1 : BigInt(a.room_id) > BigInt(b.room_id) ? 1 : 0,
    ),
    dependency_digest: Buffer.from(cut.dependency_digest ?? []).toString('base64'),
  };
}
````

## 第 5 步：wireCut

插入位置：第 4 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`export const wireCut = (cut) => (`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>cut.dependency_digest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。<br>实际返回表达式：<code>{   ...cut,   dependency_digest: Buffer.from(cut.dependency_digest, &#x27;base64&#x27;), }</code> |

<!-- write-step: 5 bytes=823:940 -->
````javascript
export const wireCut = (cut) => ({
  ...cut,
  dependency_digest: Buffer.from(cut.dependency_digest, 'base64'),
});

````

## 第 6 步：prepareBinding

插入位置：第 5 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`export function prepareBinding(input) `。

**这一段为什么这样写：** 在任何SQL锁之前拒绝无法唯一解释的主线准备请求。

1. 校验原context与expected_cut
2. 检查每个房间十进制ID/序号
3. 拒绝重复房间
4. 返回JSON可存的绑定


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 含context、barrier_generation和expected_cut的准备请求<br>在此声明中的实际使用：<code>input.context</code>；<code>input.expected_cut</code>；<code>input.barrier_generation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     context: {       ...context,       request_fingerprint: Buffer.from(context.request_fingerprint).toString(&#x27;base64&#x27;),     },     barrier_generation: input.barrier_generation,     expected_cut: storedCut(cut),   }</code> |

**跟着本段代码走一遍：**

- 检查 <code>!validID(context?.operation_id) &#124;&#124;     !positive(context?.actor_id) &#124;&#124;     !positive(input.barrier_generation) &#124;&#124;     !target &#124;&#124;     !positive(target.document_id) &#124;&#124;     !positive( …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw bad(&#x27;MAIN_PREPARE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MAIN_PREPARE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!positive(room.room_id) &#124;&#124;       !positive(room.workspace_epoch) &#124;&#124;       !(room.durable_seq === &#x27;0&#x27; &#124;&#124; positive(room.durable_seq))</code>。条件成立时执行 <code>throw bad(&#x27;MAIN_CUT_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MAIN_CUT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>new Set(cut.rooms.map((room) =&gt; room.room_id)).size !== cut.rooms.length</code>。条件成立时执行 <code>throw bad(&#x27;MAIN_CUT_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;MAIN_CUT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return {     context: {       ...context,       request_fingerprint: Buffer.from(context.request_fingerprint).toString(&#x27;base64&#x27;),     },     barrier_generation: input.barrier_gener …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=940:2502 -->
````javascript
export function prepareBinding(input) {
  const context = input.context,
    target = context?.target,
    cut = input.expected_cut;
  if (
    !validID(context?.operation_id) ||
    !positive(context?.actor_id) ||
    !positive(input.barrier_generation) ||
    !target ||
    !positive(target.document_id) ||
    !positive(target.structure_id) ||
    typeof target.purpose !== 'string' ||
    !/^[A-Z_]{1,32}$/.test(target.purpose) ||
    !context.document_admission_ref ||
    !context.identity_admission_ref ||
    Buffer.from(context.request_fingerprint ?? []).length !== 32 ||
    !Array.isArray(context.sources) ||
    !context.sources.length ||
    !cut ||
    cut.document_id !== target.document_id ||
    cut.structure_id !== target.structure_id ||
    !positive(cut.style_id) ||
    !positive(cut.control_revision) ||
    !Array.isArray(cut.rooms) ||
    cut.rooms.length > 10000 ||
    Buffer.from(cut.dependency_digest ?? []).length !== 32
  )
    throw bad('MAIN_PREPARE_INVALID');
  for (const room of cut.rooms)
    if (
      !positive(room.room_id) ||
      !positive(room.workspace_epoch) ||
      !(room.durable_seq === '0' || positive(room.durable_seq))
    )
      throw bad('MAIN_CUT_INVALID');
  if (new Set(cut.rooms.map((room) => room.room_id)).size !== cut.rooms.length)
    throw bad('MAIN_CUT_INVALID');
  return {
    context: {
      ...context,
      request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
    },
    barrier_generation: input.barrier_generation,
    expected_cut: storedCut(cut),
  };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/main_protocol.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/04-fixed-workflows.md)。
