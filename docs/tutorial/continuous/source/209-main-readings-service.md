# services/document/internal/logic/document/main_readings_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/209-main-readings-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`MainReadViews`、`MainReadClient`、`mainReadContext`、`mainReadCapture`、`NewMainReadViews`、`fixedContext`、`mainReadRequest`、`Capture`、`revalidate`、`validateCapture`、`Publish`、`MainSource`、`validateCurrent`、`validateMainReadAdmission`、`validateMainReadContext`、`mainSourceUnits`、`reliableMainBinding`、`mainReadBinding`、`reliableMainEpoch`、`validateReliableMainCapture`、`validateReliableMainContentBinding`、`validateReliableMainVersion`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

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
	"database/sql"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// main_read.go：保留原实现，按业务合并文件。

type MainReadViews struct {
	db        *sql.DB
	identity  *Identity
	client    MainReadClient
	validator *codec.Validator
}

type MainReadClient interface {
	CaptureMain(context.Context, *pb.CaptureMainRequest) (*pb.MainCaptureReply, error)
	MainRevision(context.Context, uint64) (string, error)
}

// 固定内容来自真实只读准入与完整MAIN来源，和管理控制上下文分开。
// 私有捕获可以追加阅读快照，却不能据此锁定主线、推进control或批准任何轮次。
type mainReadContext struct {
	Actor          uint64               `json:"actor"`
	SessionExpires int64                `json:"session_expires"`
	Access         DocumentAccess       `json:"access"`
	Context        *pb.OperationContext `json:"context"`
	Manifest       uint64               `json:"manifest"`
	Structure      uint64               `json:"structure"`
	Style          uint64               `json:"style"`
	Control        uint64               `json:"control"`
	MainEpoch      uint64               `json:"main_epoch,omitempty"`
	ExpiresUnixMs  int64                `json:"expires_unix_ms"`
	Units          []uint64             `json:"units"`
	Versions       map[uint64]uint64    `json:"versions"`
}

type mainReadCapture struct {
	Fixed mainReadContext
	Main  *pb.MainCaptureReply
}

func NewMainReadViews(db *sql.DB, identity *Identity, client MainReadClient, validator *codec.Validator) *MainReadViews {
	return &MainReadViews{db: db, identity: identity, client: client, validator: validator}
}

func (s *MainReadViews) fixedContext(ctx context.Context, operation string) (mainReadContext, error) {
	var fixed mainReadContext
	raw, err := repository.MainReadContext(ctx, s.db, operation)
	if err != nil {
		return fixed, err
	}
	err = json.Unmarshal(raw, &fixed)
	if err != nil {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_BINDING_UNKNOWN")
	}
	return fixed, validateMainReadContext(fixed)
}

// main_read_capture.go：保留原实现，按业务合并文件。

func mainReadRequest(fixed mainReadContext) *pb.CaptureMainRequest {
	return &pb.CaptureMainRequest{
		Context: fixed.Context, Usage: "MAIN_READ_VIEW:1",
		ExpectedManifestId: fixed.Manifest, ExpectedStructureId: fixed.Structure,
		ExpectedStyleId: fixed.Style, ExpectedControlRevision: fixed.Control,
	}
}

