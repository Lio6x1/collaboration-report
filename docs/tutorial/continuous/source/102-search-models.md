# services/document/internal/domain/document/search_models.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/102-search-models.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`IndexEvent`、`NextIndexEventSequence`、`VisibilityIndexEventID`、`EncodeIndexEvent`、`ReserveSearchBody`、`SearchPageCursor`、`SearchCursorCodec`、`NewSearchCursorCodec`、`deriveSearchCursorKey`、`validSearchPageCursor`、`valid`、`signature`、`Seal`、`Open`、`SearchManifestUnit`、`SearchQuery`、`ParseSearchQuery`、`validate`、`Binding`、`writeSearchQueryField`、`EnvelopeSearchAnchor`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// index_event.go：保留原实现，按业务合并文件。

// IndexEvent只描述正式引用及索引水位，不携带正文、策略或身份证明。
// 事件触发Search读取Doc当前权威快照；Release不是读取旧历史正文的授权。
type IndexEvent struct {
	ID, Type                                      string
	Document, Sequence, Release, PublicationEpoch uint64
	OccurredAt                                    time.Time
}

// 可见性变更不属于再次发布，所以索引水位独立推进，不能顺带改变publication_epoch。
// 调用方必须在业务事务内先取得当前水位；耗尽拒绝整个动作，不能回绕复活旧事件。
func NextIndexEventSequence(current uint64) (uint64, error) {
	if current == math.MaxUint64 {
		return 0, Violation(StateConflict, "EVENT_SEQUENCE_EXHAUSTED")
	}
	return current + 1, nil
}

// 一个KB原操作会为多个文档登记事件，不能复用原操作作为每条outbox的主键。
// 固定类型前缀及完整摘要绑定原操作和文档，重试稳定，最大长度输入也不截断摘要。
func VisibilityIndexEventID(operation string, document uint64) (string, error) {
	if !ValidOperationID(operation) || document == 0 {
		return "", Violation(InvalidRequest, "INDEX_EVENT_BINDING_INVALID")
	}
	// 合法操作编号不含换行，因此此分隔不会让不同原操作/文档组合产生相同输入。
	digest := sha256.Sum256([]byte(operation + "\n" + TextID(document)))
	return "visibility_index_" + hex.EncodeToString(digest[:]), nil
}

// EncodeIndexEvent维持既有八键消息形状。所有业务uint64使用规范十进制字符串，
// release_id缺失显式为null；不可见策略不会改变这里的正式指针事实。
func EncodeIndexEvent(event IndexEvent) ([]byte, error) {
	invalid := Violation(InvalidRequest, "INDEX_EVENT_INVALID")
	if !ValidOperationID(event.ID) || event.Document == 0 || event.Sequence == 0 ||
		event.PublicationEpoch == 0 || event.OccurredAt.IsZero() {
		return nil, invalid
	}
	switch event.Type {
	case "DOCUMENT_PUBLISHED":
		if event.Release == 0 {
			return nil, invalid
		}
	case "DOCUMENT_UNPUBLISHED":
		if event.Release != 0 {
			return nil, invalid
		}
	case "DOCUMENT_VISIBILITY_CHANGED":
		// 未发布及已有当前正式版都可变更策略；此动作不创建或撤下正式版。
	default:
		return nil, invalid
	}
	// MarshalText在输出RFC3339Nano前校验可表示年份；不能把无效时间写成毒消息。
	occurred, err := event.OccurredAt.UTC().MarshalText()
	if err != nil {
		return nil, invalid
	}
	var release *string
	if event.Release != 0 {
		value := TextID(event.Release)
		release = &value
	}
	payload := struct {
		ID       string  `json:"event_id"`
		Type     string  `json:"event_type"`
		Schema   string  `json:"schema_version"`
		Document string  `json:"document_id"`
		Sequence string  `json:"aggregate_seq"`
		Release  *string `json:"release_id"`
		Epoch    string  `json:"publication_epoch"`
		Occurred string  `json:"occurred_at"`
	}{
		ID: event.ID, Type: event.Type, Schema: "1", Document: TextID(event.Document),
		Sequence: TextID(event.Sequence), Release: release,
		Epoch: TextID(event.PublicationEpoch), Occurred: string(occurred),
	}
	return json.Marshal(payload)
}

// search_budget.go：保留原实现，按业务合并文件。

const (
	SearchDocumentBodyLimit uint64 = 24 << 20
	SearchPageBodyLimit     uint64 = 64 << 20
)

