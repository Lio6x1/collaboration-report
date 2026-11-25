// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"sort"
)

// structure_transition_commit.go：保留原实现，按业务合并文件。

func (s *MainControls) commitStructure(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery, confirmation structureConfirmation, cut *pb.CutVector, prepared *pb.MainBarrierReply, proofs []*pb.IdentityAdmission) error {
	fixed := recovery.Fixed
	raw, err := codec.Canonical(domain.CutBinding(cut))
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	if prepared == nil || prepared.State != pb.MainBarrierReply_PREPARED || prepared.OperationId != task.Operation || prepared.BarrierGeneration != fixed.Generation || !proto.Equal(prepared.PreparedCut, cut) || !bytes.Equal(prepared.Digest, digest[:]) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARED_UNKNOWN")
	}
	// RPC在锁外，最终事务重新核验原号→来源→Doc→屏障→任务→完整预览与审核guard，追加及退役整体提交。
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
	if err = domain.ValidateMainControl(fixed.Access.Permissions, current.ControlRevision, current.CurrentStyleId, current.MainLocked, fixed.Intent); err != nil {
		return err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return err
	}
	if barrier.Operation != task.Operation || barrier.Generation != fixed.Generation || barrier.Phase != "PREPARING" {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = validateMainControlProofs(fixed, proofs); err != nil {
		return err
	}
	actual, err := s.loadStructureConfirmation(ctx, tx, fixed, proofs)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual.Raw, confirmation.Raw) {
		return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_FIXED_UNKNOWN")
	}
	affected := map[string]bool{}
	for _, change := range actual.Migration.Changes {
		for _, id := range change.Sources {
			affected[id] = true
		}
	}
	for _, unit := range actual.Preview.Units {
		if !affected[domain.TextID(unit.Unit)] {
			continue
		}
		if err = repository.RequireBranchControlOpen(ctx, tx, unit.Workspace.Workspace); err != nil {
			return err
		}
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	contents := append(append([]*pb.FixedContent{}, actual.Snapshot.Main...), actual.Snapshot.Drafts...)
	if err = lockFixedContentResources(ctx, tx, fixed.Document, space, contents, "STRUCTURE_RESOURCE_NOT_READY"); err != nil {
		return err
	}
	old, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return err
	}
	structure, err := repository.InsertControlledStructure(ctx, tx, fixed.Document, fixed.Structure, task.Operation)
	if err != nil {
		return err
	}
	identities := map[string]domain.StructureMigrationIdentity{}
	for _, change := range actual.Migration.Changes {
		chapter, _ := domain.CanonicalID(change.Chapter)
		for _, target := range change.Targets {
			unit, err := repository.CreateStructureUnit(ctx, tx, fixed.Document, chapter, structure, target.Kind)
			if err != nil {
				return err
			}
			node, err := NewEntityID()
			if err != nil {
				return err
			}
			identities[target.Key] = domain.StructureMigrationIdentity{Unit: domain.TextID(unit), Node: domain.TextID(node)}
		}
	}
	entries, err := domain.ApplyStructureMigration(old.Entries, *fixed.Intent.Structure, actual.Migration, identities)
	if err != nil {
		return err
	}
	if err = repository.InsertStructureEntries(ctx, tx, fixed.Document, structure, entries); err != nil {
		return err
	}
	manifest, err := repository.InsertControlledManifest(ctx, tx, fixed.Document, fixed.Manifest, structure, fixed.Style, task.Operation, fixed.Intent.Kind, actual.Preview.Intent.Operation)
	if err != nil {
		return err
	}
	units, transition, err := s.materializeStructureUnits(ctx, tx, fixed, actual, structure, manifest, identities)
	if err != nil {
		return err
	}
	for _, unit := range actual.Preview.Units {
		if affected[domain.TextID(unit.Unit)] {
			if err = repository.RetireStructureUnit(ctx, tx, fixed.Document, unit.Unit, unit.Workspace.Workspace, structure); err != nil {
				return err
			}
		}
	}
	for _, change := range actual.Migration.Changes {
		if change.Type == "REMOVE" {
			chapter, _ := domain.CanonicalID(change.Chapter)
			if err = repository.RetireStructureChapter(ctx, tx, fixed.Document, chapter, structure); err != nil {
				return err
			}
		}
	}
	transition.ACLEpoch, _ = domain.CanonicalID(current.AclRevision)
	if err = repository.AdvanceDocumentACL(ctx, tx, fixed.Document); err != nil {
		return err
	}
	if err = QueueClosure(ctx, tx, task.Operation, fixed.Actor, domain.Closure{Scope: "doc-acl:" + domain.TextID(fixed.Document), Epoch: transition.ACLEpoch, Entity: fixed.Document, Action: "ACL"}); err != nil {
		return err
	}
	manifestDigest, err := domain.MainManifestDigest(manifest, units)
	if err != nil {
		return err
	}
	install, err := json.Marshal(mainInstallContext{Context: fixed.Context, Generation: fixed.Generation, Manifest: manifest, Units: units, Digest: manifestDigest[:], Structure: structure, Style: fixed.Style, Control: fixed.Control + 1, Entity: structure, Transition: transition})
	if err != nil {
		return err
	}
	result := map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"manifest_id": domain.TextID(manifest), "entity_id": domain.TextID(structure), "main_sync_state": "SYNCING"}}
	if err = repository.PointToControlledStructure(ctx, tx, fixed.Document, structure); err != nil {
		return err
	}
	if err = repository.CompleteMainControl(ctx, tx, task, fixed.Document, fixed.Generation, manifest, fixed.Style, current.MainLocked, result, install); err != nil {
		return err
	}
	return tx.Commit()
}

