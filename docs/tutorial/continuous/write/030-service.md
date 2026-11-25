# 分步编写 `packages/platform/service.go`

先按 [F03 · 配置、连接池、就绪与安全出口](../lessons/foundation/03-runtime-and-security.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/030-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

统一启动HTTPS、mTLS RPC、健康探测和优雅生命周期。

业务注册只拿本服务Store；Account/Document输出需同步可控，框架额外goroutine缓冲不能冒充已经排空。

写之前先按这条执行路线理解各段的连接：

1. Run打开Store、建立退出context、配置TLS1.3和RPC拦截器，装配业务后再Serve。
2. RESTConfig关闭开发pprof和原始日志；Account/Doc普通API3秒、精确资源PUT上传30秒，并设读写deadline。
3. ready通过CheckReady返回，live仅说明进程活着；退出停止RPC/HTTP并关闭池。

本文件需要守住的边界：

- live=200不代表已恢复可服务；上传长预算只适用于严格匹配的PUT路径，不应扩展普通业务。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/service.go') -and (Get-Item -LiteralPath 'packages/platform/service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/service.go')) { New-Item -ItemType File -Path 'packages/platform/service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:409 -->
````go
package platform

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

````

## 第 2 步：resourceUploadPath

插入位置：第 1 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`var resourceUploadPath = regexp.MustCompile(\`^/v1/resources/[1-9][0-9]{0,19}/upload$\`)`。

<!-- write-step: 2 bytes=409:497 -->
````go
var resourceUploadPath = regexp.MustCompile(`^/v1/resources/[1-9][0-9]{0,19}/upload$`)

````

## 第 3 步：healthServer

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`type healthServer struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>healthpb.UnimplementedHealthServer</code> | <code>healthpb.UnimplementedHealthServer</code> | 嵌入gRPC健康服务默认实现，未覆写方法保持框架约定。 |
| <code>store</code> | <code>*Store</code> | 当前服务Store，Check调用它的恢复/版本就绪屏障。 |

<!-- write-step: 3 bytes=497:577 -->
````go
type healthServer struct {
	healthpb.UnimplementedHealthServer
	store *Store
}

````

## 第 4 步：RESTConfig

插入位置：第 3 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`func RESTConfig(cfg Config) (rest.RestConf, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>Config</code> | 使用 <code>Config</code> 类型；其定义处的职责：Config每个进程只读取自己的DSN与证书，不把迁移/root账号注入应用服务。<br>在此声明中的实际使用：<code>cfg.Role</code>；<code>cfg.Host</code>；<code>cfg.HTTPPort</code>；<code>cfg.CertFile</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>rest.RestConf</code> | 实际返回表达式：<code>rc</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SERVICE_CONFIG_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>conf.FillDefault(&amp;rc) != nil</code>。条件成立时执行 <code>return rc, errors.New(&quot;SERVICE_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>cfg.Role == &quot;document&quot; &#124;&#124; cfg.Role == &quot;account&quot;</code>。
- 此处通过 <code>return rc, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=577:1373 -->
````go
func RESTConfig(cfg Config) (rest.RestConf, error) {
	var rc rest.RestConf
	if conf.FillDefault(&rc) != nil {
		return rc, errors.New("SERVICE_CONFIG_INVALID")
	}
	rc.Name = cfg.Role
	rc.Host = cfg.Host
	rc.Port = cfg.HTTPPort
	rc.CertFile = cfg.CertFile
	rc.KeyFile = cfg.KeyFile
	// go-zero默认打开额外6060/pprof端口；本项目只暴露监督器记录的HTTPS入口。
	rc.DevServer.Enabled = false
	rc.Middlewares.Log = false
	rc.Middlewares.Recover = false
	// 默认TimeoutHandler另起goroutine并缓冲正文，Flush不保留状态码，也不能证明
	// 业务handler结束后已无待发字节。文档输出必须同步排空，改用下方有限context/IO期限。
	if cfg.Role == "document" || cfg.Role == "account" {
		rc.Middlewares.Timeout = false
	}
	return rc, nil
}

````

## 第 5 步：healthServer.Check

插入位置：第 4 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`func (s *healthServer) Check(ctx context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.store.CheckReady(ctx)</code> |
| <code>_</code> | <code>*healthpb.HealthCheckRequest</code> | 健康请求对象未使用；本实现统一检查整个当前服务Store。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*healthpb.HealthCheckResponse</code> | 实际返回表达式：<code>&amp;healthpb.HealthCheckResponse{Status: value}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.store.CheckReady(ctx) != nil</code>。
- 此处通过 <code>return &amp;healthpb.HealthCheckResponse{Status: value}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1373:1695 -->
````go
func (s *healthServer) Check(ctx context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	value := healthpb.HealthCheckResponse_SERVING
	if s.store.CheckReady(ctx) != nil {
		value = healthpb.HealthCheckResponse_NOT_SERVING
	}
	return &healthpb.HealthCheckResponse{Status: value}, nil
}

````

## 第 6 步：Run

插入位置：第 5 步代码之后，紧接追加；完成稿约第 63 行。

Run 统一启动HTTPS和内部RPC；注册业务路由的函数只拿本服务的私有Store。

本步声明：`func Run(cfg Config, registerRPC func(*grpc.Server, *Store, Config), registerHTTP func(*rest.Server, *Store, Config) error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>Config</code> | 使用 <code>Config</code> 类型；其定义处的职责：Config每个进程只读取自己的DSN与证书，不把迁移/root账号注入应用服务。<br>在此声明中的实际使用：<code>OpenStore(cfg)</code>；<code>cfg.CertFile</code>；<code>cfg.KeyFile</code>；<code>cfg.CAFile</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>registerRPC</code> | <code>func(*grpc.Server, *Store, Config)</code> | 在此声明中的实际使用：<code>registerRPC(rpc, store, cfg)</code> |
| <code>registerHTTP</code> | <code>func(*rest.Server, *Store, Config) error</code> | 在此声明中的实际使用：<code>registerHTTP != nil</code>；<code>registerHTTP(server, store, cfg)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SERVICE_TLS_INVALID&quot;)</code>；<code>errors.New(&quot;SERVICE_PORT_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SERVICE_HTTP_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>store.DB.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SERVICE_TLS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!ca.AppendCertsFromPEM(caBytes)</code>。条件成立时执行 <code>return errors.New(&quot;SERVICE_TLS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SERVICE_PORT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>listener.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SERVICE_HTTP_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=1695:3892 -->
````go
// Run 统一启动HTTPS和内部RPC；注册业务路由的函数只拿本服务的私有Store。
func Run(cfg Config, registerRPC func(*grpc.Server, *Store, Config), registerHTTP func(*rest.Server, *Store, Config) error) error {
	store, err := OpenStore(cfg)
	if err != nil {
		return err
	}
	defer store.DB.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store.Context = ctx
	store.runPoolDiagnostics(ctx)
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return errors.New("SERVICE_TLS_INVALID")
	}
	caBytes, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return errors.New("SERVICE_TLS_INVALID")
	}
	ca := x509.NewCertPool()
	if !ca.AppendCertsFromPEM(caBytes) {
		return errors.New("SERVICE_TLS_INVALID")
	}
	// mTLS证明“谁在调用”，方法allowlist证明“它能调用什么”；两层都不可省。
	rpc := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}, ClientCAs: ca, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS13})), grpc.UnaryInterceptor(RPCInterceptor), grpc.MaxRecvMsgSize(32<<20), grpc.MaxSendMsgSize(32<<20))
	registerRPC(rpc, store, cfg)
	healthpb.RegisterHealthServer(rpc, &healthServer{store: store})
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.RPCPort))
	if err != nil {
		return errors.New("SERVICE_PORT_UNAVAILABLE")
	}
	defer listener.Close()
	rc, err := RESTConfig(cfg)
	if err != nil {
		return err
	}
	server, err := rest.NewServer(rc, rest.WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS13, ClientCAs: ca, ClientAuth: tls.VerifyClientCertIfGiven}), rest.WithNotFoundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, 501, ErrorResponse{Code: "FEATURE_NOT_IMPLEMENTED", Message: "该功能将在后续批次实现", TraceID: NewTraceID(), RetryAction: "NONE"})
	})))
	if err != nil {
		return errors.New("SERVICE_HTTP_INVALID")
	}
	// 框架/驱动可能把SQL和请求头写入默认日志。禁用原始日志，只写下面的允许字段。
	logx.Disable()
	server.Use(func(next http.HandlerFunc) http.HandlerFunc {
````

## 第 7 步：Run（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 108 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return func(w http.ResponseWriter, r *http.Request) { 			if cfg.Role == &quot;document&quot; &#124;&#124; cfg.Role == &quot;account&quot; { 				timeout := 3 * time.Second 				// 仅有长度上限的二进制PUT允许较长传输；普通API仍使用原期限。 …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>cfg.Role == &quot;document&quot; &#124;&#124; cfg.Role == &quot;account&quot;</code>。
- 在本段的匿名函数内：检查 <code>cfg.Role == &quot;document&quot; &amp;&amp; r.Method == http.MethodPut &amp;&amp; resourceUploadPath.MatchString(r.URL.Path)</code>。
- 在本段的匿名函数内：执行 <code>context.WithTimeout(r.Context(), timeout)</code>。
- 在本段的匿名函数内：登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：登记 <code>func() { 				if recover() != nil { 					WriteSafeError(w, NewTraceID(), errors.New(&quot;HANDLER_FAILED&quot;)) 				} 			}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>recover() != nil</code>。
- 在本段的匿名函数内：执行 <code>context.WithTimeout(r.Context(), time.Second)</code>。
- 在本段的匿名函数内：登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>registerHTTP != nil</code>。
- 检查 <code>registerHTTP(server, store, cfg) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SERVICE_FEATURE_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 用独立 goroutine 执行 <code>func() { 		if rpc.Serve(listener) != nil { 			failure &lt;- errors.New(&quot;SERVICE_RPC_STOPPED&quot;) 		} 	}()</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。

<!-- write-step: 7 bytes=3892:5653 -->
````go
		return func(w http.ResponseWriter, r *http.Request) {
			if cfg.Role == "document" || cfg.Role == "account" {
				timeout := 3 * time.Second
				// 仅有长度上限的二进制PUT允许较长传输；普通API仍使用原期限。
				if cfg.Role == "document" && r.Method == http.MethodPut && resourceUploadPath.MatchString(r.URL.Path) {
					timeout = 30 * time.Second
				}
				ctx, cancel := context.WithTimeout(r.Context(), timeout)
				defer cancel()
				r = r.WithContext(ctx)
				controller := http.NewResponseController(w)
				_ = controller.SetWriteDeadline(time.Now().Add(timeout + time.Second))
				_ = controller.SetReadDeadline(time.Now().Add(timeout + time.Second))
			}
			// 开发监督器用本轮标记拒绝旧进程就绪，标记本身不承担业务授权。
			w.Header().Set("X-Dev-Run-Owner", os.Getenv("COLLAB_RUN_OWNER"))
			defer func() {
				if recover() != nil {
					WriteSafeError(w, NewTraceID(), errors.New("HANDLER_FAILED"))
				}
			}()
			next(w, r)
		}
	})
	server.AddRoutes([]rest.Route{
		{Method: "GET", Path: "/health/live", Handler: func(w http.ResponseWriter, r *http.Request) {
			WriteJSON(w, 200, map[string]string{"state": "LIVE", "role": cfg.Role})
		}},
		{Method: "GET", Path: "/health/ready", Handler: func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), time.Second)
			defer cancel()
			if err := store.CheckReady(ctx); err != nil {
				WriteSafeError(w, NewTraceID(), err)
				return
			}
			WriteJSON(w, 200, map[string]string{"state": "READY", "role": cfg.Role})
		}},
	})
	if registerHTTP != nil {
		if registerHTTP(server, store, cfg) != nil {
			return errors.New("SERVICE_FEATURE_CONFIG_INVALID")
		}
	}
	failure := make(chan error, 1)
	go func() {
````

## 第 8 步：Run（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 153 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>rpc.Serve(listener) != nil</code>。
- 用独立 goroutine 执行 <code>server.Start()</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=5653:5909 -->
````go
		if rpc.Serve(listener) != nil {
			failure <- errors.New("SERVICE_RPC_STOPPED")
		}
	}()
	go server.Start()
	slog.Info("service_started", "role", cfg.Role)
	select {
	case <-ctx.Done():
	case err = <-failure:
	}
	rpc.Stop()
	server.Stop()
	return err
}

