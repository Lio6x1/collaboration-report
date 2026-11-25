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
