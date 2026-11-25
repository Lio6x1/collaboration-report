package platform

import (
	"context"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"os"
	"testing"
	"time"
)

func TestLocalPeerUsesBoundLoopbackWithoutDNSDelay(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_ACCOUNT_CONFIG")
	if file == "" {
		t.Skip("需要隔离开发服务")
	}
	cfg, err := LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	c, err := DialPeer(cfg, "document")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = health.NewHealthClient(c).Check(ctx, &health.HealthCheckRequest{}); err != nil {
		t.Fatal("bound local peer should fit finite deadline", err)
	}
}
