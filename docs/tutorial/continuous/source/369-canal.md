# services/search/internal/domain/event/canal.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/369-canal.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`flatMessage`、`payload`、`ParseCanal`、`RestoreEvent`、`parseRow`、`canonicalPositive`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package event

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxRecordBytes = 2 << 20

type flatMessage struct {
	ID        json.Number                  `json:"id"`
	Database  string                       `json:"database"`
	Table     string                       `json:"table"`
	PKNames   []string                     `json:"pkNames"`
	IsDDL     *bool                        `json:"isDdl"`
	Type      string                       `json:"type"`
	ES        json.Number                  `json:"es"`
	TS        json.Number                  `json:"ts"`
	SQL       string                       `json:"sql"`
	SQLTypes  map[string]int               `json:"sqlType"`
	MySQLType map[string]string            `json:"mysqlType"`
	Data      []map[string]json.RawMessage `json:"data"`
	Old       json.RawMessage              `json:"old"`
	GTID      string                       `json:"gtid"`
}

type payload struct {
	ID       string  `json:"event_id"`
	Type     string  `json:"event_type"`
	Schema   string  `json:"schema_version"`
	Document string  `json:"document_id"`
	Sequence string  `json:"aggregate_seq"`
	Release  *string `json:"release_id"`
	Epoch    string  `json:"publication_epoch"`
	Occurred string  `json:"occurred_at"`
}

var validEventID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$`)

// ParseCanal只接受受限复制表的INSERT，不把DDL、其他私有表或未知schema当可消费成功。
// 一条Kafka记录可包含多行；调用方只有在全部行完成后才能同步提交该记录的offset。
func ParseCanal(raw []byte) ([]Event, error) {
	if len(raw) == 0 || len(raw) > MaxRecordBytes || !utf8.Valid(raw) {
		return nil, errors.New("CANAL_RECORD_SIZE_INVALID")
	}
	var message flatMessage
	if err := decodeStrict(raw, &message); err != nil {
		return nil, err
	}
	if message.Database != "document_db" || message.Table != "outbox" || message.IsDDL == nil || *message.IsDDL ||
		message.Type != "INSERT" || len(message.PKNames) != 1 || message.PKNames[0] != "event_id" ||
		len(message.Data) == 0 || len(message.Data) > 1024 {
		return nil, errors.New("CANAL_RECORD_BINDING_INVALID")
	}
	events := make([]Event, 0, len(message.Data))
	for _, row := range message.Data {
		value, err := parseRow(row)
		if err != nil {
			return nil, err
		}
		events = append(events, value)
	}
	return events, nil
}

// RestoreEvent供私有记账核验原行；与真实Canal解析共用同一绑定规则，不引入较弱的恢复入口。
func RestoreEvent(raw []byte) (Event, error) {
	if len(raw) == 0 || len(raw) > MaxRecordBytes || !utf8.Valid(raw) {
		return Event{}, errors.New("CANAL_RECORD_SIZE_INVALID")
	}
	var row map[string]json.RawMessage
	if err := decodeStrict(raw, &row); err != nil {
		return Event{}, err
	}
	return parseRow(row)
}

func parseRow(row map[string]json.RawMessage) (Event, error) {
	var result Event
	allowed := map[string]bool{"event_id": true, "aggregate_kind": true, "aggregate_id": true,
		"aggregate_seq": true, "event_type": true, "schema_version": true, "payload": true, "created_at": true}
	if len(row) != len(allowed) {
		return result, errors.New("CANAL_ROW_FIELDS_INVALID")
	}
	values := make(map[string]string, len(row))
	for key, raw := range row {
		var text string
		if !allowed[key] || json.Unmarshal(raw, &text) != nil || text == "" {
			return result, errors.New("CANAL_ROW_FIELDS_INVALID")
		}
		values[key] = text
	}
	if !validEventID.MatchString(values["event_id"]) || values["aggregate_kind"] != "DOCUMENT" ||
		values["schema_version"] != "1" ||
		(values["event_type"] != "DOCUMENT_PUBLISHED" && values["event_type"] != "DOCUMENT_UNPUBLISHED" &&
			values["event_type"] != "DOCUMENT_VISIBILITY_CHANGED") {
		return result, errors.New("CANAL_EVENT_TYPE_INVALID")
	}
	var body payload
	var fields map[string]json.RawMessage
	if err := decodeStrict([]byte(values["payload"]), &fields); err != nil {
		return result, err
	}
	if len(fields) != 8 {
		return result, errors.New("CANAL_PAYLOAD_FIELDS_INVALID")
	}
	for _, key := range []string{"event_id", "event_type", "schema_version", "document_id", "aggregate_seq", "release_id", "publication_epoch", "occurred_at"} {
		if _, present := fields[key]; !present {
			return result, errors.New("CANAL_PAYLOAD_FIELDS_INVALID")
		}
	}
	if err := decodeStrict([]byte(values["payload"]), &body); err != nil {
		return result, err
	}
	if body.ID != values["event_id"] || body.Type != values["event_type"] || body.Schema != "1" ||
		body.Document != values["aggregate_id"] || body.Sequence != values["aggregate_seq"] {
		return result, errors.New("CANAL_PAYLOAD_BINDING_INVALID")
	}
	var err error
	if result.Document, err = canonicalPositive(body.Document); err != nil {
		return result, err
	}
	if result.Sequence, err = canonicalPositive(body.Sequence); err != nil {
		return result, err
	}
	if result.PublicationEpoch, err = canonicalPositive(body.Epoch); err != nil {
		return result, err
	}
	if body.Type == "DOCUMENT_PUBLISHED" && body.Release == nil {
		return result, errors.New("CANAL_RELEASE_BINDING_INVALID")
	}
	if body.Release != nil {
		if body.Type == "DOCUMENT_UNPUBLISHED" {
			return result, errors.New("CANAL_RELEASE_BINDING_INVALID")
		}
		if result.Release, err = canonicalPositive(*body.Release); err != nil {
			return result, err
		}
	}
	// 可见性变化既可发生于未发布文档，也可绑定当前正式指针；不推导匿名可读或改变发布代次。
	// Search仍取Doc当前权威快照，原发布/撤下的非空/空指针约束不能随新类型放宽。
	result.OccurredAt, err = time.Parse(time.RFC3339Nano, body.Occurred)
	if err != nil || !strings.HasSuffix(body.Occurred, "Z") {
		return result, errors.New("CANAL_EVENT_TIME_INVALID")
	}
	result.ID, result.Type = body.ID, body.Type
	// map按键排序编码，保留原payload字符串及不可变行字段。外层批次元数据重投时会改变。
	origin, err := json.Marshal(values)
	if err != nil {
		return result, errors.New("CANAL_ORIGIN_ENCODING_FAILED")
	}
	result.RawDigest = sha256.Sum256(origin)
	result.Original = origin
	return result, nil
}

func canonicalPositive(value string) (uint64, error) {
	number, err := strconv.ParseUint(value, 10, 64)
	if err != nil || number == 0 || strconv.FormatUint(number, 10) != value {
		return 0, errors.New("CANAL_UINT64_INVALID")
	}
	return number, nil
}
````

UTF-8/LF全文SHA-256：`a2b60985098828a17d36e6839acf21e8145751f754cbaeeb5c0eac208cb09d51`。[参考实现](../../../../services/search/internal/domain/event/canal.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
