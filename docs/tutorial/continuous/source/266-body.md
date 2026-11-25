# services/document/internal/handler/httpx/body.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/266-body.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`DecodeBody`、`DecodeBodyLimit`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package httpx

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
)

// 16MiB规范正文另留协议和JSON转义空间；路由框架与本地解码共用同一个传输上限。
const ContentBodyLimit int64 = 18 << 20

// 解码保留生成契约的必填/可空规则、请求上限及未知字段拒绝，不能由各用例复制较弱版本。
func DecodeBody(w http.ResponseWriter, r *http.Request, value any) bool {
	return DecodeBodyLimit(w, r, value, 16384)
}

// 正文修复按冻结的16MiB正文契约接收；其余小型控制请求保留原上限和相同字段校验。
func DecodeBodyLimit(w http.ResponseWriter, r *http.Request, value any, limit int64) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		Fail(w, 400, "JSON_REQUIRED")
		return false
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil || int64(len(raw)) > limit {
		Fail(w, 400, "REQUEST_INVALID")
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		Fail(w, 400, "REQUEST_INVALID")
		return false
	}
	name := reflect.TypeOf(value).Elem().Name()
	for field, nullable := range dto.RequiredFields[name] {
		v, ok := fields[field]
		if !ok || (!nullable && string(v) == "null") {
			Fail(w, 400, "REQUIRED_FIELD_INVALID")
			return false
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil {
		Fail(w, 400, "REQUEST_INVALID")
		return false
	}
	return true
}
````

UTF-8/LF全文SHA-256：`8cd25a7ce7dab8560cd172a57765c95210cc898df7492e6e6e80be91f6df268b`。[参考实现](../../../../services/document/internal/handler/httpx/body.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
