package account

import (
	"errors"
	"sort"
)

var ErrNotificationRecipientsInput = errors.New("NOTIFICATION_RECIPIENTS_INVALID")

type NotificationRecipientIdentity struct {
	ID, IdentityEpoch, CredentialEpoch uint64
	State                              string
}

type NotificationRecipientOrganization struct {
	Owner uint64
	State string
}

type NotificationRecipientMembership struct {
	Epoch       uint64
	Role, State string
}

// 固定升序用户锁避免同一批次反向遍历；不默默去重，调用者必须核对一一对应结果。
func NotificationRecipientIDs(ids []uint64) ([]uint64, error) {
	if len(ids) == 0 || len(ids) > 100 {
		return nil, ErrNotificationRecipientsInput
	}
	ordered := append([]uint64(nil), ids...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for i, id := range ordered {
		if id == 0 || (i > 0 && id == ordered[i-1]) {
			return nil, ErrNotificationRecipientsInput
		}
	}
	return ordered, nil
}