// 登记、跨服务固定和最终发布是三个边界；任何RPC都不能占用Doc的来源/文档SQL锁。
// 只保存actor编号和原证明，不保存可再次登录的会话Token，也不为普通读取续期身份。
func (s *MainReadViews) Capture(ctx context.Context, actor Actor, access DocumentAccess, document uint64) (*mainReadCapture, error) {
	fixed := mainReadContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Access: access, ExpiresUnixMs: actor.Proof.ExpiresUnixMs}
	for _, proof := range access.Proofs {
		if proof == nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_IDENTITY_UNKNOWN")
		}
		if proof.ExpiresUnixMs < fixed.ExpiresUnixMs {
			fixed.ExpiresUnixMs = proof.ExpiresUnixMs
		}
	}
	if fixed.SessionExpires < fixed.ExpiresUnixMs {
		fixed.ExpiresUnixMs = fixed.SessionExpires
	}
	if err := s.revalidate(ctx, fixed); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return nil, err
	}
	current, err := ValidateAccess(ctx, tx, access)
	if err != nil {
		return nil, err
	}
	if current.MainLocked || current.Lifecycle != "ACTIVE" {
		return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
	if err != nil {
		return nil, err
	}
	if barrier.Phase != "OPEN" {
		return nil, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	fixed.Access.Document = current
	fixed.Manifest, _ = domain.CanonicalID(current.CurrentManifestId)
	fixed.Structure, _ = domain.CanonicalID(current.CurrentStructureId)
	fixed.Style, _ = domain.CanonicalID(current.CurrentStyleId)
	fixed.Control, _ = domain.CanonicalRevision(current.ControlRevision)
	fixed.MainEpoch, err = repository.MainWorkspaceEpoch(ctx, tx, document)
	if err != nil {
		return nil, err
	}
	units, err := repository.ManifestUnits(ctx, tx, current.CurrentManifestId, document)
	if err != nil {
		return nil, err
	}
	structure, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return nil, err
	}
	if err = validateReadingUnits(structure, units); err != nil {
		return nil, err
	}
	fixed.Units = make([]uint64, 0, len(units))
	fixed.Versions = make(map[uint64]uint64, len(units))
	for _, unit := range units {
		id, _ := domain.CanonicalID(unit["unit_id"])
		version, _ := domain.CanonicalID(unit["content_version_id"])
		fixed.Units = append(fixed.Units, id)
		fixed.Versions[id] = version
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "READ_MAIN", admission); err != nil {
		return nil, err
	}
	target := &pb.Target{DocumentId: document, StructureId: fixed.Structure, WorkspaceEpoch: fixed.Control, Purpose: "READ_MAIN"}
	rawTarget, err := json.Marshal(target)
	if err != nil {
		return nil, err
	}
	if err = repository.BindMainReadAdmission(ctx, tx, admission, actor.Proof.OperationId, rawTarget); err != nil {
		return nil, err
	}
	fingerprint, err := domain.OperationFingerprint("/v1/documents/"+current.Id+"/main", map[string]any{"manifest_id": current.CurrentManifestId, "structure_id": current.CurrentStructureId, "style_id": current.CurrentStyleId, "control_revision": current.ControlRevision})
	if err != nil {
		return nil, err
	}
	fixed.Context = &pb.OperationContext{
		OperationId: actor.Proof.OperationId, ActorId: actor.ID,
		IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission,
		Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:],
	}
	raw, err := json.Marshal(fixed)
	if err != nil {
		return nil, err
	}
	if err = repository.InsertMainReadContext(ctx, tx, fixed.Context.OperationId, raw); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	main, err := s.client.CaptureMain(ctx, mainReadRequest(fixed))
	if err != nil {
		return nil, err
	}
	if err = s.validateCapture(fixed, main); err != nil {
		return nil, err
	}
	if err = s.revalidate(ctx, fixed); err != nil {
		return nil, err
	}
	return &mainReadCapture{Fixed: fixed, Main: main}, nil
}

func (s *MainReadViews) revalidate(ctx context.Context, fixed mainReadContext) error {
	if len(fixed.Access.Proofs) == 0 || fixed.ExpiresUnixMs <= time.Now().UnixMilli() || fixed.SessionExpires <= time.Now().UnixMilli() {
		return domain.Violation(domain.PermissionDenied, "MAIN_READ_EXPIRED")
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			return domain.Violation(domain.PermissionDenied, "MAIN_READ_EXPIRED")
		}
		if err := s.identity.Revalidate(ctx, proof); err != nil {
			return err
		}
	}
	return nil
}

func (s *MainReadViews) validateCapture(fixed mainReadContext, main *pb.MainCaptureReply) error {
	if err := validateMainReadContext(fixed); err != nil {
		return err
	}
	return validateReliableMainCapture(s.validator, mainReadBinding(fixed), main, "MAIN_READ_VIEW:1", "MAIN_READ")
}

// main_read_commit.go：保留原实现，按业务合并文件。

