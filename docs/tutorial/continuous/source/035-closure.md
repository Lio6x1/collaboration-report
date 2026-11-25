# services/account/internal/domain/account/closure.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/035-closure.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`SessionClosure`、`ClosureTask`、`TaskOwnership`、`SessionClosePreparation`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package account

// SessionClosure是持久任务的固定绑定。后续重试不能改写原scope或关闭代次。
type SessionClosure struct {
	Kind           string `json:"kind,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
	Role           string `json:"role,omitempty"`
	SessionHash    string `json:"session_hash"`
	Scope          string `json:"scope_key"`
	Epoch          string `json:"closed_epoch"`
}

type ClosureTask struct {
	ID          uint64
	OperationID string
	Payload     []byte
}

type TaskOwnership struct {
	Instance string
	State    string
}

type SessionClosePreparation struct {
	SessionID   []byte
	UserID      string
	Epoch       uint64
	OperationID string
	Scope       string
	Target      []byte
	Payload     []byte
	Fingerprint []byte
}
````

UTF-8/LF全文SHA-256：`eff554b1107859381ee747a15eaf04f205187499785a8f7bf12634624c0b4951`。[参考实现](../../../../services/account/internal/domain/account/closure.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
