# services/account/internal/repository/account/output.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/055-output.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`Outputs`、`NewOutputs`、`CurrentEpoch`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package account

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

type Outputs struct{ db *sql.DB }

func NewOutputs(db *sql.DB) *Outputs { return &Outputs{db: db} }

func (r *Outputs) CurrentEpoch(ctx context.Context, key string) (string, error) {
	parts := strings.Split(key, ":")
	var value string
	var err error
	switch parts[0] {
	case "identity":
		err = r.db.QueryRowContext(ctx, "SELECT CAST(identity_epoch AS CHAR) FROM user WHERE id=? AND state='ACTIVE'", parts[1]).Scan(&value)
	case "credential":
		err = r.db.QueryRowContext(ctx, "SELECT CAST(credential_epoch AS CHAR) FROM user WHERE id=? AND state='ACTIVE'", parts[1]).Scan(&value)
	case "session":
		id, _ := hex.DecodeString(parts[1])
		err = r.db.QueryRowContext(ctx, "SELECT CAST(epoch AS CHAR) FROM session WHERE id=? AND state='ACTIVE' AND expires_at>UTC_TIMESTAMP(6)", id).Scan(&value)
	case "org-member":
		err = r.db.QueryRowContext(ctx, `
			SELECT CAST(m.epoch AS CHAR) FROM organization_member m
			JOIN organization o ON o.id=m.organization_id
			WHERE m.organization_id=? AND m.user_id=? AND m.state='ACTIVE' AND o.lifecycle='ACTIVE'`, parts[1], parts[2]).Scan(&value)
	case "org-admin":
		err = r.db.QueryRowContext(ctx, `
			SELECT CAST(m.epoch AS CHAR) FROM organization_member m
			JOIN organization o ON o.id=m.organization_id
			WHERE m.organization_id=? AND m.user_id=? AND m.state='ACTIVE'
			AND o.lifecycle='ACTIVE' AND m.role='ADMIN'`, parts[1], parts[2]).Scan(&value)
	case "org-owner":
		err = r.db.QueryRowContext(ctx, `
			SELECT CAST(m.epoch AS CHAR) FROM organization_member m
			JOIN organization o ON o.id=m.organization_id
			WHERE m.organization_id=? AND m.user_id=? AND m.state='ACTIVE'
			AND o.lifecycle='ACTIVE' AND o.owner_id=m.user_id`, parts[1], parts[2]).Scan(&value)
	default:
		return "", errors.New("SOURCE_INVALID")
	}
	return value, err
}
````

UTF-8/LF全文SHA-256：`12bc6e9c2523e83e5f75f3abcb2e59e73fc01f91a319137694fd27afcc6166ef`。[参考实现](../../../../services/account/internal/repository/account/output.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
