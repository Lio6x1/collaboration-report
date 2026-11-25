# D06｜审核合并、冲突修复与反向更改

## 这节课要实现什么

审核者同意修改时，主线可能已经被别人更新。本课实现审核合并、冲突处理以及对历史变更提出反向修改，避免简单覆盖主线。

## 做完后能看到什么

接通后，不冲突的修改可以合并；同一位置互相矛盾的修改会形成待处理冲突。反向更改作为新的可审核变更处理，不直接抹掉历史结果。

## 先想清楚怎样实现

1. 先固定共同基线、当前主线和送审稿三份输入。
2. 计算候选与冲突。
3. 确认权限和原尝试后提交结果，再安装到协作正文。
4. 冲突修复也沿用固定证据。

## 怎样判断这一课完成

D14后运行合并规则，06结合C04验证独立修改、同位置冲突和原尝试恢复；计算出候选并不表示已经合并成功。

## 开始写之前

继续[D05](./05-submission-review.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。合并先定义整个请求的历史提交者和冲突身份，不能先写一个覆盖MAIN的SQL。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/merge_review_models.go**，用途是合并先定义整个请求的历史提交者和冲突身份，不能先写一个覆盖MAIN的SQL。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 固定三方输入与整个请求的审核身份

先完成merge_review_models.go。合并不是“把送审正文覆盖MAIN”：Base是原基线，Submitted是原固定round，Main是本次固定可靠MAIN；它们的document/unit/structure及来源必须相容。一个request的历次submitters参与自审限制，不能只读当前一页历史。

在reviews_models.go已写的MergeApprovalFacts可提示按钮，但真正写入仍由merge逻辑再核：PositionReady、EverSubmitted、OwnOpen、ForeignOpen分别表示位置可用、曾经提交、自己的OPEN意见、他人的OPEN意见。管理者可豁免自审身份；他人OPEN意见仍阻止。自己意见最多100项，必须逐项携带准确revision并原子解除。

在merge_review_models.go按操作页定位并输入完整CanApproveMerge/ValidateMergeChanges等声明，再完成approval_actions.go、merges_queries.go的实际查询；queryer必须沿原事务读取完整submitter集合及当前guard。

## 3. 先固定尝试，再计算、提交、安装

完成merges_preparation_service.go的MergeClient、mergeContext、mergeAttemptContext、reserveAttempt及固定输入流程。MergeClient由collaboration_workflows_client.go实现，使用生成pb请求；Go不实现第二套Yjs合并。每次attempt有自己的ID、Generation与InputDigest，同一原尝试重试不允许换正文。

接着在merges_execution_service.go输入核心提交：

<!-- lesson-code: services/document/internal/logic/document/merges_execution_service.go :: Merges.commitMerge -->
````go
func (s *Merges) commitMerge(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, inputs mergeFixedInputs, candidate *pb.MergeCandidateReply, prepared *pb.MainBarrierReply, proofs []*pb.IdentityAdmission) error {
	attempt := inputs.Attempt
	content := candidate.GetCandidate()
	if candidate == nil || candidate.State != pb.CaptureReply_FIXED || len(candidate.Conflicts) != 0 || !bytes.Equal(candidate.InputDigest, inputs.Input.InputDigest) || content == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_CANDIDATE_UNKNOWN")
	}
	if err := validateFixedMergeContent(s.validator, content, fixed.Pointer.Document, fixed.Pointer.Unit, attempt.Main.ExpectedStructureId); err != nil {
		return err
	}
	if content.OperationId != task.Operation || content.CaptureId != task.Operation+"-merge-"+domain.TextID(attempt.ID) || content.BranchId != fixed.Pointer.Branch || content.WorkspaceEpoch != fixed.Context.Target.WorkspaceEpoch || content.LineageRef != domain.TextID(attempt.Round.Lineage) {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_CANDIDATE_BINDING_INVALID")
	}
	if !bytes.Equal(content.CutJsonUtf8, inputs.Input.Submitted.CutJsonUtf8) || !validProjectedSubmissionCut(content.CutJsonUtf8, attempt) {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_SUBMISSION_CUT_CHANGED")
	}
	if err := validateMergeCandidateProvenance(content, attempt); err != nil {
		return err
	}
	cutBytes, err := codec.Canonical(domain.CutBinding(inputs.Main.Cut))
	if err != nil {
		return err
	}
	cutDigest := sha256.Sum256(cutBytes)
	if prepared == nil || prepared.State != pb.MainBarrierReply_PREPARED || prepared.OperationId != task.Operation || prepared.BarrierGeneration != attempt.Generation || !proto.Equal(prepared.PreparedCut, inputs.Main.Cut) || !bytes.Equal(prepared.Digest, cutDigest[:]) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARED_UNKNOWN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	if current.CurrentManifestId != domain.TextID(attempt.Main.ExpectedManifestId) || current.CurrentStructureId != domain.TextID(attempt.Main.ExpectedStructureId) || current.CurrentStyleId != domain.TextID(attempt.Main.ExpectedStyleId) || current.ControlRevision != domain.TextID(attempt.Main.ExpectedControlRevision) {
		return domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Pointer.Document)
	if err != nil {
		return err
	}
	if barrier.Operation != task.Operation || barrier.Generation != attempt.Generation || barrier.Phase != "PREPARING" {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
	branch, err := repository.LockedActiveBranch(ctx, tx, fixed.Pointer.Unit, fixed.Pointer.Document)
	if err != nil || branch != fixed.Pointer.Branch {
		return domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	workspace, err := repository.LockedEditWorkspace(ctx, tx, branch, fixed.Pointer.Document, fixed.Pointer.Unit)
	if err != nil || workspace.Epoch != fixed.Context.Target.WorkspaceEpoch {
		return domain.Violation(domain.StateConflict, "WORKSPACE_CHANGED")
	}
	pointer, err := validateMergeReview(ctx, tx, fixed.Actor, fixed.Access, fixed.Pointer, fixed.Intent, true)
	if err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = validateMergeProofs(fixed, proofs); err != nil {
		return err
	}
	latest, err := repository.LatestMergeAttempt(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	if latest.ID != attempt.ID || latest.State != "FIXED" {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_ATTEMPT_SUPERSEDED")
	}
	entries, err := repository.ManifestUnits(ctx, tx, current.CurrentManifestId, pointer.Document)
	if err != nil {
		return err
	}
	if len(entries) != len(inputs.Main.Units) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
	}
	main := map[uint64]*pb.FixedContent{}
	for _, unit := range inputs.Main.Units {
		main[unit.UnitId] = unit
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	// candidate 和 MAIN 可靠尾部引用精确当前 Doc 的计费分配，保留原 resourceId。
	contents := append(append([]*pb.FixedContent(nil), inputs.Main.Units...), content)
	if err = lockFixedContentResources(ctx, tx, fixed.Pointer.Document, space, contents, "MERGE_RESOURCE_NOT_READY"); err != nil {
		return err
	}
	version, err := repository.InsertMergeVersion(ctx, tx, content, "MERGE", content.CaptureId)
	if err != nil {
		return err
	}
	manifest, err := repository.InsertMergedManifest(ctx, tx, pointer.Document, attempt.Main.ExpectedManifestId, attempt.Main.ExpectedStructureId, attempt.Main.ExpectedStyleId, task.Operation, attemptKey(task.Operation, attempt.ID))
	if err != nil {
		return err
	}
	control := attempt.Main.ExpectedControlRevision + 1
	units := make([]*pb.ManifestUnit, 0, len(entries))
	for _, entry := range entries {
		unit, _ := domain.CanonicalID(entry["unit_id"])
		oldVersion, _ := domain.CanonicalID(entry["content_version_id"])
		captured := main[unit]
		if captured == nil {
			return domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
		}
		chosen := oldVersion
		var body *pb.FixedContent
		if unit == pointer.Unit {
			chosen, body = version, proto.Clone(content).(*pb.FixedContent)
		} else {
			old, err := repository.VersionContent(ctx, tx, oldVersion, pointer.Document)
			if err != nil {
				return err
			}
			if bytes.Equal(old.BodyHash, captured.BodyHash) {
				if err = canonicalVersion(old, oldVersion); err != nil {
					return err
				}
				body = old
			} else {
				chosen, err = repository.InsertMergeVersion(ctx, tx, captured, "MAIN_CAPTURE", captured.CaptureId)
				if err != nil {
					return err
				}
				body = proto.Clone(captured).(*pb.FixedContent)
			}
		}
		if err = repository.InsertInitialManifestEntry(ctx, tx, pointer.Document, manifest, unit, chosen); err != nil {
			return err
		}
		room, err := collaborationRoom(ctx, tx, "MAIN", unit, unit, control)
		if err != nil {
			return err
		}
		body.BranchId, body.WorkspaceEpoch = 0, control
		body.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(control), "durable_seq": "0"})
		if err != nil {
			return err
		}
		units = append(units, &pb.ManifestUnit{UnitId: unit, ContentVersionId: chosen, Content: body})
	}
	record, err := repository.InsertMergeRecord(ctx, tx, task.Operation, pointer, attempt.ID, attempt.Main.ExpectedManifestId, manifest, version, attempt.Round.Lineage, attempt.Round.Capture)
	if err != nil {
		return err
	}
	if _, err = repository.InsertMergeApproval(ctx, tx, task.Operation, pointer, fixed.Actor, fixed.Intent.Version, record); err != nil {
		return err
	}
	for i, resolution := range fixed.Intent.Resolutions {
		if err = repository.InsertMergedResolution(ctx, tx, task.Operation, pointer, fixed.Actor, resolution.ID, i+1); err != nil {
			return err
		}
	}
	lineage, err := s.acceptMergeLineage(ctx, tx, fixed, inputs, workspace, record, manifest)
	if err != nil {
		return err
	}
	digest, err := domain.MainManifestDigest(manifest, units)
	if err != nil {
		return err
	}
	install, err := json.Marshal(mainInstallContext{Context: fixed.Context, Generation: attempt.Generation, Manifest: manifest, Units: units, Digest: digest[:], Lineage: lineage, Structure: attempt.Main.ExpectedStructureId, Style: attempt.Main.ExpectedStyleId, Control: control})
	if err != nil {
		return err
	}
	result := map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"merge_record_id": domain.TextID(record), "manifest_id": domain.TextID(manifest), "content_version_id": domain.TextID(version), "request_id": domain.TextID(pointer.ID), "round_id": domain.TextID(pointer.Round), "main_sync_state": "SYNCING"}}
	if err = repository.CompleteMerge(ctx, tx, task, pointer, fixed.Actor, record, manifest, attempt.ID, attempt.Generation, result, install); err != nil {
		return err
	}
	return tx.Commit()
}
````
<!-- /lesson-code -->

