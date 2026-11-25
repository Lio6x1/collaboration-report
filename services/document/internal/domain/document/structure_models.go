// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// main_control.go：保留原实现，按业务合并文件。

// MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。
type MainControlIntent struct {
	Operation string               `json:"operation"`
	Kind      string               `json:"kind"`
	Control   uint64               `json:"control"`
	Style     uint64               `json:"style"`
	Locked    bool                 `json:"locked"`
	StyleJSON json.RawMessage      `json:"style_json,omitempty"`
	Chapter   *dto.ChapterCreate   `json:"chapter,omitempty"`
	Structure *dto.StructureChange `json:"structure,omitempty"`
}

func CanonicalRevision(value string) (uint64, error) {
	revision, err := strconv.ParseUint(value, 10, 64)
	if err != nil || TextID(revision) != value {
		return 0, Violation(InvalidRequest, "REVISION_INVALID")
	}
	return revision, nil
}

func ParseMainLock(input dto.MainLock) (MainControlIntent, error) {
	intent := MainControlIntent{Operation: input.OperationId, Kind: "MAIN_LOCK", Locked: input.Locked}
	if !ValidOperationID(intent.Operation) {
		return intent, Violation(InvalidRequest, "MAIN_CONTROL_INPUT_INVALID")
	}
	var err error
	intent.Control, err = CanonicalRevision(input.ExpectedControlRevision)
	return intent, err
}

func ParseStyleChange(input dto.StyleChange) (MainControlIntent, error) {
	intent := MainControlIntent{Operation: input.OperationId, Kind: "STYLE_CHANGE"}
	if !ValidOperationID(intent.Operation) {
		return intent, Violation(InvalidRequest, "MAIN_CONTROL_INPUT_INVALID")
	}
	var err error
	if intent.Control, err = CanonicalRevision(input.ExpectedControlRevision); err != nil {
		return intent, err
	}
	if intent.Style, err = CanonicalID(input.ExpectedStyleId); err != nil {
		return intent, err
	}
	raw, err := json.Marshal(input.Style)
	if err != nil {
		return intent, err
	}
	value, err := codec.Decode(raw)
	if err != nil {
		return intent, err
	}
	// HTTP入口已经按冻结StyleChange schema核验字段；固定规范字节用于不可变样式与同号绑定。
	intent.StyleJSON, err = codec.Canonical(value)
	return intent, err
}

func ValidateMainControl(permissions Permissions, control, style string, locked bool, intent MainControlIntent) error {
	if !permissions.Manage {
		return Violation(PermissionDenied, "MAIN_CONTROL_NOT_ALLOWED")
	}
	if control != TextID(intent.Control) {
		return Violation(StateConflict, "MAIN_CONTROL_CHANGED")
	}
	if intent.Kind == "STYLE_CHANGE" && style != TextID(intent.Style) {
		return Violation(StateConflict, "MAIN_STYLE_CHANGED")
	}
	if (intent.Kind == "STYLE_CHANGE" || intent.Kind == "STRUCTURE_CHANGE") && locked {
		return Violation(StateConflict, "MAIN_LOCKED")
	}
	return nil
}

// main_workspace.go：保留原实现，按业务合并文件。

// MainWorkspaceFact只保存原安装的标量证据，不携带正文。管理元数据修订和
// 物理MAIN代次可以不同；只有改变MAIN并完成实际安装，才产生新的物理代次。
type MainWorkspaceFact struct {
	Manifest, Structure, Style, MetadataControl                 uint64
	Product                                                     string
	Parent, Fixed, Succeeded, Installed, OriginalMatched        bool
	RecordedManifest, RecordedStructure, RecordedStyle, Control uint64
}

