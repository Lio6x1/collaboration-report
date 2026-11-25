# services/search/internal/client/logsource/records.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/381-records.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`decodeRecords`、`boundedRecordHeaders`、`decompressRecords`、`decodeSnappy`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package logsource

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"math"

	"collaboration/search/internal/domain/event"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
	"github.com/twmb/franz-go/pkg/kmsg"
)

const (
	maxRecordFrameBytes = event.MaxRecordBytes + (64 << 10)
	maxDecodedBytes     = 8 << 20
	maxRecordCount      = 10000
	maxRecordHeaders    = 128
)

var invalidRecords = errors.New("SEARCH_LOG_RECORDS_INVALID")
var recordCRC = crc32.MakeTable(crc32.Castagnoli)

// decodeRecords只在整个响应验证通过后交付，防止损坏尾部仍使持久checkpoint前移。
// 非压实主题必须连续；起点允许位于批中，H是排他上界，不能借批尾跨过它。
func decodeRecords(raw []byte, topic string, partition int, next, until int64) ([]event.Record, int64, error) {
	original := next
	fail := func() ([]event.Record, int64, error) { return nil, original, invalidRecords }
	if next < 0 || until < next || partition < 0 || partition > 2 || !topicName.MatchString(topic) || len(raw) > maxDecodedBytes {
		return fail()
	}
	var result []event.Record
	remaining, count := maxDecodedBytes, 0
	previousEnd := int64(-1)
	for len(raw) > 0 {
		if len(raw) < 61 {
			return fail()
		}
		length := int64(int32(binary.BigEndian.Uint32(raw[8:12]))) + 12
		if length < 61 || length > int64(len(raw)) {
			return fail()
		}
		encoded := raw[:int(length)]
		raw = raw[int(length):]
		var batch kmsg.RecordBatch
		if batch.ReadFrom(encoded) != nil || batch.Magic != 2 || uint32(batch.CRC) != crc32.Checksum(encoded[21:], recordCRC) || batch.Attributes & ^int16(15) != 0 || batch.Attributes&7 > 4 || batch.NumRecords <= 0 || int64(batch.NumRecords) > int64(maxRecordCount-count) || batch.LastOffsetDelta != batch.NumRecords-1 || batch.FirstOffset < 0 || batch.FirstOffset > math.MaxInt64-int64(batch.NumRecords) {
			return fail()
		}
		if previousEnd >= 0 && batch.FirstOffset != previousEnd {
			return fail()
		}
		previousEnd = batch.FirstOffset + int64(batch.NumRecords)
		decoded, err := decompressRecords(batch.Records, batch.Attributes&7, remaining)
		if err != nil {
			return fail()
		}
		remaining -= len(decoded)
		count += int(batch.NumRecords)
		for i := int32(0); i < batch.NumRecords; i++ {
			size, n := binary.Varint(decoded)
			if n <= 0 || n > 5 || size < 0 || size > maxRecordFrameBytes || size > int64(len(decoded)-n) {
				return fail()
			}
			recordBytes := decoded[:n+int(size)]
			// kmsg会按header数量分配数组；先检查长度与数量，阻断恶意短包巨量分配。
			if !boundedRecordHeaders(recordBytes[n:]) {
				return fail()
			}
			var record kmsg.Record
			if record.ReadFrom(recordBytes) != nil || record.OffsetDelta != i || record.Attributes != 0 || len(record.Value) == 0 || len(record.Value) > event.MaxRecordBytes {
				return fail()
			}
			offset := batch.FirstOffset + int64(i)
			if offset >= original && offset < until {
				if offset != next {
					return fail()
				}
				result = append(result, event.Record{Position: event.Position{Topic: topic, Partition: partition, Offset: offset}, Value: bytes.Clone(record.Value)})
				next++
			} else if offset >= until && next < until {
				return fail()
			}
			decoded = decoded[len(recordBytes):]
		}
		if len(decoded) != 0 {
			return fail()
		}
	}
	return result, next, nil
}

// 只预检会触发数组分配的前缀，字段的完整语义仍由固定版本kmsg解码。
func boundedRecordHeaders(src []byte) bool {
	if len(src) < 1 {
		return false
	}
	src = src[1:]
	take := func() (int64, bool) {
		value, n := binary.Varint(src)
		if n <= 0 {
			return 0, false
		}
		src = src[n:]
		return value, true
	}
	if _, ok := take(); !ok {
		return false
	}
	if _, ok := take(); !ok {
		return false
	}
	for i := 0; i < 2; i++ {
		size, ok := take()
		if !ok || size < -1 || size > int64(len(src)) {
			return false
		}
		if size >= 0 {
			src = src[int(size):]
		}
	}
	headers, ok := take()
	return ok && headers >= 0 && headers <= maxRecordHeaders && headers <= int64(len(src)/2)
}

func decompressRecords(src []byte, codec int16, limit int) ([]byte, error) {
	if limit < 0 {
		return nil, invalidRecords
	}
	var reader io.Reader
	switch codec {
	case 0:
		if len(src) > limit {
			return nil, invalidRecords
		}
		return src, nil
	case 1:
		value, err := gzip.NewReader(bytes.NewReader(src))
		if err != nil {
			return nil, invalidRecords
		}
		defer value.Close()
		reader = value
	case 2:
		return decodeSnappy(src, limit)
	case 3:
		reader = lz4.NewReader(bytes.NewReader(src))
	case 4:
		value, err := zstd.NewReader(bytes.NewReader(src), zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(maxDecodedBytes), zstd.WithDecoderMaxWindow(maxDecodedBytes))
		if err != nil {
			return nil, invalidRecords
		}
		defer value.Close()
		reader = value
	default:
		return nil, invalidRecords
	}
	// 多读一个字节辨别超限，不能先无界解压再检查结果大小。
	decoded, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil || len(decoded) > limit {
		return nil, invalidRecords
	}
	return decoded, nil
}

func decodeSnappy(src []byte, limit int) ([]byte, error) {
	prefix := []byte{130, 83, 78, 65, 80, 80, 89, 0}
	chunk := func(encoded []byte, available int) ([]byte, error) {
		size, err := s2.DecodedLen(encoded)
		if err != nil || size > available {
			return nil, invalidRecords
		}
		decoded, err := s2.Decode(make([]byte, 0, size), encoded)
		if err != nil {
			return nil, invalidRecords
		}
		return decoded, nil
	}
	if !bytes.HasPrefix(src, prefix) {
		return chunk(src, limit)
	}
	if len(src) < 16 || binary.BigEndian.Uint32(src[8:12]) != 1 || binary.BigEndian.Uint32(src[12:16]) != 1 {
		return nil, invalidRecords
	}
	src = src[16:]
	var result []byte
	for len(src) > 0 {
		if len(src) < 4 {
			return nil, invalidRecords
		}
		size := int64(binary.BigEndian.Uint32(src))
		src = src[4:]
		if size <= 0 || size > int64(len(src)) {
			return nil, invalidRecords
		}
		decoded, err := chunk(src[:int(size)], limit-len(result))
		if err != nil {
			return nil, err
		}
		result = append(result, decoded...)
		src = src[int(size):]
	}
	return result, nil
}
````

UTF-8/LF全文SHA-256：`fd54d3f61d13a5c695f6bf34232ebee0759bac1857a66088948494274e4dbb79`。[参考实现](../../../../services/search/internal/client/logsource/records.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
