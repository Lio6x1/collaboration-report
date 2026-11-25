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
