// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// content_versions.go：保留原实现，按业务合并文件。

type ContentVersions struct {
	db        *sql.DB
	access    *Access
	validator *codec.Validator
}

func NewContentVersions(db *sql.DB, access *Access, validator *codec.Validator) *ContentVersions {
	return &ContentVersions{db: db, access: access, validator: validator}
}

func (s *ContentVersions) Lookup(ctx context.Context, id uint64) (domain.ContentVersionScope, error) {
	scope, err := repository.ContentVersionScope(ctx, s.db, id)
	if err == sql.ErrNoRows {
		return scope, domain.Violation(domain.EntityNotFound, "CONTENT_VERSION_NOT_FOUND")
	}
	return scope, err
}

func (s *ContentVersions) Read(ctx context.Context, actor Actor, scope domain.ContentVersionScope) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, scope.Document, "READ_CONTENT_VERSION")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 归属查询只取得鉴权目标。正文交付前，在最终输出的同一事务和来源锁下重新核验。
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		current, err := repository.ContentVersionScope(ctx, tx, scope.Version)
		if err != nil || current != scope {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "CONTENT_VERSION_SCOPE_CHANGED")
		}
		allowed := access.Permissions.Manage || access.Permissions.Edit[scope.Unit]
		if !allowed && (access.Permissions.Comment || access.Permissions.Review) {
			allowed, err = repository.SharedContentVersion(ctx, tx, scope)
			if err != nil {
				return nil, err
			}
		}
		if !allowed {
			allowed, err = historicalContentRangeCovered(ctx, tx, access, scope.Document, scope.Unit, s.validator)
			if err != nil {
				return nil, err
			}
		}
		if !allowed {
			return nil, domain.Violation(domain.PermissionDenied, "CONTENT_VERSION_SCOPE_NOT_ALLOWED")
		}
		return repository.ContentRow(ctx, tx, domain.TextID(scope.Version), domain.TextID(scope.Document))
	}
	return access.Sources, run, nil
}

// edit_bootstrap.go：保留原实现，按业务合并文件。

// Node seed仅来自当前Doc准入的固定基线；Go不解析Yjs，也不读取Node私有表。
func (s *EditSessions) Bootstrap(ctx context.Context, admission, grant string) (map[string]any, error) {
	sources, err := repository.AdmissionSources(ctx, s.db, admission)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	current, err := repository.LockedEditingAdmission(ctx, tx, admission)
	if err != nil || current.State != "OPEN" || !current.Expires.After(time.Now()) {
		return nil, domain.Violation(domain.PermissionDenied, "ADMISSION_CLOSED")
	}
	var binding EditBinding
	if json.Unmarshal(current.TargetJSON, &binding) != nil || binding.GrantID != grant || binding.Target == nil {
		return nil, domain.Violation(domain.PermissionDenied, "GRANT_BINDING_INVALID")
	}
	lifecycle, structure, err := repository.LockedEditingStructure(ctx, tx, binding.Target.DocumentId)
	if err != nil || lifecycle != "ACTIVE" || structure != binding.Target.StructureId {
		return nil, domain.Violation(domain.StateConflict, "WORKSPACE_NOT_CURRENT")
	}
	var body []byte
	var version uint64
	var lineageContext any
	var mainContext map[string]string
	if binding.WorkspaceKind == "BRANCH" {
		body, version, err = repository.BranchBootstrapBody(ctx, tx, binding.WorkspaceID, binding.Target.DocumentId, binding.Target.UnitId, binding.Target.WorkspaceEpoch)
		if err == nil {
			body, err = domain.BranchBaselineBody(body)
		}
		if err == nil {
			var lineage, manifest uint64
			var provenance []byte
			lineage, manifest, provenance, err = repository.BranchBootstrapLineage(ctx, tx, binding.WorkspaceID)
			if err == nil && json.Valid(provenance) {
				lineageContext = map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(manifest), "base_version_id": domain.TextID(version), "provenance": json.RawMessage(provenance)}
			} else if err == nil {
				return nil, domain.Violation(domain.StateConflict, "LINEAGE_INVALID")
			}
		}
	} else {
		body, version, err = repository.MainBootstrapBody(ctx, tx, binding.Target.DocumentId, binding.Target.UnitId)
		if err == nil {
			mainContext, err = repository.BootstrapMainContext(ctx, tx, binding.Target.DocumentId)
		}
	}
	if err != nil || !json.Valid(body) {
		return nil, domain.Violation(domain.StateConflict, "WORKSPACE_NOT_CURRENT")
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	// HTTP BIGINT保持十进制字符串，不能经过Node的JSON Number损失房间或版本身份。
	return map[string]any{"workspace_kind": binding.WorkspaceKind, "workspace_id": domain.TextID(binding.WorkspaceID), "body": json.RawMessage(body), "base_version_id": domain.TextID(version), "actor_id": domain.TextID(current.Actor), "session_ref": base64.RawStdEncoding.EncodeToString(current.SessionRef), "sources": SourcesWire(sources), "lineage_context": lineageContext, "main_context": mainContext}, nil
}

