# F03 写公共进程底座：配置、连接池、就绪和错误出口

## 这节课要实现什么

账号、文档和搜索服务都要读取配置、连接数据库、接收请求并安全退出。本课实现共用启动底座，让后续服务能接入自己的业务，而不重复编写整套启动代码。

## 做完后能看到什么

完整接线后，配置或依赖不满足要求时服务不报告就绪；非法内部调用被拒绝，面向用户的错误不泄露SQL和凭据。这里只搭运行基础，不提供登录等业务功能。

## 先想清楚怎样实现

1. 先读配置并建立有上限的连接池。
2. 再实现就绪与调用身份检查。
3. 然后把各服务提供的路由注册函数接入统一启动和停止流程。

## 怎样判断这一课完成

公共包依赖F04的声明，等F04写齐后执行本课具名规则测试；真实证书、连接池和服务启动留到06。

## 开始写之前

本课为三个 Go 服务提供同一个启动方式。目标是以后 account/main.go 只负责读取配置、把业务注册函数交给平台，不再复制一套 TLS、信号处理和数据库连接代码。HTTP 业务路由仍由各服务拥有，公共底座不变成能访问所有私有库的总服务。

## 1. 先写配置和 Store，不先启动空服务

```powershell
# 工作目录：自己的同一个学习工程。
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
```

打开 [store.go 分步操作](../../write/031-store.md)，创建文件后先写 Config，再写它引用的 ExportConfig、ResourceConfig、ResourceOSSConfig、SearchConfig、SMTPConfig、RedisConfig。这些是当前完整配置模型；运行时值由自己的实例准备工具生成，不把体验实例的配置粘到公开教程。

<!-- lesson-code: packages/platform/store.go :: Config -->
````go
// Config每个进程只读取自己的DSN与证书，不把迁移/root账号注入应用服务。
type Config struct {
	Role                         string            `json:"role"`
	Host                         string            `json:"host"`
	HTTPPort                     int               `json:"httpPort"`
	RPCPort                      int               `json:"rpcPort"`
	DSN                          string            `json:"dsn"`
	CertFile                     string            `json:"certFile"`
	KeyFile                      string            `json:"keyFile"`
	CAFile                       string            `json:"caFile"`
	ExpectedSQLSHA               string            `json:"expectedSQLSHA"`
	ExpectedAdditionalMigrations map[int]string    `json:"expectedAdditionalMigrations"`
	Origin                       string            `json:"origin"`
	Root                         string            `json:"root"`
	Peers                        map[string]string `json:"peers"`
	Redis                        *RedisConfig      `json:"redis"`
	SMTP                         *SMTPConfig       `json:"smtp"`
	ChallengeKey                 string            `json:"challengeKey"`
	Search                       *SearchConfig     `json:"search"`
	Resources                    *ResourceConfig   `json:"resources"`
	Exports                      *ExportConfig     `json:"exports"`
}
````
<!-- /lesson-code -->

按用途理解每个 Config 字段，后面写构造函数时才能知道它从哪来：

| 字段 | 作用和使用者 |
| --- | --- |
| Role | account/document/search 角色，日志、RPC 身份与私有数据归属都依赖它。 |
| Host、HTTPPort、RPCPort | 监听地址和两个协议端口；HTTPPort 与内部 RPC 端口不能互换。 |
| DSN | 本服务应用账号访问自己私有库，不能填迁移管理员。 |
| CertFile、KeyFile、CAFile | 本服务证书、私钥文件和信任根；不是用户登录密码。 |
| ExpectedSQLSHA、ExpectedAdditionalMigrations | 发布期望的 001 和后续迁移摘要，不能让数据库自行决定什么版本可用。 |
| Origin | 浏览器入口的精确来源，写请求要核验，不能填 `*`。 |
| Root | 实例根目录，定位本实例资源；不是任意用户可提交的文件路径。 |
| Peers | 目标服务角色到内部地址的映射，用于跨服务客户端。 |
| Redis | 验证码和限流依赖的连接信息，业务层只接收小接口。 |
| SMTP、ChallengeKey | 邮件发送设置、验证码保护用密钥；与原始验证码及会话 token 不同。 |
| Search | owner、Kafka 集群/topic/group、ES 地址/别名/初始索引和游标签名密钥、期限。 |
| Resources | 对象根目录/可选 OSS、空间和上传/临时预算、上传期限。 |
| Exports | 导出根目录、Pandoc/Node/浏览器路径和 worker 数。 |

