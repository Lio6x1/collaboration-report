// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// application.go：保留原实现，按业务合并文件。

type Applications struct {
	db            *sql.DB
	mutations     *Mutations
	repository    *repository.Applications
	organizations *repository.Organizations
	invitations   *repository.Invitations
	capture       CaptureSource
}

func NewApplications(db *sql.DB, mutations *Mutations, repo *repository.Applications, organizations *repository.Organizations, invitations *repository.Invitations, capture CaptureSource) *Applications {
	return &Applications{
		db:            db,
		mutations:     mutations,
		repository:    repo,
		organizations: organizations,
		invitations:   invitations,
		capture:       capture,
	}
}

func (s *Applications) create(ctx context.Context, tx *sql.Tx, organization, actor, reason, invitation string) (string, error) {
	state, err := s.organizations.LockMemberState(ctx, tx, organization, actor)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if state == "ACTIVE" || state == "REVOKING" {
		return "", domain.RuleViolation{Kind: domain.StateConflict, Code: "ALREADY_MEMBER_OR_TRANSITIONING"}
	}
	existing, err := s.repository.Pending(ctx, tx, organization, actor)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	id, err := s.repository.Insert(ctx, tx, organization, actor, reason, invitation)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(id), nil
}

func (s *Applications) Apply(ctx context.Context, mutation domain.Mutation, organization string, input dto.ApplicationCreate) (json.RawMessage, error) {
	mutation.Kind = "ORG_APPLY"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		org, err := repository.ReadOrganization(ctx, tx, organization, true)
		if err != nil || org.Lifecycle != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "ORGANIZATION_NOT_FOUND"}
		}
		id, err := s.create(ctx, tx, organization, session.UserID, input.Reason, "")
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

func (s *Applications) AcceptInvitation(ctx context.Context, mutation domain.Mutation, hash []byte, input dto.InvitationAccept) (json.RawMessage, error) {
	mutation.Kind = "ORG_LINK_APPLY"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		identity, err := s.invitations.IdentityByToken(ctx, tx, hash)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "INVITATION_NOT_FOUND"}
		}
		org, err := repository.ReadOrganization(ctx, tx, identity.OrganizationID, true)
		if err != nil || org.Lifecycle != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "ORGANIZATION_NOT_FOUND"}
		}
		// 有效性固定在申请提交事务；之后关闭/过期不能抹去原审批依据。
		invitation, err := s.invitations.LockValidity(ctx, tx, identity.ID)
		if err != nil || invitation.State != "OPEN" || !invitation.Expires.After(time.Now()) {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "INVITATION_NOT_CURRENT"}
		}
		id, err := s.create(ctx, tx, identity.OrganizationID, session.UserID, input.Reason, identity.ID)
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

func (s *Applications) Decide(ctx context.Context, mutation domain.Mutation, id string, input dto.ApplicationReview) (json.RawMessage, error) {
	mutation.Kind = "ORG_APPLICATION_DECIDE"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		identity, err := s.repository.Identity(ctx, tx, id)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "APPLICATION_NOT_FOUND"}
		}
		var applicantState string
		var applicantError error
		if input.Decision == "ACCEPT" {
			// 注销以目标user锁固定其全部组织；新增成员也先持同一目标锁，再锁组织。
			// 外键仅保证用户存在，普通快照读ACTIVE不能阻止其等待后已变为REVOKING。
			applicantState, applicantError = s.organizations.LockUserState(ctx, tx, identity.ApplicantID)
		}
		if _, err := AccessOrganization(ctx, tx, identity.OrganizationID, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture); err != nil {
			return nil, err
		}
		application, err := s.repository.LockStatus(ctx, tx, id)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "APPLICATION_NOT_FOUND"}
		}
		if application.State != "PENDING" || application.Revision != input.ExpectedRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "APPLICATION_CHANGED"}
		}
		next := "REJECTED"
		if input.Decision == "ACCEPT" {
			if applicantError != nil || applicantState != "ACTIVE" {
				return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "APPLICANT_NOT_ACTIVE"}
			}
			// 成员状态与账号状态分开读取；不存在的成员行不能沿用账号ACTIVE值。
			memberState, err := s.organizations.LockMemberState(ctx, tx, identity.OrganizationID, identity.ApplicantID)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if memberState == "REVOKING" {
				return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "MEMBER_TRANSITION_IN_PROGRESS"}
			}
			if memberState != "ACTIVE" {
				if err := s.repository.AdmitMember(ctx, tx, identity.OrganizationID, identity.ApplicantID); err != nil {
					return nil, err
				}
			}
			next = "ACCEPTED"
		}
		// 本次只核验当前管理员及目标状态，不重查已固定的提交时邀请有效性。
		if err := s.repository.Decide(ctx, tx, id, session.UserID, next); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

