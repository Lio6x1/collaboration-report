// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"time"
)

// cursor.go：保留原实现，按业务合并文件。

type CursorPayload struct {
	Binding string `json:"binding"`
	Last    string `json:"last"`
	Expires int64  `json:"expires"`
}

// model.go：保留原实现，按业务合并文件。

// Session是已持久化的会话事实；ID和CSRF只保存摘要，不包含浏览器持有的原始令牌。
type Session struct {
	ID              []byte
	UserID          string
	CSRF            []byte
	Epoch           uint64
	CredentialEpoch string
	State           string
	Expires         time.Time
}

type LoginUser struct {
	ID              string
	DisplayName     string
	Hash            []byte
	Algorithm       string
	State           string
	CredentialEpoch string
}

type Credential struct {
	Hash          []byte
	Epoch         string
	State         string
	IdentityEpoch uint64
}

type User struct {
	ID              string
	Email           string
	DisplayName     string
	State           string
	CredentialEpoch string
	IdentityEpoch   string
}

// LoginResult只在会话提交成功后交给传输层；令牌不能写入数据库或日志。
type LoginResult struct {
	UserID       string
	DisplayName  string
	SessionToken string
	CSRFToken    string
	Expires      time.Time
}

type Authentication struct {
	Session       Session
	IdentityEpoch uint64
}

// mutation.go：保留原实现，按业务合并文件。

type RuleFailure uint8

const (
	InvalidRequest RuleFailure = iota
	SessionRequired
	PermissionDenied
	EntityNotFound
	StateConflict
	TemporarilyUnavailable
)

// RuleViolation表达业务拒绝的类别，传输层决定HTTP状态或RPC错误码。
type RuleViolation struct {
	Kind RuleFailure
	Code string
}

func (e RuleViolation) Error() string { return e.Code }

type Mutation struct {
	OperationID string
	Kind        string
	Path        string
	Fingerprint []byte
	Session     Session
}

type MutationReceipt struct {
	ActorID     string
	Kind        string
	Fingerprint []byte
	Result      []byte
}
