package document

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 直接执行目录用例及事务闭包，不连接账号服务、不创建用户或发送邮件。
// 数据库适配器只提供已存在的文档事实，便于安排ACL在读取与事务之间变化的真实反例。
type collaboratorModel struct {
	acl            string
	candidateReads int
	nameReads      int
	organization   bool
}

type collaboratorConnector struct{ model *collaboratorModel }

func (c collaboratorConnector) Connect(context.Context) (driver.Conn, error) {
	return &collaboratorConn{model: c.model}, nil
}
func (collaboratorConnector) Driver() driver.Driver { return collaboratorDriver{} }

type collaboratorDriver struct{}

func (collaboratorDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("UNSUPPORTED_COLLABORATOR_OPEN")
}

type collaboratorConn struct{ model *collaboratorModel }

func (*collaboratorConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("UNSUPPORTED_COLLABORATOR_PREPARE")
}
func (*collaboratorConn) Close() error { return nil }
func (*collaboratorConn) Begin() (driver.Tx, error) {
	return collaboratorTx{}, nil
}

type collaboratorTx struct{}

func (collaboratorTx) Commit() error   { return nil }
func (collaboratorTx) Rollback() error { return nil }

func (c *collaboratorConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	var values [][]driver.Value
	columns := 1
	switch {
	case strings.Contains(query, "SELECT candidate.user_id"):
		c.model.candidateReads++
		last, _ := strconv.Atoi(args[3].Value.(string))
		for _, id := range []int{1, 2, 3} {
			if id > last {
				values = append(values, []driver.Value{int64(id)})
			}
		}
	case strings.Contains(query, "FROM document WHERE id=?"):
		columns = 17
		values = [][]driver.Value{{"10", "目录测试", "20", nil, "1", "ACTIVE", false, "30", "31", "32", nil, "UNPUBLISHED", "1", c.model.acl, "1", "PRIVATE", "1"}}
	case strings.Contains(query, "FROM space WHERE id=?"):
		columns = 5
		values = [][]driver.Value{{"20", "PERSONAL", "1", nil, "ACTIVE"}}
		if c.model.organization {
			values = [][]driver.Value{{"20", "ORGANIZATION", nil, "40", "ACTIVE"}}
		}
	case strings.Contains(query, "FROM content_grant WHERE"):
		columns = 4
		// 用户2只有评论权，可读正文但不得查看管理候选。
		if args[1].Value == int64(2) {
			values = [][]driver.Value{{int64(2), "DOCUMENT", int64(10), "COMMENT"}}
		} else if args[1].Value == int64(4) {
			// 用户4仅能编辑一个单元；EDIT带读取资格，但不能枚举提及候选。
			values = [][]driver.Value{{int64(4), "UNIT", int64(50), "EDIT"}}
		}
	case strings.Contains(query, "FROM collaboration_unit u"):
		columns = 2
		values = [][]driver.Value{{int64(50), int64(60)}}
	default:
		return nil, errors.New("UNEXPECTED_COLLABORATOR_QUERY")
	}
	return &collaboratorRows{columns: make([]string, columns), values: values}, nil
}

type collaboratorRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *collaboratorRows) Columns() []string { return r.columns }
func (*collaboratorRows) Close() error        { return nil }
func (r *collaboratorRows) Next(destination []driver.Value) error {
	if r.index == len(r.values) {
		return io.EOF
	}
	copy(destination, r.values[r.index])
	r.index++
	return nil
}

type collaboratorIdentity struct {
	pb.IdentityInternalClient
	proofs map[string]*pb.IdentityAdmission
	actor  uint64
}

