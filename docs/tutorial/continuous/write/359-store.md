# 分步编写 `services/collaboration/src/bootstrap/store.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/359-store.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

建立有界mysql连接池并装配就绪仓储/判定器。

连接耗尽时无限排队会把所有客户端超时叠加，需限制等待且保留真实就绪检查。

写之前先按这条执行路线理解各段的连接：

1. mysql.createPool采用配置并限制queueLimit32/connectTimeout2000
2. 按迁移2是否启用恢复索引优化
3. 返回pool与checkReady方法，让Readiness读取同一store.roomsRecovered

本文件需要守住的边界：

- 数据库BIGINT配置来自config，不能擅改成Number
- checkReady必须用this保持恢复标志来源一致

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/bootstrap' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/bootstrap/store.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/bootstrap/store.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/bootstrap/store.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/bootstrap/store.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:174 -->
````javascript
import mysql from 'mysql2/promise';
import { Readiness as ReadinessRepository } from '../repository/readiness.mjs';
import { Readiness } from '../application/readiness.mjs';
````

## 第 2 步：createStore

插入位置：第 1 步代码之后，紧接追加；完成稿约第 4 行。

本步声明：`export function createStore(config) `。

**这一段为什么这样写：** 组合存储连接与数据库/恢复就绪条件。

1. 创建有限连接等待池
2. 注入预期迁移hash
3. 返回pool和就绪入口


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>config</code> | <code>JavaScript 参数</code> | 数据库配置与expectedSQLSHA/expectedAdditionalMigrations<br>在此声明中的实际使用：<code>config.database</code>；<code>config.expectedAdditionalMigrations</code>；<code>config.expectedSQLSHA</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     pool,     async checkReady() {       await readiness.check(this);     },   }</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return {     pool,     async checkReady() {       await readiness.check(this);     },   };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>readiness.check(this)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=174:643 -->
````javascript
export function createStore(config) {
  const pool = mysql.createPool({
    ...config.database,
    waitForConnections: true,
    queueLimit: 32,
    connectTimeout: 2000,
  });
  const readiness = new Readiness(
    new ReadinessRepository(pool, Boolean(config.expectedAdditionalMigrations?.['2'])),
    config.expectedSQLSHA,
    config.expectedAdditionalMigrations,
  );
  return {
    pool,
    async checkReady() {
      await readiness.check(this);
    },
  };
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/bootstrap/store.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