func ResolveMainWorkspace(fact MainWorkspaceFact) (uint64, error) {
	unknown := Violation(TemporarilyUnavailable, "MAIN_WORKSPACE_BINDING_UNKNOWN")
	if fact.Manifest == 0 || fact.Structure == 0 || fact.Style == 0 || fact.MetadataControl == 0 {
		return 0, unknown
	}
	if fact.Product == "INITIAL" {
		// INITIAL来自文档创建的固定空基线；不能把别的缺失安装事实伪装成这个分支。
		if fact.Parent || fact.Fixed || fact.Control != 0 {
			return 0, unknown
		}
		return 1, nil
	}
	switch fact.Product {
	case "MAIN_LOCK", "STYLE_CHANGE", "CHAPTER_CREATE", "STRUCTURE_CHANGE", "MERGE":
	default:
		return 0, unknown
	}
	if !fact.Parent || !fact.Fixed || !fact.Succeeded || !fact.Installed || !fact.OriginalMatched ||
		fact.RecordedManifest != fact.Manifest || fact.RecordedStructure != fact.Structure || fact.RecordedStyle != fact.Style ||
		fact.Control == 0 || fact.Control > fact.MetadataControl {
		return 0, unknown
	}
	return fact.Control, nil
}

// structure.go：保留原实现，按业务合并文件。

func ParseStructureChange(input dto.StructureChange) (MainControlIntent, error) {
	return parseStructureChange(input, false)
}

func parseStructureChange(input dto.StructureChange, preview bool) (MainControlIntent, error) {
	intent := MainControlIntent{Operation: input.OperationId, Kind: "STRUCTURE_CHANGE", Structure: &input}
	if !ValidOperationID(input.OperationId) || len(input.Changes) == 0 || len(input.Changes) > 100 {
		return intent, Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
	}
	if _, err := CanonicalID(input.ExpectedStructureId); err != nil {
		return intent, err
	}
	var err error
	if intent.Control, err = CanonicalRevision(input.ExpectedControlRevision); err != nil {
		return intent, err
	}
	for _, change := range input.Changes {
		if _, err = CanonicalID(change.ChapterId); err != nil {
			return intent, err
		}
		switch change.Type {
		case "RENAME":
			if change.Title == nil || strings.TrimSpace(*change.Title) == "" || !utf8.ValidString(*change.Title) || utf8.RuneCountInString(*change.Title) > 240 || change.AfterChapterId != nil || change.PreviewCutRef != nil {
				return intent, Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
			}
		case "MOVE":
			if change.Title != nil || change.PreviewCutRef != nil {
				return intent, Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
			}
			if change.AfterChapterId != nil {
				if _, err = CanonicalID(*change.AfterChapterId); err != nil {
					return intent, err
				}
				if *change.AfterChapterId == change.ChapterId {
					return intent, Violation(InvalidRequest, "CHAPTER_SELF_ANCHOR")
				}
			}
		case "SPLIT_LEVEL2", "JOIN_CHAPTER", "REMOVE":
			if change.Title != nil || change.AfterChapterId != nil || (preview && change.PreviewCutRef != nil) {
				return intent, Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
			}
			if !preview && (change.PreviewCutRef == nil || !ValidOperationID(*change.PreviewCutRef)) {
				return intent, Violation(StateConflict, "STRUCTURE_PREVIEW_REQUIRED")
			}
		default:
			return intent, Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
		}
	}
	if !preview && HasStructureRangeChange(input) {
		if _, err := StructureConfirmationReference(input); err != nil {
			return intent, err
		}
	}
	return intent, nil
}

