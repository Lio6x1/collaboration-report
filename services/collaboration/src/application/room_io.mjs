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

// 来源 room_updates.mjs：原业务声明与函数体保留。

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
