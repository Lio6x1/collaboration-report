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