// ReserveSearchBody只处理权威manifest的字节元数据，不读取正文，也不提供权限判断。
// 调用方须先核验所有unit/version归属；整篇占额后才读正文，lookahead使用同一请求预算。
// 随后的正文必须再次核验实际len==byte_size及完整hash/schema，不能把元数据当完整验证。
func ReserveSearchBody(used uint64, units []uint64) (uint64, error) {
	if used > SearchPageBodyLimit || len(units) == 0 {
		return 0, Violation(TemporarilyUnavailable, "SEARCH_INCOMPLETE")
	}
	var document uint64
	for _, size := range units {
		// 先检查减法边界再加，最大uint64、整篇超限及错误零字节都不能回绕成小正文。
		if size == 0 || size > SearchDocumentBodyLimit || document > SearchDocumentBodyLimit-size {
			return 0, Violation(TemporarilyUnavailable, "SEARCH_INCOMPLETE")
		}
		document += size
	}
	if document > SearchPageBodyLimit-used {
		return 0, Violation(TemporarilyUnavailable, "SEARCH_INCOMPLETE")
	}
	return used + document, nil
}

// search_cursor.go：保留原实现，按业务合并文件。

const (
	searchCursorVersion     = byte(1)
	searchCursorMaxAnchor   = 768
	searchCursorMaxEncoded  = 2048
	searchCursorPlainHeader = 43 // Until(8)、Binding(32)、Prefix(1)、anchor长度(2)。
	searchCursorPurpose     = "collaboration/document/search-page-cursor/v1"
)

// SearchPageCursor只携带Search的opaque批锚点及Doc已消费前缀；不复制候选/正文。
// Until是初次建立链时固定的UnixMillis绝对期限，Seal和Open都不续期。
type SearchPageCursor struct {
	Anchor  []byte
	Prefix  uint8
	Binding [32]byte
	Until   int64
}

type SearchCursorCodec struct {
	aead           cipher.AEAD
	authentication [32]byte
}

func NewSearchCursorCodec(key []byte) (*SearchCursorCodec, error) {
	if len(key) != 32 || subtle.ConstantTimeCompare(key, make([]byte, 32)) == 1 {
		return nil, Violation(InvalidRequest, "CURSOR_INVALID")
	}
	// 同一master仅派生两个独立用途密钥；不持有调用方可变key切片，也不复用其他游标签名。
	encryption := deriveSearchCursorKey(key, searchCursorPurpose+"/encryption")
	authentication := deriveSearchCursorKey(key, searchCursorPurpose+"/authentication")
	block, err := aes.NewCipher(encryption[:])
	clear(encryption[:])
	if err != nil {
		return nil, Violation(TemporarilyUnavailable, "CURSOR_UNAVAILABLE")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, Violation(TemporarilyUnavailable, "CURSOR_UNAVAILABLE")
	}
	return &SearchCursorCodec{aead: aead, authentication: authentication}, nil
}

func deriveSearchCursorKey(master []byte, purpose string) [32]byte {
	mac := hmac.New(sha256.New, master)
	_, _ = mac.Write([]byte(purpose))
	var key [32]byte
	copy(key[:], mac.Sum(nil))
	return key
}

func validSearchPageCursor(cursor SearchPageCursor) bool {
	return len(cursor.Anchor) >= 1 && len(cursor.Anchor) <= searchCursorMaxAnchor &&
		cursor.Prefix <= 99 && cursor.Binding != [32]byte{} && cursor.Until > 0
}

func (codec *SearchCursorCodec) valid() bool {
	return codec != nil && codec.aead != nil && codec.authentication != [32]byte{}
}

func (codec *SearchCursorCodec) signature(envelope []byte) []byte {
	mac := hmac.New(sha256.New, codec.authentication[:])
	_, _ = mac.Write([]byte(searchCursorPurpose + "/envelope"))
	_, _ = mac.Write(envelope)
	return mac.Sum(nil)
}

func (codec *SearchCursorCodec) Seal(cursor SearchPageCursor) (string, error) {
	if !codec.valid() || !validSearchPageCursor(cursor) {
		return "", Violation(InvalidRequest, "CURSOR_INVALID")
	}
	// 显式二进制字段避免JSON数值精度和另一套字段拼法；Anchor原字节只复制，不解析sort。
	plain := make([]byte, searchCursorPlainHeader+len(cursor.Anchor))
	defer clear(plain)
	binary.BigEndian.PutUint64(plain[:8], uint64(cursor.Until))
	copy(plain[8:40], cursor.Binding[:])
	plain[40] = cursor.Prefix
	binary.BigEndian.PutUint16(plain[41:43], uint16(len(cursor.Anchor)))
	copy(plain[43:], cursor.Anchor)
	nonce := make([]byte, codec.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", Violation(TemporarilyUnavailable, "CURSOR_UNAVAILABLE")
	}
	envelope := make([]byte, 1+len(nonce), 1+len(nonce)+len(plain)+codec.aead.Overhead()+sha256.Size)
	envelope[0] = searchCursorVersion
	copy(envelope[1:], nonce)
	envelope = codec.aead.Seal(envelope, nonce, plain, []byte(searchCursorPurpose))
	envelope = append(envelope, codec.signature(envelope)...)
	encoded := base64.RawURLEncoding.EncodeToString(envelope)
	if len(encoded) > searchCursorMaxEncoded {
		return "", Violation(InvalidRequest, "CURSOR_INVALID")
	}
	return encoded, nil
}

