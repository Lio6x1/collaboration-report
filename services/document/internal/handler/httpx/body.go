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