步骤是锁原operation/来源/文档→复核固定轮次、意见和当前MAIN barrier→校验worker/task代次→核候选摘要与准备回执→追加正文、manifest及merge记录→同事务解除准确的本人意见/记批准/完成业务结果/排MAIN_RECONCILE→Commit。不要先批准再计算；候选冲突时还没发生批准。

具体推演：原S1送审后支线继续写S2，当前MAIN也增加M。兼容合并将固定S1与M组成新MAIN，S2原草稿保留。若Node安装断连，Go提交事实仍SUCCEEDED并syncing；reconcile恢复同一install，不重新计算并吞进S2。审核者在Node断连期间提出新OPEN意见，最终提交再核应拒绝原merge，不留下早期批准。

conflict_contexts.go/conflicts_service.go固定真实attempt与每项原ordinal，不能从任意URL构造冲突。persistMergeConflicts保存整个冲突集合，冲突不是HTTP200就自动解除；没有可安装candidate时绝不能造正文。

## 4. 手工修复和反向更改也保留原证据

先写conflict_resolutions_queries.go，再写preparation/execution两份完整logic。在preparation文件定位fixedPreview，输入完整方法：

<!-- lesson-code: services/document/internal/logic/document/conflict_resolutions_preparation_service.go :: Resolutions.fixedPreview -->
````go
func (s *Resolutions) fixedPreview(ctx context.Context, tx *sql.Tx, raw []byte, fixed resolutionContext) (draftPrepared, error) {
	var expected draftPrepared
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_UNKNOWN")
	decoded, err := codec.Decode(raw)
	if err != nil {
		return expected, err
	}
	value, ok := decoded.(map[string]any)
	if !ok {
		return expected, unknown
	}
	digestText, ok := value["input_digest"].(string)
	if !ok {
		return expected, unknown
	}
	digest, err := base64.StdEncoding.DecodeString(digestText)
	if err != nil || len(digest) != 32 {
		return expected, unknown
	}
	delete(value, "input_digest")
	canonical, err := codec.Canonical(value)
	if err != nil {
		return expected, err
	}
	hash := sha256.Sum256(canonical)
	if !bytes.Equal(hash[:], digest) {
		return expected, unknown
	}
	bindingRaw, _ := json.Marshal(value["binding"])
	binding := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(bindingRaw, binding) != nil || !proto.Equal(binding, resolutionPreviewRequest(fixed)) || value["preview_cut_ref"] != fixed.Intent.Operation {
		return expected, unknown
	}
	source, ok := value["source"].(map[string]any)
	if !ok || (source["workspace_epoch"] != domain.TextID(draftMainEpoch(fixed.Scope)) && !(fixed.Scope.MainEpoch == 0 && source["workspace_epoch"] == nil)) {
		return expected, unknown
	}
	if !ok || source["main_manifest_id"] != domain.TextID(fixed.Scope.Manifest) || source["structure_id"] != domain.TextID(fixed.Scope.Structure) || source["style_id"] != domain.TextID(fixed.Scope.Style) || source["control_revision"] != domain.TextID(fixed.Scope.Control) || source["room_id"] != domain.TextID(fixed.Scope.Room) {
		return expected, unknown
	}
	original, versions, err := s.original(ctx, tx, fixed.Origin, fixed.Intent, false)
	if err != nil {
		return expected, err
	}
	if versions.Main != fixed.Versions.Main || versions.Submitted != fixed.Versions.Submitted || versions.Base != fixed.Versions.Base {
		return expected, unknown
	}
	baseline, err := resolutionBaseline(ctx, tx, fixed)
	if err != nil {
		return expected, err
	}
	wanted := map[string]any{}
	if err = resolutionSourceFields(wanted, fixed, original, baseline); err != nil {
		return expected, err
	}
	for key, expectedValue := range wanted {
		left, e1 := codec.Canonical(source[key])
		right, e2 := codec.Canonical(expectedValue)
		if e1 != nil || e2 != nil || !bytes.Equal(left, right) {
			return expected, unknown
		}
	}
	expected.Preview, expected.Digest = fixed.Intent.Operation, digest
	cutRaw, _ := json.Marshal(value["draft_cut"])
	if json.Unmarshal(cutRaw, &expected.Cut) != nil || expected.Cut["room_id"] != domain.TextID(fixed.Scope.Room) || expected.Cut["workspace_epoch"] != domain.TextID(fixed.Scope.Workspace.Epoch) {
		return expected, unknown
	}
	if _, err = domain.CanonicalRevision(expected.Cut["durable_seq"]); err != nil {
		return expected, unknown
	}
	if value["candidate"] == nil {
		return expected, domain.Violation(domain.StateConflict, "RESOLUTION_CONFLICT")
	}
	for _, key := range []string{"candidate", "main", "draft"} {
		part, _ := json.Marshal(value[key])
		content, e := decodePullFixedContent(part)
		if e != nil {
			return expected, e
		}
		if e = validateFixedMergeContent(s.validator, content, fixed.Scope.Document, fixed.Scope.Unit, fixed.Scope.Structure); e != nil {
			return expected, e
		}
		if content.OperationId != fixed.Intent.Operation {
			return expected, unknown
		}
		if key == "main" {
			if content.BranchId != 0 || content.WorkspaceEpoch != fixed.Scope.Control {
				return expected, unknown
			}
			expected.Main = content
		} else {
			if content.BranchId != fixed.Scope.Workspace.Workspace || content.WorkspaceEpoch != fixed.Scope.Workspace.Epoch || content.LineageRef != domain.TextID(fixed.Scope.Workspace.Lineage) {
				return expected, unknown
			}
			if _, e = resolutionBody(s.validator, content.BodyJsonUtf8); e != nil {
				return expected, unknown
			}
			if key == "candidate" {
				if content.CaptureId != fixed.Intent.Operation+":candidate" {
					return expected, unknown
				}
				expected.Candidate = content
			}
		}
	}
	return expected, nil
}
````
<!-- /lesson-code -->

