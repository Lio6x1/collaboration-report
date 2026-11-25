# 分步编写 `services/collaboration/src/repository/readiness.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/317-readiness.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取迁移校验和、待恢复任务/门闩及草稿MAIN门闩关联完整性。

进程能监听不代表可安全服务，丢失原准备/确认关联的PREPARED门闩不能静默忽略。

写之前先按这条执行路线理解各段的连接：

1. versions读schema_migration真实版本及HEX checksum
2. pendingTasks/pendingGates查未终结恢复事实
3. unboundDraftMainGates关联准备、预览/确认与安装，核operation/document/binding/cut一致
4. rooms计数供应用要求实际Yjs恢复

本文件需要守住的边界：

- PREPARED本身可由Go恢复，缺绑定证据才是不可信
- 查询一秒预算失败应保持未就绪，不能扩大期限掩盖索引问题

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/readiness.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/readiness.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/readiness.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/readiness.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：Readiness

插入位置：空文件第 1 行。

就绪查询保留原一秒期限及查询顺序；未恢复任务和门闩不能被启动成功日志替代。

本步声明：`export class Readiness `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pool</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.pool = pool</code> |
| <code>recoveryIndexes</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.recoveryIndexes = recoveryIndexes</code> |

**本段进入 Readiness.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pool</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.pool = pool</code> |
| <code>recoveryIndexes = false</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>this.recoveryIndexes = recoveryIndexes</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 Readiness.versions：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>rows</code> |

**本段进入 Readiness.pendingTasks：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>row.pending</code> |

**本段进入 Readiness.pendingGates：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>row.pending</code> |

**本段进入 Readiness.rooms：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>row.pending</code> |

**本段进入 Readiness.unboundDraftMainGates：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>row.pending</code> |

**跟着本段代码走一遍：**

