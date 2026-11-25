// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
)

// departure_status.go：保留原实现，按业务合并文件。

// DepartureStatus不根据超时猜测准备结果；同一组织/member锁保证读取原事务的持久事实。
func (s *Organizations) DepartureStatus(ctx context.Context, organization, user, operation, expectedEpoch string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	unknown := domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "DEPARTURE_STATUS_UNKNOWN"}
	if s.repository.LockDepartureOrganization(ctx, tx, organization) != nil {
		return "", unknown
	}
	state, epoch, err := s.repository.LockDepartureMember(ctx, tx, organization, user)
	if err != nil {
		return "", unknown
	}
	err = s.repository.LockDepartureReceipt(ctx, tx, operation)
	result := "COMMITTED"
	if err == sql.ErrNoRows {
		if state == "ACTIVE" && epoch == expectedEpoch {
			result = "ROLLED_BACK"
		} else {
			result = "SUPERSEDED"
		}
	} else if err != nil {
		return "", unknown
	}
	if tx.Commit() != nil {
		return "", unknown
	}
	return result, nil
}

// notification_administrators.go：保留原实现，按业务合并文件。

func (s *NotificationRecipients) Administrators(ctx context.Context, input platform.NotificationAdministratorsRequest) (platform.NotificationAdministratorsResponse, error) {
	result := platform.NotificationAdministratorsResponse{OrganizationID: input.OrganizationID, UserIDs: []uint64{}}
	if input.OrganizationID == 0 {
		return result, domain.ErrNotificationAdministratorsInput
	}
	ids, err := repository.NotificationAdministrators(ctx, s.db, input.OrganizationID, input.After)
	if err != nil {
		// 不存在或已关闭组织可以返回空目录，数据库未知必须向上报错并重试。
		return result, err
	}
	if len(ids) > domain.NotificationAdministratorPageSize {
		ids = ids[:domain.NotificationAdministratorPageSize]
		result.NextAfter = ids[len(ids)-1]
	}
	result.UserIDs = ids
	return result, nil
}

// notification_recipients.go：保留原实现，按业务合并文件。

type NotificationRecipients struct{ db *sql.DB }

func NewNotificationRecipients(db *sql.DB) *NotificationRecipients {
	return &NotificationRecipients{db: db}
}

// Check 不创建用户准入或伪造会话。Doc仅可在短期限内锁全部返回来源后提交投递。
// 当前角色变更关闭org-member，所有者转移关闭org-owner，密码变更关闭credential；
// 将来开放账号注销/组织关闭前，必须同样无条件广播identity/成员来源关闭高水位。
func (s *NotificationRecipients) Check(ctx context.Context, input platform.NotificationRecipientsRequest) (platform.NotificationRecipientsResponse, error) {
	result := platform.NotificationRecipientsResponse{OrganizationID: input.OrganizationID, Recipients: []platform.NotificationRecipient{}}
	ids, err := domain.NotificationRecipientIDs(input.UserIDs)
	if err != nil {
		return result, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	users := make([]domain.NotificationRecipientIdentity, len(ids))
	for i, id := range ids {
		users[i], err = repository.NotificationRecipientUser(ctx, tx, id)
		if err != nil && err != sql.ErrNoRows {
			return result, err
		}
	}
	var organization domain.NotificationRecipientOrganization
	if input.OrganizationID != 0 {
		organization, err = repository.NotificationRecipientOrg(ctx, tx, input.OrganizationID)
		if err != nil && err != sql.ErrNoRows {
			return result, err
		}
	}
	for _, user := range users {
		item := platform.NotificationRecipient{UserID: user.ID, IdentityEpoch: user.IdentityEpoch, CredentialEpoch: user.CredentialEpoch, Sources: []platform.NotificationRecipientSource{}}
		item.Eligible = user.State == "ACTIVE"
		if item.Eligible && (user.IdentityEpoch == 0 || user.CredentialEpoch == 0) {
			return result, errors.New("NOTIFICATION_IDENTITY_INCONSISTENT")
		}
		if input.OrganizationID != 0 {
			member, err := repository.NotificationRecipientMember(ctx, tx, input.OrganizationID, user.ID)
			if err != nil && err != sql.ErrNoRows {
				return result, err
			}
			item.Role, item.MemberEpoch = member.Role, member.Epoch
			item.OrgOwner = organization.Owner == user.ID
			item.Eligible = item.Eligible && organization.State == "ACTIVE" && member.State == "ACTIVE"
			if item.Eligible && (member.Epoch == 0 || (member.Role != "ADMIN" && member.Role != "MEMBER")) {
				return result, errors.New("NOTIFICATION_MEMBERSHIP_INCONSISTENT")
			}
		}
		if item.Eligible {
			userID := strconv.FormatUint(user.ID, 10)
			item.Sources = append(item.Sources,
				platform.NotificationRecipientSource{ScopeKey: "identity:" + userID, Epoch: item.IdentityEpoch},
				platform.NotificationRecipientSource{ScopeKey: "credential:" + userID, Epoch: item.CredentialEpoch})
			if input.OrganizationID != 0 {
				suffix := strconv.FormatUint(input.OrganizationID, 10) + ":" + userID
				item.Sources = append(item.Sources, platform.NotificationRecipientSource{ScopeKey: "org-member:" + suffix, Epoch: item.MemberEpoch})
				if item.Role == "ADMIN" {
					item.Sources = append(item.Sources, platform.NotificationRecipientSource{ScopeKey: "org-admin:" + suffix, Epoch: item.MemberEpoch})
				}
				if item.OrgOwner {
					item.Sources = append(item.Sources, platform.NotificationRecipientSource{ScopeKey: "org-owner:" + suffix, Epoch: item.MemberEpoch})
				}
			}
		}
		result.Recipients = append(result.Recipients, item)
	}
	// 在释放事实锁前确定期限，提交或传输耗时不能重新延长快照有效期。
	result.ExpiresUnixMS = time.Now().Add(5 * time.Second).UnixMilli()
	if err := tx.Commit(); err != nil {
		return platform.NotificationRecipientsResponse{}, err
	}
	return result, nil
}
