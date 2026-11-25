# services/document/internal/domain/document/export.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/093-export.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`ExportInput`、`ParseExportInput`、`Validate`、`ExportSnapshot`、`ExportRecord`、`ExportDocument`、`ExportUnit`、`ExportAsset`、`DecodeExportSnapshot`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"encoding/json"
	"time"
)

// ExportInput 是冻结 union 的服务端判别结果，不替代生成 DTO；解码仍拒绝未知字段。
type ExportInput struct {
	Operation, Target, Format, Manifest, Release string
}

// Go生成器将oneOf保留为原始JSON；这里只判别四个冻结字段，不另造HTTP DTO。
func ParseExportInput(raw dto.ExportCreate) (ExportInput, error) {
	var value ExportInput
	decoded, err := codec.Decode([]byte(raw))
	if err != nil {
		return value, Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
	}
	object, ok := decoded.(map[string]any)
	if !ok || len(object) != 4 {
		return value, Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
	}
	value.Operation, _ = object["operation_id"].(string)
	value.Target, _ = object["target"].(string)
	value.Format, _ = object["format"].(string)
	field := "expected_manifest_id"
	if value.Target == "PUBLIC" {
		field = "expected_release_id"
	}
	id, ok := object[field].(string)
	if !ok {
		return value, Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
	}
	if value.Target == "MAIN" {
		value.Manifest = id
	} else {
		value.Release = id
	}
	return value, value.Validate()
}

func (v ExportInput) Validate() error {
	if !ValidOperationID(v.Operation) || (v.Format != "DOCX" && v.Format != "MARKDOWN" && v.Format != "PDF") {
		return Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
	}
	var id string
	switch v.Target {
	case "MAIN":
		if v.Release != "" {
			return Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
		}
		id = v.Manifest
	case "PUBLIC":
		if v.Manifest != "" {
			return Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
		}
		id = v.Release
	default:
		return Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
	}
	_, err := CanonicalID(id)
	return err
}

// 固定计划只存依赖及权威来源；正文从不可变manifest装配，RUNNING任务阻止提前清理历史。
type ExportSnapshot struct {
	Document, Space, Manifest, Structure, Style uint64
	Release, Publication                        uint64
	Title, Target, Format                       string
	Captured                                    time.Time
	Sources                                     []*pb.SourceEpoch
}

type ExportRecord struct {
	Job         dto.ExportJob
	Requester   uint64
	Operation   string
	Fingerprint [32]byte
	Snapshot    ExportSnapshot
	Generation  uint64
	ArtifactKey string
	Size        uint64
	Checksum    [32]byte
	Expires     time.Time
}

// 固定输入交给受控转换器；每种输出共享相同正文、资源与版本说明。
type ExportDocument struct {
	Title, Target, Format, Manifest, Release string `json:",omitempty"`
	Captured                                 time.Time
	Structure                                dto.StructureRevision
	Style                                    dto.StyleRevision
	Units                                    []ExportUnit
	Assets                                   []ExportAsset
}

// 转换器只消费已经完整校验的正文树；生成HTTP DTO的RawMessage字段在业务边界解码。
type ExportUnit struct {
	ID   string
	Body map[string]any
}

type ExportAsset struct {
	ID, Key, MediaType string
	Size               uint64
	Checksum           [32]byte
	Data               []byte `json:"-"`
}

func DecodeExportSnapshot(raw []byte) (ExportSnapshot, error) {
	var fixed ExportSnapshot
	if len(raw) > 128<<10 || json.Unmarshal(raw, &fixed) != nil || fixed.Document == 0 || fixed.Manifest == 0 || fixed.Structure == 0 || fixed.Style == 0 || len(fixed.Sources) == 0 || len(fixed.Sources) > 512 {
		return fixed, Violation(TemporarilyUnavailable, "EXPORT_FIXED_PLAN_INVALID")
	}
	return fixed, nil
}
````

UTF-8/LF全文SHA-256：`247ba6056bb0f74292b5ada9d721a542732c88bbffd6d8cb7ed8e786615a3319`。[参考实现](../../../../services/document/internal/domain/document/export.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