// structure_transition_content.go：保留原实现，按业务合并文件。

type structureCandidateContext struct {
	Document, Unit, Structure, Branch, Epoch, Lineage, Manifest, BaseVersion uint64
	Operation, Preview, Key, Relation                                        string
	SourceVersions                                                           map[string]uint64
	Sources                                                                  []*pb.FixedContent
}

// MAIN和未审核草稿各自构造固定内容。来源仅引用实际固定版本/原预览，不把S2标为已接纳。
func buildStructureCandidate(validator *codec.Validator, raw json.RawMessage, fixed structureCandidateContext) (*pb.FixedContent, error) {
	value, err := codec.Decode(raw)
	if err != nil {
		return nil, err
	}
	if err = validator.Body(value); err != nil {
		return nil, err
	}
	body, err := codec.Canonical(value)
	if err != nil {
		return nil, err
	}
	origins := map[string]uint64{}
	accepted := []any{}
	seenAccepted := map[string]bool{}
	for _, source := range fixed.Sources {
		sourceBody, err := codec.Decode(source.BodyJsonUtf8)
		if err != nil {
			return nil, err
		}
		visitStructureBlocks(sourceBody, func(id string, _ map[string]any) { origins[id] = fixed.SourceVersions[domain.TextID(source.UnitId)] })
		prior, err := codec.Decode(source.ProvenanceJsonUtf8)
		if err != nil {
			return nil, err
		}
		for _, capture := range prior.(map[string]any)["accepted_captures"].([]any) {
			key, err := codec.Canonical(capture)
			if err != nil {
				return nil, err
			}
			if !seenAccepted[string(key)] {
				seenAccepted[string(key)] = true
				accepted = append(accepted, capture)
			}
		}
	}
	blocks := []any{}
	resources := map[uint64]bool{}
	visitStructureNodes(value, func(node map[string]any) {
		// 图片是无块ID的行内节点；按共享正文schema的resourceId收集，独立于块谱系。
		attrs, _ := node["attrs"].(map[string]any)
		if node["type"] == "image" {
			if resource, ok := attrs["resourceId"].(string); ok {
				if resourceID, err := domain.CanonicalID(resource); err == nil {
					resources[resourceID] = true
				}
			}
		}
		id, ok := attrs["id"].(string)
		if !ok {
			return
		}
		item := map[string]any{"block_id": id, "origin_block_id": id, "relation": fixed.Relation, "source_range": map[string]any{"start": json.Number("0"), "end": json.Number("0")}}
		if version := origins[id]; version != 0 {
			item["origin_content_version_id"] = domain.TextID(version)
		} else {
			item["origin_capture_ref"] = fixed.Preview
			item["relation"] = "INSERTED"
		}
		blocks = append(blocks, item)
	})
	provenance := map[string]any{"schema_version": "provenance-v1", "lineage_id": domain.TextID(fixed.Lineage), "base_manifest_id": domain.TextID(fixed.Manifest), "base_content_version_id": domain.TextID(fixed.BaseVersion), "source_operation_id": fixed.Operation, "accepted_captures": accepted, "block_sources": blocks}
	if err = validator.Provenance(provenance); err != nil {
		return nil, err
	}
	proof, err := codec.Canonical(provenance)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(resources))
	for id := range resources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	digest := sha256.Sum256(body)
	content := &pb.FixedContent{OperationId: fixed.Operation, DocumentId: fixed.Document, UnitId: fixed.Unit, StructureId: fixed.Structure, BranchId: fixed.Branch, WorkspaceEpoch: fixed.Epoch, CaptureId: fixed.Operation + ":" + fixed.Key, LineageRef: domain.TextID(fixed.Lineage), BodyJsonUtf8: body, BodyHash: digest[:], SchemaVersion: "content-v1", CanonicalizationVersion: "canonical-v1", HashAlgorithm: "SHA-256", ByteSize: uint64(len(body)), ResourceIds: ids, ProvenanceJsonUtf8: proof, ProvenanceSchema: "provenance-v1"}
	if err = domain.ValidateFixedContentResources(value, content); err != nil {
		return nil, err
	}
	return content, nil
}

