package account

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	pb "collaboration/contracts/internalv1"
	"google.golang.org/protobuf/proto"
)

// 直接执行Account原证明重验与姓名读取；不连接用户库、SMTP或浏览器。
// 目标变代和注销模拟真实权威事实，姓名查询必须留在同一个事务中。
type collaboratorIdentityModel struct {
	proof         *pb.IdentityAdmission
	state         string
	epoch         uint64
	nameReads     int
	inTransaction bool
}

type collaboratorIdentityConnector struct{ model *collaboratorIdentityModel }

func (c collaboratorIdentityConnector) Connect(context.Context) (driver.Conn, error) {
	return &collaboratorIdentityConn{c.model}, nil
}
func (collaboratorIdentityConnector) Driver() driver.Driver { return collaboratorIdentityDriver{} }

type collaboratorIdentityDriver struct{}

func (collaboratorIdentityDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("COLLABORATOR_MODEL_OPEN_UNSUPPORTED")
}

type collaboratorIdentityConn struct{ model *collaboratorIdentityModel }

func (*collaboratorIdentityConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("COLLABORATOR_MODEL_PREPARE_UNSUPPORTED")
}
func (*collaboratorIdentityConn) Close() error { return nil }
func (c *collaboratorIdentityConn) Begin() (driver.Tx, error) {
	c.model.inTransaction = true
	return collaboratorIdentityTx{c.model}, nil
}

type collaboratorIdentityTx struct{ model *collaboratorIdentityModel }

func (tx collaboratorIdentityTx) Commit() error {
	tx.model.inTransaction = false
	return nil
}
func (tx collaboratorIdentityTx) Rollback() error {
	tx.model.inTransaction = false
	return nil
}
func (*collaboratorIdentityConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}
func (c *collaboratorIdentityConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	m := c.model
	var values []driver.Value
	switch {
	case strings.HasPrefix(query, "SELECT target_json,state,expires_at FROM identity_admission"):
		raw, _ := proto.Marshal(m.proof)
		binding, _ := json.Marshal(admissionBinding{Proof: raw})
		values = []driver.Value{binding, "OPEN", time.Now().Add(time.Minute)}
	case strings.HasPrefix(query, "SELECT state,credential_epoch,identity_epoch FROM user"):
		values = []driver.Value{"ACTIVE", int64(1), int64(1)}
	case strings.HasPrefix(query, "SELECT user_id,state,credential_epoch,epoch,expires_at FROM session"):
		values = []driver.Value{int64(1), "ACTIVE", int64(1), int64(1), time.Now().Add(time.Minute)}
	case strings.HasPrefix(query, "SELECT state,identity_epoch FROM user"):
		values = []driver.Value{m.state, int64(m.epoch)}
	case strings.HasPrefix(query, "SELECT max_epoch,state FROM access_gate"):
		for _, source := range m.proof.Sources {
			if source.ScopeKey == args[0].Value {
				values = []driver.Value{int64(source.Epoch), "OPEN"}
			}
		}
	case strings.HasPrefix(query, "SELECT state,expires_at FROM identity_admission"):
		values = []driver.Value{"OPEN", time.Now().Add(time.Minute)}
	case strings.HasPrefix(query, "SELECT display_name FROM user"):
		if !m.inTransaction || args[0].Value != int64(2) {
			return nil, errors.New("COLLABORATOR_NAME_TRANSACTION_OR_TARGET_CHANGED")
		}
		m.nameReads++
		values = []driver.Value{"李小明"}
	default:
		return nil, errors.New("UNEXPECTED_COLLABORATOR_IDENTITY_QUERY")
	}
	return &collaboratorIdentityRows{values: values}, nil
}

type collaboratorIdentityRows struct {
	values []driver.Value
	read   bool
}

func (r *collaboratorIdentityRows) Columns() []string { return make([]string, len(r.values)) }
func (*collaboratorIdentityRows) Close() error        { return nil }
func (r *collaboratorIdentityRows) Next(values []driver.Value) error {
	if r.read {
		return io.EOF
	}
	copy(values, r.values)
	r.read = true
	return nil
}

