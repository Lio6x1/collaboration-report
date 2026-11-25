# services/document/internal/domain/document/merge_review_models.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/095-merge-review-models.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`ConflictAttemptRef`、`ConflictItemRef`、`ConflictResolutionIntent`、`ConflictResolutionFingerprint`、`ParseConflictResolution`、`ValidateConflictSet`、`DiscardChange`、`DiscardDraftIntent`、`ParseDiscardDraft`、`DiscardPreviewIntent`、`ParseDiscardPreview`、`MergeResolution`、`CommittedSubmissionCut`、`MergeIntent`、`MergeFact`、`ParseMerge`、`CanApproveMerge`、`ValidateMergePosition`、`ValidateMergeChanges`、`binaryText`、`binaryDigest`、`FixedMergeBinding`、`CutBinding`、`MergeInputDigest`、`MainManifestDigest`、`ValidateFixedContentResources`、`ReverseChangeIntent`、`ParseReverseChange`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// conflict_resolution.go：保留原实现，按业务合并文件。

// 公共引用编码真实attempt和原冲突数组序号；不改旧私有计算输入中的数据库主键。
func ConflictAttemptRef(attempt uint64) string {
	return fmt.Sprintf("merge-attempt-%020d", attempt)
}

func ConflictItemRef(attempt uint64, ordinal int) string {
	return fmt.Sprintf("merge-conflict-%020d-%04d", attempt, ordinal)
}

type ConflictResolutionIntent struct {
	Operation, Conflict string
	Attempt, Round      uint64
	Digest              []byte
	Body                []byte `json:"-"`
	Conflicts           []string
	Input               dto.ConflictResolution
}

// 原HTTP DTO中的RawMessage会被MySQL JSON重排键。正文先按冻结编码规范化，
// 再参与通用指纹；同一正文的键序不会改变原号，而不同正文仍不能复用原号。
func ConflictResolutionFingerprint(path string, intent ConflictResolutionIntent) ([32]byte, error) {
	input := intent.Input
	input.ResolutionContent = nil
	if err := json.Unmarshal(intent.Body, &input.ResolutionContent); err != nil {
		return [32]byte{}, err
	}
	return OperationFingerprint(path, input)
}

