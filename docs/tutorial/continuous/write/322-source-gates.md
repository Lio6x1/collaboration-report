# 分步编写 `services/collaboration/src/repository/source_gates.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/322-source-gates.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久化并锁住权限来源(scope_key,epoch)门闩。

撤权和每次输出必须竞争同一行锁，才能确定“关闭之前/之后”的明确边界。

写之前先按这条执行路线理解各段的连接：

1. ensureSourceGates批量建立缺失OPEN行但不覆盖旧状态
2. lockSourceGates按主键scope_key排序FOR UPDATE
3. 只有应用验证更高epoch后才能reopenSourceGate并递增revision

本文件需要守住的边界：

- 来源先按ASCII去重排序再锁，避免多来源相反锁序
- UPSERT冲突分支不得把旧BLOCKED重开

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/source_gates.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/source_gates.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/source_gates.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/source_gates.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:36 -->
````javascript
import { query } from './sql.mjs';

````

## 第 2 步：ensureSourceGate

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

本步声明：`export function ensureSourceGate(connection, scope, epoch) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;scope, epoch&#93;,   )</code> |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;scope, epoch&#93;,   )</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;scope, epoch&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;scope, epoch&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;,     &#91;scope, epoch&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=36:390 -->
````javascript
// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function ensureSourceGate(connection, scope, epoch) {
  return query(
    connection,
    "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)",
    [scope, epoch],
  );
}

````

## 第 3 步：lockSourceGate

插入位置：第 2 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`export function lockSourceGate(connection, scope) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;, &#91;     scope,   &#93;)</code> |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;, &#91;     scope,   &#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(connection, &#x27;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;, &#91;     scope,   &#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(connection, &#x27;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&#x27;, &#91;     scope,   &#93;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=390:563 -->
````javascript
export function lockSourceGate(connection, scope) {
  return query(connection, 'SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE', [
    scope,
  ]);
}

````

## 第 4 步：reopenSourceGate

插入位置：第 3 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`export function reopenSourceGate(connection, epoch, scope) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,revision=revision+1 WHERE scope_key=?&quot;,     &#91;epoch, scope&#93;,   )</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,revision=revision+1 WHERE scope_key=?&quot;,     &#91;epoch, scope&#93;,   )</code> |
| <code>scope</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,revision=revision+1 WHERE scope_key=?&quot;,     &#91;epoch, scope&#93;,   )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,revision=revision+1 WHERE scope_key=?&quot;,     &#91;epoch, scope&#93;,   )</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,revision=revision+1 WHERE scope_key=?&quot;,     &#91;epoch, scope&#93;,   );</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=563:870 -->
````javascript
export function reopenSourceGate(connection, epoch, scope) {
  return query(
    connection,
    "UPDATE access_gate SET max_epoch=?,state='OPEN',revision=revision+1 WHERE scope_key=?",
    [epoch, scope],
  );
}

// 调用方先按ASCII顺序去重；多行UPSERT仍逐键取得排他锁，值不改变。
````

## 第 5 步：ensureSourceGates

插入位置：第 4 步代码之后，紧接追加；完成稿约第 28 行。

随后同事务当前读，减少每个广播读者逐来源两次SQL往返的放大。

本步声明：`export function ensureSourceGates(connection, sources) `。

**这一段为什么这样写：** 一次建立所有来源占位行，同时保留已有关闭事实。

1. 按sources展开占位符和参数
2. ON DUPLICATE只写回相同scope_key，不覆盖epoch/state


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 持有当前授权事务的连接<br>在此声明中的实际使用：<code>query(     connection,     &#x27;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES &#x27; +       sources.map(() =&gt; &quot;(?,?,&#x27;OPEN&#x27;)&quot;).join(&#x27;,&#x27;) +       &#x27; ON DUPLICATE KEY UPDATE scope_ …</code>（长表达式，按本步完整代码继续） |
| <code>sources</code> | <code>JavaScript 参数</code> | 已去重、按ASCII排序的scope_key/epoch数组<br>在此声明中的实际使用：<code>sources.map</code>；<code>sources.flatMap</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &#x27;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES &#x27; +       sources.map(() =&gt; &quot;(?,?,&#x27;OPEN&#x27;)&quot;).join(&#x27;,&#x27;) +       &#x27; ON DUPLICATE KEY UPDATE scope_ …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &#x27;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES &#x27; +       sources.map(() =&gt; &quot;(?,?,&#x27;OPEN&#x27;)&quot;).join(&#x27;,&#x27;) +       &#x27; ON DUPLICATE KEY UPDATE …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=870:1306 -->
````javascript
// 随后同事务当前读，减少每个广播读者逐来源两次SQL往返的放大。
export function ensureSourceGates(connection, sources) {
  return query(
    connection,
    'INSERT INTO access_gate(scope_key,max_epoch,state) VALUES ' +
      sources.map(() => "(?,?,'OPEN')").join(',') +
      ' ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)',
    sources.flatMap((source) => [source.scope_key, source.epoch]),
  );
}

````

## 第 6 步：lockSourceGates

插入位置：第 5 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`export function lockSourceGates(connection, sources) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(     connection,     &#x27;SELECT scope_key,max_epoch,state FROM access_gate FORCE INDEX(PRIMARY) WHERE scope_key IN (&#x27; +       sources.map(() =&gt; &#x27;?&#x27;).join(&#x27;,&#x27;) +       &#x27;) ORDER B …</code>（长表达式，按本步完整代码继续） |
| <code>sources</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>sources.map</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(     connection,     &#x27;SELECT scope_key,max_epoch,state FROM access_gate FORCE INDEX(PRIMARY) WHERE scope_key IN (&#x27; +       sources.map(() =&gt; &#x27;?&#x27;).join(&#x27;,&#x27;) +       &#x27;) ORDER B …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(     connection,     &#x27;SELECT scope_key,max_epoch,state FROM access_gate FORCE INDEX(PRIMARY) WHERE scope_key IN (&#x27; +       sources.map(() =&gt; &#x27;?&#x27;).join(&#x27;,&#x27;) +       &#x27;)  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1306:1629 -->
````javascript
export function lockSourceGates(connection, sources) {
  return query(
    connection,
    'SELECT scope_key,max_epoch,state FROM access_gate FORCE INDEX(PRIMARY) WHERE scope_key IN (' +
      sources.map(() => '?').join(',') +
      ') ORDER BY scope_key FOR UPDATE',
    sources.map((source) => source.scope_key),
  );
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/source_gates.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