子配置也逐字段写清：ExportConfig.Root 保存当前实例产物，Pandoc 处理 Word，Node/Browser 完成 PDF，Workers 控制并发（0 是默认单 worker，最大边界由业务装配核验）。ResourceConfig 的 SpaceBytes/UploadBytes/TemporaryBytes 分别限制持久空间、单次上传、临时占用，UploadTTLSeconds 让未完成上传可回收。OSS 的 Region/Endpoint/Bucket/Prefix 定位对象，CredentialsFile 指向私密凭据文件；启用 OSS 后 Root 仍用于锁和临时校验，不能删掉本地协调目录。

SearchConfig 的 Owner 隔离实例，TopicID 的真实绑定在后续重建事实中核验；Group 是消费者组，不是用户组织。CursorKey 仅签名分页状态，不能授予文档访问权。SMTPConfig.Address/From 指定收件服务和发件身份，Security 指定 TLS/STARTTLS，CredentialsFile/CAFile 承载认证和信任；仅本机开发收件器可使用已有兼容分支。RedisConfig.Host/User/Password 只装配给连接客户端，不直接传给 HTTP handler。

然后写 `LoadConfig`：读取给定文件，JSON 解码到 Config，失败返回固定分类。这里尚未验证全部业务参数，Document 的导出预算等在其装配中进一步校验；文件能解析不等于可以接流量。

<!-- lesson-code: packages/platform/store.go :: Store -->
````go
type Store struct {
	Context    context.Context
	ReadyExtra func(context.Context) error
	// 仅由启动装配登记已接管的持久流程；其事实完整性由 ReadyExtra 核验，未知种类仍拒绝。
	ManagedTasks                 []string
	Role                         string
	Conn                         sqlx.SqlConn
	DB                           *sql.DB
	ExpectedSQLSHA               string
	ExpectedAdditionalMigrations map[int]string
}
````
<!-- /lesson-code -->

`Context` 随进程终止取消后台任务；`ReadyExtra` 是业务恢复屏障；`ManagedTasks` 只登记已由本服务接管的持久任务种类；`Role` 标识服务；`Conn` 给现有 go-zero SQL 适配使用，`DB` 给显式事务使用，它们包装的是同一个池；两个摘要字段保存受信的期望迁移。不能看到两个字段就创建两个连接池。

## 2. 创建有界连接池，并实现就绪检查

按操作页继续写 OpenStore。它的输入是 Config，成功返回本进程拥有的 `*Store`，失败返回 nil,error。`sql.Open` 本身不证明连通，所以用两秒 context 做 Ping；失败先 Close，不留下一个失效池。

<!-- lesson-code: packages/platform/store.go :: OpenStore -->
````go
func OpenStore(cfg Config) (*Store, error) {
	// go-zero 的 NewMysql 按 DSN 共享全局连接池；这里由服务实例拥有池的生命周期，
	// 避免一个实例退出后把同 DSN 的新实例也变成“连接池已关闭”。查询仍使用 go-zero sqlx。
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, errors.New("DATABASE_UNAVAILABLE")
	}
	conn := sqlx.NewSqlConnFromDB(db)
	db.SetMaxOpenConns(8)
	// 后台领取者会短时同时使用8个连接。空闲上限低于总上限会在每轮归还时
	// 主动关闭健康连接，下一轮重新建连，长期耗尽Windows临时TCP端口。
	// 保留同一个有界池的全部连接，不增加并发上限，也不改变查询或事务期限。
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, done := context.WithTimeout(context.Background(), 2*time.Second)
	defer done()
	if db.PingContext(ctx) != nil {
		_ = db.Close()
		return nil, errors.New("DATABASE_UNAVAILABLE")
	}
	return &Store{
		Context:                      context.Background(),
		Role:                         cfg.Role,
		Conn:                         conn,
		DB:                           db,
		ExpectedSQLSHA:               cfg.ExpectedSQLSHA,
		ExpectedAdditionalMigrations: maps.Clone(cfg.ExpectedAdditionalMigrations),
	}, nil
}
````
<!-- /lesson-code -->