func (p *collaboratorIdentity) AcquireIdentityAdmission(_ context.Context, input *pb.IdentityRequest, _ ...grpc.CallOption) (*pb.IdentityAdmission, error) {
	// 候选3模拟注销账号；残余grant不能把它重新变成可选协作者。
	if input.IdentityTarget.UserId == 3 {
		return nil, status.Error(codes.PermissionDenied, "ACCOUNT_CLOSED")
	}
	if input.RequiredIdentityCapability == pb.IdentityCapability_ORGANIZATION_ADMIN && p.actor != 1 {
		return nil, status.Error(codes.PermissionDenied, "ORGANIZATION_ADMIN_REQUIRED")
	}
	proof := &pb.IdentityAdmission{AdmissionId: input.OperationId, OperationId: input.OperationId,
		ActorId: p.actor, Purpose: input.Purpose, Target: input.Target, IdentityTarget: input.IdentityTarget,
		Capability: input.RequiredIdentityCapability, SessionRef: make([]byte, 32),
		ExpiresUnixMs: time.Now().Add(time.Minute).UnixMilli(),
		Sources:       []*pb.SourceEpoch{{ScopeKey: "identity:" + domain.TextID(input.IdentityTarget.UserId), Epoch: 1}}}
	if input.IdentityTarget.Kind == pb.IdentityTarget_ORGANIZATION {
		if input.IdentityTarget.OrganizationId != 40 || input.Target.DocumentId != 10 {
			return nil, errors.New("COLLABORATOR_TARGET_SCOPE_CHANGED")
		}
		proof.Sources = append(proof.Sources, &pb.SourceEpoch{ScopeKey: "organization-member:40:" + domain.TextID(input.IdentityTarget.UserId), Epoch: 1})
	}
	p.proofs[proof.AdmissionId] = proof
	return proof, nil
}
func (p *collaboratorIdentity) RevalidateIdentityAdmission(_ context.Context, input *pb.IdentityRevalidation, _ ...grpc.CallOption) (*pb.IdentityAdmission, error) {
	return p.proofs[input.AdmissionId], nil
}

type collaboratorSigner struct{}

func (collaboratorSigner) Sign(value []byte) []byte {
	sum := sha256.Sum256(value)
	return sum[:]
}

func collaboratorFixture(t *testing.T, actorID uint64) (*Directory, *sql.DB, *collaboratorModel, Actor) {
	t.Helper()
	model := &collaboratorModel{acl: "1"}
	db := sql.OpenDB(collaboratorConnector{model})
	t.Cleanup(func() { db.Close() })
	actor := Actor{ID: actorID, Proof: &pb.IdentityAdmission{AdmissionId: "actor", Sources: []*pb.SourceEpoch{{ScopeKey: "identity:" + domain.TextID(actorID), Epoch: 1}}}}
	peer := &collaboratorIdentity{proofs: map[string]*pb.IdentityAdmission{"actor": actor.Proof}, actor: actorID}
	identity := NewIdentity(peer, nil, nil)
	access := NewAccess(repository.NewAccess(db), identity)
	return NewDirectory(db, identity, access, nil, NewPagination(collaboratorSigner{}), collaboratorNames{model: model}), db, model, actor
}

func collaboratorPage(t *testing.T, db *sql.DB, run ReadBody) (dto.DocumentCollaboratorPage, error) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	value, err := run(context.Background(), tx, "")
	if err != nil {
		return dto.DocumentCollaboratorPage{}, err
	}
	return value.(dto.DocumentCollaboratorPage), nil
}

func TestDocumentCollaboratorsRequireManagementBeforeCandidateLookup(t *testing.T) {
	// 仅有文档评论权的真实读者不能枚举管理目标；拒绝发生在账号/目录查询之前。
	service, _, model, actor := collaboratorFixture(t, 2)
	_, _, err := service.DocumentCollaborators(context.Background(), actor, 10, "", "", "")
	var violation domain.RuleViolation
	if !errors.As(err, &violation) || violation.Code != "DOCUMENT_MANAGEMENT_REQUIRED" || model.candidateReads != 0 || model.nameReads != 0 {
		t.Fatalf("non-manager enumerated collaborators: reads=%d error=%v", model.candidateReads, err)
	}
}

