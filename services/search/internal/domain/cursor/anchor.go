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
