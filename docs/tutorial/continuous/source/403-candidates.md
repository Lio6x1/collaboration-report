# services/search/internal/svc/candidates.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/403-candidates.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：创建依赖、注入构造函数；同一私有Store贯穿本服务。

主要声明：`Candidates`、`NewCandidates`、`SearchCandidates`、`CloseCursor`、`Cleanup`、`Ready`、`Close`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package svc

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"collaboration/search/internal/client/elastic"
	logic "collaboration/search/internal/logic/candidates"
	"collaboration/search/internal/logic/generation"
	"context"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"time"
)

type Candidates struct {
	usecase *logic.Candidates
	elastic *elastic.Client
	closed  atomic.Bool
}

func NewCandidates(store *platform.Store, config platform.Config) (*Candidates, error) {
	if store == nil || store.DB == nil || store.Role != "search" || config.Role != "search" || config.Search == nil {
		return nil, errors.New("SEARCH_CANDIDATE_CONFIG_INVALID")
	}
	key, err := hex.DecodeString(config.Search.CursorKey)
	if err != nil || hex.EncodeToString(key) != config.Search.CursorKey {
		return nil, errors.New("SEARCH_CURSOR_KEY_INVALID")
	}
	sink, err := elastic.New(config.Search)
	if err != nil {
		return nil, err
	}
	assembled := false
	defer func() {
		if !assembled {
			sink.Close()
		}
	}()
	guard, err := generation.New(store.DB, sink, config.Search.Alias, config.Search.InitialIndex)
	if err != nil {
		return nil, err
	}
	usecase, err := logic.New(store.DB, guard, sink, config.Search.Alias, key, time.Duration(config.Search.CursorTTLSeconds)*time.Second)
	if err != nil {
		return nil, err
	}
	assembled = true
	return &Candidates{usecase: usecase, elastic: sink}, nil
}

func (c *Candidates) SearchCandidates(ctx context.Context, req *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	if c.closed.Load() {
		return nil, errors.New("SEARCH_CANDIDATE_STOPPED")
	}
	return c.usecase.SearchCandidates(ctx, req)
}
func (c *Candidates) CloseCursor(ctx context.Context, req *pb.CloseCursorRequest) (*pb.OperationResult, error) {
	if c.closed.Load() {
		return nil, errors.New("SEARCH_CANDIDATE_STOPPED")
	}
	return c.usecase.CloseCursor(ctx, req)
}
func (c *Candidates) Cleanup(ctx context.Context) error { return c.usecase.Cleanup(ctx) }
func (c *Candidates) Ready(ctx context.Context) error {
	if c.closed.Load() {
		return errors.New("SEARCH_CANDIDATE_STOPPED")
	}
	return c.usecase.Ready(ctx)
}
func (c *Candidates) Close() {
	if c.closed.CompareAndSwap(false, true) {
		c.elastic.Close()
	}
}
````

UTF-8/LF全文SHA-256：`5f22ca9bfd490443ab90a36f0120a2c944949452e954ee5475714c39206ce818`。[参考实现](../../../../services/search/internal/svc/candidates.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
