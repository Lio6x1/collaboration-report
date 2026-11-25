// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/proto"
	"slices"
	"time"
)

// fixed_resource_locks.go：保留原实现，按业务合并文件。

// MAIN 与 candidate 可能以不同单元顺序共享资源；先合并排序再锁定，避免跨 Doc
// 的合法转让资源在不同写入口产生相反的资源锁顺序。
func lockFixedContentResources(ctx context.Context, tx *sql.Tx, document, space uint64, contents []*pb.FixedContent, reason string) error {
	var ids []uint64
	for _, content := range contents {
		ids = append(ids, content.ResourceIds...)
	}
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		state, err := repository.LockedDocumentResource(ctx, tx, id, document, space)
		if err != nil {
			return err
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, reason)
		}
	}
	return nil
}

// resource_protection.go：保留原实现，按业务合并文件。

type NodeOutcomeReader interface {
	UpdateOutcome(context.Context, *pb.DraftResourceRequest) (domain.NodeUpdateProof, error)
}
type DraftResources struct {
	db       *sql.DB
	outcomes NodeOutcomeReader
}

func NewDraftResources(db *sql.DB, outcomes NodeOutcomeReader) *DraftResources {
	return &DraftResources{db: db, outcomes: outcomes}
}
func protectionReceipt(req *pb.DraftResourceRequest, state string) *pb.DraftResourceReceipt {
	digest := domain.ProtectionDigest(req.ResourceIds)
	value := pb.DraftResourceReceipt_State(pb.DraftResourceReceipt_State_value[state])
	return &pb.DraftResourceReceipt{State: value, OperationId: req.Context.OperationId, Generation: req.Generation, ResourceDigest: digest, ReceiptDigest: domain.ProtectionReceiptDigest(req.Context.OperationId, req.Generation, digest, state)}
}
func loadProtection(ctx context.Context, q repository.Queryer, id string) (*pb.DraftResourceRequest, error) {
	raw, err := repository.ResourceProtectionBinding(ctx, q, id)
	if err != nil {
		return nil, domain.RejectProtection(domain.ProtectionMissing, "PROTECTION_NOT_FOUND")
	}
	var saved struct {
		Wire string `json:"wire"`
	}
	if json.Unmarshal(raw, &saved) != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_BINDING_UNKNOWN")
	}
	wire, err := base64.RawStdEncoding.DecodeString(saved.Wire)
	req := new(pb.DraftResourceRequest)
	if err != nil || proto.Unmarshal(wire, req) != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_BINDING_UNKNOWN")
	}
	return req, nil
}

// 服务证书只许可调用；实际actor/target/全部来源必须匹配Doc持久的EDIT准入。
func validateProtectionGrant(ctx context.Context, tx *sql.Tx, req *pb.DraftResourceRequest) error {
	sources, err := repository.AdmissionSources(ctx, tx, req.Context.DocumentAdmissionRef)
	if errors.Is(err, domain.ErrSourcesRequired) {
		return domain.RejectProtection(domain.ProtectionForbidden, "SOURCES_REQUIRED")
	}
	if err != nil {
		return err
	}
	if err = LockSources(ctx, tx, sources); err != nil {
		return domain.RejectProtection(domain.ProtectionForbidden, "SOURCE_CLOSED")
	}
	current, err := repository.LockedProtectionAdmission(ctx, tx, req.Context.DocumentAdmissionRef)
	if err != nil || current.State != "OPEN" || !current.Expires.After(time.Now()) {
		return domain.RejectProtection(domain.ProtectionForbidden, "GRANT_CLOSED")
	}
	var binding EditBinding
	if json.Unmarshal(current.TargetJSON, &binding) != nil || binding.GrantID != req.GrantId || binding.ClientSessionID != req.ClientSessionId || binding.RoomID != req.RoomId || binding.Target == nil || binding.Target.WorkspaceEpoch != req.WorkspaceEpoch || current.Actor != req.Context.ActorId || !proto.Equal(binding.Target, req.Context.Target) || req.Context.IdentityAdmissionRef != "" {
		return domain.RejectProtection(domain.ProtectionForbidden, "RESOURCE_BINDING_INVALID")
	}
	if len(sources) != len(req.Context.Sources) {
		return domain.RejectProtection(domain.ProtectionForbidden, "SOURCE_BINDING_INVALID")
	}
	for i, source := range sources {
		if !proto.Equal(source, req.Context.Sources[i]) {
			return domain.RejectProtection(domain.ProtectionForbidden, "SOURCE_BINDING_INVALID")
		}
	}
	return nil
}
func (s *DraftResources) Protect(ctx context.Context, req *pb.DraftResourceRequest) (*pb.DraftResourceReceipt, error) {
	return s.protect(ctx, req, nil)
}

// 正文与资源使用同一原号、同一准入、同一事务，不留下正文已预留而资源拒绝的半结果。
func (s *DraftResources) ProtectBody(ctx context.Context, req *pb.DraftResourceRequest, body domain.DraftBodyMeasure) (*pb.DraftResourceReceipt, error) {
	return s.protect(ctx, req, &body)
}

