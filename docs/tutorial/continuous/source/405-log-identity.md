# services/search/internal/svc/log_identity.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/405-log-identity.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：创建依赖、注入构造函数；同一私有Store贯穿本服务。

主要声明：`bindConsumerLog`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`9a48f1a1caa2cadf495baacb45555971a01266740e4b29847bb4a92b4770dcc4`。[参考实现](../../../../services/search/internal/svc/log_identity.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
