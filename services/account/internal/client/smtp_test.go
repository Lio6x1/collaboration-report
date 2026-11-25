package client

import (
	"bufio"
	"collaboration/platform"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type smtpObservation struct {
	authenticated, authBeforeTLS, dataAccepted bool
	recipient, message                         string
}

// 本地协议服务器只接一封自有测试邮件；同时观察认证所在的连接和DATA最后确认，
// 不使用真实邮箱、SMTP授权码或已运行的开发收件器。
func smtpFixture(t *testing.T, security, mechanism string, startTLS, acceptData bool) (*platform.SMTPConfig, <-chan smtpObservation) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "SMTP fixture"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	ca := filepath.Join(folder, "smtp-ca.pem")
	credentials := filepath.Join(folder, "smtp-credentials.json")
	if os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600) != nil ||
		os.WriteFile(credentials, []byte(`{"username":"sender@qq.com","password":"test-authorization-code"}`), 0600) != nil {
		t.Fatal("fixture files unavailable")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	// 先通过JSON装配，使旧配置忽略新字段时测试仍能运行并暴露真实明文限制。
	raw, _ := json.Marshal(map[string]any{"address": listener.Addr().String(), "from": "sender@qq.com", "security": security, "credentialsFile": credentials, "caFile": ca})
	var cfg platform.SMTPConfig
	if json.Unmarshal(raw, &cfg) != nil {
		t.Fatal("fixture config invalid")
	}
	if security == "development" || security == "" {
		raw, _ = json.Marshal(map[string]any{"address": listener.Addr().String(), "from": "sender@example.test", "security": security})
		cfg = platform.SMTPConfig{}
		_ = json.Unmarshal(raw, &cfg)
	}
	result := make(chan smtpObservation, 1)
	go func() {
		observed := smtpObservation{}
		defer func() { result <- observed }()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
		secure := false
		upgrade := func() bool {
			tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12})
			if tlsConn.Handshake() != nil {
				return false
			}
			conn, secure = tlsConn, true
			return true
		}
		if security == "tls" && !upgrade() {
			return
		}
		reader := bufio.NewReader(conn)
		reply := func(text string) { _, _ = fmt.Fprint(conn, text+"\r\n") }
		read := func() string {
			line, _ := reader.ReadString('\n')
			return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		}
		reply("220 local SMTP fixture")
		for {
			line := read()
			if line == "" {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				reply("250-fixture")
				if !secure && startTLS {
					reply("250-STARTTLS")
				}
				reply("250 AUTH " + mechanism)
			case line == "STARTTLS":
				reply("220 upgrade")
				if !upgrade() {
					return
				}
				reader = bufio.NewReader(conn)
			case strings.HasPrefix(line, "AUTH "):
				observed.authBeforeTLS = !secure
				var username, password string
				if mechanism == "LOGIN" {
					reply("334 " + base64.StdEncoding.EncodeToString([]byte("Username:")))
					user, _ := base64.StdEncoding.DecodeString(read())
					reply("334 " + base64.StdEncoding.EncodeToString([]byte("Password:")))
					pass, _ := base64.StdEncoding.DecodeString(read())
					username, password = string(user), string(pass)
				} else {
					parts := strings.Fields(line)
					if len(parts) == 3 {
						decoded, _ := base64.StdEncoding.DecodeString(parts[2])
						fields := strings.Split(string(decoded), "\x00")
						if len(fields) == 3 {
							username, password = fields[1], fields[2]
						}
					}
				}
				observed.authenticated = username == "sender@qq.com" && password == "test-authorization-code"
				reply("235 authenticated")
			case strings.HasPrefix(line, "MAIL FROM:"):
				reply("250 sender")
			case strings.HasPrefix(line, "RCPT TO:"):
				observed.recipient = line
				reply("250 recipient")
			case line == "DATA":
				reply("354 send data")
				var body strings.Builder
				for {
					data, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					line := strings.TrimSuffix(strings.TrimSuffix(data, "\n"), "\r")
					if line == "." {
						break
					}
					body.WriteString(line + "\n")
				}
				observed.message = body.String()
				observed.dataAccepted = acceptData
				if acceptData {
					reply("250 queued")
				} else {
					reply("554 server-password-oracle")
				}
			case line == "QUIT":
				reply("221 bye")
				return
			default:
				reply("500 unexpected command")
			}
		}
	}()
	return &cfg, result
}

