# C04｜固定送审、三方合并与草稿变换

## 这节课要实现什么

送审需要固定当时的内容，审核通过需要合并到主线，支线更新也不能覆盖未处理修改。本课实现协作服务为这些工作流提供的内容捕获、合并和安装。

## 做完后能看到什么

接通后，本轮审核内容固定，后续编辑不会混入；独立修改可合并，冲突会保留给人工处理。候选计算、文档服务提交和协作房间安装有各自状态，重复请求继续同一结果。

## 先想清楚怎样实现

1. 先保存原捕获位置与三方输入。
2. 在隔离候选中计算并记录冲突。
3. 由文档服务确认权威结果后安装到主线或支线，超时则查询原操作，不重新选版本。

## 怎样判断这一课完成

C07齐备后运行完整协作规则，06配合D03、D05、D06验证实际送审、合并及支线更新，分别核对持久提交与安装完成。

## 开始写之前

从上一节已经写入的文件继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/collaboration/src/domain/merge/values.mjs` | 按语义值与MAIN字体规则合并属性，集中记录冲突而非最后写入覆盖。 | [逐段带写](../../write/295-values.md) |
| 2 | `services/collaboration/src/domain/merge/inline.mjs` | 按grapheme与mark合并段内独立改动，同位置不同插入必须冲突。 | [逐段带写](../../write/294-inline.md) |
| 3 | `services/collaboration/src/domain/merge/blocks.mjs` | 用稳定块/行/单元格ID合并树、移动及删除；删除父树不能悄悄吞掉另一侧子编辑。 | [逐段带写](../../write/293-blocks.md) |
| 4 | `services/collaboration/src/domain/content_merge.mjs` | 验证三份固定正文并调用值、段内与块算法，输出候选或明确冲突集合。 | [逐段带写](../../write/287-content-merge.md) |
| 5 | `services/collaboration/src/domain/merge_protocol.mjs` | 绑定base/submitted/main三份固定内容的身份、摘要和attempt，拒绝置换同字正文。 | [逐段带写](../../write/296-merge-protocol.md) |
| 6 | `services/collaboration/src/domain/capture_protocol.mjs` | 定义SUBMISSION的原FLUSH、lineage、actor与fingerprint绑定，并编码固定正文/provenance。 | [逐段带写](../../write/286-capture-protocol.md) |
| 7 | `services/collaboration/src/domain/main_protocol.mjs` | 按数值room顺序规范whole-document cut，绑定MAIN准备屏障输入。 | [逐段带写](../../write/291-main-protocol.md) |
| 8 | `services/collaboration/src/domain/main_workspace.mjs` | 区分物理MAIN epoch与后来metadata revision，准备只能接受可独立核验的原capture。 | [逐段带写](../../write/292-main-workspace.md) |
| 9 | `services/collaboration/src/domain/resolution_draft.mjs` | 把原submitted到人工resolution的差量应用到后来草稿S2，重叠修改则停。 | [逐段带写](../../write/298-resolution-draft.md) |
| 10 | `services/collaboration/src/domain/reverse_draft.mjs` | 反向撤销原固定变更，同时保留后来MAIN和草稿中兼容的改动。 | [逐段带写](../../write/299-reverse-draft.md) |
| 11 | `services/collaboration/src/domain/draft_protocol.mjs` | 固定草稿及MAIN、lineage、cut，提供PULL/弃稿投影和共享安装约束；REVERSE/人工修复算法由各自领域文件提供。 | [逐段带写](../../write/289-draft-protocol.md) |
| 12 | `services/collaboration/src/domain/discard_confirmation.mjs` | 从原预览派生新确认绑定，保持预览正文、原来源及可靠MAIN cut不变。 | [逐段带写](../../write/288-discard-confirmation.md) |
| 13 | `services/collaboration/src/repository/candidate_artifacts.mjs` | 保存CUT_FIXED/capture/merge候选及room lineage，原操作重试读取原固定输入。 | [逐段带写](../../write/310-candidate-artifacts.md) |
| 14 | `services/collaboration/src/repository/draft_transitions.mjs` | 保存预览/确认/准备artifact，阻止受控room继续写并按原操作安装新epoch。 | [逐段带写](../../write/311-draft-transitions.md) |
| 15 | `services/collaboration/src/repository/main_workspace.mjs` | 锁文档→屏障→MAIN房间与固定capture，保存准备和安装回执、退休旧MAIN与旧delivery。 | [逐段带写](../../write/313-main-workspace.md) |
| 16 | `services/collaboration/src/application/submission_captures.mjs` | 短事务先固定cut和lineage，再只从该cut生成送审正文；后来S2不进入原轮次。 | [逐段带写](../../write/347-submission-captures.md) |
| 17 | `services/collaboration/src/application/merge_executor.mjs` | 把CPU重的三方合并交给有界worker thread，超时终止原计算而非堵住WSS。 | [逐段带写](../../write/334-merge-executor.md) |
| 18 | `services/collaboration/src/worker/merge_thread.mjs` | 在线程内消费固定三方输入并回传候选/冲突，不访问数据库或当前房间。 | [逐段带写](../../write/354-merge-thread.md) |
| 19 | `services/collaboration/src/application/merge_candidates.mjs` | 按原attempt先登记绑定、执行固定合并、再落定唯一结果；重试不取新草稿。 | [逐段带写](../../write/333-merge-candidates.md) |
| 20 | `services/collaboration/src/application/draft_previews.mjs` | 从实际可靠MAIN/草稿固定预览及原切点，私有fixed读取核验原来源。 | [逐段带写](../../write/329-draft-previews.md) |
| 21 | `services/collaboration/src/application/draft_transitions.mjs` | 重核预览输入、Doc权威与屏障，安装新草稿epoch并保留旧版本。 | [逐段带写](../../write/330-draft-transitions.md) |
| 22 | `services/collaboration/src/application/main_workflows.mjs` | 实现可靠MAIN读取、准备与安装；Go业务已提交后通过原manifest续接Node安装。 | [逐段带写](../../write/332-main-workflows.md) |

## 2. 先定义三方输入与固定内容身份

按文件表从merge/values开始。`base` 是支线真实lineage基线，`submitted` 是已固定送审S1，`main` 是当前可靠主线的固定捕获；三个参数都必须通过content-v1，不能拿浏览器当前编辑内容替换S1。`MergeValues` 比较语义值，把MAIN字体作为受控覆盖；`mergeInline` 按grapheme对齐，不把一个emoji拆成两个UTF-16字符；`MergeBlocks` 用稳定块树、表格行和单元格身份合并删除、移动与插入。

`mergeContent({base,submitted,main})` 成功返回 `{body,conflicts:[]}`，冲突返回 `{body:null,conflicts:[...]}`。例如base有p-1“原文”，submitted只改p-1，main只新增p-2，两份独立修改都保留；两侧都在p-1相同位置插入不同文字则明确冲突。同一表格不同row编辑可合并，删除一row与另侧编辑该row必须冲突，不能把旧行复活。组合完的正文还要校验表格几何，算法得到树不代表可以安装。

<!-- lesson-code: services/collaboration/src/domain/content_merge.mjs :: mergeContent -->
````javascript
// 三方计算只消费固定业务正文。任何冲突都不交付可安装候选，调用方不能偷偷选择保底主线。
export function mergeContent({ base, submitted, main }) {
  const validate = (body) => normalizeContent(body, { workspaceKind: 'MAIN' });
  [base, submitted, main] = [base, submitted, main].map(validate);
  if (same(base, submitted) || same(main, submitted)) return { body: clone(main), conflicts: [] };
  if (equal(base, main) && !hasFont(main))
    return { body: validate(semantic(submitted)), conflicts: [] };
  const values = new MergeValues();
  const result = { type: 'doc', content: new MergeBlocks(values).roots(base, submitted, main) };
  if (!values.conflicts.length) {
    try {
      return { body: validate(result), conflicts: [] };
    } catch {
      values.report(
        'INVALID_MERGE_RESULT',
        '',
        '$root',
        '组合后的正文无法通过结构与表格几何校验。',
      );
    }
  }
  return { body: null, conflicts: values.conflicts };
}
````
<!-- /lesson-code -->

接着写merge_protocol的 `fixedContentBinding → mergeBinding → mergeInputDigest → validateMergeInput`。绑定包含文档/unit/structure、原capture或content version、body/provenance摘要及attempt；两份正文看起来一样也不能交换不同送审operation的身份。`captureBinding` 则限定purpose=SUBMISSION，actor与target全uint64字符串、32字节fingerprint、原FLUSH、guard_generation与lineage_ref都必需。`storeContent/wireContent` 只在SQL JSON与RPC bytes之间做base64转换，不重新规范正文。

<!-- lesson-code: services/collaboration/src/domain/capture_protocol.mjs :: captureBinding -->
````javascript
export function captureBinding(input) {
  const context = input.context,
    target = context?.target;
  if (
    !validID(context?.operation_id) ||
    !positive(context?.actor_id) ||
    !target ||
    !['document_id', 'unit_id', 'branch_id', 'structure_id', 'workspace_epoch'].every((key) =>
      positive(target[key]),
    ) ||
    target.purpose !== 'SUBMISSION' ||
    !validID(input.flush_receipt_id) ||
    !positive(input.guard_generation) ||
    !positive(input.lineage_ref) ||
    Buffer.from(context.request_fingerprint ?? []).length !== 32 ||
    !context.document_admission_ref ||
    !context.identity_admission_ref
  )
    throw bad('CAPTURE_INVALID');
  return {
    context: {
      ...context,
      request_fingerprint: Buffer.from(context.request_fingerprint).toString('base64'),
    },
    flush_receipt_id: input.flush_receipt_id,
    guard_generation: input.guard_generation,
    lineage_ref: input.lineage_ref,
  };
}
````
<!-- /lesson-code -->

## 3. 写数据库固定点，再实现送审捕获

先在candidate_artifacts操作页写room lineage、原capture operation gate、cut与candidate查询/更新，再写main_workspace与draft_transitions的artifact/屏障SQL。所有函数收外层connection，锁序沿来源→文档MAIN入口→原操作→房间；不能在仓储内begin/commit另一笔事务。

`SubmissionCaptures.capture(input)` 先从原FLUSH回执找到room，在同room队列中短事务锁sources、原operation、FLUSH、room和lineage。回执必须FIXED、actor与epoch匹配，房间必须属于同document/unit/branch且OPEN。这里FLUSH只证明依赖已经到达；首次capture实际记录 `room.last_durable_seq` 为原cut，并同事务保存真实lineage。提交之后 `complete(binding)` 只能restore到这个cut，生成body/hash/resource_ids/provenance并落候选。`CUT_FIXED` 以后失败仍能从原cut继续，不重新采新草稿。

<!-- lesson-code: services/collaboration/src/application/submission_captures.mjs :: SubmissionCaptures -->
````javascript
export class SubmissionCaptures {
  constructor(transactions, queue, snapshots, peers) {
    this.transactions = transactions;
    this.queue = queue;
    this.snapshots = snapshots;
    this.peers = peers;
  }

  async capture(input) {
    const binding = captureBinding(input),
      target = binding.context.target;
    const roomID = await this.transactions.run(async (connection) => {
      const [[prior]] = await captures.findCapture(connection, binding.context.operation_id);
      if (prior) {
        sameBinding(json(prior.candidate_json).binding, binding);
        return prior.room_id;
      }
      const [[receipt]] = await flushes.findFlushReceipt(connection, binding.flush_receipt_id);
      if (!receipt) throw deny('FLUSH_BINDING_INVALID');
      return receipt.room_id;
    });
    return this.queue.run(roomID, async () => {
      // 与UPDATE/FLUSH共用队列。首个短事务先固定cut，第二事务只能从原cut生成正文。
      await this.transactions.run(async (connection) => {
        await lockSources(connection, binding.context.sources);
        const [[prior]] = await captures.lockOperation(connection, binding.context.operation_id);
        if (prior) {
          sameBinding(json(prior.candidate_json).binding, binding);
          return prior.room_id;
        }
        const [[receipt]] = await flushes.findFlushReceipt(connection, binding.flush_receipt_id);
        if (
          !receipt ||
          receipt.state !== 'FIXED' ||
          receipt.actor_id !== binding.context.actor_id ||
          receipt.epoch !== target.workspace_epoch
        )
          throw deny('FLUSH_BINDING_INVALID');
        const [[room]] = await rooms.lockRoom(connection, receipt.room_id);
        if (
          !room ||
          room.state !== 'OPEN' ||
          room.workspace_kind !== 'BRANCH' ||
          room.epoch !== target.workspace_epoch ||
          room.document_id !== target.document_id ||
          room.unit_id !== target.unit_id ||
          room.workspace_id !== target.branch_id
        )
          throw deny('WORKSPACE_CHANGED');
        const covered = json(receipt.covered_cut);
        if (
          covered.room_id !== room.id ||
          covered.workspace_epoch !== room.epoch ||
          BigInt(covered.durable_seq) > BigInt(room.last_durable_seq)
        )
          throw deny('FLUSH_CUT_INVALID');
        const [[stored]] = await captures.lockRoomLineage(connection, room.id, room.epoch);
        if (!stored || json(stored.payload).lineage_id !== binding.lineage_ref)
          throw deny('LINEAGE_CHANGED');
        const cut = {
          room_id: room.id,
          workspace_epoch: room.epoch,
          durable_seq: String(room.last_durable_seq),
        };
        await captures.insertCut(connection, binding, room, cut, json(stored.payload));
        return room.id;
      });
      return this.complete(binding);
    });
  }

  async complete(binding) {
    return this.transactions.run(async (connection) => {
      await lockSources(connection, binding.context.sources);
      const [[row]] = await captures.lockOperation(connection, binding.context.operation_id);
      const candidate = json(row.candidate_json);
      sameBinding(candidate.binding, binding);
      if (candidate.cleaned) throw new RPCFailure(9, 'CAPTURE_CANDIDATE_CLEANED');
      if (['CAPTURED', 'BOUND'].includes(row.state))
        return { state: 'FIXED', content: wireContent(candidate.content) };
      if (row.state !== 'CUT_FIXED') return { state: 'REJECTED', business_code: 'CAPTURE_CLOSED' };
      const [[room]] = await rooms.lockRoom(connection, row.room_id);
      if (!room || room.epoch !== row.epoch || room.state !== 'OPEN')
        throw deny('WORKSPACE_CHANGED');
      // 切点与来源在首次事务一并固定；同epoch后来接纳主线，也不能改变这个候选的base/provenance。
      const lineage = candidate.lineage,
        cut = json(row.cut_ref);
      if (!lineage) throw new RPCFailure(14, 'CAPTURE_LINEAGE_UNKNOWN');
      const doc = await this.snapshots.restore(connection, room, cut.durable_seq);
      try {
        const body = normalizeContent(fromPM(yDocToProsemirrorJSON(doc))),
          bytes = Buffer.from(canonical(body)),
          digest = hash(bytes),
          resources = resourceIds(body);
        const content = {
          ...binding.context.target,
          capture_id: row.operation_id,
          operation_id: row.operation_id,
          cut_json_utf8: Buffer.from(canonical(cut)),
          lineage_ref: row.lineage_ref,
          body_json_utf8: bytes,
          schema_version: 'content-v1',
          canonicalization_version: 'canonical-v1',
          hash_algorithm: 'SHA-256',
          body_hash: digest,
          byte_size: String(bytes.length),
          resource_ids: resources,
          provenance_json_utf8: Buffer.from(
            canonical(captureProvenance(body, lineage, row.operation_id)),
          ),
          provenance_schema: 'provenance-v1',
        };
        await captures.saveCandidate(
          connection,
          row.operation_id,
          { binding, content: storeContent(content) },
          bytes,
          digest,
          resources,
        );
        return { state: 'FIXED', content };
      } finally {
        doc.destroy();
      }
    });
  }

  async get(query) {
    if (
      !validID(query.operation_id) ||
      Buffer.from(query.request_fingerprint ?? []).length !== 32 ||
      !query.target
    )
      throw bad('CAPTURE_QUERY_INVALID');
    const result = await this.transactions.run(async (connection) => {
      const [[row]] = await captures.findCapture(connection, query.operation_id);
      if (!row) return null;
      const binding = json(row.candidate_json).binding;
      matchCaptureQuery(row, binding, query);
      await lockSources(connection, binding.context.sources);
      return { row, binding };
    });
    if (!result) return { state: 'PROCESSING' };
    return this.queue.run(result.row.room_id, () => this.complete(result.binding));
  }

  async finalize(input) {
    const supplied = input.authoritative_result;
    if (!input.context || input.capture_id !== input.context.operation_id || !supplied)
      throw bad('FINALIZE_CAPTURE_INVALID');
    // 系统收尾查真实Go终态；不能把调用参数或网络超时当作可绑定/可删除许可。
    const result = await this.peers.rpc('GetOperationResult', {
      operation_id: input.context.operation_id,
      target: input.context.target,
      request_fingerprint: input.context.request_fingerprint,
    });
    if (result.state !== 'SUCCEEDED') throw new RPCFailure(14, 'CAPTURE_RESULT_UNKNOWN');
    if (
      supplied.state !== result.state ||
      supplied.operation_id !== result.operation_id ||
      Buffer.from(supplied.result_digest).toString('hex') !==
        Buffer.from(result.result_digest).toString('hex') ||
      Buffer.from(supplied.fixed_result_json_utf8).toString('base64') !==
        Buffer.from(result.fixed_result_json_utf8).toString('base64')
    )
      throw new RPCFailure(6, 'CAPTURE_RESULT_CONFLICT');
    await this.transactions.run(async (connection) => {
      const [[row]] = await captures.lockOperation(connection, input.capture_id);
      if (!row) throw deny('CAPTURE_NOT_FOUND');
      matchCaptureQuery(row, json(row.candidate_json).binding, { ...input.context });
      const reference =
        result.operation_id + ':' + Buffer.from(result.result_digest).toString('hex');
      if (row.go_result_ref && row.go_result_ref !== reference)
        throw new RPCFailure(6, 'CAPTURE_RESULT_CONFLICT');
      if (!['CAPTURED', 'BOUND'].includes(row.state)) throw deny('CAPTURE_NOT_FIXED');
      await captures.bindResult(connection, row.operation_id, reference);
    });
    return result;
  }

  async committedCut(query) {
    if (!validID(query.operation_id) || !query.target || query.target.purpose !== 'SUBMISSION')
      throw bad('CAPTURE_QUERY_INVALID');
    const fingerprint = Buffer.from(query.request_fingerprint ?? '', 'base64');
    if (fingerprint.length !== 32) throw bad('CAPTURE_QUERY_INVALID');
    const rpcQuery = { ...query, request_fingerprint: fingerprint };
    // 这是已提交送审的系统事实读取：重查Go固定终态，而不是沿用旧用户的准入许可。
    // 新审核者仍由Go合并用例检查当前权限；旧来源关闭不能抹掉早已成功的切点。
    const authority = await this.peers.rpc('GetOperationResult', rpcQuery);
    if (authority.state !== 'SUCCEEDED' || authority.operation_id !== query.operation_id)
      throw new RPCFailure(14, 'CAPTURE_RESULT_UNKNOWN');
    const fixed = JSON.parse(Buffer.from(authority.fixed_result_json_utf8).toString('utf8'));
    if (!/^[1-9][0-9]*$/.test(fixed.result?.content_version_id ?? ''))
      throw new RPCFailure(14, 'CAPTURE_RESULT_UNKNOWN');
    return this.transactions.run(async (connection) => {
      const [[row]] = await captures.findCapture(connection, query.operation_id);
      if (!row || !['CAPTURED', 'BOUND'].includes(row.state))
        throw new RPCFailure(14, 'CAPTURE_NOT_FIXED');
      const candidate = json(row.candidate_json);
      matchCaptureQuery(row, candidate.binding, rpcQuery);
      if (
        !candidate.content ||
        candidate.content.capture_id !== query.operation_id ||
        candidate.content.body_hash !== row.body_hash.toString('base64')
      )
        throw new RPCFailure(14, 'CAPTURE_FIXED_BINDING_UNKNOWN');
      return {
        operation_id: query.operation_id,
        content_version_id: fixed.result.content_version_id,
        cut_json_utf8: candidate.content.cut_json_utf8,
        body_hash: candidate.content.body_hash,
      };
    });
  }
}
````
<!-- /lesson-code -->

推演：送审时room.seq=12固定S1，生成正文前网络断了；用户又保存seq=13的S2。重试同operation读CUT_FIXED=12，所以原轮次仍S1；新的S2留给下一次送审。另一个actor的FLUSH、换epoch或换lineage都拒绝，不能因为正文相同放行。`captureProvenance` 对未改块保留旧来源、已改块记录旧版本、插入块记原capture、删除块记DELETED。它为后面的结构映射与冲突定位保留依据，不能把所有来源改成当前操作。

## 4. 固定合并结果与CPU线程接线

先写worker/merge_thread，再写MergeExecutor和MergeCandidates。`MergeExecutor.compute(input)` 是唯一线程入口：把固定输入交线程，设有界预算，返回真实候选/冲突；线程不查SQL、不取当前房间。MergeCandidates首笔事务登记原attempt绑定，SQL锁外运行CPU计算，第二事务再核原绑定与清理墓碑后保存唯一结果。原结果已落定，重试读取原固定结果。不能把worker超时当“无冲突”或用MAIN作保底候选。

```powershell
# 离线纯规则与线程测试：本课列表的merge相关文件写齐后执行。
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 services/collaboration/tests/merge.test.mjs services/collaboration/tests/merge-executor.test.mjs services/collaboration/tests/resolution-draft.test.mjs services/collaboration/tests/reverse-draft.test.mjs
if ($LASTEXITCODE -ne 0) { throw '固定三方合并或草稿差量失败' }
```

重点精确测试 `separate grapheme insertions inside one paragraph both survive`、`deleted table row is not resurrected and a concurrent edit is an explicit conflict`、`heavy merge computation keeps the main event loop responsive`、`manual resolution applies only its original submitted delta and retains later draft S2`、`compatible reverse retains MAIN additions and later reliable draft S2`。预期全部PASS、skip=0、退出0。若只有线程测试失败先核merge_thread的相对URL与worker消息格式，不能改回主线程阻塞；Unicode合并失败先核grapheme对齐而非延长超时。

## 5. 草稿预览、确认与安装不是同一个动作

写完resolution_draft、reverse_draft后写draft_protocol，再写discard_confirmation。`computePull(base,draft,main)` 把可靠MAIN增量带入当前草稿；人工resolution只取原submitted→resolution的差量叠到S2；reverse撤掉原before→after的变化并保留兼容的后来MAIN/S2；DISCARD候选是原固定MAIN投影。四者不可共用“直接覆盖当前草稿”。

`DraftPreviews.capture` 先向Doc核原已登记用例，固定branch/MAIN/cut/lineage及原输入digest；`fixed` 私有读取也核原绑定。`DraftTransitions.prepare` 先取这个固定artifact，锁外询问Doc `/infra/draft/context`，然后锁内核原cut及预算，再冻结room。弃稿是新确认操作，`deriveDiscardConfirmation` 从原预览派生新binding，保留原预览字节和sources，不能把确认操作当重新采样。`apply` 接受PULL_MAIN/DISCARD_DRAFT/REVERSE/CONFLICT_RESOLUTION，读取Doc固定系统结果，核 `validateDraftInstall` 的可靠候选、MAIN基线和accepted provenance，建立新epoch的checkpoint/lineage，旧room退休、旧grant关闭、旧输出停止；旧草稿版本仍留存。

同一预览后草稿从12变13，旧prepare拒绝，保持13；同一预览后MAIN可靠尾变化，也不能把旧MAIN覆盖回去。新epoch必须由真实Doc结果给出，不能在Node本地猜 `epoch+1`。C06提供 `requireDocumentBodyPresent` 防止迟到安装复活已清理文档，C05提供结构准备器，未写齐时不要import整套业务做检查。

## 6. MAIN可靠读取、准备、Go提交与Node安装

先写main_protocol的storedCut与prepareBinding，再写main_workspace规则，最后main_workflows的 `MainCaptures → MainCommits → MainInstallations`。whole-document cut包含document/structure/style/control_revision、所有room的epoch与durable_seq及dependency_digest；room按BigInt数值排序，不能按字符串 `10<2` 排序。

`MainCaptures.capture` 固定真实可靠MAIN读取视图，readonly仍要原admission有效；捕获不偷改业务正式指针。物理房间epoch与metadata control_revision可能出现合法间隔，所以 `matchesMainWorkspace`/`preparedMainSource` 要查独立保存的原capture，不能简单放行“任何较小epoch”。`MainCommits.prepare` 锁来源、文档、MAIN屏障和所有MAIN房间，重算cut完全一致才PREPARED；不同返回CHANGED且保留实际cut。准备屏障挡后来的MAIN OPEN/UPDATE，普通BRANCH保存仍可继续。C05的结构用例另外冻结受影响BRANCH。

<!-- lesson-code: services/collaboration/src/application/main_workflows.mjs :: MainCommits -->
````javascript
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
````
<!-- /lesson-code -->

Go文档服务提交正式manifest以后，`MainInstallations.reconcile` 沿原operation/manifest安装Node房间。Go成功但Node不可达是“业务成功、安装同步中”，不能回滚成未批准，也不能重新生成merge。安装先核原准备与候选，写新checkpoint/lineage、退休旧room、持久回执，再真实排空旧输出；重复请求读相同回执。abort只释放与原operation/generation匹配的屏障，不能清掉别人的控制。

## 7. RPC/HTTP接线和06真实测试

C07 createRPC实际注册 `CaptureSubmission/GetCapture/FinalizeCapture` 到captures，`ComputeMergeCandidate` 到mergeCandidates，`CaptureMainView/PrepareMainCommit/ReconcileMainCommit` 到MAIN三类，`CaptureDraftPreview/ApplyDraftResolution` 到preview/transitions。私有HTTP `/infra/persistence/submission-cut`、`/infra/main/revision`、`/infra/draft/fixed`、`/infra/draft/prepare` 供Doc沿原系统任务恢复，不能把它们暴露给浏览器直接写入。

06完整READY以后分组串行执行，前面的离线测试不代替这些证据：

```powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/review/capture.test.mjs tests/review/capture-recovery.test.mjs tests/review/merge.test.mjs tests/review/main-barrier.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实固定送审、合并或MAIN屏障失败' }
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/review/pull-main.test.mjs tests/review/discard-draft.test.mjs tests/review/conflict-resolutions.test.mjs tests/review/reverse-change.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实草稿变换失败' }
```

精确观察 `actual capture fixes durable S1 and replay stays S1 after S2`、`actual committed CUT_FIXED survives a failure before candidate generation and excludes later S2`、`actual approve-and-merge commits fixed S1, installs main and preserves S2 for the next submission`、`an actual document-wide main barrier blocks a late MAIN room while BRANCH still saves`。`explicit discard installs fixed MAIN in a new branch epoch and preserves old draft` 核旧版本没有被删；`explicit pull preserves reliable draft and installs a new branch epoch under the original operation` 核原号与新epoch。预期各项PASS且fail/skip=0。`CAPTURE_ID_CONFLICT` 先核原fingerprint与cut；`MAIN_PREPARE_CAPTURE_UNKNOWN` 查实际capture而非修改epoch比较；`DRAFT_TRANSITION_ID_CONFLICT` 查预览身份/原attempt/digest，不重置artifact。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/collaboration/src/domain/merge/values.mjs`：[本课完成文件](../../source/295-values.md)。
- `services/collaboration/src/domain/merge/inline.mjs`：[本课完成文件](../../source/294-inline.md)。
- `services/collaboration/src/domain/merge/blocks.mjs`：[本课完成文件](../../source/293-blocks.md)。
- `services/collaboration/src/domain/content_merge.mjs`：[本课完成文件](../../source/287-content-merge.md)。
- `services/collaboration/src/domain/merge_protocol.mjs`：[本课完成文件](../../source/296-merge-protocol.md)。
- `services/collaboration/src/domain/capture_protocol.mjs`：[本课完成文件](../../source/286-capture-protocol.md)。
- `services/collaboration/src/domain/main_protocol.mjs`：[本课完成文件](../../source/291-main-protocol.md)。
- `services/collaboration/src/domain/main_workspace.mjs`：[本课完成文件](../../source/292-main-workspace.md)。
- `services/collaboration/src/domain/resolution_draft.mjs`：[本课完成文件](../../source/298-resolution-draft.md)。
- `services/collaboration/src/domain/reverse_draft.mjs`：[本课完成文件](../../source/299-reverse-draft.md)。
- `services/collaboration/src/domain/draft_protocol.mjs`：[本课完成文件](../../source/289-draft-protocol.md)。
- `services/collaboration/src/domain/discard_confirmation.mjs`：[本课完成文件](../../source/288-discard-confirmation.md)。
- `services/collaboration/src/repository/candidate_artifacts.mjs`：[本课完成文件](../../source/310-candidate-artifacts.md)。
- `services/collaboration/src/repository/draft_transitions.mjs`：[本课完成文件](../../source/311-draft-transitions.md)。
- `services/collaboration/src/repository/main_workspace.mjs`：[本课完成文件](../../source/313-main-workspace.md)。
- `services/collaboration/src/application/submission_captures.mjs`：[本课完成文件](../../source/347-submission-captures.md)。
- `services/collaboration/src/application/merge_executor.mjs`：[本课完成文件](../../source/334-merge-executor.md)。
- `services/collaboration/src/worker/merge_thread.mjs`：[本课完成文件](../../source/354-merge-thread.md)。
- `services/collaboration/src/application/merge_candidates.mjs`：[本课完成文件](../../source/333-merge-candidates.md)。
- `services/collaboration/src/application/draft_previews.mjs`：[本课完成文件](../../source/329-draft-previews.md)。
- `services/collaboration/src/application/draft_transitions.mjs`：[本课完成文件](../../source/330-draft-transitions.md)。
- `services/collaboration/src/application/main_workflows.mjs`：[本课完成文件](../../source/332-main-workflows.md)。

