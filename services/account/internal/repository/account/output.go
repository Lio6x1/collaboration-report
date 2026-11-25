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
