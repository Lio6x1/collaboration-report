# services/account/internal/domain/account/identity_models.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/038-identity-models.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`CursorPayload`、`Session`、`LoginUser`、`Credential`、`User`、`LoginResult`、`Authentication`、`RuleFailure`、`RuleViolation`、`Error`、`Mutation`、`MutationReceipt`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`80f500d992d3140fd9d2f6b60b94af6f0bd3bb703af919b837f256524d85b391`。[参考实现](../../../../services/account/internal/domain/account/identity_models.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