// application_directory.go：保留原实现，按业务合并文件。

type ApplicationDirectory struct {
	actor        string
	organization string
	revision     string
	repository   *repository.Applications
}

func (s *Applications) OpenDirectory(ctx context.Context, actor, organization string) (*ApplicationDirectory, error) {
	directory := &ApplicationDirectory{actor: actor, organization: organization, repository: s.repository}
	if organization != "" {
		org, err := AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)
		if err != nil {
			return nil, err
		}
		directory.revision = org.MemberRevision
	}
	return directory, nil
}

func (d *ApplicationDirectory) Revision() string { return d.revision }

func (d *ApplicationDirectory) List(ctx context.Context, last string, limit int) ([]domain.OrganizationApplication, bool, error) {
	items, err := d.repository.List(ctx, d.actor, d.organization, last, limit+1)
	if err != nil {
		return nil, false, err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

// invitation.go：保留原实现，按业务合并文件。

type Invitations struct {
	db         *sql.DB
	mutations  *Mutations
	repository *repository.Invitations
	codec      *domain.InvitationCodec
	capture    CaptureSource
}

func NewInvitations(db *sql.DB, mutations *Mutations, repo *repository.Invitations, codec *domain.InvitationCodec, capture CaptureSource) *Invitations {
	return &Invitations{db: db, mutations: mutations, repository: repo, codec: codec, capture: capture}
}

func (s *Invitations) Create(ctx context.Context, mutation domain.Mutation, organization string, input dto.InvitationCreate) (json.RawMessage, error) {
	mutation.Kind = "ORG_INVITE_CREATE"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		org, err := AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture)
		if err != nil {
			return nil, err
		}
		if org.MemberRevision != input.ExpectedMemberRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		token, hash, err := domain.RandomToken()
		if err != nil {
			return nil, err
		}
		encrypted, err := s.codec.Encrypt(token, organization)
		if err != nil {
			return nil, err
		}
		expires := time.Now().UTC().Add(time.Duration(input.ExpiresInHours) * time.Hour)
		id, err := s.repository.Insert(ctx, tx, organization, session.UserID, hash, encrypted, expires)
		if err != nil {
			return nil, err
		}
		// receipt只保存实体身份；分享令牌的密文独立保存，审计与操作结果不包含令牌。
		return OperationResult(input.OperationId, "SUCCEEDED", fmt.Sprint(id)), nil
	})
}

func (s *Invitations) Share(ctx context.Context, actor, organization, id string) (domain.InvitationShare, error) {
	if _, err := AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture); err != nil {
		return domain.InvitationShare{}, err
	}
	encrypted, expires, err := s.repository.Share(ctx, id, organization)
	if err != nil {
		return domain.InvitationShare{}, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "INVITATION_RESULT_UNAVAILABLE"}
	}
	token, err := s.codec.Decrypt(encrypted, organization)
	if err != nil {
		return domain.InvitationShare{}, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "INVITATION_KEY_UNAVAILABLE"}
	}
	return domain.InvitationShare{ID: id, Token: token, Expires: expires}, nil
}

