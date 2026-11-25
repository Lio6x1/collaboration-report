# services/document/internal/domain/document/workspace_models.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/105-workspace-models.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`BranchBaselineBody`、`ParseChapterCreate`、`InsertChapterEntry`、`ContentVersionScope`、`Cursor`、`DirectorySource`、`EditableUnitFact`、`InitialContent`、`CreationKnowledgeBase`、`CanonicalJSON`、`BuildInitialContent`、`ValidName`、`Visibility`、`ValidVisibility`、`ReadView`、`FixedReadView`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

// branch_baseline.go：保留原实现，按业务合并文件。

// 只用于已核验真实MAIN版本的支线基线投影。支线字体由样式控制；正文、块ID和其他格式保持。
// Decode得到新树，绝不重写不可变MAIN版本，也不用于放宽客户端UPDATE/送审正文的校验。
func BranchBaselineBody(raw []byte) ([]byte, error) {
	value, err := codec.Decode(raw)
	if err != nil {
		return nil, err
	}
	var visit func(any)
	visit = func(value any) {
		node, ok := value.(map[string]any)
		if !ok {
			return
		}
		if marks, ok := node["marks"].([]any); ok {
			permitted := make([]any, 0, len(marks))
			for _, mark := range marks {
				if object, ok := mark.(map[string]any); ok && object["type"] == "font" {
					continue
				}
				permitted = append(permitted, mark)
			}
			if len(permitted) == 0 {
				delete(node, "marks")
			} else {
				node["marks"] = permitted
			}
		}
		if children, ok := node["content"].([]any); ok {
			for _, child := range children {
				visit(child)
			}
		}
	}
	visit(value)
	return codec.Canonical(value)
}

// chapter.go：保留原实现，按业务合并文件。

func ParseChapterCreate(input dto.ChapterCreate) (MainControlIntent, error) {
	intent := MainControlIntent{Operation: input.OperationId, Kind: "CHAPTER_CREATE", Chapter: &input}
	if !ValidOperationID(input.OperationId) || strings.TrimSpace(input.Title) == "" ||
		!utf8.ValidString(input.Title) || utf8.RuneCountInString(input.Title) > 240 {
		return intent, Violation(InvalidRequest, "CHAPTER_INPUT_INVALID")
	}
	if _, err := CanonicalID(input.ExpectedStructureId); err != nil {
		return intent, err
	}
	if input.AfterChapterId != nil {
		if _, err := CanonicalID(*input.AfterChapterId); err != nil {
			return intent, err
		}
	}
	return intent, nil
}

// 结构修订只追加新快照；一级章节移动不改变其二级孩子的局部顺序。
func InsertChapterEntry(entries []dto.StructureEntry, input dto.ChapterCreate, chapter, unit string) ([]dto.StructureEntry, error) {
	if _, err := ParseChapterCreate(input); err != nil {
		return nil, err
	}
	index, order := len(entries), int64(1)
	found := input.AfterChapterId == nil
	for i, entry := range entries {
		if entry.ParentNodeId != nil || entry.Kind != "CHAPTER" {
			continue
		}
		if input.AfterChapterId == nil {
			if order <= entry.SortOrder {
				order = entry.SortOrder + 1
			}
		} else if entry.ChapterId == *input.AfterChapterId {
			found, order, index = true, entry.SortOrder+1, i+1
			for index < len(entries) && entries[index].ChapterId == entry.ChapterId {
				index++
			}
		}
	}
	if !found {
		return nil, Violation(StateConflict, "CHAPTER_ANCHOR_CHANGED")
	}
	result := make([]dto.StructureEntry, 0, len(entries)+1)
	result = append(result, entries[:index]...)
	result = append(result, dto.StructureEntry{NodeId: chapter, ChapterId: chapter, UnitId: &unit,
		Kind: "CHAPTER", Title: input.Title, SortOrder: order})
	result = append(result, entries[index:]...)
	for i := range result {
		entry := &result[i]
		if entry.NodeId != chapter && entry.ParentNodeId == nil && entry.SortOrder >= order {
			entry.SortOrder++
		}
	}
	return result, nil
}

// content_version.go：保留原实现，按业务合并文件。

// 版本号只标识不可变正文；当前可读范围仍由文档和原单元归属决定。
type ContentVersionScope struct {
	Version, Document, Unit, Structure uint64
}

// cursor.go：保留原实现，按业务合并文件。

type Cursor struct {
	Binding string `json:"binding"`
	Last    string `json:"last"`
	Until   int64  `json:"until"`
}

// directory.go：保留原实现，按业务合并文件。

type DirectorySource struct{ ID, Epoch uint64 }

// editable.go：保留原实现，按业务合并文件。

type EditableUnitFact struct {
	ID, Title                  string
	Branch, Lifecycle, Lineage *string
	Epoch                      string
}

// initial_content.go：保留原实现，按业务合并文件。

const InitialStyle = `{"body_font":"SimSun","western_font":"Times New Roman","body_size_pt":12,"heading_sizes_pt":[22,18,16,14,12,12],"page":"A4","margin_cm":"2.54","title_font":"SimHei","title_size_pt":22}`

type InitialContent struct {
	Version, Lineage      uint64
	Canonical, Provenance []byte
	Hash                  [32]byte
}
type CreationKnowledgeBase struct {
	Space, Revision uint64
	Lifecycle       string
}

func CanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// 初始正文和provenance绑定真实版本、lineage及manifest；不使用不存在的占位编号。
func BuildInitialContent(version, lineage, unit, manifest uint64, operation string) (InitialContent, error) {
	block := "initial-" + TextID(unit)
	body := map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "attrs": map[string]string{"id": block}, "content": []any{}}}}
	canonical, err := CanonicalJSON(body)
	if err != nil {
		return InitialContent{}, err
	}
	provenance := map[string]any{"schema_version": "provenance-v1", "lineage_id": TextID(lineage), "base_manifest_id": TextID(manifest), "base_content_version_id": TextID(version), "source_operation_id": operation, "accepted_captures": []any{}, "block_sources": []any{map[string]any{"block_id": block, "origin_content_version_id": TextID(version), "origin_block_id": block, "relation": "INSERTED", "source_range": map[string]int{"start": 0, "end": 0}}}}
	raw, _ := json.Marshal(provenance)
	return InitialContent{Version: version, Lineage: lineage, Canonical: canonical, Provenance: raw, Hash: sha256.Sum256(canonical)}, nil
}

// name.go：保留原实现，按业务合并文件。

// 名称按Unicode字符限制，保留用户输入；空白名称不形成有效业务对象。
func ValidName(text string, maximum int) bool {
	return strings.TrimSpace(text) != "" && len([]rune(text)) <= maximum
}
func Visibility(value *string) string {
	if value == nil {
		return "COLLABORATORS"
	}
	return *value
}
func ValidVisibility(value string) bool {
	return value == "COLLABORATORS" || value == "SPACE_MEMBERS" || value == "PUBLIC"
}

// read_view.go：保留原实现，按业务合并文件。

type ReadView struct {
	ID                  string
	Document            uint64
	Manifest, Admission string
	Expires             time.Time
}
type FixedReadView struct {
	Manifest, Actor       uint64
	Session               []byte
	State, AdmissionState string
	Expires               time.Time
}
````

UTF-8/LF全文SHA-256：`11b53182135ae41991127181747ae3065cd220744514c73ca39ec0d556fedfd3`。[参考实现](../../../../services/document/internal/domain/document/workspace_models.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