它校验原binding、digest、preview_cut_ref、MAIN/草稿切点、base/submitted/main版本及candidate的完整body。resolutionBody拒绝重复稳定block ID，但同一resource ID被两张图引用是合法的，应保留。手工修复使用用户对整个原冲突集的选择，并和可靠后来草稿/MAIN做固定预览；不能为了修冲突丢S2或未见MAIN新增。

execution提交新lineage/epoch后仍需真实安装。确定的参数/目标失败可拒绝；proposal传输超时或未知响应要留原任务，不能同义转成用户修复失败。

reverse_changes_queries.go/reverse_changes_service.go从真实原merge的before/after和原范围生成反向候选，创建一份新的固定review。原MAIN已等于before时不再把S1加回；历史range已移除则先拒绝。反向请求仍走普通送审/审核，不能按按钮直接把MAIN覆盖回旧manifest。

用户“批准并合并”→merge_review_handlers.go→Merges.Prepare→原operation→RunMerges→reserveAttempt/真实MergeClient→commitMerge→reconcileMain；“修复冲突”经Resolutions同样先固定再安装。D14接入NewMerges/NewResolutions/NewReverseChanges与对应worker。

## 5. 验证与故障判断

包齐后离线：

~~~powershell
go -C services/document test -mod=readonly ./internal/domain/document -run '^(TestMergeEligibilityPreservesEverySubmitter|TestMergeResolutionsRequireAllCurrentOwnChanges|TestConflictResolutionChecksEntireOriginalConflictSet)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '上一组文档规则检查失败' }
go -C services/document test -mod=readonly ./internal/logic/document -run '^(TestResolutionBodyRejectsDuplicateStableBlockIdentities|TestResolutionBodyPreservesRepeatedInlineResourceReferences|TestResolutionRejectsOnlyDefiniteProposalOrTransferFailures)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '合并/冲突规则失败' }
~~~

