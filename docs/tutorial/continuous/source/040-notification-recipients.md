# services/account/internal/domain/account/notification_recipients.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/040-notification-recipients.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`NotificationRecipientIdentity`、`NotificationRecipientOrganization`、`NotificationRecipientMembership`、`NotificationRecipientIDs`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`d1299ef687c9da73fcfd55980daac9241bccd1104cdbbe3718070b8c66947edd`。[参考实现](../../../../services/account/internal/domain/account/notification_recipients.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