func (s *Invitations) Cancel(ctx context.Context, mutation domain.Mutation, id string, input dto.VersionedAction) (json.RawMessage, error) {
	mutation.Kind = "ORG_INVITE_CANCEL"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		organization, err := s.repository.Organization(ctx, tx, id)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "INVITATION_NOT_FOUND"}
		}
		if _, err := AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture); err != nil {
			return nil, err
		}
		invitation, err := s.repository.LockStatus(ctx, tx, id)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "INVITATION_NOT_FOUND"}
		}
		if invitation.Revision != input.ExpectedRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		if invitation.State == "OPEN" {
			if err := s.repository.Close(ctx, tx, id); err != nil {
				return nil, err
			}
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

// invitation_directory.go：保留原实现，按业务合并文件。

type InvitationDirectory struct {
	organization domain.Organization
	repository   *repository.Invitations
}

func (s *Invitations) OpenDirectory(ctx context.Context, actor, organization string) (*InvitationDirectory, error) {
	org, err := AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)
	if err != nil {
		return nil, err
	}
	return &InvitationDirectory{organization: org, repository: s.repository}, nil
}

func (d *InvitationDirectory) Revision() string { return d.organization.MemberRevision }

func (d *InvitationDirectory) List(ctx context.Context, last string, limit int) ([]domain.OrganizationInvitation, bool, error) {
	items, err := d.repository.List(ctx, d.organization.ID, last, limit+1)
	if err != nil {
		return nil, false, err
	}
	for index := range items {
		if items[index].State == "OPEN" && !items[index].Expires.After(time.Now()) {
			items[index].State = "EXPIRED"
		}
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

// organization_close.go：保留原实现，按业务合并文件。

type OrganizationClosePeer interface {
	Plan(context.Context, platform.OrganizationClosureBinding) error
	Page(context.Context, platform.OrganizationClosurePageRequest) (platform.OrganizationClosurePage, error)
	CleanDocument(context.Context, platform.OrganizationClosureBinding, string) error
	Complete(context.Context, platform.OrganizationClosureBinding) error
}

type OrganizationClosures struct {
	db        *sql.DB
	mutations *Mutations
	fence     AccountCloseFencePeer
	peer      OrganizationClosePeer
}

func NewOrganizationClosures(db *sql.DB, mutations *Mutations, fence AccountCloseFencePeer, peer OrganizationClosePeer) *OrganizationClosures {
	return &OrganizationClosures{db: db, mutations: mutations, fence: fence, peer: peer}
}

func (s *OrganizationClosures) Begin(ctx context.Context, mutation domain.Mutation, organization, expected string) (json.RawMessage, error) {
	if !domain.ValidEntityID(organization) || !domain.ValidEntityID(expected) {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "ORGANIZATION_CLOSE_INPUT_INVALID"}
	}
	mutation.Kind = "ORG_CLOSE"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		if _, err := repository.BeginOrganizationClose(ctx, tx, mutation.OperationID, organization, session.UserID, expected); err != nil {
			return nil, err
		}
		return OperationResult(mutation.OperationID, "PREPARING", organization), nil
	})
}

func (s *OrganizationClosures) Ready(ctx context.Context) error {
	if s.fence == nil || s.peer == nil {
		return errors.New("ORGANIZATION_CLOSE_DEPENDENCIES_REQUIRED")
	}
	return repository.OrganizationCloseReady(ctx, s.db)
}

