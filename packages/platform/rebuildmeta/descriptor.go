package rebuildmeta

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

// Descriptor是冻结RPC内watermark字段的版本化内容，不携带私钥、证书或整库元数据。
// starts依次对应分区0、1、2，使用字符串避免JSON数字转换损失原offset。
type Descriptor struct {
	Version     int       `json:"version"`
	PrepareRef  string    `json:"prepare_ref"`
	SnapshotRef string    `json:"snapshot_ref"`
	Cluster     string    `json:"cluster_id"`
	Topic       string    `json:"topic"`
	TopicID     string    `json:"topic_id"`
	Starts      [3]string `json:"starts"`
}

var ErrDescriptor = errors.New("SEARCH_REBUILD_DESCRIPTOR_INVALID")

func referenceValid(value, kind string) bool {
	prefix := "rebuild-" + kind + "-v1:"
	return strings.HasPrefix(value, prefix) && Hex(strings.TrimPrefix(value, prefix), 32)
}

func (d Descriptor) Bytes() ([]byte, error) {
	if d.Version != 1 || !referenceValid(d.PrepareRef, "prepare") || !referenceValid(d.SnapshotRef, "snapshot") ||
		len(d.Cluster) == 0 || len(d.Cluster) > 256 || !utf8.ValidString(d.Cluster) || strings.ContainsAny(d.Cluster, "\x00\r\n\t") ||
		!topicName.MatchString(d.Topic) || !Hex(d.TopicID, 16) || d.TopicID == strings.Repeat("0", 32) {
		return nil, ErrDescriptor
	}
	for _, offset := range d.Starts {
		value, err := Decimal(offset, false)
		if err != nil || value > math.MaxInt64 {
			return nil, ErrDescriptor
		}
	}
	return json.Marshal(d)
}

func ParseDescriptor(raw []byte) (Descriptor, error) {
	var d Descriptor
	if len(raw) > 4096 || json.Unmarshal(raw, &d) != nil {
		return d, ErrDescriptor
	}
	canonical, err := d.Bytes()
	if err != nil || !bytes.Equal(canonical, raw) {
		return Descriptor{}, ErrDescriptor
	}
	return d, nil
}

func DescriptorFor(p Prepare, prepareRef, snapshotRef string) (Descriptor, error) {
	if _, err := p.Bytes(); err != nil {
		return Descriptor{}, err
	}
	d := Descriptor{Version: 1, PrepareRef: prepareRef, SnapshotRef: snapshotRef,
		Cluster: p.Cluster, Topic: p.Topic, TopicID: p.TopicID, Starts: p.Starts}
	_, err := d.Bytes()
	return d, err
}

// RequestFingerprint由Doc和Search共用。长度前缀与用途标签消除字段拼接歧义；
// Get只接受原完整请求摘要，不能仅凭可猜的operation号查询另一个重建任务。
func RequestFingerprint(operation string, generation uint64, snapshotRef string, watermark []byte) ([32]byte, error) {
	var empty [32]byte
	d, err := ParseDescriptor(watermark)
	if err != nil || !OperationValid(operation) || generation == 0 || d.SnapshotRef != snapshotRef {
		return empty, ErrDescriptor
	}
	h := sha256.New()
	h.Write([]byte("search-rebuild-request-v1\x00"))
	for _, value := range [][]byte{[]byte(operation), []byte(snapshotRef), watermark} {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		h.Write(size[:])
		h.Write(value)
	}
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], generation)
	h.Write(number[:])
	copy(empty[:], h.Sum(nil))
	return empty, nil
}
