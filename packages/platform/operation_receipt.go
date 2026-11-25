package platform

import (
	"encoding/json"
	"errors"
	"fmt"
)

// EncodedOperationReceipt 是写入回执所需的已校验状态和固定 JSON，不是另一套 HTTP DTO。
// 业务结果可以携带生成契约允许的不同字段，但原号、状态与可编码性必须在持久化前确认。
type EncodedOperationReceipt struct {
	State string
	JSON  json.RawMessage
}

func EncodeOperationReceipt(operation string, value map[string]any) (EncodedOperationReceipt, error) {
	id, ok := value["operation_id"].(string)
	if !ok || operation == "" || id != operation {
		return EncodedOperationReceipt{}, errors.New("OPERATION_RESULT_BINDING_INVALID")
	}
	state, ok := value["state"].(string)
	if !ok {
		return EncodedOperationReceipt{}, errors.New("OPERATION_RESULT_STATE_INVALID")
	}
	switch state {
	case "PREPARING", "SUCCEEDED", "REJECTED", "CANCELLED":
	default:
		return EncodedOperationReceipt{}, errors.New("OPERATION_RESULT_STATE_INVALID")
	}
	// 继续使用旧 map 的确定性 JSON 编码，避免固定回执/摘要因字段顺序变化而改变。
	raw, err := json.Marshal(value)
	if err != nil {
		return EncodedOperationReceipt{}, fmt.Errorf("OPERATION_RESULT_ENCODING_FAILED: %w", err)
	}
	return EncodedOperationReceipt{State: state, JSON: raw}, nil
}
