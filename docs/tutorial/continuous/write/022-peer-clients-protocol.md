# 分步编写 `packages/platform/peer_clients_protocol.go`

先按 [F04 · 原操作、资格来源与跨服务身份](../lessons/foundation/04-original-operation.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/022-peer-clients-protocol.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

创建验证目标角色的内部RPC/HTTP客户端，并核验HTTP调用方证书。

所有本机服务可能同用localhost证书主机名，仍必须检查SPIFFE URI防止角色互相冒充。

写之前先按这条执行路线理解各段的连接：

1. DialPeer同时验证CA、localhost和目标role，localhost拨号改IPv4回环但不降低TLS验证。
2. HTTPPeer设置1.5秒总超时且拒绝重定向；HTTPCaller要求已验证证书链和精确角色URI。

本文件需要守住的边界：

- 不能信任自报Header角色，不能因IP直连关闭hostname验证；重定向不能把服务凭据转给另一目的地。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/peer_clients_protocol.go') -and (Get-Item -LiteralPath 'packages/platform/peer_clients_protocol.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/peer_clients_protocol.go')) { New-Item -ItemType File -Path 'packages/platform/peer_clients_protocol.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:322 -->
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

````

## 第 2 步：DialPeer

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

DialPeer同时验证CA/hostname和目标服务URI，localhost证书不能互相冒充目标角色。

本步声明：`func DialPeer(cfg Config, role string) (*grpc.ClientConn, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>Config</code> | 使用 <code>Config</code> 类型；其定义处的职责：Config每个进程只读取自己的DSN与证书，不把迁移/root账号注入应用服务。<br>在此声明中的实际使用：<code>cfg.CertFile</code>；<code>cfg.KeyFile</code>；<code>cfg.CAFile</code>；<code>cfg.Peers</code> |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;spiffe://collaboration-report/&quot;+role</code>；<code>target := cfg.Peers&#91;role&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*grpc.ClientConn</code> | 复用的内部RPC连接，调用沿各自ctx；连接创建者负责关闭，不要每次RPC重建连接。<br>由 <code>grpc.NewClient(target, grpc.WithTransportCredentials(credentials.NewTLS(config)))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>grpc.NewClient(target, grpc.WithTransportCredentials(credentials.NewTLS(config)))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;RPC_TLS_INVALID&quot;)</code>；<code>errors.New(&quot;RPC_PEER_MISSING&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;RPC_TLS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!ca.AppendCertsFromPEM(data)</code>。条件成立时执行 <code>return nil, errors.New(&quot;RPC_TLS_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>uri.String() == &quot;spiffe://collaboration-report/&quot;+role</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return errors.New(&quot;RPC_PEER_IDENTITY_MISMATCH&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>target == &quot;&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;RPC_PEER_MISSING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; host == &quot;localhost&quot;</code>。
- 此处通过 <code>return grpc.NewClient(target, grpc.WithTransportCredentials(credentials.NewTLS(config)))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=322:1792 -->
````go
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

````

## 第 3 步：HTTPPeer

插入位置：第 2 步代码之后，紧接追加；完成稿约第 54 行。

HTTP基础设施控制与只读目录组合也验证CA及精确服务URI；它不扩展RPC allowlist。

本步声明：`func HTTPPeer(cfg Config, role string) (*http.Client, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>Config</code> | 使用 <code>Config</code> 类型；其定义处的职责：Config每个进程只读取自己的DSN与证书，不把迁移/root账号注入应用服务。<br>在此声明中的实际使用：<code>cfg.CertFile</code>；<code>cfg.KeyFile</code>；<code>cfg.CAFile</code> |
| <code>role</code> | <code>string</code> | 期望对端精确角色，例如document，用于证书SPIFFE URI核验；不是调用方自报HTTP头。<br>在此声明中的实际使用：<code>&quot;spiffe://collaboration-report/&quot;+role</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>实际返回表达式：<code>nil</code>；<code>&amp;http.Client{Transport: transport, Timeout: 1500 * time.Millisecond, CheckRedirect: func(*http.Request, &#91;&#93;*http.Request) error { return http.ErrUseLastResponse }}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;HTTP_PEER_TLS_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;HTTP_PEER_TLS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!ca.AppendCertsFromPEM(raw)</code>。条件成立时执行 <code>return nil, errors.New(&quot;HTTP_PEER_TLS_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>u.String() == &quot;spiffe://collaboration-report/&quot;+role</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return errors.New(&quot;HTTP_PEER_IDENTITY_MISMATCH&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &amp;http.Client{Transport: transport, Timeout: 1500 * time.Millisecond, CheckRedirect: func(*http.Request, &#91;&#93;*http.Request) error { return http.ErrUseLastResponse }}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return http.ErrUseLastResponse</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1792:2900 -->
````go
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
````

## 第 4 步：HTTPCaller

插入位置：第 3 步代码之后，紧接追加；完成稿约第 78 行。

本步声明：`func HTTPCaller(r *http.Request, role string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.TLS</code> |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;spiffe://collaboration-report/&quot;+role</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>r.TLS == nil &#124;&#124; len(r.TLS.VerifiedChains) == 0 &#124;&#124; len(r.TLS.PeerCertificates) == 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>uri.String() == &quot;spiffe://collaboration-report/&quot;+role</code>。条件成立时执行 <code>return true</code>。当前函数在这里结束。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2900:3213 -->
````go
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

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/peer_clients_protocol.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/04-original-operation.md)。
