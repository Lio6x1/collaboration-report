# services/collaboration/src/bootstrap/room_components.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/357-room-components.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：注册路由、RPC及恢复器，连接入口和完整业务。

主要声明：`createRoomComponents`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { Transactions } from '../repository/sql.mjs';
import { RoomSnapshots as SnapshotRepository } from '../repository/room_snapshots.mjs';
import { RoomSnapshots } from '../application/room_snapshots.mjs';
import { SerializedRooms } from '../application/serialized_rooms.mjs';
import { RoomAdmissions } from '../application/room_admissions.mjs';
import { RoomUpdates } from '../application/room_io.mjs';
import { RoomReading } from '../application/room_io.mjs';
import { RoomAwareness } from '../application/room_io.mjs';
import { SubmissionCaptures } from '../application/submission_captures.mjs';
import { MergeCandidates } from '../application/merge_candidates.mjs';
import { MergeExecutor } from '../application/merge_executor.mjs';
import { MainCommits } from '../application/main_workflows.mjs';
import { MainCaptures } from '../application/main_workflows.mjs';
import { MainInstallations } from '../application/main_workflows.mjs';
import { DraftPreviews } from '../application/draft_previews.mjs';
import { StructurePreviews } from '../application/structure_previews.mjs';
import { StructurePreparations } from '../application/structure_preparations.mjs';
import { StructureInstallations } from '../application/structure_installations.mjs';
import { DraftTransitions } from '../application/draft_transitions.mjs';
import { OutputSessions } from '../transport/output_sessions.mjs';
import { ResourceConfirmations } from '../worker/resource_recovery.mjs';
import { OutputRecovery } from '../worker/output_recovery.mjs';
import { ProcessOwner } from '../client/process_owner.mjs';
import { AccountDocumentCleanup } from '../application/account_document_cleanup.mjs';
import { PersonalTransferStorage } from '../application/personal_transfer_storage.mjs';
import { CandidateCleanup } from '../application/candidate_cleanup.mjs';

// 显式组合唯一队列/事务/授权/输出，避免每个用例各建一份状态导致UPDATE与FLUSH互相越过。
export function createRoomComponents(store, cfg, peers, { startupStage } = {}) {
  const instance = (process.env.COLLAB_RUN_OWNER ?? '') + '-collaboration';
  const transactions = new Transactions(store.pool);
  const queue = new SerializedRooms();
  const snapshots = new RoomSnapshots(new SnapshotRepository(store.pool), store);
  const admissions = new RoomAdmissions(store, peers, instance, transactions, queue, snapshots);
  const outputs = new OutputSessions(store, instance, transactions, admissions);
  const confirmations = new ResourceConfirmations(store, peers);
  const accountDocumentCleanup = new AccountDocumentCleanup(transactions, peers);
  const personalTransferStorage = new PersonalTransferStorage(transactions);
  const candidateCleanup = new CandidateCleanup(transactions, peers);
  const updates = new RoomUpdates(
    peers,
    transactions,
    queue,
    snapshots,
    admissions,
    outputs,
    confirmations,
  );
  const reading = new RoomReading(transactions, queue, snapshots, admissions);
  const awareness = new RoomAwareness(transactions, admissions, outputs);
  const captures = new SubmissionCaptures(transactions, queue, snapshots, peers);
  const mergeCandidates = new MergeCandidates(transactions, new MergeExecutor());
  const structurePreparations = new StructurePreparations();
  const mainCommits = new MainCommits(transactions, peers, structurePreparations, outputs);
  const mainCaptures = new MainCaptures(transactions, snapshots, peers);
  const mainInstallations = new MainInstallations(
    transactions,
    snapshots,
    peers,
    outputs,
    structurePreparations,
    new StructureInstallations(snapshots),
  );
  const draftPreviews = new DraftPreviews(transactions, queue, snapshots, peers);
  const structurePreviews = new StructurePreviews(transactions, snapshots, peers, mainCaptures);
  const draftTransitions = new DraftTransitions(transactions, queue, snapshots, peers, outputs);
  const recovery = new OutputRecovery(
    store,
    cfg,
    instance,
    transactions,
    snapshots,
    confirmations,
    new ProcessOwner(cfg.root),
    startupStage,
  );
  return {
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
  };
}
````

UTF-8/LF全文SHA-256：`3ea2c1beba10e52da8df50c13519950642112841eede9e4d90298a39d8b972cc`。[参考实现](../../../../services/collaboration/src/bootstrap/room_components.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
