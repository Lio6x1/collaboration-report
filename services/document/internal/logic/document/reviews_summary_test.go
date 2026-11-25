package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

type reviewSummaryModel struct {
	access  *collaboratorModel
	body    []byte
	reads   int
	missing bool
}

type reviewSummaryConnector struct{ model *reviewSummaryModel }

func (c reviewSummaryConnector) Connect(context.Context) (driver.Conn, error) {
	return &reviewSummaryConn{collaboratorConn: &collaboratorConn{model: c.model.access}, model: c.model}, nil
}
func (reviewSummaryConnector) Driver() driver.Driver { return collaboratorDriver{} }

type reviewSummaryConn struct {
	*collaboratorConn
	model *reviewSummaryModel
	inTx  bool
}

func (c *reviewSummaryConn) Begin() (driver.Tx, error) {
	c.inTx = true
	return reviewSummaryTx{c}, nil
}

type reviewSummaryTx struct{ conn *reviewSummaryConn }

func (tx reviewSummaryTx) Commit() error   { tx.conn.inTx = false; return nil }
func (tx reviewSummaryTx) Rollback() error { tx.conn.inTx = false; return nil }

func (c *reviewSummaryConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	var values [][]driver.Value
	columns := 1
	switch {
	case strings.HasPrefix(query, "SELECT id FROM review_request WHERE"):
		values = [][]driver.Value{{int64(99)}}
	case strings.Contains(query, "COALESCE(current_round_id,0)"):
		columns = 8
		values = [][]driver.Value{{int64(99), int64(10), int64(50), int64(70), int64(88), int64(1), "WITHDRAWN", false}}
	case strings.HasPrefix(query, "SELECT CAST(r.id AS CHAR)"):
		columns = 9
		values = [][]driver.Value{{"99", "10", "50", "70", "WITHDRAWN", "88", "1", nil, int64(0)}}
	case strings.HasPrefix(query, "SELECT e.title,v.body_json"):
		c.model.reads++
		if !c.inTx || len(args) != 3 || args[0].Value != int64(99) || args[1].Value != int64(88) || args[2].Value != int64(10) {
			return nil, errors.New("REVIEW_SUMMARY_BINDING_OR_TRANSACTION_LOST")
		}
		columns = 5
		if !c.model.missing {
			values = [][]driver.Value{{"送审时的章节", c.model.body, "2", "7", time.Date(2026, 10, 9, 16, 20, 30, 123000000, time.FixedZone("CST", 8*60*60))}}
		}
	default:
		// 复用已有文档权限夹具；任何新增MAIN/草稿/完整轮次/样式查询都会被拒绝。
		return c.collaboratorConn.QueryContext(ctx, query, args)
	}
	return &collaboratorRows{columns: make([]string, columns), values: values}, nil
}

func reviewSummaryFixture(t *testing.T, actorID uint64) (*Reviews, *sql.DB, *reviewSummaryModel, Actor) {
	t.Helper()
	body, err := codec.Canonical(snippetBody(snippetBlock("paragraph", "fixed-p", snippetText("固定正文第一段")), snippetBlock("paragraph", "fixed-p2", snippetText("第二段"))))
	if err != nil {
		t.Fatal(err)
	}
	model := &reviewSummaryModel{access: &collaboratorModel{acl: "1"}, body: body}
	db := sql.OpenDB(reviewSummaryConnector{model})
	t.Cleanup(func() { db.Close() })
	actor := Actor{ID: actorID, Proof: &pb.IdentityAdmission{AdmissionId: "actor", Sources: []*pb.SourceEpoch{{ScopeKey: "identity:" + domain.TextID(actorID), Epoch: 1}}}}
	peer := &collaboratorIdentity{proofs: map[string]*pb.IdentityAdmission{"actor": actor.Proof}, actor: actorID}
	access := NewAccess(repository.NewAccess(db), NewIdentity(peer, nil, nil))
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	return NewReviews(db, access, NewPagination(collaboratorSigner{}), nil, validator), db, model, actor
}