// lockOriginal沿user→receipt→organization→member→transition顺序；后台不要求原会话仍存活。
// 关闭期间账号仍可改变凭据，也可发起注销，恢复仅依据原组织来源和固定绑定。
func (s *OrganizationClosures) lockOriginal(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding) (repository.OrganizationCloseRecord, domain.OrganizationClosePlan, error) {
	var empty repository.OrganizationCloseRecord
	var plan domain.OrganizationClosePlan
	if !b.Valid() {
		return empty, plan, errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	closures := repository.NewAccountClosures(s.db)
	if _, err := closures.LockUser(ctx, tx, b.User); err != nil {
		return empty, plan, err
	}
	receipt, err := closures.Receipt(ctx, tx, b.Operation)
	if err != nil {
		return empty, plan, err
	}
	org, err := repository.ReadOrganization(ctx, tx, strconv.FormatUint(b.Organization, 10), true)
	if err != nil {
		return empty, plan, err
	}
	member, err := repository.ReadOrganizationMember(ctx, tx, org.ID, strconv.FormatUint(b.User, 10), true)
	if err != nil {
		return empty, plan, err
	}
	record, err := repository.ReadOrganizationClose(ctx, tx, b.Operation, true)
	if err != nil {
		return record, plan, err
	}
	plan, err = domain.DecodeOrganizationClosePlan(record.Plan)
	if err != nil {
		return record, plan, err
	}
	if plan.Binding != b || record.Actor != b.User || record.Organization != b.Organization || receipt.User != b.User || receipt.Kind != "ORG_CLOSE" || org.OwnerID != strconv.FormatUint(b.User, 10) || org.Revision != strconv.FormatUint(b.Revision+1, 10) || org.MemberRevision != strconv.FormatUint(b.MemberRevision+1, 10) || member.Epoch != b.Epoch+1 {
		return record, plan, errors.New("ORGANIZATION_CLOSE_BINDING_CHANGED")
	}
	if record.State == "SUCCEEDED" {
		if org.Lifecycle != "CLOSED" || member.State != "CLOSED" || receipt.State != "SUCCEEDED" {
			return record, plan, errors.New("ORGANIZATION_CLOSE_TERMINAL_CHANGED")
		}
	} else if org.Lifecycle != "CLOSING" || member.State != "REVOKING" || receipt.State != "PREPARING" || (record.State != "FENCING" && record.State != "APPLIED") {
		return record, plan, errors.New("ORGANIZATION_CLOSE_STATE_CHANGED")
	}
	return record, plan, nil
}

func (s *OrganizationClosures) Confirmed(ctx context.Context, b platform.OrganizationClosureBinding) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	record, _, err := s.lockOriginal(ctx, tx, b)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if record.State != "APPLIED" && record.State != "SUCCEEDED" {
		return false, nil
	}
	return repository.OrganizationCloseProof(ctx, tx, b, true)
}

func (s *OrganizationClosures) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	task, err := repository.ClaimOrganizationClose(ctx, tx, instance)
	if err != nil {
		_ = tx.Rollback()
		return
	}
	if tx.Commit() != nil {
		return
	}
	if err = s.advance(ctx, task); err != nil {
		repository.RetryOrganizationClose(ctx, s.db, task, "ORGANIZATION_CLOSE_ADVANCE_UNKNOWN")
	}
}

