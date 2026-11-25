# 分步编写 `services/collaboration/src/application/room_io.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/340-room-io.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现UPDATE持久保存ACK、FLUSH依赖屏障、SYNC与在线状态广播。

SAVED必须证明更新真正提交；FLUSH必须固定点击时依赖，不能以WebSocket发送成功代替磁盘事实。

写之前先按这条执行路线理解各段的连接：

1. RoomUpdates先验证base64规范和1MiB上限，在队列/事务内authorized并按update ID查重
2. 恢复host后prepareCandidate验证真实Yjs结果；Doc protect-body先冻结图片与正文计量
3. 同事务插入update_log、推进seq、写RESOURCE_CONFIRM；每32次写检查点
4. COMMIT成功后才broadcast UPDATE，再在房间队列外尝试资源确认并返回SAVED
5. RoomReading.flush固定依赖指纹，缺日志返回WAITING，hash冲突拒绝，全部满足才固定cut并返回FLUSHED
6. sync恢复已授权cut；awareness限制4096字节并重验授权后广播

本文件需要守住的边界：

- UPDATE_ID_CONFLICT必须拒绝同号不同字节；不能分配新ID掩盖冲突
- COMMIT未知、广播失败均不能提前给SAVED，客户端应同号查询/重试
- FLUSH依赖缺失不能返回已保存，也不能悄悄改成当前可见依赖

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/room_io.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/room_io.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/room_io.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/room_io.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:880 -->
````javascript
import { bad } from '../domain/shared_protocol.mjs';
import { canonical } from '../../../../packages/content/index.mjs';
import { deny } from '../domain/shared_protocol.mjs';
import * as flushReceipts from '../repository/room_persistence.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import { json } from '../domain/shared_protocol.mjs';
import { performance } from 'node:perf_hooks';
import { prepareCandidate } from '../../../../packages/content/index.mjs';
import * as resourceConfirmations from '../repository/resource_confirmations.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { timingSafeEqual } from 'node:crypto';
import * as updateLog from '../repository/room_persistence.mjs';
import { validID } from '../domain/shared_protocol.mjs';
import * as Y from 'yjs';

// 来源 room_reading.mjs：原业务声明与函数体保留。

````

## 第 2 步：RoomReading

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

FLUSH固定原点击依赖和实际持久cut；SYNC也在当前准入与同一房间队列中读取连续日志。

本步声明：`export class RoomReading `。

**这一段为什么这样写：** 让点击保存产生可用于送审的固定证据。

1. flush对依赖排序、拒绝重复并计算actor/room/epoch绑定指纹
2. 复用同号FIXED回执；否则逐条查更新与hash
3. 实际restore验证cut后写flush_receipt
4. sync返回相同授权边界内的完整已持久状态


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>queue</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.queue = queue</code> |
| <code>snapshots</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>admissions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.admissions = admissions</code> |

