# services/document/internal/domain/document/reviews_models.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/101-reviews-models.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`MergeApprovalFacts`、`CanOfferMerge`、`ChangeFact`、`ValidateReviewRound`、`CanResolveChange`、`SubmissionIntent`、`ParseSubmission`、`CaptureExpectation`、`ValidateSubmissionCapture`、`canonicalValue`、`OperationFingerprint`、`ReviewGuard`、`ReviewPointer`、`SubmissionFact`、`SubmissionTask`、`SubmissionResultFact`、`ValidateSubmissionPosition`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/json"
	"sort"
	"time"
)

// approval_actions.go：保留原实现，按业务合并文件。

// 提示依据完整请求事实，与浏览器加载了多少页历史无关；实际写入仍由合并用例再次核验。
type MergeApprovalFacts struct {
	PositionReady bool
	EverSubmitted bool
	OwnOpen       uint64
	ForeignOpen   uint64
}

func CanOfferMerge(permissions Permissions, facts MergeApprovalFacts) bool {
	// 一并解除仅限本人要求，最多100项沿用ApproveMerge契约；管理例外只豁免自审身份。
	return facts.PositionReady && CanApproveMerge(permissions, facts.EverSubmitted) &&
		facts.ForeignOpen == 0 && facts.OwnOpen <= 100
}

// review.go：保留原实现，按业务合并文件。

type ChangeFact struct {
	ID, Request, Round, Author, Revision uint64
	Status                               string
}

// 动作期待的是当前固定轮次；请求修订只控制并发，不能把旧页面升级为新轮次授权。
func ValidateReviewRound(pointer ReviewPointer, round uint64) error {
	if pointer.Archived || pointer.Round != round || pointer.Round == 0 {
		return Violation(StateConflict, "REVIEW_ROUND_CHANGED")
	}
	// 撤回保留修改要求，但所有短审核动作都必须等待重新送审；解除意见也不能绕过状态门槛。
	if pointer.Status != "REVIEWING" {
		return Violation(StateConflict, "REVIEW_REQUEST_NOT_REVIEWING")
	}
	return nil
}

// 历次提交者限制针对整个请求，重提或取得 REVIEW 授权不会洗掉历史身份。
func CanResolveChange(permissions Permissions, actor uint64, change ChangeFact, everSubmitted bool) bool {
	return permissions.Manage || (permissions.Review && change.Author == actor && !everSubmitted)
}

// submission.go：保留原实现，按业务合并文件。

// SubmissionIntent是从生成union DTO解析的业务身份；保留规范公开输入，禁止浏览器提供正文或内部切点。
type SubmissionIntent struct {
	Operation, Flush, Mode                    string
	Branch, Structure, Request, ExpectedRound uint64
	Canonical                                 dto.Submission
}

func ParseSubmission(input dto.Submission) (SubmissionIntent, error) {
	var intent SubmissionIntent
	invalid := func() (SubmissionIntent, error) { return intent, Violation(InvalidRequest, "SUBMISSION_INVALID") }
	value, err := codec.Decode(input)
	if err != nil {
		return invalid()
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return invalid()
	}
	text := func(key string) string { value, _ := fields[key].(string); return value }
	intent.Operation, intent.Flush, intent.Mode = text("operation_id"), text("flush_receipt_id"), text("mode")
	if !ValidOperationID(intent.Operation) || !ValidOperationID(intent.Flush) {
		return invalid()
	}
	if intent.Mode != "NEW" && intent.Mode != "RESUBMIT" {
		return invalid()
	}
	allowed := map[string]bool{"operation_id": true, "flush_receipt_id": true, "mode": true, "branch_id": true, "expected_structure_id": true}
	if intent.Mode == "RESUBMIT" {
		allowed["request_id"], allowed["expected_round_id"] = true, true
	}
	if len(fields) != len(allowed) {
		return invalid()
	}
	for key := range fields {
		if !allowed[key] {
			return invalid()
		}
	}
	intent.Branch, err = CanonicalID(text("branch_id"))
	if err != nil {
		return invalid()
	}
	intent.Structure, err = CanonicalID(text("expected_structure_id"))
	if err != nil {
		return invalid()
	}
	if intent.Mode == "RESUBMIT" {
		intent.Request, err = CanonicalID(text("request_id"))
		if err != nil {
			return invalid()
		}
		intent.ExpectedRound, err = CanonicalID(text("expected_round_id"))
		if err != nil {
			return invalid()
		}
	}
	intent.Canonical, err = codec.Canonical(fields)
	if err != nil {
		return invalid()
	}
	return intent, nil
}

