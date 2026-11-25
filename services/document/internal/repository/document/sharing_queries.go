// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// application.go：保留原实现，按业务合并文件。

func ApplicationRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.DocumentApplication, error) {
	var v dto.DocumentApplication
	var raw []byte
	var validated, created time.Time
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(invitation_id AS CHAR),CAST(applicant_id AS CHAR),invitation_validated_at,requested_grants,state,reason,CAST(revision AS CHAR),created_at FROM document_application WHERE id=?"+suffix, id).Scan(&v.Id, &v.DocumentId, &v.InvitationId, &v.ApplicantId, &validated, &raw, &v.State, &v.Reason, &v.Revision, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return v, domain.Violation(domain.EntityNotFound, "APPLICATION_NOT_FOUND")
	}
	if err != nil {
		return v, err
	}
	if json.Unmarshal(raw, &v.RequestedGrants) != nil {
		return v, errors.New("APPLICATION_GRANTS_UNAVAILABLE")
	}
	v.InvitationValidatedAt = validated.UTC().Format(time.RFC3339Nano)
	stamp := created.UTC().Format(time.RFC3339Nano)
	v.CreatedAt = &stamp
	return v, nil
}
func PendingApplication(ctx context.Context, tx *sql.Tx, document, actor uint64) (uint64, error) {
	var id uint64
	err := tx.QueryRowContext(ctx, "SELECT id FROM document_application WHERE document_id=? AND applicant_id=? AND state='PENDING' ORDER BY id LIMIT 1 FOR UPDATE", document, actor).Scan(&id)
	return id, err
}
func InsertApplication(ctx context.Context, tx *sql.Tx, document, invitation, actor uint64, grants, reason string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES(?,?,?,?,TRUE,UTC_TIMESTAMP(6),?,'PENDING')", document, invitation, actor, grants, reason)
}
func ApproveInvitationGrant(ctx context.Context, tx *sql.Tx, document uint64, applicant string, grant dto.InvitationGrant) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,'ACTIVE') ON DUPLICATE KEY UPDATE state='ACTIVE',revision=revision+1", document, applicant, grant.ScopeKind, grant.ScopeId, grant.Capability)
	return err
}
func DecideApplication(ctx context.Context, tx *sql.Tx, id, reviewer uint64, state string) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?", state, reviewer, id)
	return err
}
func ApplicationIDs(ctx context.Context, q Queryer, key uint64, byDocument bool, last string, budget int, filter string) ([]uint64, error) {
	condition := "applicant_id=?"
	if byDocument {
		condition = "document_id=?"
		// 管理者归档不抹去申请人的个人进度。
		condition += " AND NOT EXISTS(SELECT 1 FROM document_sharing_archive a WHERE a.record_kind='APPLICATION' AND a.record_id=document_application.id)"
	}
	switch filter {
	case "PENDING":
		condition += " AND state='PENDING'"
	case "PROCESSED":
		condition += " AND state<>'PENDING'"
	case "ALL":
	default:
		return nil, domain.Violation(domain.InvalidRequest, "SHARING_FILTER_INVALID")
	}
	rows, err := q.QueryContext(ctx, "SELECT id FROM document_application WHERE "+condition+" AND (?='0' OR id<?) ORDER BY id DESC LIMIT ?", key, last, last, budget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		if rows.Scan(&id) != nil {
			return nil, errors.New("APPLICATIONS_UNAVAILABLE")
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func ActorOperationResult(ctx context.Context, tx *sql.Tx, operation string, actor uint64) ([]byte, error) {
	var raw []byte
	if tx.QueryRowContext(ctx, "SELECT result_json FROM operation_receipt WHERE operation_id=? AND actor_id=?", operation, actor).Scan(&raw) != nil {
		return nil, domain.Violation(domain.EntityNotFound, "OPERATION_NOT_FOUND")
	}
	return raw, nil
}

// invitation.go：保留原实现，按业务合并文件。

// InvitationFact保留真实期限与密文；是否到期由用例决定，读取不更新持久状态。
type InvitationFact struct {
	Invitation dto.DocumentInvitation
	ExpiresAt  time.Time
	Ciphertext []byte
}

func InvitationRow(ctx context.Context, q Queryer, id uint64, lock bool) (InvitationFact, error) {
	var fact InvitationFact
	var raw []byte
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	v := &fact.Invitation
	var created time.Time
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),state,expires_at,requested_grants,CAST(revision AS CHAR),token_ciphertext,created_at FROM document_invitation WHERE id=?"+suffix, id).Scan(&v.Id, &v.DocumentId, &v.State, &fact.ExpiresAt, &raw, &v.Revision, &fact.Ciphertext, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return fact, domain.Violation(domain.EntityNotFound, "INVITATION_NOT_FOUND")
	}
	if err != nil {
		return fact, err
	}
	if json.Unmarshal(raw, &v.RequestedGrants) != nil {
		return fact, errors.New("INVITATION_GRANTS_UNAVAILABLE")
	}
	stamp := created.UTC().Format(time.RFC3339Nano)
	v.CreatedAt = &stamp
	return fact, nil
}
func InvitationByHash(ctx context.Context, q Queryer, hash []byte) (uint64, uint64, error) {
	var invitation, document uint64
	if q.QueryRowContext(ctx, "SELECT id,document_id FROM document_invitation WHERE token_hash=?", hash).Scan(&invitation, &document) != nil {
		return 0, 0, domain.Violation(domain.EntityNotFound, "INVITATION_NOT_FOUND")
	}
	return invitation, document, nil
}
func InsertInvitation(ctx context.Context, tx *sql.Tx, document, actor uint64, hash, encrypted []byte, grants string, expires time.Time) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,'dev-document-invitation-v1',?,'OPEN',?)", document, actor, hash, encrypted, grants, expires)
}
func CloseInvitation(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_invitation SET state='CLOSED',revision=revision+1 WHERE id=? AND state='OPEN'", id)
	return err
}
func InvitationIDs(ctx context.Context, tx *sql.Tx, document uint64, last string, budget int, filter string) ([]uint64, error) {
	condition := ""
	switch filter {
	case "ACTIVE":
		condition = " AND state='OPEN' AND expires_at>UTC_TIMESTAMP(6)"
	case "INACTIVE":
		condition = " AND (state<>'OPEN' OR expires_at<=UTC_TIMESTAMP(6))"
	case "ALL":
	default:
		return nil, domain.Violation(domain.InvalidRequest, "SHARING_FILTER_INVALID")
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM document_invitation WHERE document_id=? AND (?='0' OR id<?)"+condition+" AND NOT EXISTS(SELECT 1 FROM document_sharing_archive a WHERE a.record_kind='INVITATION' AND a.record_id=document_invitation.id) ORDER BY id DESC LIMIT ?", document, last, last, budget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		if rows.Scan(&id) != nil {
			return nil, errors.New("INVITATIONS_UNAVAILABLE")
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func ArchiveSharingRecord(ctx context.Context, tx *sql.Tx, kind string, id, document, actor uint64) error {
	table := "document_invitation"
	if kind == "APPLICATION" {
		table = "document_application"
	} else if kind != "INVITATION" {
		return domain.Violation(domain.InvalidRequest, "SHARING_RECORD_INVALID")
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO document_sharing_archive(record_kind,record_id,document_id,actor_id) VALUES(?,?,?,?)", kind, id, document, actor); err != nil {
		return err
	}
	// 与归档事实在同一事务推进修订，挡住并发旧输入；原业务状态和授权保持原样。
	_, err := tx.ExecContext(ctx, "UPDATE "+table+" SET revision=revision+1 WHERE id=?", id)
	return err
}