下一节：[ C05 结构拆并与每个在线编辑器保存握手 ](05-structure-online.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/collaboration/src/domain/merge/values.mjs',
    'services/collaboration/src/domain/merge/inline.mjs',
    'services/collaboration/src/domain/merge/blocks.mjs',
    'services/collaboration/src/domain/content_merge.mjs',
    'services/collaboration/src/domain/merge_protocol.mjs',
    'services/collaboration/src/domain/capture_protocol.mjs',
    'services/collaboration/src/domain/main_protocol.mjs',
    'services/collaboration/src/domain/main_workspace.mjs',
    'services/collaboration/src/domain/resolution_draft.mjs',
    'services/collaboration/src/domain/reverse_draft.mjs',
    'services/collaboration/src/domain/draft_protocol.mjs',
    'services/collaboration/src/domain/discard_confirmation.mjs',
    'services/collaboration/src/repository/candidate_artifacts.mjs',
    'services/collaboration/src/repository/draft_transitions.mjs',
    'services/collaboration/src/repository/main_workspace.mjs',
    'services/collaboration/src/application/submission_captures.mjs',
    'services/collaboration/src/application/merge_executor.mjs',
    'services/collaboration/src/worker/merge_thread.mjs',
    'services/collaboration/src/application/merge_candidates.mjs',
    'services/collaboration/src/application/draft_previews.mjs',
    'services/collaboration/src/application/draft_transitions.mjs',
    'services/collaboration/src/application/main_workflows.mjs'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
