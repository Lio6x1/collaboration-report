package account

import (
	"crypto/tls"
	"crypto/x509"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAccountLifecycleBindingRequiresExactDocumentControlIdentity(t *testing.T) {
	// 已通过CA核验的其他服务也不能读原关闭范围；没有Doc身份时不应触及业务依赖。
	for _, role := range []string{"", "account", "collaboration"} {
		r := httptest.NewRequest("POST", "/infra/lifecycle/account-close-binding", strings.NewReader(`{"operation_id":"lifecycle-binding-original-01"}`))
		if role != "" {
			uri, err := url.Parse("spiffe://collaboration-report/" + role)
			if err != nil {
				t.Fatal(err)
			}
			certificate := &x509.Certificate{URIs: []*url.URL{uri}}
			r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate}, VerifiedChains: [][]*x509.Certificate{{certificate}}}
		}
		w := httptest.NewRecorder()
		NewAccountCloseDecisions(nil).Binding(w, r)
		if w.Code != 403 {
			t.Fatalf("%q服务取得了Doc控制范围: %d", role, w.Code)
		}
	}
}
