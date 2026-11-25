# C02｜真实准入、UPDATE持久确认与FLUSH

## 这节课要实现什么

用户在共同编辑器输入一句话，需要知道它何时真正保存；点击送审前还要确认此前输入已落库。本课实现进入房间、保存更新和保存确认的后端流程。

## 做完后能看到什么

接通后，只有取得当前编辑资格才能进入房间；更新持久化完成后才返回SAVED。FLUSH证明此前输入已保存，但不会单独决定送审内容，审核固定点由后续捕获操作确定。

## 先想清楚怎样实现

1. 先向文档服务兑换真实准入资格。
2. 在同一数据库连接内复核房间和来源。
3. 检查独立更新候选、保护图片并保存日志，提交后确认保存。
4. 再提供保存位置证明。

## 怎样判断这一课完成

本课按章节做语法与规则检查；C07及06通过真实WSS和数据库验证保存确认、重复更新和FLUSH，不把收到网络消息当成落库。

## 开始写之前

从上一节已经写入的文件继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/collaboration/src/client/peers.mjs` | 用本服务证书访问Doc RPC与私有HTTPS，固定URI身份和2.5秒调用期限。 | [逐段带写](../../write/306-peers.md) |
| 2 | `services/collaboration/src/repository/source_gates.mjs` | 按来源scope和epoch创建、锁定及重新开放准入门闩。 | [逐段带写](../../write/322-source-gates.md) |
| 3 | `services/collaboration/src/repository/room_access.mjs` | 持久化房间、grant来源和真实输出delivery；锁内核对房间与grant仍OPEN。 | [逐段带写](../../write/319-room-access.md) |
| 4 | `services/collaboration/src/repository/room_persistence.mjs` | 按room/epoch/client/update身份查重，插入日志与durable_seq，保存FLUSH绑定和回执。 | [逐段带写](../../write/320-room-persistence.md) |
| 5 | `services/collaboration/src/repository/resource_confirmations.mjs` | 把资源确认任务与已提交更新绑定，持久记录等待、重试及成功。 | [逐段带写](../../write/318-resource-confirmations.md) |
| 6 | `services/collaboration/src/repository/persistence_outcome.mjs` | 按原更新身份读取已提交行及其唯一资源确认任务，供Doc核验未知结果。 | [逐段带写](../../write/315-persistence-outcome.md) |
| 7 | `services/collaboration/src/application/admission_gates.mjs` | 在房间锁之前按确定顺序锁来源，检查MAIN及受影响BRANCH屏障。 | [逐段带写](../../write/326-admission-gates.md) |
| 8 | `services/collaboration/src/application/room_admissions.mjs` | 兑换一次性票据、核bootstrap来源并登记grant/delivery；每帧重新授权。 | [逐段带写](../../write/338-room-admissions.md) |
| 9 | `services/collaboration/src/application/room_io.mjs` | 实现UPDATE、SYNC、FLUSH与AWARENESS；SAVED以SQL提交为前提，FLUSH以原依赖身份固定cut。 | [逐段带写](../../write/340-room-io.md) |
| 10 | `services/collaboration/src/application/persistence_outcome.mjs` | 只有日志与唯一确认任务完整吻合才返回COMMITTED，证据不足返回UNKNOWN。 | [逐段带写](../../write/335-persistence-outcome.md) |
| 11 | `services/collaboration/src/worker/resource_recovery.mjs` | 按原保护操作恢复Doc确认；更新日志已提交也必须保留待确认资源保护。 | [逐段带写](../../write/356-resource-recovery.md) |

## 2. 定义消费者需要的真实边界

先写 `peers.mjs`，调用方得到 `rpc(method,input)`、`http(path,input,{deadlineMs})`、`close()`。RPC按internal.proto读取，`longs:String` 和 `keepCase:true` 保证uint64、字段名不会变化；mTLS还要核 `spiffe://collaboration-report/document`。HTTP保护正文用2.5秒总期限，普通HTTP还有三秒socket超时，失败返回稳定 `DEPENDENCY_UNAVAILABLE`，不是直接把Doc当同进程函数。

`RoomAdmissions` 构造参数是 `store/peers/instance/transactions/queue/snapshots`。其中instance标记持久delivery属于哪个进程；queue与C01共用；snapshots负责可靠正文。`open(input,handler)` 返回 `{binding,ready}`：binding保留服务端已经核验的grant、来源、room、epoch、client、过期时间与delivery；READY给浏览器的是已恢复的Yjs字节与durable_seq。它们不能从浏览器自报的actor构造。

OPEN输入必须含 `protocol='collab-wss-v1'`、`client_session_id`、`connection_id`、`ticket`、`workspace_epoch`。先由Doc `RedeemCollaborationTicket` 兑换，再调 `/infra/collaboration/bootstrap`。bootstrap的actor/sources必须与grant完全相等；MAIN要固定main_context，BRANCH要固定lineage_context。ticket过期或epoch不等返回 `WORKSPACE_CHANGED`，来源置换返回 `GRANT_BINDING_INVALID`。

