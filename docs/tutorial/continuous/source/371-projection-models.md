# services/search/internal/domain/index/projection_models.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/371-projection-models.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`FromSnapshot`、`Chapter`、`Authority`、`State`、`Watermark`、`ShouldApply`、`ExtractChapter`、`extract`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package index

import (
	"bytes"
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"strconv"
	"strings"
	"unicode/utf8"
)

// snapshot.go：保留原实现，按业务合并文件。

// FromSnapshot只消费Doc认证的当前固定版本，不读Yjs，不把旧事件release当正文选择器。
// 权威摘要覆盖完整protobuf固定事实；它与原Canal行摘要独立保存。
func FromSnapshot(validator *codec.Validator, snapshot *pb.PublicSnapshot, document, minimum uint64) (Authority, error) {
	var result Authority
	if validator == nil || snapshot == nil || document == 0 || snapshot.DocumentId != document || snapshot.AggregateSeq < minimum ||
		snapshot.PublicationEpoch == 0 || snapshot.VisibilityRevision == 0 || proto.Size(snapshot) > 32<<20 ||
		len(snapshot.ProtoReflect().GetUnknown()) != 0 || len(snapshot.Units) > 100000 {
		return result, errors.New("SEARCH_CURRENT_SNAPSHOT_INVALID")
	}
	result = Authority{Document: document, Sequence: snapshot.AggregateSeq, Release: snapshot.ReleaseId, PublicationEpoch: snapshot.PublicationEpoch,
		Structure: snapshot.StructureId, Visible: snapshot.Visible, Policy: snapshot.VisibilityPolicy, Title: snapshot.Title, Chapters: []Chapter{}}
	if result.Policy != "PUBLIC" && result.Policy != "SPACE_MEMBERS" && result.Policy != "COLLABORATORS" {
		return result, errors.New("SEARCH_CURRENT_POLICY_INVALID")
	}
	if !result.Visible {
		if result.Release != 0 || result.Structure != 0 || result.Title != "" || len(snapshot.Units) != 0 {
			return result, errors.New("SEARCH_HIDDEN_SNAPSHOT_HAS_BODY")
		}
	} else {
		if result.Release == 0 || result.Structure == 0 || result.Sequence == 0 || !utf8.ValidString(result.Title) || len(result.Title) > 1024 {
			return result, errors.New("SEARCH_VISIBLE_SNAPSHOT_INVALID")
		}
		seen := make(map[uint64]bool)
		var text strings.Builder
		var total uint64
		for _, unit := range snapshot.Units {
			if unit == nil || unit.UnitId == 0 || unit.ContentVersionId == 0 || seen[unit.UnitId] {
				return result, errors.New("SEARCH_FIXED_UNIT_INVALID")
			}
			seen[unit.UnitId] = true
			fixed := unit.Content
			if fixed == nil || fixed.DocumentId != document || fixed.UnitId != unit.UnitId || fixed.StructureId != result.Structure ||
				fixed.SchemaVersion != "content-v1" || fixed.CanonicalizationVersion != "canonical-v1" || fixed.HashAlgorithm != "SHA-256" ||
				fixed.ProvenanceSchema != "provenance-v1" || len(fixed.BodyJsonUtf8) > 16<<20 || fixed.ByteSize != uint64(len(fixed.BodyJsonUtf8)) ||
				fixed.ByteSize > 24<<20 || total > (24<<20)-fixed.ByteSize {
				return result, errors.New("SEARCH_FIXED_CONTENT_INVALID")
			}
			total += fixed.ByteSize
			body, err := codec.Decode(fixed.BodyJsonUtf8)
			if err != nil || validator.Body(body) != nil {
				return result, errors.New("SEARCH_FIXED_BODY_INVALID")
			}
			canonical, err := codec.Canonical(body)
			digest := sha256.Sum256(canonical)
			if err != nil || !bytes.Equal(canonical, fixed.BodyJsonUtf8) || !bytes.Equal(digest[:], fixed.BodyHash) {
				return result, errors.New("SEARCH_FIXED_BODY_DIGEST_INVALID")
			}
			provenance, err := codec.Decode(fixed.ProvenanceJsonUtf8)
			if err != nil || validator.Provenance(provenance) != nil {
				return result, errors.New("SEARCH_FIXED_PROVENANCE_INVALID")
			}
			canonical, err = codec.Canonical(provenance)
			if err != nil || !bytes.Equal(canonical, fixed.ProvenanceJsonUtf8) {
				return result, errors.New("SEARCH_FIXED_PROVENANCE_INVALID")
			}
			chapter, err := ExtractChapter(body, unit.UnitId, fixed.ResourceIds)
			if err != nil {
				return result, err
			}
			if text.Len()+len(chapter.Text)+1 > 24<<20 {
				return result, errors.New("SEARCH_FIXED_TEXT_LIMIT")
			}
			text.WriteString(chapter.Text)
			text.WriteByte('\n')
			result.Chapters = append(result.Chapters, chapter)
		}
		result.Text = text.String()
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(snapshot)
	if err != nil {
		return result, errors.New("SEARCH_AUTHORITY_DIGEST_INVALID")
	}
	result.Digest = sha256.Sum256(raw)
	return result, nil
}

// state.go：保留原实现，按业务合并文件。

type Chapter struct {
	Unit  uint64 `json:"unit"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Authority来自Doc当前正式快照；Visible是可索引状态，不代表当前浏览器匿名可读。
type Authority struct {
	Document         uint64
	Sequence         uint64
	Release          uint64
	PublicationEpoch uint64
	Structure        uint64
	Visible          bool
	Policy           string
	Title            string
	Text             string
	Chapters         []Chapter
	Digest           [32]byte
}

type State struct {
	Sequence uint64
	Release  uint64
	Visible  bool
	Policy   string
	Digest   [32]byte
	Revision uint64
}

// ES版本long不能承载全uint64。固定20位十进制keyword可在原子脚本中比较，
// 包括最大水位的隐藏墓碑；不能通过物理删除丢失水位后让旧事件复活正文。
func Watermark(value uint64) string { return fmt.Sprintf("%020d", value) }

func ShouldApply(current State, next Authority) (bool, error) {
	if next.Digest == ([32]byte{}) ||
		(next.Policy != "PUBLIC" && next.Policy != "SPACE_MEMBERS" && next.Policy != "COLLABORATORS") ||
		(next.Visible && next.Release == 0) || (!next.Visible && next.Release != 0) {
		return false, errors.New("SEARCH_AUTHORITY_INVALID")
	}
	if current.Digest == ([32]byte{}) {
		if current.Sequence != 0 || current.Visible || current.Release != 0 {
			return false, errors.New("SEARCH_INDEX_STATE_INVALID")
		}
		return true, nil
	}
	if next.Sequence < current.Sequence {
		return false, nil
	}
	if next.Sequence == current.Sequence {
		if next.Digest != current.Digest || next.Visible != current.Visible ||
			next.Release != current.Release || next.Policy != current.Policy {
			return false, errors.New("SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT")
		}
		return false, nil
	}
	return true, nil
}

// text.go：保留原实现，按业务合并文件。

// ExtractChapter从已通过共享content-v1验证的语义树提取纯文本。
// 正文、表格/代码、公式和图片说明可检索；链接目标、资源ID和provenance内部标识不进入摘要。
func ExtractChapter(body any, unit uint64, resourceIDs []uint64) (Chapter, error) {
	result := Chapter{Unit: unit}
	resources := make(map[uint64]bool)
	var text strings.Builder
	if err := extract(body, &text, &result.Title, resources); err != nil {
		return result, err
	}
	if len(resources) != len(resourceIDs) {
		return result, errors.New("SEARCH_FIXED_RESOURCES_INVALID")
	}
	seen := make(map[uint64]bool)
	for _, resource := range resourceIDs {
		if resource == 0 || seen[resource] || !resources[resource] {
			return result, errors.New("SEARCH_FIXED_RESOURCES_INVALID")
		}
		seen[resource] = true
	}
	result.Text = strings.TrimSpace(text.String())
	return result, nil
}

func extract(value any, text *strings.Builder, heading *string, resources map[uint64]bool) error {
	node, valid := value.(map[string]any)
	if !valid {
		return errors.New("SEARCH_FIXED_NODE_INVALID")
	}
	kind, _ := node["type"].(string)
	attrs, _ := node["attrs"].(map[string]any)
	switch kind {
	case "text":
		value, _ := node["text"].(string)
		text.WriteString(value)
	case "formula", "formula_block":
		value, _ := attrs["latex"].(string)
		text.WriteByte(' ')
		text.WriteString(value)
		text.WriteByte(' ')
	case "image":
		id, _ := attrs["resourceId"].(string)
		resource, err := strconv.ParseUint(id, 10, 64)
		if err != nil || resource == 0 || strconv.FormatUint(resource, 10) != id {
			return errors.New("SEARCH_FIXED_RESOURCES_INVALID")
		}
		resources[resource] = true
		for _, field := range []string{"alt", "title"} {
			if value, ok := attrs[field].(string); ok {
				text.WriteByte(' ')
				text.WriteString(value)
			}
		}
	case "hard_break":
		text.WriteByte('\n')
	default:
		start := text.Len()
		children, _ := node["content"].([]any)
		for _, child := range children {
			if err := extract(child, text, heading, resources); err != nil {
				return err
			}
		}
		if kind == "heading" && *heading == "" {
			*heading = strings.TrimSpace(text.String()[start:])
		}
		text.WriteByte('\n')
	}
	if text.Len() > 24<<20 {
		return errors.New("SEARCH_FIXED_TEXT_LIMIT")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`866a39240a28867583a645690a24519ad0f13381a45e1f20b053564d6dae7e0a`。[参考实现](../../../../services/search/internal/domain/index/projection_models.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
