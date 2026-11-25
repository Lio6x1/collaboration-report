# services/document/internal/logic/document/publication_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/220-publication-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`Publications`、`publicationContext`、`NewPublications`、`publicationRequest`、`publicationInput`、`Lookup`、`publicationLockSources`、`validatePublicationContext`、`resume`、`validatePublicationProofs`、`validatePublicationAdmission`、`publicationCaptureError`、`commit`、`applyPublication`、`Prepare`、`Ready`、`validatePendingPublicationBinding`、`presentation`、`Progress`、`readResult`、`RecoverOne`、`recoverPublication`、`checkCurrent`、`reject`、`publicationRecoveryCode`、`OperationResult`、`validateCurrent`、`MainSource`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// publication.go：保留原实现，按业务合并文件。

type Publications struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    MergeClient
	validator *codec.Validator
	instance  string
}

type publicationContext struct {
	Actor          uint64                   `json:"actor"`
	Document       uint64                   `json:"document"`
	SessionExpires int64                    `json:"session_expires"`
	Intent         domain.PublicationIntent `json:"intent"`
	Access         DocumentAccess           `json:"access"`
	Main           reliableMainBinding      `json:"main"`
	Path           string                   `json:"path"`
}

func NewPublications(db *sql.DB, identity *Identity, access *Access, client MergeClient, validator *codec.Validator, instance string) *Publications {
	return &Publications{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}

func publicationRequest(fixed publicationContext) *pb.CaptureMainRequest {
	main := fixed.Main
	return &pb.CaptureMainRequest{Context: main.Context, Usage: "RELEASE_CREATE:1", ExpectedManifestId: main.Manifest,
		ExpectedStructureId: main.Structure, ExpectedStyleId: main.Style, ExpectedControlRevision: main.Control}
}

func publicationInput(intent domain.PublicationIntent) any {
	if intent.Kind == "RELEASE_CREATE" {
		return dto.ReleaseCreate{OperationId: intent.Operation, ExpectedPublicationRevision: domain.TextID(intent.Publication), ExpectedControlRevision: domain.TextID(intent.Control)}
	}
	input := dto.Publication{OperationId: intent.Operation, ExpectedPublicationRevision: domain.TextID(intent.Publication), Action: intent.Kind}
	if intent.Kind == "SWITCH" {
		release := domain.TextID(intent.Release)
		input.ReleaseId = &release
	}
	return input
}

func (s *Publications) Lookup(ctx context.Context, operation string) (publicationContext, error) {
	var fixed publicationContext
	raw, err := repository.MergeArtifact(ctx, s.db, "PUBLICATION_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Intent.Operation != operation {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	}
	return fixed, validatePublicationContext(fixed)
}

func publicationLockSources(access DocumentAccess) []*pb.SourceEpoch {
	sources := append([]*pb.SourceEpoch(nil), access.Sources...)
	revision, _ := domain.CanonicalRevision(access.Document.PublicationEpoch)
	if revision != 0 {
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "publication:" + access.Document.Id, Epoch: revision})
	}
	return sources
}

// publication_binding.go：保留原实现，按业务合并文件。