// 一次领取最多清理一份文档；网络不持SQL锁，SQL推进前重新核验原号及租约generation。
func (s *OrganizationClosures) advance(ctx context.Context, task domain.AccountCloseFenceTask) error {
	initial, err := domain.DecodeOrganizationClosePlan(task.Payload)
	if err != nil || initial.Binding.Operation != task.Operation {
		return errors.New("ORGANIZATION_CLOSE_TASK_INVALID")
	}
	b := initial.Binding
	record, err := repository.ReadOrganizationClose(ctx, s.db, b.Operation, false)
	if err != nil {
		return err
	}
	plan, err := domain.DecodeOrganizationClosePlan(record.Plan)
	if err != nil || plan.Binding != b {
		return errors.New("ORGANIZATION_CLOSE_PLAN_CHANGED")
	}
	if record.State == "FENCING" {
		request := repository.OrganizationCloseFenceRequest(b)
		ack, err := s.fence.FenceAccess(ctx, request)
		if err != nil || !platform.MatchesBootstrapAck(request, ack, "document") {
			return errors.New("ORGANIZATION_CLOSE_FENCE_UNCONFIRMED")
		}
		return s.save(ctx, task, b, func(tx *sql.Tx, record repository.OrganizationCloseRecord, current domain.OrganizationClosePlan) (bool, error) {
			if record.State != "FENCING" {
				return false, errors.New("ORGANIZATION_CLOSE_FENCE_STATE_CHANGED")
			}
			valid, err := repository.OrganizationCloseProof(ctx, tx, b, false)
			if err != nil || !valid {
				return false, errors.New("ORGANIZATION_CLOSE_FENCE_BINDING_CHANGED")
			}
			return false, repository.SaveOrganizationCloseFence(ctx, tx, b, ack.ResultDigest)
		})
	}
	if record.State != "APPLIED" {
		return errors.New("ORGANIZATION_CLOSE_RECOVERY_STATE_INVALID")
	}
	if err := s.peer.Plan(ctx, b); err != nil {
		return err
	}
	request := platform.OrganizationClosurePageRequest{Binding: b, PlanDigest: plan.PlanDigest, AfterDocument: plan.AfterDocument}
	page, err := s.peer.Page(ctx, request)
	if err != nil {
		return err
	}
	if !page.Matches(request) {
		return errors.New("ORGANIZATION_CLOSE_PAGE_INVALID")
	}
	terminal := false
	if len(page.Documents) > 0 {
		item := page.Documents[0]
		id, parseErr := strconv.ParseUint(item.Document, 10, 64)
		var after uint64
		if plan.AfterDocument != "" {
			after, _ = strconv.ParseUint(plan.AfterDocument, 10, 64)
		}
		if parseErr != nil || id <= after || !domain.ValidEntityID(item.Document) {
			return errors.New("ORGANIZATION_CLOSE_PAGE_INVALID")
		}
		if err = s.peer.CleanDocument(ctx, b, item.Document); err != nil {
			return err
		}
		plan.AfterDocument = item.Document
	} else {
		if !page.Done || page.NextAfter != plan.AfterDocument {
			return errors.New("ORGANIZATION_CLOSE_PAGE_INVALID")
		}
		if err = s.peer.Complete(ctx, b); err != nil {
			return err
		}
		// Complete的成功回复还需原分页读取SUCCEEDED，丢失回复继续原号查询。
		proof, err := s.peer.Page(ctx, request)
		if err != nil || !proof.Matches(request) || proof.PlanDigest != page.PlanDigest || proof.State != "SUCCEEDED" || !proof.Done || len(proof.Documents) != 0 {
			return errors.New("ORGANIZATION_CLOSE_DOC_UNCONFIRMED")
		}
		terminal = true
	}
	plan.PlanDigest = page.PlanDigest
	return s.save(ctx, task, b, func(tx *sql.Tx, record repository.OrganizationCloseRecord, current domain.OrganizationClosePlan) (bool, error) {
		if record.State != "APPLIED" || current.PlanDigest != request.PlanDigest || current.AfterDocument != request.AfterDocument {
			return false, errors.New("ORGANIZATION_CLOSE_CURSOR_CHANGED")
		}
		confirmed, err := repository.OrganizationCloseProof(ctx, tx, b, true)
		if err != nil || !confirmed {
			return false, errors.New("ORGANIZATION_CLOSE_ACK_REQUIRED")
		}
		if err = repository.SaveOrganizationCloseCursor(ctx, tx, plan); err != nil {
			return false, err
		}
		if terminal {
			raw, _ := json.Marshal(OperationResult(b.Operation, "SUCCEEDED", strconv.FormatUint(b.Organization, 10)))
			err = repository.CompleteOrganizationClose(ctx, tx, b, raw)
		}
		return terminal, err
	})
}

func (s *OrganizationClosures) save(ctx context.Context, task domain.AccountCloseFenceTask, b platform.OrganizationClosureBinding, action func(*sql.Tx, repository.OrganizationCloseRecord, domain.OrganizationClosePlan) (bool, error)) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	record, plan, err := s.lockOriginal(ctx, tx, b)
	if err != nil {
		return err
	}
	if err = repository.ValidateOrganizationCloseTask(ctx, tx, task); err != nil {
		return err
	}
	terminal, err := action(tx, record, plan)
	if err != nil {
		return err
	}
	if err = repository.FinishOrganizationCloseTask(ctx, tx, task, terminal); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("ORGANIZATION_CLOSE_COMMIT_UNKNOWN")
	}
	return nil
}

// organization_read.go：保留原实现，按业务合并文件。

type CaptureSource func(context.Context, string, uint64)

type OrganizationRole uint8

