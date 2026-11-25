# services/search/internal/domain/cursor/anchor.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/366-anchor.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`Action`、`Batch`、`Codec`、`NewCodec`、`validateBatch`、`signature`、`Seal`、`Open`、`validAction`、`Envelope`、`ParseEnvelope`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package cursor

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
)

const (
	MaxEnvelopeBytes = 768
	MaxSortBytes     = 128
	anchorHeader     = 70
	anchorMinimum    = anchorHeader + 2 + sha256.Size
	envelopeHeader   = 7
	anchorPurpose    = "search-candidates-batch-anchor-v1\x00"
)

var (
	anchorMagic   = [4]byte{'S', 'C', 'B', 1}
	envelopeMagic = [4]byte{'S', 'C', 'E', 1}
)

type Action uint8

const (
	ActionNext   Action = 1
	ActionReplay Action = 2
)

// Batch绑定一个已经完整取得的批次，而非可随意指定的ES位置。nil位置表示首批起点。
// Query/scope/session/物理代次/绝对期限由Chain对应SQL行绑定，不重复塞进签名token。
// Digest由上层对完整有序候选计算；此层仅检查非零并签名，不能代替ES完整性验证。
type Batch struct {
	Chain  [32]byte
	Before json.RawMessage
	After  json.RawMessage
	Count  uint8
	More   bool
	Digest [32]byte
}

type Codec struct{ key []byte }

func NewCodec(key []byte) (*Codec, error) {
	if len(key) != sha256.Size {
		return nil, errors.New("SEARCH_CURSOR_KEY_INVALID")
	}
	// 全零master是公开已知密钥；固定时间比较避免按首个非零字节提前返回。
	var zero [sha256.Size]byte
	if subtle.ConstantTimeCompare(key, zero[:]) == 1 {
		return nil, errors.New("SEARCH_CURSOR_KEY_INVALID")
	}
	return &Codec{key: append([]byte(nil), key...)}, nil
}

func validateBatch(batch Batch) error {
	if batch.Chain == ([32]byte{}) || batch.Digest == ([32]byte{}) || batch.Count > 100 ||
		(batch.More && batch.Count != 100) {
		return errors.New("SEARCH_CURSOR_BATCH_INVALID")
	}
	for _, position := range []json.RawMessage{batch.Before, batch.After} {
		if len(position) != 0 {
			if err := ValidateSort(position); err != nil {
				return err
			}
		}
	}
	if (batch.Count == 0 && (!bytes.Equal(batch.Before, batch.After) || batch.More)) ||
		(batch.Count != 0 && len(batch.After) == 0) {
		return errors.New("SEARCH_CURSOR_BATCH_INVALID")
	}
	return nil
}

func (c *Codec) signature(data []byte) []byte {
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(anchorPurpose))
	mac.Write(data)
	return mac.Sum(nil)
}

func (c *Codec) Seal(batch Batch) ([]byte, error) {
	if c == nil || len(c.key) != sha256.Size {
		return nil, errors.New("SEARCH_CURSOR_KEY_INVALID")
	}
	if err := validateBatch(batch); err != nil {
		return nil, err
	}
	data := make([]byte, 0, anchorMinimum+len(batch.Before)+len(batch.After))
	data = append(data, anchorMagic[:]...)
	data = append(data, batch.Chain[:]...)
	more := byte(0)
	if batch.More {
		more = 1
	}
	data = append(data, batch.Count, more)
	data = append(data, batch.Digest[:]...)
	for _, position := range []json.RawMessage{batch.Before, batch.After} {
		data = append(data, byte(len(position)))
		data = append(data, position...)
	}
	return append(data, c.signature(data)...), nil
}

func (c *Codec) Open(raw []byte) (Batch, error) {
	invalid := errors.New("SEARCH_CURSOR_ANCHOR_INVALID")
	if c == nil || len(c.key) != sha256.Size || len(raw) < anchorMinimum ||
		len(raw) > anchorMinimum+2*MaxSortBytes || !bytes.Equal(raw[:4], anchorMagic[:]) {
		return Batch{}, invalid
	}
	data, signature := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	if !hmac.Equal(signature, c.signature(data)) || data[37] > 1 {
		return Batch{}, invalid
	}
	var batch Batch
	copy(batch.Chain[:], data[4:36])
	batch.Count, batch.More = data[36], data[37] == 1
	copy(batch.Digest[:], data[38:anchorHeader])
	offset := anchorHeader
	for _, position := range []*json.RawMessage{&batch.Before, &batch.After} {
		if offset >= len(data) {
			return Batch{}, invalid
		}
		length := int(data[offset])
		offset++
		if length > MaxSortBytes || length > len(data)-offset {
			return Batch{}, invalid
		}
		// 返回值属于调用方；不能让后续修改sort改变验签原输入或另一次回放。
		*position = append(json.RawMessage(nil), data[offset:offset+length]...)
		offset += length
	}
	if offset != len(data) || validateBatch(batch) != nil {
		return Batch{}, invalid
	}
	return batch, nil
}

func validAction(action Action) bool { return action == ActionNext || action == ActionReplay }

// 动作由受信Doc在已签锚点上选择；这里只检查有界framing，调用方随后必须Codec.Open。
// 空RPC cursor的首次建链不经过本函数，不能把无锚点请求解释为合法续接。
func Envelope(action Action, anchor []byte) ([]byte, error) {
	if !validAction(action) || len(anchor) == 0 || len(anchor) > MaxEnvelopeBytes-envelopeHeader {
		return nil, errors.New("SEARCH_CURSOR_ENVELOPE_INVALID")
	}
	raw := make([]byte, envelopeHeader, envelopeHeader+len(anchor))
	copy(raw, envelopeMagic[:])
	raw[4] = byte(action)
	binary.BigEndian.PutUint16(raw[5:7], uint16(len(anchor)))
	return append(raw, anchor...), nil
}

func ParseEnvelope(raw []byte) (Action, []byte, error) {
	if len(raw) <= envelopeHeader || len(raw) > MaxEnvelopeBytes ||
		!bytes.Equal(raw[:4], envelopeMagic[:]) || !validAction(Action(raw[4])) ||
		int(binary.BigEndian.Uint16(raw[5:7])) != len(raw)-envelopeHeader {
		return 0, nil, errors.New("SEARCH_CURSOR_ENVELOPE_INVALID")
	}
	return Action(raw[4]), append([]byte(nil), raw[envelopeHeader:]...), nil
}
````

UTF-8/LF全文SHA-256：`e5407df00cfb7cac30003709dad9a06bc99ff766ad42e4cf1bc3b39870f54433`。[参考实现](../../../../services/search/internal/domain/cursor/anchor.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
