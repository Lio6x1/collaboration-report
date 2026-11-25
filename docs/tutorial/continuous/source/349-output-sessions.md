# services/collaboration/src/transport/output_sessions.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/349-output-sessions.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：接入HTTP/RPC/WSS；只在业务确认后返回相应成功事实。

主要声明：`OutputSessions`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import * as deliverySessions from '../repository/room_access.mjs';
import { WebSocket } from 'ws';
import { randomUUID } from 'node:crypto';
import { canonical } from '../../../../packages/content/index.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';

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

UTF-8/LF全文SHA-256：`f5af1d05c274f92c6eb1e4c593ee82244c0c7b20dc3286a79fdb15c51ca44029`。[参考实现](../../../../services/collaboration/src/transport/output_sessions.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
