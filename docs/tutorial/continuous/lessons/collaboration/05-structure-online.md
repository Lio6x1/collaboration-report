# C05｜结构拆并与每个在线编辑器保存握手

## 这节课要实现什么

作者拆分章节时，另一位协作者可能正在旧章节打字。本课实现在线拆分协调，让主线结构变化能够迁移支线内容，并保护每个人已经输入的内容。

## 做完后能看到什么

接通后，章节、前言、子节与表格保持正确归属；落后支线先按固定证据生成迁移候选。每条参与连接都需确认自己的保存进度，不能由管理员替所有人确认后强行拆分。

## 先想清楚怎样实现

1. 先固定主线切点和支线映射。
2. 生成迁移预览并通知在线连接。
3. 逐个收集真实保存确认，核对未变化的预览后安装新结构与正文，同时保留旧位置的导航依据。

## 怎样判断这一课完成

C07后执行结构规则，06结合D04用两个编辑器验证拆分、落后支线和保存握手；未确认或有冲突时检查是否按流程停下。

## 开始写之前

从上一节已经写入的文件继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/collaboration/src/domain/structure_content.mjs` | 按H2稳定块拆章、并章；前言、表格和空节保持真实归属。 | [逐段带写](../../write/302-structure-content.md) |
| 2 | `services/collaboration/src/domain/structure_migration.mjs` | 将MAIN切点映射到落后草稿，先虚拟PULL再拆分，生成完整source interval与候选。 | [逐段带写](../../write/303-structure-migration.md) |
| 3 | `services/collaboration/src/domain/structure_preview_protocol.mjs` | 绑定整文档预览目的、fingerprint及结构来源，不接受局部替换目标。 | [逐段带写](../../write/305-structure-preview-protocol.md) |
| 4 | `services/collaboration/src/domain/structure_preparation.mjs` | 把已固定预览、Doc authority和原binding转换为受影响branch准备计划。 | [逐段带写](../../write/304-structure-preparation.md) |
| 5 | `services/collaboration/src/repository/structure_workspace.mjs` | 保存受影响branch的持久gate，锁定已有房间并防止虚拟房间后来悄悄加入。 | [逐段带写](../../write/324-structure-workspace.md) |
| 6 | `services/collaboration/src/application/structure_previews.mjs` | 同时固定MAIN与所有受影响草稿及lineage，生成整范围预览。 | [逐段带写](../../write/346-structure-previews.md) |
| 7 | `services/collaboration/src/application/structure_preparations.mjs` | 锁内重算实际cut、核房间存在性与lineage，完全一致后才冻结受影响branch。 | [逐段带写](../../write/345-structure-preparations.md) |
| 8 | `services/collaboration/src/application/structure_installations.mjs` | 按完整迁移候选建立新房间checkpoint/lineage，退休原房间并生成稳定块导航。 | [逐段带写](../../write/344-structure-installations.md) |
| 9 | `services/collaboration/src/transport/output_sessions.mjs` | 管理实际socket close及结构协调参与者；每连接必须提交本连接真实FLUSH再ACK。 | [逐段带写](../../write/349-output-sessions.md) |

## 2. 先定义拆章内容和原范围

先写structure_content，再写structure_migration。`splitChapterBody(input,{workspaceKind})` 以H2为章界；前言只在存在时留父章，空节的原H2作为管理边界节点保存，空正文使用可追溯的确定性占位；不把没有前言凭空补一章。`joinChapterBodies(parts,...)` 按固定边界拼回，必须保留原节点、图片ID、表格和稳定块ID。结构不能按纯文本换行切割，否则表格、公式、mark和资源归属会丢失。

`splitChapterDrafts(mainInput,draftInput)`/`buildStructureMigration(fixed)` 处理落后支线：MAIN后来新增H2，而支线从来没有这条H2，也可以先按真实lineage基线虚拟PULL、保留兼容草稿S2，再按MAIN切点映射拆分。三方有真实重叠冲突则停，不能为了拆章静默丢掉S2。返回migration中的changes/targets保存main_body、draft_body、source_ranges与原范围，不只有“新章数量”。

<!-- lesson-code: services/collaboration/src/domain/structure_content.mjs :: splitChapterBody -->
````javascript
// 只按顶层H2分界；表格、列表、H3及以下子树原样迁移。管理标题的完整原节点随映射保存供合章还原。
export function splitChapterBody(input, { workspaceKind = 'BRANCH' } = {}) {
  const body = normalizeContent(input, { workspaceKind });
  const pieces = [];
  let current = { kind: 'INTRODUCTION', title: '', nodes: [] };
  for (const node of body.content) {
    if (node.type === 'heading' && node.attrs.level === 2) {
      if (current.kind === 'SECTION' || current.nodes.length) pieces.push(current);
      current = { kind: 'SECTION', title: headingTitle(node), heading: node, nodes: [] };
    } else current.nodes.push(node);
  }
  if (current.kind === 'SECTION' || current.nodes.length) pieces.push(current);
  if (!pieces.some((piece) => piece.kind === 'SECTION')) throw failure('STRUCTURE_LEVEL2_MISSING');
  const originalIds = new Set(blockIds(body.content));
  return pieces.map(({ nodes, ...piece }) => {
    const part = {
      ...piece,
      blockIds: blockIds(piece.heading ? [piece.heading, ...nodes] : nodes),
    };
    if (!nodes.length) {
      // ProseMirror根节点要求至少一个块。确定性占位不冒充原来源；只有未改动的空占位可在合章时消除。
      part.emptyBlockId =
        'structure-empty-' +
        createHash('sha256').update(piece.heading.attrs.id).digest('hex').slice(0, 32);
      if (originalIds.has(part.emptyBlockId)) throw failure('STRUCTURE_BLOCK_ID_COLLISION');
      nodes = [emptyParagraph(part.emptyBlockId)];
    }
    part.body = normalizeContent({ type: 'doc', content: nodes }, { workspaceKind });
    return part;
  });
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/collaboration/src/domain/structure_migration.mjs :: buildStructureMigration -->
````javascript
// 只消费同一个固定预览；不访问时钟、数据库或活动房间，候选与范围声明一起受原摘要保护。
export function buildStructureMigration(fixed) {
  const source = fixed.source,
    target = fixed.binding?.context?.target;
  if (
    !target ||
    !source?.structure ||
    source.structure.id !== target.structure_id ||
    source.structure.document_id !== target.document_id ||
    !Array.isArray(source.structure.entries) ||
    !Array.isArray(source.changes)
  )
    throw failure('STRUCTURE_SCOPE_UNKNOWN');
  const { chapters, units } = chapterGroups(source.structure);
  const main = indexExact(fixed.main_units, units),
    drafts = indexExact(fixed.drafts, units);
  const workspaces = indexExact(source.drafts, units),
    bodies = new Map();
  for (const unit of units) {
    const workspace = workspaces.get(unit);
    bodies.set(unit, {
      main: decodeBody(main.get(unit), fixed, unit, '0'),
      draft: decodeBody(
        drafts.get(unit),
        fixed,
        unit,
        workspace.branch_id,
        workspace.workspace_epoch,
      ),
    });
  }
  const boundaries = new Map();
  for (const boundary of source.unit_boundaries ?? []) {
    if (!units.has(boundary.unit_id) || boundaries.has(boundary.unit_id))
      throw failure('STRUCTURE_BOUNDARY_UNKNOWN');
    boundaries.set(boundary.unit_id, boundary);
  }
  const changes = [],
    affected = new Set();
  for (const input of source.changes) {
    if (!rangeActions.has(input.type)) continue;
    const chapter = chapters.get(input.chapter_id);
    if (!chapter) throw failure('STRUCTURE_SCOPE_UNKNOWN');
    if (affected.has(input.chapter_id)) throw failure('STRUCTURE_RANGE_OVERLAP');
    affected.add(input.chapter_id);
    const change = {
      type: input.type,
      chapter_id: input.chapter_id,
      source_units: chapter.units.map((v) => v.unit_id),
      targets: [],
    };
    if (input.type === 'SPLIT_LEVEL2') {
      if (chapter.root.split_level2) throw failure('STRUCTURE_MODE_CHANGED');
      const pair = bodies.get(chapter.root.unit_id);
      change.targets = splitTargets(
        change,
        pair.main,
        pair.draft,
        workspaces.get(chapter.root.unit_id).lineage,
      );
    } else if (input.type === 'JOIN_CHAPTER') {
      if (!chapter.root.split_level2) throw failure('STRUCTURE_MODE_CHANGED');
      change.targets = [joinTarget(change, chapter, bodies, boundaries)];
    }
    for (const candidate of change.targets)
      if (
        Buffer.byteLength(canonical(candidate.main_body)) > 16 << 20 ||
        Buffer.byteLength(canonical(candidate.draft_body)) > 16 << 20
      )
        throw failure('STRUCTURE_CONTENT_LIMIT');
    changes.push(change);
  }
  return { schema_version: 'structure-migration-v1', changes };
}
````
<!-- /lesson-code -->

例子：父章是前言p-0、H2-A、表格t-1、H2-B、段落p-2。MAIN新增H2-B，落后支线只在t-1的一个单元格加字。虚拟对齐必须使t-1继续归A，p-2归B，前言留父章，支线新增字保留。支线若删除了MAIN正修改的t-1，则返回冲突，没有可安装candidate。join则使用固定边界节点重建，不能重新造H2 ID使旧选区失去位置。

## 3. 固定预览包含真实与虚拟房间

写 `structurePreviewBinding` 校验整文档purpose=STRUCTURE_PREVIEW与fingerprint；写 `structurePreparation(fixed,authority,binding,now)` 验原预览、有效期、Doc原authority与完整branch集合。`StructurePreviews.capture` 通过MainCaptures取得可靠MAIN，再固定受影响支线的cut/lineage。没有OPEN过的支线也以 `present:false` 保存在计划，不能把“现在没有room行”理解为没有需要冻结的范围。

先在repository/structure_workspace写branchGate/prepareBranchGate/finishBranchGate/lockBranchRooms，再写StructurePreparations。`prepare(connection,binding,authority,mainRooms)` 用同一事务锁实际BRANCH，检查room存在性、workspace/epoch/state、lineage与durable_seq；排序后重算cut与digest。只要任何事实变化就CHANGED并返回实际cut；完全一致才写持久gate并BLOCKED原受影响room。一个预览时不存在的branch后来刚OPEN且seq=0，也属于变化，不能仍当虚拟基线。未受影响branch不被冻结。

<!-- lesson-code: services/collaboration/src/application/structure_preparations.mjs :: StructurePreparations -->
````javascript
export class StructurePreparations {
  async plan(connection, binding, authority) {
    const [[row]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREVIEW_FIXED',
      authority.reference,
    );
    if (!row) throw new RPCFailure(14, 'STRUCTURE_PREPARATION_UNKNOWN');
    return structurePreparation(json(row.payload), authority, binding);
  }

  async prepare(connection, binding, authority, mainRooms) {
    const plan = await this.plan(connection, binding, authority);
    if (canonical(plan.cut) !== canonical(binding.expected_cut))
      throw new RPCFailure(6, 'STRUCTURE_PREVIEW_CUT_CONFLICT');
    const [branchRooms] = await previews.lockBranchRooms(
      connection,
      binding.context.target.document_id,
    );
    const actual = mainRooms.map((r) => ({
      room_id: r.id,
      workspace_epoch: r.epoch,
      durable_seq: r.last_durable_seq,
    }));
    for (const branch of plan.branches) {
      const found = branchRooms.filter((r) => r.unit_id === branch.unit_id);
      const room = found[0];
      // 预览时不存在的房间若后来OPEN，零尾也属于切点变化，不能当作原虚拟基线。
      if (found.length > 1 || !!room !== branch.present) return { state: 'CHANGED', cut: plan.cut };
      if (room) {
        if (
          room.id !== branch.room_id ||
          room.workspace_id !== branch.branch_id ||
          room.epoch !== branch.workspace_epoch ||
          room.state !== 'OPEN'
        )
          return { state: 'CHANGED', cut: plan.cut };
        const [[lineage]] = await lineages.lockRoomLineage(connection, room.id, room.epoch);
        if (
          !lineage ||
          canonical(draftLineage(json(lineage.payload))) !== canonical(draftLineage(branch.lineage))
        )
          return { state: 'CHANGED', cut: plan.cut };
        actual.push({
          room_id: room.id,
          workspace_epoch: room.epoch,
          durable_seq: room.last_durable_seq,
        });
      }
    }
    actual.sort((a, b) => (BigInt(a.room_id) < BigInt(b.room_id) ? -1 : 1));
    const cut = { ...plan.cut, rooms: actual, dependency_digest: cutDigest(actual) };
    if (canonical(cut) !== canonical(plan.cut)) return { state: 'CHANGED', cut };
    for (const branch of plan.branches) {
      const [[prior]] = await repository.branchGate(
        connection,
        branch.room_id,
        branch.workspace_epoch,
      );
      if (prior && json(prior.payload).state !== 'ABORTED')
        throw new RPCFailure(14, 'STRUCTURE_BRANCH_GATE_PENDING');
      await repository.prepareBranchGate(connection, branch, binding);
      if (branch.present)
        await artifacts.blockRoom(connection, branch.room_id, branch.workspace_epoch);
    }
    await artifacts.insertArtifact(
      connection,
      'STRUCTURE_PREPARATION',
      binding.context.operation_id,
      { binding, authority, branches: plan.branches },
    );
    return { state: 'PREPARED', cut };
  }

  async abort(connection, binding) {
    const [[row]] = await artifacts.artifact(
      connection,
      'STRUCTURE_PREPARATION',
      binding.context.operation_id,
    );
    if (!row) return;
    const stored = json(row.payload);
    if (canonical(stored.binding) !== canonical(binding))
      throw new RPCFailure(6, 'STRUCTURE_PREPARATION_CONFLICT');
    for (const branch of stored.branches) {
      await repository.finishBranchGate(connection, branch, binding, 'ABORTED');
      if (branch.present)
        await artifacts.openRoom(connection, branch.room_id, branch.workspace_epoch);
    }
  }
}
````
<!-- /lesson-code -->

## 4. 在线用户必须逐连接确认自己的真实FLUSH

现在按OutputSessions操作页顺序写attach/send/broadcast/settle，再写结构控制方法。构造参数 `store/instance/transactions/admissions` 使输出与C02当前资格共用；`outputs` 以delivery为键，不能只按用户计数，两个标签是两个真实输出。

`prepareStructure(binding,plan)` 固定本次参与连接，发送仅含operation/coordination/deadline的 `STRUCTURE_PAUSE`；8秒内每连接保存已经输入的本地更新，再在自己的WSS链发FLUSH。`RoomEngine.flush` 调reading取得真FLUSHED后调用 `recordFlush` 保存该delivery的原回执，客户端再发STRUCTURE_READY。`structureReady(b,input)` 比对原operation/coordination及该连接真实lastFlush，伪造receipt或借用另一连接的回执都返回 `STRUCTURE_SAVE_UNCONFIRMED`。

如果a、b两连接，a只报ready、b真实FLUSH，整体仍WAITING。a真实FLUSH并ACK后不允许继续UPDATE，返回 `STRUCTURE_CHANGE_PENDING`；ACK之前再次UPDATE会清lastFlush，不能沿旧证明参与；ACK之后UPDATE在assertStructureWrite处被拒绝。参与者固定后新连接被终止，避免“全员已保存”后又加入新写者。未确认连接断开或超时使协调FAILED，给其他连接STRUCTURE_RESUME；临时协调不是持久准备事实，重启重新握手。断线不是自动同意丢输入。

`MainCommits.prepare` 先在SQL外等待这个协调返回READY，再进入本课持久gate准备；WAITING返回PROCESSING，FAILED为 `STRUCTURE_EDITORS_NOT_READY`。不能持SQL行锁等待用户八秒，也不能只做在线握手就跳过锁内cut核验。

## 5. 安装、退休与稳定块导航

最后写StructureInstallations。`install(connection,context,transition,mainUnits)` 只消费Doc已固定transition，核原preview_ref/digest与targets数量完整；每个新branch都要核body_hash、MAIN正文、provenance、lineage、room/epoch与cut.seq=0。然后在同事务插新room/checkpoint/lineage、把原room退休、关闭旧grant/delivery、把持久branch gate记INSTALLED。缺一个target返回 `STRUCTURE_INSTALL_INCOMPLETE`，不能只安装能找到的那几章。

`navigation(connection,operation,transition)` 从保存的source_ranges得到稳定block→新unit对应。安装后 `installedStructure` 发送STRUCTURE_INSTALLED导航控制帧，只发unit定位，不夹正文；即使旧ACL已关闭，也能安全通知旧连接离开。进入新章仍须重新申请真实票据。不得将新正文塞给旧grant绕过权限。

## 6. 离线与06检查

```powershell
# 工作目录：自己的lessonRoot；本课与C01-C04相关文件已写齐，无真实依赖。
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 services/collaboration/tests/structure-content.test.mjs services/collaboration/tests/structure-migration.test.mjs services/collaboration/tests/structure-preparation.test.mjs services/collaboration/tests/structure-branch-gate.test.mjs services/collaboration/tests/structure-editor-coordination.test.mjs
if ($LASTEXITCODE -ne 0) { throw '结构迁移或在线协调规则失败' }
```

精确测试 `split virtually pulls new MAIN H2 while preserving independent pending edits and original fixed bytes` 核落后支线；`absent affected branch is still part of the durable gate scope` 核虚拟范围；`structure switch waits for each connection own real FLUSH and blocks new writes after ACK` 核伪造回执与逐连接冻结；`unconfirmed disconnect cancels pause and a new operation can start without losing original inputs` 核断线释放；`installed navigation uses saved stable block ownership and does not send document content` 核导航无正文。预期PASS、fail/skip=0。规则测试的socket/SQL替身不能证明浏览器或数据库已经实际完成结构切换。

06在完整READY、开发SMTP身份夹具及本学习独立文档下执行：

```powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/review/structure-migration-preview.test.mjs tests/review/structure-transition.test.mjs tests/review/structure-rich-boundaries.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实结构固定预览或安装失败' }
```

精确测试 `actual structure preview fixes split candidates from reliable MAIN and S2 with the original digest`、`actual split and join install new common drafts, retire old identities and preserve accepted MAIN separately`、`actual repeated split and join preserve introduction, edited empty section, MAIN fonts and pinned images` 核真实固定内容、新旧房间与资源。两浏览器在线保存还可在06按既定Edge/Playwright前置运行 `tests/browser/safe-structure-editors.test.mjs`，精确名 `real online editors preserve stale branch edits, reject changed previews and reopen split sections`；只用本学习工程的运行配置与身份，不读取体验工程配置。本次文档编写没有启动这些环境组。出现 `STRUCTURE_PREVIEW_CUT_CONFLICT` 查固定预览与实际cut；`STRUCTURE_EDITORS_NOT_READY` 查哪个连接没收到自身FLUSHED；`STRUCTURE_BRANCH_GATE_UNKNOWN` 查原operation/generation，不能手改gate为INSTALLED。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/collaboration/src/domain/structure_content.mjs`：[本课完成文件](../../source/302-structure-content.md)。
- `services/collaboration/src/domain/structure_migration.mjs`：[本课完成文件](../../source/303-structure-migration.md)。
- `services/collaboration/src/domain/structure_preview_protocol.mjs`：[本课完成文件](../../source/305-structure-preview-protocol.md)。
- `services/collaboration/src/domain/structure_preparation.mjs`：[本课完成文件](../../source/304-structure-preparation.md)。
- `services/collaboration/src/repository/structure_workspace.mjs`：[本课完成文件](../../source/324-structure-workspace.md)。
- `services/collaboration/src/application/structure_previews.mjs`：[本课完成文件](../../source/346-structure-previews.md)。
- `services/collaboration/src/application/structure_preparations.mjs`：[本课完成文件](../../source/345-structure-preparations.md)。
- `services/collaboration/src/application/structure_installations.mjs`：[本课完成文件](../../source/344-structure-installations.md)。
- `services/collaboration/src/transport/output_sessions.mjs`：[本课完成文件](../../source/349-output-sessions.md)。

下一节：[ C06 固定候选清理、账号清理与转移计量 ](06-cleanup-storage.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/collaboration/src/domain/structure_content.mjs',
    'services/collaboration/src/domain/structure_migration.mjs',
    'services/collaboration/src/domain/structure_preview_protocol.mjs',
    'services/collaboration/src/domain/structure_preparation.mjs',
    'services/collaboration/src/repository/structure_workspace.mjs',
    'services/collaboration/src/application/structure_previews.mjs',
    'services/collaboration/src/application/structure_preparations.mjs',
    'services/collaboration/src/application/structure_installations.mjs',
    'services/collaboration/src/transport/output_sessions.mjs'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