// edit_protocol.go：保留原实现，按业务合并文件。

// EditBinding是持久准入的固定绑定，沿用生成Target；它不包含可变Yjs正文。
type EditBinding struct {
	Target          *pb.Target `json:"target"`
	RoomID          uint64     `json:"room_id"`
	WorkspaceKind   string     `json:"workspace_kind"`
	WorkspaceID     uint64     `json:"workspace_id"`
	ClientSessionID string     `json:"client_session_id"`
	ConnectionID    string     `json:"connection_id,omitempty"`
	GrantID         string     `json:"grant_id,omitempty"`
}

// edit_redeem.go：保留原实现，按业务合并文件。

func ticketHash(token []byte) ([32]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(string(token))
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != string(token) {
		return [32]byte{}, domain.Violation(domain.InvalidRequest, "TICKET_INVALID")
	}
	return sha256.Sum256(raw), nil
}

// 一次票据只可被原client/connection重取原grant，丢回执不会变成多人可复用凭据。
func (s *EditSessions) Redeem(ctx context.Context, req *pb.RedeemTicketRequest) (*pb.CollaborationGrant, error) {
	hash, err := ticketHash(req.Ticket)
	if err != nil {
		return nil, err
	}
	admission, err := repository.TicketAdmission(ctx, s.db, hash[:])
	if err != nil {
		return nil, domain.Violation(domain.PermissionDenied, "TICKET_NOT_CURRENT")
	}
	sources, err := repository.AdmissionSources(ctx, s.db, admission)
	if errors.Is(err, domain.ErrSourcesRequired) {
		return nil, domain.Violation(domain.PermissionDenied, "SOURCES_REQUIRED")
	}
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// 先锁真实来源，再锁准入与票据，保持与关闭路径一致的锁顺序。
	if err = LockSources(ctx, tx, sources); err != nil {
		return nil, domain.Violation(domain.PermissionDenied, "SOURCE_CLOSED")
	}
	ticket, err := repository.LockedEditingTicket(ctx, tx, hash[:])
	if err != nil || ticket.Admission.State != "OPEN" || ticket.Client != req.ClientSessionId || !ticket.Admission.Expires.After(time.Now()) || ticket.State == "CLOSED" {
		return nil, domain.Violation(domain.PermissionDenied, "TICKET_NOT_CURRENT")
	}
	var binding EditBinding
	if json.Unmarshal(ticket.Admission.TargetJSON, &binding) != nil || binding.Target == nil || binding.ClientSessionID != ticket.Client {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "TICKET_BINDING_UNKNOWN")
	}
	if ticket.State == "REDEEMED" {
		if binding.ConnectionID != req.ConnectionId || binding.GrantID == "" {
			return nil, domain.Violation(domain.PermissionDenied, "TICKET_ALREADY_REDEEMED")
		}
	} else {
		if !ticket.Expires.After(time.Now()) {
			return nil, domain.Violation(domain.PermissionDenied, "TICKET_EXPIRED")
		}
		binding.ConnectionID = req.ConnectionId
		binding.GrantID = platform.NewTraceID()
		encoded, _ := json.Marshal(binding)
		if err = repository.BindEditingGrant(ctx, tx, admission, binding.GrantID, hash[:], encoded); err != nil {
			return nil, err
		}
	}
	fingerprint := sha256.Sum256(append(append([]byte{}, hash[:]...), []byte(req.ClientSessionId+"\n"+req.ConnectionId)...))
	operation := &pb.OperationContext{OperationId: binding.GrantID, ActorId: ticket.Admission.Actor, DocumentAdmissionRef: admission, Target: binding.Target, Sources: sources, RequestFingerprint: fingerprint[:]}
	result := &pb.CollaborationGrant{GrantId: binding.GrantID, Context: operation, RoomId: binding.RoomID, Capabilities: []string{"EDIT", "READ"}, ExpiresUnixMs: ticket.Admission.Expires.UnixMilli(), ClientSessionId: ticket.Client}
	if err = tx.Commit(); err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REDEEM_COMMIT_UNKNOWN")
	}
	return result, nil
}

