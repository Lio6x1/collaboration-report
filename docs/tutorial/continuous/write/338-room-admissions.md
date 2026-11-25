# 分步编写 `services/collaboration/src/application/room_admissions.mjs`

先按 [C02 · 真实准入、UPDATE持久确认与FLUSH](../lessons/collaboration/02-admission-save.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/338-room-admissions.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

兑换Doc票据、建立本地房间与输出登记，并对每个消息重验当前授权。

READY会交付正文；必须等Doc授权、持久本地准入和实际Edge输出登记都成立。

写之前先按这条执行路线理解各段的连接：

1. open检查协议及客户端身份，RedeemCollaborationTicket后取得bootstrap并核对actor/sources
2. 同房间队列与事务中锁来源/文档入口/room；新房间用合法正文建立seq0检查点
3. 固定MAIN上下文或BRANCH lineage，登记grant/source/delivery并实际恢复Yjs
4. Doc register-wss成功才返回binding和READY；authorized每帧再核房间/代次/客户端、有效grant与OPEN状态

本文件需要守住的边界：

- 不能让客户端ticket决定任意room或actor
- 已有room身份、workspace、epoch不匹配必须拒绝；不得用bootstrap正文覆盖已保存room

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/application' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/application/room_admissions.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/application/room_admissions.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/application/room_admissions.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/application/room_admissions.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:764 -->
````javascript
import * as Y from 'yjs';
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import { randomUUID } from 'node:crypto';
import { schema, toPM, normalizeContent, canonical } from '../../../../packages/content/index.mjs';
import * as roomAdmissions from '../repository/room_access.mjs';
import * as deliverySessions from '../repository/room_access.mjs';
import * as captures from '../repository/candidate_artifacts.mjs';
import * as mainBarriers from '../repository/main_workspace.mjs';
import { requireMainOpen, requireBranchOpen } from './admission_gates.mjs';
import { validID, deny, bad, json } from '../domain/shared_protocol.mjs';
import { lockSources } from './admission_gates.mjs';
import { draftBody, draftLineage } from '../domain/draft_protocol.mjs';

````

## 第 2 步：RoomAdmissions

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

准入及每帧最终核验复用同一来源规则；Doc票据和实际Edge登记均成立后才返回READY。

本步声明：`export class RoomAdmissions `。

**这一段为什么这样写：** 贯穿连接建立和持续授权，避免“连接成功后永久有权限”。

1. open建立固定binding并注册具体输出
2. authorized复用来源锁序核验每一帧
3. READY仅包含已验证持久cut及其Yjs状态


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.store = store</code> |
| <code>peers</code> | <code>实例字段</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |
| <code>instance</code> | <code>实例字段</code> | 当前Node进程实例ID，用于重启恢复输出<br>在此声明中的实际使用：<code>this.instance = instance</code> |
| <code>transactions</code> | <code>实例字段</code> | 同连接SQL事务边界<br>在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>queue</code> | <code>实例字段</code> | 按room串行执行器<br>在此声明中的实际使用：<code>this.queue = queue</code> |
| <code>snapshots</code> | <code>实例字段</code> | 真实Yjs恢复和检查点用例<br>在此声明中的实际使用：<code>this.snapshots = snapshots</code> |

**本段进入 RoomAdmissions.constructor：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.store = store</code> |
| <code>peers</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.peers = peers</code> |
| <code>instance</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.instance = instance</code> |
| <code>transactions</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.transactions = transactions</code> |
| <code>queue</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.queue = queue</code> |
| <code>snapshots</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>this.snapshots = snapshots</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。 |

**本段进入 RoomAdmissions.open：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.protocol</code>；<code>input.client_session_id</code>；<code>input.connection_id</code>；<code>input.ticket</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>handler</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>deliverySessions.insertDelivery(           c,           binding.delivery,           grant.grant_id,           this.instance,           handler,           new Date(binding.expires),         )</code>；<code>this.peers.http(&#x27;/infra/output/register-wss&#x27;, {         admission_id: grant.context.document_admission_ref,         grant_id: grant.grant_id,         handler_id: handler,       })</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>{ binding, ready }</code> |

**跟着本段代码走一遍：**

- 检查 <code>input.protocol !== &#x27;collab-wss-v1&#x27; &#124;&#124;       !validID(input.client_session_id) &#124;&#124;       !validID(input.connection_id) &#124;&#124;       typeof input.ticket !== &#x27;string&#x27;</code>。条件成立时执行 <code>throw bad(&#x27;OPEN_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw bad(&#x27;OPEN_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.peers.rpc(&#x27;RedeemCollaborationTicket&#x27;, {       ticket: Buffer.from(input.ticket),       client_session_id: input.client_session_id,       connection_id: input.connection_id,     })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>grant.context.target.workspace_epoch !== input.workspace_epoch &#124;&#124;       Number(grant.expires_unix_ms) &lt;= Date.now()</code>。条件成立时执行 <code>throw deny(&#x27;WORKSPACE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw deny(&#x27;WORKSPACE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>this.peers.http(&#x27;/infra/collaboration/bootstrap&#x27;, {       admission_id: grant.context.document_admission_ref,       grant_id: grant.grant_id,     })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>boot.actor_id !== grant.context.actor_id &#124;&#124;       canonical(boot.sources) !== canonical(grant.context.sources)</code>。条件成立时执行 <code>throw deny(&#x27;GRANT_BINDING_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw deny(&#x27;GRANT_BINDING_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 2 bytes=764:2662 -->
````javascript
// 准入及每帧最终核验复用同一来源规则；Doc票据和实际Edge登记均成立后才返回READY。
export class RoomAdmissions {
  constructor(store, peers, instance, transactions, queue, snapshots) {
    this.store = store;
    this.peers = peers;
    this.instance = instance;
    this.transactions = transactions;
    this.queue = queue;
    this.snapshots = snapshots;
  }

  async open(input, handler) {
    if (
      input.protocol !== 'collab-wss-v1' ||
      !validID(input.client_session_id) ||
      !validID(input.connection_id) ||
      typeof input.ticket !== 'string'
    )
      throw bad('OPEN_INVALID');
    const grant = await this.peers.rpc('RedeemCollaborationTicket', {
      ticket: Buffer.from(input.ticket),
      client_session_id: input.client_session_id,
      connection_id: input.connection_id,
    });
    if (
      grant.context.target.workspace_epoch !== input.workspace_epoch ||
      Number(grant.expires_unix_ms) <= Date.now()
    )
      throw deny('WORKSPACE_CHANGED');
    const boot = await this.peers.http('/infra/collaboration/bootstrap', {
      admission_id: grant.context.document_admission_ref,
      grant_id: grant.grant_id,
    });
    const target = grant.context.target,
      body =
        boot.workspace_kind === 'BRANCH'
          ? draftBody(boot.body)
          : normalizeContent(boot.body, { workspaceKind: boot.workspace_kind });
    if (
      boot.actor_id !== grant.context.actor_id ||
      canonical(boot.sources) !== canonical(grant.context.sources)
    )
      throw deny('GRANT_BINDING_INVALID');
    const binding = {
      grant,
      sources: grant.context.sources,
      room: grant.room_id,
      epoch: target.workspace_epoch,
      client: grant.client_session_id,
      workspaceKind: boot.workspace_kind,
      expires: Number(grant.expires_unix_ms),
      handler,
      delivery: randomUUID(),
    };
````

## 第 3 步：RoomAdmissions（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 68 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 等待 <code>this.queue.run(binding.room, () =&gt;       this.transactions.run(async (c) =&gt; {         await lockSources(c, binding.sources);         if (boot.workspace_kind === &#x27;MAIN&#x27;) await requi …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>lockSources(c, binding.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>boot.workspace_kind === &#x27;MAIN&#x27;</code>。
- 在本段的内部函数内：等待 <code>requireMainOpen(c, target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>requireBranchOpen(c, target.document_id, binding.room, binding.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>roomAdmissions.lockRoom(c, binding.room)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>!room</code>。
- 在本段的内部函数内：等待 <code>roomAdmissions.insertRoom(             c,             binding.room,             target.document_id,             target.unit_id,             boot.workspace_kind,             boot.wo …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>this.snapshots.checkpoint(c, room, seed, &#x27;0&#x27;)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>room.document_id !== target.document_id &#124;&#124;           room.unit_id !== target.unit_id &#124;&#124;           room.workspace_kind !== boot.workspace_kind &#124;&#124;           room.workspace_id !== boo …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>throw deny(&#x27;WORKSPACE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;WORKSPACE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：检查 <code>boot.workspace_kind === &#x27;MAIN&#x27;</code>。
- 在本段的内部函数内：检查 <code>!boot.main_context &#124;&#124; boot.main_context.control_revision !== binding.epoch</code>。条件成立时执行 <code>throw deny(&#x27;MAIN_CONTEXT_REQUIRED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;MAIN_CONTEXT_REQUIRED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>mainBarriers.mainContext(c, binding.room, binding.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>context</code>。
- 在本段的内部函数内：检查 <code>canonical(json(context.payload)) !== canonical(boot.main_context)</code>。条件成立时执行 <code>throw deny(&#x27;MAIN_CONTEXT_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;MAIN_CONTEXT_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。

<!-- write-step: 3 bytes=2662:4559 -->
````javascript
    const ready = await this.queue.run(binding.room, () =>
      this.transactions.run(async (c) => {
        await lockSources(c, binding.sources);
        if (boot.workspace_kind === 'MAIN') await requireMainOpen(c, target.document_id);
        else await requireBranchOpen(c, target.document_id, binding.room, binding.epoch);
        const [found] = await roomAdmissions.lockRoom(c, binding.room);
        let room = found[0];
        if (!room) {
          await roomAdmissions.insertRoom(
            c,
            binding.room,
            target.document_id,
            target.unit_id,
            boot.workspace_kind,
            boot.workspace_id,
            binding.epoch,
          );
          room = {
            id: binding.room,
            epoch: binding.epoch,
            workspace_kind: boot.workspace_kind,
            last_durable_seq: '0',
          };
          const seed = prosemirrorJSONToYDoc(schema, toPM(body));
          try {
            await this.snapshots.checkpoint(c, room, seed, '0');
          } finally {
            seed.destroy();
          }
        } else if (
          room.document_id !== target.document_id ||
          room.unit_id !== target.unit_id ||
          room.workspace_kind !== boot.workspace_kind ||
          room.workspace_id !== boot.workspace_id ||
          room.epoch !== binding.epoch ||
          room.state !== 'OPEN'
        )
          throw deny('WORKSPACE_CHANGED');
        if (boot.workspace_kind === 'MAIN') {
          if (!boot.main_context || boot.main_context.control_revision !== binding.epoch)
            throw deny('MAIN_CONTEXT_REQUIRED');
          const [[context]] = await mainBarriers.mainContext(c, binding.room, binding.epoch);
          if (context) {
            if (canonical(json(context.payload)) !== canonical(boot.main_context))
              throw deny('MAIN_CONTEXT_CHANGED');
          } else
````

## 第 4 步：RoomAdmissions（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 114 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>mainBarriers.insertMainContext(c, binding.room, binding.epoch, boot.main_context)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>boot.workspace_kind === &#x27;BRANCH&#x27;</code>。
- 在本段的内部函数内：检查 <code>!boot.lineage_context</code>。条件成立时执行 <code>throw deny(&#x27;LINEAGE_REQUIRED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;LINEAGE_REQUIRED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>captures.lockRoomLineage(c, binding.room, binding.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>oldLineage</code>。
- 在本段的内部函数内：检查 <code>canonical(draftLineage(json(oldLineage.payload))) !== canonical(lineageContext)</code>。条件成立时执行 <code>throw deny(&#x27;LINEAGE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;LINEAGE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>captures.insertRoomLineage(c, binding.room, binding.epoch, lineageContext)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>roomAdmissions.lockGrant(c, grant.grant_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：检查 <code>existing.length</code>。
- 在本段的内部函数内：检查 <code>old.state !== &#x27;OPEN&#x27; &#124;&#124;             old.room_id !== binding.room &#124;&#124;             old.client_session_id !== binding.client &#124;&#124;             old.document_admission_ref !== grant.context.document_admission_ref</code>。条件成立时执行 <code>throw deny(&#x27;GRANT_CLOSED&#x27;);</code>。转入外层异常处理路径。
- 在本段的内部函数内：执行 <code>throw deny(&#x27;GRANT_CLOSED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 在本段的内部函数内：等待 <code>roomAdmissions.insertGrant(             c,             grant.grant_id,             grant.context.document_admission_ref,             grant.context.actor_id,             Buffer.from …</code>（长表达式，按本步完整代码继续） 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>roomAdmissions.insertGrantSource(               c,               grant.grant_id,               source.scope_key,               source.epoch,             )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：等待 <code>deliverySessions.insertDelivery(           c,           binding.delivery,           grant.grant_id,           this.instance,           handler,           new Date(binding.expires),         )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 4 bytes=4559:6624 -->
````javascript
            await mainBarriers.insertMainContext(c, binding.room, binding.epoch, boot.main_context);
        }
        if (boot.workspace_kind === 'BRANCH') {
          if (!boot.lineage_context) throw deny('LINEAGE_REQUIRED');
          // 传输只交付一次正文；本地固定来源上下文复用已核验的同一基线。
          const lineageContext = { ...boot.lineage_context, body };
          const [[oldLineage]] = await captures.lockRoomLineage(c, binding.room, binding.epoch);
          if (oldLineage) {
            if (canonical(draftLineage(json(oldLineage.payload))) !== canonical(lineageContext))
              throw deny('LINEAGE_CHANGED');
          } else await captures.insertRoomLineage(c, binding.room, binding.epoch, lineageContext);
        }
        const [existing] = await roomAdmissions.lockGrant(c, grant.grant_id);
        if (existing.length) {
          const old = existing[0];
          if (
            old.state !== 'OPEN' ||
            old.room_id !== binding.room ||
            old.client_session_id !== binding.client ||
            old.document_admission_ref !== grant.context.document_admission_ref
          )
            throw deny('GRANT_CLOSED');
        } else {
          await roomAdmissions.insertGrant(
            c,
            grant.grant_id,
            grant.context.document_admission_ref,
            grant.context.actor_id,
            Buffer.from(boot.session_ref, 'base64'),
            binding.client,
            binding.room,
            binding.epoch,
            JSON.stringify(grant.capabilities),
            new Date(binding.expires),
          );
          for (const source of binding.sources)
            await roomAdmissions.insertGrantSource(
              c,
              grant.grant_id,
              source.scope_key,
              source.epoch,
            );
        }
        await deliverySessions.insertDelivery(
          c,
          binding.delivery,
          grant.grant_id,
          this.instance,
          handler,
          new Date(binding.expires),
        );
````

## 第 5 步：RoomAdmissions（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 165 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**本段进入 RoomAdmissions.authorized：**


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>c</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>lockSources(c, b.sources)</code>；<code>requireMainOpen(c, b.grant.context.target.document_id)</code>；<code>requireBranchOpen(c, b.grant.context.target.document_id, b.room, b.epoch)</code>；<code>roomAdmissions.lockLiveGrant(c, b.grant.grant_id)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>b</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>b.room</code>；<code>b.epoch</code>；<code>b.client</code>；<code>b.stopping</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>JavaScript 参数</code> | 在此声明中的实际使用：<code>input.room_id</code>；<code>input.workspace_epoch</code>；<code>input.client_session_id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>Promise</code> | 实际返回表达式：<code>room</code> |

**跟着本段代码走一遍：**

- 在本段的内部函数内：等待 <code>this.snapshots.restore(c, room)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 在本段的内部函数内：此处通过 <code>return {             type: &#x27;READY&#x27;,             protocol: &#x27;collab-wss-v1&#x27;,             room_id: binding.room,             workspace_epoch: binding.epoch,             durable_seq: S …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 等待 <code>this.peers.http(&#x27;/infra/output/register-wss&#x27;, {         admission_id: grant.context.document_admission_ref,         grant_id: grant.grant_id,         handler_id: handler,       })</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>deliverySessions.settleDelivery(         this.store.pool,         &#x27;OPEN_REJECTED&#x27;,         binding.delivery,         this.instance,       )</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 执行 <code>throw err;</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return { binding, ready };</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>input.room_id !== b.room &#124;&#124;       input.workspace_epoch !== b.epoch &#124;&#124;       input.client_session_id !== b.client</code>。条件成立时执行 <code>throw deny(&#x27;MESSAGE_BINDING_INVALID&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw deny(&#x27;MESSAGE_BINDING_INVALID&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 检查 <code>b.stopping &#124;&#124; b.expires &lt;= Date.now()</code>。条件成立时执行 <code>throw deny(&#x27;ACCESS_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw deny(&#x27;ACCESS_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>lockSources(c, b.sources)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>b.workspaceKind === &#x27;MAIN&#x27;</code>。
- 等待 <code>requireMainOpen(c, b.grant.context.target.document_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 等待 <code>requireBranchOpen(c, b.grant.context.target.document_id, b.room, b.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。

<!-- write-step: 5 bytes=6624:8160 -->
````javascript
        const doc = await this.snapshots.restore(c, room);
        try {
          return {
            type: 'READY',
            protocol: 'collab-wss-v1',
            room_id: binding.room,
            workspace_epoch: binding.epoch,
            durable_seq: String(room.last_durable_seq),
            update: Buffer.from(Y.encodeStateAsUpdate(doc)).toString('base64'),
          };
        } finally {
          doc.destroy();
        }
      }),
    );
    // Doc先登记Edge输出再释放READY。此处未知结果保留登记，断开后按实际close或退出证据清理。
    try {
      await this.peers.http('/infra/output/register-wss', {
        admission_id: grant.context.document_admission_ref,
        grant_id: grant.grant_id,
        handler_id: handler,
      });
    } catch (err) {
      await deliverySessions.settleDelivery(
        this.store.pool,
        'OPEN_REJECTED',
        binding.delivery,
        this.instance,
      );
      throw err;
    }
    return { binding, ready };
  }

  async authorized(c, b, input) {
    if (
      input.room_id !== b.room ||
      input.workspace_epoch !== b.epoch ||
      input.client_session_id !== b.client
    )
      throw deny('MESSAGE_BINDING_INVALID');
    if (b.stopping || b.expires <= Date.now()) throw deny('ACCESS_CHANGED');
    await lockSources(c, b.sources);
    if (b.workspaceKind === 'MAIN') await requireMainOpen(c, b.grant.context.target.document_id);
    else await requireBranchOpen(c, b.grant.context.target.document_id, b.room, b.epoch);
````

## 第 6 步：RoomAdmissions（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 210 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 等待 <code>roomAdmissions.lockLiveGrant(c, b.grant.grant_id)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!grant &#124;&#124;       grant.state !== &#x27;OPEN&#x27; &#124;&#124;       !grant.alive &#124;&#124;       grant.client_session_id !== b.client &#124;&#124;       grant.room_id !== b.room &#124;&#124;       grant.workspace_epoch !== b.epoch</code>。条件成立时执行 <code>throw deny(&#x27;ACCESS_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw deny(&#x27;ACCESS_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 等待 <code>roomAdmissions.lockWorkspaceRoom(c, b.room, b.epoch)</code> 完成才继续本异步函数；reject 会进入对应的错误处理路径。
- 检查 <code>!room &#124;&#124; room.state !== &#x27;OPEN&#x27;</code>。条件成立时执行 <code>throw deny(&#x27;WORKSPACE_CHANGED&#x27;);</code>。转入外层异常处理路径。
- 执行 <code>throw deny(&#x27;WORKSPACE_CHANGED&#x27;);</code>，把失败交给外层 catch 或调用者，不能继续发送成功响应。
- 此处通过 <code>return room;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=8160:8657 -->
````javascript
    const [[grant]] = await roomAdmissions.lockLiveGrant(c, b.grant.grant_id);
    if (
      !grant ||
      grant.state !== 'OPEN' ||
      !grant.alive ||
      grant.client_session_id !== b.client ||
      grant.room_id !== b.room ||
      grant.workspace_epoch !== b.epoch
    )
      throw deny('ACCESS_CHANGED');
    const [[room]] = await roomAdmissions.lockWorkspaceRoom(c, b.room, b.epoch);
    if (!room || room.state !== 'OPEN') throw deny('WORKSPACE_CHANGED');
    return room;
  }
}
````

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/application/room_admissions.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/02-admission-save.md)。
