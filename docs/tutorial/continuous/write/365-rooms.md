# 分步编写 `services/collaboration/src/rooms.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/365-rooms.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供兼容构造器，将旧RoomEngine(store,cfg,peers)调用接到新装配。

既有测试和调用者无需同时改所有导入，同时业务实现保持分层。

写之前先按这条执行路线理解各段的连接：

1. 重导出hash/query/lockSources兼容能力
2. RoomEngine继承应用层类，构造时调用createRoomComponents

本文件需要守住的边界：

- 兼容层不应再添加房间业务或维护独立Map
- 构造依赖发生变化要改room_components中的实际装配

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/rooms.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/rooms.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/rooms.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/rooms.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:153 -->
````javascript
import { RoomEngine as ApplicationEngine } from './application/room_engine.mjs';
import { createRoomComponents } from './bootstrap/room_components.mjs';
````

## 第 2 步：statement-153

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`export { hash } from './domain/shared_protocol.mjs';`。

<!-- write-step: 2 bytes=153:206 -->
````javascript
export { hash } from './domain/shared_protocol.mjs';
````

## 第 3 步：statement-206

插入位置：第 2 步代码之后，紧接追加；完成稿约第 4 行。

本步声明：`export { query } from './repository/sql.mjs';`。

<!-- write-step: 3 bytes=206:252 -->
````javascript
export { query } from './repository/sql.mjs';
````

## 第 4 步：statement-252

插入位置：第 3 步代码之后，紧接追加；完成稿约第 5 行。

本步声明：`export { lockSources } from './application/admission_gates.mjs';`。

<!-- write-step: 4 bytes=252:318 -->
````javascript
export { lockSources } from './application/admission_gates.mjs';

````

## 第 5 步：RoomEngine

插入位置：第 4 步代码之后，紧接追加；完成稿约第 7 行。

兼容既有构造调用；实现已经按用例、领域、仓储、运输和恢复分别归属。

本步声明：`export class RoomEngine extends ApplicationEngine `。

**本段进入 RoomEngine.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>createRoomComponents(store, cfg, peers)</code> |
| <code>cfg</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>createRoomComponents(store, cfg, peers)</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>createRoomComponents(store, cfg, peers)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

<!-- write-step: 5 bytes=318:566 -->
````javascript
// 兼容既有构造调用；实现已经按用例、领域、仓储、运输和恢复分别归属。
export class RoomEngine extends ApplicationEngine {
  constructor(store, cfg, peers) {
    super(createRoomComponents(store, cfg, peers));
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/rooms.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