// 快照物化与输出登记由调用方同一事务提交；此处不安装MAIN、不改变任何业务指针。
// 每次阅读版本保留实际捕获来源和VERSION资源引用，不能给可靠尾部套用旧版本编号。
func (s *MainReadViews) Publish(ctx context.Context, tx *sql.Tx, actor Actor, capture *mainReadCapture, admission string) (dto.MainView, error) {
	var view dto.MainView
	fixed := capture.Fixed
	if actor.ID != fixed.Actor || actor.Proof.AdmissionId != fixed.Context.IdentityAdmissionRef || actor.Proof.OperationId != fixed.Context.OperationId {
		return view, domain.Violation(domain.PermissionDenied, "MAIN_READ_ACTOR_CHANGED")
	}
	current, err := s.validateCurrent(ctx, tx, fixed)
	if err != nil {
		return view, err
	}
	document := fixed.Context.Target.DocumentId
	space, _ := domain.CanonicalID(current.SpaceId)
	resources := make(map[uint64]bool)
	for _, content := range capture.Main.Units {
		if err = validateReliableMainVersion(ctx, tx, mainReadBinding(fixed), content, "MAIN_READ"); err != nil {
			return view, err
		}
		for _, resource := range content.ResourceIds {
			resources[resource] = true
		}
	}
	// 按数值顺序锁资源，避免多单元重复引用造成锁顺序翻转。
	ordered := make([]uint64, 0, len(resources))
	for resource := range resources {
		ordered = append(ordered, resource)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, resource := range ordered {
		state, err := repository.LockedDocumentResource(ctx, tx, resource, document, space)
		if err != nil {
			return view, err
		}
		if state != "READY" {
			return view, domain.Violation(domain.StateConflict, "RESOURCE_NOT_READY")
		}
	}
	manifest, err := repository.InsertControlledManifest(ctx, tx, document, fixed.Manifest, fixed.Structure, fixed.Style, fixed.Context.OperationId, "READ_VIEW", fixed.Context.OperationId+":MAIN_READ_VIEW:1")
	if err != nil {
		return view, err
	}
	for _, content := range capture.Main.Units {
		version, err := repository.InsertReadingVersion(ctx, tx, content)
		if err != nil {
			return view, err
		}
		if err = repository.InsertInitialManifestEntry(ctx, tx, document, manifest, content.UnitId, version); err != nil {
			return view, err
		}
	}
	return readingMainView(ctx, tx, current, document, domain.TextID(manifest), admission)
}

// main_read_source.go：保留原实现，按业务合并文件。

func (s *MainReadViews) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error) {
	if input == nil || input.Context == nil || input.Context.Target == nil {
		return nil, domain.Violation(domain.InvalidRequest, "MAIN_SOURCE_QUERY_INVALID")
	}
	fixed, err := s.fixedContext(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(mainReadRequest(fixed), input) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	if err = s.revalidate(ctx, fixed); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	current, err := s.validateCurrent(ctx, tx, fixed)
	if err != nil {
		return nil, err
	}
	units, epoch, err := mainSourceUnits(ctx, tx, fixed.Context.Target.DocumentId, current.CurrentManifestId, fixed.Control)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"manifest_id": current.CurrentManifestId, "structure_id": current.CurrentStructureId, "style_id": current.CurrentStyleId, "control_revision": current.ControlRevision, "workspace_epoch": domain.TextID(epoch), "units": units}, nil
}

// 原来源锁、MAIN门闩和原准入一起复核；私有mTLS资格不能代替用户的READ资格。
// 发布再次调用本函数，所以捕获后发生撤权/结构切换时不会释放过时的敏感正文。
func (s *MainReadViews) validateCurrent(ctx context.Context, tx *sql.Tx, fixed mainReadContext) (dto.Document, error) {
	var current dto.Document
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return current, err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return current, err
	}
	if !fixed.Access.Permissions.Read || current.MainLocked || current.Lifecycle != "ACTIVE" ||
		current.CurrentManifestId != domain.TextID(fixed.Manifest) ||
		current.CurrentStructureId != domain.TextID(fixed.Structure) ||
		current.CurrentStyleId != domain.TextID(fixed.Style) || current.ControlRevision != domain.TextID(fixed.Control) {
		return current, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Context.Target.DocumentId)
	if err != nil {
		return current, err
	}
	if barrier.Phase != "OPEN" {
		return current, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	epoch, err := repository.MainWorkspaceEpoch(ctx, tx, fixed.Context.Target.DocumentId)
	if err != nil {
		return current, err
	}
	if epoch != reliableMainEpoch(mainReadBinding(fixed)) {
		return current, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	actual, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return current, err
	}
	if err = validateMainReadAdmission(fixed, actual); err != nil {
		return current, err
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return current, err
	}
	if state.State != "OPEN" || !state.Expires.After(time.Now()) || fixed.ExpiresUnixMs <= time.Now().UnixMilli() || fixed.SessionExpires <= time.Now().UnixMilli() {
		return current, domain.Violation(domain.PermissionDenied, "MAIN_READ_NOT_CURRENT")
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			return current, domain.Violation(domain.PermissionDenied, "MAIN_READ_EXPIRED")
		}
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return current, domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_BINDING_UNKNOWN")
	}
	return current, nil
}

