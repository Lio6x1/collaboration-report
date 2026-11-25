package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

var ErrInvalid = errors.New("FIXED_CONTENT_INVALID")

// Decode拒绝非法UTF-8、孤立代理项、重复对象键和超出规范范围的数字，避免标准库静默替换正文。
func Decode(raw []byte) (any, error) {
	if len(raw) == 0 || len(raw) > 32<<20 || !utf8.Valid(raw) || !json.Valid(raw) || !validSurrogates(raw) {
		return nil, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	count := 0
	value, err := decodeValue(decoder, 0, &count)
	if err != nil {
		return nil, ErrInvalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder, depth int, count *int) (any, error) {
	*count += 1
	if depth > 128 || *count > 1000000 {
		return nil, ErrInvalid
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token := token.(type) {
	case json.Delim:
		switch token {
		case '{':
			object := map[string]any{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, ErrInvalid
				}
				if _, exists := object[key]; exists {
					return nil, ErrInvalid
				}
				child, err := decodeValue(decoder, depth+1, count)
				if err != nil {
					return nil, err
				}
				object[key] = child
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return nil, ErrInvalid
			}
			return object, nil
		case '[':
			array := []any{}
			for decoder.More() {
				child, err := decodeValue(decoder, depth+1, count)
				if err != nil {
					return nil, err
				}
				array = append(array, child)
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return nil, ErrInvalid
			}
			return array, nil
		}
	case json.Number:
		value, err := token.Int64()
		if err != nil || value < -2147483647 || value > 2147483647 {
			return nil, ErrInvalid
		}
		return json.Number(strconv.FormatInt(value, 10)), nil
	case string, bool, nil:
		return token, nil
	}
	return nil, ErrInvalid
}

func validSurrogates(raw []byte) bool {
	inside := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || raw[i] != '\\' {
			continue
		}
		if i+1 >= len(raw) {
			return false
		}
		if raw[i+1] != 'u' {
			i++
			continue
		}
		if i+5 >= len(raw) {
			return false
		}
		value, err := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
		if err != nil {
			return false
		}
		if value >= 0xd800 && value <= 0xdbff {
			if i+11 >= len(raw) || raw[i+6] != '\\' || raw[i+7] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[i+8:i+12]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 11
		} else {
			if value >= 0xdc00 && value <= 0xdfff {
				return false
			}
			i += 5
		}
	}
	return !inside
}
