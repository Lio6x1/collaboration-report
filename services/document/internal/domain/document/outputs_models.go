// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"time"
)

// edit_session.go：保留原实现，按业务合并文件。

// 支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。
type EditWorkspace struct {
	Workspace, Epoch, Lineage, BaseManifest, BaseVersion uint64
}
type EditingAdmission struct {
	Actor                  uint64
	SessionRef, TargetJSON []byte
	State                  string
	Expires                time.Time
}
type EditingTicket struct {
	Admission              EditingAdmission
	Purpose, Client, State string
	Expires                time.Time
}

// fence.go：保留原实现，按业务合并文件。

// FenceState保存本服务已经持久登记的关闭绑定，固定来源和代次不能被同号新请求替换。
type FenceState struct {
	Scope            string
	Epoch            uint64
	State            string
	TargetPermission []byte
}
type PersistedOutput struct {
	ID      string
	Payload []byte
}

// output.go：保留原实现，按业务合并文件。

// 输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。
type OutputInstance struct {
	ID   string `json:"instance_id"`
	PID  uint64 `json:"pid"`
	Boot string `json:"boot_digest"`
}
type LeaseSource struct {
	Key   string `json:"scope_key"`
	Epoch string `json:"epoch"`
}
type OutputLease struct {
	ID        string        `json:"lease_id"`
	Instance  string        `json:"instance_id"`
	Handler   string        `json:"handler_id"`
	Sources   []LeaseSource `json:"sources"`
	Document  uint64        `json:"document_id,omitempty"`
	Admission string        `json:"admission_id,omitempty"`
	Authority string        `json:"authority,omitempty"`
	// 正式输出没有虚构admission；十进制字符串避免Edge的JSON数字丢失完整uint64。
	PublicationEpoch string `json:"publication_epoch,omitempty"`
	Release          string `json:"release_id,omitempty"`
}

// CollaborationOutput引用已兑换准入，调用方只提交固定绑定，不能自定权限来源。
type CollaborationOutput struct {
	Admission, Grant, Handler string
}
