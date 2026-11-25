package document

import (
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"os"
	"slices"
	"testing"
	"time"
)

// 本测试验证真实MySQL的事务和双退互斥，不声称已经完成Account注销HTTP或排空ACK。
// 仅创建本测试的隔离空间/管理员事实，结束后按精确主键清理，不借用产品内容。
func TestActualAccountRetirementAtomicHandoffAndConcurrentExit(t *testing.T) {
	config, err := platform.LoadConfig(os.Getenv("COLLAB_TEST_DOCUMENT_CONFIG"))
	if err != nil {
		t.Fatal("ACTUAL_DOCUMENT_CONFIG_REQUIRED")
	}
	store, err := platform.OpenStore(config)
	if err != nil {
		t.Fatal("ACTUAL_DOCUMENT_DATABASE_UNAVAILABLE")
	}
	t.Cleanup(func() { _ = store.DB.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	var entropy [16]byte
	if _, err = rand.Read(entropy[:]); err != nil {
		t.Fatal(err)
	}
	// 高位随机编号仅是私有SQL样本身份，不冒充Account准入。
	first := binary.BigEndian.Uint64(entropy[:8]) | (uint64(1) << 63)
	if first == ^uint64(0) {
		first--
	}
	users := []uint64{first, first + 1}
	operations := []string{"retirement-a-" + hex.EncodeToString(entropy[:]), "retirement-b-" + hex.EncodeToString(entropy[:])}
	for _, user := range users {
		var existing int
		if err := store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM access_gate WHERE scope_key=?", "identity:"+domain.TextID(user)).Scan(&existing); err != nil || existing != 0 {
			t.Fatal("FIXTURE_SOURCE_ALREADY_EXISTS")
		}
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	insert := func(statement string, args ...any) uint64 {
		t.Helper()
		result, err := tx.ExecContext(ctx, statement, args...)
		if err != nil {
			t.Fatal(err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return uint64(id)
	}
	space := insert("INSERT INTO space(kind,organization_id,lifecycle) VALUES('ORGANIZATION',?,'ACTIVE')", first)
	firstKB := insert("INSERT INTO knowledge_base(space_id,name,lifecycle) VALUES(?,'并发退出测试库','ACTIVE')", space)
	secondKB := insert("INSERT INTO knowledge_base(space_id,name,lifecycle) VALUES(?,'唯一管理员测试库','ACTIVE')", space)
	for _, pair := range [][2]uint64{{firstKB, users[0]}, {firstKB, users[1]}, {secondKB, users[0]}} {
		if _, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		// 必须先释放测试自有退休记录，再删除自有成员、库和空间，遵守真实外键。
		statements := []struct {
			sql  string
			args []any
		}{
			{"DELETE FROM admin_retirement WHERE knowledge_base_id IN (?,?)", []any{firstKB, secondKB}},
			{"DELETE FROM lifecycle_operation WHERE operation_id IN (?,?)", []any{operations[0], operations[1]}},
			{"DELETE FROM access_gate WHERE scope_key IN (?,?) AND last_revocation_id IN (?,?)", []any{"identity:" + domain.TextID(users[0]), "identity:" + domain.TextID(users[1]), operations[0], operations[1]}},
			{"DELETE FROM knowledge_base_member WHERE knowledge_base_id IN (?,?)", []any{firstKB, secondKB}},
			{"DELETE FROM knowledge_base WHERE id IN (?,?) AND space_id=?", []any{firstKB, secondKB, space}},
			{"DELETE FROM space WHERE id=?", []any{space}},
		}
		for _, statement := range statements {
			if _, err := store.DB.ExecContext(cleanup, statement.sql, statement.args...); err != nil {
				t.Errorf("OWNED_RETIREMENT_FIXTURE_CLEANUP: %v", err)
			}
		}
	})
	service := NewAccountRetirements(store.DB, nil)
	bindings := []domain.AccountRetirementBinding{
		{Operation: operations[0], User: users[0], IdentityEpoch: 1, ClosingOrganizations: []uint64{}},
		{Operation: operations[1], User: users[1], IdentityEpoch: 1, ClosingOrganizations: []uint64{}},
	}
	blocked, err := service.Prepare(ctx, bindings[0])
	if err != nil || blocked.State != "HANDOFF_REQUIRED" || !slices.Equal(blocked.RequiredKnowledgeBases, []uint64{secondKB}) || len(blocked.ReservedKnowledgeBases) != 0 {
		t.Fatalf("HANDOFF_MUST_KEEP_ALL_SOURCES_OPEN: %+v %v", blocked, err)
	}
	var count int
	if err := store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_retirement WHERE knowledge_base_id IN (?,?)", firstKB, secondKB).Scan(&count); err != nil || count != 0 {
		t.Fatal("PARTIAL_RETIREMENT_RESERVATION_COMMITTED")
	}
	if err := store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM access_gate WHERE scope_key=?", "identity:"+domain.TextID(users[0])).Scan(&count); err != nil || count != 0 {
		t.Fatal("HANDOFF_FROZE_IDENTITY_SOURCE")
	}
	if _, err := store.DB.ExecContext(ctx, "INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", secondKB, users[1]); err != nil {
		t.Fatal(err)
	}
	// 模拟早已开始的角色变更事务：先建立RR快照，再等待注销预留完成。
	// 后续取得知识库锁时，最后管理员判断必须看见新预留，不能复用旧COUNT快照。
	roleTx, err := store.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer roleTx.Rollback()
	if err := roleTx.QueryRowContext(ctx, "SELECT COUNT(*) FROM knowledge_base_member WHERE knowledge_base_id=?", firstKB).Scan(&count); err != nil || count != 2 {
		t.Fatal("ROLE_SNAPSHOT_NOT_ESTABLISHED")
	}
	type answer struct {
		index  int
		result domain.AccountRetirementResult
		err    error
	}
	answers := make(chan answer, 2)
	start := make(chan struct{})
	for index := range bindings {
		go func(index int) {
			<-start
			result, err := service.Prepare(ctx, bindings[index])
			answers <- answer{index, result, err}
		}(index)
	}
	close(start)
	winner, losers := -1, 0
	// 即使一侧失败也先收齐另一个事务，避免测试清理与尚在提交的事务交叉。
	completed := []answer{<-answers, <-answers}
	for _, answer := range completed {
		if answer.err != nil {
			t.Fatalf("CONCURRENT_RETIREMENT_FAILED: %v", answer.err)
		}
		switch answer.result.State {
		case "FROZEN":
			if winner != -1 {
				t.Fatal("BOTH_LAST_ADMINISTRATORS_RETIRED")
			}
			winner = answer.index
			if !slices.Equal(answer.result.ReservedKnowledgeBases, []uint64{firstKB, secondKB}) {
				t.Fatal("RETIREMENT_LOST_KNOWLEDGE_BASE")
			}
		case "HANDOFF_REQUIRED":
			losers++
		default:
			t.Fatalf("UNEXPECTED_RETIREMENT_STATE: %s", answer.result.State)
		}
	}
	if winner < 0 || losers != 1 {
		t.Fatal("LAST_ADMINISTRATOR_NOT_PROTECTED")
	}
	if _, _, err := repository.LockedKnowledgeBase(ctx, roleTx, domain.TextID(firstKB)); err != nil {
		t.Fatal(err)
	}
	available, err := repository.AvailableKBAdministrators(ctx, roleTx, firstKB)
	if err != nil || available != 1 {
		t.Fatalf("ROLE_CHANGE_MISSED_COMMITTED_RETIREMENT: available=%d err=%v", available, err)
	}
	if err := roleTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	replayed, err := service.Prepare(ctx, bindings[winner])
	if err != nil || replayed.State != "FROZEN" || len(replayed.ReservedKnowledgeBases) != 2 {
		t.Fatal("ORIGINAL_RETIREMENT_NOT_REPLAYABLE")
	}
	changed := bindings[winner]
	changed.ClosingOrganizations = []uint64{first}
	if _, err := service.Prepare(ctx, changed); err == nil {
		t.Fatal("RETIREMENT_PLAN_CAN_BE_REBOUND")
	}
	check, err := store.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer check.Rollback()
	if err := check.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_retirement WHERE knowledge_base_id IN (?,?) AND state='RESERVED'", firstKB, secondKB).Scan(&count); err != nil || count != 2 {
		t.Fatal("RETIREMENT_RESERVATION_COUNT_CHANGED")
	}
}
