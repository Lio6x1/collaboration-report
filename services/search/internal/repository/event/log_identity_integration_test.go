package event

import (
	"collaboration/platform"
	"context"
	"os"
	"testing"
)

func TestActualPersistentLogIdentityRejectsReplacement(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	cfg, err := platform.LoadConfig(file)
	if err != nil || cfg.Search == nil {
		t.Fatal("ACTUAL_SEARCH_CONFIG_INVALID")
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_DATABASE_UNAVAILABLE")
	}
	defer store.DB.Close()
	ctx := context.Background()
	var topic, cluster string
	var raw []byte
	if store.DB.QueryRowContext(ctx, `SELECT kafka_topic,cluster_id,topic_id FROM consumer_log_identity WHERE consumer_name=?`, cfg.Search.Group).Scan(&topic, &cluster, &raw) != nil || len(raw) != 16 {
		t.Fatal("ACTUAL_BOUND_IDENTITY_REQUIRED")
	}
	var original [16]byte
	copy(original[:], raw)
	// 不删除真实主题；用现有持久绑定验证替代UUID不能覆盖历史检查点的身份。
	// 真实broker同名主题重建反例由已验收UUID读取器覆盖，此处补私有SQL的组合关口。
	changed := original
	changed[0] ^= 0xff
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal("ACTUAL_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if err := BindLogIdentity(ctx, tx, cfg.Search.Group, topic, cluster, changed); err == nil || err.Error() != "SEARCH_LOG_IDENTITY_CHANGED" {
		t.Fatal("REPLACEMENT_LOG_IDENTITY_ACCEPTED")
	}
	if err := BindLogIdentity(ctx, tx, cfg.Search.Group, topic, cluster, original); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal("ACTUAL_ROLLBACK_FAILED")
	}
	var after []byte
	if store.DB.QueryRowContext(ctx, `SELECT topic_id FROM consumer_log_identity WHERE consumer_name=?`, cfg.Search.Group).Scan(&after) != nil || string(after) != string(raw) {
		t.Fatal("ORIGINAL_LOG_IDENTITY_CHANGED")
	}
}