func TestSMTPEncryptedAuthenticationAndDataConfirmation(t *testing.T) {
	for _, sample := range []struct {
		security, auth string
		accept         bool
	}{{"tls", "LOGIN", true}, {"starttls", "PLAIN", true}, {"starttls", "LOGIN", false}} {
		t.Run(sample.security+sample.auth+fmt.Sprint(sample.accept), func(t *testing.T) {
			cfg, result := smtpFixture(t, sample.security, sample.auth, true, sample.accept)
			mailer := NewSMTP(cfg)
			if mailer.DeliveryMode() != "SMTP" {
				t.Fatal("encrypted instance did not identify SMTP delivery")
			}
			err := mailer.Send(context.Background(), "recipient@example.net", "test-challenge", "123456")
			if sample.accept && err != nil || !sample.accept && (err == nil || err.Error() != "SMTP_DELIVERY_FAILED") {
				t.Fatalf("encrypted DATA outcome = %v", err)
			}
			observed := <-result
			if !observed.authenticated || observed.authBeforeTLS || observed.dataAccepted != sample.accept ||
				observed.recipient != "RCPT TO:<recipient@example.net>" || !strings.Contains(observed.message, "Code: 123456") {
				t.Fatal("authentication must use verified TLS and DATA must reach the intended non-QQ recipient")
			}
		})
	}
}

func TestSMTPRejectsTLSFailureWithoutSendingCredentials(t *testing.T) {
	for _, mode := range []string{"untrusted certificate", "wrong hostname", "missing STARTTLS"} {
		t.Run(mode, func(t *testing.T) {
			security := "starttls"
			if mode != "missing STARTTLS" {
				security = "tls"
			}
			cfg, result := smtpFixture(t, security, "PLAIN", false, true)
			if mode == "untrusted certificate" {
				// 不安装测试CA时必须沿系统信任拒绝自签证书，不能为方便联调跳过验证。
				var values map[string]any
				raw, _ := json.Marshal(cfg)
				_ = json.Unmarshal(raw, &values)
				values["caFile"] = ""
				raw, _ = json.Marshal(values)
				_ = json.Unmarshal(raw, cfg)
			}
			if mode == "wrong hostname" {
				_, port, _ := net.SplitHostPort(cfg.Address)
				cfg.Address = net.JoinHostPort("localhost", port)
			}
			err := NewSMTP(cfg).Send(context.Background(), "recipient@example.net", "test-challenge", "123456")
			if err == nil || err.Error() != "SMTP_TLS_FAILED" {
				t.Fatalf("TLS rejection = %v", err)
			}
			observed := <-result
			if observed.authenticated || observed.authBeforeTLS || observed.recipient != "" {
				t.Fatal("TLS failure leaked authentication or envelope")
			}
		})
	}
}

func TestSMTPDevelopmentAndCancellation(t *testing.T) {
	cfg, result := smtpFixture(t, "", "PLAIN", false, true)
	mailer := NewSMTP(cfg)
	if mailer.DeliveryMode() != "DEVELOPMENT" {
		t.Fatal("legacy receiver delivery must be reported as development")
	}
	// 接口调用者也必须受输入检查保护；不能等发出MAIL/RCPT后才发现头注入。
	if err := mailer.Send(context.Background(), "victim@example.net\r\nBcc: stolen@example.net", "test-challenge", "123456"); err == nil || err.Error() != "SMTP_INPUT_INVALID" {
		t.Fatalf("header injection = %v", err)
	}
	if err := mailer.Send(context.Background(), "recipient@example.net", "test-challenge", "123456"); err != nil {
		t.Fatal(err)
	}
	if observed := <-result; observed.authenticated || !observed.dataAccepted {
		t.Fatal("legacy loopback receiver must remain unauthenticated")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() { conn, _ := listener.Accept(); accepted <- conn }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		finished <- NewSMTP(&platform.SMTPConfig{Address: listener.Addr().String(), From: "sender@example.test"}).Send(ctx, "recipient@example.net", "test-challenge", "123456")
	}()
	conn := <-accepted
	defer conn.Close()
	// 服务器卡在banner时取消必须立即关连接；仅设置旧的绝对deadline无法做到这一点。
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("cancelled send succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("SMTP context cancellation did not interrupt the blocked command")
	}
}

func TestSMTPCommandDeadlineStopsStalledServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
		if conn == nil {
			return
		}
		defer conn.Close()
		_, _ = fmt.Fprint(conn, "220 stalled command fixture\r\n")
		_, _ = io.Copy(io.Discard, conn)
	}()
	finished := make(chan error, 1)
	go func() {
		finished <- NewSMTP(&platform.SMTPConfig{Address: listener.Addr().String(), From: "sender@example.test"}).Send(context.Background(), "recipient@example.net", "test-challenge", "123456")
	}()
	conn := <-accepted
	defer conn.Close()
	// 没有调用方deadline也必须有命令期限；服务器读到EHLO后永不应答，不能挂住请求。
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("stalled SMTP command was reported as delivered")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SMTP command deadline did not bound the blocked server")
	}
}
