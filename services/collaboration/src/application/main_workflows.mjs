import { bad } from '../domain/shared_protocol.mjs';
import * as barriers from '../repository/main_workspace.mjs';
import { canonical } from '../../../../packages/content/index.mjs';
import { captureProvenance } from '../domain/capture_protocol.mjs';
import * as captures from '../repository/main_workspace.mjs';
import { cutDigest } from '../domain/main_protocol.mjs';
import { fixedContentBinding } from '../domain/merge_protocol.mjs';
import { fromPM } from '../../../../packages/content/index.mjs';
import { hash } from '../domain/shared_protocol.mjs';
import * as installations from '../repository/main_workspace.mjs';
import { json } from '../domain/shared_protocol.mjs';
import * as lineages from '../repository/candidate_artifacts.mjs';
import { lockSources } from './admission_gates.mjs';
import { matchesMainWorkspace } from '../domain/main_workspace.mjs';
import { normalizeContent } from '../../../../packages/content/index.mjs';
import { prepareBinding } from '../domain/main_protocol.mjs';
import { preparedMainSource } from '../domain/main_workspace.mjs';
import { prosemirrorJSONToYDoc } from 'y-prosemirror';
import * as repository from '../repository/main_workspace.mjs';
import { requireDocumentBodyPresent } from './account_document_cleanup.mjs';
import { requireDraftMainOpen } from './admission_gates.mjs';
import { requireMainOpen } from './admission_gates.mjs';
import { resourceIds } from '../../../../packages/content/index.mjs';
import * as rooms from '../repository/room_access.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';
import { schema } from '../../../../packages/content/index.mjs';
import { storeContent } from '../domain/capture_protocol.mjs';
import { toPM } from '../../../../packages/content/index.mjs';
import { validID } from '../domain/shared_protocol.mjs';
import { wireContent } from '../domain/capture_protocol.mjs';
import { wireCut } from '../domain/main_protocol.mjs';
import { yDocToProsemirrorJSON } from 'y-prosemirror';

// 来源 main_captures.mjs：原业务声明与函数体保留。

export class MainCaptures {
  constructor(transactions, snapshots, peers) {
    this.transactions = transactions;
    this.snapshots = snapshots;
    this.peers = peers;
  }

  async revision(input) {
    if (
      typeof input?.document_id !== 'string' ||
      !/^[1-9][0-9]{0,19}$/.test(input.document_id) ||
      BigInt(input.document_id) > 18446744073709551615n
    )
      throw bad('MAIN_REVISION_INVALID');
    // 仅向Doc证书返回不透明摘要。它只是变化提示，不能作为正文读取或编辑准入。
    return this.transactions.run(async (connection) => {
      const [rooms] = await captures.mainReadingRevision(connection, input.document_id);
      return hash(Buffer.from(canonical(rooms))).toString('hex');
    });
  }

  async capture(input) {
    const context = input.context;
    const purpose = context?.target?.purpose;
    const usages = {
      MERGE: /^MERGE_ATTEMPT:[1-9][0-9]{0,19}$/,
      MAIN_CONTROL: /^MAIN_CONTROL:[1-9][0-9]{0,19}$/,
      READ_MAIN: /^MAIN_READ_VIEW:1$/,
      PUBLICATION: /^RELEASE_CREATE:1$/,
    };
    const usagePattern = Object.hasOwn(usages, purpose) ? usages[purpose] : undefined;
    if (
      !validID(context?.operation_id) ||
      !usagePattern?.test(input.usage) ||
      Buffer.from(context.request_fingerprint ?? []).length !== 32
    )
      throw bad('MAIN_CAPTURE_INVALID');
    const key = context.operation_id + ':' + input.usage;
    const binding = {
      ...input,
      context: {
        ...context,
        request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
      },
    };
    const prior = await this.transactions.run(async (connection) => {
      await lockSources(connection, context.sources);
      const [[stored]] = await captures.lockCapture(connection, key);
      return stored ? this.replay(json(stored.payload), binding) : null;
    });
    if (prior) {
      // 读取没有后台续期责任：原固定结果存在也须核验原短期Doc准入，不能过期后借mTLS重放正文。
      if (purpose === 'READ_MAIN') await this.peers.http('/infra/main/context', binding);
      return prior;
    }
    // 不持Node SQL锁调用Doc；固定清单只从Doc已登记原操作取得，不能靠调用者填的正文采样。
    const source = await this.peers.http('/infra/main/context', binding);
    return this.transactions.run((connection) =>
      this.captureCurrent(connection, input, source, key, binding),
    );
  }