const (
	OrganizationMember OrganizationRole = iota
	OrganizationAdministrator
	OrganizationOwner
)

// 角色要求与行锁是两个独立维度，用命名策略避免调用点出现连续的 true/false。
type OrganizationAccess struct {
	Role OrganizationRole
	Lock bool
}

func AccessOrganization(ctx context.Context, query repository.Queryer, id, actor string, access OrganizationAccess, capture CaptureSource) (domain.Organization, error) {
	org, err := repository.ReadOrganization(ctx, query, id, access.Lock)
	if err != nil || org.Lifecycle != "ACTIVE" {
		return org, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "ORGANIZATION_NOT_FOUND"}
	}
	member, err := repository.ReadOrganizationMember(ctx, query, id, actor, access.Lock)
	if err != nil || member.State != "ACTIVE" {
		return org, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "ORGANIZATION_NOT_FOUND"}
	}
	member.UserID = actor
	org.CurrentMember = &member
	owner, admin := access.Role == OrganizationOwner, access.Role == OrganizationAdministrator
	capture(ctx, "org-member:"+id+":"+actor, member.Epoch)
	if owner || (admin && org.OwnerID == actor) {
		capture(ctx, "org-owner:"+id+":"+actor, member.Epoch)
	} else if admin {
		capture(ctx, "org-admin:"+id+":"+actor, member.Epoch)
	}
	if owner && org.OwnerID != actor {
		return org, domain.RuleViolation{Kind: domain.PermissionDenied, Code: "ORGANIZATION_OWNER_REQUIRED"}
	}
	if admin && org.OwnerID != actor && member.Role != "ADMIN" {
		return org, domain.RuleViolation{Kind: domain.PermissionDenied, Code: "ORGANIZATION_ADMIN_REQUIRED"}
	}
	return org, nil
}

type Organizations struct {
	db         *sql.DB
	repository *repository.Organizations
	capture    CaptureSource
}

func NewOrganizations(db *sql.DB, repo *repository.Organizations, capture CaptureSource) *Organizations {
	return &Organizations{db: db, repository: repo, capture: capture}
}

func (s *Organizations) Get(ctx context.Context, actor, id string) (domain.Organization, error) {
	return AccessOrganization(ctx, s.db, id, actor, OrganizationAccess{Role: OrganizationMember}, s.capture)
}