func validateMainReadAdmission(fixed mainReadContext, actual repository.OriginalAdmissionBinding) error {
	if err := validateMainReadContext(fixed); err != nil {
		return err
	}
	ctx := fixed.Context
	if actual.Actor != fixed.Actor || actual.Document != ctx.Target.DocumentId || actual.Unit != 0 ||
		actual.Identity != ctx.IdentityAdmissionRef || actual.Operation != ctx.OperationId ||
		actual.Purpose != "READ_MAIN" || actual.SessionExpires != fixed.Access.Proofs[0].ExpiresUnixMs ||
		!bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_BINDING_UNKNOWN")
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, ctx.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_BINDING_UNKNOWN")
	}
	return nil
}

func validateMainReadContext(fixed mainReadContext) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_BINDING_UNKNOWN")
	if epoch := reliableMainEpoch(mainReadBinding(fixed)); epoch == 0 || epoch > fixed.Control {
		return invalid
	}
	ctx := fixed.Context
	if ctx == nil || ctx.Target == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil {
		return invalid
	}
	if fixed.Actor == 0 || ctx.ActorId != fixed.Actor || ctx.DocumentAdmissionRef == "" ||
		!fixed.Access.Permissions.Read || fixed.Access.Document.MainLocked ||
		fixed.Access.Document.Lifecycle != "ACTIVE" || fixed.ExpiresUnixMs <= 0 ||
		fixed.ExpiresUnixMs > fixed.SessionExpires {
		return invalid
	}
	target := &pb.Target{DocumentId: ctx.Target.DocumentId, StructureId: fixed.Structure, WorkspaceEpoch: fixed.Control, Purpose: "READ_MAIN"}
	if target.DocumentId == 0 || fixed.Manifest == 0 || fixed.Structure == 0 || fixed.Style == 0 || fixed.Control == 0 || !proto.Equal(ctx.Target, target) || !equalPullSources(ctx.Sources, fixed.Access.Sources) {
		return invalid
	}
	document := fixed.Access.Document
	if document.Id != domain.TextID(target.DocumentId) ||
		document.CurrentManifestId != domain.TextID(fixed.Manifest) ||
		document.CurrentStructureId != domain.TextID(fixed.Structure) ||
		document.CurrentStyleId != domain.TextID(fixed.Style) || document.ControlRevision != domain.TextID(fixed.Control) {
		return invalid
	}
	first := fixed.Access.Proofs[0]
	if first.AdmissionId != ctx.IdentityAdmissionRef || first.OperationId != ctx.OperationId {
		return invalid
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.Target == nil || proof.ActorId != fixed.Actor || proof.Purpose != "READ_MAIN" ||
			proof.Target.DocumentId != target.DocumentId || proof.Target.Purpose != "READ_MAIN" ||
			len(proof.SessionRef) != 32 || !bytes.Equal(proof.SessionRef, first.SessionRef) ||
			fixed.ExpiresUnixMs > proof.ExpiresUnixMs {
			return invalid
		}
	}
	fingerprint, err := domain.OperationFingerprint("/v1/documents/"+document.Id+"/main", map[string]any{"manifest_id": document.CurrentManifestId, "structure_id": document.CurrentStructureId, "style_id": document.CurrentStyleId, "control_revision": document.ControlRevision})
	if err != nil || !bytes.Equal(fingerprint[:], ctx.RequestFingerprint) {
		return invalid
	}
	if len(fixed.Versions) != len(fixed.Units) {
		return invalid
	}
	seen := make(map[uint64]bool, len(fixed.Units))
	for _, unit := range fixed.Units {
		if unit == 0 || seen[unit] || fixed.Versions[unit] == 0 {
			return invalid
		}
		seen[unit] = true
	}
	return nil
}

// main_source_units.go：保留原实现，按业务合并文件。