// edit_session.go：保留原实现，按业务合并文件。

type EditSessions struct {
	db     *sql.DB
	access *Access
	tokens InvitationCipher
	origin string
}
type editResult struct {
	Session    dto.EditSession `json:"session"`
	Ciphertext []byte          `json:"ciphertext"`
	Binding    EditBinding     `json:"binding"`
}

func NewEditSessions(db *sql.DB, access *Access, tokens InvitationCipher, origin string) *EditSessions {
	return &EditSessions{db: db, access: access, tokens: tokens, origin: origin}
}

// 房间身份由Doc持久登记，同一workspace/epoch复用；Go不查询Node的私有库。
func collaborationRoom(ctx context.Context, tx *sql.Tx, kind string, workspace, unit, epoch uint64) (uint64, error) {
	key := kind + ":" + domain.TextID(workspace) + ":" + domain.TextID(unit) + ":" + domain.TextID(epoch)
	room, err := NewEntityID()
	if err != nil {
		return 0, err
	}
	raw, _ := json.Marshal(map[string]string{"room_id": domain.TextID(room)})
	if err = repository.EnsureCollaborationRoom(ctx, tx, key, raw); err != nil {
		return 0, err
	}
	raw, err = repository.LockedCollaborationRoom(ctx, tx, key)
	if err != nil {
		return 0, err
	}
	var found map[string]string
	if json.Unmarshal(raw, &found) != nil {
		return 0, domain.Violation(domain.TemporarilyUnavailable, "ROOM_BINDING_UNAVAILABLE")
	}
	return domain.CanonicalID(found["room_id"])
}
func (s *EditSessions) Create(ctx context.Context, actor Actor, document, unit uint64, input dto.EditSessionCreate) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.access.Authorize(ctx, actor, document, "EDIT_SESSION")
	if err != nil {
		return actor, nil, nil, err
	}
	if !access.Permissions.Edit[unit] {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.CurrentStructureId != input.ExpectedStructureId {
			return nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
		}
		if input.WorkspaceKind == "MAIN" && current.MainLocked {
			return nil, domain.Violation(domain.StateConflict, "MAIN_LOCKED")
		}
		branch, err := repository.LockedActiveBranch(ctx, tx, unit, document)
		if err != nil {
			return nil, domain.Violation(domain.StateConflict, "UNIT_CHANGED")
		}
		workspace := domain.EditWorkspace{Workspace: unit, Epoch: 1}
		if input.WorkspaceKind == "BRANCH" {
			workspace, err = repository.LockedEditWorkspace(ctx, tx, branch, document, unit)
			if err != nil {
				return nil, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
			}
		} else {
			workspace.Epoch, err = repository.MainWorkspaceEpoch(ctx, tx, document)
			if err != nil {
				return nil, err
			}
			branch = 0
		}
		room, err := collaborationRoom(ctx, tx, input.WorkspaceKind, workspace.Workspace, unit, workspace.Epoch)
		if err != nil {
			return nil, err
		}
		// grant不超过实际登录会话期限；兑换票据另有一分钟期限，二者不混用。
		grantExpires := time.Now().UTC().Add(5 * time.Minute)
		sessionExpires := time.UnixMilli(actor.SessionExpiresUnixMs)
		if sessionExpires.Before(grantExpires) {
			grantExpires = sessionExpires
		}
		ticketExpires := time.Now().UTC().Add(time.Minute)
		if grantExpires.Before(ticketExpires) {
			ticketExpires = grantExpires
		}
		var secret [32]byte
		if _, err = rand.Read(secret[:]); err != nil {
			return nil, err
		}
		token := base64.RawURLEncoding.EncodeToString(secret[:])
		hash := sha256.Sum256(secret[:])
		admission := platform.NewTraceID()
		structure, _ := domain.CanonicalID(current.CurrentStructureId)
		binding := EditBinding{Target: &pb.Target{DocumentId: document, UnitId: unit, BranchId: branch, StructureId: structure, WorkspaceEpoch: workspace.Epoch, Purpose: "EDIT_" + input.WorkspaceKind}, RoomID: room, WorkspaceKind: input.WorkspaceKind, WorkspaceID: workspace.Workspace, ClientSessionID: input.ClientSessionId}
		if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "EDIT_"+input.WorkspaceKind, admission); err != nil {
			return nil, err
		}
		target, _ := json.Marshal(binding)
		if err = repository.ConfigureEditAdmission(ctx, tx, unit, admission, target, grantExpires); err != nil {
			return nil, err
		}
		if err = repository.InsertEditTicket(ctx, tx, hash[:], admission, input.ClientSessionId, input.WorkspaceKind, workspace.Workspace, workspace.Epoch, ticketExpires); err != nil {
			return nil, err
		}
		encrypted, err := s.tokens.Seal(token, "edit:"+domain.TextID(actor.ID)+":"+input.OperationId)
		if err != nil {
			return nil, err
		}
		result := editResult{Session: dto.EditSession{TicketExpiresAt: ticketExpires.Format(time.RFC3339Nano), WssUrl: strings.Replace(s.origin, "https://", "wss://", 1) + "/collaboration", RoomId: domain.TextID(room), WorkspaceEpoch: domain.TextID(workspace.Epoch)}, Ciphertext: encrypted, Binding: binding}
		if input.WorkspaceKind == "BRANCH" {
			b, l, m, v := domain.TextID(branch), domain.TextID(workspace.Lineage), domain.TextID(workspace.BaseManifest), domain.TextID(workspace.BaseVersion)
			result.Session.BranchId = &b
			result.Session.LineageId = &l
			result.Session.BaseManifestId = &m
			result.Session.BaseVersionId = &v
		}
		encoded, _ := json.Marshal(result)
		if err = repository.InsertEditSessionResult(ctx, tx, input.OperationId, encoded); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", room), nil
	}
	// 原号重试返回同一票据密文，但当前EDIT资格与原期限仍须成立，不发行第二张票据。
	present := func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, nil, err
		}
		raw, err := repository.EditSessionResult(ctx, tx, input.OperationId)
		if err != nil {
			return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "EDIT_SESSION_UNKNOWN")
		}
		var result editResult
		if json.Unmarshal(raw, &result) != nil {
			return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "EDIT_SESSION_UNKNOWN")
		}
		expiry, err := time.Parse(time.RFC3339Nano, result.Session.TicketExpiresAt)
		if err != nil || !expiry.After(time.Now()) {
			return nil, nil, domain.Violation(domain.StateConflict, "TICKET_EXPIRED")
		}
		result.Session.Ticket, err = s.tokens.Open(result.Ciphertext, "edit:"+domain.TextID(actor.ID)+":"+input.OperationId)
		return result.Session, access.Sources, err
	}
	return actor, run, present, nil
}