func TestDocumentCollaboratorsBindTargetsCursorAndTransactionalACL(t *testing.T) {
	// 连续小页不重复；旧ACL的cursor和已准备输出都必须在撤权后失效。
	service, db, model, actor := collaboratorFixture(t, 1)
	sources, run, err := service.DocumentCollaborators(context.Background(), actor, 10, "", "1", "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := collaboratorPage(t, db, run)
	if err != nil || len(first.Items) != 1 || first.Items[0].UserId != "1" || first.Items[0].DisplayName != "文档负责人" || !first.HasMore {
		t.Fatalf("first page: %+v error=%v", first, err)
	}
	if _, _, err = service.DocumentMentionCandidates(context.Background(), actor, 10, "", "1", *first.NextCursor); err == nil {
		t.Fatal("management cursor was accepted by the mention directory")
	}
	_, run, err = service.DocumentCollaborators(context.Background(), actor, 10, "", "1", *first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	second, err := collaboratorPage(t, db, run)
	if err != nil || len(second.Items) != 1 || second.Items[0].UserId != "2" || second.Items[0].DisplayName != "李小明" {
		t.Fatalf("second page: %+v error=%v", second, err)
	}
	sources, pending, err := service.DocumentCollaborators(context.Background(), actor, 10, "", "100", "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := collaboratorPage(t, db, pending)
	if err != nil || len(page.Items) != 2 || page.HasMore {
		t.Fatalf("closed identity was included: %+v error=%v", page, err)
	}
	bound := map[string]bool{}
	for _, source := range sources {
		bound[source.ScopeKey] = true
	}
	if !bound["doc-acl:10"] || !bound["identity:1"] || !bound["identity:2"] || bound["identity:3"] {
		t.Fatalf("output lost target identity/ACL sources: %v", bound)
	}
	model.acl = "2"
	if _, _, err = service.DocumentCollaborators(context.Background(), actor, 10, "", "1", *first.NextCursor); err == nil {
		t.Fatal("cursor accepted after ACL changed")
	}
	if _, err = collaboratorPage(t, db, pending); err == nil {
		t.Fatal("prepared output accepted after ACL changed")
	}
}

type collaboratorNames struct {
	model       *collaboratorModel
	wrongTarget bool
}

func (n collaboratorNames) Collaborator(_ context.Context, proof *pb.IdentityRevalidation) (dto.DocumentCollaborator, error) {
	if n.model != nil {
		n.model.nameReads++
	}
	if proof.AdmissionId == "" || proof.OperationId == "" || (proof.Purpose != "DOCUMENT_COLLABORATORS" && proof.Purpose != "DOCUMENT_MENTION_CANDIDATES") || proof.Target.DocumentId != 10 || proof.IdentityTarget.Kind != pb.IdentityTarget_USER {
		return dto.DocumentCollaborator{}, errors.New("COLLABORATOR_NAME_SCOPE_CHANGED")
	}
	user := domain.TextID(proof.IdentityTarget.UserId)
	if n.wrongTarget {
		user = "99"
	}
	return dto.DocumentCollaborator{UserId: user, DisplayName: map[uint64]string{1: "文档负责人", 2: "李小明"}[proof.IdentityTarget.UserId]}, nil
}

func TestDocumentMentionCandidatesRequireCommentAndBindCurrentTargets(t *testing.T) {
	// COMMENT用户可按真实姓名提及实际协作者；残余grant中的注销账号仍须过滤。
	service, db, model, actor := collaboratorFixture(t, 2)
	sources, pending, err := service.DocumentMentionCandidates(context.Background(), actor, 10, "", "100", "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := collaboratorPage(t, db, pending)
	if err != nil || len(page.Items) != 2 || page.Items[1].UserId != "2" || page.Items[1].DisplayName != "李小明" || page.HasMore {
		t.Fatalf("mention candidates included unrelated/closed identities: %+v error=%v", page, err)
	}
	bound := map[string]bool{}
	for _, source := range sources {
		bound[source.ScopeKey] = true
	}
	if !bound["doc-acl:10"] || !bound["identity:1"] || !bound["identity:2"] || bound["identity:3"] {
		t.Fatalf("mention output lost actual target identity/ACL sources: %v", bound)
	}
	_, firstRead, err := service.DocumentMentionCandidates(context.Background(), actor, 10, "", "1", "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := collaboratorPage(t, db, firstRead)
	if err != nil || len(first.Items) != 1 || first.Items[0].UserId != "1" || !first.HasMore {
		t.Fatalf("mention first keyset page: %+v error=%v", first, err)
	}
	_, nextRead, err := service.DocumentMentionCandidates(context.Background(), actor, 10, "", "1", *first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	next, err := collaboratorPage(t, db, nextRead)
	if err != nil || len(next.Items) != 1 || next.Items[0].UserId != "2" {
		t.Fatalf("mention next keyset page: %+v error=%v", next, err)
	}
	model.acl = "2"
	if _, _, err = service.DocumentMentionCandidates(context.Background(), actor, 10, "", "1", *first.NextCursor); err == nil {
		t.Fatal("mention cursor survived an ACL change")
	}
	if _, err = collaboratorPage(t, db, pending); err == nil {
		t.Fatal("prepared mention output survived an ACL change")
	}
	// 仅有单元EDIT/读取资格不能枚举姓名，拒绝应发生在候选和Account查询之前。
	service, _, model, actor = collaboratorFixture(t, 4)
	_, _, err = service.DocumentMentionCandidates(context.Background(), actor, 10, "", "100", "")
	var violation domain.RuleViolation
	if !errors.As(err, &violation) || violation.Code != "COMMENT_REQUIRED" || model.candidateReads != 0 || model.nameReads != 0 {
		t.Fatalf("edit-only user enumerated mention candidates: reads=%d names=%d error=%v", model.candidateReads, model.nameReads, err)
	}
}

func TestDocumentMentionCandidatesUseVisibleOrganizationMembers(t *testing.T) {
	// 组织普通COMMENT成员可提及当前可见成员；无需先变成管理员或为目标创建文档grant。
	service, db, model, actor := collaboratorFixture(t, 2)
	model.organization = true
	sources, run, err := service.DocumentMentionCandidates(context.Background(), actor, 10, "", "100", "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := collaboratorPage(t, db, run)
	if err != nil || len(page.Items) != 3 || page.Items[2].UserId != "4" || page.Items[2].DisplayName != "尚无文档授权的成员" || model.candidateReads != 0 {
		t.Fatalf("organization mention directory: %+v error=%v", page, err)
	}
	for _, source := range sources {
		if source.ScopeKey == "organization-member:40:4" {
			return
		}
	}
	t.Fatal("mention output omitted actual target membership source")
}

func TestDocumentCollaboratorsRejectNameForAnotherTarget(t *testing.T) {
	// 账号适配误返其他目标时必须关闭整页，不能把真实姓名与错误账号拼接输出。
	service, _, model, actor := collaboratorFixture(t, 1)
	service.directory = collaboratorNames{model: model, wrongTarget: true}
	if _, run, err := service.DocumentCollaborators(context.Background(), actor, 10, "", "100", ""); err == nil || run != nil {
		t.Fatal("mismatched collaborator name was accepted")
	}
}

func (collaboratorNames) Read(_ context.Context, _, path string, value any) error {
	if path != "/v1/organizations/40/members?limit=100&cursor=" {
		return errors.New("COLLABORATOR_DIRECTORY_SCOPE_CHANGED")
	}
	*value.(*dto.MemberPage) = dto.MemberPage{Items: []dto.Member{
		{UserId: "1", DisplayName: "负责人", State: "ACTIVE"},
		{UserId: "2", DisplayName: "已邀请成员", State: "ACTIVE"},
		{UserId: "3", DisplayName: "历史成员", State: "CLOSED"},
		{UserId: "4", DisplayName: "尚无文档授权的成员", State: "ACTIVE"},
	}}
	return nil
}

func TestDocumentCollaboratorsUseCurrentOrganizationNamesAndMembershipSources(t *testing.T) {
	// 首次授权必须能选到尚无文档grant的当前成员；历史成员仍不得返回。
	service, db, model, actor := collaboratorFixture(t, 1)
	model.organization = true
	service.directory = collaboratorNames{}
	sources, run, err := service.DocumentCollaborators(context.Background(), actor, 10, "", "100", "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := collaboratorPage(t, db, run)
	if err != nil || len(page.Items) != 3 || page.Items[1].DisplayName != "已邀请成员" || page.Items[2].UserId != "4" || page.Items[2].DisplayName != "尚无文档授权的成员" || model.candidateReads != 0 {
		t.Fatalf("organization directory page: %+v error=%v", page, err)
	}
	_, firstRead, err := service.DocumentCollaborators(context.Background(), actor, 10, "", "2", "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := collaboratorPage(t, db, firstRead)
	if err != nil || len(first.Items) != 2 || first.Items[0].UserId != "1" || first.Items[1].UserId != "2" || !first.HasMore {
		t.Fatalf("organization first keyset page: %+v error=%v", first, err)
	}
	_, nextRead, err := service.DocumentCollaborators(context.Background(), actor, 10, "", "2", *first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	next, err := collaboratorPage(t, db, nextRead)
	if err != nil || len(next.Items) != 1 || next.Items[0].UserId != "4" || next.HasMore {
		t.Fatalf("organization next keyset page: %+v error=%v", next, err)
	}
	for _, source := range sources {
		if source.ScopeKey == "organization-member:40:4" {
			return
		}
	}
	t.Fatal("output omitted target organization membership source")
}