  // 全篇预览调用方已取得自己的原准入来源；共享同一事务快照，不能另开池破坏MAIN/BRANCH切点的一致性。
  async captureCurrent(connection, input, source, key, binding) {
    const context = input.context;
    await lockSources(connection, context.sources);
    await requireMainOpen(connection, context.target.document_id);
    const [[stored]] = await captures.lockCapture(connection, key);
    if (stored) return this.replay(json(stored.payload), binding);
    if (
      source.manifest_id !== input.expected_manifest_id ||
      source.structure_id !== input.expected_structure_id ||
      source.style_id !== input.expected_style_id ||
      source.control_revision !== input.expected_control_revision ||
      !Array.isArray(source.units)
    )
      throw new RPCFailure(9, 'MAIN_SOURCE_CHANGED');
    const [rooms] = await barriers.lockMainRooms(connection, context.target.document_id);
    const byUnit = new Map();
    for (const room of rooms) {
      const [[row]] = await barriers.mainContext(connection, room.id, room.epoch);
      const current = row && json(row.payload);
      if (!matchesMainWorkspace(current, room, source) || byUnit.has(room.unit_id))
        throw new RPCFailure(14, 'MAIN_CONTEXT_UNKNOWN');
      byUnit.set(room.unit_id, room);
    }
    const cuts = rooms.map((room) => ({
      room_id: room.id,
      workspace_epoch: room.epoch,
      durable_seq: room.last_durable_seq,
    }));
    const cut = {
      document_id: context.target.document_id,
      structure_id: source.structure_id,
      style_id: source.style_id,
      control_revision: source.control_revision,
      rooms: cuts,
      dependency_digest: cutDigest(cuts),
    };
    let transfer = 0;
    const units = [];
    const seen = new Set();
    for (const unit of source.units) {
      if (seen.has(unit.unit_id)) throw new RPCFailure(14, 'MAIN_MANIFEST_INVALID');
      seen.add(unit.unit_id);
      const original = wireContent(unit.content);
      if (
        original.unit_id !== unit.unit_id ||
        original.document_id !== context.target.document_id ||
        original.structure_id !== source.structure_id ||
        !hash(original.body_json_utf8).equals(original.body_hash)
      )
        throw new RPCFailure(14, 'MAIN_MANIFEST_INVALID');
      const baseline = normalizeContent(JSON.parse(original.body_json_utf8), {
        workspaceKind: 'MAIN',
      });
      const room = byUnit.get(unit.unit_id);
      let body = baseline;
      if (room) {
        const doc = await this.snapshots.restore(connection, room, room.last_durable_seq);
        try {
          body = normalizeContent(fromPM(yDocToProsemirrorJSON(doc)), { workspaceKind: 'MAIN' });
        } finally {
          doc.destroy();
        }
      }
      const bytes = Buffer.from(canonical(body));
      if (bytes.length > 16 << 20) throw new RPCFailure(8, 'MAIN_CONTENT_TOO_LARGE');
      const provenance = JSON.parse(original.provenance_json_utf8);
      const content = {
        ...original,
        branch_id: '0',
        capture_id: key + ':' + unit.unit_id,
        operation_id: context.operation_id,
        workspace_epoch: source.control_revision,
        cut_json_utf8: room
          ? Buffer.from(canonical(cuts.find((item) => item.room_id === room.id)))
          : original.cut_json_utf8,
        body_json_utf8: bytes,
        body_hash: hash(bytes),
        byte_size: String(bytes.length),
        resource_ids: resourceIds(body),
        provenance_json_utf8: Buffer.from(
          canonical(
            captureProvenance(
              body,
              {
                body: baseline,
                lineage_id: provenance.lineage_id,
                base_manifest_id: source.manifest_id,
                base_version_id: unit.content_version_id,
                provenance,
              },
              context.operation_id,
            ),
          ),
        ),
      };
      transfer +=
        bytes.length + content.provenance_json_utf8.length + content.cut_json_utf8.length + 4096;
      // 单次全篇RPC保留32MiB传输余量；超限明确失败，不截断全篇清单或声称覆盖最新主线。
      if (transfer > 24 << 20) throw new RPCFailure(8, 'MAIN_VIEW_TRANSFER_LIMIT');
      units.push(storeContent(content));
    }
    if ([...byUnit.keys()].some((unit) => !seen.has(unit)))
      throw new RPCFailure(14, 'MAIN_MANIFEST_INVALID');
    const result = { binding, cut, units };
    const digest = hash(Buffer.from(canonical({ cut, units }))).toString('base64');
    await captures.insertCapture(connection, key, context.operation_id, { ...result, digest });
    return {
      state: 'FIXED',
      cut: wireCut(cut),
      units: units.map(wireContent),
      result_digest: Buffer.from(digest, 'base64'),
    };
  }

