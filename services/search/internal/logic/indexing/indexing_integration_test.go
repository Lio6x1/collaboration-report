package indexing

import (
	"collaboration/platform"
	snapshots "collaboration/search/internal/client/document"
	"collaboration/search/internal/client/elastic"
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	repository "collaboration/search/internal/repository/event"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

type actualCase struct {
	config                   platform.Config
	store                    *platform.Store
	source                   *snapshots.Snapshots
	sink                     *elastic.Client
	event                    domain.Event
	position                 repository.Position
	group, physical, release string
	record                   []byte
}

func prepareActualCase(t *testing.T) *actualCase {
	t.Helper()
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile := filepath.Join(t.TempDir(), "search-index-fixture.json")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(config.Root, ".runtime", "toolchains", "node-v24.19.0-win-x64", "node.exe"),
		"--no-maglev", filepath.Join(config.Root, "tests", "search", "prepare-index-fixture.mjs"), fixtureFile)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("ACTUAL_INDEX_FIXTURE_FAILED exit=%t bytes=%d", err != nil, len(output))
	}
	raw, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatal("ACTUAL_INDEX_FIXTURE_UNAVAILABLE")
	}
	var fixture struct {
		Document, Release, Operation, Topic, Offset string
		Partition                                   int
		Record                                      json.RawMessage
	}
	if json.Unmarshal(raw, &fixture) != nil {
		t.Fatal("ACTUAL_INDEX_FIXTURE_INVALID")
	}
	events, err := domain.ParseCanal(fixture.Record)
	if err != nil {
		t.Fatal(err)
	}
	var original domain.Event
	for _, value := range events {
		if value.ID == fixture.Operation {
			original = value
		}
	}
	if original.ID == "" {
		t.Fatal("ACTUAL_ORIGINAL_EVENT_MISSING")
	}
	store, err := platform.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.DB.Close() })
	source, err := snapshots.NewSnapshots(config)
	if err != nil {
		t.Fatal(err)
	}
	sink, err := elastic.New(config.Search)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sink.Close)
	group := fmt.Sprintf("index-fixture-%d", time.Now().UnixNano())
	physical := "actual-" + group
	if err = sink.EnsureIndex(ctx, physical); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		store.DB.ExecContext(context.Background(), "DELETE FROM consumer_checkpoint WHERE consumer_name=?", group)
		store.DB.ExecContext(context.Background(), "DELETE FROM event_origin WHERE consumer_name=?", group)
		store.DB.ExecContext(context.Background(), "DELETE FROM consumed_event WHERE consumer_name=?", group)
		store.DB.ExecContext(context.Background(), "DELETE FROM index_application WHERE physical_index=?", physical)
		if err := sink.DeleteOwnedIndex(context.Background(), physical); err != nil {
			t.Error(err)
		}
	})
	offset, err := strconv.ParseInt(fixture.Offset, 10, 64)
	if err != nil {
		t.Fatal("ACTUAL_OFFSET_INVALID")
	}
	position := repository.Position{Topic: fixture.Topic, Partition: fixture.Partition, Offset: offset}
	return &actualCase{config: config, store: store, source: source, sink: sink, event: original, position: position, group: group,
		physical: physical, release: fixture.Release, record: fixture.Record}
}

func TestActualDocSnapshotESAndSQLUseOriginalCanalEvent(t *testing.T) {
	f := prepareActualCase(t)
	ctx := context.Background()
	processor := New(f.store.DB, f.source, f.sink, f.group)
	if err := processor.Event(ctx, f.physical, f.event, f.position); err != nil {
		t.Fatal(err)
	}
	stored, err := f.sink.Get(ctx, f.physical, f.event.Document)
	if err != nil || !stored.Visible || strconv.FormatUint(stored.Release, 10) != f.release || stored.Sequence != 1 || stored.Title != "协作报告" || len(stored.Chapters) != 1 {
		t.Fatal("CURRENT_FIXED_AUTHORITY_NOT_INDEXED")
	}
	// 原事件重读不能增加应用修订或替换原摘要；未知Doc不能生成隐藏墓碑。
	if err = processor.Event(ctx, f.physical, f.event, f.position); err != nil {
		t.Fatal(err)
	}
	tx, err := f.store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	receipt, err := repository.Receive(ctx, tx, f.group, f.event, f.position)
	if err != nil || receipt.State != "SUCCEEDED" || receipt.Applied.Digest != stored.Digest {
		t.Fatal("ORIGINAL_SUCCESS_NOT_REPLAYABLE")
	}
	if _, err = f.source.Snapshot(ctx, ^uint64(0), 0, 0); err == nil {
		t.Fatal("UNKNOWN_DOC_BECAME_DELETE")
	}
}

