# 分步编写 `services/collaboration/src/repository/room_snapshots.mjs`

先按 [C01 · 正文契约、隔离候选与连续日志恢复](../lessons/collaboration/01-content-replay.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/321-room-snapshots.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取固定cut以内的检查点/日志并写入新检查点。

一次只取最新可用检查点可控制启动内存和SQL时间，仍保留完整尾部日志验证。

写之前先按这条执行路线理解各段的连接：

1. listRooms排除已具成功清理回执的退休房间，普通退休房间仍验证
2. latestCheckpoint要求validated_at且covered_seq<=cut，按序号/id降序取一条
3. updatesBetween只读(after,cut]并ORDER BY seq
4. 插入检查点后在同连接用CAST(LAST_INSERT_ID() AS CHAR)取得身份，再挂到room

本文件需要守住的边界：

- mysql2的insertId可能经过Number舍入，不可用它保存BIGINT检查点身份
- 不能把未validated的检查点纳入恢复

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/repository' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/repository/room_snapshots.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/repository/room_snapshots.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/repository/room_snapshots.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/repository/room_snapshots.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:36 -->
````javascript
import { query } from './sql.mjs';

````

## 第 2 步：snapshotQuery

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

启动失败只给出阶段和稳定码；不能打印包含正文或配置的SQL/驱动原文。

本步声明：`async function snapshotQuery(stage, connection, sql, args = []) `。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>stage</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify({ event: &#x27;room_snapshot_read_failed&#x27;, stage, code })</code> |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, sql, args)</code> |
| <code>sql</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, sql, args)</code> |
| <code>args = &#91;&#93;</code> | <code>带默认值的参数</code> | 在此声明中的实际使用：<code>query(connection, sql, args)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>await query(connection, sql, args)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return await query(connection, sql, args);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>query(connection, sql, args)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === &#x27;1&#x27;</code>。
- 执行 <code>throw error;</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 2 bytes=36:623 -->
````javascript
// 启动失败只给出阶段和稳定码；不能打印包含正文或配置的SQL/驱动原文。
async function snapshotQuery(stage, connection, sql, args = []) {
  try {
    return await query(connection, sql, args);
  } catch (error) {
    if (process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === '1') {
      const code =
        typeof error?.code === 'string' && /^[A-Z][A-Z0-9_]{1,95}$/.test(error.code)
          ? error.code
          : 'SNAPSHOT_READ_UNKNOWN';
      console.error(JSON.stringify({ event: 'room_snapshot_read_failed', stage, code }));
    }
    throw error;
  }
}

````

## 第 3 步：RoomSnapshots

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`export class RoomSnapshots `。

**这一段为什么这样写：** 封装固定范围的快照SQL，不自行决定业务cut。

1. 读取单个最近验证检查点
2. 读取其后有界连续日志
3. 新检查点插入和room挂接由外层事务原子提交


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pool</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.pool = pool</code> |

**本段进入 RoomSnapshots.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pool</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.pool = pool</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 RoomSnapshots.listRooms：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>snapshotQuery(       &#x27;ROOM_LIST&#x27;,       this.pool,       `SELECT r.* FROM room r WHERE NOT (r.state=&#x27;RETIRED&#x27; AND EXISTS (         SELECT 1 FROM durable_task t WHERE t.state=&#x27;SUCCE …</code>（长表达式，按本步完整代码继续） |

**本段进入 RoomSnapshots.latestCheckpoint：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>snapshotQuery(       &#x27;ROOM_CHECKPOINT&#x27;,       connection,       &#x27;SELECT * FROM checkpoint WHERE room_id=? AND epoch=? AND covered_seq&lt;=? AND validated_at IS NOT NULL ORDER BY cover …</code>（长表达式，按本步完整代码继续） |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>snapshotQuery(       &#x27;ROOM_CHECKPOINT&#x27;,       connection,       &#x27;SELECT * FROM checkpoint WHERE room_id=? AND epoch=? AND covered_seq&lt;=? AND validated_at IS NOT NULL ORDER BY cover …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>snapshotQuery(       &#x27;ROOM_CHECKPOINT&#x27;,       connection,       &#x27;SELECT * FROM checkpoint WHERE room_id=? AND epoch=? AND covered_seq&lt;=? AND validated_at IS NOT NULL ORDER BY cover …</code>（长表达式，按本步完整代码继续） |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>snapshotQuery(       &#x27;ROOM_CHECKPOINT&#x27;,       connection,       &#x27;SELECT * FROM checkpoint WHERE room_id=? AND epoch=? AND covered_seq&lt;=? AND validated_at IS NOT NULL ORDER BY cover …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>snapshotQuery(       &#x27;ROOM_CHECKPOINT&#x27;,       connection,       &#x27;SELECT * FROM checkpoint WHERE room_id=? AND epoch=? AND covered_seq&lt;=? AND validated_at IS NOT NULL ORDER BY cover …</code>（长表达式，按本步完整代码继续） |

**本段进入 RoomSnapshots.updatesBetween：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>snapshotQuery(       &#x27;ROOM_UPDATE_LOG&#x27;,       connection,       &#x27;SELECT seq,payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND seq&gt;? AND seq&lt;=? ORDER BY seq&#x27;,       &#91;room, epoch, after, cut&#93;,     )</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>snapshotQuery(       &#x27;ROOM_UPDATE_LOG&#x27;,       connection,       &#x27;SELECT seq,payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND seq&gt;? AND seq&lt;=? ORDER BY seq&#x27;,       &#91;room, epoch, after, cut&#93;,     )</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>snapshotQuery(       &#x27;ROOM_UPDATE_LOG&#x27;,       connection,       &#x27;SELECT seq,payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND seq&gt;? AND seq&lt;=? ORDER BY seq&#x27;,       &#91;room, epoch, after, cut&#93;,     )</code> |
| <code>after</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>snapshotQuery(       &#x27;ROOM_UPDATE_LOG&#x27;,       connection,       &#x27;SELECT seq,payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND seq&gt;? AND seq&lt;=? ORDER BY seq&#x27;,       &#91;room, epoch, after, cut&#93;,     )</code> |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>snapshotQuery(       &#x27;ROOM_UPDATE_LOG&#x27;,       connection,       &#x27;SELECT seq,payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND seq&gt;? AND seq&lt;=? ORDER BY seq&#x27;,       &#91;room, epoch, after, cut&#93;,     )</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>snapshotQuery(       &#x27;ROOM_UPDATE_LOG&#x27;,       connection,       &#x27;SELECT seq,payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND seq&gt;? AND seq&lt;=? ORDER BY seq&#x27;,       &#91;room, epoch, after, cut&#93;,     )</code> |

**本段进入 RoomSnapshots.insertCheckpoint：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(       connection,       &quot;INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,?,?,?,?,&#x27;collab-yjs-v1&#x27;,UTC_TIMESTAMP(6))&quot; …</code>（长表达式，按本步完整代码继续） |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(       connection,       &quot;INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,?,?,?,?,&#x27;collab-yjs-v1&#x27;,UTC_TIMESTAMP(6))&quot; …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(       connection,       &quot;INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,?,?,?,?,&#x27;collab-yjs-v1&#x27;,UTC_TIMESTAMP(6))&quot; …</code>（长表达式，按本步完整代码继续） |
| <code>cut</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(       connection,       &quot;INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,?,?,?,?,&#x27;collab-yjs-v1&#x27;,UTC_TIMESTAMP(6))&quot; …</code>（长表达式，按本步完整代码继续） |
| <code>bytes</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(       connection,       &quot;INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,?,?,?,?,&#x27;collab-yjs-v1&#x27;,UTC_TIMESTAMP(6))&quot; …</code>（长表达式，按本步完整代码继续） |
| <code>checksum</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(       connection,       &quot;INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,?,?,?,?,&#x27;collab-yjs-v1&#x27;,UTC_TIMESTAMP(6))&quot; …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(       connection,       &quot;INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,?,?,?,?,&#x27;collab-yjs-v1&#x27;,UTC_TIMESTAMP(6))&quot; …</code>（长表达式，按本步完整代码继续） |

**本段进入 RoomSnapshots.lastInsertedID：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;SELECT CAST(LAST_INSERT_ID() AS CHAR) AS id&#x27;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(connection, &#x27;SELECT CAST(LAST_INSERT_ID() AS CHAR) AS id&#x27;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return snapshotQuery(       &#x27;ROOM_LIST&#x27;,       this.pool,       `SELECT r.* FROM room r WHERE NOT (r.state=&#x27;RETIRED&#x27; AND EXISTS (         SELECT 1 FROM durable_task t WHERE t.state …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return snapshotQuery(       &#x27;ROOM_CHECKPOINT&#x27;,       connection,       &#x27;SELECT * FROM checkpoint WHERE room_id=? AND epoch=? AND covered_seq&lt;=? AND validated_at IS NOT NULL ORDER B …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return snapshotQuery(       &#x27;ROOM_UPDATE_LOG&#x27;,       connection,       &#x27;SELECT seq,payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND seq&gt;? AND seq&lt;=? ORDER BY se …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return query(       connection,       &quot;INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,?,?,?,?,&#x27;collab-yjs-v1&#x27;,UTC_TIMESTA …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=623:2529 -->
````javascript
export class RoomSnapshots {
  constructor(pool) {
    this.pool = pool;
  }

  listRooms() {
    // 普通退休房间仍检查持久性；仅原清理回执与退休状态同事务提交的房间没有正文。
    return snapshotQuery(
      'ROOM_LIST',
      this.pool,
      `SELECT r.* FROM room r WHERE NOT (r.state='RETIRED' AND EXISTS (
        SELECT 1 FROM durable_task t WHERE t.state='SUCCEEDED' AND (
          (t.kind='ACCOUNT_DOCUMENT_CLEANUP' AND t.business_key=CAST(r.document_id AS CHAR)) OR
          (t.kind='PURGE_DOCUMENT_CLEANUP' AND t.business_key LIKE CONCAT(CAST(r.document_id AS CHAR),':%'))
        )
      )) ORDER BY r.id`,
    );
  }

  latestCheckpoint(connection, room, epoch, cut) {
    // 恢复只使用不越过cut的最新已验证检查点；旧检查点可含多份MiB正文，
    // 全部返回会挤占三秒SQL预算及启动内存，却不参与任何持久性判断。
    return snapshotQuery(
      'ROOM_CHECKPOINT',
      connection,
      'SELECT * FROM checkpoint WHERE room_id=? AND epoch=? AND covered_seq<=? AND validated_at IS NOT NULL ORDER BY covered_seq DESC,id DESC LIMIT 1',
      [room, epoch, cut],
    );
  }

  updatesBetween(connection, room, epoch, after, cut) {
    return snapshotQuery(
      'ROOM_UPDATE_LOG',
      connection,
      'SELECT seq,payload,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND seq>? AND seq<=? ORDER BY seq',
      [room, epoch, after, cut],
    );
  }

  insertCheckpoint(connection, room, epoch, cut, bytes, checksum) {
    return query(
      connection,
      "INSERT INTO checkpoint(room_id,epoch,covered_seq,yjs_state,checksum,format_version,validated_at) VALUES(?,?,?,?,?,'collab-yjs-v1',UTC_TIMESTAMP(6))",
      [room, epoch, cut, bytes, checksum],
    );
  }

  // 同一连接读取BIGINT原值，不能把mysql2浮点insertId当作checkpoint身份。
  lastInsertedID(connection) {
````

## 第 4 步：RoomSnapshots（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 68 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 RoomSnapshots.attachCheckpoint：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>connection</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;UPDATE room SET checkpoint_id=? WHERE id=? AND epoch=?&#x27;, &#91;       id,       room,       epoch,     &#93;)</code> |
| <code>id</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;UPDATE room SET checkpoint_id=? WHERE id=? AND epoch=?&#x27;, &#91;       id,       room,       epoch,     &#93;)</code> |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;UPDATE room SET checkpoint_id=? WHERE id=? AND epoch=?&#x27;, &#91;       id,       room,       epoch,     &#93;)</code> |
| <code>epoch</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>query(connection, &#x27;UPDATE room SET checkpoint_id=? WHERE id=? AND epoch=?&#x27;, &#91;       id,       room,       epoch,     &#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>query(connection, &#x27;UPDATE room SET checkpoint_id=? WHERE id=? AND epoch=?&#x27;, &#91;       id,       room,       epoch,     &#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return query(connection, &#x27;SELECT CAST(LAST_INSERT_ID() AS CHAR) AS id&#x27;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return query(connection, &#x27;UPDATE room SET checkpoint_id=? WHERE id=? AND epoch=?&#x27;, &#91;       id,       room,       epoch,     &#93;);</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2529:2799 -->
````javascript
    return query(connection, 'SELECT CAST(LAST_INSERT_ID() AS CHAR) AS id');
  }

  attachCheckpoint(connection, id, room, epoch) {
    return query(connection, 'UPDATE room SET checkpoint_id=? WHERE id=? AND epoch=?', [
      id,
      room,
      epoch,
    ]);
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/repository/room_snapshots.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/01-content-replay.md)。
