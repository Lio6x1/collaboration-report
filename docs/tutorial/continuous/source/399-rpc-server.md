# services/search/internal/server/rpc_server.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/399-rpc-server.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`RebuildOperations`、`NewSearchWithRebuild`、`StartRebuild`、`GetRebuild`、`rebuildError`、`Search`、`CandidateOperations`、`NewSearch`、`SearchCandidates`、`CloseCursor`、`candidateError`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package server

import (
	pb "collaboration/contracts/internalv1"
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
)

// rebuild.go：保留原实现，按业务合并文件。

type RebuildOperations interface {
	StartRebuild(context.Context, *pb.RebuildRequest) (*pb.OperationResult, error)
	GetRebuild(context.Context, *pb.OperationQuery) (*pb.OperationResult, error)
}

func NewSearchWithRebuild(candidates CandidateOperations, rebuild RebuildOperations) *Search {
	return &Search{candidates: candidates, rebuild: rebuild}
}

func (s *Search) StartRebuild(ctx context.Context, request *pb.RebuildRequest) (*pb.OperationResult, error) {
	if s.rebuild == nil {
		return nil, status.Error(codes.Unavailable, "SEARCH_REBUILD_UNAVAILABLE")
	}
	result, err := s.rebuild.StartRebuild(ctx, request)
	return result, rebuildError(err)
}

func (s *Search) GetRebuild(ctx context.Context, request *pb.OperationQuery) (*pb.OperationResult, error) {
	if s.rebuild == nil {
		return nil, status.Error(codes.Unavailable, "SEARCH_REBUILD_UNAVAILABLE")
	}
	result, err := s.rebuild.GetRebuild(ctx, request)
	return result, rebuildError(err)
}

func rebuildError(err error) error {
	if err == nil {
		return nil
	}
	code := err.Error()
	if !strings.HasPrefix(code, "SEARCH_REBUILD_") || len(code) > 64 {
		return status.Error(codes.Unavailable, "SEARCH_REBUILD_UNAVAILABLE")
	}
	for _, r := range code {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return status.Error(codes.Unavailable, "SEARCH_REBUILD_UNAVAILABLE")
		}
	}
	// 只有输入错误/确定冲突按拒绝处理；提交或依赖未知仍须查询原操作。
	if strings.HasSuffix(code, "_INVALID") {
		return status.Error(codes.InvalidArgument, code)
	}
	if strings.HasSuffix(code, "_CONFLICT") || strings.HasSuffix(code, "_MISMATCH") || strings.HasSuffix(code, "_NOT_IMPORTED") {
		return status.Error(codes.FailedPrecondition, code)
	}
	if strings.HasSuffix(code, "_NOT_FOUND") {
		return status.Error(codes.NotFound, code)
	}
	return status.Error(codes.Unavailable, code)
}

// search.go：保留原实现，按业务合并文件。

// Search只做冻结RPC的传输适配，候选、关闭和重建分别委派业务用例。
type Search struct {
	pb.UnimplementedSearchInternalServer
	candidates CandidateOperations
	rebuild    RebuildOperations
}

type CandidateOperations interface {
	SearchCandidates(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)
	CloseCursor(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error)
}

func NewSearch(usecases ...CandidateOperations) *Search {
	server := &Search{}
	if len(usecases) == 1 {
		server.candidates = usecases[0]
	}
	return server
}

func (s *Search) SearchCandidates(ctx context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	if s.candidates == nil {
		return nil, status.Error(codes.Unavailable, "SEARCH_CANDIDATE_UNAVAILABLE")
	}
	result, err := s.candidates.SearchCandidates(ctx, request)
	return result, candidateError(err)
}

func (s *Search) CloseCursor(ctx context.Context, request *pb.CloseCursorRequest) (*pb.OperationResult, error) {
	if s.candidates == nil {
		return nil, status.Error(codes.Unavailable, "SEARCH_CANDIDATE_UNAVAILABLE")
	}
	result, err := s.candidates.CloseCursor(ctx, request)
	return result, candidateError(err)
}

func candidateError(err error) error {
	if err == nil {
		return nil
	}
	code := err.Error()
	// 本层只交付固定业务码，不暴露SQL/ES原文；当前依赖均返回这些明确的封闭码。
	if !strings.HasPrefix(code, "SEARCH_") || len(code) > 64 {
		return status.Error(codes.Unavailable, "SEARCH_CANDIDATE_UNAVAILABLE")
	}
	for _, r := range code {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return status.Error(codes.Unavailable, "SEARCH_CANDIDATE_UNAVAILABLE")
		}
	}
	switch code {
	case "SEARCH_QUERY_BINDING_INVALID", "SEARCH_SCOPE_INVALID", "SEARCH_CURSOR_INVALID", "SEARCH_CURSOR_ANCHOR_INVALID", "SEARCH_CURSOR_ENVELOPE_INVALID", "SEARCH_CANDIDATE_BATCH_SIZE_INVALID":
		return status.Error(codes.InvalidArgument, code)
	case "SEARCH_CURSOR_EXPIRED", "SEARCH_CURSOR_STALE", "SEARCH_CURSOR_CLOSED", "SEARCH_CURSOR_BINDING_MISMATCH", "SEARCH_CURSOR_REPLAY_CHANGED":
		return status.Error(codes.FailedPrecondition, code)
	case "SEARCH_CURSOR_QUOTA_EXCEEDED", "SEARCH_CURSOR_BUSY", "SEARCH_CURSOR_CLOSE_BUSY":
		return status.Error(codes.ResourceExhausted, code)
	default:
		return status.Error(codes.Unavailable, code)
	}
}
````

UTF-8/LF全文SHA-256：`93069c8b983bd1c21cd9e5c7890834534c1e7b01b7c7e8396d40f7bdb71ae596`。[参考实现](../../../../services/search/internal/server/rpc_server.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