func validatePublicationContext(fixed publicationContext) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	main, ctx := fixed.Main, fixed.Main.Context
	if epoch := reliableMainEpoch(main); epoch == 0 || epoch > main.Control {
		return invalid
	}
	if ctx == nil || ctx.Target == nil || fixed.Actor == 0 || fixed.Document == 0 ||
		fixed.SessionExpires <= 0 || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil ||
		!domain.ValidOperationID(fixed.Intent.Operation) || !fixed.Access.Permissions.Manage ||
		ctx.OperationId != fixed.Intent.Operation || ctx.ActorId != fixed.Actor || ctx.DocumentAdmissionRef == "" {
		return invalid
	}
	if domain.ValidatePublication(fixed.Access.Permissions, fixed.Access.Document, fixed.Intent) != nil {
		return invalid
	}
	document := fixed.Access.Document
	target := &pb.Target{DocumentId: fixed.Document, StructureId: main.Structure, WorkspaceEpoch: main.Control, Purpose: "PUBLICATION"}
	if !proto.Equal(ctx.Target, target) || document.Id != domain.TextID(fixed.Document) ||
		main.Manifest == 0 || main.Structure == 0 || main.Style == 0 ||
		document.CurrentManifestId != domain.TextID(main.Manifest) || document.CurrentStructureId != domain.TextID(main.Structure) ||
		document.CurrentStyleId != domain.TextID(main.Style) || document.ControlRevision != domain.TextID(main.Control) {
		return invalid
	}
	first := fixed.Access.Proofs[0]
	if first.AdmissionId != ctx.IdentityAdmissionRef || len(first.SessionRef) != 32 {
		return invalid
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.IdentityTarget == nil || proof.ActorId != fixed.Actor || proof.OperationId != ctx.OperationId ||
			proof.Purpose != "PUBLICATION" || !proto.Equal(proof.Target, target) || !bytes.Equal(proof.SessionRef, first.SessionRef) ||
			proof.ExpiresUnixMs <= 0 || proof.ExpiresUnixMs > fixed.SessionExpires {
			return invalid
		}
	}
	sources, err := MergeSources(fixed.Access.Proofs...)
	if err != nil {
		return invalid
	}
	acl, err := domain.CanonicalID(document.AclRevision)
	if err != nil {
		return invalid
	}
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + document.Id, Epoch: acl})
	if document.KnowledgeBaseId != nil {
		if _, err := domain.CanonicalID(*document.KnowledgeBaseId); err != nil || fixed.Access.KBRevision == 0 {
			return invalid
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *document.KnowledgeBaseId, Epoch: fixed.Access.KBRevision})
	}
	if !equalPullSources(sources, fixed.Access.Sources) || !equalPullSources(ctx.Sources, sources) {
		return invalid
	}
	path := "/v1/documents/" + document.Id + "/publication"
	if fixed.Intent.Kind == "RELEASE_CREATE" {
		path = "/v1/documents/" + document.Id + "/releases"
	}
	fingerprint, err := domain.OperationFingerprint(path, publicationInput(fixed.Intent))
	if err != nil || fixed.Path != path || !bytes.Equal(fingerprint[:], ctx.RequestFingerprint) {
		return invalid
	}
	if len(main.Units) != len(main.Versions) {
		return invalid
	}
	seen := make(map[uint64]bool, len(main.Units))
	for _, unit := range main.Units {
		if unit == 0 || seen[unit] || main.Versions[unit] == 0 {
			return invalid
		}
		seen[unit] = true
	}
	if fixed.Intent.Kind == "RELEASE_CREATE" && len(main.Units) == 0 {
		return invalid
	}
	return nil
}

func (s *Publications) resume(ctx context.Context, fixed publicationContext) ([]*pb.IdentityAdmission, error) {
	if err := validatePublicationContext(fixed); err != nil {
		return nil, err
	}
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return nil, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	proofs := make([]*pb.IdentityAdmission, 0, len(fixed.Access.Proofs))
	for _, original := range fixed.Access.Proofs {
		proof, err := s.identity.Resume(ctx, original)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proof)
	}
	return proofs, nil
}

func validatePublicationProofs(fixed publicationContext, proofs []*pb.IdentityAdmission) error {
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	if len(proofs) != len(fixed.Access.Proofs) {
		return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_PROOF_REFRESH_REQUIRED")
	}
	for i, proof := range proofs {
		old := fixed.Access.Proofs[i]
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() || proof.ActorId != old.ActorId ||
			proof.Purpose != old.Purpose || proof.OperationId != old.OperationId || !proto.Equal(proof.Target, old.Target) ||
			!proto.Equal(proof.IdentityTarget, old.IdentityTarget) || !bytes.Equal(proof.SessionRef, old.SessionRef) ||
			!equalPullSources(proof.Sources, old.Sources) {
			return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_PROOF_REFRESH_REQUIRED")
		}
	}
	return nil
}