func visitStructureBlocks(value any, visit func(string, map[string]any)) {
	visitStructureNodes(value, func(node map[string]any) {
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["id"].(string); ok {
				visit(id, node)
			}
		}
	})
}

func visitStructureNodes(value any, visit func(map[string]any)) {
	switch node := value.(type) {
	case map[string]any:
		visit(node)
		if children, ok := node["content"].([]any); ok {
			for _, child := range children {
				visitStructureNodes(child, visit)
			}
		}
	case []any:
		for _, child := range node {
			visitStructureNodes(child, visit)
		}
	}
}

// structure_transition_install.go：保留原实现，按业务合并文件。

func structureInstallationWire(fixed *structureInstallation) (any, error) {
	if fixed == nil {
		return nil, nil
	}
	branches := make([]any, 0, len(fixed.Branches))
	for _, branch := range fixed.Branches {
		content, err := fixedWire(branch.Content)
		if err != nil {
			return nil, err
		}
		branches = append(branches, map[string]any{"candidate_key": branch.Key, "unit_id": domain.TextID(branch.Unit), "branch_id": domain.TextID(branch.Branch), "room_id": domain.TextID(branch.Room), "epoch": domain.TextID(branch.Epoch), "lineage_id": domain.TextID(branch.Lineage), "content": content, "lineage_context": branch.Context})
	}
	return map[string]any{"preview_cut_ref": fixed.Preview, "preview_digest": base64.StdEncoding.EncodeToString(fixed.Digest), "acl_epoch": domain.TextID(fixed.ACLEpoch), "branches": branches}, nil
}

// structure_transition_readiness.go：保留原实现，按业务合并文件。

func validStructureInstallation(original mainControlContext, install mainInstallContext) bool {
	fixed := install.Transition
	if original.StructurePreview == nil {
		return fixed == nil
	}
	epoch, err := domain.CanonicalID(original.Access.Document.AclRevision)
	if err != nil || fixed == nil || fixed.ACLEpoch != epoch || fixed.Preview != original.StructurePreview.Reference || !bytes.Equal(fixed.Digest, original.StructurePreview.Digest) {
		return false
	}
	units := map[uint64]uint64{}
	rooms, keys := map[uint64]bool{}, map[string]bool{}
	for _, unit := range install.Units {
		if unit == nil {
			return false
		}
		units[unit.UnitId] = unit.ContentVersionId
	}
	for _, branch := range fixed.Branches {
		content := branch.Content
		if branch.Key == "" || keys[branch.Key] || branch.Unit == 0 || units[branch.Unit] == 0 || branch.Branch == 0 || branch.Room == 0 || rooms[branch.Room] || branch.Epoch != 1 || branch.Lineage == 0 || content == nil || content.DocumentId != original.Document || content.UnitId != branch.Unit || content.BranchId != branch.Branch || content.WorkspaceEpoch != branch.Epoch || content.StructureId != install.Structure || content.OperationId != original.Intent.Operation || content.LineageRef != domain.TextID(branch.Lineage) || content.HashAlgorithm != "SHA-256" || content.ByteSize != uint64(len(content.BodyJsonUtf8)) {
			return false
		}
		body, err := codec.Decode(content.BodyJsonUtf8)
		if err != nil {
			return false
		}
		raw, err := codec.Canonical(body)
		digest := sha256.Sum256(raw)
		if err != nil || !bytes.Equal(raw, content.BodyJsonUtf8) || !bytes.Equal(digest[:], content.BodyHash) {
			return false
		}
		var cut map[string]string
		if json.Unmarshal(content.CutJsonUtf8, &cut) != nil || cut["room_id"] != domain.TextID(branch.Room) || cut["workspace_epoch"] != "1" || cut["durable_seq"] != "0" {
			return false
		}
		if branch.Context["lineage_id"] != domain.TextID(branch.Lineage) || branch.Context["base_manifest_id"] != domain.TextID(install.Manifest) || branch.Context["base_version_id"] != domain.TextID(units[branch.Unit]) {
			return false
		}
		encoded, err := json.Marshal(branch.Context["provenance"])
		if err != nil {
			return false
		}
		decoded, err := codec.Decode(encoded)
		if err != nil {
			return false
		}
		proof, err := codec.Canonical(decoded)
		if err != nil || !bytes.Equal(proof, content.ProvenanceJsonUtf8) {
			return false
		}
		rooms[branch.Room] = true
		keys[branch.Key] = true
	}
	return true
}

