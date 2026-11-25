package account

import (
	"crypto/tls"
	"crypto/x509"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCollaboratorNameRequiresDocumentControlCertificate(t *testing.T) {
	// Cookie、自报请求头和其他合法服务证书都不能枚举姓名；拒绝先于业务依赖访问。
	for _, role := range []string{"", "account", "collaboration"} {
		request := httptest.NewRequest("POST", "/infra/identity/document-collaborator", strings.NewReader(`{}`))
		request.Header.Set("Cookie", "session=browser-cookie")
		request.Header.Set("X-Service-Role", "document")
		if role != "" {
			uri, _ := url.Parse("spiffe://collaboration-report/" + role)
			certificate := &x509.Certificate{URIs: []*url.URL{uri}}
			request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate}, VerifiedChains: [][]*x509.Certificate{{certificate}}}
		}
		response := httptest.NewRecorder()
		NewIdentityRenewal(nil).DocumentCollaborator(response, request)
		if response.Code != 403 {
			t.Fatalf("role %q queried collaborator identity: %d", role, response.Code)
		}
	}
}