func validatePublicationAdmission(ctx context.Context, tx *sql.Tx, fixed publicationContext) error {
	actual, err := repository.OriginalAdmission(ctx, tx, fixed.Main.Context.DocumentAdmissionRef, true)
	if err != nil {
		return err
	}
	target := &pb.Target{}
	if actual.Actor != fixed.Actor || actual.Document != fixed.Document || actual.Unit != 0 ||
		actual.Identity != fixed.Main.Context.IdentityAdmissionRef || actual.Operation != fixed.Intent.Operation ||
		actual.Purpose != "PUBLICATION" || actual.SessionExpires != fixed.SessionExpires ||
		!bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) || json.Unmarshal(actual.Target, target) != nil ||
		!proto.Equal(target, fixed.Main.Context.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Main.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	if state.State != "OPEN" || !state.Expires.After(time.Now()) {
		return domain.Violation(domain.PermissionDenied, "PUBLICATION_ADMISSION_CLOSED")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Main.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	if !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	}
	return nil
}

// publication_capture_errors.go：保留原实现，按业务合并文件。

func publicationCaptureError(err error) error {
	if err == nil {
		return nil
	}
	rpc, encoded := status.FromError(err)
	if !encoded || rpc.Code() != codes.ResourceExhausted {
		return err
	}
	// 仅信任Node已执行的固定容量规则；通用传输大小、配额或依赖耗尽没有
	// 足够事实确认业务失败，仍沿原号恢复。未知远端文本不进入公开错误。
	switch rpc.Message() {
	case "MAIN_CONTENT_TOO_LARGE":
		return domain.Violation(domain.StateConflict, "PUBLICATION_CONTENT_TOO_LARGE")
	case "MAIN_VIEW_TRANSFER_LIMIT":
		return domain.Violation(domain.StateConflict, "PUBLICATION_TRANSFER_LIMIT")
	default:
		return err
	}
}

// publication_commit.go：保留原实现，按业务合并文件。