func ParseConflictResolution(conflict string, input dto.ConflictResolution) (ConflictResolutionIntent, error) {
	value := ConflictResolutionIntent{Operation: input.OperationId, Conflict: conflict, Input: input}
	invalid := Violation(InvalidRequest, "CONFLICT_RESOLUTION_INVALID")
	if !ValidOperationID(value.Operation) || !ValidOperationID(conflict) || value.Operation == conflict {
		return value, invalid
	}
	attempt, err := strconv.ParseUint(strings.TrimPrefix(input.ExpectedAttemptId, "merge-attempt-"), 10, 64)
	if err != nil || attempt == 0 || input.ExpectedAttemptId != ConflictAttemptRef(attempt) {
		return value, invalid
	}
	value.Attempt = attempt
	if value.Round, err = CanonicalID(input.ExpectedRoundId); err != nil {
		return value, invalid
	}
	value.Digest, err = hex.DecodeString(input.ExpectedInputDigest)
	if err != nil || len(value.Digest) != 32 || hex.EncodeToString(value.Digest) != input.ExpectedInputDigest {
		return value, invalid
	}
	if len(input.ResolvedConflictIds) == 0 || len(input.ResolvedConflictIds) > 10000 {
		return value, invalid
	}
	seen := map[string]bool{}
	prefix := fmt.Sprintf("merge-conflict-%020d-", attempt)
	for _, reference := range input.ResolvedConflictIds {
		ordinal, e := strconv.Atoi(strings.TrimPrefix(reference, prefix))
		if e != nil || ordinal < 1 || ordinal > 10000 || reference != ConflictItemRef(attempt, ordinal) || seen[reference] {
			return value, invalid
		}
		seen[reference] = true
		value.Conflicts = append(value.Conflicts, reference)
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	// <、>、&是合法正文文本。预算按canonical-v1字节计算，不能先用HTML转义膨胀文本。
	encoder.SetEscapeHTML(false)
	if encoder.Encode(input.ResolutionContent) != nil || len(input.ResolutionContent) == 0 {
		return value, invalid
	}
	body, err := codec.Decode(encoded.Bytes())
	if err != nil {
		return value, invalid
	}
	value.Body, err = codec.Canonical(body)
	if err != nil || len(value.Body) > 16<<20 {
		return value, invalid
	}
	return value, nil
}

// 当前真实attempt的完整集合由事务传入；语法正确的子集不能被当作已完成全部人工修复。
func (value ConflictResolutionIntent) ValidateConflictSet(count int) error {
	if count < 1 || count > 10000 || len(value.Conflicts) != count {
		return Violation(StateConflict, "CONFLICT_SET_CHANGED")
	}
	seen := map[string]bool{}
	for _, reference := range value.Conflicts {
		seen[reference] = true
	}
	for ordinal := 1; ordinal <= count; ordinal++ {
		if !seen[ConflictItemRef(value.Attempt, ordinal)] {
			return Violation(StateConflict, "CONFLICT_SET_CHANGED")
		}
	}
	return nil
}

// discard_draft.go：保留原实现，按业务合并文件。

type DiscardChange struct {
	ID, Revision uint64
	Reason       string
}

// DTO保留冻结接口；领域意图只负责规范ID、理由和集合唯一性，实际意见归属由事务核验。
type DiscardDraftIntent struct {
	Operation, Preview       string
	Branch, Structure, Round uint64
	Changes                  []DiscardChange
	Input                    dto.DiscardDraft
}

func ParseDiscardDraft(input dto.DiscardDraft) (DiscardDraftIntent, error) {
	result := DiscardDraftIntent{Operation: input.OperationId, Preview: input.PreviewCutRef, Input: input}
	if !ValidOperationID(result.Operation) || !ValidOperationID(result.Preview) || result.Operation == result.Preview {
		return result, Violation(InvalidRequest, "DISCARD_REFERENCE_INVALID")
	}
	var err error
	if result.Branch, err = CanonicalID(input.ExpectedBranchId); err != nil {
		return result, err
	}
	if result.Structure, err = CanonicalID(input.ExpectedStructureId); err != nil {
		return result, err
	}
	if input.ExpectedRoundId != nil {
		if result.Round, err = CanonicalID(*input.ExpectedRoundId); err != nil {
			return result, err
		}
	}
	seen := map[uint64]bool{}
	for _, item := range input.ChangesToResolve {
		id, e := CanonicalID(item.Id)
		if e != nil {
			return result, e
		}
		revision, e := CanonicalRevision(item.Revision)
		if e != nil {
			return result, e
		}
		if seen[id] || !ValidName(item.Reason, 2000) {
			return result, Violation(InvalidRequest, "DISCARD_CHANGES_INVALID")
		}
		seen[id] = true
		result.Changes = append(result.Changes, DiscardChange{ID: id, Revision: revision, Reason: item.Reason})
	}
	return result, nil
}

// discard_preview.go：保留原实现，按业务合并文件。

// 预览原意图只包含调用方声明的支线和结构；epoch/lineage必须由真实数据库读取，不能由浏览器补齐。
type DiscardPreviewIntent struct {
	Operation         string
	Branch, Structure uint64
	Input             dto.DiscardPreviewCreate
}

func ParseDiscardPreview(input dto.DiscardPreviewCreate) (DiscardPreviewIntent, error) {
	result := DiscardPreviewIntent{Operation: input.OperationId, Input: input}
	if !ValidOperationID(result.Operation) {
		return result, Violation(InvalidRequest, "OPERATION_ID_INVALID")
	}
	var err error
	if result.Branch, err = CanonicalID(input.ExpectedBranchId); err != nil {
		return result, err
	}
	result.Structure, err = CanonicalID(input.ExpectedStructureId)
	return result, err
}

// merge.go：保留原实现，按业务合并文件。

type MergeResolution struct {
	ID       uint64 `json:"id"`
	Revision uint64 `json:"revision"`
}

// CommittedSubmissionCut只保存既有成功送审的切点证明，不携带或延长原用户的权限。
type CommittedSubmissionCut struct {
	Operation string `json:"operation_id"`
	Version   uint64 `json:"content_version_id,string"`
	Cut       []byte `json:"cut_json_utf8"`
	BodyHash  []byte `json:"body_hash"`
}

type MergeIntent struct {
	Operation      string `json:"operation"`
	Round, Version uint64
	Resolutions    []MergeResolution `json:"resolutions"`
}

type MergeFact struct {
	Operation, State                   string
	Actor, Request, Round, Version     uint64
	Document, Unit, Branch, Generation uint64
}

func ParseMerge(input dto.ApproveMerge) (MergeIntent, error) {
	intent := MergeIntent{Operation: input.OperationId, Resolutions: []MergeResolution{}}
	if !ValidOperationID(input.OperationId) || input.OwnChangesToResolve == nil || len(input.OwnChangesToResolve) > 100 {
		return intent, Violation(InvalidRequest, "MERGE_INPUT_INVALID")
	}
	var err error
	if intent.Round, err = CanonicalID(input.RoundId); err != nil {
		return intent, err
	}
	if intent.Version, err = CanonicalID(input.ContentVersionId); err != nil {
		return intent, err
	}
	seen := map[uint64]bool{}
	for _, item := range input.OwnChangesToResolve {
		id, err := CanonicalID(item.Id)
		if err != nil {
			return intent, err
		}
		// revision是非负预期值，与必须为正的实体ID不同；零随后由意见CAS核对。
		var revision uint64
		if item.Revision != "0" {
			revision, err = CanonicalID(item.Revision)
			if err != nil {
				return intent, err
			}
		}
		if seen[id] {
			return intent, Violation(InvalidRequest, "MERGE_RESOLUTION_DUPLICATED")
		}
		seen[id] = true
		intent.Resolutions = append(intent.Resolutions, MergeResolution{ID: id, Revision: revision})
	}
	return intent, nil
}

func CanApproveMerge(permissions Permissions, everSubmitted bool) bool {
	return permissions.Manage || (permissions.Review && !everSubmitted)
}

// 首次登记与最终提交复用同一门槛；计算中不会持请求锁，因此后来撤回或换轮次仍能使提交失效。
func ValidateMergePosition(pointer ReviewPointer, guard ReviewGuard, active string, intent MergeIntent, version uint64) error {
	if err := ValidateReviewRound(pointer, intent.Round); err != nil {
		return err
	}
	if version != intent.Version {
		return Violation(StateConflict, "REVIEW_CONTENT_CHANGED")
	}
	if guard.Ongoing != pointer.ID || guard.Submission != "" {
		return Violation(StateConflict, "UNIT_REVIEW_POSITION_CHANGED")
	}
	if active != "" && active != intent.Operation {
		return Violation(StateConflict, "MERGE_OPERATION_ACTIVE")
	}
	return nil
}

func ValidateMergeChanges(actor uint64, changes []ChangeFact, resolutions []MergeResolution) error {
	wanted := map[uint64]uint64{}
	for _, item := range resolutions {
		if _, duplicate := wanted[item.ID]; duplicate {
			return Violation(InvalidRequest, "MERGE_RESOLUTION_DUPLICATED")
		}
		wanted[item.ID] = item.Revision
	}
	for _, change := range changes {
		revision, included := wanted[change.ID]
		if included {
			if change.Author != actor {
				return Violation(PermissionDenied, "MERGE_RESOLUTION_NOT_OWN")
			}
			if change.Status != "OPEN" || change.Revision != revision {
				return Violation(StateConflict, "CHANGE_REQUEST_CHANGED")
			}
			delete(wanted, change.ID)
		} else if change.Status == "OPEN" {
			return Violation(StateConflict, "CHANGES_PENDING")
		}
	}
	if len(wanted) != 0 {
		return Violation(StateConflict, "CHANGE_REQUEST_CHANGED")
	}
	return nil
}

// merge_binding.go：保留原实现，按业务合并文件。

func binaryText(value []byte) string { return base64.StdEncoding.EncodeToString(value) }
func binaryDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return binaryText(digest[:])
}

