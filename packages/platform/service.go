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

var resourceUploadPath = regexp.MustCompile(`^/v1/resources/[1-9][0-9]{0,19}/upload$`)

type healthServer struct {
	healthpb.UnimplementedHealthServer
	store *Store
}

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

func (s *healthServer) Check(ctx context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	value := healthpb.HealthCheckResponse_SERVING
	if s.store.CheckReady(ctx) != nil {
		value = healthpb.HealthCheckResponse_NOT_SERVING
	}
	return &healthpb.HealthCheckResponse{Status: value}, nil
}

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
