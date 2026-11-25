package logsource

import (
	"collaboration/platform"
	"context"
	"os"
	"testing"
)

func TestActualCaptureBindsOriginalTopicIdentity(t *testing.T) {
	// 重建起点必须来自真实broker；重复读取可接受增长，但不能接受同名的新TopicID。
	// 本关口只读已有owner主题，不加入消费组、不提交位置、不删除或重建主题。
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil || config.Search == nil {
		t.Fatal("ACTUAL_SEARCH_CONFIG_INVALID")
	}
	client, err := New(config.Search.Brokers, config.Search.Topic)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	original, err := client.Capture(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := client.Capture(context.Background(), &original.Identity)
	if err != nil {
		t.Fatal(err)
	}
	for partition := range original.Partitions {
		if repeated.Partitions[partition].Last < original.Partitions[partition].Last {
			t.Fatal("ACTUAL_LOG_END_MOVED_BACKWARDS")
		}
	}
	wrong := original.Identity
	wrong.TopicID[0] ^= 1
	if _, err := client.Capture(context.Background(), &wrong); err == nil || err.Error() != "SEARCH_LOG_IDENTITY_CHANGED" {
		t.Fatal("ACTUAL_LOG_ACCEPTED_DIFFERENT_IDENTITY")
	}
}
