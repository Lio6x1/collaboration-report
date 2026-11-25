package platform

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func identityContext(role string, duration time.Duration) (context.Context, context.CancelFunc) {
	uri, _ := url.Parse("spiffe://collaboration-report/" + role)
	ctx := peer.NewContext(context.Background(), &peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{PeerCertificates: []*x509.Certificate{{URIs: []*url.URL{uri}}}, VerifiedChains: [][]*x509.Certificate{{{URIs: []*url.URL{uri}}}}}}})
	if duration == 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, duration)
}

func TestRPCRejectsMissingDeadline(t *testing.T) {
	ctx, done := identityContext("document", 0)
	defer done()
	if status.Code(AuthorizeRPC(ctx, "/grpc.health.v1.Health/Check")) != codes.InvalidArgument {
		t.Fatal("RPC without finite deadline accepted")
	}
}
func TestRPCRejectsExcessiveDeadline(t *testing.T) {
	ctx, done := identityContext("document", time.Minute)
	defer done()
	if status.Code(AuthorizeRPC(ctx, "/grpc.health.v1.Health/Check")) != codes.InvalidArgument {
		t.Fatal("unbounded resource budget accepted")
	}
}
func TestRPCCallerCannotUseOtherServiceCapability(t *testing.T) {
	ctx, done := identityContext("account", time.Second)
	defer done()
	if status.Code(AuthorizeRPC(ctx, "/collaboration.v1.CollaborationInternal/CaptureSubmission")) != codes.PermissionDenied {
		t.Fatal("account was authorized to capture a document")
	}
}
func TestRPCRejectsUntrustedIdentity(t *testing.T) {
	ctx, done := identityContext("intruder", time.Second)
	defer done()
	if status.Code(AuthorizeRPC(ctx, "/grpc.health.v1.Health/Check")) != codes.Unauthenticated {
		t.Fatal("unknown SAN role accepted")
	}
}
func TestRPCRejectsCallerClaimWithoutTLS(t *testing.T) {
	ctx, done := context.WithTimeout(context.Background(), time.Second)
	defer done()
	if status.Code(AuthorizeRPC(ctx, "/grpc.health.v1.Health/Check")) != codes.Unauthenticated {
		t.Fatal("non-TLS call accepted")
	}
}
func TestRPCAllowsBoundedAuthorizedCall(t *testing.T) {
	ctx, done := identityContext("document", time.Second)
	defer done()
	if err := AuthorizeRPC(ctx, "/collaboration.v1.CollaborationInternal/CaptureSubmission"); err != nil {
		t.Fatal(err)
	}
}
func TestHTTPErrorDoesNotDiscloseSQLOrCredential(t *testing.T) {
	w := httptest.NewRecorder()
	WriteSafeError(w, "trace-example", errors.New("mysql SELECT password_hash FROM user; Cookie=secret-test-value"))
	if w.Code != 503 {
		t.Fatalf("unknown dependency outcome code=%d", w.Code)
	}
	if strings.Contains(w.Body.String(), "password_hash") || strings.Contains(w.Body.String(), "secret-test-value") {
		t.Fatal("private dependency failure disclosed")
	}
	if !strings.Contains(w.Body.String(), "SERVICE_UNAVAILABLE") || !strings.Contains(w.Body.String(), "trace-example") {
		t.Fatal("missing retryable safe response")
	}
}