8 个最大连接限制数据库并发，8 个空闲连接避免每轮 worker 归还后又主动关掉健康连接，在 Windows 上反复建 TCP 连接耗尽临时端口；五分钟生命周期用于连接轮换，不是业务事务期限。期望迁移 map 用 `maps.Clone`，防止外部继续修改配置而改变就绪判断。

新建 [schema_readiness.go](../../write/028-schema-readiness.md)，写下面完整比较器，再回到 Store.CheckReady 的分段步骤：

<!-- lesson-code: packages/platform/schema_readiness.go :: matchesSchemaMigrations -->
````go
// 期望集合来自受校验的发布配置；数据库不能自行声明一个新摘要就让服务认可。
// 001仍独立比较，附加迁移既不能缺失，也不能跳过版本或忽略未知记录。
func matchesSchemaMigrations(baseline string, additional, actual map[int]string) bool {
	if len(actual) != 1+len(additional) {
		return false
	}
	for version := 1; version <= 1+len(additional); version++ {
		expected := baseline
		if version > 1 {
			expected = additional[version]
		}
		decoded, err := hex.DecodeString(expected)
		if err != nil || len(decoded) != 32 || actual[version] != expected {
			return false
		}
	}
	return true
}
````
<!-- /lesson-code -->

以 baseline 为 001、additional 含 002 为例：实际必须恰有两条，版本从 1 连续到 2，摘要都为规范的 32 字节十六进制，逐项相等。实际只有 001、存在未知 003、把 002 的摘要改成 001，任何一种都返回 false。这样 READY 不会只检查“最大版本等于 12”却漏掉中间缺失或改过的迁移。

CheckReady 再看 durable_task 和 access_gate 的恢复事实。被 ManagedTasks 排除的任务必须有 ReadyExtra 负责核验，不能为了变绿直接忽略未恢复任务。数据库连通是第一层，schema 正确是第二层，原流程恢复是第三层。`LIVE` 只表示进程还活着，不能替代 READY。

新建 [store_diagnostics.go](../../write/032-store-diagnostics.md)。这个函数只在 `COLLAB_SQL_POOL_DIAGNOSTICS=1` 时每十秒记录聚合统计，并随 Context 取消；不记录 SQL/用户/DSN，也不另建池。普通用户页面不展示它的运维信息。

## 3. 先写安全边界，再写服务器启动器

按 [security.go](../../write/029-security.md) 写嵌入 RPC policy、AuthorizeRPC/RPCInterceptor、ErrorResponse、NewTraceID、WriteSafeError、WriteJSON。

<!-- lesson-code: packages/platform/security.go :: AuthorizeRPC -->
````go
func AuthorizeRPC(ctx context.Context, method string) error {
	// 身份来自握手验证过的证书URI，禁止把浏览器头或证书CN当成服务能力。
	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "SERVICE_IDENTITY_REQUIRED")
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.VerifiedChains) == 0 || len(info.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "SERVICE_IDENTITY_REQUIRED")
	}
	role := ""
	for _, uri := range info.State.PeerCertificates[0].URIs {
		for _, candidate := range []string{"account", "document", "search", "collaboration"} {
			if uri.String() == "spiffe://collaboration-report/"+candidate {
				if role != "" && role != candidate {
					return status.Error(codes.Unauthenticated, "AMBIGUOUS_SERVICE_IDENTITY")
				}
				role = candidate
			}
		}
	}
	if role == "" {
		return status.Error(codes.Unauthenticated, "SERVICE_IDENTITY_REQUIRED")
	}
	deadline, ok := ctx.Deadline()
	// 调用方必须明确给出期限；超长期限也拒绝，避免后台资源被未知调用占住。
	if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 5*time.Second {
		return status.Error(codes.InvalidArgument, "FINITE_DEADLINE_REQUIRED")
	}
	for _, allowed := range rpcPolicy[method] {
		if role == allowed {
			return nil
		}
	}
	return status.Error(codes.PermissionDenied, "SERVICE_METHOD_FORBIDDEN")
}
````
<!-- /lesson-code -->

