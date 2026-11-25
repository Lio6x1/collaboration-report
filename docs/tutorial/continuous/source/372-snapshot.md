# services/search/internal/client/document/snapshot.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/372-snapshot.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`Snapshots`、`NewSnapshots`、`Current`、`Snapshot`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	"context"
	"errors"
	"google.golang.org/grpc"
	"time"
)

type Snapshots struct {
	configuration platform.Config
	validator     *codec.Validator
}

func NewSnapshots(configuration platform.Config) (*Snapshots, error) {
	if configuration.Role != "search" {
		return nil, errors.New("SEARCH_RPC_IDENTITY_INVALID")
	}
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, errors.New("SEARCH_CONTENT_SCHEMA_UNAVAILABLE")
	}
	return &Snapshots{configuration: configuration, validator: validator}, nil
}

func (s *Snapshots) Current(ctx context.Context, event domain.Event) (index.Authority, error) {
	return s.Snapshot(ctx, event.Document, event.Sequence, event.Release)
}

func (s *Snapshots) Snapshot(ctx context.Context, document, minimum, release uint64) (index.Authority, error) {
	var result index.Authority
	peer, err := platform.DialPeer(s.configuration, "document")
	if err != nil {
		return result, errors.New("SEARCH_DOC_RPC_UNAVAILABLE")
	}
	defer peer.Close()
	ctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	reply, err := pb.NewDocumentInternalClient(peer).GetPublicSnapshot(ctx, &pb.PublicSnapshotRequest{
		DocumentId: document, AggregateSeq: minimum, ExpectedReleaseId: release,
	}, grpc.MaxCallRecvMsgSize(32<<20))
	// 包括NotFound在内的未知返回都不能解释成删除。隐藏必须来自Doc的完整当前权威回复。
	if err != nil {
		return result, errors.New("SEARCH_DOC_SNAPSHOT_UNKNOWN")
	}
	return index.FromSnapshot(s.validator, reply, document, minimum)
}
````

UTF-8/LF全文SHA-256：`fcc9290355e4735f7bc532e642555e16db75d17d315af3aa374a7d0195c6655a`。[参考实现](../../../../services/search/internal/client/document/snapshot.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
