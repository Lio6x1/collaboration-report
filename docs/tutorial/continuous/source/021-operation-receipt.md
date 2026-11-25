# packages/platform/operation_receipt.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/021-operation-receipt.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`EncodedOperationReceipt`、`EncodeOperationReceipt`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`75352799ca548af428c7d54b5bebf17123678e6c7d697176748985c3a88dcf4d`。[参考实现](../../../../packages/platform/operation_receipt.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