func (codec *SearchCursorCodec) Open(encoded string, expectedBinding [32]byte, now time.Time) (SearchPageCursor, error) {
	invalid := func() (SearchPageCursor, error) {
		return SearchPageCursor{}, Violation(InvalidRequest, "CURSOR_INVALID")
	}
	if !codec.valid() || expectedBinding == [32]byte{} || len(encoded) == 0 || len(encoded) > searchCursorMaxEncoded {
		return invalid()
	}
	envelope, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	// Go的base64解码器会忽略CR/LF；重新编码核对还拒绝这些非规范表示。
	if err != nil || base64.RawURLEncoding.EncodeToString(envelope) != encoded {
		return invalid()
	}
	header := 1 + codec.aead.NonceSize()
	minimum := header + searchCursorPlainHeader + 1 + codec.aead.Overhead() + sha256.Size
	maximum := header + searchCursorPlainHeader + searchCursorMaxAnchor + codec.aead.Overhead() + sha256.Size
	if len(envelope) < minimum || len(envelope) > maximum || envelope[0] != searchCursorVersion {
		return invalid()
	}
	sealed, signature := envelope[:len(envelope)-sha256.Size], envelope[len(envelope)-sha256.Size:]
	// 外层先验证版本/nonce/密文整体，再进入GCM；所有认证错误只返回固定码。
	if !hmac.Equal(signature, codec.signature(sealed)) {
		return invalid()
	}
	plain, err := codec.aead.Open(nil, sealed[1:header], sealed[header:], []byte(searchCursorPurpose))
	if err != nil {
		return invalid()
	}
	defer clear(plain)
	if len(plain) < searchCursorPlainHeader || int(binary.BigEndian.Uint16(plain[41:43])) != len(plain)-searchCursorPlainHeader {
		return invalid()
	}
	cursor := SearchPageCursor{Prefix: plain[40], Until: int64(binary.BigEndian.Uint64(plain[:8]))}
	copy(cursor.Binding[:], plain[8:40])
	cursor.Anchor = plain[searchCursorPlainHeader:]
	if !validSearchPageCursor(cursor) {
		return invalid()
	}
	if subtle.ConstantTimeCompare(cursor.Binding[:], expectedBinding[:]) != 1 {
		return SearchPageCursor{}, Violation(StateConflict, "CURSOR_STALE")
	}
	if cursor.Until <= now.UnixMilli() {
		return SearchPageCursor{}, Violation(StateConflict, "CURSOR_EXPIRED")
	}
	// 解密临时缓冲退出时清除；返回独立Anchor，连续Open或调用方改写均互不影响。
	cursor.Anchor = append([]byte(nil), cursor.Anchor...)
	return cursor, nil
}

// search_manifest.go：保留原实现，按业务合并文件。

// SearchManifestMaxUnits沿冻结HTTP MainView.units/StructureRevision.entries的10000上限，
// 只限制异常持久清单读取规模，不另设搜索业务文档数或正文预算。
const SearchManifestMaxUnits = 10000

// SearchManifestUnit是Doc私有固定清单的元数据；ByteSize用于读取正文之前预留预算。
// 它不包含正文/摘要或RPC DTO，读取后仍须验证实际字节、hash、schema与完整单位绑定。
type SearchManifestUnit struct {
	Unit     uint64
	Version  uint64
	ByteSize uint64
}

// search_query.go：保留原实现，按业务合并文件。

// SearchQuery是参数解析后的domain值，不是另一份HTTP/RPC DTO；SpaceID=0表示未限定空间。
// HTTP limit属于Doc整页，不能拿它替换Search固定候选批大小。
type SearchQuery struct {
	Text    string
	Limit   int
	SpaceID uint64
	Cursor  string
}

