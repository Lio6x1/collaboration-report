# 分步编写 `services/collaboration/src/application/room_snapshots.mjs`

先按 [C01 · 正文契约、隔离候选与连续日志恢复](../lessons/collaboration/01-content-replay.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/341-room-snapshots.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

组织启动恢复、按cut还原和检查点落库。

数据库有检查点行还不足以证明可用；就绪前必须实际解码Yjs并核对连续日志。

写之前先按这条执行路线理解各段的连接：

1. verifyRooms用最多四个I/O通道验证全部保留房间，某个失败后停止启动新任务并等待已有任务释放
2. restore先openCheckpoint，再读取尾部并applyDurableLog；异常销毁doc
3. checkpoint编码状态、插入记录、读取字符串ID并挂接room

本文件需要守住的边界：

- 只有全部房间验证成功才能设置roomsRecovered
- 恢复到检查点精确cut可以省SQL，但仍须正文校验

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/room_snapshots.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/room_snapshots.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/room_snapshots.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/room_snapshots.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:96 -->
````javascript
import { openCheckpoint, applyDurableLog, encodeCheckpoint } from '../domain/room_replay.mjs';

````

## 第 2 步：RoomSnapshots

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`export class RoomSnapshots `。

**这一段为什么这样写：** 让恢复验证与普通SYNC/UPDATE使用同一条持久性路径。

1. verifyRooms恢复后立即销毁临时文档
2. restore成功时把doc所有权交给调用方
3. checkpoint在传入事务连接内完成所有写入


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repository</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.repository = repository</code> |
| <code>store</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.store = store</code> |

**本段进入 RoomSnapshots.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repository</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.repository = repository</code> |
| <code>store</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.store = store</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 RoomSnapshots.verifyRooms：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**本段进入 RoomSnapshots.restore：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.repository.latestCheckpoint(       connection,       room.id,       room.epoch,       cut,     )</code>；<code>this.repository.updatesBetween(                 connection,                 room.id,                 room.epoch,                 checkpoint.covered_seq,                 cut,               )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room.id</code>；<code>room.epoch</code>；<code>applyDurableLog(doc, checkpoint.covered_seq, updates, room, cut)</code> |
| <code>cut = room.last_durable_seq</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>this.repository.latestCheckpoint(       connection,       room.id,       room.epoch,       cut,     )</code>；<code>BigInt(cut)</code>；<code>this.repository.updatesBetween(                 connection,                 room.id,                 room.epoch,                 checkpoint.covered_seq,                 cut,               )</code>；<code>applyDurableLog(doc, checkpoint.covered_seq, updates, room, cut)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>applyDurableLog(doc, checkpoint.covered_seq, updates, room, cut)</code> |

**跟着本段代码走一遍：**

- 等待 <code>this.repository.listRooms()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>this.restore(this.store.pool, room)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!failed</code>。
- 等待 <code>Promise.all(Array.from({ length: 4 }, () =&gt; recover()))</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>failed</code>。条件成立时执行 <code>throw failure;</code>。转入外层异常处理路径。
- 执行 <code>throw failure;</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.repository.latestCheckpoint(       connection,       room.id,       room.epoch,       cut,     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 2 bytes=96:1895 -->
````javascript
export class RoomSnapshots {
  constructor(repository, store) {
    this.repository = repository;
    this.store = store;
  }

  async verifyRooms() {
    const [rooms] = await this.repository.listRooms();
    let cursor = 0,
      failed = false,
      failure;
    const recover = async () => {
      while (!failed && cursor < rooms.length) {
        const room = rooms[cursor++];
        try {
          const doc = await this.restore(this.store.pool, room);
          doc.destroy();
        } catch (error) {
          if (!failed) failure = error;
          failed = true;
        }
      }
    };
    // 旧 owner 已经核验退出，监听尚未开放；不同房间的只读恢复可重叠 I/O。
    // 固定四个I/O通道消化大量保留房间，并限制同时载入的正文数量；
    // 保留每房间 checkpoint→连续日志→内容校验顺序，不把全部房间一起载入内存。
    // 任一路失败后不启动新房间，并等待已开始的恢复完成/释放后才返回失败；绝不提前 READY。
    await Promise.all(Array.from({ length: 4 }, () => recover()));
    if (failed) throw failure;
    this.store.roomsRecovered = true;
  }

  async restore(connection, room, cut = room.last_durable_seq) {
    const [checkpoints] = await this.repository.latestCheckpoint(
      connection,
      room.id,
      room.epoch,
      cut,
    );
    const checkpoint = checkpoints[0],
      doc = openCheckpoint(checkpoint);
    try {
      // 保持原顺序：损坏checkpoint/Yjs先失败，然后才读取固定范围内的实际日志。
      // 检查点已覆盖精确cut时区间为空，省去每个已保存房间的额外往返。
      // Yjs/checksum已验证，正文规则仍在applyDurableLog检查；未覆盖的尾部照常读连续日志。
````

## 第 3 步：RoomSnapshots（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 48 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 RoomSnapshots.checkpoint：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.repository.insertCheckpoint(       connection,       room.id,       room.epoch,       cut,       bytes,       checksum,     )</code>；<code>this.repository.lastInsertedID(connection)</code>；<code>this.repository.attachCheckpoint(connection, id.id, room.id, room.epoch)</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>room.id</code>；<code>room.epoch</code> |
| <code>doc</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>encodeCheckpoint(doc)</code> |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.repository.insertCheckpoint(       connection,       room.id,       room.epoch,       cut,       bytes,       checksum,     )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>String(result.affectedRows)</code> |

**跟着本段代码走一遍：**

- 等待 <code>this.repository.updatesBetween(                 connection,                 room.id,                 room.epoch,                 checkpoint.covered_seq,                 cut,               )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return applyDurableLog(doc, checkpoint.covered_seq, updates, room, cut);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>throw error;</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.repository.insertCheckpoint(       connection,       room.id,       room.epoch,       cut,       bytes,       checksum,     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>this.repository.lastInsertedID(connection)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>this.repository.attachCheckpoint(connection, id.id, room.id, room.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return String(result.affectedRows);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1895:2843 -->
````javascript
      const updates =
        BigInt(checkpoint.covered_seq) === BigInt(cut)
          ? []
          : (
              await this.repository.updatesBetween(
                connection,
                room.id,
                room.epoch,
                checkpoint.covered_seq,
                cut,
              )
            )[0];
      return applyDurableLog(doc, checkpoint.covered_seq, updates, room, cut);
    } catch (error) {
      doc.destroy();
      throw error;
    }
  }

  async checkpoint(connection, room, doc, cut) {
    const { bytes, checksum } = encodeCheckpoint(doc);
    const [result] = await this.repository.insertCheckpoint(
      connection,
      room.id,
      room.epoch,
      cut,
      bytes,
      checksum,
    );
    const [[id]] = await this.repository.lastInsertedID(connection);
    await this.repository.attachCheckpoint(connection, id.id, room.id, room.epoch);
    return String(result.affectedRows);
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/room_snapshots.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/01-content-replay.md)。