// commit把已捕获的固定主线发布为正式版本；它不读取正在变化的编辑草稿。
// RPC捕获已完成后才开启最终事务，事务内重检权威资格、切点和任务归属。
func (s *Publications) commit(ctx context.Context, task domain.SubmissionTask, fixed publicationContext, main *pb.MainCaptureReply, proofs []*pb.IdentityAdmission) error {
	if err := validateReliableMainCapture(s.validator, fixed.Main, main, "RELEASE_CREATE:1", "PUBLICATION"); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 固定结果的RPC已完成；最终锁序为operation→完整来源→Doc/KB→MAIN屏障→原任务。
	// 只有当前原管理资格和同一切点全部成立，才能一次落定版本、指针与事件。
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	current, err := s.validateCurrent(ctx, tx, fixed, proofs)
	if err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	resources := make(map[uint64]bool)
	for _, content := range main.Units {
		if err = validateReliableMainVersion(ctx, tx, fixed.Main, content, "PUBLICATION"); err != nil {
			return err
		}
		for _, resource := range content.ResourceIds {
			resources[resource] = true
		}
	}
	// 合并各单元引用并按数值升序锁资源，避免两篇文档以相反顺序持锁形成死锁。
	ordered := make([]uint64, 0, len(resources))
	for resource := range resources {
		ordered = append(ordered, resource)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, resource := range ordered {
		state, err := repository.LockedDocumentResource(ctx, tx, resource, fixed.Document, space)
		if err != nil {
			return err
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "RESOURCE_NOT_READY")
		}
	}
	manifest, err := repository.InsertControlledManifest(ctx, tx, fixed.Document, fixed.Main.Manifest, fixed.Main.Structure, fixed.Main.Style, task.Operation, "RELEASE", task.Operation+":RELEASE_CREATE:1")
	if err != nil {
		return err
	}
	for _, content := range main.Units {
		version, err := repository.InsertMergeVersion(ctx, tx, content, "MAIN_CAPTURE", content.CaptureId)
		if err != nil {
			return err
		}
		if err = repository.InsertInitialManifestEntry(ctx, tx, fixed.Document, manifest, content.UnitId, version); err != nil {
			return err
		}
	}
	release, err := repository.InsertRelease(ctx, tx, fixed.Document, manifest, fixed.Actor, current.Title, task.Operation)
	if err != nil {
		return err
	}
	result, err := s.applyPublication(ctx, tx, fixed, release)
	if err != nil {
		return err
	}
	// 正式版/发布指针/outbox事件/原号结果与任务终态一起提交，不能拆成独立DAO提交。
	// tx.Commit若返回未知，恢复器仍读取这个operation，不新建一份正式版冒充重试。
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, result["state"].(string), raw); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Publications) applyPublication(ctx context.Context, tx *sql.Tx, fixed publicationContext, release uint64) (map[string]any, error) {
	sequence, err := repository.PublicationEventSequence(ctx, tx, fixed.Document)
	if err != nil {
		return nil, err
	}
	epoch, sequence, err := domain.NextPublicationSequences(fixed.Intent.Publication, sequence)
	if err != nil {
		return nil, err
	}
	eventType := "DOCUMENT_UNPUBLISHED"
	if release != 0 {
		eventType = "DOCUMENT_PUBLISHED"
	}
	// 事件仅承载固定业务水位和正式引用；正文、身份票据及内部草稿不进入消息总线。
	payload, err := domain.EncodeIndexEvent(domain.IndexEvent{
		ID: fixed.Intent.Operation, Type: eventType, Document: fixed.Document,
		Sequence: sequence, Release: release, PublicationEpoch: epoch, OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	if err = repository.SetPublication(ctx, tx, fixed.Document, release, epoch, sequence, fixed.Intent.Operation, eventType, payload); err != nil {
		return nil, err
	}
	state := "SUCCEEDED"
	if fixed.Intent.Publication != 0 {
		// 已提交业务不等于旧公开输出已结束；关闭由既有系统worker沿原号完成，用户无需在线。
		closure := domain.Closure{Scope: "publication:" + domain.TextID(fixed.Document), Epoch: fixed.Intent.Publication}
		if err = QueueClosure(ctx, tx, fixed.Intent.Operation, fixed.Actor, closure); err != nil {
			return nil, err
		}
		state = "PREPARING"
	}
	result := map[string]any{"operation_id": fixed.Intent.Operation, "state": state}
	if release != 0 {
		result["result"] = map[string]string{"entity_id": domain.TextID(release)}
	}
	if state == "PREPARING" {
		result["poll_url"] = "/v1/operations/" + fixed.Intent.Operation
	}
	return result, nil
}

// publication_prepare.go：保留原实现，按业务合并文件。

func (s *Publications) Prepare(ctx context.Context, actor Actor, document uint64, intent domain.PublicationIntent, input any, path string) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.access.Authorize(ctx, actor, document, "PUBLICATION")
	if err != nil {
		return actor, nil, nil, err
	}
	if !access.Permissions.Manage {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "PUBLICATION_NOT_ALLOWED")
	}
	actor.Sources = publicationLockSources(access)
	present := s.presentation(actor, access, intent.Operation)
	if _, err = repository.PublicationResult(ctx, s.db, intent.Operation); err == nil {
		// 先由通用receipt核验原actor/kind/输入指纹，再投影原结果，不重新要求旧期望匹配新水位。
		actor.Sources = access.Sources
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, present, nil
	} else if err != sql.ErrNoRows {
		return actor, nil, nil, err
	}
	structure, _ := domain.CanonicalID(access.Document.CurrentStructureId)
	control, _ := domain.CanonicalRevision(access.Document.ControlRevision)
	target := &pb.Target{DocumentId: document, StructureId: structure, WorkspaceEpoch: control, Purpose: "PUBLICATION"}
	proofs := access.Proofs
	access.Proofs, err = s.identity.BindProofs(ctx, actor, proofs, intent.Operation, target)
	if err != nil {
		return actor, nil, nil, err
	}
	actor.Proof = access.Proofs[0]
	fingerprint, err := domain.OperationFingerprint(path, input)
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidatePublication(access.Permissions, current, intent); err != nil {
			return nil, err
		}
		barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
		if err != nil {
			return nil, err
		}
		if intent.Kind == "RELEASE_CREATE" && barrier.Phase != "OPEN" {
			return nil, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
		}
		admission := platform.NewTraceID()
		if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "PUBLICATION", admission); err != nil {
			return nil, err
		}
		rawTarget, err := json.Marshal(target)
		if err != nil {
			return nil, err
		}
		if err = repository.ConfigureMainOperationAdmission(ctx, tx, admission, intent.Operation, rawTarget, actor.SessionExpiresUnixMs); err != nil {
			return nil, err
		}
		access.Document = current
		manifest, _ := domain.CanonicalID(current.CurrentManifestId)
		style, _ := domain.CanonicalID(current.CurrentStyleId)
		fixed := publicationContext{Actor: actor.ID, Document: document, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Path: path,
			Main: reliableMainBinding{Manifest: manifest, Structure: structure, Style: style, Control: control,
				Context: &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, IdentityAdmissionRef: actor.Proof.AdmissionId,
					DocumentAdmissionRef: admission, Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:]}}}
		if intent.Kind == "RELEASE_CREATE" {
			fixed.Main.Epoch, err = repository.MainWorkspaceEpoch(ctx, tx, document)
			if err != nil {
				return nil, err
			}
			entries, err := repository.ManifestUnits(ctx, tx, current.CurrentManifestId, document)
			if err != nil {
				return nil, err
			}
			structureRow, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
			if err != nil {
				return nil, err
			}
			if err = validateReadingUnits(structureRow, entries); err != nil {
				return nil, err
			}
			fixed.Main.Versions = make(map[uint64]uint64, len(entries))
			for _, entry := range entries {
				unit, _ := domain.CanonicalID(entry["unit_id"])
				version, _ := domain.CanonicalID(entry["content_version_id"])
				fixed.Main.Units = append(fixed.Main.Units, unit)
				fixed.Main.Versions[unit] = version
			}
		}
		if err = validatePublicationContext(fixed); err != nil {
			return nil, err
		}
		original, err := json.Marshal(fixed)
		if err != nil {
			return nil, err
		}
		if err = repository.RegisterPublication(ctx, tx, intent.Operation, original, intent.Kind == "RELEASE_CREATE"); err != nil {
			return nil, err
		}
		if intent.Kind == "RELEASE_CREATE" {
			return map[string]any{"operation_id": intent.Operation, "state": "PREPARING", "poll_url": "/v1/operations/" + intent.Operation}, nil
		}
		if intent.Kind == "SWITCH" {
			if _, err = repository.ReleaseManifest(ctx, tx, document, intent.Release); err != nil {
				return nil, err
			}
		}
		return s.applyPublication(ctx, tx, fixed, intent.Release)
	}
	return actor, run, present, nil
}

