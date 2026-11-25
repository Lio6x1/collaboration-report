# 分步编写 `services/collaboration/src/transport/output_sessions.mjs`

先按 [C05 · 结构拆并与每个在线编辑器保存握手](../lessons/collaboration/05-structure-online.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/349-output-sessions.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

管理真实socket生命周期、每次输出授权和结构变更的全编辑器保存握手。

数据库写STOPPING或套接字CLOSING都不能证明TLS字节排完；结构变更也不能漏掉某个在线编辑器未保存内容。

写之前先按这条执行路线理解各段的连接：

1. attach登记delivery→socket/closed，close事件才解决closed Promise并结算
2. send在当前来源授权事务内发送；broadcast逐连接复核，失败连接terminate
3. prepareStructure冻结参与者，发STRUCTURE_PAUSE并等8秒；新连接不得加入可写集合
4. recordFlush记录该实际连接的真实FLUSHED，structureReady只接受匹配receipt和coordination ID
5. 所有参与者ready后阻止其继续UPDATE；失败发RESUME；安装后按块范围发新unit导航
6. stopScope/stopRooms先terminate并await真实closed，再检查持久delivery属于本实例才SETTLED

本文件需要守住的边界：

- 不得把readyState=CLOSING当结束
- 客户端自报durable_seq不是保存证明，必须同连接FLUSH用例回执
- 其他实例未结算delivery必须报未知，不能擅自清掉
- 临时握手不是持久准备；重启需重新协调

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/transport' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/transport/output_sessions.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/transport/output_sessions.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/transport/output_sessions.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/transport/output_sessions.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:270 -->
````javascript
import * as deliverySessions from '../repository/room_access.mjs';
import { WebSocket } from 'ws';
import { randomUUID } from 'node:crypto';
import { canonical } from '../../../../packages/content/index.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';

````

## 第 2 步：OutputSessions

插入位置：第 1 步代码之后，紧接追加；完成稿约第 7 行。

socket生命周期属于实际运输层；停止等待真实close，持久结算仍使用所属实例与原租约。

本步声明：`export class OutputSessions `。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.store = store</code> |
| <code>instance</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.instance = instance</code> |
| <code>transactions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>admissions</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.admissions = admissions</code> |
| <code>outputs</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.outputs = new Map()</code> |
| <code>structureChanges</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.structureChanges = new Map()</code> |

**本段进入 OutputSessions.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.store = store</code> |
| <code>instance</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.instance = instance</code> |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>admissions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.admissions = admissions</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 OutputSessions.attach：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>b</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.inStructure(s, b)</code>；<code>b.stopping</code>；<code>this.settle(b, &#x27;STRUCTURE_COORDINATING&#x27;)</code>；<code>b.delivery</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>socket</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>socket.terminate</code>；<code>socket.once</code>；<code>socket.readyState</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>undefined</code> |

**跟着本段代码走一遍：**

- 检查 <code>&#91;...this.structureChanges.values()&#93;.some(         (s) =&gt; s.state !== &#x27;FAILED&#x27; &amp;&amp; this.inStructure(s, b),       )</code>。
- 此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的内部函数内：检查 <code>finished</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。
- 在本段的内部函数内：检查 <code>participant &amp;&amp; !participant.ready</code>。

<!-- write-step: 2 bytes=270:1962 -->
````javascript
// socket生命周期属于实际运输层；停止等待真实close，持久结算仍使用所属实例与原租约。
export class OutputSessions {
  constructor(store, instance, transactions, admissions) {
    this.store = store;
    this.instance = instance;
    this.transactions = transactions;
    this.admissions = admissions;
    this.outputs = new Map();
    this.structureChanges = new Map();
  }

  attach(b, socket) {
    // 参与者集合固定后，新连接必须等待；不能在所有人确认保存后又放入一个可写编辑器。
    if (
      [...this.structureChanges.values()].some(
        (s) => s.state !== 'FAILED' && this.inStructure(s, b),
      )
    ) {
      b.stopping = true;
      socket.terminate();
      this.settle(b, 'STRUCTURE_COORDINATING').catch(() => {});
      return;
    }
    let done;
    const closed = new Promise((r) => {
      done = r;
    });
    const output = { b, socket, closed };
    this.outputs.set(b.delivery, output);
    let finished = false;
    const finish = () => {
      // attach 时已 CLOSED 与随后 close 事件可能重叠，原 delivery 只执行一次本地收尾。
      if (finished) return;
      finished = true;
      b.stopping = true;
      done();
      this.settle(b, 'SOCKET_CLOSED').catch(() => {});
      this.outputs.delete(b.delivery);
      for (const state of this.structureChanges.values()) {
        const participant = state.participants.get(b.delivery);
        if (participant && !participant.ready) this.failStructure(state);
      }
    };
    socket.once('close', finish);
    // CLOSING 仍有正在排出的 TLS 字节；只有 CLOSED 或实际 close 才能证明输出结束。
````

## 第 3 步：OutputSessions（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 52 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 OutputSessions.send：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>b</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>b.delivery</code>；<code>b.stopping</code>；<code>b.expires</code>；<code>this.admissions.authorized(c, b, {         room_id: b.room,         workspace_epoch: b.epoch,         client_session_id: b.client,       })</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>message</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify(message)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>undefined</code> |

**本段进入 OutputSessions.broadcast：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>room</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>b.room === room</code> |
| <code>message</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.send(b, message)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**本段进入 OutputSessions.settle：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>b</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>b.delivery</code> |
| <code>reason</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>deliverySessions.settleDelivery(this.store.pool, reason, b.delivery, this.instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**本段进入 OutputSessions.inStructure：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>state</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>state.document</code>；<code>state.rooms</code> |
| <code>b</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>b.grant</code>；<code>b.workspaceKind</code>；<code>b.room</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>b.grant.context.target.document_id === state.document &amp;&amp;       (b.workspaceKind === &#x27;MAIN&#x27; &#124;&#124; state.rooms.has(b.room))</code> |

**跟着本段代码走一遍：**

- 检查 <code>socket.readyState === WebSocket.CLOSED</code>。
- 此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>b.expires &lt;= Date.now()</code>。
- 检查 <code>!output &#124;&#124; b.stopping &#124;&#124; b.expires &lt;= Date.now() &#124;&#124; output.socket.readyState !== 1</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。
- 等待 <code>this.transactions.run(async (c) =&gt; {       await this.admissions.authorized(c, b, {         room_id: b.room,         workspace_epoch: b.epoch,         client_session_id: b.client,  …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>this.admissions.authorized(c, b, {         room_id: b.room,         workspace_epoch: b.epoch,         client_session_id: b.client,       })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>b.stopping</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。
- 检查 <code>b.room === room &amp;&amp; !b.stopping</code>。
- 等待 <code>this.send(b, message)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>deliverySessions.settleDelivery(this.store.pool, reason, b.delivery, this.instance)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 3 bytes=1962:3259 -->
````javascript
    if (socket.readyState === WebSocket.CLOSED) {
      finish();
      return;
    }
    const timer = setTimeout(
      () => {
        b.stopping = true;
        socket.terminate();
      },
      Math.max(1, b.expires - Date.now()),
    );
    timer.unref();
    socket.once('close', () => clearTimeout(timer));
    if (b.expires <= Date.now()) socket.terminate();
  }

  async send(b, message) {
    const output = this.outputs.get(b.delivery);
    if (!output || b.stopping || b.expires <= Date.now() || output.socket.readyState !== 1) return;
    await this.transactions.run(async (c) => {
      await this.admissions.authorized(c, b, {
        room_id: b.room,
        workspace_epoch: b.epoch,
        client_session_id: b.client,
      });
      if (b.stopping) return;
      output.socket.send(JSON.stringify(message));
    });
  }

  async broadcast(room, message) {
    for (const { b } of this.outputs.values())
      if (b.room === room && !b.stopping)
        try {
          await this.send(b, message);
        } catch {
          b.stopping = true;
          this.outputs.get(b.delivery)?.socket.terminate();
        }
  }

  async settle(b, reason) {
    await deliverySessions.settleDelivery(this.store.pool, reason, b.delivery, this.instance);
  }

  inStructure(state, b) {
````

## 第 4 步：OutputSessions（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 98 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 OutputSessions.structureControl：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>output</code> | <code>JavaScript 参数</code> | 原连接输出对象，含绑定b及socket；只发送暂停/恢复/导航控制帧，不交付正文。 |
| <code>message</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>JSON.stringify({           room_id: b.room,           workspace_epoch: b.epoch,           client_session_id: b.client,           ...message,         })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>Promise.resolve(false)</code>；<code>new Promise((done) =&gt; {       const timer = setTimeout(() =&gt; done(false), 1000);       timer.unref();       socket.send(         JSON.stringify({           room_id: b.room,         …</code>（长表达式，按本步完整代码继续） |

**本段进入 OutputSessions.prepareStructure：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.context</code>；<code>canonical(binding)</code> |
| <code>plan</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>plan.cut</code>；<code>plan.branches</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>&#x27;WAITING&#x27;</code>；<code>state.state</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return (       b.grant.context.target.document_id === state.document &amp;&amp;       (b.workspaceKind === &#x27;MAIN&#x27; &#124;&#124; state.rooms.has(b.room))     );</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>b.stopping &#124;&#124; b.expires &lt;= Date.now() &#124;&#124; socket.readyState !== WebSocket.OPEN</code>。条件成立时执行 <code>return Promise.resolve(false);</code>。当前函数在这里结束。
- 此处通过 <code>return new Promise((done) =&gt; {       const timer = setTimeout(() =&gt; done(false), 1000);       timer.unref();       socket.send(         JSON.stringify({           room_id: b.room,  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>state</code>。
- 检查 <code>state.operation !== operation</code>。条件成立时执行 <code>return &#x27;WAITING&#x27;;</code>。当前函数在这里结束。
- 检查 <code>state.binding !== canonical(binding)</code>。条件成立时执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_COORDINATION_CONFLICT&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(6, &#x27;STRUCTURE_COORDINATION_CONFLICT&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return state.state;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=3259:4890 -->
````javascript
    return (
      b.grant.context.target.document_id === state.document &&
      (b.workspaceKind === 'MAIN' || state.rooms.has(b.room))
    );
  }

  structureControl(output, message) {
    // 控制帧只含原连接身份和导航信息，不交付正文；安装后旧ACL关闭也必须能通知退出。
    const { b, socket } = output;
    if (b.stopping || b.expires <= Date.now() || socket.readyState !== WebSocket.OPEN)
      return Promise.resolve(false);
    return new Promise((done) => {
      const timer = setTimeout(() => done(false), 1000);
      timer.unref();
      socket.send(
        JSON.stringify({
          room_id: b.room,
          workspace_epoch: b.epoch,
          client_session_id: b.client,
          ...message,
        }),
        (error) => {
          clearTimeout(timer);
          done(!error);
        },
      );
    });
  }

  prepareStructure(binding, plan) {
    const operation = binding.context.operation_id,
      document = binding.context.target.document_id;
    let state = this.structureChanges.get(document);
    if (state) {
      if (state.operation !== operation) return 'WAITING';
      if (state.binding !== canonical(binding))
        throw new RPCFailure(6, 'STRUCTURE_COORDINATION_CONFLICT');
      return state.state;
    }
    state = {
      operation,
      document,
      binding: canonical(binding),
      coordination: randomUUID(),
      rooms: new Set([
        ...plan.cut.rooms.map((r) => r.room_id),
        ...plan.branches.map((r) => r.room_id),
      ]),
      participants: new Map(),
      state: 'WAITING',
      deadline: Date.now() + 8000,
    };
````

## 第 5 步：OutputSessions（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 150 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 OutputSessions.recordFlush：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>b</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>b.delivery</code> |
| <code>result</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>result?.type</code>；<code>output.lastFlush = result</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**本段进入 OutputSessions.structureReady：**

**这一段为什么这样写：** 把每个在线编辑器的确认关联到已经实际持久的保存回执。

1. 核operation/coordination及连接身份
2. ready=false立即失败恢复编辑
3. 核同delivery.lastFlush的receipt/room/epoch
4. 全部参与者ready才置READY


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>b</code> | <code>JavaScript 参数</code> | 该socket固定准入binding<br>在此声明中的实际使用：<code>b.grant</code>；<code>b.delivery</code>；<code>b.room</code>；<code>b.epoch</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>JavaScript 参数</code> | STRUCTURE_READY控制帧，含receipt_id与可选block_id<br>在此声明中的实际使用：<code>input.operation_id</code>；<code>input.coordination_id</code>；<code>input.room_id</code>；<code>input.workspace_epoch</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>undefined</code> |

**跟着本段代码走一遍：**

- 检查 <code>!output.b.stopping &amp;&amp; this.inStructure(state, output.b)</code>。
- 检查 <code>!state.participants.size</code>。
- 在本段的内部函数内：检查 <code>this.structureChanges.get(document) === state</code>。
- 此处通过 <code>return state.state;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>output &amp;&amp; result?.type === &#x27;FLUSHED&#x27;</code>。
- 检查 <code>!state &#124;&#124;       state.operation !== input.operation_id &#124;&#124;       state.coordination !== input.coordination_id &#124;&#124;       state.state === &#x27;FAILED&#x27;</code>。
- 检查 <code>output</code>。

<!-- write-step: 5 bytes=4890:6651 -->
````javascript
    this.structureChanges.set(document, state);
    for (const output of this.outputs.values()) {
      if (!output.b.stopping && this.inStructure(state, output.b))
        state.participants.set(output.b.delivery, { output, ready: false, block: null });
    }
    for (const participant of state.participants.values())
      this.structureControl(participant.output, {
        type: 'STRUCTURE_PAUSE',
        operation_id: operation,
        coordination_id: state.coordination,
        deadline_unix_ms: state.deadline,
      });
    if (!state.participants.size) state.state = 'READY';
    else {
      state.timer = setTimeout(() => this.failStructure(state), 8000);
      state.timer.unref();
    }
    // 临时协调不充当持久准备事实。进程重启后重新握手；遗失调用由有界租期释放连接侧状态。
    state.retention = setTimeout(() => {
      this.failStructure(state);
      if (this.structureChanges.get(document) === state) this.structureChanges.delete(document);
    }, 60000);
    state.retention.unref();
    return state.state;
  }

  recordFlush(b, result) {
    const output = this.outputs.get(b.delivery);
    if (output && result?.type === 'FLUSHED') output.lastFlush = result;
  }

  structureReady(b, input) {
    const state = this.structureChanges.get(b.grant.context.target.document_id);
    if (
      !state ||
      state.operation !== input.operation_id ||
      state.coordination !== input.coordination_id ||
      state.state === 'FAILED'
    ) {
      const output = this.outputs.get(b.delivery);
      if (output)
        this.structureControl(output, {
          type: 'STRUCTURE_RESUME',
          operation_id: input.operation_id,
          coordination_id: input.coordination_id,
        });
````

## 第 6 步：OutputSessions（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 196 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 OutputSessions.assertStructureWrite：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>b</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>b.grant</code>；<code>b.delivery</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。 |

**本段进入 OutputSessions.failStructure：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>state</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>state.state</code>；<code>state.timer</code>；<code>state.participants</code>；<code>state.operation</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>undefined</code> |

**本段进入 OutputSessions.finishStructure：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>binding.context</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>undefined</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!participant &#124;&#124;       input.room_id !== b.room &#124;&#124;       input.workspace_epoch !== b.epoch &#124;&#124;       input.client_session_id !== b.client</code>。条件成立时执行 <code>throw new RPCFailure(3, &#x27;STRUCTURE_COORDINATION_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(3, &#x27;STRUCTURE_COORDINATION_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>input.ready === false</code>。
- 此处通过 <code>return;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>input.ready !== true &#124;&#124;       !receipt &#124;&#124;       receipt.receipt_id !== input.receipt_id &#124;&#124;       receipt.covered_cut?.room_id !== b.room &#124;&#124;       receipt.covered_cut?.workspace_epoch !== b.epoch</code>。条件成立时执行 <code>throw new RPCFailure(9, &#x27;STRUCTURE_SAVE_UNCONFIRMED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(9, &#x27;STRUCTURE_SAVE_UNCONFIRMED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>&#91;...state.participants.values()&#93;.every((p) =&gt; p.ready)</code>。
- 检查 <code>state &amp;&amp; state.state !== &#x27;FAILED&#x27; &amp;&amp; state.participants.get(b.delivery)?.ready</code>。条件成立时执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_CHANGE_PENDING&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw new RPCFailure(14, &#x27;STRUCTURE_CHANGE_PENDING&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>state.state === &#x27;FAILED&#x27;</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=6651:8423 -->
````javascript
      return;
    }
    const participant = state.participants.get(b.delivery),
      receipt = participant?.output.lastFlush;
    if (
      !participant ||
      input.room_id !== b.room ||
      input.workspace_epoch !== b.epoch ||
      input.client_session_id !== b.client
    )
      throw new RPCFailure(3, 'STRUCTURE_COORDINATION_INVALID');
    if (input.ready === false) {
      this.failStructure(state);
      return;
    }
    // 只认同一实际连接经过原FLUSH用例取得的回执，不能用客户端自报的durable_seq代替落库。
    if (
      input.ready !== true ||
      !receipt ||
      receipt.receipt_id !== input.receipt_id ||
      receipt.covered_cut?.room_id !== b.room ||
      receipt.covered_cut?.workspace_epoch !== b.epoch
    )
      throw new RPCFailure(9, 'STRUCTURE_SAVE_UNCONFIRMED');
    participant.ready = true;
    participant.block =
      typeof input.block_id === 'string' && input.block_id.length <= 256 ? input.block_id : null;
    if ([...state.participants.values()].every((p) => p.ready)) {
      state.state = 'READY';
      clearTimeout(state.timer);
    }
  }

  assertStructureWrite(b) {
    const state = this.structureChanges.get(b.grant.context.target.document_id);
    if (state && state.state !== 'FAILED' && state.participants.get(b.delivery)?.ready)
      throw new RPCFailure(14, 'STRUCTURE_CHANGE_PENDING');
  }

  failStructure(state) {
    if (state.state === 'FAILED') return;
    state.state = 'FAILED';
    clearTimeout(state.timer);
    for (const { output } of state.participants.values())
      this.structureControl(output, {
        type: 'STRUCTURE_RESUME',
        operation_id: state.operation,
        coordination_id: state.coordination,
      });
  }

  finishStructure(binding) {
````

## 第 7 步：OutputSessions（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 248 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 OutputSessions.installedStructure：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>s.operation === operation</code>；<code>this.structureControl(output, {           type: &#x27;STRUCTURE_INSTALLED&#x27;,           operation_id: operation,           coordination_id: state.coordination,           unit_id: target?.unit_id ?? null,         })</code> |
| <code>ranges</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>ranges.filter</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>undefined</code> |

**本段进入 OutputSessions.stopScope：**

**这一段为什么这样写：** 使撤权ACK真正代表旧输出已停。

1. 定位scope且epoch不高于目标的全部socket
2. 标记stopping并terminate
3. 等待真实close
4. 核持久输出归属后结算


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>scope</code> | <code>JavaScript 参数</code> | 已持久BLOCKED的scope_key/epoch<br>在此声明中的实际使用：<code>scope.scope_key</code>；<code>scope.epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 检查 <code>!state &#124;&#124; state.operation !== binding.context.operation_id</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。
- 检查 <code>!state</code>。条件成立时执行 <code>return;</code>。当前函数在这里结束。
- 等待 <code>Promise.all(       &#91;...state.participants.values()&#93;.map(({ output, block }) =&gt; {         const candidates = ranges.filter(           (r) =&gt; r.source_unit_id === output.b.grant.cont …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return this.structureControl(output, {           type: &#x27;STRUCTURE_INSTALLED&#x27;,           operation_id: operation,           coordination_id: state.coordination,           unit_id: target?.unit_id ?? null,         });</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>output.b.sources.some(           (s) =&gt; s.scope_key === scope.scope_key &amp;&amp; BigInt(s.epoch) &lt;= BigInt(scope.epoch),         )</code>。
- 等待 <code>Promise.all(pending)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 7 bytes=8423:10134 -->
````javascript
    const state = this.structureChanges.get(binding.context.target.document_id);
    if (!state || state.operation !== binding.context.operation_id) return;
    this.failStructure(state);
    clearTimeout(state.retention);
    this.structureChanges.delete(state.document);
  }

  async installedStructure(operation, ranges) {
    const state = [...this.structureChanges.values()].find((s) => s.operation === operation);
    if (!state) return;
    clearTimeout(state.timer);
    clearTimeout(state.retention);
    await Promise.all(
      [...state.participants.values()].map(({ output, block }) => {
        const candidates = ranges.filter(
          (r) => r.source_unit_id === output.b.grant.context.target.unit_id,
        );
        const key = output.b.workspaceKind === 'MAIN' ? 'main_block_ids' : 'draft_block_ids';
        const matches = candidates.filter((r) => r[key].includes(block));
        const target =
          matches.length === 1 ? matches[0] : candidates.length === 1 ? candidates[0] : null;
        return this.structureControl(output, {
          type: 'STRUCTURE_INSTALLED',
          operation_id: operation,
          coordination_id: state.coordination,
          unit_id: target?.unit_id ?? null,
        });
      }),
    );
    this.structureChanges.delete(state.document);
  }

  async stopScope(scope) {
    const pending = [];
    for (const output of this.outputs.values())
      if (
        output.b.sources.some(
          (s) => s.scope_key === scope.scope_key && BigInt(s.epoch) <= BigInt(scope.epoch),
        )
      ) {
        output.b.stopping = true;
        output.socket.terminate();
        pending.push(output.closed);
      }
    await Promise.all(pending);
````

## 第 8 步：OutputSessions（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 293 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 OutputSessions.stopRooms：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>rooms</code> | <code>JavaScript 参数</code> | 待停止的房间编号集合；终止匹配连接并等待closed，还需核验持久输出会话已停止。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。 |

**跟着本段代码走一遍：**

- 等待 <code>deliverySessions.pendingScopeDeliveries(       this.store.pool,       scope.scope_key,       scope.epoch,     )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>s.instance_id !== this.instance</code>。条件成立时执行 <code>throw Error(&#x27;OUTPUT_INSTANCE_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;OUTPUT_INSTANCE_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>this.outputs.has(s.delivery_id)</code>。条件成立时执行 <code>throw Error(&#x27;OUTPUT_STOP_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;OUTPUT_STOP_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>deliverySessions.settleClosedDelivery(this.store.pool, s.delivery_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!selected.has(output.b.room)</code>。条件成立时执行 <code>continue;</code>。跳过当前循环本轮剩余语句。
- 等待 <code>Promise.all(pending)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>deliverySessions.pendingRoomDeliveries(this.store.pool, room)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>session.instance_id !== this.instance &#124;&#124; this.outputs.has(session.delivery_id)</code>。条件成立时执行 <code>throw Error(&#x27;MAIN_OUTPUT_STOP_UNKNOWN&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw Error(&#x27;MAIN_OUTPUT_STOP_UNKNOWN&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>deliverySessions.settleClosedDelivery(this.store.pool, session.delivery_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 8 bytes=10134:11306 -->
````javascript
    const [sessions] = await deliverySessions.pendingScopeDeliveries(
      this.store.pool,
      scope.scope_key,
      scope.epoch,
    );
    for (const s of sessions) {
      if (s.instance_id !== this.instance) throw Error('OUTPUT_INSTANCE_UNKNOWN');
      if (this.outputs.has(s.delivery_id)) throw Error('OUTPUT_STOP_UNKNOWN');
      await deliverySessions.settleClosedDelivery(this.store.pool, s.delivery_id);
    }
  }

  async stopRooms(rooms) {
    const selected = new Set(rooms),
      pending = [];
    for (const output of this.outputs.values()) {
      if (!selected.has(output.b.room)) continue;
      output.b.stopping = true;
      output.socket.terminate();
      pending.push(output.closed);
    }
    await Promise.all(pending);
    for (const room of rooms) {
      const [sessions] = await deliverySessions.pendingRoomDeliveries(this.store.pool, room);
      for (const session of sessions) {
        if (session.instance_id !== this.instance || this.outputs.has(session.delivery_id))
          throw Error('MAIN_OUTPUT_STOP_UNKNOWN');
        await deliverySessions.settleClosedDelivery(this.store.pool, session.delivery_id);
      }
    }
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/transport/output_sessions.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/05-structure-online.md)。
