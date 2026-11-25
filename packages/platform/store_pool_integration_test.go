package platform

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestStoreReusesBoundedWorkerBurstConnections(t *testing.T) {
	// 已观察到后台突发任务反复把池扩到8再缩到4，长期运行耗尽Windows临时端口。
	// 借用全部已允许连接后归还，证明下一轮复用同一有界池；不扩大并发或查询期限。
	store := integrationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for round := 0; round < 3; round++ {
		connections := make([]*sql.Conn, 0, 8)
		for len(connections) < 8 {
			conn, err := store.DB.Conn(ctx)
			if err != nil {
				for _, held := range connections {
					_ = held.Close()
				}
				t.Fatal(err)
			}
			connections = append(connections, conn)
		}
		for _, conn := range connections {
			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}
		}
		stats := store.DB.Stats()
		if stats.MaxOpenConnections != 8 || stats.OpenConnections != 8 || stats.Idle != 8 || stats.MaxIdleClosed != 0 {
			t.Fatalf("bounded worker burst discarded reusable connections: %+v", stats)
		}
	}
}