// 纯目录动作移动完整章组。复制条目后修改，不写穿不可变历史，也不改变孩子的局部顺序和单元身份。
func ApplyStableStructureChanges(entries []dto.StructureEntry, input dto.StructureChange) ([]dto.StructureEntry, error) {
	if HasStructureRangeChange(input) {
		return nil, Violation(StateConflict, "STRUCTURE_PREVIEW_REQUIRED")
	}
	if _, err := ParseStructureChange(input); err != nil {
		return nil, err
	}
	groups := make(map[string][]dto.StructureEntry)
	roots := make([]dto.StructureEntry, 0)
	for _, entry := range entries {
		if entry.ParentNodeId == nil && entry.Kind == "CHAPTER" {
			if _, exists := groups[entry.ChapterId]; exists {
				return nil, Violation(StateConflict, "STRUCTURE_SHAPE_UNKNOWN")
			}
			groups[entry.ChapterId] = []dto.StructureEntry{entry}
			roots = append(roots, entry)
		}
	}
	for _, entry := range entries {
		if entry.ParentNodeId == nil && entry.Kind == "CHAPTER" {
			continue
		}
		group, exists := groups[entry.ChapterId]
		if !exists || entry.ParentNodeId == nil || *entry.ParentNodeId != group[0].NodeId {
			return nil, Violation(StateConflict, "STRUCTURE_SHAPE_UNKNOWN")
		}
		groups[entry.ChapterId] = append(group, entry)
	}
	sort.SliceStable(roots, func(i, j int) bool { return roots[i].SortOrder < roots[j].SortOrder })
	order := make([]string, len(roots))
	for i, root := range roots {
		order[i] = root.ChapterId
	}
	for _, change := range input.Changes {
		group, exists := groups[change.ChapterId]
		if !exists {
			return nil, Violation(StateConflict, "CHAPTER_CHANGED")
		}
		if change.Type == "RENAME" {
			group[0].Title = *change.Title
			continue
		}
		next := make([]string, 0, len(order))
		for _, id := range order {
			if id != change.ChapterId {
				next = append(next, id)
			}
		}
		// 无after锚点表示放到首位；有锚点则紧随该完整章组。
		index := 0
		if change.AfterChapterId != nil {
			index = -1
			for i, id := range next {
				if id == *change.AfterChapterId {
					index = i + 1
					break
				}
			}
			if index < 0 {
				return nil, Violation(StateConflict, "CHAPTER_ANCHOR_CHANGED")
			}
		}
		next = append(next, "")
		copy(next[index+1:], next[index:])
		next[index] = change.ChapterId
		order = next
	}
	result := make([]dto.StructureEntry, 0, len(entries))
	for i, id := range order {
		group := groups[id]
		group[0].SortOrder = int64(i + 1)
		sort.SliceStable(group[1:], func(a, b int) bool { return group[a+1].SortOrder < group[b+1].SortOrder })
		result = append(result, group...)
	}
	return result, nil
}

// structure_confirmation.go：保留原实现，按业务合并文件。

func HasStructureRangeChange(input dto.StructureChange) bool {
	for _, change := range input.Changes {
		if change.Type == "SPLIT_LEVEL2" || change.Type == "JOIN_CHAPTER" || change.Type == "REMOVE" {
			return true
		}
	}
	return false
}

// 全部范围动作共享同一份完整预览；不能拼接多个操作各自看见的目录、正文与切点。
func StructureConfirmationReference(input dto.StructureChange) (string, error) {
	if !ValidOperationID(input.OperationId) {
		return "", Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
	}
	reference := ""
	for _, change := range input.Changes {
		switch change.Type {
		case "SPLIT_LEVEL2", "JOIN_CHAPTER", "REMOVE":
			if change.PreviewCutRef == nil || !ValidOperationID(*change.PreviewCutRef) {
				return "", Violation(StateConflict, "STRUCTURE_PREVIEW_REQUIRED")
			}
			if reference != "" && reference != *change.PreviewCutRef {
				return "", Violation(StateConflict, "STRUCTURE_PREVIEW_MISMATCH")
			}
			reference = *change.PreviewCutRef
		default:
			if change.PreviewCutRef != nil {
				return "", Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
			}
		}
	}
	if reference == "" {
		return "", Violation(StateConflict, "STRUCTURE_PREVIEW_REQUIRED")
	}
	return reference, nil
}