func TestActualRecordOnlyAdvancesCompleteContinuousSQLCheckpoint(t *testing.T) {
	f := prepareActualCase(t)
	ctx := context.Background()
	processor := New(f.store.DB, f.source, f.sink, f.group)
	// 独立消费者的第一条复制夹具从0开始；此处只验SQL记录边界，Kafka实际提交另验。
	position := repository.Position{Topic: f.group, Partition: 0, Offset: 0}
	if err := processor.Record(ctx, f.physical, position, f.record); err != nil {
		t.Fatal(err)
	}
	if err := processor.Record(ctx, f.physical, position, f.record); err != nil {
		t.Fatal(err)
	}
	position.Offset = 2
	if err := processor.Record(ctx, f.physical, position, f.record); err == nil {
		t.Fatal("INCOMPLETE_OFFSET_GAP_ACCEPTED")
	}
	var next, revision uint64
	if f.store.DB.QueryRowContext(ctx, "SELECT next_offset,revision FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=0", f.group, position.Topic).
		Scan(&next, &revision) != nil || next != 1 || revision != 1 {
		t.Fatal("DUPLICATE_OR_GAP_MOVED_CHECKPOINT")
	}
}

type interruptedIndexSink struct {
	real  *elastic.Client
	after func()
}

func (s interruptedIndexSink) Apply(ctx context.Context, physical string, authority index.Authority) (index.State, error) {
	actual, err := s.real.Apply(ctx, physical, authority)
	if err != nil {
		return actual, err
	}
	s.after()
	return actual, nil
}

func TestActualESSuccessBeforeSQLFailureRecoversSameOriginalRecord(t *testing.T) {
	f := prepareActualCase(t)
	interrupted, cancel := context.WithCancel(context.Background())
	defer cancel()
	position := repository.Position{Topic: f.group, Partition: 0, Offset: 0}
	// 关闭DB池不会中断已借出的事务；在真实ES成功后取消BeginTx的context，
	// 才会真实中断当前SQL事务。原事件已在此前事务落定，恢复仍必须使用原行。
	broken := New(f.store.DB, f.source, interruptedIndexSink{real: f.sink, after: cancel}, f.group)
	if err := broken.Record(interrupted, f.physical, position, f.record); err == nil {
		t.Fatal("SQL_FAILURE_AFTER_ES_REPORTED_SUCCESS")
	}
	ctx := context.Background()
	stored, err := f.sink.Get(ctx, f.physical, f.event.Document)
	if err != nil || !stored.Visible || stored.Sequence != 1 {
		t.Fatal("ES_DID_NOT_ACTUALLY_SUCCEED")
	}
	var count int
	if f.store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM consumer_checkpoint WHERE consumer_name=?", f.group).Scan(&count) != nil || count != 0 {
		t.Fatal("FAILED_SQL_ADVANCED_CHECKPOINT")
	}
	if f.store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM index_application WHERE physical_index=?", f.physical).Scan(&count) != nil || count != 0 {
		t.Fatal("ROLLED_BACK_WATERMARK_WAS_COMMITTED")
	}
	recovered := New(f.store.DB, f.source, f.sink, f.group)
	if err := recovered.Record(ctx, f.physical, position, f.record); err != nil {
		t.Fatal(err)
	}
	tx, err := f.store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	receipt, err := repository.Receive(ctx, tx, f.group, f.event, position)
	if err != nil || receipt.State != "SUCCEEDED" || receipt.Applied.Digest != stored.Digest {
		t.Fatal("ORIGINAL_EVENT_RECOVERY_CHANGED_AUTHORITY")
	}
}
