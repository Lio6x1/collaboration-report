# services/collaboration/src/application/room_engine.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/339-room-engine.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。

主要声明：`RoomEngine`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
// RoomEngine保留服务调用面，协调已分工的用例；这里不解释正文、执行SQL或直接发送socket。
export class RoomEngine {
  constructor({
    store,
    cfg,
    peers,
    instance,
    transactions,
    queue,
    snapshots,
    admissions,
    updates,
    reading,
    awareness,
    captures,
    mergeCandidates,
    mainCommits,
    mainCaptures,
    mainInstallations,
    draftPreviews,
    structurePreviews,
    draftTransitions,
    outputs,
    confirmations,
    recovery,
    accountDocumentCleanup,
    personalTransferStorage,
    candidateCleanup,
  }) {
    this.store = store;
    this.cfg = cfg;
    this.peers = peers;
    this.instance = instance;
    this.transactions = transactions;
    this.queue = queue;
    this.snapshots = snapshots;
    this.admissions = admissions;
    this.updates = updates;
    this.reading = reading;
    this.awarenessUpdates = awareness;
    this.captures = captures;
    this.mergeCandidates = mergeCandidates;
    this.mainCommits = mainCommits;
    this.mainCaptures = mainCaptures;
    this.mainInstallations = mainInstallations;
    this.draftPreviews = draftPreviews;
    this.structurePreviews = structurePreviews;
    this.draftTransitions = draftTransitions;
    this.deliveryOutputs = outputs;
    this.confirmations = confirmations;
    this.recovery = recovery;
    this.accountDocumentCleanup = accountDocumentCleanup;
    this.personalTransferStorage = personalTransferStorage;
    this.candidateCleanup = candidateCleanup;
  }
  get outputs() {
    return this.deliveryOutputs.outputs;
  }
  get roomLocks() {
    return this.queue.locks;
  }
  get recovered() {
    return this.recovery.recovered;
  }
  async transaction(body) {
    return this.transactions.run(body);
  }
  async serialized(room, body) {
    return this.queue.run(room, body);
  }
  async initialize() {
    return this.recovery.initialize();
  }
  async verifyRooms() {
    return this.snapshots.verifyRooms();
  }
  async restore(connection, room, cut = room.last_durable_seq) {
    return this.snapshots.restore(connection, room, cut);
  }
  async checkpoint(connection, room, doc, cut) {
    return this.snapshots.checkpoint(connection, room, doc, cut);
  }
  async open(input, handler) {
    return this.admissions.open(input, handler);
  }
  async authorized(connection, binding, input) {
    return this.admissions.authorized(connection, binding, input);
  }
  async update(binding, input) {
    this.deliveryOutputs.assertStructureWrite(binding);
    const output = this.outputs.get(binding.delivery);
    if (output) output.lastFlush = null;
    return this.updates.update(binding, input);
  }
  async flush(binding, input) {
    const result = await this.reading.flush(binding, input);
    this.deliveryOutputs.recordFlush(binding, result);
    return result;
  }
  structureReady(binding, input) {
    this.deliveryOutputs.structureReady(binding, input);
  }
  async sync(binding, input) {
    return this.reading.sync(binding, input);
  }
  async awareness(binding, input) {
    return this.awarenessUpdates.publish(binding, input);
  }
  attach(binding, socket) {
    return this.deliveryOutputs.attach(binding, socket);
  }
  async send(binding, message) {
    return this.deliveryOutputs.send(binding, message);
  }
  async broadcast(room, message) {
    return this.deliveryOutputs.broadcast(room, message);
  }
  async settle(binding, reason) {
    return this.deliveryOutputs.settle(binding, reason);
  }
  async recoverResources(operation) {
    return this.confirmations.recover(operation);
  }
  async stopScope(scope) {
    return this.deliveryOutputs.stopScope(scope);
  }
}
````

UTF-8/LF全文SHA-256：`8663e6db2ef48e339f0114acf42edf3a2dd9ba3287a2c1901434132443d403d5`。[参考实现](../../../../services/collaboration/src/application/room_engine.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
