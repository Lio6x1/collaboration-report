import * as Y from 'yjs';
import type { EditSession, PullMain, MainView } from '../../../../contracts/http/generated';
import { readFixedContent } from '../draft/api';
import { api, transportGeneration, ApiError, UserFacingError, explain } from '../../lib/http';
import {
  readQueue,
  changeQueue,
  acknowledge,
  queueKey,
  retainedQueues,
  type Partition,
  type DraftQueue,
  type PendingUpdate,
} from './queue';

import { REMOTE, encode, decode, digest } from './domain/bytes';
import { reconstructStaleBody } from './domain/recovery_body';
import { acquireDraftTab } from './local/tab_lease';
import { DraftSocket } from './transport/draft_socket';
import type { DraftMessage as Message, SubmissionBoundary } from './domain/model';
export { REMOTE, encode, decode } from './domain/bytes';

export class DraftController {
  doc = new Y.Doc();
  partition!: Partition;
  queue: DraftQueue | null = null;
  stale: DraftQueue | null = null;
  retained: DraftQueue[] = [];
  connected = false;
  loading = true;
  stopped = false;
  composing = false;
  notice = '正在核验编辑资格';
  problem = '';
  receipt: Message | null = null;
  structurePause: Message | null = null;
  private structureTimer: ReturnType<typeof setTimeout> | undefined;
  private flushPromise: Promise<Message> | null = null;
  private structureBlock: string | null = null;
  private socket: DraftSocket | null = null;
  private session: EditSession | null = null;
  private reconnect: ReturnType<typeof setTimeout> | undefined;
  private local: Uint8Array[] = [];
  private persisting = 0;
  private inflight = new Set<string>();
  private pending = Promise.resolve();
  private releaseTab: (() => void) | undefined;
  private ticketInput: Record<string, string> | undefined;
  private atStart = transportGeneration();
  private flushInput: Message | null = null;
  private flushSent = false;
  private flushDone: ((v: Message) => void) | undefined;
  private flushReject: ((e: Error) => void) | undefined;
  constructor(
    readonly owner: Omit<Partition, 'tab'>,
    readonly structure: string,
    readonly changed: () => void,
    readonly initialized: () => void,
    readonly saved: () => void = () => {},
    readonly prepareStructure: () => Promise<string | null> = async () => null,
    readonly structureInstalled: (unit: string | null) => void = () => {},
  ) {}
  async start() {
    const lease = await acquireDraftTab(this.owner.actor);
    this.releaseTab = lease.release;
    this.partition = { ...this.owner, tab: lease.tab };
    this.queue = await readQueue(this.partition);
    if (this.queue?.retained) this.queue = null;
    await this.connect();
  }
  private active() {
    return !this.stopped && this.atStart === transportGeneration();
  }
  resourceBranchID(): string | null {
    // 仅暴露当前已安装草稿身份，不向图片视图泄露编辑ticket或借用旧代次。
    if (!this.active() || this.loading || this.stale || this.owner.kind !== 'BRANCH') return null;
    return this.session?.branch_id ?? null;
  }
  discussionBoundary() {
    const session = this.session;
    // 讨论只引用已安装READY的固定基线；离线、旧epoch与本机恢复输入不能冒充固定正文。
    if (
      !this.active() ||
      !this.connected ||
      this.loading ||
      this.stale ||
      this.owner.kind !== 'BRANCH' ||
      !session?.branch_id ||
      !session.base_version_id ||
      !session.base_manifest_id ||
      this.queue?.room !== session.room_id ||
      this.queue?.epoch !== session.workspace_epoch
    )
      return null;
    return {
      branch: session.branch_id,
      version: session.base_version_id,
      manifest: session.base_manifest_id,
      lineage: session.lineage_id ?? '',
    };
  }
  private status() {
    if (this.stale) this.notice = '工作区已变化，旧输入保留在本机';
    else if (this.structurePause) this.notice = '正在调整章节结构：当前输入保存后将自动恢复编辑';
    else if (this.composing) this.notice = '中文输入进行中';
    else if (this.local.length || this.persisting) this.notice = '正在保留本机输入，服务端尚未确认';
    else if (this.queue?.entries.length)
      this.notice = `本机已保留 ${this.queue.entries.length} 条输入，等待服务端保存`;
    else this.notice = this.connected ? '服务端已保存' : '连接中断，本机输入保留';
    this.changed();
  }
  async connect() {
    if (!this.active()) return;
    this.problem = '';
    this.connected = false;
    this.status();
    const client = this.queue?.client ?? crypto.randomUUID();
    this.ticketInput ??= {
      operation_id: crypto.randomUUID(),
      workspace_kind: this.owner.kind,
      client_session_id: client,
      expected_structure_id: this.structure,
    };
    try {
      const session = await api<EditSession>(
        `/v1/documents/${this.owner.document}/units/${this.owner.unit}/edit-sessions`,
        'POST',
        this.ticketInput,
      );
      if (!this.active()) return;
      this.ticketInput = undefined;
      const socket = new DraftSocket(session, client, {
        message: (raw, connection) => {
          this.pending = this.pending
            .then(() =>
              this.socket === connection
                ? this.receive(JSON.parse(raw), session, client)
                : undefined,
            )
            .catch((e) => {
              this.problem = explain(
                e,
                '协作连接暂时异常，请重新打开编辑器；尚未保存的输入会继续保留',
              );
              this.status();
            });
        },
        closed: (connection) => {
          if (this.socket !== connection) return;
          this.connected = false;
          this.inflight.clear();
          this.flushSent = false;
          const structureChange = this.structurePause;
          this.structurePause = null;
          clearTimeout(this.structureTimer);
          this.status();
          // 目录提交会推进ACL，来源关闭有可能先于安装通知结束旧连接；重读真实目录作导航兜底。
          if (structureChange) void this.recoverStructureNavigation();
          if (this.active()) this.reconnect = setTimeout(() => void this.connect(), 1500);
        },
      });
      this.socket = socket;
    } catch (e) {
      this.problem = explain(e, '暂时无法连接编辑器，请稍后重试');
      this.status();
      if (e instanceof ApiError && [401, 403, 409].includes(e.status)) {
        if (e.code === 'TICKET_EXPIRED') {
          this.ticketInput = undefined;
          this.reconnect = setTimeout(() => void this.connect(), 1500);
        }
        return;
      }
      if (this.active()) this.reconnect = setTimeout(() => void this.connect(), 1500);
    }
  }
  private async receive(m: Message, session: EditSession, client: string) {
    if (!this.active()) return;
    if (['STRUCTURE_PAUSE', 'STRUCTURE_RESUME', 'STRUCTURE_INSTALLED'].includes(m.type)) {
      if (
        m.room_id !== session.room_id ||
        m.workspace_epoch !== session.workspace_epoch ||
        m.client_session_id !== client
      )
        return;
      if (m.type === 'STRUCTURE_PAUSE') {
        if (this.structurePause?.coordination_id === m.coordination_id) return;
        this.structurePause = m;
        this.status();
        clearTimeout(this.structureTimer);
        this.structureTimer = setTimeout(() => {
          if (this.structurePause?.coordination_id !== m.coordination_id) return;
          this.problem = '章节调整尚未完成，正在重新核对连接；本机输入会保留';
          this.socket?.close();
        }, 20000);
        // receive本身位于pending顺序链，不能在链内等待FLUSH，否则SAVED永远无法被处理。
        void this.pauseForStructure(m);
        return;
      }
      if (this.structurePause?.coordination_id !== m.coordination_id) return;
      clearTimeout(this.structureTimer);
      if (m.type === 'STRUCTURE_RESUME') {
        this.structurePause = null;
        this.status();
      } else if (
        this.local.length ||
        this.persisting ||
        this.queue?.entries.length ||
        this.composing
      ) {
        // 未确认输入绝不跨房间重放，也不能为了自动导航丢弃；原页面保留可复制的本机内容。
        this.stale = this.queue;
        this.problem = '章节已调整，本机仍有待恢复输入。请保留或复制当前内容，再返回阅读页';
        this.status();
      } else {
        this.structureInstalled(typeof m.unit_id === 'string' ? m.unit_id : null);
      }
      return;
    }
    if (m.type === 'READY') {
      if (m.room_id !== session.room_id || m.workspace_epoch !== session.workspace_epoch)
        throw new UserFacingError('编辑状态已变化，请重新打开编辑器；本机输入会继续保留');
      if (
        !this.loading &&
        this.queue &&
        (this.queue.room !== m.room_id || this.queue.epoch !== m.workspace_epoch)
      ) {
        // 新epoch不能复用旧Y.Doc，旧待发snapshot另存展示；当前正文只从新READY安装。
        this.stale = this.queue.entries.length ? this.queue : null;
        this.queue = null;
        this.doc.off('update', this.onUpdate);
        this.doc.destroy();
        this.doc = new Y.Doc();
        this.loading = true;
        this.receipt = null;
      }
      if (this.loading) {
        Y.applyUpdate(this.doc, decode(m.update), REMOTE);
        if (this.queue) {
          if (this.queue.room !== m.room_id || this.queue.epoch !== m.workspace_epoch) {
            this.stale = this.queue.entries.length ? this.queue : null;
            this.queue = null;
          } else if (this.queue.entries.length)
            Y.applyUpdate(this.doc, decode(this.queue.snapshot), REMOTE);
        }
        if (!this.queue) {
          this.queue = {
            ...this.partition,
            key: queueKey(this.partition),
            client,
            room: m.room_id,
            epoch: m.workspace_epoch,
            snapshot: encode(Y.encodeStateAsUpdate(this.doc)),
            entries: [],
            updatedAt: Date.now(),
          };
          if (!this.stale) await changeQueue(this.partition, () => this.queue);
        }
        this.loading = false;
        this.doc.on('update', this.onUpdate);
        this.initialized();
      } else if (
        this.queue &&
        this.queue.room === m.room_id &&
        this.queue.epoch === m.workspace_epoch
      )
        Y.applyUpdate(this.doc, decode(m.update), REMOTE);
      else {
        this.stale = this.queue;
        this.connected = false;
        this.socket?.close();
        this.status();
        return;
      }
      this.connected = true;
      this.session = session;
      this.inflight.clear();
      this.flushSent = false;
      this.problem = '';
      this.status();
      this.sendPending();
      await this.findRetained();
      return;
    }
    if (m.type === 'UPDATE' || m.type === 'SYNC') {
      if (this.queue && m.room_id === this.queue.room && m.workspace_epoch === this.queue.epoch)
        Y.applyUpdate(this.doc, decode(m.update), REMOTE);
      return;
    }
    if (m.type === 'SAVED') {
      const q = await acknowledge(this.partition, {
        room: m.room_id,
        epoch: m.workspace_epoch,
        client_session_id: m.client_session_id,
        client_update_id: m.client_update_id,
        update_hash: m.update_hash,
      });
      if (q) this.queue = q;
      this.inflight.delete(m.client_update_id);
      this.status();
      this.sendPending();
      // 保存协议和队列核验完成后才通知视图；这只刷新展示，不产生新的正文写入。
      if (q && this.active() && !this.stale && !this.loading) this.saved();
      return;
    }
    if (m.type === 'WAITING') {
      this.flushSent = false;
      this.sendPending();
      return;
    }
    if (m.type === 'FLUSHED') {
      this.receipt = m;
      this.flushInput = null;
      this.flushSent = false;
      this.flushDone?.(m);
      this.flushDone = undefined;
      this.flushReject = undefined;
      this.changed();
      return;
    }
    if (m.type === 'ERROR') {
      // 协作协议仍保留原错误码供重连判断；界面只展示经过统一转换的说明。
      const failure = new ApiError(m.code, 0);
      this.problem = explain(failure, '当前输入尚未保存，请保留页面并稍后重试');
      this.flushReject?.(failure);
      this.flushDone = undefined;
      this.flushReject = undefined;
      // SQL/RPC超时不是业务拒绝，窗口中的原update身份仍在IndexedDB；新准入后原样重试。
      if (
        ['ACCESS_CHANGED', 'WORKSPACE_CHANGED', 'SOURCE_CLOSED'].includes(m.code) ||
        /_(UNAVAILABLE|UNKNOWN)$/.test(m.code)
      ) {
        this.connected = false;
        this.socket?.close();
      }
      this.status();
    }
  }
  private async pauseForStructure(message: Message) {
    try {
      const block = await this.prepareStructure();
      this.structureBlock = block;
      const receipt = await this.flush();
      if (this.structurePause?.coordination_id !== message.coordination_id) return;
      this.send({
        type: 'STRUCTURE_READY',
        operation_id: message.operation_id,
        coordination_id: message.coordination_id,
        ready: true,
        receipt_id: receipt.receipt_id,
        block_id: block,
      });
    } catch {
      if (this.structurePause?.coordination_id === message.coordination_id)
        this.send({
          type: 'STRUCTURE_READY',
          operation_id: message.operation_id,
          coordination_id: message.coordination_id,
          ready: false,
        });
    }
  }
  private async recoverStructureNavigation() {
    for (let attempt = 0; attempt < 8 && this.active(); attempt++) {
      if (this.local.length || this.persisting || this.queue?.entries.length || this.composing)
        return;
      try {
        const current = await api<MainView>('/v1/documents/' + this.owner.document + '/main');
        if (!this.active()) return;
        if (current.structure.id !== this.structure) {
          const matches: string[] = [];
          if (this.structureBlock)
            for (const unit of current.units) {
              const content = await readFixedContent(unit.content_version_id);
              const contains = (node: any): boolean =>
                node?.attrs?.id === this.structureBlock || (node?.content ?? []).some(contains);
              if (contains(content.body_json)) matches.push(unit.unit_id);
            }
          if (
            !this.active() ||
            this.local.length ||
            this.persisting ||
            this.queue?.entries.length ||
            this.composing
          )
            return;
          // 草稿独有块可能不在MAIN中；无法唯一定位就回阅读目录，绝不猜小节或搬旧Yjs更新。
          this.structureInstalled(matches.length === 1 ? matches[0] : null);
          return;
        }
      } catch (error) {
        if (error instanceof ApiError && [401, 403].includes(error.status)) return;
      }
      await new Promise((done) => setTimeout(done, 1000));
    }
  }
  private onUpdate = (update: Uint8Array, origin: unknown) => {
    if (origin === REMOTE || !this.active()) return;
    this.receipt = null;
    this.local.push(update);
    this.status();
    queueMicrotask(() => {
      if (!this.composing) void this.persistLocal();
    });
  };
  async persistLocal() {
    if (!this.local.length || this.stale || !this.queue) return;
    const bytes = Y.mergeUpdates(this.local.splice(0)),
      snapshot = encode(Y.encodeStateAsUpdate(this.doc));
    this.persisting++;
    this.status();
    this.pending = this.pending
      .then(async () => {
        const q = this.queue;
        if (!q) return;
        const entry: PendingUpdate = {
          client_session_id: q.client,
          client_update_id: crypto.randomUUID(),
          update_hash: await digest(bytes),
          update: encode(bytes),
        };
        const saved = await changeQueue(this.partition, (old) => {
          if (!old || old.room !== q.room || old.epoch !== q.epoch)
            throw new UserFacingError('编辑状态已变化，本机输入已保留，请重新打开编辑器后恢复');
          old.entries.push(entry);
          old.snapshot = snapshot;
          old.updatedAt = Date.now();
          return old;
        });
        this.queue = saved;
        this.send({ type: 'UPDATE', ...entry });
      })
      .catch((e) => {
        this.local.unshift(bytes);
        this.problem = explain(e, '浏览器暂时无法保存本机草稿，请保留页面并复制当前内容备份');
        this.connected = false;
      })
      .finally(() => {
        this.persisting--;
        this.status();
      });
    await this.pending;
  }
  setComposing(value: boolean) {
    this.composing = value;
    this.status();
    if (!value) setTimeout(() => void this.persistLocal(), 0);
  }
  private send(input: Message) {
    if (
      this.active() &&
      this.connected &&
      !this.stale &&
      this.queue &&
      this.socket?.open &&
      navigator.onLine
    ) {
      // 服务端同连接队列最多32帧，客户端只保留16条未确认UPDATE；ACK落本机后才推进下一条。
      // 断线只清发送窗口，绝不清持久队列；重连仍使用原update ID/hash重试。
      if (input.type === 'UPDATE') {
        if (this.inflight.has(input.client_update_id) || this.inflight.size >= 16) return false;
        this.inflight.add(input.client_update_id);
      }
      this.socket.send({
        room_id: this.queue.room,
        workspace_epoch: this.queue.epoch,
        client_session_id: this.queue.client,
        ...input,
      });
      return true;
    }
    return false;
  }
  private sendPending() {
    for (const entry of this.queue?.entries ?? []) this.send({ type: 'UPDATE', ...entry });
    // 依赖身份在点击时固定；先等待SAVED全部落本机，再发送一次FLUSH。
    // 每条ACK都重复FLUSH会占满同连接队列，不能把反复过载重连当成正常保存路径。
    if (this.flushInput && !this.queue?.entries.length && !this.flushSent)
      this.flushSent = this.send(this.flushInput);
  }
  async flush(): Promise<Message> {
    await this.persistLocal();
    await this.pending;
    if (this.stale || !this.connected || this.composing || this.problem)
      throw new UserFacingError('当前修改尚未保存，请等待连接恢复并完成输入法选词后重试');
    if (this.flushPromise) return this.flushPromise;
    this.flushInput ??= {
      type: 'FLUSH',
      operation_id: crypto.randomUUID(),
      dependencies: (this.queue?.entries ?? []).map(
        ({ client_session_id, client_update_id, update_hash }) => ({
          client_session_id,
          client_update_id,
          update_hash,
        }),
      ),
    };
    this.flushPromise = new Promise<Message>((done, reject) => {
      this.flushDone = done;
      this.flushReject = reject;
      this.sendPending();
    }).finally(() => {
      this.flushPromise = null;
    });
    return this.flushPromise;
  }
  submissionBoundary(receipt: Message): SubmissionBoundary {
    const session = this.session;
    if (
      this.owner.kind !== 'BRANCH' ||
      !session?.branch_id ||
      !this.queue ||
      receipt.type !== 'FLUSHED' ||
      receipt.covered_cut?.room_id !== session.room_id ||
      receipt.covered_cut?.workspace_epoch !== session.workspace_epoch ||
      this.queue.room !== session.room_id ||
      this.queue.epoch !== session.workspace_epoch ||
      typeof receipt.receipt_id !== 'string'
    )
      throw new UserFacingError('共同支线的编辑状态已变化，请重新打开后确认保存');
    return {
      branch_id: session.branch_id,
      expected_structure_id: this.structure,
      flush_receipt_id: receipt.receipt_id,
    };
  }