func reviewSummaryPage(t *testing.T, db *sql.DB, run ReadBody) (dto.ReviewRequestPage, error) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	value, err := run(context.Background(), tx, "")
	if err != nil {
		return dto.ReviewRequestPage{}, err
	}
	return value.(dto.ReviewRequestPage), nil
}

func TestReviewSummaryUsesFixedRoundWithinOriginalReadTransaction(t *testing.T) {
	service, db, model, actor := reviewSummaryFixture(t, 1)
	sources, run, err := service.List(context.Background(), actor, 10, "20", "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := reviewSummaryPage(t, db, run)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%+v error=%v", page, err)
	}
	summary := page.Items[0].Summary
	if summary == nil || summary.Title != "送审时的章节" || summary.Excerpt != "固定正文第一段 第二段" || summary.RoundNo != "2" || summary.SubmitterId != "7" || summary.SubmittedAt != "2026-10-09T08:20:30.123Z" || page.Items[0].CurrentRound != nil {
		t.Fatalf("fixed lightweight summary=%+v", page.Items[0])
	}
	bound := map[string]bool{}
	for _, source := range sources {
		bound[source.ScopeKey] = true
	}
	if !bound["identity:1"] || !bound["doc-acl:10"] {
		t.Fatalf("output sources=%v", bound)
	}
	model.access.acl = "2"
	if _, err = reviewSummaryPage(t, db, run); err == nil || model.reads != 1 {
		t.Fatalf("revoked pending output queried summary: reads=%d error=%v", model.reads, err)
	}
}

func TestReviewSummaryRejectsUnauthorizedAndMissingFixedRound(t *testing.T) {
	service, _, model, actor := reviewSummaryFixture(t, 5)
	if _, _, err := service.List(context.Background(), actor, 10, "20", ""); err == nil || model.reads != 0 {
		t.Fatalf("unauthorized summary reads=%d error=%v", model.reads, err)
	}
	service, db, model, actor := reviewSummaryFixture(t, 1)
	_, run, err := service.List(context.Background(), actor, 10, "20", "")
	if err != nil {
		t.Fatal(err)
	}
	model.missing = true
	if _, err = reviewSummaryPage(t, db, run); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing fixed round silently replaced: %v", err)
	}
}

func TestReviewSummaryExcerptUnicodeAndSemanticText(t *testing.T) {
	service, _, _, _ := reviewSummaryFixture(t, 1)
	link := snippetText("可见文本")
	link["marks"] = []any{map[string]any{"type": "link", "attrs": map[string]any{"href": "https://private.invalid", "title": "不可泄露链接属性"}}}
	for _, sample := range []struct {
		body any
		want string
	}{
		{snippetBody(snippetBlock("paragraph", "private-id", link), snippetBlock("paragraph", "p2", snippetText("下一段"))), "可见文本 下一段"},
		{snippetBody(snippetBlock("paragraph", "p", snippetText(strings.Repeat("中😀", 70)))), strings.Repeat("中😀", 70)},
		{snippetBody(snippetBlock("paragraph", "p", snippetText(strings.Repeat("中😀", 71)))), strings.Repeat("中😀", 70) + "…"},
		{snippetBody(snippetBlock("paragraph", "p")), ""},
	} {
		raw, err := codec.Canonical(sample.body)
		if err != nil {
			t.Fatal(err)
		}
		got, err := service.reviewSummaryExcerpt(raw)
		if err != nil || got != sample.want || !utf8.ValidString(got) || utf8.RuneCountInString(got) > 141 {
			t.Fatalf("excerpt=%q error=%v want=%q", got, err, sample.want)
		}
	}
	if _, err := service.reviewSummaryExcerpt([]byte(`{"type":"unknown","content":[]}`)); err == nil {
		t.Fatal("invalid fixed body produced a successful summary")
	}
}