func (s *Organizations) List(ctx context.Context, actor, last string, limit int) ([]domain.Organization, bool, error) {
	entries, err := s.repository.List(ctx, actor, last, limit+1)
	if err != nil {
		return nil, false, err
	}
	items := make([]domain.Organization, 0, len(entries))
	for _, entry := range entries {
		s.capture(ctx, "org-member:"+entry.Organization.ID+":"+actor, entry.MemberEpoch)
		items = append(items, entry.Organization)
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

// MemberDirectory只由当前资格核验产生，不能从浏览器自报的组织对象构造。
// 它在同一请求里提供绑定member_revision的分页，最终输出仍核验所捕获的来源代次。
type MemberDirectory struct {
	organization domain.Organization
	repository   *repository.Organizations
	capture      CaptureSource
}

func (s *Organizations) OpenMembers(ctx context.Context, actor, id string) (*MemberDirectory, error) {
	org, err := s.Get(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	return &MemberDirectory{organization: org, repository: s.repository, capture: s.capture}, nil
}

func (d *MemberDirectory) Revision() string { return d.organization.MemberRevision }

func (d *MemberDirectory) List(ctx context.Context, last string, limit int, query ...string) ([]domain.OrganizationMember, bool, error) {
	items, err := d.repository.Members(ctx, d.organization.ID, last, limit+1, query...)
	if err != nil {
		return nil, false, err
	}
	for _, member := range items {
		d.capture(ctx, "org-member:"+d.organization.ID+":"+member.UserID, member.Epoch)
		d.capture(ctx, "identity:"+member.UserID, member.IdentityEpoch)
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

// organization_transition.go：保留原实现，按业务合并文件。

type DeparturePeer interface {
	Check(context.Context, string, string, string, uint64) error
}

type OrganizationTransitions struct {
	mutations  *Mutations
	repository *repository.Organizations
	departure  DeparturePeer
	capture    CaptureSource
}

func NewOrganizationTransitions(mutations *Mutations, repo *repository.Organizations, departure DeparturePeer, capture CaptureSource) *OrganizationTransitions {
	return &OrganizationTransitions{mutations: mutations, repository: repo, departure: departure, capture: capture}
}

func (s *OrganizationTransitions) ChangeMember(ctx context.Context, mutation domain.Mutation, organization, user, expected, kind, nextRole string) (json.RawMessage, error) {
	mutation.Kind = kind
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		org, err := AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: organizationChangeRole(kind), Lock: true}, s.capture)
		if err != nil {
			return nil, err
		}
		if org.OwnerID == user {
			return nil, domain.RuleViolation{Kind: domain.PermissionDenied, Code: "OWNER_CANNOT_BE_REMOVED_OR_DEMOTED"}
		}
		member, err := s.repository.LockTransitionMember(ctx, tx, organization, user)
		if err != nil || member.State != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "TARGET_NOT_MEMBER"}
		}
		if member.Revision != expected {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		if member.Epoch == ^uint64(0) {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "EPOCH_EXHAUSTED"}
		}
		if kind == "ORG_LEAVE" && member.Role == "ADMIN" {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "ADMIN_RETIREMENT_REQUIRED"}
		}
		if kind == "ORG_LEAVE" || kind == "ORG_REMOVE" {
			if err := s.departure.Check(ctx, organization, user, mutation.OperationID, member.Epoch); err != nil {
				return nil, err
			}
		}
		if kind == "ORG_ROLE" && member.Role == nextRole {
			return OperationResult(mutation.OperationID, "SUCCEEDED", organization), nil
		}
		payload := domain.SessionClosure{
			Kind:           kind,
			OrganizationID: organization,
			UserID:         user,
			Role:           nextRole,
			Scope:          "org-member:" + organization + ":" + user,
			Epoch:          strconv.FormatUint(member.Epoch, 10),
		}
		// 先暂停旧来源再登记固定关闭计划；完成后普通成员身份以新代次恢复。
		if err := s.repository.PrepareMemberTransition(ctx, tx, organization, user); err != nil {
			return nil, err
		}
		if err := EnqueueSourceClosure(ctx, tx, mutation.OperationID, session.UserID, payload); err != nil {
			return nil, err
		}
		transition := "ROLE_CHANGE"
		if kind == "ORG_LEAVE" {
			transition = "LEAVE"
		}
		plan, _ := json.Marshal(payload)
		if err := s.repository.InsertMemberTransition(ctx, tx, mutation.OperationID, organization, session.UserID, user, transition, expected, plan); err != nil {
			return nil, err
		}
		return OperationResult(mutation.OperationID, "PREPARING", organization), nil
	})
}

// organization_write.go：保留原实现，按业务合并文件。

type OrganizationActions struct {
	mutations  *Mutations
	repository *repository.Organizations
	capture    CaptureSource
}

func NewOrganizationActions(mutations *Mutations, repo *repository.Organizations, capture CaptureSource) *OrganizationActions {
	return &OrganizationActions{mutations: mutations, repository: repo, capture: capture}
}

func (s *OrganizationActions) Create(ctx context.Context, mutation domain.Mutation, input dto.NamedCreate) (json.RawMessage, error) {
	mutation.Kind = "ORG_CREATE"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		id, err := s.repository.Insert(ctx, tx, input.Name, session.UserID)
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", fmt.Sprint(id)), nil
	})
}