func (s *DraftResources) protect(ctx context.Context, req *pb.DraftResourceRequest, body *domain.DraftBodyMeasure) (*pb.DraftResourceReceipt, error) {
	if err := ValidateDraftResourceRequest(req); err != nil {
		return nil, err
	}
	wire, _ := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	fingerprint := sha256.Sum256(wire)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = validateProtectionGrant(ctx, tx, req); err != nil {
		return nil, err
	}
	space, lifecycle, err := repository.LockedResourceDocument(ctx, tx, req.Context.Target.DocumentId)
	if err != nil || lifecycle != "ACTIVE" {
		return nil, domain.RejectProtection(domain.ProtectionForbidden, "DOCUMENT_NOT_CURRENT")
	}
	// 配额总是位于资源锁之前；不以事务外余额接受正文。
	if body != nil {
		if err = repository.LockDocumentCapacity(ctx, tx, req.Context.Target.DocumentId); err != nil {
			return nil, err
		}
	}
	prior, err := repository.LockedResourceProtection(ctx, tx, req.Context.OperationId)
	exists := err == nil
	if exists {
		if !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
			return nil, domain.RejectProtection(domain.ProtectionConflict, "PROTECTION_ID_CONFLICT")
		}
		if prior.State == "RELEASED" {
			return nil, domain.RejectProtection(domain.ProtectionPrecondition, "PROTECTION_RELEASED")
		}
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	if body != nil {
		limit, err := resources.QuotaLimit(ctx, tx, space)
		if err != nil {
			return nil, err
		}
		q, err := resources.LockQuota(ctx, tx, space, limit)
		if err != nil {
			return nil, err
		}
		if err = resources.ReserveDraftBody(ctx, tx, q, req.Context.Target.DocumentId, req.RoomId, req.WorkspaceEpoch, req.Context.OperationId, *body, exists); err != nil {
			return nil, err
		}
	}
	if exists {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return protectionReceipt(req, prior.State), nil
	}
	// READY核验与pin插入持有同一资源行锁；删除器也取得此锁，避免核验后资源被删除。
	for _, id := range req.ResourceIds {
		_, state, err := repository.LockedResourceSpace(ctx, tx, id)
		if err != nil || state != "READY" {
			return nil, domain.RejectProtection(domain.ProtectionPrecondition, "RESOURCE_NOT_READY")
		}
		allowed, err := resources.ResourceAllowedForDocument(ctx, tx, id, req.Context.Target.DocumentId, space)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, domain.RejectProtection(domain.ProtectionPrecondition, "RESOURCE_NOT_READY")
		}
	}
	ids, _ := json.Marshal(req.ResourceIds)
	sources, _ := json.Marshal(SourcesWire(req.Context.Sources))
	record := domain.ProtectionRecord{Operation: req.Context.OperationId, Document: req.Context.Target.DocumentId, Room: req.RoomId, Grant: req.GrantId, Epoch: req.WorkspaceEpoch, Generation: req.Generation, Fingerprint: fingerprint[:], ResourceJSON: string(ids), SourceJSON: string(sources)}
	if err = repository.InsertResourceProtection(ctx, tx, record); err != nil {
		return nil, err
	}
	saved, _ := json.Marshal(map[string]string{"wire": base64.RawStdEncoding.EncodeToString(wire)})
	if err = repository.InsertResourceProtectionBinding(ctx, tx, req.Context.OperationId, saved); err != nil {
		return nil, err
	}
	for _, id := range req.ResourceIds {
		if err = repository.InsertOperationResourceRef(ctx, tx, id, req.Context.Target.DocumentId, req.Context.OperationId, req.Generation); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_COMMIT_UNKNOWN")
	}
	return protectionReceipt(req, "PROTECTED"), nil
}
func (s *DraftResources) Get(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceReceipt, error) {
	req, err := s.queryProtection(ctx, in)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = validateProtectionGrant(ctx, tx, req); err != nil {
		return nil, err
	}
	state, err := repository.LockedResourceProtectionState(ctx, tx, req.Context.OperationId)
	if err != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_UNKNOWN")
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return protectionReceipt(req, state), nil
}
func (s *DraftResources) queryProtection(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceRequest, error) {
	if in == nil || in.Context == nil {
		return nil, domain.RejectProtection(domain.ProtectionInvalid, "PROTECTION_QUERY_INVALID")
	}
	req, err := loadProtection(ctx, s.db, in.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(req.Context, in.Context) || req.Generation != in.Generation || !bytes.Equal(domain.ProtectionDigest(req.ResourceIds), in.ResourceDigest) {
		return nil, domain.RejectProtection(domain.ProtectionConflict, "PROTECTION_BINDING_CONFLICT")
	}
	return req, nil
}

// resource_protection_input.go：保留原实现，按业务合并文件。

// RPC 和正文侧账 HTTP 共享请求校验，传输扩展不能绕过原 UPDATE 身份绑定。
func ValidateDraftResourceRequest(req *pb.DraftResourceRequest) error {
	if req == nil || req.Context == nil || req.Context.Target == nil || !domain.ValidOperationID(req.Context.OperationId) || req.Generation != 1 || len(req.UpdateHash) != 32 || !bytes.Equal(req.Context.RequestFingerprint, req.UpdateHash) || !domain.ValidOperationID(req.ClientUpdateId) || len(req.ResourceIds) > 10000 {
		return domain.RejectProtection(domain.ProtectionInvalid, "PROTECTION_INVALID")
	}
	expected := sha256.Sum256([]byte(domain.TextID(req.RoomId) + "\n" + domain.TextID(req.WorkspaceEpoch) + "\n" + req.ClientSessionId + "\n" + req.ClientUpdateId + "\n" + req.GrantId))
	if req.Context.OperationId != hex.EncodeToString(expected[:]) {
		return domain.RejectProtection(domain.ProtectionInvalid, "PROTECTION_ID_INVALID")
	}
	for i, id := range req.ResourceIds {
		if id == 0 || (i > 0 && req.ResourceIds[i-1] >= id) {
			return domain.RejectProtection(domain.ProtectionInvalid, "RESOURCE_SET_INVALID")
		}
	}
	return nil
}
