# 分步编写 `services/collaboration/src/repository/output_instances.mjs`

先按 [C03 · 撤权门闩、真实输出排空与进程恢复](../lessons/collaboration/03-fence-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/314-output-instances.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存Node进程输出所有者事实和已核验退出证明。

服务重启后数据库delivery仍可能属于活着的旧进程，必须先确认owner已退出。

写之前先按这条执行路线理解各段的连接：

1. listOutputInstances读取NODE_INSTANCE任务
2. insertOutputInstance记录pid/entry/owner
3. settleExitedOutputs只处理指定instance未结束delivery
4. recordOutputExit保存原实例exit_proof

本文件需要守住的边界：

- settleExitedOutputs和recordOutputExit必须同事务提交
- 不可按最近心跳或启动时间批量结算其他实例

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/output_instances.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/output_instances.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/output_instances.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/output_instances.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:36 -->
````javascript
import { query } from './sql.mjs';

````

## 第 2 步：listOutputInstances

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

本步声明：`export function listOutputInstances(connection) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;NODE_INSTANCE&#x27;&quot;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;NODE_INSTANCE&#x27;&quot;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;NODE_INSTANCE&#x27;&quot;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=36:310 -->
````javascript
// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function listOutputInstances(connection) {
  return query(
    connection,
    "SELECT business_key,payload FROM durable_task WHERE kind='NODE_INSTANCE'",
  );
}

````

## 第 3 步：settleExitedOutputs

插入位置：第 2 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`export function settleExitedOutputs(connection, instance) `。

**这一段为什么这样写：** 仅在外层取得真实退出证据后结算指定旧进程输出。

1. 按instance_id更新未SETTLED记录
2. 原因固定EXIT_CONFIRMED


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 与记录exit_proof相同的事务连接<br>在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;EXIT_CONFIRMED&#x27; WHERE instance_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;instance&#93;,   )</code> |
| <code>instance</code> | <code>JavaScript 参数</code> | 已确认退出的旧进程实例ID<br>在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;EXIT_CONFIRMED&#x27; WHERE instance_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;instance&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;EXIT_CONFIRMED&#x27; WHERE instance_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;instance&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;EXIT_CONFIRMED&#x27; WHERE instance_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;&quot;,     &#91;instance&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=310:547 -->
````javascript
export function settleExitedOutputs(connection, instance) {
  return query(
    connection,
    "UPDATE delivery_session SET state='SETTLED',stop_reason='EXIT_CONFIRMED' WHERE instance_id=? AND state<>'SETTLED'",
    [instance],
  );
}

````

## 第 4 步：recordOutputExit

插入位置：第 3 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`export function recordOutputExit(connection, payload, instance) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET payload=? WHERE kind=&#x27;NODE_INSTANCE&#x27; AND business_key=?&quot;,     &#91;payload, instance&#93;,   )</code> |
| <code>payload</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET payload=? WHERE kind=&#x27;NODE_INSTANCE&#x27; AND business_key=?&quot;,     &#91;payload, instance&#93;,   )</code> |
| <code>instance</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE durable_task SET payload=? WHERE kind=&#x27;NODE_INSTANCE&#x27; AND business_key=?&quot;,     &#91;payload, instance&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;UPDATE durable_task SET payload=? WHERE kind=&#x27;NODE_INSTANCE&#x27; AND business_key=?&quot;,     &#91;payload, instance&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;UPDATE durable_task SET payload=? WHERE kind=&#x27;NODE_INSTANCE&#x27; AND business_key=?&quot;,     &#91;payload, instance&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=547:765 -->
````javascript
export function recordOutputExit(connection, payload, instance) {
  return query(
    connection,
    "UPDATE durable_task SET payload=? WHERE kind='NODE_INSTANCE' AND business_key=?",
    [payload, instance],
  );
}

````

## 第 5 步：insertOutputInstance

插入位置：第 4 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`export function insertOutputInstance(connection, instance, payload) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;NODE_INSTANCE&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;instance, payload&#93;,   )</code> |
| <code>instance</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;NODE_INSTANCE&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;instance, payload&#93;,   )</code> |
| <code>payload</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;NODE_INSTANCE&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;instance, payload&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;NODE_INSTANCE&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;instance, payload&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;NODE_INSTANCE&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;,     &#91;instance, payload&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=765:1030 -->
````javascript
export function insertOutputInstance(connection, instance, payload) {
  return query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('NODE_INSTANCE',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [instance, payload],
  );
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/output_instances.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/03-fence-recovery.md)。