// structure_transition_recovery.go：保留原实现，按业务合并文件。

// 协调服务已确定未准备完成的编辑器才拒绝本次确认；断连和未知RPC仍沿原号恢复，不能误判为安全未提交。
func structurePreparationError(err error) error {
	if status.Code(err) == codes.FailedPrecondition && status.Convert(err).Message() == "STRUCTURE_EDITORS_NOT_READY" {
		return domain.Violation(domain.StateConflict, "STRUCTURE_EDITORS_NOT_READY")
	}
	return err
}

func (s *MainControls) recoverStructure(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery, proofs []*pb.IdentityAdmission) error {
	fixed := recovery.Fixed
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	if _, err = ValidateAccess(ctx, tx, fixed.Access); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	confirmation, err := s.loadStructureConfirmation(ctx, tx, fixed, proofs)
	if err != nil {
		return err
	}
	cut, err := structureConfirmationCut(confirmation)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// 确认只消费原预览；CHANGED拒绝该原号，不能沿原号采新cut或推进attempt替换用户确认对象。
	prepared, err := s.client.PrepareMain(ctx, &pb.PrepareMainCommitRequest{Context: fixed.Context, BarrierGeneration: fixed.Generation, ExpectedCut: cut})
	if err != nil {
		return structurePreparationError(err)
	}
	if prepared == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_UNKNOWN")
	}
	if prepared.State == pb.MainBarrierReply_CHANGED {
		return domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CHANGED")
	}
	if prepared.State != pb.MainBarrierReply_PREPARED {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_PENDING")
	}
	return s.commitStructure(ctx, task, recovery, confirmation, cut, prepared, proofs)
}

// structure_transition_units.go：保留原实现，按业务合并文件。

type structureInstalledBranch struct {
	Key                                string `json:"candidate_key"`
	Unit, Branch, Room, Epoch, Lineage uint64
	Content                            *pb.FixedContent `json:"content"`
	Context                            map[string]any   `json:"lineage_context"`
}
type structureInstallation struct {
	Preview  string                     `json:"preview"`
	Digest   []byte                     `json:"preview_digest"`
	ACLEpoch uint64                     `json:"acl_epoch"`
	Branches []structureInstalledBranch `json:"branches"`
}

