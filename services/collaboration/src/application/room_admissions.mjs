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
