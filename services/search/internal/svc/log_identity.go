package svc

import (
	"collaboration/platform"
	"collaboration/search/internal/client/logsource"
	event "collaboration/search/internal/repository/event"
	generation "collaboration/search/internal/repository/generation"
	rebuild "collaboration/search/internal/repository/rebuild"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
)

// bindConsumerLog在首次持久绑定时保守处理旧检查点。已有消费历史必须有本owner
// 先前签名prepare作为身份依据；不能仅凭今天的同名主题把历史offset重新解释。
func bindConsumerLog(ctx context.Context, db *sql.DB, cfg *platform.SearchConfig, identity logsource.Identity) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_LOG_IDENTITY_TRANSACTION_FAILED")
	}
	defer tx.Rollback()
	if _, found, err := generation.Find(ctx, tx, cfg.Alias, false); err != nil {
		return err
	} else if !found {
		return errors.New("SEARCH_GENERATION_MISSING")
	}
	var exists uint64
	if tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_log_identity WHERE consumer_name=?`, cfg.Group).Scan(&exists) != nil {
		return errors.New("SEARCH_LOG_IDENTITY_READ_FAILED")
	}
	if exists == 0 {
		var history uint64
		if tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_checkpoint WHERE consumer_name=?`, cfg.Group).Scan(&history) != nil {
			return errors.New("SEARCH_CHECKPOINT_READ_FAILED")
		}
		if history > 0 {
			var operation string
			if tx.QueryRowContext(ctx, `SELECT operation_id FROM rebuild_binding WHERE alias_name=? ORDER BY created_at,operation_id LIMIT 1`, cfg.Alias).Scan(&operation) != nil {
				return errors.New("SEARCH_LOG_IDENTITY_BOOTSTRAP_REQUIRED")
			}
			binding, found, err := rebuild.Find(ctx, tx, operation)
			if err != nil {
				return err
			}
			p := binding.Prepare
			if !found || p.Owner != cfg.Owner || p.Alias != cfg.Alias || p.Topic != identity.Topic || p.Cluster != identity.Cluster || p.TopicID != hex.EncodeToString(identity.TopicID[:]) {
				return errors.New("SEARCH_LOG_IDENTITY_CHANGED")
			}
		}
	}
	if err := event.BindLogIdentity(ctx, tx, cfg.Group, identity.Topic, identity.Cluster, identity.TopicID); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_LOG_IDENTITY_COMMIT_UNKNOWN")
	}
	return nil
}