func ParseSearchQuery(values url.Values) (SearchQuery, error) {
	// 必须先检查完整参数集合，不能用Get悄悄取重复参数的第一项或丢掉伪造身份参数。
	// handler须用url.ParseQuery并核对其error；URL.Query会先隐藏错误，domain无法恢复原字节。
	for name, items := range values {
		switch name {
		case "q", "limit", "space_id", "cursor":
			if len(items) != 1 {
				return SearchQuery{}, Violation(InvalidRequest, "SEARCH_PARAMETERS_INVALID")
			}
		default:
			return SearchQuery{}, Violation(InvalidRequest, "SEARCH_PARAMETERS_INVALID")
		}
	}
	query := SearchQuery{Text: values.Get("q"), Limit: 20, Cursor: values.Get("cursor")}
	if items, present := values["limit"]; present {
		value := items[0]
		if len(value) < 1 || len(value) > 3 {
			return SearchQuery{}, Violation(InvalidRequest, "LIMIT_INVALID")
		}
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 || strconv.Itoa(limit) != value {
			return SearchQuery{}, Violation(InvalidRequest, "LIMIT_INVALID")
		}
		query.Limit = limit
	}
	if space := values.Get("space_id"); space != "" {
		id, err := CanonicalID(space)
		if err != nil {
			return SearchQuery{}, err
		}
		query.SpaceID = id
	}
	if err := query.validate(); err != nil {
		return SearchQuery{}, err
	}
	return query, nil
}

func (query SearchQuery) validate() error {
	// 先按UTF8最大字节数截住无界输入，再计Unicode码点；保留原词法，不trim/NFC改写。
	if len(query.Text) > 500*utf8.UTFMax || !utf8.ValidString(query.Text) ||
		utf8.RuneCountInString(query.Text) < 1 || utf8.RuneCountInString(query.Text) > 500 ||
		strings.TrimSpace(query.Text) == "" {
		return Violation(InvalidRequest, "SEARCH_QUERY_INVALID")
	}
	if query.Limit < 1 || query.Limit > 100 {
		return Violation(InvalidRequest, "LIMIT_INVALID")
	}
	if query.Cursor == "" {
		return nil
	}
	if len(query.Cursor) > searchCursorMaxEncoded {
		return Violation(InvalidRequest, "CURSOR_INVALID")
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(query.Cursor)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != query.Cursor {
		return Violation(InvalidRequest, "CURSOR_INVALID")
	}
	// 规范base64仍可是不可信内容；MAC、版本、字段和绝对期限留给SearchCursorCodec。
	return nil
}

// Binding只标记已核验输入，不授予任何权限；session必须来自OptionalActor真实Proof.SessionRef，
// 不能传Cookie token、用户ID或HTTP自报scope。匿名与登录不能互相降级。
func (query SearchQuery) Binding(anonymous bool, session []byte) ([32]byte, error) {
	if err := query.validate(); err != nil {
		return [32]byte{}, err
	}
	if (anonymous && len(session) != 0) || (!anonymous && (len(session) != 32 || subtle.ConstantTimeCompare(session, make([]byte, 32)) == 1)) {
		return [32]byte{}, Violation(InvalidRequest, "SEARCH_BINDING_INVALID")
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte("collaboration/document/search-query-binding/v1\x00"))
	writeSearchQueryField(digest, []byte(query.Text))
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], uint64(query.Limit))
	writeSearchQueryField(digest, number[:])
	binary.BigEndian.PutUint64(number[:], query.SpaceID)
	writeSearchQueryField(digest, number[:])
	audience := byte(0)
	if anonymous {
		audience = 1
	}
	writeSearchQueryField(digest, []byte{audience})
	writeSearchQueryField(digest, session)
	// 显式长度前缀与版本用途标签固定各字段边界；不拼接可能撞界的分隔文本。
	// Cursor故意不入绑定，否则同一查询的下一页无法用原Binding解密。
	var binding [32]byte
	copy(binding[:], digest.Sum(nil))
	return binding, nil
}

func writeSearchQueryField(digest hash.Hash, value []byte) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write(value)
}

// search_rpc.go：保留原实现，按业务合并文件。

// EnvelopeSearchAnchor只为冻结RPC选择下一批或重放，绝不解析SCB内部或验证Search的MAC。
// Doc的HTTP外层另用AEAD保护；这里保留opaque原字节，位置及签名权威仍在Search。
func EnvelopeSearchAnchor(anchor []byte, replay bool) ([]byte, error) {
	const header = 7
	if len(anchor) == 0 || len(anchor) > searchCursorMaxAnchor-header {
		return nil, Violation(InvalidRequest, "CURSOR_INVALID")
	}
	envelope := make([]byte, header+len(anchor))
	copy(envelope, []byte{'S', 'C', 'E', 1})
	envelope[4] = 1
	if replay {
		envelope[4] = 2
	}
	binary.BigEndian.PutUint16(envelope[5:7], uint16(len(anchor)))
	copy(envelope[header:], anchor)
	return envelope, nil
}
````

UTF-8/LF全文SHA-256：`9975a9bff62238e6064e6c28ae5711f95791357626b2f5721df99970ac153010`。[参考实现](../../../../services/document/internal/domain/document/search_models.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