// publication_readiness.go：保留原实现，按业务合并文件。

func (s *Publications) Ready(ctx context.Context) error {
	facts, err := repository.PendingPublicationBindings(ctx, s.db)
	if err != nil {
		return errors.New("PUBLICATION_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		if err := validatePendingPublicationBinding(fact); err != nil {
			return err
		}
	}
	return nil
}

func validatePendingPublicationBinding(fact repository.PendingPublicationBinding) error {
	var original, payload publicationContext
	var target struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(fact.Original, &original) != nil || json.Unmarshal(fact.Payload, &payload) != nil ||
		json.Unmarshal(fact.Target, &target) != nil || validatePublicationContext(original) != nil ||
		fact.Kind != "RELEASE_CREATE" || fact.State != "PREPARING" || fact.Actor != original.Actor ||
		fact.Operation != original.Intent.Operation || original.Intent.Kind != fact.Kind || target.Path != original.Path ||
		!bytes.Equal(fact.Fingerprint, original.Main.Context.RequestFingerprint) {
		return errors.New("PUBLICATION_BINDING_UNKNOWN")
	}
	left, err := json.Marshal(original)
	if err != nil {
		return errors.New("PUBLICATION_BINDING_UNKNOWN")
	}
	right, err := json.Marshal(payload)
	if err != nil || !bytes.Equal(left, right) {
		return errors.New("PUBLICATION_RECOVERY_UNKNOWN")
	}
	return nil
}

// publication_readings.go：保留原实现，按业务合并文件。

func (s *Publications) presentation(actor Actor, access DocumentAccess, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		value, err := s.readResult(ctx, tx, actor, access, operation)
		return value, access.Sources, err
	}
}

