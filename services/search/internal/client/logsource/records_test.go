package logsource

import (
	"bytes"
	"collaboration/search/internal/domain/event"
	"compress/gzip"
	"encoding/binary"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
	"github.com/twmb/franz-go/pkg/kmsg"
	"hash/crc32"
	"testing"
)

func encodedBatch(t *testing.T, codec int16, first int64, values ...[]byte) []byte {
	t.Helper()
	var records []byte
	for i, value := range values {
		record := kmsg.Record{OffsetDelta: int32(i), Value: value}
		record.Length = int32(len(record.AppendTo(nil)) - 1)
		records = record.AppendTo(records)
	}
	var compressed bytes.Buffer
	switch codec {
	case 0:
		compressed.Write(records)
	case 1:
		writer := gzip.NewWriter(&compressed)
		writer.Write(records)
		writer.Close()
	case 2:
		compressed.Write([]byte{130, 83, 78, 65, 80, 80, 89, 0, 0, 0, 0, 1, 0, 0, 0, 1})
		chunk := s2.EncodeSnappy(nil, records)
		binary.Write(&compressed, binary.BigEndian, uint32(len(chunk)))
		compressed.Write(chunk)
	case 3:
		writer := lz4.NewWriter(&compressed)
		writer.Write(records)
		writer.Close()
	case 4:
		writer, err := zstd.NewWriter(&compressed)
		if err != nil {
			t.Fatal(err)
		}
		writer.Write(records)
		writer.Close()
	}
	batch := kmsg.RecordBatch{FirstOffset: first, Magic: 2, Attributes: codec, NumRecords: int32(len(values)), LastOffsetDelta: int32(len(values) - 1), Records: compressed.Bytes()}
	batch.Length = int32(49 + len(batch.Records))
	raw := batch.AppendTo(nil)
	batch.CRC = int32(crc32.Checksum(raw[21:], recordCRC))
	return batch.AppendTo(nil)
}

func TestDecodeRecordsBoundedWindow(t *testing.T) {
	// 每种支持的codec共用主线断言：批中续读，H之前交付且不越界。
	for codec := int16(0); codec <= 4; codec++ {
		raw := encodedBatch(t, codec, 10, []byte("before"), []byte("wanted"), []byte("after"))
		records, next, err := decodeRecords(raw, "events", 1, 11, 12)
		if err != nil || next != 12 || len(records) != 1 || records[0].Offset != 11 || records[0].Partition != 1 || records[0].Topic != "events" || string(records[0].Value) != "wanted" {
			t.Fatalf("codec %d: count=%d next=%d error=%v", codec, len(records), next, err)
		}
	}
}

func TestDecodeRecordsRejectsEntireCorruptResponse(t *testing.T) {
	// 损坏尾部不能交付正确前缀，更不能前移持久checkpoint。
	good := encodedBatch(t, 0, 10, []byte("a"))
	damaged := encodedBatch(t, 0, 11, []byte("b"))
	damaged[len(damaged)-1] ^= 1
	oversized := encodedBatch(t, 1, 10, bytes.Repeat([]byte("x"), event.MaxRecordBytes+1))
	transaction := encodedBatch(t, 0, 10, []byte("a"))
	transaction[22] |= 16
	binary.BigEndian.PutUint32(transaction[17:21], crc32.Checksum(transaction[21:], recordCRC))
	for name, raw := range map[string][]byte{
		"crc after good prefix": append(bytes.Clone(good), damaged...),
		"incomplete tail":       append(bytes.Clone(good), 1),
		"oversized record":      oversized,
		"missing exact offset":  encodedBatch(t, 0, 11, []byte("a")),
		"transaction":           transaction,
	} {
		t.Run(name, func(t *testing.T) {
			records, next, err := decodeRecords(raw, "events", 0, 10, 12)
			if err == nil || len(records) != 0 || next != 10 {
				t.Fatalf("must reject all records, got count=%d next=%d error=%v", len(records), next, err)
			}
		})
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	writer.Write(bytes.Repeat([]byte("x"), 4096))
	writer.Close()
	if _, err := decompressRecords(compressed.Bytes(), 1, 128); err == nil {
		t.Fatal("unbounded decompression")
	}
	record := kmsg.Record{Headers: make([]kmsg.Header, maxRecordHeaders+1)}
	raw := record.AppendTo(nil)
	if boundedRecordHeaders(raw[1:]) {
		t.Fatal("unbounded headers")
	}
}

func TestDecodeRecordsMaximumValue(t *testing.T) {
	// 2MiB限制属于业务Value，Kafka的长度、offset与header元数据不能侵占该额度。
	value := bytes.Repeat([]byte("x"), event.MaxRecordBytes)
	raw := encodedBatch(t, 0, 10, value)
	records, next, err := decodeRecords(raw, "events", 0, 10, 11)
	if err != nil || next != 11 || len(records) != 1 || !bytes.Equal(records[0].Value, value) {
		t.Fatalf("maximum Value rejected: count=%d next=%d error=%v", len(records), next, err)
	}
}