// submission_capture.go：保留原实现，按业务合并文件。

type CaptureExpectation struct {
	Request           *pb.CaptureSubmissionRequest
	Workspace         EditWorkspace
	Room              uint64
	LineageProvenance []byte
}

// Go校验固定业务JSON与来源，不解析Yjs。错误依赖结果不能被当成用户操作失败释放guard。
func ValidateSubmissionCapture(validator *codec.Validator, content *pb.FixedContent, expected CaptureExpectation) ([]uint64, error) {
	invalid := func() ([]uint64, error) { return nil, Violation(TemporarilyUnavailable, "CAPTURE_CONTENT_INVALID") }
	if content == nil || expected.Request == nil || expected.Request.Context == nil || expected.Request.Context.Target == nil || validator == nil {
		return invalid()
	}
	context, target := expected.Request.Context, expected.Request.Context.Target
	if content.DocumentId != target.DocumentId || content.UnitId != target.UnitId || content.BranchId != target.BranchId ||
		content.StructureId != target.StructureId || content.WorkspaceEpoch != target.WorkspaceEpoch || content.LineageRef != TextID(expected.Workspace.Lineage) ||
		content.OperationId != context.OperationId || content.CaptureId != context.OperationId || content.SchemaVersion != "content-v1" ||
		content.CanonicalizationVersion != "canonical-v1" || content.HashAlgorithm != "SHA-256" || content.ProvenanceSchema != "provenance-v1" ||
		len(content.BodyJsonUtf8) > 16<<20 || content.ByteSize != uint64(len(content.BodyJsonUtf8)) {
		return invalid()
	}
	digest := sha256.Sum256(content.BodyJsonUtf8)
	if !bytes.Equal(digest[:], content.BodyHash) {
		return invalid()
	}
	body, err := canonicalValue(content.BodyJsonUtf8)
	if err != nil || validator.Body(body) != nil {
		return invalid()
	}
	cutValue, err := canonicalValue(content.CutJsonUtf8)
	if err != nil {
		return invalid()
	}
	cut, ok := cutValue.(map[string]any)
	if !ok || len(cut) != 3 || cut["room_id"] != TextID(expected.Room) || cut["workspace_epoch"] != TextID(expected.Workspace.Epoch) {
		return invalid()
	}
	sequence, ok := cut["durable_seq"].(string)
	if !ok {
		return invalid()
	}
	if sequence != "0" {
		if _, err := CanonicalID(sequence); err != nil {
			return invalid()
		}
	}
	provenanceValue, err := canonicalValue(content.ProvenanceJsonUtf8)
	if err != nil || validator.Provenance(provenanceValue) != nil {
		return invalid()
	}
	provenance := provenanceValue.(map[string]any)
	if provenance["lineage_id"] != TextID(expected.Workspace.Lineage) || provenance["base_manifest_id"] != TextID(expected.Workspace.BaseManifest) ||
		provenance["base_content_version_id"] != TextID(expected.Workspace.BaseVersion) || provenance["source_operation_id"] != context.OperationId {
		return invalid()
	}
	originalValue, err := codec.Decode(expected.LineageProvenance)
	if err != nil || validator.Provenance(originalValue) != nil {
		return invalid()
	}
	original := originalValue.(map[string]any)
	accepted, err := codec.Canonical(provenance["accepted_captures"])
	if err != nil {
		return invalid()
	}
	originalAccepted, err := codec.Canonical(original["accepted_captures"])
	if err != nil || !bytes.Equal(accepted, originalAccepted) {
		return invalid()
	}
	resources, blocks := map[uint64]bool{}, map[string]bool{}
	count := 0
	var inspect func(map[string]any, int) bool
	inspect = func(node map[string]any, depth int) bool {
		count++
		if depth > 64 || count > 100000 {
			return false
		}
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["id"].(string); ok {
				if blocks[id] {
					return false
				}
				blocks[id] = true
			}
			if node["type"] == "image" {
				text, ok := attrs["resourceId"].(string)
				if !ok {
					return false
				}
				id, err := CanonicalID(text)
				if err != nil {
					return false
				}
				resources[id] = true
			}
		}
		if marks, ok := node["marks"].([]any); ok {
			for _, mark := range marks {
				if mark.(map[string]any)["type"] == "font" {
					return false
				}
			}
		}
		if children, ok := node["content"].([]any); ok {
			for _, child := range children {
				if !inspect(child.(map[string]any), depth+1) {
					return false
				}
			}
		}
		return true
	}
	if !inspect(body.(map[string]any), 0) {
		return invalid()
	}
	ids := make([]uint64, 0, len(resources))
	for id := range resources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != len(content.ResourceIds) {
		return invalid()
	}
	for index, id := range ids {
		if id != content.ResourceIds[index] {
			return invalid()
		}
	}
	return ids, nil
}