// MAIN阅读、合并与管理控制共享全篇版本读取；所有字节属于同一Doc事务，不向Node开放任意版本查询。
func mainSourceUnits(ctx context.Context, tx *sql.Tx, document uint64, manifest string, control uint64) ([]any, uint64, error) {
	epoch, err := repository.MainWorkspaceEpoch(ctx, tx, document)
	if err != nil {
		return nil, 0, err
	}
	units, err := repository.ManifestUnits(ctx, tx, manifest, document)
	if err != nil {
		return nil, 0, err
	}
	result := make([]any, 0, len(units))
	for _, unit := range units {
		version, _ := domain.CanonicalID(unit["content_version_id"])
		content, err := repository.VersionContent(ctx, tx, version, document)
		if err != nil {
			return nil, 0, err
		}
		if err = canonicalVersion(content, version); err != nil {
			return nil, 0, err
		}
		content.WorkspaceEpoch = control
		wire, err := fixedWire(content)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, map[string]any{"unit_id": unit["unit_id"], "content_version_id": unit["content_version_id"], "content": wire})
	}
	return result, epoch, nil
}

// reliable_main_capture.go：保留原实现，按业务合并文件。

// reliableMainBinding只表达可靠整篇内容的固定来源。阅读准入的短期限与发布的
// 持久管理证明分别由各自用例验证，不能因为复用内容验证而共享授权用途。
type reliableMainBinding struct {
	Context   *pb.OperationContext `json:"context"`
	Manifest  uint64               `json:"manifest"`
	Structure uint64               `json:"structure"`
	Style     uint64               `json:"style"`
	Control   uint64               `json:"control"`
	Epoch     uint64               `json:"main_epoch,omitempty"`
	Units     []uint64             `json:"units"`
	Versions  map[uint64]uint64    `json:"versions"`
}

func mainReadBinding(fixed mainReadContext) reliableMainBinding {
	return reliableMainBinding{Context: fixed.Context, Manifest: fixed.Manifest, Structure: fixed.Structure,
		Style: fixed.Style, Control: fixed.Control, Epoch: fixed.MainEpoch, Units: fixed.Units, Versions: fixed.Versions}
}

// 旧固定上下文缺少此字段时，原实现的物理epoch就是原control。新上下文
// 必须从当前manifest的实际完成安装取得epoch，不能用当前元数据修订猜测。
func reliableMainEpoch(fixed reliableMainBinding) uint64 {
	if fixed.Epoch != 0 {
		return fixed.Epoch
	}
	return fixed.Control
}

func validateReliableMainCapture(validator *codec.Validator, fixed reliableMainBinding, main *pb.MainCaptureReply, usage, prefix string) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, prefix+"_CAPTURE_INVALID")
	if fixed.Context == nil || fixed.Context.Target == nil || main == nil || main.State != pb.CaptureReply_FIXED || main.Cut == nil {
		return invalid
	}
	if main.Cut.DocumentId != fixed.Context.Target.DocumentId ||
		main.Cut.StructureId != fixed.Structure || main.Cut.StyleId != fixed.Style ||
		main.Cut.ControlRevision != fixed.Control || len(main.Units) != len(fixed.Units) {
		return invalid
	}
	rooms := make(map[uint64]*pb.RoomCut, len(main.Cut.Rooms))
	for _, room := range main.Cut.Rooms {
		if room == nil || room.RoomId == 0 || room.WorkspaceEpoch != reliableMainEpoch(fixed) || rooms[room.RoomId] != nil {
			return invalid
		}
		rooms[room.RoomId] = room
	}
	if err := validateMainCaptureDigest(main); err != nil {
		return err
	}
	expected := make(map[uint64]bool, len(fixed.Units))
	for _, unit := range fixed.Units {
		if unit == 0 || expected[unit] || fixed.Versions[unit] == 0 {
			return invalid
		}
		expected[unit] = true
	}
	for _, content := range main.Units {
		if content == nil || !expected[content.UnitId] || content.BranchId != 0 ||
			content.WorkspaceEpoch != fixed.Control || content.OperationId != fixed.Context.OperationId ||
			content.CaptureId != fixed.Context.OperationId+":"+usage+":"+domain.TextID(content.UnitId) {
			return invalid
		}
		delete(expected, content.UnitId)
		if err := validateFixedMergeContent(validator, content, fixed.Context.Target.DocumentId, content.UnitId, fixed.Structure); err != nil {
			return err
		}
		if err := validateReliableMainContentBinding(fixed, content, rooms, prefix); err != nil {
			return err
		}
	}
	if len(rooms) != 0 {
		return invalid
	}
	return nil
}

