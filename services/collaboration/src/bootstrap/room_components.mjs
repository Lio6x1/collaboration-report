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
