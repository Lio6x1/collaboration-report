package client

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	searchRPCDeadline = 5 * time.Second
	searchRPCMaxBytes = 64 << 10
	searchRPCMaxFrame = 768
	searchRPCMaxSort  = 128
	searchRPCBatch    = 100
)

var searchPhysicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)

// Search不拥有连接，svc用Doc的真实mTLS身份建立并关闭peer；此层仅适配冻结RPC。
// 候选完整也不代表Doc读取权限，正式版及当前来源资格由整页用例再核验。
type Search struct {
	peer pb.SearchInternalClient
}

func NewSearch(peer pb.SearchInternalClient) *Search { return &Search{peer: peer} }

func searchIncomplete() error {
	return domain.Violation(domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
}

func searchBindingInvalid() error {
	return domain.Violation(domain.InvalidRequest, "SEARCH_BINDING_INVALID")
}

func (s *Search) SearchCandidates(ctx context.Context, input *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	if s == nil || s.peer == nil {
		return nil, searchIncomplete()
	}
	if !validSearchRequest(input) {
		return nil, searchBindingInvalid()
	}
	// WithTimeout保留原请求取消/值及更早的HTTP期限，不创建脱离原请求的重试。
	ctx, cancel := context.WithTimeout(ctx, searchRPCDeadline)
	defer cancel()
	if ctx.Err() != nil {
		return nil, searchIncomplete()
	}
	request := proto.Clone(input).(*pb.SearchCandidatesRequest)
	reply, err := s.peer.SearchCandidates(ctx, request, grpc.MaxCallRecvMsgSize(searchRPCMaxBytes))
	if err != nil {
		return nil, searchRPCError(err)
	}
	received := time.Now()
	if ctx.Err() != nil || !validSearchReply(reply, received) {
		return nil, searchIncomplete()
	}
	if reply.ExpiresUnixMs <= received.UnixMilli() {
		return nil, domain.Violation(domain.StateConflict, "CURSOR_EXPIRED")
	}
	// 不向用例暴露peer持有的切片，防止验证后变化或分页前缀改写影响原响应。
	return proto.Clone(reply).(*pb.SearchCandidatesReply), nil
}

func (s *Search) CloseCursor(ctx context.Context, input *pb.CloseCursorRequest) (*pb.OperationResult, error) {
	if s == nil || s.peer == nil {
		return nil, searchIncomplete()
	}
	// Close使用原SCB锚点，不套SCE；Search权威负责验签、scope及关闭资源的持久状态。
	if input == nil || len(input.Cursor) == 0 || len(input.Cursor) > searchRPCMaxFrame || !validSearchScope(input.Scope) || proto.Size(input) > searchRPCMaxBytes {
		return nil, searchBindingInvalid()
	}
	ctx, cancel := context.WithTimeout(ctx, searchRPCDeadline)
	defer cancel()
	if ctx.Err() != nil {
		return nil, searchIncomplete()
	}
	reply, err := s.peer.CloseCursor(ctx, proto.Clone(input).(*pb.CloseCursorRequest), grpc.MaxCallRecvMsgSize(searchRPCMaxBytes))
	if err != nil {
		return nil, searchRPCError(err)
	}
	if ctx.Err() != nil || !validSearchClose(reply) {
		return nil, searchIncomplete()
	}
	return proto.Clone(reply).(*pb.OperationResult), nil
}

func validSearchRequest(input *pb.SearchCandidatesRequest) bool {
	if input == nil || len(input.Query) > 500*utf8.UTFMax || !utf8.ValidString(input.Query) || utf8.RuneCountInString(input.Query) > 500 || strings.TrimSpace(input.Query) == "" || input.PageSize != searchRPCBatch || !validSearchScope(input.Scope) {
		return false
	}
	// exact UTF8摘要沿冻结RPC核对，不trim/NFC改写查询，也不生成或替换调用方真实scope。
	digest := sha256.Sum256([]byte(input.Query))
	if input.QueryDigest != hex.EncodeToString(digest[:]) || proto.Size(input) > searchRPCMaxBytes {
		return false
	}
	if len(input.Cursor) == 0 {
		return true
	}
	frame := input.Cursor
	return len(frame) > 7 && len(frame) <= searchRPCMaxFrame && bytes.Equal(frame[:4], []byte{'S', 'C', 'E', 1}) &&
		(frame[4] == 1 || frame[4] == 2) && int(binary.BigEndian.Uint16(frame[5:7])) == len(frame)-7
}

func validSearchScope(scope *pb.SearchScope) bool {
	if scope == nil || !searchNonzeroDigest(scope.ScopeHash) || len(scope.SpaceIds) > 1024 {
		return false
	}
	if scope.Anonymous {
		if len(scope.SessionRef) != 0 {
			return false
		}
	} else {
		var zero [sha256.Size]byte
		if len(scope.SessionRef) != sha256.Size || subtle.ConstantTimeCompare(scope.SessionRef, zero[:]) == 1 {
			return false
		}
	}
	for i, space := range scope.SpaceIds {
		if space == 0 || (i > 0 && space <= scope.SpaceIds[i-1]) {
			return false
		}
	}
	return true
}

func searchNonzeroDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	raw, err := hex.DecodeString(value)
	var zero [sha256.Size]byte
	return err == nil && hex.EncodeToString(raw) == value && subtle.ConstantTimeCompare(raw, zero[:]) != 1
}

