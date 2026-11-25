package consumer

import (
	domain "collaboration/search/internal/domain/event"
	"testing"
)

func TestWindowRejectsLostHistoryAndBrokerPositionAheadOfSQL(t *testing.T) {
	// SQL没有消费过的位置已经被保留窗口越过，或broker提交领先SQL，
	// 都不能靠自动reset到最新offset恢复；合法的旧broker位置则允许原消息重读。
	cases := []struct {
		name                         string
		first, last, committed, next int64
		rejected                     bool
	}{
		{"valid-replay", 0, 5, 2, 3, false},
		{"new-group", 0, 5, -1, 0, false},
		{"lost-unprocessed-history", 4, 5, 4, 3, true},
		{"broker-ahead-of-sql", 0, 5, 4, 3, true},
		{"sql-past-observed-log", 0, 5, 5, 6, true},
		{"expired-old-broker-position", 2, 5, 1, 3, true},
		{"malformed-log-window", 4, 3, -1, 4, true},
	}
	for _, value := range cases {
		t.Run(value.name, func(t *testing.T) {
			window := map[int]domain.PartitionWindow{0: {First: value.first, Last: value.last, Committed: value.committed}, 1: {Committed: -1}, 2: {Committed: -1}}
			positions := map[int]int64{0: value.next, 1: 0, 2: 0}
			if err := validateWindow(window, positions); (err != nil) != value.rejected {
				t.Fatal("WINDOW_RECOVERY_DECISION_CHANGED")
			}
		})
	}
	if validateWindow(map[int]domain.PartitionWindow{}, map[int]int64{}) == nil {
		t.Fatal("MISSING_PARTITIONS_WERE_READY")
	}
}
