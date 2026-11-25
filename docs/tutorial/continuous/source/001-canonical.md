# contracts/content/canonical.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/001-canonical.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`Canonical`、`writeCanonical`、`nodeStringLess`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package content

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// Canonical与Node canonical-v1使用同一UTF-16键序，保留Unicode、正文空白与数组顺序。
func Canonical(value any) ([]byte, error) {
	var result bytes.Buffer
	if err := writeCanonical(&result, value); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func writeCanonical(result *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return nodeStringLess(keys[i], keys[j]) })
		result.WriteByte('{')
		for index, key := range keys {
			if index != 0 {
				result.WriteByte(',')
			}
			if err := writeCanonical(result, key); err != nil {
				return err
			}
			result.WriteByte(':')
			if err := writeCanonical(result, value[key]); err != nil {
				return err
			}
		}
		result.WriteByte('}')
	case []any:
		result.WriteByte('[')
		for index, child := range value {
			if index != 0 {
				result.WriteByte(',')
			}
			if err := writeCanonical(result, child); err != nil {
				return err
			}
		}
		result.WriteByte(']')
	case string:
		if !utf8.ValidString(value) {
			return ErrInvalid
		}
		var encoded bytes.Buffer
		encoder := json.NewEncoder(&encoded)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(value); err != nil {
			return ErrInvalid
		}
		result.Write(bytes.TrimSuffix(encoded.Bytes(), []byte("\n")))
	case json.Number:
		number, err := value.Int64()
		if err != nil || number < -2147483647 || number > 2147483647 {
			return ErrInvalid
		}
		result.WriteString(strconv.FormatInt(number, 10))
	case bool:
		result.WriteString(strconv.FormatBool(value))
	case nil:
		result.WriteString("null")
	default:
		return ErrInvalid
	}
	return nil
}

func nodeStringLess(left, right string) bool {
	a, b := utf16.Encode([]rune(left)), utf16.Encode([]rune(right))
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}
````

UTF-8/LF全文SHA-256：`70d90e793ecbd8055992c9ef8f5cbe898d6f0ceea9f64b509c4e2d815b982837`。[参考实现](../../../../contracts/content/canonical.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