本课 `room_admissions.mjs` 已引用C04的 `draft_protocol.mjs`，`room_io.mjs` 依赖C07装配的输出器；保持真实import，C04/C07写齐前不import整套RoomEngine做运行测试，不为让半包运行而填空实现。现在可用 `node --check` 检查语法，正文规则仍可独立执行。

## 3. 先写SQL，再在同连接内准入

按表先写source_gates，再写room_access，最后admission_gates。`lockSources(c,sources)` 先规范排序并锁来源，再核epoch与OPEN；`requireMainOpen` 和 `requireBranchOpen` 处理准备中的屏障，不能只看room.state。第一次OPEN插room和seq=0的checkpoint；已有房间必须document/unit/workspace/epoch都相等且OPEN。grant和它的每个来源必须持久保存，delivery另存handler/instance/期限。

在事务提交并恢复正文后，还要向Doc `/infra/output/register-wss` 登记真实Edge输出，完成后才返回READY。登记未知不能说“没有输出”；本地记录保持可恢复，并按实际close/进程退出证据结算。`authorized(c,b,input)` 每帧核room/epoch/client绑定、期限、来源门闩、MAIN/BRANCH屏障及数据库grant/room，长连接不是永久权限。

<!-- lesson-code: services/collaboration/src/application/room_admissions.mjs :: RoomAdmissions -->
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
<!-- /lesson-code -->

## 4. 写UPDATE：候选、资源保护、日志与SAVED的次序

打开 `room_io.mjs` 的操作页，先写imports与 `RoomReading`，再写 `RoomUpdates`，最后 `RoomAwareness`。`RoomUpdates.update(b,input)` 中b只能来自准入；input的 `client_update_id` 是幂等身份，`update` 是严格base64编码，解码不超过1MiB。相同room/epoch/client/update身份、同hash返回原seq；换字节返回 `UPDATE_ID_CONFLICT`，不会新增seq。

先在room队列中授权，恢复独立host，再通过C01的prepareCandidate生成候选。以room/epoch/client/update/grant生成稳定protectId，调用Doc `/infra/draft/protect-body` 固定正文bytes/hash及资源集合。即使没有图片也要保护正文配额；Doc不返回 `PROTECTED/CONFIRMED` 则是 `RESOURCE_PROTECTION_UNKNOWN`，不能继续落日志。只发送计量和资源身份，不把整篇正文跨服务搬运。

SQL本步先写 `insertUpdate` 与 `advanceDurableSeq`：日志payload/hash、room.last_durable_seq和资源确认durable_task同事务提交。seq使用 `BigInt(last_durable_seq)+1n` 再转字符串，每32条顺便写checkpoint。然后COMMIT，广播UPDATE，最后构成SAVED。广播与SAVED失败可能只意味着回执丢失；客户端沿原update身份重试得到原seq。候选与host在finally销毁。

