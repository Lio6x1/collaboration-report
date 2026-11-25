// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	"collaboration/platform"
	"collaboration/search/internal/client/document"
	"collaboration/search/internal/client/elastic"
	"collaboration/search/internal/client/logsource"
	"collaboration/search/internal/logic/indexing"
	"collaboration/search/internal/logic/rebuild"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
)

// rebuild.go：保留原实现，按业务合并文件。

func NewRebuild(store *platform.Store, cfg platform.Config) (*rebuild.JobService, error) {
	if store == nil || store.DB == nil || store.Role != "search" || cfg.Role != "search" || cfg.Search == nil || !filepath.IsAbs(cfg.Root) || cfg.Search.Owner == "" || cfg.Search.Alias == "" {
		return nil, errors.New("SEARCH_REBUILD_CONFIG_INVALID")
	}
	return &rebuild.JobService{DB: store.DB, Owner: cfg.Search.Owner, Alias: cfg.Search.Alias, AcceptedDir: filepath.Join(cfg.Root, ".runtime", "rebuild", "search", "accepted")}, nil
}

// rebuild_worker.go：保留原实现，按业务合并文件。

type RebuildWorker struct {
	logic       *rebuild.Seeder
	replay      *rebuild.Replayer
	index       *elastic.Client
	logs        *logsource.Client
	coordinator *rebuild.Coordinator
}

func NewRebuildWorker(store *platform.Store, cfg platform.Config) (*RebuildWorker, error) {
	if store == nil || store.DB == nil || cfg.Role != "search" || cfg.Search == nil || !filepath.IsAbs(cfg.Root) {
		return nil, errors.New("SEARCH_REBUILD_CONFIG_INVALID")
	}
	source, err := document.NewSnapshots(cfg)
	if err != nil {
		return nil, err
	}
	sink, err := elastic.New(cfg.Search)
	if err != nil {
		return nil, err
	}
	logs, err := logsource.New(cfg.Search.Brokers, cfg.Search.Topic)
	if err != nil {
		sink.Close()
		return nil, err
	}
	// 种子不写事件账本，Processor的consumer字段不会参与此入口。
	writer := indexing.New(store.DB, source, sink, "")
	seed := &rebuild.Seeder{DB: store.DB, Owner: cfg.Search.Owner, Alias: cfg.Search.Alias,
		AcceptedDir: filepath.Join(cfg.Root, ".runtime", "rebuild", "search", "accepted"), Writer: writer, Indexes: sink, Logs: logs}
	replay := &rebuild.Replayer{Seed: seed, Logs: logs, Processor: func(consumer string) rebuild.ReplayProcessor {
		return indexing.New(store.DB, source, sink, consumer)
	}}
	coordinator := &rebuild.Coordinator{Barrier: &rebuild.Barrier{Seed: seed, Indexes: sink}, Validator: &rebuild.Validator{Seed: seed, Indexes: sink}}
	return &RebuildWorker{logic: seed, replay: replay, index: sink, logs: logs, coordinator: coordinator}, nil
}
func (s *RebuildWorker) Step(ctx context.Context) error {
	if err := s.logic.Step(ctx); err != nil {
		return err
	}
	if err := s.replay.Step(ctx); err != nil {
		return err
	}
	return s.coordinator.Step(ctx)
}

// RecoveryReady只判定持久切换屏障；普通consumer自己的Initialize仍负责日志恢复。
func (s *RebuildWorker) RecoveryReady(ctx context.Context) error {
	var id uint64
	err := s.logic.DB.QueryRowContext(ctx, `SELECT id FROM rebuild_job WHERE state='SWITCHING' LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_REBUILD_JOB_READ_FAILED")
	}
	return errors.New("SEARCH_GENERATION_RECOVERY_REQUIRED")
}
func (s *RebuildWorker) Close() { s.logs.Close(); s.index.Close() }
