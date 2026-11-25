# 分步编写 `services/collaboration/src/application/serialized_rooms.mjs`

先按 [C01 · 正文契约、隔离候选与连续日志恢复](../lessons/collaboration/01-content-replay.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/343-serialized-rooms.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

给每个room提供UPDATE、FLUSH与SYNC共享的串行队列。

同房间并发读取旧last_durable_seq会产生错误顺序；不同房间又应允许并行。

写之前先按这条执行路线理解各段的连接：

1. run把新latch串接到该room的prior之后
2. 先等待prior结束，再执行fn并统计active
3. finally释放自己的latch；仅当仍是最新tail才删除Map项

本文件需要守住的边界：

- prior失败只表示前一任务结束，不应永远阻塞后续任务
- 不能提前delete仍有后继等待的room锁

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/serialized_rooms.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/serialized_rooms.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/serialized_rooms.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/serialized_rooms.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：SerializedRooms

插入位置：空文件第 1 行。

所有读写共用一个房间队列；失败只释放自己的latch，不让后续动作越过尚在提交的动作。

本步声明：`export class SerializedRooms `。

**这一段为什么这样写：** 以Promise链建立进程内房间先后关系，SQL事务继续负责持久原子性。

1. pending在排队前递增
2. await prior后增加active
3. finally同时回收计数和latch


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>locks</code> | <code>实例字段</code> | room到最后一个队列尾Promise的映射<br>在此声明中的实际使用：<code>this.locks = new Map()</code> |
| <code>pending</code> | <code>实例字段</code> | 执行中加排队中的任务总数<br>在此声明中的实际使用：<code>this.pending = 0</code> |
| <code>active</code> | <code>实例字段</code> | 已获得房间执行权的任务数<br>在此声明中的实际使用：<code>this.active = 0</code> |
| <code>peakPending</code> | <code>实例字段</code> | 进程期间排队总量峰值<br>在此声明中的实际使用：<code>this.peakPending = 0</code> |

**本段进入 SerializedRooms.constructor：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 SerializedRooms.run：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.locks.get(room)</code>；<code>this.locks.set(room, tail)</code>；<code>this.locks.delete(room)</code> |
| <code>fn</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>fn()</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>await fn()</code> |

**本段进入 SerializedRooms.metrics：**


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{       pending: this.pending,       active: this.active,       waiting: this.pending - this.active,       peakPending: this.peakPending,       rooms: this.locks.size,     }</code> |

**跟着本段代码走一遍：**

- 等待 <code>prior.catch(() =&gt; {})</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 此处通过 <code>return await fn();</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>fn()</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>this.locks.get(room) === tail</code>。
- 此处通过 <code>return {       pending: this.pending,       active: this.active,       waiting: this.pending - this.active,       peakPending: this.peakPending,       rooms: this.locks.size,     };</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 1 bytes=0:1063 -->
````javascript
// 所有读写共用一个房间队列；失败只释放自己的latch，不让后续动作越过尚在提交的动作。
export class SerializedRooms {
  constructor() {
    this.locks = new Map();
    this.pending = 0;
    this.active = 0;
    this.peakPending = 0;
  }

  async run(room, fn) {
    this.pending++;
    this.peakPending = Math.max(this.peakPending, this.pending);
    const prior = this.locks.get(room) ?? Promise.resolve();
    let release;
    const latch = new Promise((r) => {
      release = r;
    });
    const tail = prior.catch(() => {}).then(() => latch);
    this.locks.set(room, tail);
    await prior.catch(() => {});
    this.active++;
    try {
      return await fn();
    } finally {
      this.active--;
      this.pending--;
      release();
      if (this.locks.get(room) === tail) this.locks.delete(room);
    }
  }

  metrics() {
    return {
      pending: this.pending,
      active: this.active,
      waiting: this.pending - this.active,
      peakPending: this.peakPending,
      rooms: this.locks.size,
    };
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/serialized_rooms.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/01-content-replay.md)。