<!-- lesson-code: services/collaboration/src/repository/room_persistence.mjs :: insertUpdate -->
````javascript
export function insertUpdate(connection, room, epoch, seq, client, update, payload, hash) {
  return query(
    connection,
    'INSERT INTO update_log(room_id,epoch,seq,client_session_id,client_update_id,payload,payload_hash) VALUES(?,?,?,?,?,?,?)',
    [room, epoch, seq, client, update, payload, hash],
  );
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/collaboration/src/application/room_io.mjs :: RoomUpdates -->
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
<!-- /lesson-code -->

具体推演：room=30、epoch=1、client=session-A，第一次update-U正文“共同编辑”，hash=H，落seq=1。重复U/H还是seq=1；U/H2拒绝；SQL行锁被占用时没有COMMIT，浏览器不能先收到SAVED。确认任务在日志事务里，进程刚提交就退出仍可恢复Doc资源确认。`ResourceConfirmations.recover` 沿原任务 `ConfirmDraftResources`；临时错误RETRY，闭来源固定为 `PINNED_AWAITING_CLEANUP`，不会重新开放资格或把资源删掉。

`PersistenceOutcome.update(input)` 为Doc处理保护超时提供读证据：必须存在原日志和唯一确认任务且grant完整，才返回 `COMMITTED/durable_seq/update_hash/protection_operation_id/body_bytes/body_hash`。只有日志没有唯一绑定仍是UNKNOWN，不能据此释放pin。

## 5. 写FLUSH：证明点击前的输入已经保存

`RoomReading.flush(b,input)` 的 `operation_id` 绑定原点击；`dependencies` 是每条 `{client_session_id,client_update_id,update_hash}`。先校验、排序、拒绝重复，再对actor/room/epoch/依赖求fingerprint并持久绑定。相同operation换依赖返回 `FLUSH_ID_CONFLICT`。逐条查原update hash：没到返回 `WAITING/DEPENDENCIES_MISSING`，相同update身份hash不同返回 `DEPENDENCY_HASH_CONFLICT`。

<!-- lesson-code: services/collaboration/src/application/room_io.mjs :: RoomReading -->
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
````
<!-- /lesson-code -->

接上一个例子：先发送依赖delete-D的FLUSH，而D还没落库，必须WAITING；D是纯删除，state vector可能与删除前相同，但D/hash已落seq=2后FLUSH就能返回 `FLUSHED/receipt_id/covered_cut/dependency_digest`。cut固定room=30、epoch=1、durable_seq=2。再有seq=3不改变原回执；同号重试读取原FIXED回执。SYNC也走同room队列和授权，读取连续可靠日志，不返回浏览器未确认的本地内容。AWARENESS最大4096字节并执行同一当前来源检查，因为在线成员状态同样会泄露协作信息。

## 6. HTTP/WSS/worker实际调用与检查

C07 `attachWSS` 的OPEN→engine.open、UPDATE→engine.update、FLUSH→engine.flush、SYNC→engine.sync最终调用本课类。`createHTTP` 的 `/infra/persistence/update` 调本课PersistenceOutcome。`createRoomComponents` 将本课资源确认器传给updates，runtime启动每秒不重叠恢复轮；不能仅new类却忘记worker。

```powershell
# 当前只做语法检查；node --check不执行尚未写齐的import，也不验证持久性。
Set-Location -LiteralPath $lessonRoot
node --check services/collaboration/src/application/room_admissions.mjs
if ($LASTEXITCODE -ne 0) { throw '准入语法错误' }
node --check services/collaboration/src/application/room_io.mjs
if ($LASTEXITCODE -ne 0) { throw '保存语法错误' }
```

06完成独立MySQL、身份服务、Doc、Edge及WSS后，串行执行：

```powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/collaboration/wss.test.mjs tests/collaboration/resource-update.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实持久保存或资源引用失败' }
```

精确测试 `the actual edge WSS stores an update before SAVED, deduplicates bytes, and flushes pure deletion by update identity` 核原seq、重复hash冲突与删除FLUSH；`actual room row lock prevents SAVED until commit and a rejected candidate leaves durable SYNC unchanged` 核行锁期间无SAVED、非法候选没有污染持久正文；`actual image UPDATE protects only same-space READY fixtures and durable proof transfers references before a release tombstone` 核资源引用。预期全部PASS、fail/skip=0。`WORKSPACE_CHANGED` 查票据epoch与原房间；`DEPENDENCIES_MISSING` 查原更新是否先SAVED；`RESOURCE_PROTECTION_UNKNOWN` 查Doc恢复与私有mTLS，沿原update重试，不能清任务或换操作号。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/collaboration/src/client/peers.mjs`：[本课完成文件](../../source/306-peers.md)。
- `services/collaboration/src/repository/source_gates.mjs`：[本课完成文件](../../source/322-source-gates.md)。
- `services/collaboration/src/repository/room_access.mjs`：[本课完成文件](../../source/319-room-access.md)。
- `services/collaboration/src/repository/room_persistence.mjs`：[本课完成文件](../../source/320-room-persistence.md)。
- `services/collaboration/src/repository/resource_confirmations.mjs`：[本课完成文件](../../source/318-resource-confirmations.md)。
- `services/collaboration/src/repository/persistence_outcome.mjs`：[本课完成文件](../../source/315-persistence-outcome.md)。
- `services/collaboration/src/application/admission_gates.mjs`：[本课完成文件](../../source/326-admission-gates.md)。
- `services/collaboration/src/application/room_admissions.mjs`：[本课完成文件](../../source/338-room-admissions.md)。
- `services/collaboration/src/application/room_io.mjs`：[本课完成文件](../../source/340-room-io.md)。
- `services/collaboration/src/application/persistence_outcome.mjs`：[本课完成文件](../../source/335-persistence-outcome.md)。
- `services/collaboration/src/worker/resource_recovery.mjs`：[本课完成文件](../../source/356-resource-recovery.md)。

下一节：[ C03 撤权门闩、真实输出排空与进程恢复 ](03-fence-recovery.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/collaboration/src/client/peers.mjs',
    'services/collaboration/src/repository/source_gates.mjs',
    'services/collaboration/src/repository/room_access.mjs',
    'services/collaboration/src/repository/room_persistence.mjs',
    'services/collaboration/src/repository/resource_confirmations.mjs',
    'services/collaboration/src/repository/persistence_outcome.mjs',
    'services/collaboration/src/application/admission_gates.mjs',
    'services/collaboration/src/application/room_admissions.mjs',
    'services/collaboration/src/application/room_io.mjs',
    'services/collaboration/src/application/persistence_outcome.mjs',
    'services/collaboration/src/worker/resource_recovery.mjs'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