func (s *MainControls) materializeStructureUnits(ctx context.Context, tx *sql.Tx, fixed mainControlContext, confirmation structureConfirmation, structure, manifest uint64, identities map[string]domain.StructureMigrationIdentity) ([]*pb.ManifestUnit, *structureInstallation, error) {
	units := []*pb.ManifestUnit{}
	transition := &structureInstallation{Preview: confirmation.Preview.Intent.Operation, Digest: confirmation.Digest, Branches: []structureInstalledBranch{}}
	main := map[string]*pb.FixedContent{}
	affected := map[string]bool{}
	for _, content := range confirmation.Snapshot.Main {
		main[domain.TextID(content.UnitId)] = content
	}
	for _, change := range confirmation.Migration.Changes {
		for _, id := range change.Sources {
			affected[id] = true
		}
	}
	// 把受影响整份可靠MAIN留为不可变来源版本，范围边只能引用这些实际落库事实。
	sources := map[string]uint64{}
	for _, original := range confirmation.Snapshot.Main {
		id := domain.TextID(original.UnitId)
		if !affected[id] {
			continue
		}
		content := proto.Clone(original).(*pb.FixedContent)
		content.OperationId = fixed.Intent.Operation
		version, err := repository.InsertMergeVersion(ctx, tx, content, "MAIN_CAPTURE", "structure-source:"+id)
		if err != nil {
			return nil, nil, err
		}
		sources[id] = version
	}
	for _, original := range confirmation.Snapshot.Main {
		if affected[domain.TextID(original.UnitId)] {
			continue
		}
		content := proto.Clone(original).(*pb.FixedContent)
		content.OperationId = fixed.Intent.Operation
		content.StructureId = structure
		version, err := repository.InsertMergeVersion(ctx, tx, content, "MAIN_CAPTURE", "structure-stable:"+domain.TextID(content.UnitId))
		if err != nil {
			return nil, nil, err
		}
		installed, err := controlledMainUnit(ctx, tx, fixed.Document, manifest, content.UnitId, version, fixed.Control+1, content)
		if err != nil {
			return nil, nil, err
		}
		units = append(units, installed)
	}
	for _, change := range confirmation.Migration.Changes {
		sourceContent := []*pb.FixedContent{}
		for _, id := range change.Sources {
			sourceContent = append(sourceContent, main[id])
		}
		relation := "SPLIT"
		if change.Type == "JOIN_CHAPTER" {
			relation = "JOINED"
		}
		for _, target := range change.Targets {
			unit, _ := domain.CanonicalID(identities[target.Key].Unit)
			lineage, err := NewEntityID()
			if err != nil {
				return nil, nil, err
			}
			candidate := structureCandidateContext{Document: fixed.Document, Unit: unit, Structure: structure, Epoch: fixed.Control + 1, Lineage: lineage, Manifest: manifest, BaseVersion: sources[change.Sources[0]], Operation: fixed.Intent.Operation, Preview: confirmation.Preview.Intent.Operation, Key: target.Key, Relation: relation, SourceVersions: sources, Sources: sourceContent}
			accepted, err := buildStructureCandidate(s.validator, target.Main, candidate)
			if err != nil {
				return nil, nil, err
			}
			version, err := repository.InsertMergeVersion(ctx, tx, accepted, "STRUCTURE", target.Key)
			if err != nil {
				return nil, nil, err
			}
			installed, err := controlledMainUnit(ctx, tx, fixed.Document, manifest, unit, version, fixed.Control+1, accepted)
			if err != nil {
				return nil, nil, err
			}
			units = append(units, installed)
			branch, err := repository.CreateInitialBranch(ctx, tx, fixed.Document, unit, version)
			if err != nil {
				return nil, nil, err
			}
			candidate.Branch, candidate.Epoch, candidate.BaseVersion = branch, 1, version
			draft, err := buildStructureCandidate(s.validator, target.Draft, candidate)
			if err != nil {
				return nil, nil, err
			}
			room, err := collaborationRoom(ctx, tx, "BRANCH", branch, unit, 1)
			if err != nil {
				return nil, nil, err
			}
			draft.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": "1", "durable_seq": "0"})
			if err != nil {
				return nil, nil, err
			}
			if err = repository.InsertInitialLineage(ctx, tx, fixed.Document, unit, branch, manifest, fixed.Intent.Operation, domain.InitialContent{Version: version, Lineage: lineage, Provenance: draft.ProvenanceJsonUtf8}); err != nil {
				return nil, nil, err
			}
			if err = repository.RestoreStructureBranch(ctx, tx, branch, lineage); err != nil {
				return nil, nil, err
			}
			if err = repository.BindInitialBranch(ctx, tx, fixed.Document, unit, branch); err != nil {
				return nil, nil, err
			}
			for _, resource := range draft.ResourceIds {
				if err = repository.InsertDraftResourceRef(ctx, tx, resource, fixed.Document, domain.TextID(room)+":1", 1); err != nil {
					return nil, nil, err
				}
			}
			branchBaseline, err := domain.BranchBaselineBody(accepted.BodyJsonUtf8)
			if err != nil {
				return nil, nil, err
			}
			baseline, err := codec.Decode(branchBaseline)
			if err != nil {
				return nil, nil, err
			}
			provenance, err := codec.Decode(draft.ProvenanceJsonUtf8)
			if err != nil {
				return nil, nil, err
			}
			transition.Branches = append(transition.Branches, structureInstalledBranch{Key: target.Key, Unit: unit, Branch: branch, Room: room, Epoch: 1, Lineage: lineage, Content: draft, Context: map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(manifest), "base_version_id": domain.TextID(version), "body": baseline, "provenance": provenance}})
			for _, rangeProof := range target.Ranges {
				source, _ := domain.CanonicalID(rangeProof.Source)
				mapping := map[string]any{"schema_version": "full-range-v1", "type": change.Type, "chapter_id": change.Chapter, "operation_id": fixed.Intent.Operation, "source_unit_id": rangeProof.Source, "target_unit_id": domain.TextID(unit), "source_version_id": domain.TextID(sources[rangeProof.Source]), "target_version_id": domain.TextID(version), "source_structure_id": domain.TextID(fixed.Structure), "target_structure_id": domain.TextID(structure), "source_body_hash": main[rangeProof.Source].BodyHash, "target_body_hash": accepted.BodyHash, "source_scope": rangeProof.Scope, "start_boundary": rangeProof.Start, "end_boundary": rangeProof.End, "main_block_ids": rangeProof.MainBlocks, "draft_block_ids": rangeProof.DraftBlocks, "boundary": target.Boundary, "target_kind": target.Kind, "preview_cut_ref": confirmation.Preview.Intent.Operation, "preview_digest": confirmation.Digest}
				mapping["target_key"] = target.Key
				mapping["target_scope"] = rangeProof.TargetScope
				mapping["target_start_boundary"] = rangeProof.TargetStart
				mapping["target_end_boundary"] = rangeProof.TargetEnd
				raw, err := json.Marshal(mapping)
				if err != nil {
					return nil, nil, err
				}
				if err = repository.InsertUnitLineage(ctx, tx, fixed.Intent.Operation, fixed.Document, source, unit, sources[rangeProof.Source], version, raw); err != nil {
					return nil, nil, err
				}
			}
		}
	}
	return units, transition, nil
}