````

## 第 9 步：Main

插入位置：第 8 步代码之后，紧接追加；完成稿约第 168 行。

Main把配置/驱动细节留在进程内；启动失败只输出稳定错误码。

本步声明：`func Main(role string, registerRPC func(*grpc.Server, *Store, Config), registerHTTP func(*rest.Server, *Store, Config) error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>cfg.Role != role</code>；<code>slog.Error(&quot;SERVICE_START_FAILED&quot;, &quot;role&quot;, role)</code> |
| <code>registerRPC</code> | <code>func(*grpc.Server, *Store, Config)</code> | 在此声明中的实际使用：<code>Run(cfg, registerRPC, registerHTTP)</code> |
| <code>registerHTTP</code> | <code>func(*rest.Server, *Store, Config) error</code> | 在此声明中的实际使用：<code>Run(cfg, registerRPC, registerHTTP)</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(os.Args) &lt; 3 &#124;&#124; os.Args&#91;1&#93; != &quot;--config&quot;</code>。
- 检查 <code>err != nil &#124;&#124; cfg.Role != role</code>。
- 检查 <code>Run(cfg, registerRPC, registerHTTP) != nil</code>。

<!-- write-step: 9 bytes=5909:6541 -->
````go
// Main把配置/驱动细节留在进程内；启动失败只输出稳定错误码。
func Main(role string, registerRPC func(*grpc.Server, *Store, Config), registerHTTP func(*rest.Server, *Store, Config) error) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if len(os.Args) < 3 || os.Args[1] != "--config" {
		slog.Error("SERVICE_CONFIG_REQUIRED")
		os.Exit(1)
	}
	cfg, err := LoadConfig(os.Args[2])
	if err != nil || cfg.Role != role {
		slog.Error("SERVICE_CONFIG_INVALID")
		os.Exit(1)
	}
	if Run(cfg, registerRPC, registerHTTP) != nil {
		slog.Error("SERVICE_START_FAILED", "role", role)
		os.Exit(1)
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/03-runtime-and-security.md)。