源文件merge_test.go、conflict_resolution_test.go、conflict_resolution_body_test.go、conflict_resolution_errors_test.go；应精确RUN/PASS，验证身份、意见集合、稳定块和未知结果，尚未调用真实Node。

06 READY后：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/review/merge.test.mjs tests/review/conflict-resolutions.test.mjs tests/review/reverse-change.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实合并/修复/反向失败' }
~~~

actual approve-and-merge commits fixed S1, installs main and preserves S2 for the next submission 与 manual conflict repair preserves S2 and unseen MAIN additions before normal resubmission and approval 应同时成立。409先看round/revision/guard是否变；冲突看完整原集合；SUCCEEDED但仍syncing查原安装确认，不能改Go业务结果或删除Node房间。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/merge_review_models.go | [逐声明/函数带写](../../write/095-merge-review-models.md) |
| 2 | services/document/internal/repository/document/approval_actions.go | [逐声明/函数带写](../../write/145-approval-actions.md) |
| 3 | services/document/internal/repository/document/conflict_contexts.go | [逐声明/函数带写](../../write/148-conflict-contexts.md) |
| 4 | services/document/internal/repository/document/conflict_resolutions_queries.go | [逐声明/函数带写](../../write/149-conflict-resolutions-queries.md) |
| 5 | services/document/internal/repository/document/merges_queries.go | [逐声明/函数带写](../../write/160-merges-queries.md) |
| 6 | services/document/internal/repository/document/reverse_changes_queries.go | [逐声明/函数带写](../../write/173-reverse-changes-queries.md) |
| 7 | services/document/internal/logic/document/conflict_resolutions_execution_service.go | [逐声明/函数带写](../../write/197-conflict-resolutions-execution-service.md) |
| 8 | services/document/internal/logic/document/conflict_resolutions_preparation_service.go | [逐声明/函数带写](../../write/198-conflict-resolutions-preparation-service.md) |
| 9 | services/document/internal/logic/document/conflicts_service.go | [逐声明/函数带写](../../write/199-conflicts-service.md) |
| 10 | services/document/internal/logic/document/merges_execution_service.go | [逐声明/函数带写](../../write/210-merges-execution-service.md) |
| 11 | services/document/internal/logic/document/merges_preparation_service.go | [逐声明/函数带写](../../write/211-merges-preparation-service.md) |
| 12 | services/document/internal/logic/document/reverse_changes_service.go | [逐声明/函数带写](../../write/233-reverse-changes-service.md) |
| 13 | services/document/internal/client/collaboration_workflows_client.go | [逐声明/函数带写](../../write/109-collaboration-workflows-client.md) |
| 14 | services/document/internal/handler/document/merge_review_handlers.go | [逐声明/函数带写](../../write/254-merge-review-handlers.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D07](./07-publication.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/merge_review_models.go',
    'services/document/internal/repository/document/approval_actions.go',
    'services/document/internal/repository/document/conflict_contexts.go',
    'services/document/internal/repository/document/conflict_resolutions_queries.go',
    'services/document/internal/repository/document/merges_queries.go',
    'services/document/internal/repository/document/reverse_changes_queries.go',
    'services/document/internal/logic/document/conflict_resolutions_execution_service.go',
    'services/document/internal/logic/document/conflict_resolutions_preparation_service.go',
    'services/document/internal/logic/document/conflicts_service.go',
    'services/document/internal/logic/document/merges_execution_service.go',
    'services/document/internal/logic/document/merges_preparation_service.go',
    'services/document/internal/logic/document/reverse_changes_service.go',
    'services/document/internal/client/collaboration_workflows_client.go',
    'services/document/internal/handler/document/merge_review_handlers.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
