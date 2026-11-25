# services/document/internal/logic/document/pagination.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/216-pagination.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`CursorSigner`、`Pagination`、`NewPagination`、`Parse`、`Next`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"time"
)

type CursorSigner interface{ Sign([]byte) []byte }
type Pagination struct{ signer CursorSigner }

func NewPagination(signer CursorSigner) *Pagination { return &Pagination{signer: signer} }
func (s *Pagination) Parse(limitText, encoded, binding string) (int, string, error) {
	limit := 20
	if limitText != "" {
		value, err := strconv.Atoi(limitText)
		if err != nil || value < 1 || value > 100 {
			return 0, "", domain.Violation(domain.InvalidRequest, "LIMIT_INVALID")
		}
		limit = value
	}
	if encoded == "" {
		return limit, "", nil
	}
	if len(encoded) > 2048 {
		return 0, "", domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) < 33 {
		return 0, "", domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
	}
	data, signature := raw[:len(raw)-32], raw[len(raw)-32:]
	if !hmac.Equal(signature, s.signer.Sign(data)) {
		return 0, "", domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
	}
	var cursor domain.Cursor
	if json.Unmarshal(data, &cursor) != nil || cursor.Binding != binding || cursor.Until <= time.Now().Unix() {
		return 0, "", domain.Violation(domain.StateConflict, "CURSOR_STALE")
	}
	return limit, cursor.Last, nil
}
func (s *Pagination) Next(binding, last string) *string {
	raw, _ := json.Marshal(domain.Cursor{Binding: binding, Last: last, Until: time.Now().Add(5 * time.Minute).Unix()})
	encoded := base64.RawURLEncoding.EncodeToString(append(raw, s.signer.Sign(raw)...))
	return &encoded
}
````

UTF-8/LF全文SHA-256：`ddb65afd780f21859c065add265eabcec147be43efe1c20011b372c8a51bcb0b`。[参考实现](../../../../services/document/internal/logic/document/pagination.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