func (s *Publications) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.Lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if fixed.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "PUBLICATION_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fixed.Document, "PUBLICATION_RESULT")
	if err != nil {
		return nil, nil, err
	}
	if !access.Permissions.Manage {
		return nil, nil, domain.Violation(domain.PermissionDenied, "PUBLICATION_NOT_ALLOWED")
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		return s.readResult(ctx, tx, actor, access, operation)
	}, nil
}

func (s *Publications) readResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (any, error) {
	if !access.Permissions.Manage {
		return nil, domain.Violation(domain.PermissionDenied, "PUBLICATION_NOT_ALLOWED")
	}
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return nil, err
	}
	receipt, err := repository.PublicationResult(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if receipt.Actor != actor.ID {
		return nil, domain.Violation(domain.EntityNotFound, "PUBLICATION_NOT_FOUND")
	}
	if !receipt.Expires.After(time.Now()) {
		return nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	var value map[string]any
	if json.Unmarshal(receipt.Result, &value) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_RESULT_UNKNOWN")
	}
	return value, nil
}

// publication_recovery.go：保留原实现，按业务合并文件。

func (s *Publications) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimPublicationTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	var fixed publicationContext
	if json.Unmarshal(task.Payload, &fixed) != nil || validatePublicationContext(fixed) != nil {
		err = domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	} else {
		// 领取载荷还必须与独立原上下文相同；不能靠两份可变进度字段一致就继续业务。
		original, lookupErr := s.Lookup(ctx, task.Operation)
		left, leftErr := json.Marshal(original)
		right, rightErr := json.Marshal(fixed)
		if lookupErr != nil || leftErr != nil || rightErr != nil || string(left) != string(right) {
			err = domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
		} else {
			err = s.recoverPublication(ctx, task, fixed)
		}
		var violation domain.RuleViolation
		if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound || violation.Kind == domain.StateConflict) {
			err = s.reject(ctx, task, violation.Code)
		}
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, publicationRecoveryCode(err))
	}
}

func (s *Publications) recoverPublication(ctx context.Context, task domain.SubmissionTask, fixed publicationContext) error {
	proofs, err := s.resume(ctx, fixed)
	if err != nil {
		return err
	}
	// 不可达是未知结果，权威标题/控制变化却是明确冲突。先在短事务核验原来源，
	// 避免已经无效的预约因Node持续不可达而无限等待；事务提交后才发捕获RPC。
	if err = s.checkCurrent(ctx, fixed, proofs); err != nil {
		return err
	}
	main, err := s.client.CaptureMain(ctx, publicationRequest(fixed))
	if err != nil {
		// 私有来源查询到Node之间仍可能变化；用Doc真实当前事实判定失效，
		// 不把一个远端错误码或deadline猜成最终业务拒绝。
		if currentErr := s.checkCurrent(ctx, fixed, proofs); currentErr != nil {
			var violation domain.RuleViolation
			if errors.As(currentErr, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound || violation.Kind == domain.StateConflict) {
				return currentErr
			}
		}
		return publicationCaptureError(err)
	}
	proofs, err = s.resume(ctx, fixed)
	if err != nil {
		return err
	}
	return s.commit(ctx, task, fixed, main, proofs)
}

func (s *Publications) checkCurrent(ctx context.Context, fixed publicationContext, proofs []*pb.IdentityAdmission) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = s.validateCurrent(ctx, tx, fixed, proofs); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Publications) reject(ctx context.Context, task domain.SubmissionTask, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED",
		"error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	if err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", result); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "REJECTED"); err != nil {
		return err
	}
	return tx.Commit()
}

func publicationRecoveryCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		return fmt.Sprintf("PUBLICATION_SQL_%d", database.Number)
	}
	if rpc, ok := status.FromError(err); ok {
		return "PUBLICATION_RPC_" + rpc.Code().String()
	}
	return "PUBLICATION_RECOVERY_UNKNOWN"
}

// publication_results.go：保留原实现，按业务合并文件。

