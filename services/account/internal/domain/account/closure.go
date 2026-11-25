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