// structure_unit_boundaries.go：保留原实现，按业务合并文件。

func readStructureUnitBoundaries(ctx context.Context, tx *sql.Tx, document uint64, entries []dto.StructureEntry) ([]any, error) {
	result := []any{}
	for _, entry := range entries {
		if entry.ParentNodeId == nil {
			continue
		}
		if entry.UnitId == nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_BOUNDARY_UNKNOWN")
		}
		unit, _ := domain.CanonicalID(*entry.UnitId)
		mappings, err := repository.StructureUnitMappings(ctx, tx, document, unit)
		if err != nil {
			return nil, err
		}
		if len(mappings) != 1 || mappings[0].Schema != "full-range-v1" {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_BOUNDARY_UNKNOWN")
		}
		item := mappings[0]
		var fixed struct {
			Schema        string                            `json:"schema_version"`
			Type          string                            `json:"type"`
			Kind          string                            `json:"target_kind"`
			Source        string                            `json:"source_unit_id"`
			Target        string                            `json:"target_unit_id"`
			SourceVersion string                            `json:"source_version_id"`
			TargetVersion string                            `json:"target_version_id"`
			SourceHash    []byte                            `json:"source_body_hash"`
			TargetHash    []byte                            `json:"target_body_hash"`
			Boundary      domain.StructureMigrationBoundary `json:"boundary"`
		}
		if json.Unmarshal(item.Mapping, &fixed) != nil || fixed.Schema != item.Schema || fixed.Type != "SPLIT_LEVEL2" || fixed.Kind != entry.Kind || fixed.Source != domain.TextID(item.Source) || fixed.Target != *entry.UnitId || fixed.SourceVersion != domain.TextID(item.SourceVersion) || fixed.TargetVersion != domain.TextID(item.TargetVersion) || !bytes.Equal(fixed.SourceHash, item.SourceHash) || !bytes.Equal(fixed.TargetHash, item.TargetHash) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_BOUNDARY_UNKNOWN")
		}
		result = append(result, map[string]any{"unit_id": *entry.UnitId, "heading": fixed.Boundary.Heading, "empty_block_id": fixed.Boundary.EmptyBlock})
	}
	return result, nil
}
