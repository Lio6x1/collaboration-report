# services/document/internal/domain/document/resources_models.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/099-resources-models.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`DraftBodyMeasure`、`ParseDraftBodyMeasure`、`ProtectionFailureKind`、`ProtectionFailure`、`Error`、`RejectProtection`、`ProtectionDigest`、`ProtectionReceiptDigest`、`ProtectionRecord`、`ProtectionState`、`NodeUpdateProof`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// draft_body_capacity.go：保留原实现，按业务合并文件。

type DraftBodyMeasure struct {
	Bytes uint64
	Hash  []byte
}

func ParseDraftBodyMeasure(size, hash string) (DraftBodyMeasure, error) {
	bytes, err := CanonicalID(size)
	digest, decodeErr := hex.DecodeString(hash)
	if err != nil || bytes > 16<<20 || decodeErr != nil || len(digest) != 32 || hex.EncodeToString(digest) != hash {
		return DraftBodyMeasure{}, Violation(InvalidRequest, "DRAFT_BODY_MEASURE_INVALID")
	}
	return DraftBodyMeasure{Bytes: bytes, Hash: digest}, nil
}

// resource_protection.go：保留原实现，按业务合并文件。

// ProtectionFailure区分请求冲突与缺少持久证据；gRPC状态由server映射，领域不依赖RPC包。
type ProtectionFailureKind uint8

const (
	ProtectionInvalid ProtectionFailureKind = iota
	ProtectionForbidden
	ProtectionMissing
	ProtectionConflict
	ProtectionPrecondition
	ProtectionUnknown
)

type ProtectionFailure struct {
	Kind ProtectionFailureKind
	Code string
}

func (e ProtectionFailure) Error() string { return e.Code }
func RejectProtection(kind ProtectionFailureKind, code string) error {
	return ProtectionFailure{Kind: kind, Code: code}
}
func ProtectionDigest(ids []uint64) []byte {
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = TextID(id)
	}
	raw, _ := json.Marshal(values)
	sum := sha256.Sum256(raw)
	return sum[:]
}
func ProtectionReceiptDigest(operation string, generation uint64, resources []byte, state string) []byte {
	sum := sha256.Sum256([]byte(operation + "\n" + TextID(generation) + "\n" + hex.EncodeToString(resources) + "\n" + state))
	return sum[:]
}

type ProtectionRecord struct {
	Operation                         string
	Document, Room, Epoch, Generation uint64
	Grant                             string
	Fingerprint                       []byte
	ResourceJSON, SourceJSON          string
}
type ProtectionState struct {
	Fingerprint []byte
	State       string
}
type NodeUpdateProof struct {
	State      string `json:"state"`
	Seq        string `json:"durable_seq"`
	Hash       string `json:"update_hash"`
	Protection string `json:"protection_operation_id"`
	Grant      string `json:"grant_id"`
	BodyBytes  string `json:"body_bytes"`
	BodyHash   string `json:"body_hash"`
}
````

UTF-8/LF全文SHA-256：`53a4a10fa5a7918186bf129cc2a52ce202a5439b8931d34b3cdd607bbdc34aeb`。[参考实现](../../../../services/document/internal/domain/document/resources_models.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
