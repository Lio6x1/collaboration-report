// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package event

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"
)

// PartitionCount 属于当前日志身份和重建前沿 v1 协议，不能当作可随意修改的容量参数。
// 扩分区必须先设计前沿版本/检查点迁移，不能只调整 broker 分区数或这个常量。
const PartitionCount = 3

// event.go：保留原实现，按业务合并文件。

// Event只保存outbox公开引用及原内容摘要。消息批次/投递时间不属于业务身份，
// Doc后来返回的当前权威摘要必须另存，不能覆盖RawDigest来伪造原事件一致性。
type Event struct {
	ID               string
	Type             string
	Document         uint64
	Sequence         uint64
	Release          uint64
	PublicationEpoch uint64
	OccurredAt       time.Time
	RawDigest        [32]byte
	Original         []byte
}

// json.go：保留原实现，按业务合并文件。

// encoding/json默认接受重复键、忽略未知字段。事件身份不能依赖“后一个值覆盖前一个”，
// 所以先检查完整JSON及重复键，再按冻结字段解码；所有错误只返回固定码。
func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := checkValue(decoder, 0); err != nil {
		return errors.New("CANAL_JSON_INVALID")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("CANAL_JSON_INVALID")
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return errors.New("CANAL_FIELDS_INVALID")
	}
	return nil
}

func checkValue(decoder *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("CANAL_JSON_DEPTH_EXCEEDED")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			name, valid := key.(string)
			if err != nil || !valid || seen[name] {
				return errors.New("CANAL_JSON_DUPLICATE_KEY")
			}
			seen[name] = true
			if err = checkValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err = checkValue(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("CANAL_JSON_INVALID")
	}
	_, err = decoder.Token()
	return err
}

// record.go：保留原实现，按业务合并文件。

// Kafka位置属于有符号协议offset，业务文档/发布/序号仍使用Event中的全uint64。
// 传输记录模型隔离第三方Kafka客户端，业务用例不持有其Reader或Message。
type Position struct {
	Topic     string
	Partition int
	Offset    int64
}

type Record struct {
	Position
	Value []byte
}

type PartitionWindow struct {
	First     int64
	Last      int64
	Committed int64
}
