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
