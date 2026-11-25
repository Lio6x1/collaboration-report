# 分步编写 `services/collaboration/src/repository/resource_confirmations.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/318-resource-confirmations.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

存储与UPDATE同事务产生的Doc资源确认任务。

进程可在COMMIT后、确认Doc前退出；固定payload必须留在数据库以便重启补发。

写之前先按这条执行路线理解各段的连接：

1. queueResourceConfirmation以保护operation为business_key插入QUEUED任务
2. pendingResourceConfirmations按指定id或到期32条读取
3. CONFIRMED后置SUCCEEDED；未知结果记RETRY、递增attempts并延后一秒

本文件需要守住的边界：

- 不得在写日志事务之外补插任务，那个窗口会留下有正文无确认输入的状态
- last_error只保存稳定码，payload保留原保护绑定

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/resource_confirmations.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/resource_confirmations.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/resource_confirmations.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/resource_confirmations.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:36 -->
````javascript
import { query } from './sql.mjs';

````

## 第 2 步：queueResourceConfirmation

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

本步声明：`export function queueResourceConfirmation(connection, operation, payload) `。

**这一段为什么这样写：** 把远端确认工作变成可恢复的持久待办。

1. 在当前UPDATE事务插入RESOURCE_CONFIRM
2. payload包含原请求指纹、node_receipt_ref、body_hash与资源摘要


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 写update_log的同一连接<br>在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;RESOURCE_CONFIRM&#x27;,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;operation, payload&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 稳定protectId，也是幂等恢复键<br>在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;RESOURCE_CONFIRM&#x27;,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;operation, payload&#93;,   )</code> |
| <code>payload</code> | <code>JavaScript 参数</code> | 完整确认请求的JSON字符串<br>在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;RESOURCE_CONFIRM&#x27;,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;operation, payload&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;RESOURCE_CONFIRM&#x27;,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;operation, payload&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;RESOURCE_CONFIRM&#x27;,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;operation, payload&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=36:413 -->
````javascript
// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function queueResourceConfirmation(connection, operation, payload) {
  return query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('RESOURCE_CONFIRM',?,?,'QUEUED',UTC_TIMESTAMP(6))",
    [operation, payload],
  );
}

````

## 第 3 步：markResourceConfirmed

插入位置：第 2 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`export function markResourceConfirmed(connection, operation) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,last_error=NULL WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND business_key=?&quot;,     &#91;operation&#93;,   )</code> |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,last_error=NULL WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND business_key=?&quot;,     &#91;operation&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,last_error=NULL WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND business_key=?&quot;,     &#91;operation&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,last_error=NULL WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND business_key=?&quot;,     &#91;operation&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=413:647 -->
````javascript
export function markResourceConfirmed(connection, operation) {
  return query(
    connection,
    "UPDATE durable_task SET state='SUCCEEDED',last_error=NULL WHERE kind='RESOURCE_CONFIRM' AND business_key=?",
    [operation],
  );
}

````

## 第 4 步：retryResourceConfirmation

插入位置：第 3 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`export function retryResourceConfirmation(connection, state, error, operation) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET state=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),attempts=attempts+1,last_error=? WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND …</code>（长表达式，按本步完整代码继续） |
| <code>state</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET state=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),attempts=attempts+1,last_error=? WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND …</code>（长表达式，按本步完整代码继续） |
| <code>error</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET state=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),attempts=attempts+1,last_error=? WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET state=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),attempts=attempts+1,last_error=? WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;UPDATE durable_task SET state=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),attempts=attempts+1,last_error=? WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;UPDATE durable_task SET state=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),attempts=attempts+1,last_error=? WHERE kind=&#x27;RESOURCE_CONFI …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=647:974 -->
````javascript
export function retryResourceConfirmation(connection, state, error, operation) {
  return query(
    connection,
    "UPDATE durable_task SET state=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),attempts=attempts+1,last_error=? WHERE kind='RESOURCE_CONFIRM' AND business_key=?",
    [state, error, operation],
  );
}

````

## 第 5 步：pendingResourceConfirmations

插入位置：第 4 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`export function pendingResourceConfirmations(connection, id) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND state&lt;&gt;&#x27;SUCCEEDED&#x27; AND &quot; +       (id ? &#x27;business_key=?&#x27; : &#x27;retry_at&lt;=UTC …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>&quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND state&lt;&gt;&#x27;SUCCEEDED&#x27; AND &quot; +       (id ? &#x27;business_key=?&#x27; : &#x27;retry_at&lt;=UTC_TIMESTAMP(6) ORDER BY retry_at LIMIT 32&#x27;)</code>；<code>query(     connection,     &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND state&lt;&gt;&#x27;SUCCEEDED&#x27; AND &quot; +       (id ? &#x27;business_key=?&#x27; : &#x27;retry_at&lt;=UTC …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND state&lt;&gt;&#x27;SUCCEEDED&#x27; AND &quot; +       (id ? &#x27;business_key=?&#x27; : &#x27;retry_at&lt;=UTC …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;RESOURCE_CONFIRM&#x27; AND state&lt;&gt;&#x27;SUCCEEDED&#x27; AND &quot; +       (id ? &#x27;business_key=?&#x27; : &#x27;retry_ …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=974:1297 -->
````javascript
export function pendingResourceConfirmations(connection, id) {
  return query(
    connection,
    "SELECT business_key,payload FROM durable_task WHERE kind='RESOURCE_CONFIRM' AND state<>'SUCCEEDED' AND " +
      (id ? 'business_key=?' : 'retry_at<=UTC_TIMESTAMP(6) ORDER BY retry_at LIMIT 32'),
    id ? [id] : [],
  );
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/resource_confirmations.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