`ctx` 必须含握手验证过的 TLS 身份，`method` 是完整 RPC 方法名。函数依次验证 peer/TLS、证书 URI 的唯一角色、有限调用期限、该角色是否在方法 allowlist；成功返回 nil，其他返回对应 gRPC 状态。浏览器请求头里的 `role=account` 不会进入证书验证链。证书正确也不意味着它可以调用任意方法；期限大于五秒同样拒绝，不能占住后台资源。

ErrorResponse 的 Code 是稳定机器分类，Message 是可展示文字，TraceID 用于排查，RetryAction 告诉客户端怎样处理原操作。WriteSafeError 不把 err.Error 原样发给用户；即使驱动错误含 SQL 和 DSN，出口也只返回固定分类。业务 handler 会进一步映射邮箱占用、弱密码等具体错误，公共错误出口不能代替所有业务判断。

最后按 [service.go](../../write/030-service.md) 编写 RESTConfig、healthServer.Check、Run 和文件余下的同步输出边界。RESTConfig 关闭额外调试端口；account/document 不使用把响应放进另一个 goroutine 缓冲的默认 TimeoutHandler，因为撤权排空需要知道 handler 真正退出、已无待发字节。有限 context 与读写期限仍在，不是取消超时控制。

## 4. 接线：为什么注册函数是参数

Run 的签名接收 `registerRPC func(*grpc.Server,*Store,Config)` 与 `registerHTTP func(*rest.Server,*Store,Config) error`。它不 import account/document/search；各服务把自己的注册函数传入，依赖方向保持从服务到公共库。

实际顺序是 OpenStore→创建可取消进程 context→证书和信任根→创建 mTLS RPC server 与拦截器→调用 registerRPC→监听 RPC→创建 HTTPS server 与公共中间件/health→调用 registerHTTP→启动并响应终止信号。handler 必须在自己的注册函数中接上，不能指望定义一个 Login 方法后框架自动发现。

F03 的平台同包还引用 F04 中的协议声明。现在文件可以 gofmt，但平台整包测试等 F04 写齐；账号 HTTP 登录要等 A08 完整接线和 06 的独立实例，不在这里启动“全成功”的空业务服务。

## 5. 本课检查及排错

F04 写齐后在原工程运行下面纯规则关口，不需要 Docker：

```powershell
# 工作目录：自己的学习工程。
go -C packages/platform test -mod=readonly . -run '^(TestSchemaReadinessRequiresExactConfiguredMigrationSet|TestRPCRejectsMissingDeadline|TestRPCRejectsExcessiveDeadline|TestRPCCallerCannotUseOtherServiceCapability|TestRPCRejectsUntrustedIdentity|TestRPCRejectsCallerClaimWithoutTLS|TestRPCAllowsBoundedAuthorizedCall|TestHTTPErrorDoesNotDiscloseSQLOrCredential|TestDefaultFrameworkDebugServerRequiresExplicitDisable|TestProtectedAccountAndDocumentOutputsDoNotUseDetachedTimeoutBuffer)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '平台启动/安全规则失败' }
```

每个具名 Test 应 RUN/PASS，无 SKIP。这些测试用构造的 TLS/context 和配置核验规则，不代表真实证书握手或 MySQL 恢复已通过；真实环境检查在 06。

若出现 undefined，先核对 F04 前向声明，不删调用或临时返回 nil。真实启动若报 SERVICE_TLS_INVALID，查自己实例的证书路径和 CA；若数据库可 Ping 却不 READY，查迁移/未恢复任务，不删 CheckReady；若 HTTP 无此路由，去业务 Register 函数查 Method/Path，不向公共 Run 填业务 SQL。

继续 [F04 原操作与关闭协议](04-original-operation.md)。