  replay(stored, binding) {
    if (canonical(stored.binding) !== canonical(binding))
      throw new RPCFailure(6, 'MAIN_CAPTURE_ID_CONFLICT');
    return {
      state: 'FIXED',
      cut: wireCut(stored.cut),
      units: stored.units.map(wireContent),
      result_digest: Buffer.from(stored.digest, 'base64'),
    };
  }
}

// 来源 main_commits.mjs：原业务声明与函数体保留。

export class MainCommits {
  constructor(transactions, peers, structurePreparations, outputs) {
    this.transactions = transactions;
    this.peers = peers;
    this.structurePreparations = structurePreparations;
    this.outputs = outputs;
  }

  async prepare(input) {
    const binding = prepareBinding(input),
      expected = binding.expected_cut;
    // 权威原确认在SQL锁外查询；Node不接受调用方另传候选或假称预览引用。
    const authority =
      binding.context.target.purpose === 'MAIN_CONTROL'
        ? await this.peers.http('/infra/structure/confirmation', binding)
        : { structure_preview: null };
    if (authority.structure_preview) {
      const plan = await this.transactions.run((c) =>
        this.structurePreparations.plan(c, binding, authority.structure_preview),
      );
      // 等待浏览器不占SQL事务/RPC长等待。持久任务以同一原号续接，不能换预览或采新cut。
      const state = this.outputs.prepareStructure(binding, plan);
      if (state === 'WAITING') return this.reply(binding, 'PROCESSING');
      if (state === 'FAILED') {
        this.outputs.finishStructure(binding);
        throw new RPCFailure(9, 'STRUCTURE_EDITORS_NOT_READY');
      }
    }
    let reply;
    try {
      reply = await this.transactions.run(async (connection) => {
        await lockSources(connection, binding.context.sources);
        await repository.lockDocument(connection, expected.document_id);
        await requireDraftMainOpen(connection, expected.document_id);
        const [[prior]] = await repository.lockBarrier(connection, expected.document_id);
        if (prior && !['INSTALLED', 'ABORTED'].includes(prior.state)) {
          if (prior.operation_id !== binding.context.operation_id)
            return this.reply(binding, 'PROCESSING');
          const stored = json(prior.cut_vector);
          if (
            prior.generation !== binding.barrier_generation ||
            canonical(stored.binding) !== canonical(binding)
          )
            throw new RPCFailure(6, 'MAIN_BARRIER_CONFLICT');
          return this.reply(
            binding,
            prior.state === 'PREPARED' ? 'PREPARED' : 'PROCESSING',
            stored.prepared_cut,
          );
        }
        if (prior && BigInt(prior.generation) >= BigInt(binding.barrier_generation))
          throw new RPCFailure(6, 'MAIN_BARRIER_GENERATION_CHANGED');
        const [rows] = await repository.lockMainRooms(connection, expected.document_id);
        const rooms = rows.map((room) => ({
          room_id: room.id,
          workspace_epoch: room.epoch,
          durable_seq: room.last_durable_seq,
        }));
        const cut = { ...expected, rooms, dependency_digest: cutDigest(rooms) };
        let capturedSource;
        for (const room of rows) {
          const [[context]] = await repository.mainContext(connection, room.id, room.epoch);
          if (!context) throw new RPCFailure(14, 'MAIN_CONTEXT_UNKNOWN');
          const current = json(context.payload);
          if (current.control_revision !== expected.control_revision) {
            // 元数据没有安装新房间：只允许同原操作真正捕获过的物理代次，不能放宽成任意较小epoch。
            if (capturedSource === undefined) {
              const [facts] = await captures.preparationFacts(
                connection,
                binding.context.operation_id,
              );
              capturedSource = preparedMainSource(
                binding,
                facts.map((fact) => ({
                  ...fact,
                  context: json(fact.context),
                  cut: json(fact.cut),
                })),
              );
            }
            if (!capturedSource) throw new RPCFailure(14, 'MAIN_PREPARE_CAPTURE_UNKNOWN');
            const originalRoom = expected.rooms.find((item) => item.room_id === room.id);
            if (
              !matchesMainWorkspace(current, room, {
                ...capturedSource,
                workspace_epoch: originalRoom?.workspace_epoch,
              })
            )
              return this.reply(binding, 'CHANGED', cut);
          } else if (
            current.structure_id !== expected.structure_id ||
            current.style_id !== expected.style_id ||
            current.control_revision !== room.epoch
          )
            return this.reply(binding, 'CHANGED', cut);
        }
        if (authority.structure_preview) {
          const prepared = await this.structurePreparations.prepare(
            connection,
            binding,
            authority.structure_preview,
            rows,
          );
          if (prepared.state === 'PREPARED')
            await repository.prepareBarrier(
              connection,
              expected.document_id,
              binding,
              prepared.cut,
            );
          return this.reply(binding, prepared.state, prepared.cut);
        }
        if (canonical(cut) !== canonical(expected)) return this.reply(binding, 'CHANGED', cut);
        // 全部已提交MAIN尾部已锁定，新OPEN/UPDATE在同一文档锁后看到此持久屏障；BRANCH不参加。
        await repository.prepareBarrier(connection, expected.document_id, binding, cut);
        return this.reply(binding, 'PREPARED', cut);
      });
    } catch (error) {
      if (authority.structure_preview) this.outputs.finishStructure(binding);
      throw error;
    }
    if (authority.structure_preview && reply.state !== 'PREPARED')
      this.outputs.finishStructure(binding);
    return reply;
  }

