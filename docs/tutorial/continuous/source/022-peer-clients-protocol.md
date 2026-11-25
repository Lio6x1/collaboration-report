# packages/platform/peer_clients_protocol.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/022-peer-clients-protocol.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`DialPeer`、`HTTPPeer`、`HTTPCaller`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package platform

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
	"net/http"
	"os"
	"time"
)

// client.go：保留原实现，按业务合并文件。

// DialPeer同时验证CA/hostname和目标服务URI，localhost证书不能互相冒充目标角色。
func DialPeer(cfg Config, role string) (*grpc.ClientConn, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, errors.New("RPC_TLS_INVALID")
	}
	data, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, errors.New("RPC_TLS_INVALID")
	}
	ca := x509.NewCertPool()
	if !ca.AppendCertsFromPEM(data) {
		return nil, errors.New("RPC_TLS_INVALID")
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: ca, ServerName: "localhost", MinVersion: tls.VersionTLS13, VerifyConnection: func(state tls.ConnectionState) error {
		for _, uri := range state.PeerCertificates[0].URIs {
			if uri.String() == "spiffe://collaboration-report/"+role {
				return nil
			}
		}
		return errors.New("RPC_PEER_IDENTITY_MISMATCH")
	}}
	target := cfg.Peers[role]
	if target == "" {
		return nil, errors.New("RPC_PEER_MISSING")
	}
	// 服务实际绑定IPv4 loopback。本机localhost的DNS/IPv6解析曾让有限RPC超时，
	// 直接连已知绑定地址；TLS仍验证localhost和目标服务URI，不降低证书校验。
	if host, port, err := net.SplitHostPort(target); err == nil && host == "localhost" {
		target = net.JoinHostPort("127.0.0.1", port)
	}
	return grpc.NewClient(target, grpc.WithTransportCredentials(credentials.NewTLS(config)))
}

// http_peer.go：保留原实现，按业务合并文件。

// HTTP基础设施控制与只读目录组合也验证CA及精确服务URI；它不扩展RPC allowlist。
func HTTPPeer(cfg Config, role string) (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, errors.New("HTTP_PEER_TLS_INVALID")
	}
	raw, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, errors.New("HTTP_PEER_TLS_INVALID")
	}
	ca := x509.NewCertPool()
	if !ca.AppendCertsFromPEM(raw) {
		return nil, errors.New("HTTP_PEER_TLS_INVALID")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: ca, ServerName: "localhost", MinVersion: tls.VersionTLS13, VerifyConnection: func(s tls.ConnectionState) error {
		for _, u := range s.PeerCertificates[0].URIs {
			if u.String() == "spiffe://collaboration-report/"+role {
				return nil
			}
		}
		return errors.New("HTTP_PEER_IDENTITY_MISMATCH")
	}}}
	return &http.Client{Transport: transport, Timeout: 1500 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}
func HTTPCaller(r *http.Request, role string) bool {
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
		return false
	}
	for _, uri := range r.TLS.PeerCertificates[0].URIs {
		if uri.String() == "spiffe://collaboration-report/"+role {
			return true
		}
	}
	return false
}
````

UTF-8/LF全文SHA-256：`c59e3550f54bd1b446277d64947ba9e1cb36c16b7350a77f28f1342695c45529`。[参考实现](../../../../packages/platform/peer_clients_protocol.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
