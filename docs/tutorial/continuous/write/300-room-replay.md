# 分步编写 `services/collaboration/src/domain/room_replay.mjs`

先按 [C01 · 正文契约、隔离候选与连续日志恢复](../lessons/collaboration/01-content-replay.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/300-room-replay.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

验证检查点并严格重放到指定持久cut。

恢复不能通过跳日志或构造空房间掩盖已确认保存的数据丢失。

写之前先按这条执行路线理解各段的连接：

1. openCheckpoint校验collab-yjs-v1和SHA-256，再应用真实Yjs字节
2. applyDurableLog逐条检查seq连续、payload_hash正确，应用后必须刚好到cut
3. inspectDocument再次校验最终正文；encodeCheckpoint生成完整状态字节和摘要

本文件需要守住的边界：

- 缺检查点/摘要错误必须失败
- 最后序号不足报DURABLE_LOG_GAP，不可把缺口当空更新

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/domain' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/domain/room_replay.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/domain/room_replay.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/domain/room_replay.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/domain/room_replay.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:194 -->
````javascript
import * as Y from 'yjs';
import { timingSafeEqual } from 'node:crypto';
import { inspectDocument } from '../../../../packages/content/index.mjs';
import { hash } from './shared_protocol.mjs';

````

## 第 2 步：openCheckpoint

插入位置：第 1 步代码之后，紧接追加；完成稿约第 6 行。

检查点先验证再应用；损坏字节不能被后续日志掩盖，也不能合成一个空房间。

本步声明：`export function openCheckpoint(checkpoint) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>checkpoint</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>!checkpoint &#124;&#124;     checkpoint.format_version !== &#x27;collab-yjs-v1&#x27;</code>；<code>checkpoint.format_version</code>；<code>checkpoint.yjs_state</code>；<code>checkpoint.checksum</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>doc</code> |

**跟着本段代码走一遍：**

- 检查 <code>!checkpoint &#124;&#124;     checkpoint.format_version !== &#x27;collab-yjs-v1&#x27; &#124;&#124;     !timingSafeEqual(hash(checkpoint.yjs_state), checkpoint.checksum)</code>。条件成立时执行 <code>throw Error(&#x27;CHECKPOINT_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;CHECKPOINT_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return doc;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>throw error;</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 2 bytes=194:700 -->
````javascript
// 检查点先验证再应用；损坏字节不能被后续日志掩盖，也不能合成一个空房间。
export function openCheckpoint(checkpoint) {
  if (
    !checkpoint ||
    checkpoint.format_version !== 'collab-yjs-v1' ||
    !timingSafeEqual(hash(checkpoint.yjs_state), checkpoint.checksum)
  )
    throw Error('CHECKPOINT_INVALID');
  const doc = new Y.Doc();
  try {
    Y.applyUpdate(doc, checkpoint.yjs_state);
    return doc;
  } catch (error) {
    doc.destroy();
    throw error;
  }
}

````

## 第 3 步：applyDurableLog

插入位置：第 2 步代码之后，紧接追加；完成稿约第 24 行。

纯规则只应用指定cut的连续持久日志；Y.Doc由调用用例负责归还或释放。

本步声明：`export function applyDurableLog(doc, covered, updates, room, cut) `。

**这一段为什么这样写：** 证明恢复结果覆盖了每一个已持久序号。

1. 从covered开始用BigInt逐条递增
2. 对每条字节验证哈希后applyUpdate
3. 检查终点cut及真实正文后返回同一个doc


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>doc</code> | <code>JavaScript 参数</code> | 从已验证检查点打开的独立Y.Doc<br>在此声明中的实际使用：<code>Y.applyUpdate(doc, update.payload)</code>；<code>inspectDocument(doc, {     workspaceKind: room.workspace_kind,   })</code>；<code>return doc;</code> |
| <code>covered</code> | <code>JavaScript 参数</code> | 检查点已覆盖的序号<br>在此声明中的实际使用：<code>BigInt(covered)</code> |
| <code>updates</code> | <code>JavaScript 参数</code> | 按seq升序读取的区间日志 |
| <code>room</code> | <code>JavaScript 参数</code> | 含workspace_kind的房间元数据<br>在此声明中的实际使用：<code>room.workspace_kind</code> |
| <code>cut</code> | <code>JavaScript 参数</code> | 此次固定恢复目标序号<br>在此声明中的实际使用：<code>BigInt(cut)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>doc</code> |

**跟着本段代码走一遍：**

- 检查 <code>BigInt(update.seq) !== ++sequence &#124;&#124;       !timingSafeEqual(hash(update.payload), update.payload_hash)</code>。条件成立时执行 <code>throw Error(&#x27;DURABLE_LOG_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;DURABLE_LOG_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>sequence !== BigInt(cut)</code>。条件成立时执行 <code>throw Error(&#x27;DURABLE_LOG_GAP&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;DURABLE_LOG_GAP&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return doc;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=700:1420 -->
````javascript
// 纯规则只应用指定cut的连续持久日志；Y.Doc由调用用例负责归还或释放。
export function applyDurableLog(doc, covered, updates, room, cut) {
  let sequence = BigInt(covered);
  for (const update of updates) {
    if (
      BigInt(update.seq) !== ++sequence ||
      !timingSafeEqual(hash(update.payload), update.payload_hash)
    )
      throw Error('DURABLE_LOG_INVALID');
    Y.applyUpdate(doc, update.payload);
  }
  if (sequence !== BigInt(cut)) throw Error('DURABLE_LOG_GAP');
  // 恢复文档本就独立于在线房间；完整检查沿共享规则直接只读执行，避免再次克隆正文。
  inspectDocument(doc, {
    workspaceKind: room.workspace_kind,
  });
  return doc;
}

````

## 第 4 步：encodeCheckpoint

插入位置：第 3 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`export function encodeCheckpoint(doc) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>doc</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>Y.encodeStateAsUpdate(doc)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{ bytes, checksum: hash(bytes) }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return { bytes, checksum: hash(bytes) };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1420:1562 -->
````javascript
export function encodeCheckpoint(doc) {
  const bytes = Buffer.from(Y.encodeStateAsUpdate(doc));
  return { bytes, checksum: hash(bytes) };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/domain/room_replay.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/01-content-replay.md)。