  reply(binding, state, cut) {
    return {
      state,
      operation_id: binding.context.operation_id,
      barrier_generation: binding.barrier_generation,
      ...(cut ? { prepared_cut: wireCut(cut), digest: hash(Buffer.from(canonical(cut))) } : {}),
    };
  }
}

// 来源 main_installations.mjs：原业务声明与函数体保留。

export class MainInstallations {
  constructor(
    transactions,
    snapshots,
    peers,
    outputs,
    structurePreparations,
    structureInstallations,
  ) {
    this.transactions = transactions;
    this.snapshots = snapshots;
    this.peers = peers;
    this.outputs = outputs;
    this.structurePreparations = structurePreparations;
    this.structureInstallations = structureInstallations;
  }

  async reconcile(input) {
    const context = input.context,
      supplied = input.authoritative_result;
    if (
      !context?.target ||
      !['MERGE', 'MAIN_CONTROL'].includes(context.target.purpose) ||
      !supplied ||
      !input.barrier_generation
    )
      throw bad('MAIN_RECONCILE_INVALID');
    // 即使原用户已经撤权，系统也只安装Go原号的既成事实；传入的SUCCEEDED本身没有权威性。
    const result = await this.peers.rpc('GetOperationResult', {
      operation_id: context.operation_id,
      target: context.target,
      request_fingerprint: context.request_fingerprint,
    });
    if (!['SUCCEEDED', 'REJECTED', 'CANCELLED'].includes(result.state))
      throw new RPCFailure(14, 'MAIN_RESULT_UNKNOWN');
    if (
      result.operation_id !== supplied.operation_id ||
      result.state !== supplied.state ||
      !Buffer.from(result.result_digest).equals(Buffer.from(supplied.result_digest)) ||
      !Buffer.from(result.fixed_result_json_utf8).equals(
        Buffer.from(supplied.fixed_result_json_utf8),
      )
    )
      throw new RPCFailure(6, 'MAIN_RESULT_CONFLICT');
    if (result.state !== 'SUCCEEDED') return this.abort(input);
    const authority = await this.peers.http('/infra/main/committed', {
      operation_id: context.operation_id,
      target: context.target,
      request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
    });
    const units = authority.units.map((unit) => ({ ...unit, content: wireContent(unit.content) }));
    const digest = hash(
      Buffer.from(
        canonical({
          manifest_id: authority.manifest_id,
          units: units.map((unit) => ({
            unit_id: unit.unit_id,
            content_version_id: unit.content_version_id,
            content: fixedContentBinding(unit.content),
          })),
        }),
      ),
    );
    if (
      authority.manifest_id !== input.manifest_id ||
      authority.generation !== input.barrier_generation ||
      !digest.equals(Buffer.from(input.manifest_digest)) ||
      !digest.equals(Buffer.from(authority.manifest_digest, 'base64')) ||
      canonical(units.map((unit) => ({ ...unit, content: storeContent(unit.content) }))) !==
        canonical(
          (input.units ?? []).map((unit) => ({ ...unit, content: storeContent(unit.content) })),
        )
    )
      throw new RPCFailure(6, 'MAIN_MANIFEST_CONFLICT');
    const installed = await this.transactions.run(async (connection) => {
      await barriers.lockDocument(connection, context.target.document_id);
      await requireDocumentBodyPresent(connection, context.target.document_id);
      const [[receipt]] = await installations.lockReceipt(connection, context.operation_id);
      if (receipt) {
        if (
          receipt.generation !== input.barrier_generation ||
          receipt.manifest_id !== input.manifest_id ||
          !receipt.manifest_digest.equals(digest)
        )
          throw new RPCFailure(6, 'MAIN_INSTALL_CONFLICT');
        return this.receiptReply(receipt);
      }
      const [[barrier]] = await barriers.lockBarrier(connection, context.target.document_id);
      if (
        !barrier ||
        barrier.operation_id !== context.operation_id ||
        barrier.generation !== input.barrier_generation ||
        barrier.state !== 'PREPARED'
      )
        throw new RPCFailure(14, 'MAIN_BARRIER_UNKNOWN');
      this.matchContext(barrier, context);
      await barriers.lockMainRooms(connection, context.target.document_id);
      await installations.retireMainRooms(connection, context.target.document_id);
      await installations.closeRetiredGrants(connection, context.target.document_id);
      await installations.stopRetiredDeliveries(connection, context.target.document_id);
      const cuts = [];
      for (const unit of units) {
        const content = unit.content,
          cut = JSON.parse(content.cut_json_utf8);
        const body = normalizeContent(JSON.parse(content.body_json_utf8), {
          workspaceKind: 'MAIN',
        });
        if (
          !hash(Buffer.from(canonical(body))).equals(content.body_hash) ||
          cut.workspace_epoch !== authority.control_revision ||
          cut.durable_seq !== '0' ||
          content.unit_id !== unit.unit_id ||
          content.document_id !== context.target.document_id ||
          content.structure_id !== authority.structure_id
        )
          throw new RPCFailure(6, 'MAIN_INSTALL_CONTENT_INVALID');
        const [[existing]] = await rooms.lockRoom(connection, cut.room_id);
        if (existing) throw new RPCFailure(6, 'MAIN_INSTALL_ROOM_CONFLICT');
        await rooms.insertRoom(
          connection,
          cut.room_id,
          context.target.document_id,
          unit.unit_id,
          'MAIN',
          unit.unit_id,
          cut.workspace_epoch,
        );
        const room = { id: cut.room_id, epoch: cut.workspace_epoch };
        const doc = prosemirrorJSONToYDoc(schema, toPM(body));
        try {
          await this.snapshots.checkpoint(connection, room, doc, '0');
        } finally {
          doc.destroy();
        }
        await barriers.insertMainContext(connection, cut.room_id, cut.workspace_epoch, {
          structure_id: authority.structure_id,
          style_id: authority.style_id,
          control_revision: authority.control_revision,
          manifest_id: authority.manifest_id,
        });
        cuts.push(cut);
      }
      // 管理控制只物化MAIN，不接纳任何支线；MERGE仍必须证明并追加原ACCEPTED_CUT来源。
      if (context.target.purpose === 'MERGE')
        await this.acceptLineage(connection, authority.lineage);
      else if (authority.lineage != null) throw new RPCFailure(6, 'MAIN_CONTROL_LINEAGE_INVALID');
      if (authority.structure_transition) {
        if (context.target.purpose !== 'MAIN_CONTROL')
          throw new RPCFailure(6, 'STRUCTURE_INSTALL_PURPOSE_INVALID');
        cuts.push(
          ...(await this.structureInstallations.install(
            connection,
            context,
            authority.structure_transition,
            units,
          )),
        );
      }
      await installations.insertReceipt(connection, input, cuts);
      await installations.installedBarrier(
        connection,
        context.target.document_id,
        context.operation_id,
        result.result_digest,
        input.manifest_id,
      );
      return {
        state: 'READY',
        operation_id: context.operation_id,
        barrier_generation: input.barrier_generation,
        manifest_id: input.manifest_id,
        installed_epochs: cuts,
        manifest_digest: digest,
      };
    });
    const retired = await this.transactions.run(async (connection) => {
      const [rows] = await installations.retiredMainRooms(connection, context.target.document_id);
      return [
        ...rows.map((room) => room.id),
        ...(await this.structureInstallations.retiredRooms(connection, context.operation_id)),
      ];
    });
    // 发送READY之前等待真实旧socket退出。丢ACK时只重读原receipt，不会再覆盖后来MAIN代次。
    if (authority.structure_transition) {
      const navigation = await this.transactions.run((c) =>
        this.structureInstallations.navigation(
          c,
          context.operation_id,
          authority.structure_transition,
        ),
      );
      await this.outputs.installedStructure(context.operation_id, navigation);
    }
    await this.outputs.stopRooms(retired);
    return installed;
  }

