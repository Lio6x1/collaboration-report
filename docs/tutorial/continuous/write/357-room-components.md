# 分步编写 `services/collaboration/src/bootstrap/room_components.mjs`

先按 [C07 · 协作完整依赖装配、传输与就绪](../lessons/collaboration/07-wiring-readiness.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/357-room-components.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

显式构建全服务唯一事务、房间队列、授权器、快照和输出管理器。

若每个用例各new队列，UPDATE与FLUSH会绕过彼此；若输出状态不共享，撤权找不到实际socket。

写之前先按这条执行路线理解各段的连接：

1. 先建Transactions/SerializedRooms/RoomSnapshots/RoomAdmissions/OutputSessions
2. 将相同实例注入UPDATE/Reading/Capture/Draft/MAIN/Structure各用例
3. MergeCandidates单独注入受限MergeExecutor
4. OutputRecovery复用相同snapshots/confirmations和ProcessOwner，返回完整组件对象

本文件需要守住的边界：

- instance来自COLLAB_RUN_OWNER加服务后缀，必须与监督启动身份一致
- 修改构造依赖时沿此唯一装配点传递

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/collaboration/src/bootstrap' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/collaboration/src/bootstrap/room_components.mjs') -and (Get-Item -LiteralPath 'services/collaboration/src/bootstrap/room_components.mjs').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/collaboration/src/bootstrap/room_components.mjs')) { New-Item -ItemType File -Path 'services/collaboration/src/bootstrap/room_components.mjs' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:1837 -->
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

````

## 第 2 步：createRoomComponents

插入位置：第 1 步代码之后，紧接追加；完成稿约第 28 行。

显式组合唯一队列/事务/授权/输出，避免每个用例各建一份状态导致UPDATE与FLUSH互相越过。

本步声明：`export function createRoomComponents(store, cfg, peers, { startupStage } = {}) `。

**这一段为什么这样写：** 在一处决定所有状态共享关系。

1. 先建基础能力
2. 按依赖顺序注入用例
3. 最后建恢复器并返回组件集合


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>JavaScript 参数</code> | 含mysql pool的存储对象<br>在此声明中的实际使用：<code>store.pool</code>；<code>return {     store,     cfg,     peers,     instance,     transactions,     queue,     snapshots,     admissions,     updates,     reading,     awareness,     captures,     mergeCa …</code>（长表达式，按本步完整代码继续） |
| <code>cfg</code> | <code>JavaScript 参数</code> | 服务配置<br>在此声明中的实际使用：<code>cfg.root</code>；<code>return {     store,     cfg,     peers,     instance,     transactions,     queue,     snapshots,     admissions,     updates,     reading,     awareness,     captures,     mergeCa …</code>（长表达式，按本步完整代码继续） |
| <code>peers</code> | <code>JavaScript 参数</code> | Doc私有客户端<br>在此声明中的实际使用：<code>return {     store,     cfg,     peers,     instance,     transactions,     queue,     snapshots,     admissions,     updates,     reading,     awareness,     captures,     mergeCa …</code>（长表达式，按本步完整代码继续） |
| <code>{ startupStage } = {}</code> | <code>带默认值的参数</code> | 可选固定阶段诊断回调 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(返回值)</code> | <code>JavaScript 返回值</code> | 实际返回表达式：<code>{     store,     cfg,     peers,     instance,     transactions,     queue,     snapshots,     admissions,     updates,     reading,     awareness,     captures,     mergeCandidate …</code>（长表达式，按本步完整代码继续） |

<!-- write-step: 2 bytes=1837:4088 -->
````javascript
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
````

## 第 3 步：createRoomComponents（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 77 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return {     store,     cfg,     peers,     instance,     transactions,     queue,     snapshots,     admissions,     updates,     reading,     awareness,     captures,     mergeCa …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=4088:4534 -->
````javascript
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

## 文件完成后的检查

```powershell
node --check 'services/collaboration/src/bootstrap/room_components.mjs'
if ($LASTEXITCODE -ne 0) { throw '本文件 JavaScript 语法错误' }
```

语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/collaboration/07-wiring-readiness.md)。
