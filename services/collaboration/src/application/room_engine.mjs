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
