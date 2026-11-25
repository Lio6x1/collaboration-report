# 分步编写 `services/collaboration/src/application/readiness.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/337-readiness.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将数据库版本、恢复任务和房间恢复结果合并为就绪判定。

既要校验版本号，也要校验SQL内容hash和连续完整版本集合，防止错库或改写迁移。

写之前先按这条执行路线理解各段的连接：

1. check核expected版本从1连续、无重复且checksum逐条相等
2. 依次拒绝pendingTasks、未完成gate及不完整DRAFT_MAIN_GATE
3. 数据库有room且store.roomsRecovered不真则ROOM_RECOVERY_REQUIRED

本文件需要守住的边界：

- 不能只比较迁移数量或最新版本
- 附加迁移不得覆盖1；启动日志不能替代roomsRecovered

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/readiness.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/readiness.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/readiness.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/readiness.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：Readiness

插入位置：空文件第 1 行。

本步声明：`export class Readiness `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repository</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.repository = repository</code> |
| <code>expectedSQLSHA</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.expectedSQLSHA = expectedSQLSHA</code> |
| <code>expectedAdditionalMigrations</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.expectedAdditionalMigrations = { ...expectedAdditionalMigrations }</code> |

**本段进入 Readiness.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repository</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.repository = repository</code> |
| <code>expectedSQLSHA</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.expectedSQLSHA = expectedSQLSHA</code> |
| <code>expectedAdditionalMigrations = {}</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>this.expectedAdditionalMigrations = { ...expectedAdditionalMigrations }</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 Readiness.check：**

**这一段为什么这样写：** 只在所有持久安全前置条件成立后允许READY。

1. 验证迁移集合与hash
2. 核恢复任务和门闩
3. 核房间已实际恢复


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>JavaScript 参数</code> | 包含roomsRecovered恢复标志的存储对象<br>在此声明中的实际使用：<code>store.roomsRecovered</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>this.repository.versions()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>Object.hasOwn(this.expectedAdditionalMigrations, &#x27;1&#x27;) &#124;&#124;       versions.length !== keys.length &#124;&#124;       Object.values(expected).some(         (checksum) =&gt; typeof checksum !== &#x27;str …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw Error(&#x27;SCHEMA_VERSION_MISMATCH&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;SCHEMA_VERSION_MISMATCH&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>Number(await this.repository.pendingTasks()) !== 0</code>。条件成立时执行 <code>throw Error(&#x27;RECOVERY_REQUIRED&#x27;);</code>。转入外层异常处理路径。
- 等待 <code>this.repository.pendingTasks()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 执行 <code>throw Error(&#x27;RECOVERY_REQUIRED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>Number(await this.repository.pendingGates()) !== 0</code>。条件成立时执行 <code>throw Error(&#x27;GATE_RECOVERY_REQUIRED&#x27;);</code>。转入外层异常处理路径。
- 等待 <code>this.repository.pendingGates()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 执行 <code>throw Error(&#x27;GATE_RECOVERY_REQUIRED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>Number(await this.repository.unboundDraftMainGates()) !== 0</code>。条件成立时执行 <code>throw Error(&#x27;DRAFT_MAIN_GATE_BINDING_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 等待 <code>this.repository.unboundDraftMainGates()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 执行 <code>throw Error(&#x27;DRAFT_MAIN_GATE_BINDING_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>Number(await this.repository.rooms()) !== 0 &amp;&amp; !store.roomsRecovered</code>。条件成立时执行 <code>throw Error(&#x27;ROOM_RECOVERY_REQUIRED&#x27;);</code>。转入外层异常处理路径。
- 等待 <code>this.repository.rooms()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 执行 <code>throw Error(&#x27;ROOM_RECOVERY_REQUIRED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 1 bytes=0:1730 -->
````javascript
export class Readiness {
  constructor(repository, expectedSQLSHA, expectedAdditionalMigrations = {}) {
    this.repository = repository;
    this.expectedSQLSHA = expectedSQLSHA;
    this.expectedAdditionalMigrations = { ...expectedAdditionalMigrations };
  }
  async check(store) {
    // 版本是INT，BIGINT数量允许字符串；摘要从仓储HEX取得，避免二进制编码歧义。
    const versions = await this.repository.versions();
    const expected = { 1: this.expectedSQLSHA, ...this.expectedAdditionalMigrations };
    // 附加版本必须构成完整连续集合；按原版本身份比较，不信返回行序或数据库自报期望。
    const keys = Object.keys(expected);
    if (
      Object.hasOwn(this.expectedAdditionalMigrations, '1') ||
      versions.length !== keys.length ||
      Object.values(expected).some(
        (checksum) => typeof checksum !== 'string' || !/^[a-f0-9]{64}$/.test(checksum),
      ) ||
      keys.some((_, index) => !Object.hasOwn(expected, String(index + 1))) ||
      versions.some(
        (row) => !Number.isInteger(row.version) || expected[String(row.version)] !== row.checksum,
      ) ||
      new Set(versions.map((row) => row.version)).size !== versions.length
    )
      throw Error('SCHEMA_VERSION_MISMATCH');
    if (Number(await this.repository.pendingTasks()) !== 0) throw Error('RECOVERY_REQUIRED');
    if (Number(await this.repository.pendingGates()) !== 0) throw Error('GATE_RECOVERY_REQUIRED');
    if (Number(await this.repository.unboundDraftMainGates()) !== 0)
      throw Error('DRAFT_MAIN_GATE_BINDING_UNKNOWN');
    if (Number(await this.repository.rooms()) !== 0 && !store.roomsRecovered)
      throw Error('ROOM_RECOVERY_REQUIRED');
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/readiness.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
