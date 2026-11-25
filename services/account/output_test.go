package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	domain "collaboration/account/internal/domain/account"
	authhandler "collaboration/account/internal/handler/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	"collaboration/account/internal/svc"
	"collaboration/platform"
)

func TestAccountOutputRejectsPausedOldAdministratorResponse(t *testing.T) {
	// 真实私有库，暂停在原资格已经读取、敏感响应尚未释放的位置。
	// 降级提交后旧响应必须被挡住，不能把新member epoch贴给旧管理员查询结果。
	file := os.Getenv("COLLAB_TEST_ACCOUNT_CONFIG")
	if file == "" {
		t.Fatal("explicit owned account test configuration required")
	}
	cfg, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.DB.Close() })
	ctx := context.Background()
	insert := func(query string, args ...any) int64 {
		r, e := store.DB.ExecContext(ctx, query, args...)
		if e != nil {
			t.Fatal(e)
		}
		id, _ := r.LastInsertId()
		return id
	}
	owner := insert("INSERT INTO user(display_name,state) VALUES('输出反例所有者','ACTIVE')")
	actor := insert("INSERT INTO user(display_name,state) VALUES('输出反例管理员','ACTIVE')")
	org := insert("INSERT INTO organization(name,owner_id,lifecycle) VALUES('输出反例组织',?,'ACTIVE')", owner)
	insert("INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", org, actor)
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash, _ := domain.TokenHash(token)
	insert("INSERT INTO session(id,user_id,csrf_hash,state,expires_at,last_seen_at) VALUES(?,?,?,'ACTIVE',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR),UTC_TIMESTAMP(6))", hash, actor, hash)
	t.Cleanup(func() {
		store.DB.Exec("DELETE FROM session WHERE id=?", hash)
		store.DB.Exec("DELETE FROM organization_member WHERE organization_id=?", org)
		store.DB.Exec("DELETE FROM organization WHERE id=?", org)
		store.DB.Exec("DELETE FROM user WHERE id IN (?,?)", owner, actor)
	})
	// 本样本只核验已有会话，不调用登录限流；使用生产装配和真实同池repository。
	authentication, err := svc.NewAuthentication(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	authenticationHTTP := authhandler.NewAuthentication(authentication, cfg.Origin, authhandler.CaptureSessionSources)
	checked := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	documentHTTP, err := platform.HTTPPeer(cfg, "document")
	if err != nil {
		t.Fatal(err)
	}
	outputs := authhandler.NewOutputs(svc.NewOutputs(store, documentHTTP, cfg.Peers["documentHttp"]))
	handler := outputs.Protect(func(w http.ResponseWriter, r *http.Request) {
		s, ok := authenticationHTTP.Authenticate(w, r, false)
		if !ok {
			return
		}
		if _, e := logic.AccessOrganization(r.Context(), store.DB, fmt.Sprint(org), s.UserID, logic.OrganizationAccess{Role: logic.OrganizationAdministrator}, authhandler.CaptureSource); e != nil {
			httpx.WriteError(w, e)
			return
		}
		close(checked)
		<-release
		platform.WriteJSON(w, 200, map[string]string{"reason": "private-old-admin-data"})
	})
	request := httptest.NewRequest("GET", "/v1/organizations/1/applications", nil)
	request.AddCookie(&http.Cookie{Name: httpx.SessionCookie, Value: token})
	request.Header.Set("X-Edge-Handler", "handler-0123456789abcdef")
	writer := httptest.NewRecorder()
	go func() { defer close(done); handler(writer, request) }()
	select {
	case <-checked:
	case <-time.After(time.Second):
		t.Fatal("authorization did not complete")
	}
	if _, err = store.DB.Exec("UPDATE organization_member SET role='MEMBER',epoch=epoch+1 WHERE organization_id=? AND user_id=?", org, actor); err != nil {
		close(release)
		<-done
		t.Fatal(err)
	}
	close(release)
	<-done
	if writer.Code != 503 || strings.Contains(writer.Body.String(), "private-old-admin-data") {
		t.Fatal("old administrator output was released after the member epoch changed")
	}
}

func TestMinimalAccountReceiptNeverContainsSensitiveProjection(t *testing.T) {
	if !authhandler.MinimalReceipt([]byte(`{"operation_id":"operation-0123456789","state":"SUCCEEDED","result":{"entity_id":"1"}}`)) {
		t.Fatal("minimal receipt rejected")
	}
	for _, raw := range []string{`{"operation_id":"operation-0123456789","state":"SUCCEEDED","share_url":"secret"}`, `{"operation_id":"operation-0123456789","state":"SUCCEEDED","result":{"entity_id":"1","email":"secret"}}`} {
		if authhandler.MinimalReceipt([]byte(raw)) {
			t.Fatal("sensitive projection bypassed output qualification")
		}
	}
}

func TestMinimalAccountClosureReceiptBoundToCanonicalOriginalPlan(t *testing.T) {
	// 实际完整注销曾被OUTPUT_LEASE_REQUIRED挡住；专用恢复豁免仍必须拒绝私密附加字段及重复键。
	path := "/v1/account/closures/account-close-12345678"
	raw := `{"operation_id":"account-close-12345678","state":"CLEANING","required_successions":[],"retained_release_ids":["12"]}`
	if !authhandler.MinimalClosureReceipt(path, []byte(raw+"\n")) {
		t.Fatal("original canonical closure plan rejected")
	}
	for _, body := range []string{
		strings.Replace(raw, `"state":`, `"state":"secret-prefix","state":`, 1),
		strings.Replace(raw, `"retained_release_ids":`, `"email":"secret","retained_release_ids":`, 1),
		strings.Replace(raw, `"12"`, `"18446744073709551616"`, 1),
	} {
		if authhandler.MinimalClosureReceipt(path, []byte(body)) {
			t.Fatal("sensitive or malformed closure response accepted")
		}
	}
	if authhandler.MinimalClosureReceipt("/v1/organizations", []byte(raw)) || authhandler.MinimalClosureReceipt(path+"-another", []byte(raw)) {
		t.Fatal("closure output accepted outside original route")
	}
}
