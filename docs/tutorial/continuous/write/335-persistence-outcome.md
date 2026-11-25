# 分步编写 `services/collaboration/src/application/persistence_outcome.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/335-persistence-outcome.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

向Doc返回可核验的Node更新提交结果，证据不足保守返回UNKNOWN。

保护已建立但保存响应丢失时，不能因没有收到SAVED而释放可能仍被正文引用的资源。

写之前先按这条执行路线理解各段的连接：

1. 查真实update_log
2. 按room:epoch:seq查确认任务，只有恰好一条才读取固定payload
3. 日志存在且绑定grant_id才返回COMMITTED、seq/hash、保护操作和正文计量
4. 其余路径全部UNKNOWN

本文件需要守住的边界：

- 没有日志并不构成NOT_COMMITTED证明
- 不得用当前grant/正文重新生成历史确认绑定

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/persistence_outcome.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/persistence_outcome.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/persistence_outcome.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/persistence_outcome.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:69 -->
````javascript
import * as repository from '../repository/persistence_outcome.mjs';
````

## 第 2 步：PersistenceOutcome

插入位置：第 1 步代码之后，紧接追加；完成稿约第 2 行。

本步声明：`export class PersistenceOutcome `。

**这一段为什么这样写：** 将本地持久证据翻译成跨服务恢复响应。

1. 日志与唯一确认任务联合验证
2. 返回原update_hash和body_hash
3. 缺证据保留UNKNOWN


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pool</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.pool = pool</code> |

**本段进入 PersistenceOutcome.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pool</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.pool = pool</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 PersistenceOutcome.update：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>repository.committedUpdate(this.pool, input)</code>；<code>input.room_id</code>；<code>input.workspace_epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>row &amp;&amp; binding?.grant_id       ? {           state: &#x27;COMMITTED&#x27;,           durable_seq: row.seq,           update_hash: row.payload_hash.toString(&#x27;hex&#x27;),           protection_opera …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 等待 <code>repository.committedUpdate(this.pool, input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>row</code>。
- 等待 <code>repository.confirmationsForReceipt(         this.pool,         input.room_id + &#x27;:&#x27; + input.workspace_epoch + &#x27;:&#x27; + row.seq,       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>tasks.length === 1</code>。
- 此处通过 <code>return row &amp;&amp; binding?.grant_id       ? {           state: &#x27;COMMITTED&#x27;,           durable_seq: row.seq,           update_hash: row.payload_hash.toString(&#x27;hex&#x27;),           protectio …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=69:1136 -->
````javascript
export class PersistenceOutcome {
  constructor(pool) {
    this.pool = pool;
  }
  async update(input) {
    const row = await repository.committedUpdate(this.pool, input);
    let task;
    if (row) {
      const tasks = await repository.confirmationsForReceipt(
        this.pool,
        input.room_id + ':' + input.workspace_epoch + ':' + row.seq,
      );
      if (tasks.length === 1) task = tasks[0];
    }
    const binding = task
      ? typeof task.payload === 'string'
        ? JSON.parse(task.payload)
        : task.payload
      : null;
    // 行已提交但唯一保护绑定不完整仍返回UNKNOWN，不能把不确定证明转换成可释放资源的成功。
    return row && binding?.grant_id
      ? {
          state: 'COMMITTED',
          durable_seq: row.seq,
          update_hash: row.payload_hash.toString('hex'),
          protection_operation_id: task.business_key,
          grant_id: binding.grant_id,
          body_bytes: binding.body_bytes,
          body_hash: binding.body_hash,
        }
      : { state: 'UNKNOWN' };
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/persistence_outcome.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