func TestDocumentCollaboratorNameUsesCurrentOriginalIdentity(t *testing.T) {
	proof := &pb.IdentityAdmission{AdmissionId: "collaborator-admission-original", OperationId: "collaborator-operation-original", ActorId: 1,
		Purpose: "DOCUMENT_COLLABORATORS", Target: &pb.Target{DocumentId: 10, Purpose: "DOCUMENT_COLLABORATORS"},
		IdentityTarget: &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: 2}, Capability: pb.IdentityCapability_ACTIVE_ACCOUNT,
		SessionRef: make([]byte, 32), ExpiresUnixMs: time.Now().Add(time.Minute).UnixMilli()}
	proof.Sources = []*pb.SourceEpoch{{ScopeKey: "identity:1", Epoch: 1}, {ScopeKey: "credential:1", Epoch: 1},
		{ScopeKey: "session:" + hex.EncodeToString(proof.SessionRef), Epoch: 1}, {ScopeKey: "identity:2", Epoch: 5}}
	m := &collaboratorIdentityModel{proof: proof, state: "ACTIVE", epoch: 5}
	db := sql.OpenDB(collaboratorIdentityConnector{m})
	defer db.Close()
	service := NewIdentity(db, repository.NewIdentity())
	request := &pb.IdentityRevalidation{AdmissionId: proof.AdmissionId, OperationId: proof.OperationId, Purpose: proof.Purpose, Target: proof.Target, IdentityTarget: proof.IdentityTarget}
	name, err := service.DocumentCollaborator(context.Background(), request)
	if err != nil || name.UserId != "2" || name.DisplayName != "李小明" || m.nameReads != 1 {
		t.Fatalf("authoritative collaborator name: %+v reads=%d error=%v", name, m.nameReads, err)
	}
	// 新提及用途复用同一原会话、文档、目标和gate重验，不能只凭改写purpose借用旧证明。
	mention := proto.Clone(request).(*pb.IdentityRevalidation)
	mention.Purpose, mention.Target.Purpose = "DOCUMENT_MENTION_CANDIDATES", "DOCUMENT_MENTION_CANDIDATES"
	if _, err = service.DocumentCollaborator(context.Background(), mention); !errors.Is(err, domain.ErrIdentityDenied) {
		t.Fatalf("mention purpose borrowed a management admission: %v", err)
	}
	m.proof.Purpose, m.proof.Target.Purpose = mention.Purpose, mention.Purpose
	request = mention
	name, err = service.DocumentCollaborator(context.Background(), request)
	if err != nil || name.UserId != "2" || name.DisplayName != "李小明" || m.nameReads != 2 {
		t.Fatalf("authoritative mention name: %+v reads=%d error=%v", name, m.nameReads, err)
	}
	application := proto.Clone(request).(*pb.IdentityRevalidation)
	application.Purpose, application.Target.Purpose = "DOCUMENT_APPLICATION_NAMES", "DOCUMENT_APPLICATION_NAMES"
	if _, err = service.DocumentCollaborator(context.Background(), application); !errors.Is(err, domain.ErrIdentityDenied) {
		t.Fatalf("application name borrowed a mention admission: %v", err)
	}
	m.proof.Purpose, m.proof.Target.Purpose = application.Purpose, application.Purpose
	request = application
	name, err = service.DocumentCollaborator(context.Background(), request)
	if err != nil || name.DisplayName != "李小明" || m.nameReads != 3 {
		t.Fatalf("authoritative application name: %+v reads=%d error=%v", name, m.nameReads, err)
	}
	// 改写目标不能重用原准入，注销与身份变代也必须在读取姓名前拒绝。
	wrong := proto.Clone(request).(*pb.IdentityRevalidation)
	wrong.IdentityTarget.UserId = 9
	if _, err = service.DocumentCollaborator(context.Background(), wrong); !errors.Is(err, domain.ErrIdentityDenied) {
		t.Fatalf("changed target was accepted: %v", err)
	}
	for _, state := range []string{"CLOSED", "ACTIVE"} {
		m.state, m.epoch = state, 6
		if _, err = service.DocumentCollaborator(context.Background(), request); !errors.Is(err, domain.ErrIdentityDenied) || m.nameReads != 3 {
			t.Fatalf("stale target %s leaked a name: reads=%d error=%v", state, m.nameReads, err)
		}
	}
	wrong.Purpose = "READ_DOCUMENT"
	if _, err = service.DocumentCollaborator(context.Background(), wrong); !errors.Is(err, domain.ErrIdentityBinding) {
		t.Fatalf("unrelated purpose was accepted: %v", err)
	}
}
