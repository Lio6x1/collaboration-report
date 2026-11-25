package svc

import (
	"collaboration/platform"
	"collaboration/search/internal/client/elastic"
	"collaboration/search/internal/worker"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

func TestActualAssembledConsumerIndexesAndRestartsFromPersistedFrontier(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fixtureFile := filepath.Join(t.TempDir(), "search-index-fixture.json")
	command := exec.CommandContext(ctx, filepath.Join(config.Root, ".runtime", "toolchains", "node-v24.19.0-win-x64", "node.exe"), "--no-maglev", filepath.Join(config.Root, "tests", "search", "prepare-index-fixture.mjs"), fixtureFile)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("ACTUAL_CONSUMER_FIXTURE_FAILED bytes=%d", len(output))
	}
	raw, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatal("ACTUAL_CONSUMER_FIXTURE_UNAVAILABLE")
	}
	var fixture struct {
		Document, Release string
		Record            json.RawMessage
	}
	if json.Unmarshal(raw, &fixture) != nil {
		t.Fatal("ACTUAL_CONSUMER_FIXTURE_INVALID")
	}
	document, err := strconv.ParseUint(fixture.Document, 10, 64)
	if err != nil {
		t.Fatal("ACTUAL_CONSUMER_DOCUMENT_INVALID")
	}
	settings := *config.Search
	settings.Group = fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	settings.Topic = "actual-" + settings.Group
	settings.Alias = settings.Topic + "-alias"
	settings.InitialIndex = settings.Topic + "-g1"
	config.Search = &settings
	store, err := platform.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.DB.Close() })
	sink, err := elastic.New(config.Search)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sink.Close)
	transport := &kafka.Transport{DialTimeout: 2 * time.Second}
	t.Cleanup(transport.CloseIdleConnections)
	admin := &kafka.Client{Addr: kafka.TCP(settings.Brokers...), Transport: transport, Timeout: 5 * time.Second}
	created, err := admin.CreateTopics(ctx, &kafka.CreateTopicsRequest{Topics: []kafka.TopicConfig{{Topic: settings.Topic, NumPartitions: 3, ReplicationFactor: 1}}})
	if err != nil || created == nil || created.Errors[settings.Topic] != nil {
		t.Fatal("ACTUAL_CONSUMER_TOPIC_UNAVAILABLE")
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		request, _ := http.NewRequestWithContext(cleanup, "DELETE", settings.ElasticEndpoint+"/"+settings.InitialIndex+"/_alias/"+settings.Alias, nil)
		if response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request); err != nil {
			t.Error("ACTUAL_ALIAS_CLEANUP_FAILED")
		} else {
			response.Body.Close()
			if response.StatusCode != 200 && response.StatusCode != 404 {
				t.Error("ACTUAL_ALIAS_CLEANUP_FAILED")
			}
		}
		for _, query := range []string{"DELETE FROM event_origin WHERE consumer_name=?", "DELETE FROM consumed_event WHERE consumer_name=?", "DELETE FROM consumer_checkpoint WHERE consumer_name=?"} {
			if _, err := store.DB.ExecContext(cleanup, query, settings.Group); err != nil {
				t.Error("ACTUAL_CONSUMER_LEDGER_CLEANUP_FAILED")
			}
		}
		if _, err := store.DB.ExecContext(cleanup, "DELETE FROM index_application WHERE physical_index=?", settings.InitialIndex); err != nil {
			t.Error("ACTUAL_CONSUMER_APPLICATION_CLEANUP_FAILED")
		}
		if _, err := store.DB.ExecContext(cleanup, "DELETE FROM index_generation WHERE alias_name=?", settings.Alias); err != nil {
			t.Error("ACTUAL_CONSUMER_GENERATION_CLEANUP_FAILED")
		}
		if err := sink.DeleteOwnedIndex(cleanup, settings.InitialIndex); err != nil {
			t.Error(err)
		}
		deleted, err := admin.DeleteTopics(cleanup, &kafka.DeleteTopicsRequest{Topics: []string{settings.Topic}})
		if err != nil || deleted == nil || deleted.Errors[settings.Topic] != nil {
			t.Error("ACTUAL_CONSUMER_TOPIC_CLEANUP_FAILED")
		}
	})
	writer := &kafka.Writer{Addr: kafka.TCP(settings.Brokers...), Topic: settings.Topic, Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	if err := writer.WriteMessages(ctx, kafka.Message{Key: []byte(fixture.Document), Value: fixture.Record}); err != nil {
		t.Fatal("ACTUAL_CONSUMER_ORIGINAL_WRITE_FAILED")
	}
	writer.Close()
	consumer, err := NewConsumer(store, config)
	if err != nil {
		t.Fatal(err)
	}
	if consumer.Ready(ctx) == nil {
		t.Fatal("UNINITIALIZED_CONSUMER_WAS_READY")
	}
	start := func(service *Consumer) func() {
		loop, stop := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() { defer close(done); worker.Run(loop, service) }()
		finish := func() {
			stop()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Error("ACTUAL_CONSUMER_DID_NOT_STOP")
			}
		}
		t.Cleanup(finish)
		return finish
	}
	waitReady := func(service *Consumer) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		var failure error
		for time.Now().Before(deadline) {
			check, done := context.WithTimeout(ctx, time.Second)
			failure = service.Ready(check)
			done()
			if failure == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("ACTUAL_CONSUMER_NOT_READY code=%v", failure)
	}
	stopFirst := start(consumer)
	waitReady(consumer)
	stored, err := sink.Get(ctx, settings.InitialIndex, document)
	if err != nil || !stored.Visible || stored.Sequence != 1 || strconv.FormatUint(stored.Release, 10) != fixture.Release || stored.Title != "协作报告" {
		t.Fatal("ASSEMBLED_CONSUMER_DID_NOT_INDEX_CURRENT_AUTHORITY")
	}
	var revision uint64
	if store.DB.QueryRowContext(ctx, "SELECT revision FROM index_application WHERE physical_index=? AND document_id=?", settings.InitialIndex, document).Scan(&revision) != nil {
		t.Fatal("ACTUAL_CONSUMER_APPLICATION_UNAVAILABLE")
	}
	stopFirst()
	// 完整装配重启沿原SQL代次、原Kafka组和原检查点，不新增应用revision。
	restarted, err := NewConsumer(store, config)
	if err != nil {
		t.Fatal(err)
	}
	start(restarted)
	waitReady(restarted)
	var nextRevision uint64
	if store.DB.QueryRowContext(ctx, "SELECT revision FROM index_application WHERE physical_index=? AND document_id=?", settings.InitialIndex, document).Scan(&nextRevision) != nil || nextRevision != revision {
		t.Fatal("CONSUMER_RESTART_REAPPLIED_COMMITTED_AUTHORITY")
	}
}
