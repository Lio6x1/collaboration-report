# 分步编写 `services/collaboration/src/repository/persistence_outcome.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/315-persistence-outcome.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

从实际update_log及唯一RESOURCE_CONFIRM查找私有提交证明。

资源释放端不能把Node超时当未提交，必须通过本地持久日志和原确认绑定证明。

写之前先按这条执行路线理解各段的连接：

1. committedUpdate用room/epoch/session/update查seq和payload_hash
2. confirmationsForReceipt用room:epoch:seq查询原node_receipt_ref
3. 应用层据日志与唯一任务联合决定是否COMMITTED

本文件需要守住的边界：

- CAST(seq AS CHAR)保留大整数精度
- 只找到日志或多个确认任务都不够给出完整资源释放证据

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/persistence_outcome.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/persistence_outcome.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/persistence_outcome.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/persistence_outcome.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：committedUpdate

插入位置：空文件第 1 行。

私有持久证明仅从真实Node日志及唯一固定确认任务取得；保留原pool.query调用和SQL。

本步声明：`export async function committedUpdate(pool, input) `。

**这一段为什么这样写：** 读取重试更新的已提交事实，不推测事务执行结果。

1. 按完整客户端幂等身份定位日志
2. 返回首行或undefined交应用判断


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pool</code> | <code>JavaScript 参数</code> | 只读查询所用mysql连接池<br>在此声明中的实际使用：<code>pool.query</code> |
| <code>input</code> | <code>JavaScript 参数</code> | room_id/workspace_epoch/client_session_id/client_update_id完整定位键<br>在此声明中的实际使用：<code>input.room_id</code>；<code>input.workspace_epoch</code>；<code>input.client_session_id</code>；<code>input.client_update_id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>rows&#91;0&#93;</code> |

**跟着本段代码走一遍：**

- 等待 <code>pool.query(     &#x27;SELECT CAST(seq AS CHAR) AS seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?&#x27;,     &#91;input.room_id, input …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return rows&#91;0&#93;;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 1 bytes=0:463 -->
````javascript
// 私有持久证明仅从真实Node日志及唯一固定确认任务取得；保留原pool.query调用和SQL。
export async function committedUpdate(pool, input) {
  const [rows] = await pool.query(
    'SELECT CAST(seq AS CHAR) AS seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?',
    [input.room_id, input.workspace_epoch, input.client_session_id, input.client_update_id],
  );
  return rows[0];
}
````

## 第 2 步：confirmationsForReceipt

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`export async function confirmationsForReceipt(pool, receipt) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pool</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>pool.query</code> |
| <code>receipt</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>pool.query(     &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND JSON_UNQUOTE(JSON_EXTRACT(payload,&#x27;$.node_receipt_ref&#x27;))=?&quot;,     &#91;receipt&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>rows</code> |

**跟着本段代码走一遍：**

- 等待 <code>pool.query(     &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND JSON_UNQUOTE(JSON_EXTRACT(payload,&#x27;$.node_receipt_ref&#x27;))=?&quot;,     &#91;receipt&#93;,   )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return rows;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=463:744 -->
````javascript
export async function confirmationsForReceipt(pool, receipt) {
  const [rows] = await pool.query(
    "SELECT business_key,payload FROM durable_task WHERE kind='RESOURCE_CONFIRM' AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.node_receipt_ref'))=?",
    [receipt],
  );
  return rows;
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/persistence_outcome.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