// OperationResult供已认证的内部参与者核验原系统事实；业务成功后的关闭/清理
// 不再要求用户在线。浏览器进度仍走Progress，必须复核当前身份及管理权限。
func (s *Publications) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_QUERY_INVALID")
	}
	fixed, err := s.Lookup(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Main.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Main.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.PublicationResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	if fact.Actor != fixed.Actor || fact.Kind != fixed.Intent.Kind || fact.Generation == 0 || !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, unknown
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: fact.Generation, State: pb.OperationResult_PROCESSING}
	if fact.State == "PREPARING" {
		return result, nil
	}
	if fact.State != "SUCCEEDED" && fact.State != "REJECTED" {
		return nil, unknown
	}
	var value struct {
		Operation string `json:"operation_id"`
		State     string `json:"state"`
		Result    struct {
			Entity string `json:"entity_id"`
		} `json:"result"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(fact.Result, &value) != nil || value.Operation != query.OperationId || value.State != fact.State {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_RESULT_UNKNOWN")
	}
	result.State = pb.OperationResult_REJECTED
	if fact.State == "SUCCEEDED" {
		result.State = pb.OperationResult_SUCCEEDED
		if value.Result.Entity != "" {
			id, err := domain.CanonicalID(value.Result.Entity)
			if err != nil {
				return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_RESULT_UNKNOWN")
			}
			result.ResultIds = []uint64{id}
		}
	}
	// 摘要只来自原SQL固定字节，不用当前HTTP投影、当前正式指针或一次重算结果替换。
	digest := sha256.Sum256(fact.Result)
	result.FixedResultJsonUtf8, result.ResultDigest, result.TerminalRevision = fact.Result, digest[:], fact.Generation
	result.BusinessCode = value.Error.Code
	return result, nil
}

// publication_source.go：保留原实现，按业务合并文件。

func (s *Publications) validateCurrent(ctx context.Context, tx *sql.Tx, fixed publicationContext, proofs []*pb.IdentityAdmission) (dto.Document, error) {
	var current dto.Document
	if err := validatePublicationContext(fixed); err != nil {
		return current, err
	}
	if err := LockSources(ctx, tx, publicationLockSources(fixed.Access)); err != nil {
		return current, err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return current, err
	}
	if err = domain.ValidatePublication(fixed.Access.Permissions, current, fixed.Intent); err != nil {
		return current, err
	}
	if current.Title != fixed.Access.Document.Title || current.CurrentManifestId != domain.TextID(fixed.Main.Manifest) ||
		current.CurrentStructureId != domain.TextID(fixed.Main.Structure) || current.CurrentStyleId != domain.TextID(fixed.Main.Style) ||
		current.ControlRevision != domain.TextID(fixed.Main.Control) {
		return current, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return current, err
	}
	if barrier.Phase != "OPEN" {
		return current, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	epoch, err := repository.MainWorkspaceEpoch(ctx, tx, fixed.Document)
	if err != nil {
		return current, err
	}
	if epoch != reliableMainEpoch(fixed.Main) {
		return current, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	if err = validatePublicationAdmission(ctx, tx, fixed); err != nil {
		return current, err
	}
	return current, validatePublicationProofs(fixed, proofs)
}

func (s *Publications) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error) {
	if input == nil || input.Context == nil || input.Context.Target == nil {
		return nil, domain.Violation(domain.InvalidRequest, "MAIN_SOURCE_QUERY_INVALID")
	}
	fixed, err := s.Lookup(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if fixed.Intent.Kind != "RELEASE_CREATE" || !proto.Equal(publicationRequest(fixed), input) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	proofs, err := s.resume(ctx, fixed)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	current, err := s.validateCurrent(ctx, tx, fixed, proofs)
	if err != nil {
		return nil, err
	}
	units, epoch, err := mainSourceUnits(ctx, tx, fixed.Document, current.CurrentManifestId, fixed.Main.Control)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"manifest_id": current.CurrentManifestId, "structure_id": current.CurrentStructureId, "style_id": current.CurrentStyleId, "control_revision": current.ControlRevision, "workspace_epoch": domain.TextID(epoch), "units": units}, nil
}
````

UTF-8/LF全文SHA-256：`30d1b1358314a90069c2727047ef5f7d2ede442ccdeddc1819391623a98c0ca5`。[参考实现](../../../../services/document/internal/logic/document/publication_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
