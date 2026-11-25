// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"context"
	"database/sql"
)

// departure_status.go：保留原实现，按业务合并文件。

func (r *Organizations) LockDepartureOrganization(ctx context.Context, tx *sql.Tx, organization string) error {
	var id string
	return tx.QueryRowContext(ctx, "SELECT CAST(id AS CHAR) FROM organization WHERE id=? FOR UPDATE", organization).Scan(&id)
}

func (r *Organizations) LockDepartureMember(ctx context.Context, tx *sql.Tx, organization, user string) (string, string, error) {
	var state, epoch string
	err := tx.QueryRowContext(ctx, "SELECT state,CAST(epoch AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", organization, user).Scan(&state, &epoch)
	return state, epoch, err
}

func (r *Organizations) LockDepartureReceipt(ctx context.Context, tx *sql.Tx, operation string) error {
	var state string
	return tx.QueryRowContext(ctx, "SELECT state FROM operation_receipt WHERE operation_id=? FOR UPDATE", operation).Scan(&state)
}

// notification_administrators.go：保留原实现，按业务合并文件。

// 单条本库查询保持候选目录的语句级一致性；资格锁及撤权来源仍由后续Check负责。
// 多取一条明确告知还有下一页，避免管理员超过100人时静默遗漏通知。
func NotificationAdministrators(ctx context.Context, db *sql.DB, organization, after uint64) ([]uint64, error) {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT m.user_id
		FROM organization o
		JOIN organization_member m ON m.organization_id=o.id
		JOIN user u ON u.id=m.user_id
		WHERE o.id=? AND o.lifecycle='ACTIVE' AND m.state='ACTIVE' AND u.state='ACTIVE'
		AND (m.role='ADMIN' OR m.user_id=o.owner_id) AND m.user_id>?
		ORDER BY m.user_id LIMIT ?`, organization, after, domain.NotificationAdministratorPageSize+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uint64, 0, domain.NotificationAdministratorPageSize+1)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// notification_recipients.go：保留原实现，按业务合并文件。

// 所有资格事实来自同一事务权威行锁，SQL未知交给业务层报不可用，不能等同不存在。
func NotificationRecipientUser(ctx context.Context, tx *sql.Tx, id uint64) (domain.NotificationRecipientIdentity, error) {
	value := domain.NotificationRecipientIdentity{ID: id}
	err := tx.QueryRowContext(ctx, `SELECT state,identity_epoch,credential_epoch FROM user WHERE id=? FOR UPDATE`, id).Scan(&value.State, &value.IdentityEpoch, &value.CredentialEpoch)
	return value, err
}

func NotificationRecipientOrg(ctx context.Context, tx *sql.Tx, id uint64) (domain.NotificationRecipientOrganization, error) {
	var value domain.NotificationRecipientOrganization
	err := tx.QueryRowContext(ctx, `SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE`, id).Scan(&value.Owner, &value.State)
	return value, err
}

func NotificationRecipientMember(ctx context.Context, tx *sql.Tx, organization, user uint64) (domain.NotificationRecipientMembership, error) {
	var value domain.NotificationRecipientMembership
	err := tx.QueryRowContext(ctx, `SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user).Scan(&value.Role, &value.State, &value.Epoch)
	return value, err
}
