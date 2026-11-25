# services/account/internal/repository/account/notifications_queries.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/052-notifications-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`LockDepartureOrganization`、`LockDepartureMember`、`LockDepartureReceipt`、`NotificationAdministrators`、`NotificationRecipientUser`、`NotificationRecipientOrg`、`NotificationRecipientMember`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`b12294e40a38e88db98f25f34a8885ec7d3c3084ea446328a7c4a16644d063b1`。[参考实现](../../../../services/account/internal/repository/account/notifications_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
