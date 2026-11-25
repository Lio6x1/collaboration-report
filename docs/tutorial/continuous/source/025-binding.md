# packages/platform/rebuildmeta/binding.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/025-binding.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`Prepare`、`Seal`、`OperationValid`、`Decimal`、`Hex`、`Bytes`、`ParsePrepare`、`ParseSeal`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package rebuildmeta

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Prepare的整数采用字符串。原L与源/目标代次一起签名，不能靠两台机器的时间戳证明先后。
type Prepare struct {
	Version          int       `json:"version"`
	Operation        string    `json:"operation_id"`
	Owner            string    `json:"owner"`
	Alias            string    `json:"alias"`
	Source           string    `json:"source_index"`
	SourceGeneration string    `json:"source_generation"`
	SourceRevision   string    `json:"source_revision"`
	Target           string    `json:"target_index"`
	TargetGeneration string    `json:"target_generation"`
	Cluster          string    `json:"cluster_id"`
	Topic            string    `json:"topic"`
	TopicID          string    `json:"topic_id"`
	Starts           [3]string `json:"starts"`
	Nonce            string    `json:"nonce"`
}

// Seal通过完整原prepare内容地址绑定owner、operation、代次和L，避免复制两份易漂移事实。
type Seal struct {
	Version       int    `json:"version"`
	PrepareRef    string `json:"prepare_ref"`
	Count         string `json:"entry_count"`
	EntriesDigest string `json:"entries_digest"`
}

var operationName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$`)
var physicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)
var ownerName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
var topicName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$`)

func OperationValid(value string) bool { return operationName.MatchString(value) }

func Decimal(value string, positive bool) (uint64, error) {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil || strconv.FormatUint(n, 10) != value || (positive && n == 0) {
		return 0, errors.New("REBUILD_NUMBER_INVALID")
	}
	return n, nil
}

func Hex(value string, size int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == size && hex.EncodeToString(decoded) == value
}

func (p Prepare) Bytes() ([]byte, error) {
	bad := errors.New("REBUILD_PREPARE_INVALID")
	if p.Version != 1 || !OperationValid(p.Operation) || !ownerName.MatchString(p.Owner) || !physicalName.MatchString(p.Alias) || !physicalName.MatchString(p.Source) || !physicalName.MatchString(p.Target) ||
		p.Alias == p.Source || p.Alias == p.Target || p.Source == p.Target || !topicName.MatchString(p.Topic) || !Hex(p.TopicID, 16) || p.TopicID == strings.Repeat("0", 32) || !Hex(p.Nonce, 32) ||
		len(p.Cluster) == 0 || len(p.Cluster) > 256 || !utf8.ValidString(p.Cluster) || strings.ContainsAny(p.Cluster, "\x00\r\n\t") {
		return nil, bad
	}
	source, err := Decimal(p.SourceGeneration, true)
	if err != nil || source == math.MaxUint64 {
		return nil, bad
	}
	revision, err := Decimal(p.SourceRevision, true)
	if err != nil || revision == math.MaxUint64 {
		return nil, bad
	}
	target, err := Decimal(p.TargetGeneration, true)
	if err != nil || target != source+1 {
		return nil, bad
	}
	for _, offset := range p.Starts {
		n, err := Decimal(offset, false)
		if err != nil || n > math.MaxInt64 {
			return nil, bad
		}
	}
	return json.Marshal(p)
}

func ParsePrepare(raw []byte) (Prepare, error) {
	var p Prepare
	if len(raw) > 16384 || json.Unmarshal(raw, &p) != nil {
		return p, errors.New("REBUILD_PREPARE_INVALID")
	}
	canonical, err := p.Bytes()
	if err != nil || !bytes.Equal(canonical, raw) {
		return Prepare{}, errors.New("REBUILD_PREPARE_INVALID")
	}
	return p, nil
}

func (s Seal) Bytes() ([]byte, error) {
	if s.Version != 1 || !strings.HasPrefix(s.PrepareRef, "rebuild-prepare-v1:") || !Hex(strings.TrimPrefix(s.PrepareRef, "rebuild-prepare-v1:"), 32) || !Hex(s.EntriesDigest, 32) {
		return nil, errors.New("REBUILD_SEAL_INVALID")
	}
	if _, err := Decimal(s.Count, false); err != nil {
		return nil, errors.New("REBUILD_SEAL_INVALID")
	}
	return json.Marshal(s)
}

func ParseSeal(raw []byte) (Seal, error) {
	var s Seal
	if len(raw) > 4096 || json.Unmarshal(raw, &s) != nil {
		return s, errors.New("REBUILD_SEAL_INVALID")
	}
	canonical, err := s.Bytes()
	if err != nil || !bytes.Equal(canonical, raw) {
		return Seal{}, errors.New("REBUILD_SEAL_INVALID")
	}
	return s, nil
}
````

UTF-8/LF全文SHA-256：`29f8c1117c7d5e0c6b932823ebb6c6680336e34a810aa8ce5f89711442507980`。[参考实现](../../../../packages/platform/rebuildmeta/binding.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