func canonicalValue(raw []byte) (any, error) {
	value, err := codec.Decode(raw)
	if err != nil {
		return nil, err
	}
	canonical, err := codec.Canonical(value)
	if err != nil || !bytes.Equal(raw, canonical) {
		return nil, codec.ErrInvalid
	}
	return value, nil
}

// OperationFingerprint复用现有receipt编码，供持久跨服务上下文使用同一个指纹。
func OperationFingerprint(path string, input any) ([32]byte, error) {
	encoded, err := json.Marshal(struct {
		Path  string
		Input any
	}{path, input})
	return sha256.Sum256(encoded), err
}

// submission_state.go：保留原实现，按业务合并文件。

type ReviewGuard struct {
	Ongoing, Resumable, Generation uint64
	Submission                     string
}

type ReviewPointer struct {
	ID, Document, Unit, Branch, Round, Revision uint64
	Status                                      string
	Archived                                    bool
}

type SubmissionFact struct {
	Operation                                                      string
	Actor, Document, Unit, Branch, GuardGeneration, Request, Round uint64
	State                                                          string
}

type SubmissionTask struct {
	ID, Generation            uint64
	Operation, Kind, Instance string
	Payload                   []byte
}

type SubmissionResultFact struct {
	State               string
	Generation          uint64
	Fingerprint, Result []byte
	Expires             time.Time
}

// 一个单元只能有一个正在审核或可续接的请求；撤回不会变成创建新请求的许可。
func ValidateSubmissionPosition(intent SubmissionIntent, guard ReviewGuard, pointer *ReviewPointer, document, unit uint64) error {
	if guard.Submission != "" || guard.Ongoing != 0 {
		return Violation(StateConflict, "UNIT_REVIEW_OCCUPIED")
	}
	if intent.Mode == "NEW" {
		if guard.Resumable != 0 {
			return Violation(StateConflict, "RESUBMIT_REQUIRED")
		}
		return nil
	}
	if guard.Resumable != intent.Request || pointer == nil || pointer.ID != intent.Request || pointer.Document != document || pointer.Unit != unit || pointer.Branch != intent.Branch || pointer.Status != "WITHDRAWN" || pointer.Archived || pointer.Round != intent.ExpectedRound {
		return Violation(StateConflict, "RESUBMIT_POSITION_CHANGED")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`60c296be00e5558f5e0b92ee96fec89c07d5107f7c0086c493db9e01290012dc`。[参考实现](../../../../services/document/internal/domain/document/reviews_models.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
