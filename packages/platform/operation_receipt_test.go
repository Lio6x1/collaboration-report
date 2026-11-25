package platform

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEncodeOperationReceipt(t *testing.T) {
	const operation = "operation-123456789"
	// 回执的扩展字段和旧 map 编码字节不变，不能因为抽取公共边界改掉固定结果。
	value := map[string]any{"operation_id": operation, "state": "PREPARING", "result": map[string]string{"entity_id": "42"}, "poll_url": "/v1/operations/" + operation}
	want, _ := json.Marshal(value)
	receipt, err := EncodeOperationReceipt(operation, value)
	if err != nil || receipt.State != "PREPARING" || !bytes.Equal(receipt.JSON, want) {
		t.Fatalf("fixed receipt changed: %+v, %v", receipt, err)
	}
	// 回调遗漏状态、错绑原号或载荷无法编码时必须阻止持久化，不能默认为成功或 panic。
	for name, invalid := range map[string]map[string]any{
		"missing state":       {"operation_id": operation},
		"wrong type":          {"operation_id": operation, "state": 1},
		"unknown state":       {"operation_id": operation, "state": "RUNNING"},
		"wrong operation":     {"operation_id": "another-operation", "state": "SUCCEEDED"},
		"unencodable payload": {"operation_id": operation, "state": "SUCCEEDED", "result": make(chan int)},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := EncodeOperationReceipt(operation, invalid)
			if err == nil || len(got.JSON) != 0 || got.State != "" {
				t.Fatalf("invalid result became persistable: %+v, %v", got, err)
			}
		})
	}
}