func (s *OrganizationActions) Edit(ctx context.Context, mutation domain.Mutation, id string, input dto.OrganizationEdit) (json.RawMessage, error) {
	mutation.Kind = "ORG_EDIT"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		organization, err := AccessOrganization(ctx, tx, id, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture)
		if err != nil {
			return nil, err
		}
		if organization.Revision != input.ExpectedRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		if err := s.repository.Rename(ctx, tx, id, input.Name); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

func (s *OrganizationActions) ChangeCreationGrant(ctx context.Context, mutation domain.Mutation, id string, input dto.CreationGrant) (json.RawMessage, error) {
	mutation.Kind = "ORG_CREATION_GRANT"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		organization, err := AccessOrganization(ctx, tx, id, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture)
		if err != nil {
			return nil, err
		}
		if organization.MemberRevision != input.ExpectedRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		state, err := s.repository.LockMemberState(ctx, tx, id, input.UserId)
		if err != nil || state != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "TARGET_NOT_MEMBER"}
		}
		state = "CLOSED"
		pending := false
		if input.Allowed {
			state = "ACTIVE"
		} else {
			// 撤销创建资格仍须阻止旧准入提交建库；门闩、变更和恢复任务一起提交。
			revision, previousState, err := s.repository.LockCreationGrant(ctx, tx, id, input.UserId)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if err == nil && previousState == "ACTIVE" {
				payload := domain.SessionClosure{
					Kind:           "ORG_CREATE_REVOKE",
					OrganizationID: id,
					UserID:         input.UserId,
					Scope:          "org-create:" + id + ":" + input.UserId,
					Epoch:          strconv.FormatUint(revision, 10),
				}
				if err := EnqueueSourceClosure(ctx, tx, input.OperationId, session.UserID, payload); err != nil {
					return nil, err
				}
				pending = true
			}
		}
		if err := s.repository.WriteCreationGrant(ctx, tx, id, input.UserId, state); err != nil {
			return nil, err
		}
		if pending {
			return OperationResult(input.OperationId, "PREPARING", id), nil
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

// owner_transfer.go：保留原实现，按业务合并文件。

func (s *OrganizationTransitions) TransferOwner(ctx context.Context, mutation domain.Mutation, organization string, input dto.OwnerChange) (json.RawMessage, error) {
	mutation.Kind = "ORG_OWNER_TRANSFER"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		var targetState string
		var targetError error
		if input.TargetUserId != session.UserID {
			// 与账号注销一致，先锁目标身份再锁组织；注销冻结后不能授予新的组织所有权。
			targetState, targetError = s.repository.LockUserState(ctx, tx, input.TargetUserId)
		}
		org, err := AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: OrganizationOwner, Lock: true}, s.capture)
		if err != nil {
			return nil, err
		}
		if org.Revision != input.ExpectedRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		if input.TargetUserId == session.UserID {
			return OperationResult(input.OperationId, "SUCCEEDED", organization), nil
		}
		if targetError != nil || targetState != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "TARGET_NOT_MEMBER"}
		}
		state, err := s.repository.LockMemberState(ctx, tx, organization, input.TargetUserId)
		if err != nil || state != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "TARGET_NOT_MEMBER"}
		}
		member, err := s.repository.LockOwnerMember(ctx, tx, organization, session.UserID)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "OWNER_CHANGED"}
		}
		if member.Epoch == ^uint64(0) {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "EPOCH_EXHAUSTED"}
		}
		payload := domain.SessionClosure{
			Kind:           "ORG_OWNER_TRANSFER",
			OrganizationID: organization,
			UserID:         session.UserID,
			Role:           member.Role,
			Scope:          "org-owner:" + organization + ":" + session.UserID,
			Epoch:          strconv.FormatUint(member.Epoch, 10),
		}
		if err := s.repository.PrepareOwnerTransfer(ctx, tx, organization, session.UserID, input.TargetUserId); err != nil {
			return nil, err
		}
		if err := EnqueueSourceClosure(ctx, tx, input.OperationId, session.UserID, payload); err != nil {
			return nil, err
		}
		plan, _ := json.Marshal(payload)
		if err := s.repository.InsertOwnerTransfer(ctx, tx, input.OperationId, organization, session.UserID, input.TargetUserId, input.ExpectedRevision, plan); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", organization), nil
	})
}

func organizationChangeRole(kind string) OrganizationRole {
	switch kind {
	case "ORG_LEAVE":
		return OrganizationMember
	case "ORG_ROLE":
		return OrganizationOwner
	default:
		return OrganizationAdministrator
	}
}
