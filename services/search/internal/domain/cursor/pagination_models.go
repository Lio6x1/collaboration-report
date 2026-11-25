// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package cursor

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// control.go：保留原实现，按业务合并文件。

const (
	PhaseOpening    = "OPENING"
	PhaseIdle       = "IDLE"
	PhaseInflight   = "INFLIGHT"
	MaxControlBytes = 2048
)

// Control只保存单链最新确认位置与未知调用事实，不存批次历史。Deadline/RetryAt
// 均为UnixMillis；是否到期与SQL OPEN/CLOSING/CLOSED状态由持同一事务的用例决定。
type Control struct {
	Version   uint8           `json:"version"`
	Phase     string          `json:"phase"`
	Tail      json.RawMessage `json:"tail"`
	Nonce     string          `json:"nonce"`
	Deadline  int64           `json:"deadline"`
	RetryAt   int64           `json:"retry_at"`
	Uncertain bool            `json:"uncertain"`
}

// 持久格式与Go内存字段刻意区分：MySQL JSON会正规化数组中的数字，
// tail必须作为UTF8字符串保存；读取时再校验内部sort，不能直接Marshal Control入库。
type persistedControl struct {
	Version   uint8   `json:"version"`
	Phase     string  `json:"phase"`
	Tail      *string `json:"tail"`
	Nonce     string  `json:"nonce"`
	Deadline  int64   `json:"deadline"`
	RetryAt   int64   `json:"retry_at"`
	Uncertain bool    `json:"uncertain"`
}

func validControlNonce(nonce string) bool {
	if len(nonce) != 64 {
		return false
	}
	nonzero := false
	for _, digit := range []byte(nonce) {
		if !((digit >= '0' && digit <= '9') || (digit >= 'a' && digit <= 'f')) {
			return false
		}
		nonzero = nonzero || digit != '0'
	}
	return nonzero
}

func validateControl(control Control) error {
	invalid := errors.New("SEARCH_CURSOR_CONTROL_INVALID")
	if control.Version != 1 || control.RetryAt < 0 || (len(control.Tail) != 0 && ValidateSort(control.Tail) != nil) {
		return invalid
	}
	switch control.Phase {
	case PhaseOpening:
		if len(control.Tail) != 0 {
			return invalid
		}
		fallthrough
	case PhaseInflight:
		if !validControlNonce(control.Nonce) || control.Deadline <= 0 {
			return invalid
		}
	case PhaseIdle:
		if control.Nonce != "" || control.Deadline != 0 {
			return invalid
		}
	default:
		return invalid
	}
	return nil
}

func EncodeControl(control Control) ([]byte, error) {
	if err := validateControl(control); err != nil {
		return nil, err
	}
	wire := persistedControl{Version: control.Version, Phase: control.Phase, Nonce: control.Nonce,
		Deadline: control.Deadline, RetryAt: control.RetryAt, Uncertain: control.Uncertain}
	if len(control.Tail) != 0 {
		tail := string(control.Tail)
		wire.Tail = &tail
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, errors.New("SEARCH_CURSOR_CONTROL_INVALID")
	}
	if len(data) > MaxControlBytes {
		return nil, errors.New("SEARCH_CURSOR_CONTROL_INVALID")
	}
	return data, nil
}

func strictControlRoot(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		key, isKey := token.(string)
		if err != nil || !isKey || seen[key] {
			return false
		}
		switch key {
		case "version", "phase", "tail", "nonce", "deadline", "retry_at", "uncertain":
		default:
			return false
		}
		seen[key] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil || (key != "tail" && bytes.Equal(bytes.TrimSpace(value), []byte("null"))) {
			return false
		}
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') || !seen["version"] || !seen["phase"] {
		return false
	}
	var extra any
	return decoder.Decode(&extra) == io.EOF
}

func DecodeControl(data []byte) (Control, error) {
	var control Control
	invalid := errors.New("SEARCH_CURSOR_CONTROL_INVALID")
	// 标准Decoder对重复键采用最后值；先检查根键，避免SQL JSON与Go恢复规则分歧。
	if len(data) == 0 || len(data) > MaxControlBytes || !utf8.Valid(data) || !strictControlRoot(data) {
		return control, invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var wire persistedControl
	if decoder.Decode(&wire) != nil {
		return Control{}, invalid
	}
	control = Control{Version: wire.Version, Phase: wire.Phase, Nonce: wire.Nonce,
		Deadline: wire.Deadline, RetryAt: wire.RetryAt, Uncertain: wire.Uncertain}
	if wire.Tail != nil {
		if *wire.Tail == "" {
			return Control{}, invalid
		}
		control.Tail = json.RawMessage(*wire.Tail)
	}
	if validateControl(control) != nil {
		return Control{}, invalid
	}
	control.Tail = append(json.RawMessage(nil), control.Tail...)
	return control, nil
}

// query.go：保留原实现，按业务合并文件。

// Query只保存校验后的RPC输入，不代表Doc当前读取资格；实际权限由Doc最终复核。
// ScopeHash是受mTLS保护的Doc绑定，不能把它当作浏览器自报身份或准入证明。
type Query struct {
	Query     string
	QueryHash string
	PageSize  uint32
	Scope     *pb.SearchScope
}

func QueryDigest(query string) string {
	digest := sha256.Sum256([]byte(query))
	return hex.EncodeToString(digest[:])
}

func canonicalDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == value &&
		value != strings.Repeat("0", sha256.Size*2)
}