// 只去掉确认入口自己新增的原号/引用；动作集合、顺序、目录标题与期待修订均必须等于原预览。
func ValidateStructureConfirmation(input dto.StructureChange, preview dto.StructurePreviewCreate) error {
	reference, err := StructureConfirmationReference(input)
	if err != nil {
		return err
	}
	if reference != preview.OperationId || input.OperationId == preview.OperationId {
		return Violation(StateConflict, "STRUCTURE_PREVIEW_MISMATCH")
	}
	if _, err = ParseStructurePreview(preview); err != nil {
		return Violation(TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	normalized := dto.StructurePreviewCreate{
		OperationId:             preview.OperationId,
		ExpectedStructureId:     input.ExpectedStructureId,
		ExpectedControlRevision: input.ExpectedControlRevision,
		Changes:                 slices.Clone(input.Changes),
	}
	for i := range normalized.Changes {
		normalized.Changes[i].PreviewCutRef = nil
	}
	if _, err = ParseStructurePreview(normalized); err != nil {
		return err
	}
	original, err := json.Marshal(preview)
	if err != nil {
		return err
	}
	confirmed, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	if !bytes.Equal(original, confirmed) {
		return Violation(StateConflict, "STRUCTURE_PREVIEW_MISMATCH")
	}
	return nil
}

// structure_migration.go：保留原实现，按业务合并文件。

// 这是系统内部固定候选与范围证据，不复制浏览器HTTP输入，也不预先分配正式单元身份。
type StructureMigration struct {
	Schema  string                     `json:"schema_version"`
	Changes []StructureMigrationChange `json:"changes"`
}
type StructureMigrationChange struct {
	Type    string                     `json:"type"`
	Chapter string                     `json:"chapter_id"`
	Sources []string                   `json:"source_units"`
	Targets []StructureMigrationTarget `json:"targets"`
}
type StructureMigrationTarget struct {
	Key      string                     `json:"key"`
	Kind     string                     `json:"kind"`
	Title    string                     `json:"title"`
	Main     json.RawMessage            `json:"main_body"`
	Draft    json.RawMessage            `json:"draft_body"`
	Boundary StructureMigrationBoundary `json:"boundary"`
	Ranges   []StructureMigrationRange  `json:"source_ranges"`
}
type StructureMigrationBoundary struct {
	Heading    json.RawMessage `json:"heading,omitempty"`
	EmptyBlock string          `json:"empty_block_id,omitempty"`
}
type StructureMigrationRange struct {
	Source      string   `json:"source_unit_id"`
	Scope       string   `json:"source_scope"`
	Start       *string  `json:"start_boundary"`
	End         *string  `json:"end_boundary"`
	MainBlocks  []string `json:"main_block_ids"`
	DraftBlocks []string `json:"draft_block_ids"`
	TargetScope string   `json:"target_scope,omitempty"`
	TargetStart *string  `json:"target_start_boundary"`
	TargetEnd   *string  `json:"target_end_boundary"`
}
type StructureMigrationSource struct{ Unit, Chapter uint64 }

func ParseStructureMigration(raw []byte, input dto.StructurePreviewCreate, sources []StructureMigrationSource, validator *codec.Validator) (StructureMigration, error) {
	var migration StructureMigration
	invalid := Violation(TemporarilyUnavailable, "STRUCTURE_MIGRATION_UNKNOWN")
	if validator == nil || len(raw) > 24<<20 {
		return migration, invalid
	}
	if _, err := codec.Decode(raw); err != nil {
		return migration, invalid
	}
	if json.Unmarshal(raw, &migration) != nil || migration.Schema != "structure-migration-v1" {
		return migration, invalid
	}
	chapters := map[string]map[string]bool{}
	seen := map[uint64]bool{}
	for _, source := range sources {
		if source.Unit == 0 || source.Chapter == 0 || seen[source.Unit] {
			return migration, invalid
		}
		seen[source.Unit] = true
		chapter := TextID(source.Chapter)
		if chapters[chapter] == nil {
			chapters[chapter] = map[string]bool{}
		}
		chapters[chapter][TextID(source.Unit)] = true
	}
	index := 0
	keys := map[string]bool{}
	for _, intent := range input.Changes {
		if intent.Type != "SPLIT_LEVEL2" && intent.Type != "JOIN_CHAPTER" && intent.Type != "REMOVE" {
			continue
		}
		if index >= len(migration.Changes) {
			return migration, invalid
		}
		change := &migration.Changes[index]
		index++
		if change.Type != intent.Type || change.Chapter != intent.ChapterId || !exactMigrationSources(change.Sources, chapters[change.Chapter]) {
			return migration, invalid
		}
		if change.Type == "REMOVE" {
			if len(change.Targets) != 0 {
				return migration, invalid
			}
			continue
		}
		if len(change.Targets) == 0 || len(change.Targets) > 10000 || (change.Type == "SPLIT_LEVEL2" && len(change.Sources) != 1) || (change.Type == "JOIN_CHAPTER" && len(change.Targets) != 1) {
			return migration, invalid
		}
		sections := 0
		for i := range change.Targets {
			target := &change.Targets[i]
			if target.Key == "" || len(target.Key) > 160 || keys[target.Key] || !utf8.ValidString(target.Title) || utf8.RuneCountInString(target.Title) > 240 {
				return migration, invalid
			}
			keys[target.Key] = true
			var err error
			if target.Main, err = canonicalStructureCandidateBody(validator, target.Main, false); err != nil {
				return migration, invalid
			}
			if target.Draft, err = canonicalStructureCandidateBody(validator, target.Draft, true); err != nil {
				return migration, invalid
			}
			if change.Type == "JOIN_CHAPTER" {
				if target.Kind != "CHAPTER" || len(target.Boundary.Heading) != 0 || !exactMigrationRanges(target.Ranges, change.Sources, true) || !validJoinedTargetIntervals(target) {
					return migration, invalid
				}
				continue
			}
			if !exactMigrationRanges(target.Ranges, change.Sources, false) {
				return migration, invalid
			}
			if target.Kind == "INTRODUCTION" {
				if i != 0 || target.Title != "" || len(target.Boundary.Heading) != 0 || target.Ranges[0].Start != nil {
					return migration, invalid
				}
			} else if target.Kind == "SECTION" {
				id, err := structureMigrationHeadingID(validator, target.Boundary.Heading)
				if err != nil || target.Title == "" || target.Ranges[0].Start == nil || *target.Ranges[0].Start != id {
					return migration, invalid
				}
				sections++
			} else {
				return migration, invalid
			}
			if i > 0 {
				previous := change.Targets[i-1].Ranges[0].End
				if previous == nil || target.Ranges[0].Start == nil || *previous != *target.Ranges[0].Start {
					return migration, invalid
				}
			}
		}
		if change.Type == "SPLIT_LEVEL2" && (sections == 0 || change.Targets[len(change.Targets)-1].Ranges[0].End != nil) {
			return migration, invalid
		}
	}
	if index == 0 || index != len(migration.Changes) {
		return migration, invalid
	}
	return migration, nil
}

func validJoinedTargetIntervals(target *StructureMigrationTarget) bool {
	// 首轮旧固定候选没有目标区间：允许沿原事实完成安装，历史读权限只能要求整个目标覆盖。
	withInterval := false
	for _, segment := range target.Ranges {
		if segment.TargetScope != "" {
			withInterval = true
		}
	}
	if !withInterval {
		return true
	}
	heads, ok := StructureTopLevel2IDs(target.Main)
	if !ok || len(heads) == 0 {
		return false
	}
	previous := -2
	for i, segment := range target.Ranges {
		if segment.TargetScope != "INTERVAL" {
			return false
		}
		start, ok := historicalBoundaryIndex(heads, segment.TargetStart, false)
		if !ok || start <= previous || (i > 0 && segment.TargetStart == nil) || (i == 0 && start > 0) {
			return false
		}
		previous = start
		if i == len(target.Ranges)-1 {
			if segment.TargetEnd != nil {
				return false
			}
		} else if segment.TargetEnd == nil || !equalMigrationBoundary(segment.TargetEnd, target.Ranges[i+1].TargetStart) {
			return false
		}
	}
	return true
}

func equalMigrationBoundary(a, b *string) bool { return a != nil && b != nil && *a == *b }

// 授权区间只读取不可变整份MAIN中的顶层二级标题身份，拒绝重复和不完整的正文。
func StructureTopLevel2IDs(raw []byte) ([]string, bool) {
	value, err := codec.Decode(raw)
	if err != nil {
		return nil, false
	}
	root, ok := value.(map[string]any)
	if !ok || root["type"] != "doc" {
		return nil, false
	}
	heads, seen := []string{}, map[string]bool{}
	for _, item := range structureMigrationArray(root["content"]) {
		node, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		attrs, _ := node["attrs"].(map[string]any)
		if node["type"] != "heading" || attrs["level"] != json.Number("2") {
			continue
		}
		id, ok := attrs["id"].(string)
		if !ok || id == "" || seen[id] {
			return nil, false
		}
		seen[id] = true
		heads = append(heads, id)
	}
	return heads, true
}

func exactMigrationSources(items []string, expected map[string]bool) bool {
	if len(items) == 0 || len(items) != len(expected) {
		return false
	}
	seen := map[string]bool{}
	for _, id := range items {
		if !expected[id] || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func exactMigrationRanges(ranges []StructureMigrationRange, sources []string, joining bool) bool {
	if len(ranges) != len(sources) {
		return false
	}
	for i, segment := range ranges {
		// WHOLE_UNIT绑定被固定的原完整来源；start/end界定这条迁移边的区间，不能把当前块列表当全覆盖。
		if segment.Source != sources[i] || segment.Scope != "WHOLE_UNIT" || (joining && (segment.Start != nil || segment.End != nil)) {
			return false
		}
	}
	return true
}

func structureMigrationHeadingID(validator *codec.Validator, raw []byte) (string, error) {
	invalid := Violation(TemporarilyUnavailable, "STRUCTURE_MIGRATION_UNKNOWN")
	value, err := codec.Decode(raw)
	if err != nil {
		return "", invalid
	}
	node, ok := value.(map[string]any)
	if !ok || node["type"] != "heading" {
		return "", invalid
	}
	attrs, ok := node["attrs"].(map[string]any)
	if !ok || attrs["level"] != json.Number("2") || validator.Body(map[string]any{"type": "doc", "content": []any{node}}) != nil {
		return "", invalid
	}
	id, ok := attrs["id"].(string)
	if !ok {
		return "", invalid
	}
	return id, nil
}

func canonicalStructureCandidateBody(validator *codec.Validator, raw []byte, branch bool) ([]byte, error) {
	invalid := Violation(TemporarilyUnavailable, "STRUCTURE_MIGRATION_UNKNOWN")
	value, err := codec.Decode(raw)
	if err != nil || validator.Body(value) != nil {
		return nil, invalid
	}
	count, blocks := 0, map[string]bool{}
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
				id, ok := attrs["resourceId"].(string)
				if !ok {
					return false
				}
				if _, err := CanonicalID(id); err != nil {
					return false
				}
			}
		}
		if branch {
			for _, mark := range structureMigrationArray(node["marks"]) {
				if mark.(map[string]any)["type"] == "font" {
					return false
				}
			}
		}
		for _, child := range structureMigrationArray(node["content"]) {
			if !visit(child.(map[string]any), depth+1) {
				return false
			}
		}
		return true
	}
	if !visit(value.(map[string]any), 0) {
		return nil, invalid
	}
	canonical, err := codec.Canonical(value)
	if err != nil || len(canonical) > 16<<20 {
		return nil, invalid
	}
	return canonical, nil
}

func structureMigrationArray(value any) []any { result, _ := value.([]any); return result }

// structure_preparation.go：保留原实现，按业务合并文件。

// 只缩小原固定cut的支线范围。虚拟支线不产生假room cut，但其入口仍由Node持久屏障保护。
func StructurePreparationCut(original *pb.CutVector, branchRooms, affectedRooms []uint64) (*pb.CutVector, error) {
	invalid := Violation(TemporarilyUnavailable, "STRUCTURE_PREPARATION_UNKNOWN")
	if original == nil {
		return nil, invalid
	}
	branches, affected := map[uint64]bool{}, map[uint64]bool{}
	for _, id := range branchRooms {
		if id == 0 || branches[id] {
			return nil, invalid
		}
		branches[id] = true
	}
	for _, id := range affectedRooms {
		if !branches[id] || affected[id] {
			return nil, invalid
		}
		affected[id] = true
	}
	if len(affected) == 0 {
		return nil, invalid
	}
	result := proto.Clone(original).(*pb.CutVector)
	result.Rooms = make([]*pb.RoomCut, 0, len(original.Rooms))
	seen := map[uint64]bool{}
	for _, cut := range original.Rooms {
		if cut == nil || cut.RoomId == 0 || seen[cut.RoomId] {
			return nil, invalid
		}
		seen[cut.RoomId] = true
		if !branches[cut.RoomId] || affected[cut.RoomId] {
			result.Rooms = append(result.Rooms, proto.Clone(cut).(*pb.RoomCut))
		}
	}
	raw, err := codec.Canonical(CutBinding(result)["rooms"])
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	result.DependencyDigest = digest[:]
	return result, nil
}

// structure_preview.go：保留原实现，按业务合并文件。

type StructurePreviewIntent struct {
	Operation          string
	Structure, Control uint64
	Input              dto.StructurePreviewCreate
}

func ParseStructurePreview(input dto.StructurePreviewCreate) (StructurePreviewIntent, error) {
	result := StructurePreviewIntent{Operation: input.OperationId, Input: input}
	// 共享结构输入的字段规则，保留生成DTO；预览允许声明拆合目标，但不能携带确认引用。
	change := dto.StructureChange{OperationId: input.OperationId, ExpectedStructureId: input.ExpectedStructureId, ExpectedControlRevision: input.ExpectedControlRevision, Changes: input.Changes}
	parsed, err := parseStructureChange(change, true)
	if err != nil {
		return result, err
	}
	result.Control = parsed.Control
	result.Structure, err = CanonicalID(input.ExpectedStructureId)
	return result, err
}

// structure_transition.go：保留原实现，按业务合并文件。

type StructureMigrationIdentity struct{ Unit, Node string }

// 按原HTTP动作次序变换完整章组；新unit身份由事务分配，章身份与历史条目保持不变。
func ApplyStructureMigration(original []dto.StructureEntry, input dto.StructureChange, migration StructureMigration, identities map[string]StructureMigrationIdentity) ([]dto.StructureEntry, error) {
	result := slices.Clone(original)
	changes := map[string]StructureMigrationChange{}
	for _, change := range migration.Changes {
		if _, exists := changes[change.Chapter]; exists {
			return nil, Violation(StateConflict, "STRUCTURE_RANGE_OVERLAP")
		}
		changes[change.Chapter] = change
	}
	for _, intent := range input.Changes {
		if intent.Type == "RENAME" || intent.Type == "MOVE" {
			one := input
			one.Changes = slices.Clone(input.Changes[:1])
			one.Changes[0] = intent
			var err error
			result, err = ApplyStableStructureChanges(result, one)
			if err != nil {
				return nil, err
			}
			continue
		}
		change, ok := changes[intent.ChapterId]
		if !ok || change.Type != intent.Type {
			return nil, Violation(TemporarilyUnavailable, "STRUCTURE_MIGRATION_UNKNOWN")
		}
		next := make([]dto.StructureEntry, 0, len(result)+len(change.Targets))
		found := false
		for _, entry := range result {
			if entry.ChapterId != change.Chapter {
				next = append(next, entry)
				continue
			}
			if entry.ParentNodeId != nil {
				continue
			}
			if found || entry.Kind != "CHAPTER" {
				return nil, Violation(StateConflict, "STRUCTURE_SHAPE_UNKNOWN")
			}
			found = true
			if change.Type == "REMOVE" {
				continue
			}
			root := entry
			if change.Type == "JOIN_CHAPTER" {
				if !root.SplitLevel2 || len(change.Targets) != 1 {
					return nil, Violation(StateConflict, "STRUCTURE_MODE_CHANGED")
				}
				identity := identities[change.Targets[0].Key]
				if _, err := CanonicalID(identity.Unit); err != nil {
					return nil, err
				}
				root.SplitLevel2 = false
				root.UnitId = &identity.Unit
				next = append(next, root)
				continue
			}
			if change.Type != "SPLIT_LEVEL2" || root.SplitLevel2 || root.UnitId == nil {
				return nil, Violation(StateConflict, "STRUCTURE_MODE_CHANGED")
			}
			root.SplitLevel2 = true
			root.UnitId = nil
			next = append(next, root)
			for i, target := range change.Targets {
				identity := identities[target.Key]
				if _, err := CanonicalID(identity.Unit); err != nil {
					return nil, err
				}
				if _, err := CanonicalID(identity.Node); err != nil {
					return nil, err
				}
				parent := root.NodeId
				next = append(next, dto.StructureEntry{NodeId: identity.Node, ParentNodeId: &parent, ChapterId: root.ChapterId, Kind: target.Kind, Title: target.Title, SortOrder: int64(i + 1), UnitId: &identity.Unit})
			}
		}
		if !found {
			return nil, Violation(StateConflict, "CHAPTER_CHANGED")
		}
		order := int64(0)
		for i := range next {
			if next[i].ParentNodeId == nil {
				order++
				next[i].SortOrder = order
			}
		}
		result = next
	}
	if len(result) > 10000 {
		return nil, Violation(StateConflict, "STRUCTURE_ENTRY_LIMIT")
	}
	return result, nil
}