// reliable_main_content.go：保留原实现，按业务合并文件。

// 全篇摘要自洽仍不等于归属正确：每个单元必须消费唯一真实room cut，或明确引用原固定版本。
// provenance的顶层来源同时绑定原业务清单、单元基线和本次操作号。
func validateReliableMainContentBinding(fixed reliableMainBinding, content *pb.FixedContent, rooms map[uint64]*pb.RoomCut, prefix string) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, prefix+"_CONTENT_BINDING_INVALID")
	cutValue, err := codec.Decode(content.CutJsonUtf8)
	if err != nil {
		return invalid
	}
	canonical, err := codec.Canonical(cutValue)
	if err != nil || !bytes.Equal(canonical, content.CutJsonUtf8) {
		return invalid
	}
	cut, ok := cutValue.(map[string]any)
	if !ok {
		return invalid
	}
	if roomText, ok := cut["room_id"].(string); ok {
		roomID, err := domain.CanonicalID(roomText)
		room := rooms[roomID]
		if err != nil || room == nil || len(cut) != 3 || cut["workspace_epoch"] != domain.TextID(room.WorkspaceEpoch) || cut["durable_seq"] != domain.TextID(room.DurableSeq) {
			return invalid
		}
		delete(rooms, roomID)
	} else {
		ref, ok := cut["source_ref"].(string)
		if len(cut) != 2 || cut["content_version_id"] != domain.TextID(fixed.Versions[content.UnitId]) || !ok || ref == "" {
			return invalid
		}
	}
	value, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil {
		return invalid
	}
	provenance, ok := value.(map[string]any)
	if !ok || provenance["base_manifest_id"] != domain.TextID(fixed.Manifest) ||
		provenance["base_content_version_id"] != domain.TextID(fixed.Versions[content.UnitId]) ||
		provenance["source_operation_id"] != fixed.Context.OperationId || provenance["lineage_id"] != content.LineageRef {
		return invalid
	}
	return nil
}

func validateReliableMainVersion(ctx context.Context, tx *sql.Tx, fixed reliableMainBinding, content *pb.FixedContent, prefix string) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, prefix+"_VERSION_BINDING_INVALID")
	version := fixed.Versions[content.UnitId]
	original, err := repository.VersionContent(ctx, tx, version, fixed.Context.Target.DocumentId)
	if err != nil {
		return err
	}
	if err = canonicalVersion(original, version); err != nil {
		return err
	}
	if original.UnitId != content.UnitId || original.StructureId != fixed.Structure || original.LineageRef != content.LineageRef {
		return invalid
	}
	oldValue, err := codec.Decode(original.ProvenanceJsonUtf8)
	if err != nil {
		return invalid
	}
	newValue, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil {
		return invalid
	}
	old, oldOK := oldValue.(map[string]any)
	next, nextOK := newValue.(map[string]any)
	if !oldOK || !nextOK {
		return invalid
	}
	accepted, err := codec.Canonical(old["accepted_captures"])
	if err != nil {
		return invalid
	}
	current, err := codec.Canonical(next["accepted_captures"])
	if err != nil || !bytes.Equal(accepted, current) {
		return invalid
	}
	var cut map[string]string
	if json.Unmarshal(content.CutJsonUtf8, &cut) != nil {
		return invalid
	}
	if cut["room_id"] == "" {
		if cut["source_ref"] != original.CaptureId || !bytes.Equal(original.BodyHash, content.BodyHash) {
			return invalid
		}
		return nil
	}
	// 只读取Doc已经分配的真实房间，不在读取时创建或重置一个MAIN房间。
	// 管理元数据可以增加control而不安装新MAIN；房间编号必须仍绑定原物理代次。
	key := "MAIN:" + domain.TextID(content.UnitId) + ":" + domain.TextID(content.UnitId) + ":" + domain.TextID(reliableMainEpoch(fixed))
	raw, err := repository.LockedCollaborationRoom(ctx, tx, key)
	if err != nil {
		return err
	}
	var room map[string]string
	if json.Unmarshal(raw, &room) != nil || room["room_id"] != cut["room_id"] {
		return invalid
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`17fa3080603217b39f90b65f6fc127c932a3c134fae8421c73ffb0f2327688c3`。[参考实现](../../../../services/document/internal/logic/document/main_readings_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
