package document

import (
	"bytes"
	"collaboration/platform"
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

// 已固定旧REVERSE的兼容只读，不制造或更新旧业务记录。干净库没有历史行时由纯边界测试覆盖解码。
func TestActualLegacyReverseRoundStyleReadOnly(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_DOCUMENT_CONFIG")
	if file == "" {
		t.Skip("需要显式指定本项目隔离文档数据库")
	}
	cfg, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	ctx := context.Background()
	rows, err := store.DB.QueryContext(ctx, `SELECT r.id,r.request_id,CAST(r.document_id AS CHAR),t.payload
		FROM review_round r JOIN durable_task t ON t.kind='REVIEW_ROUND_CONTEXT' AND t.business_key=CAST(r.id AS CHAR) AND t.operation_id=r.submission_operation_id
		JOIN content_version v ON v.id=r.content_version_id AND v.source_kind='REVERSE'
		WHERE JSON_TYPE(JSON_EXTRACT(t.payload,'$.style_id'))='INTEGER' ORDER BY r.id DESC LIMIT 3`)
	if err != nil {
		t.Fatal(err)
	}
	type source struct {
		round, request uint64
		document       string
		raw            []byte
	}
	var originals []source
	for rows.Next() {
		var value source
		if err = rows.Scan(&value.round, &value.request, &value.document, &value.raw); err != nil {
			t.Fatal(err)
		}
		originals = append(originals, value)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	for _, original := range originals {
		tx, err := store.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		fixed, err := ReviewRoundRow(ctx, tx, original.request, original.round, original.document)
		if err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		var old struct {
			Style uint64 `json:"style_id"`
		}
		if err = json.Unmarshal(original.raw, &old); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if fixed.Style.Id != strconv.FormatUint(old.Style, 10) {
			tx.Rollback()
			t.Fatal("replaced the original style")
		}
		var after []byte
		if err = tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='REVIEW_ROUND_CONTEXT' AND business_key=?", fixed.Id).Scan(&after); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		tx.Rollback()
		if !bytes.Equal(after, original.raw) {
			t.Fatal("rewrote the fixed historical context")
		}
	}
	t.Logf("read %d actual legacy reverse rounds without changing their contexts", len(originals))
}