func validSearchReply(reply *pb.SearchCandidatesReply, received time.Time) bool {
	// 建链预留发生在RPC发出之后；仅核收到时的最大TTL，不改写或续期原expires。
	if reply == nil || !reply.Complete || len(reply.Candidates) > searchRPCBatch || (reply.HasMore && len(reply.Candidates) != searchRPCBatch) ||
		reply.IndexGeneration == 0 || !searchPhysicalName.MatchString(reply.PhysicalIndex) || reply.ExpiresUnixMs <= 0 || reply.ExpiresUnixMs > received.Add(2*time.Minute).UnixMilli() ||
		len(reply.Continuation) == 0 || len(reply.Continuation) > searchRPCMaxFrame {
		return false
	}
	// EOF仍需保留终批锚点给批内lookahead重放；拒绝后续无法构造有界RPC的opaque字节。
	if _, err := domain.EnvelopeSearchAnchor(reply.Continuation, false); err != nil {
		return false
	}
	seen := make(map[uint64]bool, len(reply.Candidates))
	for _, candidate := range reply.Candidates {
		if candidate == nil || candidate.DocumentId == 0 || candidate.ReleaseId == 0 || candidate.AggregateSeq == 0 || seen[candidate.DocumentId] ||
			candidate.PhysicalIndex != reply.PhysicalIndex || !validSearchSort(candidate.SortJsonUtf8, candidate.DocumentId) {
			return false
		}
		seen[candidate.DocumentId] = true
	}
	return proto.Size(reply) <= searchRPCMaxBytes
}

func validSearchSort(raw []byte, document uint64) bool {
	if len(raw) == 0 || len(raw) > searchRPCMaxSort || !utf8.Valid(raw) {
		return false
	}
	var tuple []json.RawMessage
	if json.Unmarshal(raw, &tuple) != nil || len(tuple) != 3 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(tuple[0]))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return false
	}
	score, ok := value.(json.Number)
	if !ok {
		return false
	}
	finite, err := score.Float64()
	if err != nil || math.IsNaN(finite) || math.IsInf(finite, 0) {
		return false
	}
	// 只把score解析成数值检查有限性；Doc/shard直接核对原十进制，返回sort绝不重写。
	doc := bytes.TrimSpace(tuple[1])
	if !bytes.Equal(doc, []byte(`"`+strconv.FormatUint(document, 10)+`"`)) {
		return false
	}
	shard := bytes.TrimSpace(tuple[2])
	if len(shard) == 0 || len(shard) > 20 || (shard[0] == '0' && len(shard) > 1) {
		return false
	}
	for _, digit := range shard {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	_, err = strconv.ParseUint(string(shard), 10, 64)
	return err == nil
}

func validSearchClose(reply *pb.OperationResult) bool {
	if reply == nil || reply.Generation == 0 || !searchNonzeroDigest(reply.OperationId) || reply.TerminalRevision != 0 || len(reply.ResultIds) != 0 ||
		len(reply.ResultDigest) != 0 || len(reply.FixedResultJsonUtf8) != 0 || len(reply.CurrentStateJsonUtf8) != 0 || proto.Size(reply) > searchRPCMaxBytes {
		return false
	}
	// PROCESSING是持久关闭尚未完成，不能当作PIT已释放；仅精确状态/固定码组合通过。
	return (reply.State == pb.OperationResult_PROCESSING && reply.BusinessCode == "CURSOR_CLOSING") ||
		(reply.State == pb.OperationResult_SUCCEEDED && reply.BusinessCode == "CURSOR_CLOSED")
}

func searchRPCError(err error) error {
	// 同时匹配gRPC类别和封闭业务码，禁止把peer的SQL/ES正文带进HTTP或误判成过期。
	wire, ok := status.FromError(err)
	if !ok {
		return searchIncomplete()
	}
	switch wire.Code() {
	case codes.FailedPrecondition:
		switch wire.Message() {
		case "SEARCH_CURSOR_EXPIRED":
			return domain.Violation(domain.StateConflict, "CURSOR_EXPIRED")
		case "SEARCH_CURSOR_STALE", "SEARCH_CURSOR_CLOSED", "SEARCH_CURSOR_REPLAY_CHANGED":
			return domain.Violation(domain.StateConflict, "CURSOR_STALE")
		case "SEARCH_CURSOR_BINDING_MISMATCH":
			return domain.Violation(domain.StateConflict, "SEARCH_BINDING_INVALID")
		}
	case codes.InvalidArgument:
		switch wire.Message() {
		case "SEARCH_CURSOR_INVALID", "SEARCH_CURSOR_ANCHOR_INVALID", "SEARCH_CURSOR_ENVELOPE_INVALID":
			return domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
		case "SEARCH_QUERY_BINDING_INVALID", "SEARCH_SCOPE_INVALID", "SEARCH_CANDIDATE_BATCH_SIZE_INVALID":
			return searchBindingInvalid()
		}
	}
	return searchIncomplete()
}
