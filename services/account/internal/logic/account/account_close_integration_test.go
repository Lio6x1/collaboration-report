package account

import (
	"bytes"
	"collaboration/account/internal/client"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type lostAccountClosePreparation struct {
	peer AccountClosurePreparer
	lost bool
}

func (p *lostAccountClosePreparation) Prepare(ctx context.Context, binding platform.AccountClosureBinding) (platform.AccountClosurePreparation, error) {
	result, err := p.peer.Prepare(ctx, binding)
	if err == nil && result.State == "FROZEN" && !p.lost {
		p.lost = true
		return platform.AccountClosurePreparation{}, errors.New("TEST_DOC_COMMITTED_REPLY_LOST")
	}
	return result, err
}

func (p *lostAccountClosePreparation) Cancel(ctx context.Context, binding platform.AccountClosureBinding) error {
	return p.peer.Cancel(ctx, binding)
}

// 直接建立自有SQL身份/组织图用于验证准备协议，跨服务部分走运行中的真实mTLS。
// 这不声称公开注销页面、保留计划或物理清理已上线。
func TestActualAccountCloseCaptureAndAuthoritativeCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	accountConfig, err := platform.LoadConfig(os.Getenv("COLLAB_TEST_ACCOUNT_CONFIG"))
	if err != nil {
		t.Fatal("ACTUAL_ACCOUNT_CONFIG_REQUIRED")
	}
	docConfig, err := platform.LoadConfig(os.Getenv("COLLAB_TEST_DOCUMENT_CONFIG"))
	if err != nil {
		t.Fatal("ACTUAL_DOCUMENT_CONFIG_REQUIRED")
	}
	accountStore, err := platform.OpenStore(accountConfig)
	if err != nil {
		t.Fatal("ACTUAL_ACCOUNT_DATABASE_UNAVAILABLE")
	}
	t.Cleanup(func() { _ = accountStore.DB.Close() })
	docStore, err := platform.OpenStore(docConfig)
	if err != nil {
		t.Fatal("ACTUAL_DOCUMENT_DATABASE_UNAVAILABLE")
	}
	t.Cleanup(func() { _ = docStore.DB.Close() })
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		t.Fatal(err)
	}
	operation := "account-close-" + hex.EncodeToString(entropy[:12])
	password := "Closure-fixture-" + hex.EncodeToString(entropy[12:20])
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	insert := func(db *sql.DB, statement string, args ...any) uint64 {
		t.Helper()
		result, err := db.ExecContext(ctx, statement, args...)
		if err != nil {
			t.Fatal(err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return uint64(id)
	}
	user := insert(accountStore.DB, "INSERT INTO user(display_name,password_hash,password_algorithm,state) VALUES('注销准备测试',?,'bcrypt','ACTIVE')", hash)
	other := insert(accountStore.DB, "INSERT INTO user(display_name,password_hash,password_algorithm,state) VALUES('保留管理员测试',?,'bcrypt','ACTIVE')", hash)
	organization := insert(accountStore.DB, "INSERT INTO organization(name,owner_id,lifecycle) VALUES('注销准备测试组织',?,'ACTIVE')", other)
	for _, id := range []uint64{user, other} {
		if _, err := accountStore.DB.ExecContext(ctx, "INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", organization, id); err != nil {
			t.Fatal(err)
		}
	}
	space := insert(docStore.DB, "INSERT INTO space(kind,organization_id,lifecycle) VALUES('ORGANIZATION',?,'ACTIVE')", organization)
	kb := insert(docStore.DB, "INSERT INTO knowledge_base(space_id,name,lifecycle) VALUES(?,'注销预留测试库','ACTIVE')", space)
	for _, id := range []uint64{user, other} {
		if _, err := docStore.DB.ExecContext(ctx, "INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", kb, id); err != nil {
			t.Fatal(err)
		}
	}
	expires := time.Now().UTC().Add(time.Hour)
	if _, err := accountStore.DB.ExecContext(ctx, "INSERT INTO session(id,user_id,csrf_hash,state,expires_at,last_seen_at) VALUES(?,?,?,'ACTIVE',?,UTC_TIMESTAMP(6))", entropy[:], user, entropy[:], expires); err != nil {
		t.Fatal(err)
	}
	otherSession := sha256.Sum256(entropy[:])
	if _, err := accountStore.DB.ExecContext(ctx, "INSERT INTO session(id,user_id,csrf_hash,state,expires_at,last_seen_at) VALUES(?,?,?,'ACTIVE',?,UTC_TIMESTAMP(6))", otherSession[:], other, otherSession[:], expires); err != nil {
		t.Fatal(err)
	}
	successOperation := operation + "-success"
	unboundOperation := operation + "-unbound"
	var raceOrganization, raceApplication uint64
	var soleOrganization, soleSpace, soleKB uint64
	var personalSpace, restrictedKB uint64
	retentionDocuments := []closeDocumentFixture{}
	var departureEvent string
	retentionScopes := []struct {
		Key       string `json:"scope_key"`
		Epoch     uint64 `json:"epoch"`
		Operation string `json:"revocation_id"`
	}{}
	successScopes := accountCloseScopes(platform.AccountClosureBinding{Operation: successOperation, User: user, IdentityEpoch: 1}, []domain.AccountCloseOrganization{{ID: organization, Epoch: 1}})
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if departureEvent != "" {
			for _, item := range []struct {
				statement string
				args      []any
			}{
				{"DELETE FROM notification WHERE recipient_id=? AND event_id=? AND kind='RESPONSIBLE_DEPARTURE'", []any{other, departureEvent}},
				{"DELETE FROM durable_task WHERE kind='RESPONSIBLE_DEPARTURE_NOTIFICATION' AND business_key=?", []any{departureEvent}},
				{"DELETE FROM lifecycle_operation WHERE operation_id=? AND kind='OWNER_CHANGE'", []any{departureEvent}},
			} {
				if _, err := docStore.DB.ExecContext(cleanup, item.statement, item.args...); err != nil {
					t.Errorf("OWNED_CLOSURE_NOTICE_CLEANUP: %v", err)
				}
			}
		}
		for _, scope := range retentionScopes {
			if _, err := docStore.DB.ExecContext(cleanup, "DELETE FROM durable_task WHERE kind='AUTHORIZATION_CHANGE' AND business_key=?", scope.Operation); err != nil {
				t.Errorf("OWNED_RETENTION_TASK_CLEANUP: %v", err)
			}
			for _, statement := range []string{"DELETE FROM authorization_ack WHERE revocation_id=?", "DELETE FROM authorization_change WHERE revocation_id=?"} {
				if _, err := docStore.DB.ExecContext(cleanup, statement, scope.Operation); err != nil {
					t.Errorf("OWNED_RETENTION_FENCE_CLEANUP: %v", err)
				}
			}
			if _, err := docStore.DB.ExecContext(cleanup, "DELETE FROM access_gate WHERE scope_key=? AND last_revocation_id=?", scope.Key, scope.Operation); err != nil {
				t.Errorf("OWNED_RETENTION_GATE_CLEANUP: %v", err)
			}
		}
		for _, d := range retentionDocuments {
			cleanupCloseDocumentFixture(t, cleanup, docStore.DB, d)
		}
		for _, scope := range successScopes {
			for dbIndex, db := range []*sql.DB{accountStore.DB, docStore.DB} {
				// Doc来源Fence不生成durable_task；不要用未带kind的business_key删除
				// 扫描整个Doc任务表并等待无关worker。只删除本测试确实创建的精确事实。
				if dbIndex == 0 {
					if _, err := db.ExecContext(cleanup, "DELETE FROM durable_task WHERE kind='ACCOUNT_CLOSE_FENCE' AND business_key=?", scope.Revocation); err != nil {
						t.Errorf("OWNED_FENCE_TASK_CLEANUP: %v", err)
					}
				}
				for _, statement := range []string{
					"DELETE FROM authorization_ack WHERE revocation_id=?",
					"DELETE FROM authorization_change WHERE revocation_id=?",
				} {
					if _, err := db.ExecContext(cleanup, statement, scope.Revocation); err != nil {
						t.Errorf("OWNED_FENCE_FIXTURE_CLEANUP database=%d statement=%s: %v", dbIndex, statement, err)
					}
				}
				if _, err := db.ExecContext(cleanup, "DELETE FROM access_gate WHERE scope_key=? AND last_revocation_id=?", scope.Scope, scope.Revocation); err != nil {
					t.Errorf("OWNED_FENCE_GATE_CLEANUP: %v", err)
				}
			}
		}
		// 清理只使用刚创建的精确主键，保留运行中项目的全部既有数据与证据。
		items := []struct {
			db        *sql.DB
			statement string
			args      []any
		}{
			{docStore.DB, "DELETE FROM knowledge_base WHERE id=?", []any{restrictedKB}},
			{docStore.DB, "DELETE FROM space WHERE id=?", []any{personalSpace}},
			{docStore.DB, "DELETE FROM knowledge_base_member WHERE knowledge_base_id=?", []any{soleKB}},
			{docStore.DB, "DELETE FROM knowledge_base WHERE id=?", []any{soleKB}},
			{docStore.DB, "DELETE FROM space WHERE id=?", []any{soleSpace}},
			{accountStore.DB, "DELETE FROM organization_member WHERE organization_id=?", []any{soleOrganization}},
			{accountStore.DB, "DELETE FROM organization WHERE id=?", []any{soleOrganization}},
			{accountStore.DB, "DELETE FROM organization_application WHERE id=?", []any{raceApplication}},
			{accountStore.DB, "DELETE FROM organization_member WHERE organization_id=?", []any{raceOrganization}},
			{accountStore.DB, "DELETE FROM organization WHERE id=?", []any{raceOrganization}},
			{docStore.DB, "DELETE FROM admin_retirement WHERE knowledge_base_id=?", []any{kb}},
			{docStore.DB, "DELETE FROM lifecycle_operation WHERE operation_id IN (?,?,?)", []any{operation, operation + "-lost", successOperation}},
			{docStore.DB, "DELETE FROM access_gate WHERE scope_key=? AND last_revocation_id=?", []any{"identity:" + strconv.FormatUint(user, 10), operation}},
			{docStore.DB, "DELETE FROM knowledge_base_member WHERE knowledge_base_id=?", []any{kb}},
			{docStore.DB, "DELETE FROM knowledge_base WHERE id=?", []any{kb}},
			{docStore.DB, "DELETE FROM space WHERE id=?", []any{space}},
			{accountStore.DB, "DELETE FROM durable_task WHERE operation_id IN (?,?,?)", []any{operation, operation + "-lost", successOperation}},
			{accountStore.DB, "DELETE FROM account_close WHERE operation_id IN (?,?,?,?)", []any{operation, operation + "-lost", successOperation, unboundOperation}},
			{accountStore.DB, "DELETE FROM operation_receipt WHERE operation_id IN (?,?,?,?)", []any{operation, operation + "-lost", successOperation, unboundOperation}},
			{accountStore.DB, "DELETE FROM session WHERE id IN (?,?)", []any{entropy[:], otherSession[:]}},
			{accountStore.DB, "DELETE FROM organization_member WHERE organization_id=?", []any{organization}},
			{accountStore.DB, "DELETE FROM organization WHERE id=?", []any{organization}},
			{accountStore.DB, "DELETE FROM user WHERE id IN (?,?)", []any{user, other}},
		}
		for _, item := range items {
			if _, err := item.db.ExecContext(cleanup, item.statement, item.args...); err != nil {
				t.Errorf("OWNED_CLOSURE_FIXTURE_CLEANUP: %v", err)
			}
		}
	})
	session := domain.Session{ID: entropy[:], UserID: strconv.FormatUint(user, 10), CredentialEpoch: "1", Epoch: 1, State: "ACTIVE", Expires: expires}
	service := NewAccountClosures(accountStore.DB, nil)
	input := dto.AccountClose{OperationId: operation, ExpectedIdentityEpoch: "1", CurrentPassword: password}
	fingerprint := func(value dto.AccountClose) []byte {
		raw, _ := json.Marshal(value)
		sum := sha256.Sum256(raw)
		return sum[:]
	}
	// Begin后、下游绑定前发生凭据变化应原号拒绝，不留下永久PREPARING占位。
	unbound := input
	unbound.OperationId = unboundOperation
	if _, err := service.Begin(ctx, session, unbound, fingerprint(unbound)); err != nil {
		t.Fatal(err)
	}
	if _, err := accountStore.DB.ExecContext(ctx, "UPDATE user SET credential_epoch=2 WHERE id=?", user); err != nil {
		t.Fatal(err)
	}
	unboundPlan, err := service.PrepareClose(ctx, unboundOperation, nil)
	if err != nil || unboundPlan.State != "REJECTED" {
		t.Fatal("UNBOUND_PREPARATION_NOT_TERMINATED", err)
	}
	var unboundTasks int
	if err := accountStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='ACCOUNT_CLOSE_CANCEL' AND business_key=?", unboundOperation).Scan(&unboundTasks); err != nil || unboundTasks != 0 {
		t.Fatal("UNBOUND_REJECTION_CREATED_FAKE_CANCEL", err)
	}
	// 自有SQL身份恢复到其初始代次，后面的Begin应证明原待处理记录已释放资格。
	if _, err := accountStore.DB.ExecContext(ctx, "UPDATE user SET credential_epoch=1 WHERE id=?", user); err != nil {
		t.Fatal(err)
	}
	// 在新组织审批与目标注销互斥时，审批等待目标user锁，然后当前读REVOKING。
	// 旧普通SELECT即使被成员外键等待，目标提交后仍会插入ACTIVE成员，故不能替代此锁。
	raceOrganization = insert(accountStore.DB, "INSERT INTO organization(name,owner_id,lifecycle) VALUES('注销审批竞态测试',?,'ACTIVE')", other)
	if _, err := accountStore.DB.ExecContext(ctx, "INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", raceOrganization, other); err != nil {
		t.Fatal(err)
	}
	raceApplication = insert(accountStore.DB, "INSERT INTO organization_application(organization_id,applicant_id,reason,state) VALUES(?,?,'注销竞态','PENDING')", raceOrganization, user)
	closingTX, err := accountStore.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closingTX.Rollback()
	if _, err := repository.NewAccountClosures(accountStore.DB).LockUser(ctx, closingTX, user); err != nil {
		t.Fatal(err)
	}
	applications := NewApplications(accountStore.DB, NewMutations(accountStore.DB, repository.NewMutation()), repository.NewApplications(accountStore.DB), repository.NewOrganizations(accountStore.DB), repository.NewInvitations(accountStore.DB), func(context.Context, string, uint64) {})
	approval := make(chan error, 1)
	go func() {
		_, err := applications.Decide(ctx, domain.Mutation{Session: domain.Session{ID: otherSession[:], UserID: strconv.FormatUint(other, 10), CredentialEpoch: "1"}, OperationID: operation + "-approval", Fingerprint: entropy[:]}, strconv.FormatUint(raceApplication, 10), dto.ApplicationReview{OperationId: operation + "-approval", Decision: "ACCEPT", ExpectedRevision: "1"})
		approval <- err
	}()
	waiting := 0
	for until := time.Now().Add(2 * time.Second); time.Now().Before(until); {
		if err := accountStore.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.processlist WHERE DB=DATABASE() AND ID<>CONNECTION_ID() AND (INFO LIKE '%SELECT state FROM user WHERE id=%' OR INFO LIKE '%INSERT INTO organization_member%')`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if waiting == 0 {
		t.Fatal("APPROVAL_DID_NOT_REACH_TARGET_LOCK")
	}
	if _, err := closingTX.ExecContext(ctx, "UPDATE user SET state='REVOKING' WHERE id=?", user); err != nil {
		t.Fatal(err)
	}
	if err := closingTX.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-approval; err == nil {
		t.Fatal("NEW_ORGANIZATION_ADMITTED_REVOKING_ACCOUNT")
	} else {
		var violation domain.RuleViolation
		if !errors.As(err, &violation) || violation.Code != "APPLICANT_NOT_ACTIVE" {
			t.Fatal("APPROVAL_FAILED_WITHOUT_CURRENT_IDENTITY_CHECK", err)
		}
	}
	var unexpectedMembers int
	if err := accountStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM organization_member WHERE organization_id=? AND user_id=?", raceOrganization, user).Scan(&unexpectedMembers); err != nil || unexpectedMembers != 0 {
		t.Fatal("CLOSING_ACCOUNT_GAINED_NEW_MEMBERSHIP", err)
	}
	if _, err := accountStore.DB.ExecContext(ctx, "UPDATE user SET state='ACTIVE' WHERE id=?", user); err != nil {
		t.Fatal(err)
	}
	wrong := input
	wrong.CurrentPassword = "incorrect-password"
	if _, err := service.Begin(ctx, session, wrong, fingerprint(wrong)); err == nil {
		t.Fatal("WRONG_PASSWORD_ACCEPTED")
	}
	plan, err := service.Begin(ctx, session, input, fingerprint(input))
	if err != nil || plan.State != "PREPARING" {
		t.Fatalf("ACCOUNT_CLOSE_CAPTURE_FAILED: %v", err)
	}
	if _, err := service.Begin(ctx, session, input, fingerprint(input)); err != nil {
		t.Fatal("ORIGINAL_CAPTURE_NOT_REPLAYABLE")
	}
	another := input
	another.OperationId += "-second"
	if _, err := service.Begin(ctx, session, another, fingerprint(another)); err == nil {
		t.Fatal("SECOND_PENDING_ACCOUNT_CLOSE_ACCEPTED")
	}
	binding := platform.AccountClosureBinding{Operation: operation, User: user, IdentityEpoch: 1, ClosingOrganizations: []uint64{}}
	if err := service.Bind(ctx, binding); err != nil {
		t.Fatal(err)
	}
	changed := binding
	changed.ClosingOrganizations = []uint64{organization}
	if err := service.Bind(ctx, changed); err == nil {
		t.Fatal("ACCOUNT_CLOSE_SCOPE_REBOUND")
	}
	httpClient, err := platform.HTTPPeer(accountConfig, "document")
	if err != nil {
		t.Fatal(err)
	}
	control := client.NewAccountClosureControl(httpClient, accountConfig.Peers["documentHttp"])
	docControlHTTP, err := platform.HTTPPeer(docConfig, "account")
	if err != nil {
		t.Fatal(err)
	}
	confirmFromDoc := func(binding platform.AccountClosureBinding) bool {
		t.Helper()
		raw, err := json.Marshal(binding)
		if err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequestWithContext(ctx, "POST", docConfig.Peers["accountHttp"]+"/infra/lifecycle/account-close-confirmation", bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := docControlHTTP.Do(request)
		if err != nil {
			t.Fatal("ACTUAL_ACCOUNT_CONFIRMATION_UNAVAILABLE", err)
		}
		defer response.Body.Close()
		var result platform.AccountClosureConfirmation
		if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&result) != nil {
			t.Fatal("ACTUAL_ACCOUNT_CONFIRMATION_NOT_VALID")
		}
		return result.Confirmed
	}
	prepared, err := control.Prepare(ctx, binding)
	if err != nil || prepared.State != "FROZEN" || len(prepared.ReservedKnowledgeBases) != 1 || prepared.ReservedKnowledgeBases[0] != kb {
		t.Fatalf("ACTUAL_DOC_PREPARATION_FAILED: %+v %v", prepared, err)
	}
	if err := control.Cancel(ctx, binding); err == nil {
		t.Fatal("UNCONFIRMED_ACCOUNT_CLOSE_CANCELLED")
	}
	var gate, reservation string
	if err := docStore.DB.QueryRowContext(ctx, "SELECT state FROM access_gate WHERE scope_key=?", "identity:"+session.UserID).Scan(&gate); err != nil || gate != "BLOCKED" {
		t.Fatal("UNKNOWN_DECISION_REOPENED_SOURCE")
	}
	if err := service.Reject(ctx, binding, "PREPARATION_CHANGED"); err != nil {
		t.Fatal(err)
	}
	if err := control.Cancel(ctx, binding); err != nil {
		t.Fatal("AUTHORITATIVE_REJECTION_NOT_RECOVERED", err)
	}
	if err := control.Cancel(ctx, binding); err != nil {
		t.Fatal("CANCELLATION_NOT_IDEMPOTENT", err)
	}
	if err := docStore.DB.QueryRowContext(ctx, "SELECT state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=?", kb, user).Scan(&reservation); err != nil || reservation != "CANCELLED" {
		t.Fatal("ADMINISTRATOR_RESERVATION_NOT_RELEASED")
	}
	prepared, err = control.Prepare(ctx, binding)
	if err != nil || prepared.State != "REJECTED" {
		t.Fatal("LATE_PREPARE_REVIVED_CANCELLED_PLAN")
	}
	connection, err := platform.DialPeer(accountConfig, "document")
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	scope := "identity:" + session.UserID
	rpcContext, rpcDone := context.WithTimeout(ctx, 2*time.Second)
	_, err = pb.NewDocumentInternalClient(connection).FenceAccess(rpcContext, &pb.FenceRequest{RevocationId: operation, Generation: 1, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope, Epoch: 1}}, TargetDigest: platform.SourceClosureDigest(scope, 1)})
	rpcDone()
	if status.Code(err) != codes.AlreadyExists || status.Convert(err).Message() != "ACCOUNT_RETIREMENT_CANCELLED" {
		t.Fatalf("LATE_FENCE_NOT_REJECTED: %v", err)
	}
	if err := docStore.DB.QueryRowContext(ctx, "SELECT state FROM access_gate WHERE scope_key=?", scope).Scan(&gate); err != nil || gate != "OPEN" {
		t.Fatal("LATE_FENCE_REBLOCKED_SOURCE")
	}
	var fences int
	if err := docStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM authorization_change WHERE revocation_id=?", operation).Scan(&fences); err != nil || fences != 0 {
		t.Fatal("CANCELLED_OPERATION_CREATED_FENCE")
	}
	plan, err = service.Begin(ctx, session, input, fingerprint(input))
	if err != nil || plan.State != "REJECTED" {
		t.Fatal("REJECTED_ORIGINAL_RESULT_NOT_REPLAYABLE")
	}
	// 第二次注销的Prepare尚未到达时，取消应持久收口；随后迟到的Prepare不可复活。
	lostInput := input
	lostInput.OperationId += "-lost"
	if _, err := service.Begin(ctx, session, lostInput, fingerprint(lostInput)); err != nil {
		t.Fatal(err)
	}
	lostBinding := binding
	lostBinding.Operation = lostInput.OperationId
	if err := service.Bind(ctx, lostBinding); err != nil {
		t.Fatal(err)
	}
	if err := service.Reject(ctx, lostBinding, "PREPARATION_CHANGED"); err != nil {
		t.Fatal(err)
	}
	if err := control.Cancel(ctx, lostBinding); err != nil {
		t.Fatal("ABSENT_PREPARATION_CANNOT_CANCEL", err)
	}
	prepared, err = control.Prepare(ctx, lostBinding)
	if err != nil || prepared.State != "REJECTED" {
		t.Fatal("LATE_PREPARATION_AFTER_CANCEL_REVIVED")
	}
	if err := docStore.DB.QueryRowContext(ctx, "SELECT state FROM access_gate WHERE scope_key=?", scope).Scan(&gate); err != nil || gate != "OPEN" {
		t.Fatal("ABSENT_CANCEL_CHANGED_OTHER_SOURCE")
	}
	// 同一真实身份图接着验证成功的冻结/排空路径：有其他成员的所有者先交接，
	// 下游已提交但回复丢失时保持原号恢复，真实ACK只结束屏障而不冒充完整注销。
	soleOrganization = insert(accountStore.DB, "INSERT INTO organization(name,owner_id,lifecycle) VALUES('唯一成员注销测试',?,'ACTIVE')", user)
	if _, err := accountStore.DB.ExecContext(ctx, "INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", soleOrganization, user); err != nil {
		t.Fatal(err)
	}
	soleSpace = insert(docStore.DB, "INSERT INTO space(kind,organization_id,lifecycle) VALUES('ORGANIZATION',?,'ACTIVE')", soleOrganization)
	soleKB = insert(docStore.DB, "INSERT INTO knowledge_base(space_id,name,lifecycle) VALUES(?,'随账号关闭测试库','ACTIVE')", soleSpace)
	if _, err := docStore.DB.ExecContext(ctx, "INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", soleKB, user); err != nil {
		t.Fatal(err)
	}
	if _, err := docStore.DB.ExecContext(ctx, "UPDATE knowledge_base SET visibility_policy='PUBLIC' WHERE id=?", soleKB); err != nil {
		t.Fatal(err)
	}
	personalSpace = insert(docStore.DB, "INSERT INTO space(kind,account_owner_id,lifecycle) VALUES('PERSONAL',?,'ACTIVE')", user)
	restrictedKB = insert(docStore.DB, "INSERT INTO knowledge_base(space_id,name,lifecycle,visibility_policy) VALUES(?,'受限公开测试库','ACTIVE','COLLABORATORS')", soleSpace)
	for _, spec := range []struct {
		space, kb      uint64
		suffix, policy string
		published      bool
	}{
		{soleSpace, soleKB, "-retained-org", "PUBLIC", true},
		{soleSpace, soleKB, "-withdrawn", "PUBLIC", false},
		{soleSpace, soleKB, "-private", "COLLABORATORS", true},
		{personalSpace, 0, "-retained-personal", "PUBLIC", true},
		{soleSpace, restrictedKB, "-kb-restricted", "PUBLIC", true},
		{space, kb, "-surviving", "PUBLIC", true},
	} {
		retentionDocuments = append(retentionDocuments, createCloseDocumentFixture(t, ctx, docStore.DB, spec.space, spec.kb, user, operation+spec.suffix, spec.policy, spec.published))
	}
	successScopes = accountCloseScopes(platform.AccountClosureBinding{Operation: successOperation, User: user, IdentityEpoch: 1}, []domain.AccountCloseOrganization{{ID: organization, Epoch: 1}, {ID: soleOrganization, Epoch: 1}})
	if _, err := accountStore.DB.ExecContext(ctx, "UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1 WHERE id=?", user, organization); err != nil {
		t.Fatal(err)
	}
	successInput := input
	successInput.OperationId = successOperation
	if _, err := service.Begin(ctx, session, successInput, fingerprint(successInput)); err != nil {
		t.Fatal(err)
	}
	plan, err = service.PrepareClose(ctx, successOperation, nil)
	if err != nil || plan.State != "PREPARING" || len(plan.RequiredSuccessions) != 1 || plan.RequiredSuccessions[0].Kind != "ORGANIZATION_OWNER" {
		t.Fatalf("OWNER_HANDOFF_NOT_REQUIRED: %+v %v", plan, err)
	}
	var userState string
	if err := accountStore.DB.QueryRowContext(ctx, "SELECT state FROM user WHERE id=?", user).Scan(&userState); err != nil || userState != "ACTIVE" {
		t.Fatal("HANDOFF_REQUIRED_FROZE_ACCOUNT")
	}
	if _, err := accountStore.DB.ExecContext(ctx, "UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1 WHERE id=?", other, organization); err != nil {
		t.Fatal(err)
	}
	// 验证目标组织成员证明包含目标identity来源，不仅检查其组织成员代次。
	identity := NewIdentity(accountStore.DB, repository.NewIdentity())
	target := &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}
	identityTX, err := accountStore.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	sources, err := identity.currentSources(ctx, identityTX, other, otherSession[:], pb.IdentityCapability_ORGANIZATION_MEMBER, target)
	_ = identityTX.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	foundIdentity := false
	for _, source := range sources {
		if source.ScopeKey == scope && source.Epoch == 1 {
			foundIdentity = true
		}
	}
	if !foundIdentity {
		t.Fatal("TARGET_IDENTITY_SOURCE_MISSING")
	}
	lostReply := &lostAccountClosePreparation{peer: control}
	successBinding := platform.AccountClosureBinding{Operation: successOperation, User: user, IdentityEpoch: 1, ClosingOrganizations: []uint64{soleOrganization}}
	if _, err := service.PrepareClose(ctx, successOperation, lostReply); err == nil || !lostReply.lost {
		t.Fatal("PREPARATION_REPLY_LOSS_NOT_EXERCISED")
	}
	if err := accountStore.DB.QueryRowContext(ctx, "SELECT state FROM user WHERE id=?", user).Scan(&userState); err != nil || userState != "ACTIVE" {
		t.Fatal("UNKNOWN_PREPARATION_FROZE_ACCOUNT")
	}
	if confirmFromDoc(successBinding) {
		t.Fatal("UNFROZEN_ACCOUNT_CONFIRMED_CLEANUP")
	}
	planCall := func() (platform.AccountClosureRetention, error) {
		callCtx, done := context.WithTimeout(ctx, 2*time.Second)
		defer done()
		return control.Plan(callCtx, successBinding)
	}
	if _, err := planCall(); err == nil {
		t.Fatal("UNFROZEN_ACCOUNT_RETENTION_PLANNED")
	}
	plan, err = service.PrepareClose(ctx, successOperation, control)
	if err != nil || plan.State != "FROZEN" {
		t.Fatalf("ACCOUNT_CLOSE_FREEZE_FAILED: %+v %v", plan, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	var tasks, acks int
	for time.Now().Before(deadline) {
		if err := accountStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='ACCOUNT_CLOSE_FENCE' AND operation_id=? AND state='SUCCEEDED'", successOperation).Scan(&tasks); err != nil {
			t.Fatal(err)
		}
		if tasks == len(successScopes) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if tasks != len(successScopes) {
		var pending string
		_ = accountStore.DB.QueryRowContext(ctx, "SELECT COALESCE(last_error,'') FROM durable_task WHERE kind='ACCOUNT_CLOSE_FENCE' AND operation_id=? AND state<>'SUCCEEDED' LIMIT 1", successOperation).Scan(&pending)
		t.Fatalf("REAL_ACCOUNT_CLOSE_FENCES_PENDING: %d %s", tasks, pending)
	}
	for _, source := range successScopes {
		if err := accountStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant='document' AND scope_key=? AND epoch=1 AND phase='OUTPUT_STOPPED'", source.Revocation, source.Scope).Scan(&acks); err != nil || acks != 1 {
			t.Fatal("REAL_ACCOUNT_CLOSE_ACK_MISSING", err)
		}
	}
	if !confirmFromDoc(successBinding) {
		t.Fatal("REAL_DRAINED_ACCOUNT_NOT_CONFIRMED")
	}
	// 即使原ACK存在，原任务仍未终结时也不能批准Doc清理；仅计数ACK不构成确认。
	if _, err := accountStore.DB.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR) WHERE kind='ACCOUNT_CLOSE_FENCE' AND business_key=?", successOperation); err != nil {
		t.Fatal(err)
	}
	if confirmFromDoc(successBinding) {
		t.Fatal("NONTERMINAL_FENCE_CONFIRMED_CLEANUP")
	}
	if _, err := accountStore.DB.ExecContext(ctx, "UPDATE durable_task SET state='SUCCEEDED' WHERE kind='ACCOUNT_CLOSE_FENCE' AND business_key=?", successOperation); err != nil {
		t.Fatal(err)
	}
	if result, err := planCall(); err != nil || result.State != "PLANNED" {
		t.Fatal("REAL_ACCOUNT_RETENTION_PLAN_FAILED", result, err)
	}
	var saved []byte
	if err := docStore.DB.QueryRowContext(ctx, "SELECT fixed_plan FROM lifecycle_operation WHERE operation_id=? AND state='PLANNED'", successOperation).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	var fixed struct {
		Retention struct {
			Spaces         []uint64 `json:"space_ids"`
			KnowledgeBases []uint64 `json:"knowledge_base_ids"`
			Documents      []struct {
				ID       uint64   `json:"document_id"`
				Release  uint64   `json:"release_id"`
				Manifest uint64   `json:"manifest_id"`
				Versions []uint64 `json:"version_ids"`
			} `json:"documents"`
			Scopes []struct {
				Key       string `json:"scope_key"`
				Epoch     uint64 `json:"epoch"`
				Operation string `json:"revocation_id"`
			} `json:"scopes"`
		} `json:"retention"`
	}
	if err := json.Unmarshal(saved, &fixed); err != nil {
		t.Fatal(err)
	}
	retentionScopes = fixed.Retention.Scopes
	if len(fixed.Retention.Documents) != 5 || len(retentionScopes) != 12 || !slices.Contains(fixed.Retention.Spaces, personalSpace) || !slices.Contains(fixed.Retention.Spaces, soleSpace) || slices.Contains(fixed.Retention.Spaces, space) {
		t.Fatal("RETENTION_CLOSING_SPACE_RANGE_INVALID")
	}
	for _, item := range fixed.Retention.Documents {
		for i, fixture := range retentionDocuments {
			if fixture.id != item.ID {
				continue
			}
			if i == 0 || i == 3 {
				if item.Release != fixture.release || item.Manifest != fixture.manifest || !slices.Equal(item.Versions, []uint64{fixture.version}) || slices.Contains(item.Versions, fixture.privateVersion) {
					t.Fatal("RETENTION_SELECTED_PRIVATE_MAIN_OR_WRONG_RELEASE")
				}
			} else if item.Release != 0 || item.Manifest != 0 || len(item.Versions) != 0 {
				t.Fatal("WITHDRAWN_OR_RESTRICTED_HISTORY_RETAINED")
			}
		}
	}
	if _, err := planCall(); err != nil {
		t.Fatal("RETENTION_ORIGINAL_REPLAY_FAILED", err)
	}
	var replay []byte
	if err := docStore.DB.QueryRowContext(ctx, "SELECT fixed_plan FROM lifecycle_operation WHERE operation_id=?", successOperation).Scan(&replay); err != nil || !bytes.Equal(saved, replay) {
		t.Fatal("RETENTION_ORIGINAL_PLAN_REPLACED", err)
	}
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		tasks = 0
		for _, scope := range retentionScopes {
			var state string
			if err := docStore.DB.QueryRowContext(ctx, "SELECT state FROM durable_task WHERE kind='AUTHORIZATION_CHANGE' AND business_key=?", scope.Operation).Scan(&state); err != nil {
				t.Fatal(err)
			}
			if state == "SUCCEEDED" {
				tasks++
			}
		}
		if tasks == len(retentionScopes) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if tasks != len(retentionScopes) {
		t.Fatal("REAL_RETENTION_DRAINS_PENDING", tasks)
	}
	for _, scope := range retentionScopes {
		if err := docStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED' AND participant IN ('edge','collaboration')", scope.Operation, scope.Key, scope.Epoch).Scan(&acks); err != nil || acks != 2 {
			t.Fatal("REAL_RETENTION_OUTPUT_ACK_MISSING", err)
		}
	}
	var survivingState string
	var survivingManifest uint64
	surviving := retentionDocuments[5]
	if err := docStore.DB.QueryRowContext(ctx, "SELECT lifecycle,current_manifest_id FROM document WHERE id=?", surviving.id).Scan(&survivingState, &survivingManifest); err != nil || survivingState != "ACTIVE" || survivingManifest != surviving.privateManifest {
		t.Fatal("SURVIVING_ORGANIZATION_CONTENT_CHANGED", err)
	}
	relationsCall := func() (platform.AccountClosureRetention, error) {
		callCtx, done := context.WithTimeout(ctx, 2*time.Second)
		defer done()
		return control.Relations(callCtx, successBinding)
	}
	// ACK存在不代表任务已终结；关系清理不得越过原计划的真实排空终态。
	if _, err := docStore.DB.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR) WHERE kind='AUTHORIZATION_CHANGE' AND business_key=?", retentionScopes[0].Operation); err != nil {
		t.Fatal(err)
	}
	if _, err := relationsCall(); err == nil {
		t.Fatal("NONTERMINAL_RETENTION_ALLOWED_RELATION_CLEANUP")
	}
	var responsible sql.NullString
	if err := docStore.DB.QueryRowContext(ctx, "SELECT CAST(responsible_user_id AS CHAR) FROM document WHERE id=?", surviving.id).Scan(&responsible); err != nil || responsible.String != session.UserID {
		t.Fatal("UNCONFIRMED_CLEANUP_CLEARED_RESPONSIBLE", err)
	}
	if _, err := docStore.DB.ExecContext(ctx, "UPDATE durable_task SET state='SUCCEEDED' WHERE kind='AUTHORIZATION_CHANGE' AND business_key=?", retentionScopes[0].Operation); err != nil {
		t.Fatal(err)
	}
	if result, err := relationsCall(); err != nil || result.State != "CLEANING" {
		t.Fatal("REAL_ACCOUNT_RELATIONS_FAILED", result, err)
	}
	if _, err := relationsCall(); err != nil {
		t.Fatal("ACCOUNT_RELATIONS_ORIGINAL_REPLAY_FAILED", err)
	}
	if err := docStore.DB.QueryRowContext(ctx, "SELECT lifecycle,current_manifest_id,CAST(responsible_user_id AS CHAR) FROM document WHERE id=?", surviving.id).Scan(&survivingState, &survivingManifest, &responsible); err != nil || survivingState != "ACTIVE" || survivingManifest != surviving.privateManifest || responsible.Valid {
		t.Fatal("SURVIVING_ORGANIZATION_RESPONSIBLE_CLEANUP_INVALID", err)
	}
	var preservedVersions, preservedReleases int
	if err := docStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM content_version WHERE id IN (?,?) AND document_id=?", surviving.version, surviving.privateVersion, surviving.id).Scan(&preservedVersions); err != nil || preservedVersions != 2 {
		t.Fatal("SURVIVING_VERSIONS_DELETED", err)
	}
	if err := docStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM `release` WHERE id=? AND document_id=?", surviving.release, surviving.id).Scan(&preservedReleases); err != nil || preservedReleases != 1 {
		t.Fatal("SURVIVING_RELEASE_DELETED", err)
	}
	var retiredState, memberState string
	if err := docStore.DB.QueryRowContext(ctx, "SELECT r.state,m.state FROM admin_retirement r JOIN knowledge_base_member m ON m.knowledge_base_id=r.knowledge_base_id AND m.user_id=r.user_id WHERE r.knowledge_base_id=? AND r.user_id=? AND r.operation_id=?", kb, user, successOperation).Scan(&retiredState, &memberState); err != nil || retiredState != "APPLIED" || memberState != "CLOSED" {
		t.Fatal("ADMIN_RETIREMENT_NOT_APPLIED", err)
	}
	eventDigest := sha256.Sum256([]byte(successOperation + ":" + strconv.FormatUint(surviving.id, 10)))
	departureEvent = "departure-" + hex.EncodeToString(eventDigest[:])
	deadline = time.Now().Add(5 * time.Second)
	var noticeState string
	for time.Now().Before(deadline) {
		if err := docStore.DB.QueryRowContext(ctx, "SELECT state FROM durable_task WHERE kind='RESPONSIBLE_DEPARTURE_NOTIFICATION' AND business_key=?", departureEvent).Scan(&noticeState); err != nil {
			t.Fatal(err)
		}
		if noticeState == "SUCCEEDED" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if noticeState != "SUCCEEDED" {
		t.Fatal("ACCOUNT_DEPARTURE_NOTICE_NOT_DELIVERED")
	}
	var notices int
	if err := docStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM notification WHERE recipient_id=? AND event_id=? AND kind='RESPONSIBLE_DEPARTURE'", other, departureEvent).Scan(&notices); err != nil || notices != 1 {
		t.Fatal("CURRENT_KB_ADMIN_NOTICE_MISSING_OR_DUPLICATED", err)
	}
	verifyActualNodeAccountCleanup(t, ctx, successOperation, user, retentionDocuments[0], surviving, retentionDocuments[2])
	var identityEpoch, memberEpoch uint64
	if err := accountStore.DB.QueryRowContext(ctx, "SELECT state,identity_epoch FROM user WHERE id=?", user).Scan(&userState, &identityEpoch); err != nil || userState != "REVOKING" || identityEpoch != 2 {
		t.Fatal("ACCOUNT_IDENTITY_NOT_FROZEN", err)
	}
	if err := accountStore.DB.QueryRowContext(ctx, "SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=?", organization, user).Scan(&userState, &memberEpoch); err != nil || userState != "REVOKING" || memberEpoch != 2 {
		t.Fatal("ACCOUNT_MEMBERSHIP_NOT_FROZEN", err)
	}
	plan, err = service.Begin(ctx, session, successInput, fingerprint(successInput))
	if err != nil || plan.State != "FROZEN" {
		t.Fatal("FROZEN_ORIGINAL_RESULT_NOT_REPLAYABLE", err)
	}
	var soleLifecycle string
	if err := accountStore.DB.QueryRowContext(ctx, "SELECT lifecycle FROM organization WHERE id=?", soleOrganization).Scan(&soleLifecycle); err != nil || soleLifecycle != "CLOSING" {
		t.Fatal("SOLE_OWNER_ORGANIZATION_NOT_FROZEN", err)
	}
	var soleReservations int
	if err := docStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_retirement WHERE knowledge_base_id=? AND state='RESERVED'", soleKB).Scan(&soleReservations); err != nil || soleReservations != 0 {
		t.Fatal("CLOSING_ORGANIZATION_REQUIRED_ADMIN_SUCCESSION", err)
	}
	if err := service.Reject(ctx, platform.AccountClosureBinding{Operation: successOperation, User: user, IdentityEpoch: 1, ClosingOrganizations: []uint64{soleOrganization}}, "PREPARATION_CHANGED"); err == nil {
		t.Fatal("FROZEN_ACCOUNT_CLOSE_CANCELLED")
	}
	// 两个先前REJECTED操作的持久取消无需再次用户请求，应由运行服务恢复完成。
	var cancellations int
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := accountStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='ACCOUNT_CLOSE_CANCEL' AND operation_id IN (?,?) AND state='SUCCEEDED'", operation, lostInput.OperationId).Scan(&cancellations); err != nil {
			t.Fatal(err)
		}
		if cancellations == 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if cancellations != 2 {
		t.Fatal("DURABLE_CANCELLATION_NOT_RECOVERED")
	}
}