- 等待 <code>this.pool.query({       sql: &#x27;SELECT version,LOWER(HEX(checksum)) AS checksum FROM schema_migration&#x27;,       timeout: 1000,     })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return rows;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.pool.query({       sql: `SELECT COUNT(*) AS pending FROM durable_task ${this.recoveryIndexes ? &#x27;FORCE INDEX (task_pending)&#x27; : &#x27;&#x27;} WHERE state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;)`,       timeout: 1000,     })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return row.pending;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.pool.query({       sql: `SELECT COUNT(*) AS pending FROM access_gate g ${this.recoveryIndexes ? &#x27;FORCE INDEX (gate_recovery)&#x27; : &#x27;&#x27;} LEFT JOIN authorization_change c ${this.rec …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return row.pending;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.pool.query({       sql: &#x27;SELECT COUNT(*) AS pending FROM room&#x27;,       timeout: 1000,     })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return row.pending;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.pool.query({       sql: `SELECT COUNT(*) AS pending FROM durable_task g         LEFT JOIN durable_task p ON p.kind=&#x27;DRAFT_TRANSITION_PREPARED&#x27; AND p.business_key=g.operation_i …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 1 bytes=0:3673 -->
````javascript
// 就绪查询保留原一秒期限及查询顺序；未恢复任务和门闩不能被启动成功日志替代。
export class Readiness {
  constructor(pool, recoveryIndexes = false) {
    this.pool = pool;
    this.recoveryIndexes = recoveryIndexes;
  }
  async versions() {
    const [rows] = await this.pool.query({
      sql: 'SELECT version,LOWER(HEX(checksum)) AS checksum FROM schema_migration',
      timeout: 1000,
    });
    return rows;
  }
  async pendingTasks() {
    const [[row]] = await this.pool.query({
      sql: `SELECT COUNT(*) AS pending FROM durable_task ${this.recoveryIndexes ? 'FORCE INDEX (task_pending)' : ''} WHERE state NOT IN ('SUCCEEDED','REJECTED')`,
      timeout: 1000,
    });
    return row.pending;
  }
  async pendingGates() {
    const [[row]] = await this.pool.query({
      sql: `SELECT COUNT(*) AS pending FROM access_gate g ${this.recoveryIndexes ? 'FORCE INDEX (gate_recovery)' : ''} LEFT JOIN authorization_change c ${this.recoveryIndexes ? 'FORCE INDEX (auth_recovery)' : ''} ON c.revocation_id=g.last_revocation_id AND c.scope_key=g.scope_key AND c.epoch>=g.max_epoch WHERE g.state='BLOCKED' AND (c.state IS NULL OR c.state<>'SUCCEEDED')`,
      timeout: 1000,
    });
    return row.pending;
  }
  async rooms() {
    const [[row]] = await this.pool.query({
      sql: 'SELECT COUNT(*) AS pending FROM room',
      timeout: 1000,
    });
    return row.pending;
  }

  async unboundDraftMainGates() {
    // PREPARED本身可以由Go原责任方恢复；缺原固定确认/准备/终态安装关联才是不可信的启动状态。
    const [[row]] = await this.pool.query({
      sql: `SELECT COUNT(*) AS pending FROM durable_task g
        LEFT JOIN durable_task p ON p.kind='DRAFT_TRANSITION_PREPARED' AND p.business_key=g.operation_id AND p.operation_id=g.operation_id AND p.state='SUCCEEDED'
        LEFT JOIN durable_task f ON f.kind=CASE JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.binding.context.target.purpose')) WHEN 'REVERSE' THEN 'DRAFT_PREVIEW_FIXED' ELSE 'DRAFT_CONFIRMATION_FIXED' END AND f.business_key=g.operation_id AND f.operation_id=g.operation_id AND f.state='SUCCEEDED'
        LEFT JOIN durable_task i ON i.kind='DRAFT_TRANSITION_INSTALLED' AND i.business_key=g.operation_id AND i.operation_id=g.operation_id AND i.state='SUCCEEDED'
        WHERE g.kind='DRAFT_MAIN_GATE' AND (
          p.id IS NULL OR f.id IS NULL OR
          g.state<>'SUCCEEDED' OR
          COALESCE(JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.operation_id')),'')<>g.operation_id OR
          COALESCE(JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.document_id')),'')<>g.business_key OR
          COALESCE(JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.binding.context.target.document_id')),'')<>g.business_key OR
          COALESCE(JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.binding.context.target.purpose')),'') NOT IN ('DISCARD_DRAFT','REVERSE') OR
          COALESCE(JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.state')),'') NOT IN ('PREPARED','INSTALLED','ABORTED') OR
          COALESCE(JSON_EXTRACT(g.payload,'$.binding'),JSON_OBJECT())<>COALESCE(JSON_EXTRACT(p.payload,'$.binding'),JSON_OBJECT()) OR
          COALESCE(JSON_EXTRACT(p.payload,'$.binding'),JSON_OBJECT())<>COALESCE(JSON_EXTRACT(f.payload,'$.binding'),JSON_OBJECT()) OR
          COALESCE(JSON_EXTRACT(p.payload,'$.draft_cut'),JSON_OBJECT())<>COALESCE(JSON_EXTRACT(f.payload,'$.draft_cut'),JSON_OBJECT()) OR
          COALESCE(JSON_EXTRACT(p.payload,'$.main_cut'),JSON_ARRAY())<>COALESCE(JSON_EXTRACT(f.payload,'$.main_cut'),JSON_ARRAY()) OR
          (JSON_UNQUOTE(JSON_EXTRACT(g.payload,'$.state')) IN ('INSTALLED','ABORTED') AND i.id IS NULL)
        )`,
      timeout: 1000,
    });
````

## 第 2 步：Readiness（接着实现）

插入位置：第 1 步代码之后，紧接追加；完成稿约第 59 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return row.pending;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=3673:3703 -->
````javascript
    return row.pending;
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/readiness.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