  async acceptLineage(connection, patch) {
    if (!patch?.context) throw new RPCFailure(14, 'MAIN_LINEAGE_UNKNOWN');
    const [[room]] = await rooms.lockRoom(connection, patch.room_id);
    const [[stored]] = await lineages.lockRoomLineage(connection, patch.room_id, patch.epoch);
    if (
      !room ||
      room.workspace_kind !== 'BRANCH' ||
      room.epoch !== patch.epoch ||
      room.state !== 'OPEN' ||
      !stored
    )
      throw new RPCFailure(14, 'MAIN_LINEAGE_UNKNOWN');
    const current = json(stored.payload);
    if (current.lineage_id === patch.context.lineage_id) {
      if (canonical(current) !== canonical(patch.context))
        throw new RPCFailure(6, 'MAIN_LINEAGE_CONFLICT');
      return;
    }
    if (current.lineage_id !== patch.previous_lineage_id)
      throw new RPCFailure(14, 'MAIN_LINEAGE_PENDING');
    // 同epoch只换来源基线，不修改Yjs草稿。已CUT_FIXED保存了自己的旧来源，继续保持原固定语义。
    await installations.updateLineage(connection, patch.room_id, patch.epoch, patch.context);
  }

  matchContext(barrier, context) {
    const binding = json(barrier.cut_vector).binding.context;
    if (
      canonical(binding) !==
      canonical({
        ...context,
        request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
      })
    )
      throw new RPCFailure(6, 'MAIN_BARRIER_CONFLICT');
  }

  async abort(input) {
    const result = await this.transactions.run(async (connection) => {
      await barriers.lockDocument(connection, input.context.target.document_id);
      const [[barrier]] = await barriers.lockBarrier(connection, input.context.target.document_id);
      if (barrier && barrier.operation_id === input.context.operation_id) {
        if (barrier.generation !== input.barrier_generation || barrier.state === 'INSTALLED')
          throw new RPCFailure(6, 'MAIN_BARRIER_CONFLICT');
        this.matchContext(barrier, input.context);
        await this.structurePreparations.abort(connection, json(barrier.cut_vector).binding);
        await installations.abortBarrier(
          connection,
          input.context.target.document_id,
          input.context.operation_id,
          input.barrier_generation,
        );
      }
      return {
        state: 'ABORTED',
        operation_id: input.context.operation_id,
        barrier_generation: input.barrier_generation,
      };
    });
    this.outputs.finishStructure({ context: input.context });
    return result;
  }

  receiptReply(receipt) {
    return {
      state: 'READY',
      operation_id: receipt.operation_id,
      barrier_generation: receipt.generation,
      manifest_id: receipt.manifest_id,
      installed_epochs: json(receipt.installed_epochs),
      manifest_digest: receipt.manifest_digest,
    };
  }
}