**本段进入 RoomReading.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>queue</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.queue = queue</code> |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>admissions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions = admissions</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 RoomReading.flush：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>b</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>b.grant</code>；<code>b.room</code>；<code>b.epoch</code>；<code>this.admissions.authorized(c, b, input)</code> |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.operation_id</code>；<code>input.dependencies</code>；<code>this.admissions.authorized(c, b, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.queue.run(b.room, () =&gt;       this.transactions.run(async (c) =&gt; {         const room = await this.admissions.authorized(c, b, input),           &#91;prior&#93; = await flushReceipts. …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>!validID(input.operation_id) &#124;&#124;       !Array.isArray(input.dependencies) &#124;&#124;       input.dependencies.length &gt; 10000</code>。条件成立时执行 <code>throw bad(&#x27;FLUSH_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;FLUSH_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>!validID(d.client_session_id) &#124;&#124;           !validID(d.client_update_id) &#124;&#124;           !/^&#91;a-f0-9&#93;{64}$/.test(d.update_hash)</code>。条件成立时执行 <code>throw bad(&#x27;FLUSH_INVALID&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw bad(&#x27;FLUSH_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return {           client_session_id: d.client_session_id,           client_update_id: d.client_update_id,           update_hash: d.update_hash,         };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>new Set(deps.map((d) =&gt; d.client_session_id + &#x27;\n&#x27; + d.client_update_id)).size !== deps.length</code>。条件成立时执行 <code>throw bad(&#x27;FLUSH_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;FLUSH_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return this.queue.run(b.room, () =&gt;       this.transactions.run(async (c) =&gt; {         const room = await this.admissions.authorized(c, b, input),           &#91;prior&#93; = await flushRe …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=880:2316 -->
````javascript
// FLUSH固定原点击依赖和实际持久cut；SYNC也在当前准入与同一房间队列中读取连续日志。
export class RoomReading {
  constructor(transactions, queue, snapshots, admissions) {
    this.transactions = transactions;
    this.queue = queue;
    this.snapshots = snapshots;
    this.admissions = admissions;
  }

  async flush(b, input) {
    if (
      !validID(input.operation_id) ||
      !Array.isArray(input.dependencies) ||
      input.dependencies.length > 10000
    )
      throw bad('FLUSH_INVALID');
    const deps = input.dependencies
      .map((d) => {
        if (
          !validID(d.client_session_id) ||
          !validID(d.client_update_id) ||
          !/^[a-f0-9]{64}$/.test(d.update_hash)
        )
          throw bad('FLUSH_INVALID');
        return {
          client_session_id: d.client_session_id,
          client_update_id: d.client_update_id,
          update_hash: d.update_hash,
        };
      })
      .sort((a, z) => canonical(a).localeCompare(canonical(z)));
    if (
      new Set(deps.map((d) => d.client_session_id + '\n' + d.client_update_id)).size !== deps.length
    )
      throw bad('FLUSH_INVALID');
    const fingerprint = hash(
      canonical({
        actor: b.grant.context.actor_id,
        room: b.room,
        epoch: b.epoch,
        dependencies: deps,
      }),
    );
    return this.queue.run(b.room, () =>
      this.transactions.run(async (c) => {
````

## 第 3 步：RoomReading（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 63 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>this.admissions.authorized(c, b, input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>flushReceipts.lockFlushBinding(c, input.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>prior.length</code>。
- 在本段的内部函数内：检查 <code>json(prior&#91;0&#93;.payload).fingerprint !== fingerprint.toString(&#x27;hex&#x27;)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;FLUSH_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;FLUSH_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>flushReceipts.insertFlushBinding(             c,             input.operation_id,             JSON.stringify({ fingerprint: fingerprint.toString(&#x27;hex&#x27;) }),           )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>flushReceipts.findFlushReceipt(c, input.operation_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>receipts.length</code>。
- 在本段的内部函数内：检查 <code>r.state !== &#x27;FIXED&#x27; &#124;&#124; r.actor_id !== b.grant.context.actor_id</code>。条件成立时执行 <code>throw deny(&#x27;FLUSH_CLOSED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;FLUSH_CLOSED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return {             type: &#x27;FLUSHED&#x27;,             receipt_id: r.receipt_id,             covered_cut: json(r.covered_cut),             dependency_digest: r.dependency_digest.toString(&#x27;hex&#x27;),           };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>updateLog.findDependencyHash(             c,             b.room,             b.epoch,             d.client_session_id,             d.client_update_id,           )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!found</code>。条件成立时执行 <code>return {               type: &#x27;WAITING&#x27;,               operation_id: input.operation_id,               code: &#x27;DEPENDENCIES_MISSING&#x27;,             };</code>。当前函数在这里结束。
- 在本段的内部函数内：检查 <code>found.payload_hash.toString(&#x27;hex&#x27;) !== d.update_hash</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;DEPENDENCY_HASH_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;DEPENDENCY_HASH_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>this.snapshots.restore(c, room)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=2316:4171 -->
````javascript
        const room = await this.admissions.authorized(c, b, input),
          [prior] = await flushReceipts.lockFlushBinding(c, input.operation_id);
        if (prior.length) {
          if (json(prior[0].payload).fingerprint !== fingerprint.toString('hex'))
            throw new RPCFailure(6, 'FLUSH_ID_CONFLICT');
        } else
          await flushReceipts.insertFlushBinding(
            c,
            input.operation_id,
            JSON.stringify({ fingerprint: fingerprint.toString('hex') }),
          );
        const [receipts] = await flushReceipts.findFlushReceipt(c, input.operation_id);
        if (receipts.length) {
          const r = receipts[0];
          if (r.state !== 'FIXED' || r.actor_id !== b.grant.context.actor_id)
            throw deny('FLUSH_CLOSED');
          return {
            type: 'FLUSHED',
            receipt_id: r.receipt_id,
            covered_cut: json(r.covered_cut),
            dependency_digest: r.dependency_digest.toString('hex'),
          };
        }
        for (const d of deps) {
          const [[found]] = await updateLog.findDependencyHash(
            c,
            b.room,
            b.epoch,
            d.client_session_id,
            d.client_update_id,
          );
          if (!found)
            return {
              type: 'WAITING',
              operation_id: input.operation_id,
              code: 'DEPENDENCIES_MISSING',
            };
          if (found.payload_hash.toString('hex') !== d.update_hash)
            throw new RPCFailure(6, 'DEPENDENCY_HASH_CONFLICT');
        }
        const doc = await this.snapshots.restore(c, room);
        doc.destroy();
        const cut = {
            room_id: b.room,
            workspace_epoch: b.epoch,
            durable_seq: String(room.last_durable_seq),
          },
          dependencyDigest = hash(canonical(deps));
````

## 第 4 步：RoomReading（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 111 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 RoomReading.sync：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>b</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>b.room</code>；<code>this.admissions.authorized(c, b, input)</code>；<code>b.epoch</code> |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions.authorized(c, b, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.queue.run(b.room, () =&gt;       this.transactions.run(async (c) =&gt; {         const room = await this.admissions.authorized(c, b, input),           doc = await this.snapshots.res …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>flushReceipts.insertFlushReceipt(           c,           input.operation_id,           b.grant.grant_id,           b.grant.context.actor_id,           b.room,           b.epoch,    …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return {           type: &#x27;FLUSHED&#x27;,           receipt_id: input.operation_id,           covered_cut: cut,           dependency_digest: dependencyDigest.toString(&#x27;hex&#x27;),         };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return this.queue.run(b.room, () =&gt;       this.transactions.run(async (c) =&gt; {         const room = await this.admissions.authorized(c, b, input),           doc = await this.snapsh …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>this.admissions.authorized(c, b, input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>this.snapshots.restore(c, room)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return {             type: &#x27;SYNC&#x27;,             room_id: b.room,             workspace_epoch: b.epoch,             durable_seq: String(room.last_durable_seq),             update: Bu …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=4171:5295 -->
````javascript
        await flushReceipts.insertFlushReceipt(
          c,
          input.operation_id,
          b.grant.grant_id,
          b.grant.context.actor_id,
          b.room,
          b.epoch,
          JSON.stringify(cut),
          dependencyDigest,
        );
        return {
          type: 'FLUSHED',
          receipt_id: input.operation_id,
          covered_cut: cut,
          dependency_digest: dependencyDigest.toString('hex'),
        };
      }),
    );
  }

  async sync(b, input) {
    return this.queue.run(b.room, () =>
      this.transactions.run(async (c) => {
        const room = await this.admissions.authorized(c, b, input),
          doc = await this.snapshots.restore(c, room);
        try {
          return {
            type: 'SYNC',
            room_id: b.room,
            workspace_epoch: b.epoch,
            durable_seq: String(room.last_durable_seq),
            update: Buffer.from(Y.encodeStateAsUpdate(doc)).toString('base64'),
          };
        } finally {
          doc.destroy();
        }
      }),
    );
  }
}

// 来源 room_updates.mjs：原业务声明与函数体保留。

````

## 第 5 步：RoomUpdates

插入位置：第 4 步代码之后，紧接追加；完成稿约第 154 行。

UPDATE依次经过隔离候选、Doc保护、同事务日志/确认任务、COMMIT与广播，最后才构成SAVED。

本步声明：`export class RoomUpdates `。

**这一段为什么这样写：** 把隔离验证、资源保护、本地提交和客户端ACK按不能交换的顺序连接。

1. 解析字节并散列
2. 同号同hash直接返回原seq，不重复广播
3. 新更新只应用到独立candidate，保护成功后才写日志
4. COMMIT返回后广播，最后构造SAVED
5. 确认Doc的重试在队列外进行，持久任务和pin保障恢复


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>peers</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>queue</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.queue = queue</code> |
| <code>snapshots</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>admissions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.admissions = admissions</code> |
| <code>outputs</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.outputs = outputs</code> |
| <code>confirmations</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.confirmations = confirmations</code> |

**本段进入 RoomUpdates.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>queue</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.queue = queue</code> |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |
| <code>admissions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions = admissions</code> |
| <code>outputs</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.outputs = outputs</code> |
| <code>confirmations</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.confirmations = confirmations</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 RoomUpdates.update：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>b</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>b.room</code>；<code>this.admissions.authorized(c, b, input)</code>；<code>b.epoch</code>；<code>b.client</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.client_update_id</code>；<code>input.update</code>；<code>this.admissions.authorized(c, b, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>this.queue       .run(b.room, async () =&gt; {         measure(&#x27;queued&#x27;);         const result = await this.transactions.run(async (c) =&gt; {           const room = await this.admission …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>!validID(input.client_update_id) &#124;&#124;       typeof input.update !== &#x27;string&#x27; &#124;&#124;       input.update.length &gt; 1400000</code>。条件成立时执行 <code>throw bad(&#x27;UPDATE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;UPDATE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>!bytes.length &#124;&#124; bytes.length &gt; 1 &lt;&lt; 20 &#124;&#124; bytes.toString(&#x27;base64&#x27;) !== input.update</code>。条件成立时执行 <code>throw bad(&#x27;UPDATE_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;UPDATE_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>!diagnostics</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。
- 此处通过 <code>return this.queue       .run(b.room, async () =&gt; {         measure(&#x27;queued&#x27;);         const result = await this.transactions.run(async (c) =&gt; {           const room = await this.ad …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>this.transactions.run(async (c) =&gt; {           const room = await this.admissions.authorized(c, b, input),             &#91;prior&#93; = await updateLog.findUpdate(               c,        …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>this.admissions.authorized(c, b, input)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>updateLog.findUpdate(               c,               b.room,               b.epoch,               b.client,               input.client_update_id,             )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 5 bytes=5295:6917 -->
````javascript
// UPDATE依次经过隔离候选、Doc保护、同事务日志/确认任务、COMMIT与广播，最后才构成SAVED。
export class RoomUpdates {
  constructor(peers, transactions, queue, snapshots, admissions, outputs, confirmations) {
    this.peers = peers;
    this.transactions = transactions;
    this.queue = queue;
    this.snapshots = snapshots;
    this.admissions = admissions;
    this.outputs = outputs;
    this.confirmations = confirmations;
  }

  async update(b, input) {
    if (
      !validID(input.client_update_id) ||
      typeof input.update !== 'string' ||
      input.update.length > 1400000
    )
      throw bad('UPDATE_INVALID');
    const bytes = Buffer.from(input.update, 'base64');
    if (!bytes.length || bytes.length > 1 << 20 || bytes.toString('base64') !== input.update)
      throw bad('UPDATE_INVALID');
    const digest = hash(bytes);
    const diagnostics = process.env.COLLAB_SUPERVISOR_DIAGNOSTICS === '1';
    const phases = {};
    let last = performance.now();
    const measure = (phase) => {
      if (!diagnostics) return;
      const now = performance.now();
      phases[phase] = Math.round(now - last);
      last = now;
    };
    return this.queue
      .run(b.room, async () => {
        measure('queued');
        const result = await this.transactions.run(async (c) => {
          const room = await this.admissions.authorized(c, b, input),
            [prior] = await updateLog.findUpdate(
              c,
              b.room,
              b.epoch,
              b.client,
              input.client_update_id,
            );
          measure('authorized');
````

## 第 6 步：RoomUpdates（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 199 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：检查 <code>prior.length</code>。
- 在本段的内部函数内：检查 <code>!timingSafeEqual(prior&#91;0&#93;.payload_hash, digest)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;UPDATE_ID_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(6, &#x27;UPDATE_ID_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：此处通过 <code>return { seq: prior&#91;0&#93;.seq, duplicate: true };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>this.snapshots.restore(c, room)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>this.peers.http(               &#x27;/infra/draft/protect-body&#x27;,               {                 protection: {                   context: { ...context, request_fingerprint: digest.toStr …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>protection.state !== &#x27;PROTECTED&#x27; &amp;&amp; protection.state !== &#x27;CONFIRMED&#x27;</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;RESOURCE_PROTECTION_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw new RPCFailure(14, &#x27;RESOURCE_PROTECTION_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 6 bytes=6917:9046 -->
````javascript
          if (prior.length) {
            if (!timingSafeEqual(prior[0].payload_hash, digest))
              throw new RPCFailure(6, 'UPDATE_ID_CONFLICT');
            return { seq: prior[0].seq, duplicate: true };
          }
          const host = await this.snapshots.restore(c, room);
          measure('restored');
          let candidate;
          try {
            candidate = prepareCandidate(host, bytes, { workspaceKind: room.workspace_kind });
            measure('candidate');
            const protectId = hash(
              [b.room, b.epoch, b.client, input.client_update_id, b.grant.grant_id].join('\n'),
            ).toString('hex');
            const context = {
              ...b.grant.context,
              operation_id: protectId,
              request_fingerprint: digest,
            };
            const bodyBytes = String(Buffer.byteLength(candidate.canonical));
            const bodyHash = hash(candidate.canonical).toString('hex');
            // 新正文和图片共享 Doc 原子保护。只发送计量与冻结请求，不跨服务搬运整篇正文。
            const protection = await this.peers.http(
              '/infra/draft/protect-body',
              {
                protection: {
                  context: { ...context, request_fingerprint: digest.toString('base64') },
                  grant_id: b.grant.grant_id,
                  room_id: b.room,
                  workspace_epoch: b.epoch,
                  generation: '1',
                  resource_ids: candidate.resources,
                  update_hash: digest.toString('base64'),
                  client_session_id: b.client,
                  client_update_id: input.client_update_id,
                },
                body_bytes: bodyBytes,
                body_hash: bodyHash,
              },
              { deadlineMs: 2500 },
            );
            if (protection.state !== 'PROTECTED' && protection.state !== 'CONFIRMED')
              throw new RPCFailure(14, 'RESOURCE_PROTECTION_UNKNOWN');
            measure('protected');
            const seq = (BigInt(room.last_durable_seq) + 1n).toString();
````

## 第 7 步：RoomUpdates（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 244 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>updateLog.insertUpdate(               c,               b.room,               b.epoch,               seq,               b.client,               input.client_update_id,               …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>updateLog.advanceDurableSeq(c, seq, b.room, b.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>resourceConfirmations.queueResourceConfirmation(               c,               protectId,               JSON.stringify(confirmation),             )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>BigInt(seq) % 32n === 0n</code>。
- 在本段的内部函数内：等待 <code>this.snapshots.checkpoint(c, room, candidate.doc, seq)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return { seq, protectId, protection, context };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>!result.duplicate</code>。
- 在本段的内部函数内：等待 <code>this.outputs.broadcast(b.room, {             type: &#x27;UPDATE&#x27;,             room_id: b.room,             workspace_epoch: b.epoch,             durable_seq: String(result.seq),         …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 7 bytes=9046:11095 -->
````javascript
            await updateLog.insertUpdate(
              c,
              b.room,
              b.epoch,
              seq,
              b.client,
              input.client_update_id,
              bytes,
              digest,
            );
            await updateLog.advanceDurableSeq(c, seq, b.room, b.epoch);
            const confirmation = {
              grant_id: b.grant.grant_id,
              query: {
                context: { ...context, request_fingerprint: digest.toString('base64') },
                generation: '1',
                resource_digest: protection.resource_digest,
              },
              node_receipt_ref: b.room + ':' + b.epoch + ':' + seq,
              durable_seq: seq,
              update_hash: digest.toString('base64'),
              proven_node_outcome: 'COMMITTED',
              body_bytes: bodyBytes,
              body_hash: bodyHash,
            };
            // 确认任务与日志同事务写入。进程在SAVED前后退出，都有固定输入续接Doc资源保护。
            await resourceConfirmations.queueResourceConfirmation(
              c,
              protectId,
              JSON.stringify(confirmation),
            );
            if (BigInt(seq) % 32n === 0n)
              await this.snapshots.checkpoint(c, room, candidate.doc, seq);
            return { seq, protectId, protection, context };
          } finally {
            candidate?.doc.destroy();
            host.destroy();
          }
        });
        measure('committed');
        if (!result.duplicate) {
          // 广播发生在COMMIT之后；失败/未知提交从不安装到公共房间，不发SAVED。
          await this.outputs.broadcast(b.room, {
            type: 'UPDATE',
            room_id: b.room,
            workspace_epoch: b.epoch,
            durable_seq: String(result.seq),
            client_session_id: b.client,
            client_update_id: input.client_update_id,
            update_hash: digest.toString('hex'),
            update: input.update,
          });
        }
````

## 第 8 步：RoomUpdates（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 297 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：此处通过 <code>return {           type: &#x27;SAVED&#x27;,           room_id: b.room,           workspace_epoch: b.epoch,           client_session_id: b.client,           client_update_id: input.client_upd …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：等待 <code>this.confirmations.recover(           hash(             &#91;b.room, b.epoch, b.client, input.client_update_id, b.grant.grant_id&#93;.join(&#x27;\n&#x27;),           ).toString(&#x27;hex&#x27;),         )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return saved;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>diagnostics</code>。

<!-- write-step: 8 bytes=11095:12341 -->
````javascript
        measure('broadcast');
        return {
          type: 'SAVED',
          room_id: b.room,
          workspace_epoch: b.epoch,
          client_session_id: b.client,
          client_update_id: input.client_update_id,
          update_hash: digest.toString('hex'),
          durable_seq: String(result.seq),
        };
      })
      .then(async (saved) => {
        // 原保护确认仍沿同号尝试后返回SAVED，但不占用房间串行队列。
        // 更新日志与确认任务已同事务COMMIT，待确认pin仍保护资源；后台
        // 原本就可能并行确认。广播顺序保留，后续UPDATE/FLUSH无需等待
        // 另一服务对上一条已持久更新的独立确认往返。
        await this.confirmations.recover(
          hash(
            [b.room, b.epoch, b.client, input.client_update_id, b.grant.grant_id].join('\n'),
          ).toString('hex'),
        );
        measure('confirmed');
        return saved;
      })
      .finally(() => {
        // 容量故障定位只输出固定阶段耗时，不包含正文、更新字节、身份或凭据。
        if (diagnostics)
          console.error(JSON.stringify({ event: 'collaboration_update_timing', phases }));
      });
  }
}

````

## 第 9 步：RoomAwareness

插入位置：第 8 步代码之后，紧接追加；完成稿约第 329 行。

来源 room_awareness.mjs：原业务声明与函数体保留。

本步声明：`export class RoomAwareness `。

**这一段为什么这样写：** 限制在线光标状态的体积并服从当前撤权。

1. JSON字节数超过4096拒绝
2. 用事务重验authorized
3. 只在授权成功后broadcast


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>admissions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.admissions = admissions</code> |
| <code>outputs</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.outputs = outputs</code> |

**本段进入 RoomAwareness.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>admissions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions = admissions</code> |
| <code>outputs</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.outputs = outputs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 RoomAwareness.publish：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions.authorized(connection, binding, input)</code>；<code>binding.room</code>；<code>binding.client</code> |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.state</code>；<code>this.admissions.authorized(connection, binding, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 检查 <code>Buffer.byteLength(JSON.stringify(input.state)) &gt; 4096</code>。条件成立时执行 <code>throw new RPCFailure(3, &#x27;AWARENESS_TOO_LARGE&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(3, &#x27;AWARENESS_TOO_LARGE&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.transactions.run((connection) =&gt;       this.admissions.authorized(connection, binding, input),     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>this.outputs.broadcast(binding.room, {       type: &#x27;AWARENESS&#x27;,       client_session_id: binding.client,       state: input.state,     })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 9 bytes=12341:13118 -->
````javascript
// 来源 room_awareness.mjs：原业务声明与函数体保留。

export class RoomAwareness {
  constructor(transactions, admissions, outputs) {
    this.transactions = transactions;
    this.admissions = admissions;
    this.outputs = outputs;
  }
  async publish(binding, input) {
    if (Buffer.byteLength(JSON.stringify(input.state)) > 4096)
      throw new RPCFailure(3, 'AWARENESS_TOO_LARGE');
    // 在线状态也会暴露协作信息，发送前使用与正文相同的当前来源核验。
    await this.transactions.run((connection) =>
      this.admissions.authorized(connection, binding, input),
    );
    await this.outputs.broadcast(binding.room, {
      type: 'AWARENESS',
      client_session_id: binding.client,
      state: input.state,
    });
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/room_io.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