// editable.go：保留原实现，按业务合并文件。

type EditableUnits struct {
	access     *Access
	pagination *Pagination
}

func NewEditableUnits(access *Access, pagination *Pagination) *EditableUnits {
	return &EditableUnits{access: access, pagination: pagination}
}
func (s *EditableUnits) Prepare(ctx context.Context, actor Actor, id uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "EDITABLE_UNITS")
	if err != nil {
		return nil, nil, err
	}
	// 游标绑定当前文档ACL、结构和库代次；翻页不能沿用已失效的可编辑范围。
	binding := "editable:" + domain.TextID(actor.ID) + ":" + domain.TextID(id) + ":" + access.Document.AclRevision + ":" + access.Document.CurrentStructureId + ":" + domain.TextID(access.KBRevision)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		page := dto.EditableUnitPage{Items: []dto.EditableUnit{}}
		err := repository.VisitEditableUnits(ctx, tx, access.Document.CurrentStructureId, id, last, func(fact domain.EditableUnitFact) bool {
			unitID, _ := domain.CanonicalID(fact.ID)
			if !access.Permissions.Edit[unitID] {
				return true
			}
			unit := dto.EditableUnit{UnitId: fact.ID, Path: fact.Title, BranchId: fact.Branch, WorkspaceEpoch: fact.Epoch, LineageId: fact.Lineage, State: "PREPARING"}
			if fact.Lifecycle != nil && *fact.Lifecycle == "ACTIVE" {
				unit.State = "READY"
			}
			page.Items = append(page.Items, unit)
			if len(page.Items) > limit {
				page.HasMore = true
				page.Items = page.Items[:limit]
				page.NextCursor = s.pagination.Next(binding, page.Items[limit-1].UnitId)
				return false
			}
			return true
		})
		return page, err
	}
	return access.Sources, run, nil
}