func ValidateQuery(request *pb.SearchCandidatesRequest) (Query, error) {
	invalid := errors.New("SEARCH_QUERY_BINDING_INVALID")
	if request == nil || len(request.Query) > 500*utf8.UTFMax || !utf8.ValidString(request.Query) || utf8.RuneCountInString(request.Query) > 500 ||
		strings.TrimSpace(request.Query) == "" || request.QueryDigest != QueryDigest(request.Query) ||
		request.PageSize == 0 || request.PageSize > 100 || len(request.Cursor) > MaxEnvelopeBytes {
		return Query{}, invalid
	}
	if err := ValidateScope(request.Scope); err != nil {
		return Query{}, err
	}
	// 自有副本阻止调用者在SQL绑定后改变space/session；禁止依赖共享protobuf可变数组。
	scope := &pb.SearchScope{ScopeHash: request.Scope.ScopeHash, Anonymous: request.Scope.Anonymous,
		SessionRef: append([]byte(nil), request.Scope.SessionRef...),
		SpaceIds:   append([]uint64(nil), request.Scope.SpaceIds...)}
	return Query{Query: request.Query, QueryHash: request.QueryDigest, PageSize: request.PageSize, Scope: scope}, nil
}

// CloseCursor与查询共用同一个scope边界。位置验签及SQL中的原scope/session仍由用例核对。
func ValidateScope(scope *pb.SearchScope) error {
	if scope == nil || !canonicalDigest(scope.ScopeHash) || len(scope.SpaceIds) > 1024 {
		return errors.New("SEARCH_SCOPE_INVALID")
	}
	if scope.Anonymous {
		if len(scope.SessionRef) != 0 {
			return errors.New("SEARCH_SCOPE_INVALID")
		}
	} else {
		if len(scope.SessionRef) != sha256.Size {
			return errors.New("SEARCH_SCOPE_INVALID")
		}
		var zero [sha256.Size]byte
		if string(scope.SessionRef) == string(zero[:]) {
			return errors.New("SEARCH_SCOPE_INVALID")
		}
	}
	for i, space := range scope.SpaceIds {
		if space == 0 || (i > 0 && space <= scope.SpaceIds[i-1]) {
			return errors.New("SEARCH_SCOPE_INVALID")
		}
	}
	return nil
}

// sort.go：保留原实现，按业务合并文件。

// ValidateSort只检查固定score/Doc/shard三元组，原JSON始终由锚点原样保存。
// uint64不能解成float64再输出，否则2^53以上位置会移动、导致分页漏项或重复。
func ValidateSort(raw json.RawMessage) error {
	invalid := errors.New("SEARCH_CURSOR_SORT_INVALID")
	if len(raw) == 0 || len(raw) > MaxSortBytes || !utf8.Valid(raw) {
		return invalid
	}
	var tuple []json.RawMessage
	if json.Unmarshal(raw, &tuple) != nil || len(tuple) != 3 {
		return invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(tuple[0]))
	decoder.UseNumber()
	var scoreValue any
	if decoder.Decode(&scoreValue) != nil {
		return invalid
	}
	score, number := scoreValue.(json.Number)
	if !number {
		return invalid
	}
	// score转数值只为排除不可表示的无穷值，绝不据此重写已绑定的原sort词法。
	finite, err := score.Float64()
	if err != nil || math.IsNaN(finite) || math.IsInf(finite, 0) {
		return invalid
	}
	document := bytes.TrimSpace(tuple[1])
	if len(document) < 3 || document[0] != '"' || document[len(document)-1] != '"' ||
		!canonicalUnsigned(document[1:len(document)-1], false) {
		return invalid
	}
	if !canonicalUnsigned(bytes.TrimSpace(tuple[2]), true) {
		return invalid
	}
	return nil
}

// 直接核对十进制词法与完整范围，拒绝转义Doc字符串、指数/小数shard、前导零及回绕。
func canonicalUnsigned(raw []byte, allowZero bool) bool {
	if len(raw) == 0 || len(raw) > 20 || (raw[0] == '0' && (len(raw) != 1 || !allowZero)) {
		return false
	}
	for _, digit := range raw {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(string(raw), 10, 64)
	return err == nil
}
