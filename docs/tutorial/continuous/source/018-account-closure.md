# packages/platform/account_closure.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/018-account-closure.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`AccountClosureBinding`、`ValidateAccountClosureBinding`、`EqualAccountClosureBinding`、`AccountClosureDecision`、`AccountClosureConfirmation`、`AccountClosureRetention`、`AccountClosurePreparation`、`AccountClosurePageRequest`、`AccountClosureDocument`、`AccountClosurePage`、`Matches`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package platform

import (
	"encoding/hex"
	"regexp"
	"slices"
	"strconv"
)

// AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。
// 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。
type AccountClosureBinding struct {
	Operation            string   `json:"operation_id"`
	User                 uint64   `json:"user_id"`
	IdentityEpoch        uint64   `json:"identity_epoch"`
	ClosingOrganizations []uint64 `json:"closing_organizations"`
}

var accountClosureOperation = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$`)

func ValidateAccountClosureBinding(binding AccountClosureBinding) bool {
	if !accountClosureOperation.MatchString(binding.Operation) || binding.User == 0 || binding.IdentityEpoch == 0 || binding.ClosingOrganizations == nil {
		return false
	}
	previous := uint64(0)
	for _, id := range binding.ClosingOrganizations {
		if id <= previous {
			return false
		}
		previous = id
	}
	return true
}

func EqualAccountClosureBinding(left, right AccountClosureBinding) bool {
	return left.Operation == right.Operation && left.User == right.User && left.IdentityEpoch == right.IdentityEpoch && slices.Equal(left.ClosingOrganizations, right.ClosingOrganizations)
}

type AccountClosureDecision struct {
	Rejected bool `json:"rejected"`
}

// Confirmed只证明同一固定Account计划已经冻结并持久取得全部来源ACK，
// 不证明Doc公开保留、引用释放或整个账号注销已经完成。
type AccountClosureConfirmation struct {
	Confirmed bool `json:"confirmed"`
}

// PLANNED仅确认Doc固定了保留对象并登记输出屏障，不能解释为清理或注销完成。
type AccountClosureRetention struct {
	State string `json:"state"`
}

type AccountClosurePreparation struct {
	State                  string   `json:"state"`
	RequiredKnowledgeBases []uint64 `json:"required_knowledge_bases"`
	ReservedKnowledgeBases []uint64 `json:"reserved_knowledge_bases"`
}

// 控制页只暴露固定依赖编号，不复制Doc正文。游标绑定不可变计划摘要，
// Account逐页/逐文档恢复，不能把一页已清完误当成整个账号完成。
type AccountClosurePageRequest struct {
	AccountClosureBinding
	AfterDocument string `json:"after_document"`
	PlanDigest    string `json:"plan_digest"`
}

type AccountClosureDocument struct {
	Document string `json:"document_id"`
	Release  string `json:"release_id"`
}

type AccountClosurePage struct {
	Binding    AccountClosureBinding    `json:"binding"`
	State      string                   `json:"state"`
	PlanDigest string                   `json:"plan_digest"`
	Documents  []AccountClosureDocument `json:"documents"`
	NextAfter  string                   `json:"next_after"`
	Done       bool                     `json:"done"`
}

func (p AccountClosurePage) Matches(request AccountClosurePageRequest) bool {
	digest, err := hex.DecodeString(p.PlanDigest)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != p.PlanDigest || !EqualAccountClosureBinding(p.Binding, request.AccountClosureBinding) || p.Documents == nil || len(p.Documents) > 100 || (request.PlanDigest != "" && request.PlanDigest != p.PlanDigest) || (p.State != "PLANNED" && p.State != "CLEANING" && p.State != "SUCCEEDED") {
		return false
	}
	canonical := func(text string, zero bool) (uint64, bool) {
		id, err := strconv.ParseUint(text, 10, 64)
		return id, err == nil && (zero || id != 0) && strconv.FormatUint(id, 10) == text
	}
	var previous uint64
	if request.AfterDocument != "" {
		var valid bool
		previous, valid = canonical(request.AfterDocument, false)
		if !valid || request.PlanDigest == "" {
			return false
		}
	}
	last := request.AfterDocument
	for _, document := range p.Documents {
		id, valid := canonical(document.Document, false)
		_, releaseValid := canonical(document.Release, true)
		if !valid || !releaseValid || id <= previous {
			return false
		}
		previous, last = id, document.Document
	}
	return p.NextAfter == last && (p.Done || len(p.Documents) == 100)
}
````

UTF-8/LF全文SHA-256：`1d24959fc0efd457ce3a8aff467eeb7af804b77322c4063852ba6366a0e65996`。[参考实现](../../../../packages/platform/account_closure.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