// 与Node merge_protocol使用相同显式字段和canonical-v1，不依赖proto默认值/Go JSON字段名偶然一致。
func FixedMergeBinding(content *pb.FixedContent) map[string]any {
	resources := make([]any, 0, len(content.ResourceIds))
	for _, id := range content.ResourceIds {
		resources = append(resources, TextID(id))
	}
	return map[string]any{
		"document_id":              TextID(content.DocumentId),
		"unit_id":                  TextID(content.UnitId),
		"branch_id":                TextID(content.BranchId),
		"structure_id":             TextID(content.StructureId),
		"workspace_epoch":          TextID(content.WorkspaceEpoch),
		"capture_id":               content.CaptureId,
		"operation_id":             content.OperationId,
		"lineage_ref":              content.LineageRef,
		"schema_version":           content.SchemaVersion,
		"canonicalization_version": content.CanonicalizationVersion,
		"hash_algorithm":           content.HashAlgorithm,
		"byte_size":                TextID(content.ByteSize),
		"body_hash":                binaryText(content.BodyHash),
		"cut_digest":               binaryDigest(content.CutJsonUtf8),
		"provenance_digest":        binaryDigest(content.ProvenanceJsonUtf8),
		"provenance_schema":        content.ProvenanceSchema,
		"resource_ids":             resources,
	}
}

func CutBinding(cut *pb.CutVector) map[string]any {
	rooms := append([]*pb.RoomCut(nil), cut.Rooms...)
	sort.Slice(rooms, func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId })
	values := make([]any, 0, len(rooms))
	for _, room := range rooms {
		values = append(values, map[string]any{"room_id": TextID(room.RoomId), "workspace_epoch": TextID(room.WorkspaceEpoch), "durable_seq": TextID(room.DurableSeq)})
	}
	return map[string]any{
		"document_id":       TextID(cut.DocumentId),
		"structure_id":      TextID(cut.StructureId),
		"style_id":          TextID(cut.StyleId),
		"control_revision":  TextID(cut.ControlRevision),
		"rooms":             values,
		"dependency_digest": binaryText(cut.DependencyDigest),
	}
}