  pullBoundary(receipt: Message): Omit<PullMain, 'operation_id'> {
    // 只交付当前可靠会话的最小业务期待值；票据、原会话对象及Yjs正文不离开控制器。
    // 复用真实FLUSH/queue/room/epoch核验，避免PULL按钮从可变目录猜支线身份。
    const boundary = this.submissionBoundary(receipt);
    if (
      !this.active() ||
      !this.connected ||
      this.loading ||
      this.stale ||
      !this.session?.lineage_id
    )
      throw new UserFacingError('当前共同草稿已变化，请重新打开后拉取主线');
    return {
      expected_branch_id: boundary.branch_id,
      expected_lineage_id: this.session.lineage_id,
      expected_structure_id: boundary.expected_structure_id,
    };
  }
  staleBody() {
    return reconstructStaleBody(this.stale, this.owner.kind);
  }
  async findRetained() {
    const all = await retainedQueues(this.owner),
      locks = await navigator.locks.query();
    this.retained = all.filter(
      (q) =>
        q.key !== this.queue?.key &&
        q.key !== this.stale?.key &&
        !locks.held?.some(
          (lock) => lock.name === 'collaboration-report.tab:' + this.owner.actor + ':' + q.tab,
        ),
    );
    this.changed();
  }
  async selectRetained(key: string) {
    if (this.stale) throw new UserFacingError('请先处理已查看的本机输入');
    const old = this.retained.find((q) => q.key === key);
    if (!old) throw new UserFacingError('本机输入已变化，请重新读取');
    await navigator.locks.request(
      'collaboration-report.tab:' + this.owner.actor + ':' + old.tab,
      { ifAvailable: true },
      async (lock) => {
        if (!lock) throw new UserFacingError('原标签正在使用这份输入，请先在原标签处理');
        const fresh = await readQueue({ ...this.partition, tab: old.tab });
        if (!fresh || fresh.retained || !fresh.entries.length)
          throw new UserFacingError('本机输入已变化，请重新读取');
        this.stale = fresh;
        this.status();
      },
    );
  }
  async beginRecovery() {
    if (!this.stale || !this.queue || !this.connected)
      throw new UserFacingError('请先重新打开当前编辑器，再恢复本机输入');
    const old = this.stale;
    const retain = async () => {
      if (!old.archive) {
        const archive = {
            ...old,
            archive: true,
            retained: false,
            tab: old.tab + ':retained:' + crypto.randomUUID(),
          },
          p = { ...this.partition, tab: archive.tab };
        archive.key = queueKey(p);
        await changeQueue(p, () => archive);
        // 跨标签显式复制之后保留原记录与归档，原队列不再自动重放；未知服务端结果没有被抹掉。
        if (old.key !== this.queue?.key)
          await changeQueue({ ...this.partition, tab: old.tab }, (current) => {
            if (!current || current.snapshot !== old.snapshot)
              throw new UserFacingError('原标签输入已变化，请重新选择');
            return { ...current, retained: true };
          });
      }
      await changeQueue(this.partition, () => this.queue);
      this.stale = null;
      this.status();
      await this.findRetained();
    };
    if (old.tab === this.partition.tab) await retain();
    else
      await navigator.locks.request(
        'collaboration-report.tab:' + this.owner.actor + ':' + old.tab,
        { ifAvailable: true },
        async (lock) => {
          if (!lock) throw new UserFacingError('原标签已重新打开，请先在原标签处理');
          await retain();
        },
      );
  }
  offline = () => {
    this.connected = false;
    this.socket?.close();
    this.status();
  };
  hasVolatileInput() {
    return this.composing || this.local.length > 0 || this.persisting > 0;
  }
  async stop() {
    this.stopped = true;
    clearTimeout(this.reconnect);
    clearTimeout(this.structureTimer);
    this.socket?.close();
    await this.persistLocal();
    this.releaseTab?.();
    this.doc.off('update', this.onUpdate);
    this.doc.destroy();
  }
}