func MergeInputDigest(input *pb.MergeCandidateRequest) ([32]byte, error) {
	context := input.Context
	sources := append([]*pb.SourceEpoch(nil), context.Sources...)
	sort.Slice(sources, func(i, j int) bool { return sources[i].ScopeKey < sources[j].ScopeKey })
	values := make([]any, 0, len(sources))
	for _, source := range sources {
		values = append(values, map[string]any{"scope_key": source.ScopeKey, "epoch": TextID(source.Epoch)})
	}
	target := context.Target
	binding := map[string]any{
		"attempt_id": input.AttemptId,
		"context": map[string]any{
			"operation_id":           context.OperationId,
			"actor_id":               TextID(context.ActorId),
			"identity_admission_ref": context.IdentityAdmissionRef,
			"document_admission_ref": context.DocumentAdmissionRef,
			"target": map[string]any{
				"document_id":     TextID(target.DocumentId),
				"unit_id":         TextID(target.UnitId),
				"branch_id":       TextID(target.BranchId),
				"structure_id":    TextID(target.StructureId),
				"workspace_epoch": TextID(target.WorkspaceEpoch),
				"purpose":         target.Purpose,
			},
			"sources":             values,
			"request_fingerprint": binaryText(context.RequestFingerprint),
		},
		"base":      FixedMergeBinding(input.Base),
		"submitted": FixedMergeBinding(input.Submitted),
		"main":      FixedMergeBinding(input.Main),
		"main_cut":  CutBinding(input.MainCut),
	}
	raw, err := codec.Canonical(binding)
	return sha256.Sum256(raw), err
}

func MainManifestDigest(manifest uint64, units []*pb.ManifestUnit) ([32]byte, error) {
	values := make([]any, 0, len(units))
	for _, unit := range units {
		values = append(values, map[string]any{"unit_id": TextID(unit.UnitId), "content_version_id": TextID(unit.ContentVersionId), "content": FixedMergeBinding(unit.Content)})
	}
	raw, err := codec.Canonical(map[string]any{"manifest_id": TextID(manifest), "units": values})
	return sha256.Sum256(raw), err
}

// merge_content.go：保留原实现，按业务合并文件。

// MAIN允许独立字体覆盖，但引用仍必须逐项与实际正文一致；不能相信Node仅填resource_ids列表。
func ValidateFixedContentResources(body any, content *pb.FixedContent) error {
	invalid := Violation(TemporarilyUnavailable, "MERGE_FIXED_CONTENT_INVALID")
	digest := sha256.Sum256(content.BodyJsonUtf8)
	if !bytes.Equal(digest[:], content.BodyHash) {
		return invalid
	}
	seen, blocks := map[uint64]bool{}, map[string]bool{}
	count := 0
	var visit func(map[string]any, int) bool
	visit = func(node map[string]any, depth int) bool {
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
				seen[id] = true
			}
		}
		if children, ok := node["content"].([]any); ok {
			for _, child := range children {
				value, ok := child.(map[string]any)
				if !ok || !visit(value, depth+1) {
					return false
				}
			}
		}
		return true
	}
	root, ok := body.(map[string]any)
	if !ok || !visit(root, 0) {
		return invalid
	}
	ids := make([]uint64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != len(content.ResourceIds) {
		return invalid
	}
	for i, id := range ids {
		if content.ResourceIds[i] != id {
			return invalid
		}
	}
	return nil
}

// reverse_change.go：保留原实现，按业务合并文件。

// 反向意图指定不可变合并记录和当前清单；旧请求从不回退，理由属于新操作的审计事实。
type ReverseChangeIntent struct {
	Operation, Reason string
	Manifest, Merge   uint64
	Input             dto.ReverseChange
}

func ParseReverseChange(input dto.ReverseChange) (ReverseChangeIntent, error) {
	value := ReverseChangeIntent{Operation: input.OperationId, Reason: input.Reason, Input: input}
	if !ValidOperationID(value.Operation) || !ValidName(value.Reason, 2000) {
		return value, Violation(InvalidRequest, "REVERSE_INPUT_INVALID")
	}
	var err error
	if value.Manifest, err = CanonicalID(input.ExpectedManifestId); err != nil {
		return value, err
	}
	value.Merge, err = CanonicalID(input.MergeRecordId)
	return value, err
}
````

UTF-8/LF全文SHA-256：`01aacc14018596f3d155426a7caf89301488a8954d863f5ad017f008534bb395`。[参考实现](../../../../services/document/internal/domain/document/merge_review_models.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
