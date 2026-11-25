# A08｜挡住过时响应，再把全部账号用例接进 HTTP、RPC 与后台任务

## 这节课要实现什么

前七课已经写了账号用例，现在要让浏览器和其他服务真正调用它们，并在权限撤销后阻止旧响应继续发送。本课完成账号服务接线与输出保护。

## 做完后能看到什么

本课结束可以构建完整账号包并运行离线规则；06接通依赖后，注册、登录、组织和通知相关入口才能通过真实请求工作。敏感查询曾获准，不代表撤权后仍可把结果发出。

## 先想清楚怎样实现

1. 先给响应绑定查询时实际使用的权限来源并在发送前复核。
2. 再创建共用依赖、登记HTTP和内部RPC路由、启动后台任务，最后接入进程入口和退出流程。

## 怎样判断这一课完成

本课执行账号全包构建和具名离线测试；真实Cookie、SMTP、组织审批和撤权中的响应关闭，按06逐项验证。

## 开始写之前

前七课已经写出业务，但“管理员曾被允许查询”不等于“现在仍能把查询结果发给浏览器”。本课先为敏感JSON绑定真实来源，输出前核代次并向Document登记受控handler，随后完成九个装配文件。课末才构建完整账号包，真实启动继续后面的Document/Collaboration/Search课程和06。

功能讲义与分步页讲的是同一份代码，同名声明只输入一次。先理解本课功能推演，再沿分步页的完整顺序录入；章末全文只作核对。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

依次按[来源模型](../../write/043-output.md)→[来源SQL](../../write/055-output.md)→[输出用例](../../write/066-output.md)→[输出中间件](../../write/077-outputs-handlers.md)→[身份装配](../../write/082-identity-wiring.md)→[完整HTTP路由](../../write/079-routes.md)→[RPC适配](../../write/080-rpc-server.md)→[组合入口](../../write/084-account.md)→[main](../../write/088-main.md)逐声明输入。正文完整代码与操作页是同一位置的两种讲解，已经输入同名声明时仅核对，不重复追加。现在补齐A01 CaptureSessionSources、A02/A03 CaptureSource，保持单向依赖：handler→logic→repository，svc/bootstrap只装配。

## 2. 先模型和消费者接口：绑定的是旧查询真实用过的代次

<!-- lesson-code: services/account/internal/domain/account/output.go :: OutputSource -->
````go
type OutputSource struct {
	Key   string `json:"scope_key"`
	Epoch string `json:"epoch"`
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/output.go :: OutputBinding -->
````go
type OutputBinding struct {
	Sources map[string]string
	Invalid bool
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/output.go :: OutputRegistrar -->
````go
type OutputRegistrar interface {
	Register(context.Context, string, []domain.OutputSource) error
}
````
<!-- /lesson-code -->

OutputSource.Key序列化scope_key，Epoch是十进制字符串；Binding.Sources将一次响应实际依赖的scope映射到当时epoch。Invalid表示同一scope在一个响应内出现冲突代次或零epoch，不能临时取“最新的那个”掩盖混快照。OutputRegistrar.Register(ctx,handlerID,sources) error要求Document为入口真实handler登记可排空的输出资格；返回nil才能释放响应，error可能是未知，不能继续把敏感正文写到浏览器。

这里仓储直接依赖*repository.Outputs，其CurrentEpoch先有ValidAccountOutputScope验证再按前缀查当前权威user/session/member/owner。identity/credential要求user ACTIVE；session要求ACTIVE且未过期；组织来源要求组织和成员ACTIVE、相应role或owner成立。SQL查无/故障与代次不匹配统一阻止当前输出，不能把新代次贴回旧结果。

## 3. 写来源捕获、重新核验与响应暂存

先写repository.CurrentEpoch和logic.Register，再写handler CaptureSource/CaptureSessionSources/MinimalReceipt/Protect。

<!-- lesson-code: services/account/internal/logic/account/output.go :: Outputs.Register -->
````go
func (s *Outputs) Register(ctx context.Context, handler string, binding *domain.OutputBinding) error {
	sources := make([]domain.OutputSource, 0, len(binding.Sources))
	for key, epoch := range binding.Sources {
		if !platform.ValidAccountOutputScope(key) {
			return errors.New("SOURCE_INVALID")
		}
		// 重检构造响应时捕获的代次；不能把新成员/角色代次贴到旧查询结果上。
		current, err := s.repository.CurrentEpoch(ctx, key)
		if err != nil || current != epoch {
			return errors.New("SOURCE_CHANGED")
		}
		sources = append(sources, domain.OutputSource{Key: key, Epoch: epoch})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Key < sources[j].Key })
	return s.registrar.Register(ctx, handler, sources)
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/handler/account/outputs_handlers.go :: CaptureSource -->
````go
func CaptureSource(ctx context.Context, key string, epoch uint64) {
	binding, _ := ctx.Value(outputKey{}).(*domain.OutputBinding)
	if binding == nil {
		return
	}
	value := strconv.FormatUint(epoch, 10)
	if epoch == 0 || (binding.Sources[key] != "" && binding.Sources[key] != value) {
		binding.Invalid = true
		return
	}
	binding.Sources[key] = value
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/handler/account/outputs_handlers.go :: CaptureSessionSources -->
````go
func CaptureSessionSources(ctx context.Context, session domain.Session, identity uint64) {
	credential, _ := strconv.ParseUint(session.CredentialEpoch, 10, 64)
	CaptureSource(ctx, "identity:"+session.UserID, identity)
	CaptureSource(ctx, "credential:"+session.UserID, credential)
	CaptureSource(ctx, "session:"+hex.EncodeToString(session.ID), session.Epoch)
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/handler/account/outputs_handlers.go :: Outputs.Protect -->
````go
func (h *Outputs) Protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") || anonymousOutput(r.URL.Path) || platform.HTTPCaller(r, "document") {
			next(w, r)
			return
		}
		binding := &domain.OutputBinding{Sources: map[string]string{}}
		r = r.WithContext(context.WithValue(r.Context(), outputKey{}, binding))
		capture := httpx.NewResponseCapture()
		next(capture, r)
		if capture.Status() >= 400 || MinimalReceipt(capture.Body()) || MinimalClosureReceipt(r.URL.Path, capture.Body()) {
			capture.CopyTo(w)
			return
		}
		// 短JSON完全构造后才登记入口实际handler，私有直连没有可控制输出资格。
		if r.Header.Get("X-Edge-Handler") == "" || binding.Invalid || len(binding.Sources) == 0 || capture.Size() > 2<<20 {
			httpx.Failure(w, 503, "OUTPUT_LEASE_REQUIRED")
			return
		}
		if err := h.logic.Register(r.Context(), r.Header.Get("X-Edge-Handler"), binding); err != nil {
			reason := "OUTPUT_REGISTER_UNKNOWN"
			if err.Error() == "SOURCE_CHANGED" || err.Error() == "SOURCE_INVALID" {
				reason = err.Error()
			}
			slog.Warn("account_output_pending", "reason", reason)
			httpx.Failure(w, 503, "ACCOUNT_OUTPUT_NOT_CURRENT")
			return
		}
		capture.CopyTo(w)
		_ = http.NewResponseController(w).Flush()
	}
}
````
<!-- /lesson-code -->

用组织9管理员17读取申请目录推演：

1. Protect为请求context放新的OutputBinding，用ResponseCapture暂存下游HTTP结果。Authentication.Authenticate捕获identity/credential/session来源；AccessOrganization捕获org-member/org-admin（owner路径捕获org-owner）。捕获用的必须是构造该响应时真实读到的epoch。
2. 下游SQL读出申请列表之后、尚未输出时，另一事务把17降级并将成员epoch5→6。Protect拿到旧JSON，logic.Register对org-admin:9:17做CurrentEpoch，查无当前ADMIN或值不同，返回SOURCE_CHANGED；浏览器得到503 ACCOUNT_OUTPUT_NOT_CURRENT，旧申请原因没有释放。
3. 当前来源仍相同时，来源按key排序，AccountOutput客户端以mTLS请求Document `/infra/output/register-account`，带真实X-Edge-Handler和全部sources。Document确认OPEN后才CopyTo真实writer并Flush。Account本库重读只是第一关，跨服务登记还会与撤权gate协调，不能把它删掉当性能优化。
4. 没有X-Edge-Handler、binding.Invalid、空sources或敏感响应超过2MiB，返回503 OUTPUT_LEASE_REQUIRED。直接访问Account私有HTTP端口没有入口受控handler，不能绕过Document/edge控制输出。

非/v1内部路由、匿名login/signup/challenges/password-resets及实际document控制caller按既有规则直接走；>=400错误也直接返回，不包含敏感投影。MinimalReceipt仅允许operation_id/state/可选poll_url/单entity_id结果；分享URL、姓名、邮箱、目录不能藏在“操作结果”里豁免。注销恢复由A07 MinimalClosureReceipt精确路由/原号/规范四字段单独校验，不能给所有JSON开一个全局恢复例外。

## 4. 写svc构造器：一个Store拥有连接池和生命周期

<!-- lesson-code: services/account/internal/svc/identity_wiring.go :: NewAuthentication -->
````go
func NewAuthentication(store *platform.Store, limit *redis.Redis) (*logic.Authentication, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy-not-a-user-credential"), 10)
	if err != nil {
		return nil, err
	}
	// Store拥有池的生命周期；装配只传递原池，业务事务不会跨连接池拆成两部分。
	repo := repository.NewAuthentication(store.Conn, store.DB)
	return logic.NewAuthentication(store.DB, repo, repository.NewLoginLimit(limit), dummy), nil
}
````
<!-- /lesson-code -->

NewAuthentication只在装配时算cost10 dummyHash，给repository.NewAuthentication传Store.Conn和同一Store.DB，给LoginLimit传Redis，logic只收消费者接口。不能每次未知邮箱临时生成dummy hash。NewChallenges注入Secrets、Redis仓储、SMTP客户端；NewSignups/NewCredentials共享同库与同挑战服务；NewIdentity/NewAuthorizationResults/NewMutations共享原DB；NewOutputs连接本库来源查询及mTLS AccountOutput客户端。A07 organization_lifecycle_wiring已装好成员/关闭/退出依赖，这里不再重建第二个池。

## 5. 写完整HTTP与RPC登记，每个动作都有落点

在routes.go按操作页输入全部Register函数；下面把实际入口与责任一一对应，不能只写Login留下其他未注册：

| 用户/控制入口 | handler与后续用例 |
| --- | --- |
| POST `/v1/account/login`、GET `/me`、POST `/infra/identity/verify-session` | Authentication → Login/CurrentUser/真实会话核验。 |
| POST `/v1/account/challenges`、`/signup`、`/password-resets`、`/password`、`/email` | Challenges/Credentials → 挑战、注册与凭据变更。 |
| POST `/v1/account/logout`、GET `/v1/account/operations/:operationId` | Logout → Prepare/Progress，原会话有限恢复。 |
| POST `/v1/account/closures`、GET `/:operationId` | AccountClosures → Start/Progress与自动恢复任务。 |
| GET/POST `/v1/organizations`、GET/PATCH `/:organizationId`、GET成员目录 | Organizations/OrganizationActions → 当前目录、创建和改名。 |
| 成员role、DELETE成员、leave、owner-transfers、creation-grants、closures | OrganizationTransitions/Actions/Closures → 原版本保护与来源关闭。 |
| 组织invitations、invitation cancel/accept、组织/本人applications、application decisions | Invitations/Applications → 分享、提交、审批，接受邀请不直接入会。 |
| `/infra/identity/renew-operation`、`/document-collaborator` | IdentityRenewal → 原证明续期/目标姓名，仅document证书。 |
| `/infra/notification/recipients`、`/administrators`、`/events`、`/events/ack` | A05资格、候选、事件与ACK。 |
| `/infra/lifecycle/account-close-binding/decision/confirmation`、`organization-close-confirmation`、`member-departure-status` | A03/A05/A06/A07权威控制查询，不能按超时猜结果。 |

<!-- lesson-code: services/account/internal/handler/routes.go :: RegisterAuthentication -->
````go
func RegisterAuthentication(server *rest.Server, authentication *account.Authentication) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/login", Handler: authentication.Login},
		{Method: "GET", Path: "/v1/account/me", Handler: authentication.Me},
		{Method: "POST", Path: "/infra/identity/verify-session", Handler: authentication.VerifySession},
	})
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/server/rpc_server.go :: Identity -->
````go
// RPC只适配已装配用例；服务身份、冻结调用者策略和deadline继续由平台拦截器核验。
type Identity struct {
	pb.UnimplementedIdentityInternalServer
	logic          *logic.Identity
	operations     *logic.OperationResults
	authorizations *logic.AuthorizationResults
}
````
<!-- /lesson-code -->

RPC Identity嵌入生成UnimplementedIdentityInternalServer，持logic.Identity/OperationResults/AuthorizationResults。Acquire/Revalidate适配身份证明；FenceAccess/GetAuthorizationChange只读原关闭证明；GetAccountOperation只查原回执。领域绑定错误转InvalidArgument、身份拒绝PermissionDenied、查无NotFound、不可核验Unavailable，不能把所有error当成功空结果。服务mTLS、调用方策略与deadline由platform拦截器统一核验，HTTPCaller检查仍留在内部HTTP入口。

## 6. 最后写bootstrap和main：启动时就明确所有依赖

<!-- lesson-code: services/account/internal/bootstrap/account.go :: RegisterAccount -->
````go
// RegisterAccount是组合入口：显式装配依赖和路由，不承载账号业务规则或SQL。
func RegisterAccount(server *rest.Server, store *platform.Store, cfg platform.Config, bindOperations func(*logic.OperationResults)) error {
	if cfg.Redis == nil {
		// 未启用登录子功能时，第1批底座仍可独立启动。
		return nil
	}
	limiter, err := redis.NewRedis(redis.RedisConf{
		Host:            cfg.Redis.Host,
		Type:            "node",
		User:            cfg.Redis.User,
		Pass:            cfg.Redis.Password,
		Protocol:        2,
		DisableIdentity: true,
		PingTimeout:     time.Second,
	})
	if err != nil {
		return errors.New("LOGIN_LIMITER_UNAVAILABLE")
	}
	peer, err := platform.DialPeer(cfg, "document")
	if err != nil {
		return err
	}
	authentication, err := svc.NewAuthentication(store, limiter)
	if err != nil {
		return err
	}
	secretKey, _ := hex.DecodeString(cfg.ChallengeKey)
	secrets := domain.NewSecrets(secretKey)
	authenticationHTTP := account.NewAuthentication(authentication, cfg.Origin, account.CaptureSessionSources)
	mutations := svc.NewMutations(store)
	documentHTTP, err := platform.HTTPPeer(cfg, "document")
	if err != nil {
		return err
	}
	if cfg.SMTP != nil {
		key, err := hex.DecodeString(cfg.ChallengeKey)
		if err != nil || len(key) != 32 {
			return errors.New("CHALLENGE_KEY_INVALID")
		}
	}
	challenges := svc.NewChallenges(store, limiter, secrets, cfg.SMTP)
	signups := svc.NewSignups(store, challenges)
	notificationEvents := logic.NewNotificationEvents(store.DB)
	closeFences := logic.NewAccountCloseFences(store.DB, client.NewDocument(pb.NewDocumentInternalClient(peer), documentHTTP, cfg.Peers["documentHttp"]))
	closeControl := client.NewAccountClosureControl(documentHTTP, cfg.Peers["documentHttp"])
	accountClosures := logic.NewAccountClosures(store.DB, authentication)
	logout := svc.NewLogout(store, authentication)
	bindOperations(logic.NewOperationResults(store.DB, logout, accountClosures))
	closeCancellations := logic.NewAccountCloseCancellations(store.DB, closeControl)
	closeRecoveries := logic.NewAccountCloseRecoveries(store.DB, accountClosures, closeControl)
	governanceOperator := logic.NewGovernanceOperator(store.DB)
	organizationClosures := svc.NewOrganizationClosures(store.DB, mutations, client.NewDocument(pb.NewDocumentInternalClient(peer), documentHTTP, cfg.Peers["documentHttp"]), documentHTTP, cfg.Peers["documentHttp"])
	store.ManagedTasks = append(store.ManagedTasks, "ACCOUNT_NOTICE_DELIVERY", "ACCOUNT_CLOSE_FENCE", "ACCOUNT_CLOSE_CANCEL", "ACCOUNT_CLOSE_RECOVER", "PLATFORM_GOVERNANCE_CHANGE", "ORG_CLOSE_RECOVER")
	store.ReadyExtra = func(ctx context.Context) error {
		if !limiter.PingCtx(ctx) {
			return errors.New("LOGIN_LIMITER_UNAVAILABLE")
		}
		if err := notificationEvents.Ready(ctx); err != nil {
			return err
		}
		if err := closeFences.Ready(ctx); err != nil {
			return err
		}
		if err := closeCancellations.Ready(ctx); err != nil {
			return err
		}
		if err := governanceOperator.Ready(ctx); err != nil {
			return err
		}
		if err := organizationClosures.Ready(ctx); err != nil {
			return err
		}
		return closeRecoveries.Ready(ctx)
	}

	// 来源捕获和输出保护必须使用同一上下文；真实响应在Doc登记成功后才释放。
	outputs := account.NewOutputs(svc.NewOutputs(store, documentHTTP, cfg.Peers["documentHttp"]))
	server.Use(outputs.Protect)
	handler.RegisterAuthentication(server, authenticationHTTP)
	handler.RegisterIdentityRenewal(server, account.NewIdentityRenewal(svc.NewIdentity(store)))
	handler.RegisterNotificationRecipients(server, account.NewNotificationRecipients(logic.NewNotificationRecipients(store.DB)))
	handler.RegisterNotificationEvents(server, account.NewNotificationEvents(notificationEvents))
	handler.RegisterAccountCloseDecisions(server, account.NewAccountCloseDecisions(accountClosures))
	handler.RegisterAccountClosures(server, account.NewAccountClosures(accountClosures, secrets, cfg.Origin))
	pages := account.NewPagination(logic.NewCursors(secrets))
	requests := account.NewMutationRequests(authenticationHTTP, secrets.Fingerprint)
	handler.RegisterOrganizationClosures(server, account.NewOrganizationClosures(requests, organizationClosures))
	organizations := account.NewOrganizations(
		authenticationHTTP,
		svc.NewOrganizations(store, account.CaptureSource),
		pages.Page,
		pages.Next,
	)
	handler.RegisterOrganizationReads(server, organizations)
	organizationActions := account.NewOrganizationActions(
		requests,
		svc.NewOrganizationActions(store, mutations, account.CaptureSource),
	)
	handler.RegisterOrganizationActions(server, organizationActions)
	transitions := account.NewOrganizationTransitions(
		authenticationHTTP,
		requests,
		svc.NewOrganizationTransitions(store, mutations, documentHTTP, cfg.Peers["documentHttp"], account.CaptureSource),
	)
	handler.RegisterOrganizationTransitions(server, transitions)

	invitationKey, _ := hex.DecodeString(secrets.Digest("invitation-encryption-v1"))
	invitations := account.NewInvitations(
		authenticationHTTP,
		requests,
		svc.NewInvitations(store, mutations, invitationKey, account.CaptureSource),
		cfg.Origin,
		pages.Page,
		pages.Next,
	)
	applications := account.NewApplications(
		authenticationHTTP,
		requests,
		svc.NewApplications(store, mutations, account.CaptureSource),
		pages.Page,
		pages.Next,
	)
	handler.RegisterMembership(server, invitations, applications)
	handler.RegisterLogout(server, account.NewLogout(logout, cfg.Origin))
	handler.RegisterChallenges(server, account.NewChallenges(authenticationHTTP, challenges, signups, secrets, cfg.Origin))
	credentials := account.NewCredentials(
		authenticationHTTP,
		svc.NewCredentials(store, authentication, challenges),
		signups,
		secrets,
		cfg.Origin,
	)
	handler.RegisterCredentials(server, credentials)

	closures := svc.NewClosure(store, pb.NewDocumentInternalClient(peer), documentHTTP, cfg.Peers["documentHttp"])
	go worker.RunAccountCloseRecovery(store.Context, closeFences)
	go worker.RunAccountCloseRecovery(store.Context, closeCancellations)
	go worker.RunAccountCloseRecovery(store.Context, closeRecoveries)
	go worker.RunOrganizationCloseRecovery(store.Context, organizationClosures)
	go func() {
		defer peer.Close()
		worker.RunSessionClosures(store.Context, closures)
	}()
	return nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/main.go :: main -->
````go
func main() {
	var identity *server.Identity
	platform.Main("account", func(rpc *grpc.Server, store *platform.Store, _ platform.Config) {
		identity = server.NewIdentity(svc.NewIdentity(store))
		identity.BindAuthorizationResults(svc.NewAuthorizationResults(store))
		pb.RegisterIdentityInternalServer(rpc, identity)
	}, func(http *rest.Server, store *platform.Store, cfg platform.Config) error {
		return bootstrap.RegisterAccount(http, store, cfg, identity.BindOperationResults)
	})
}
````
<!-- /lesson-code -->

RegisterAccount读取公开类型中的Redis配置创建受限客户端；无Redis是底座子功能未启用的旧能力边界，完整学习实例06必须生成Redis配置，不能把没注册账号路由误认为完成。然后DialPeer(document)和HTTPPeer(document)，创建Authentication、Secrets、Mutations、Challenges、Signups、Notifications、AccountClosures、Logout、Fence/Cancel/Recoveries、Governance、OrganizationClosures。

Store.ManagedTasks登记ACCOUNT_NOTICE_DELIVERY、ACCOUNT_CLOSE_FENCE/CANCEL/RECOVER、PLATFORM_GOVERNANCE_CHANGE、ORG_CLOSE_RECOVER；ReadyExtra既Ping Redis又查各恢复绑定。准备期/已领租约/真实原任务允许恢复，损坏绑定不能被“服务活着”遮住。Outputs.Protect装为server middleware，再注册全部HTTP。

后台分别运行账号Fence/Cancel/Recover、组织恢复和SessionClosures；都用Store.Context，Document peer在Session worker退出时Close。main先在platform.Main的RPC回调创建server.Identity、绑定AuthorizationResults、注册生成服务；HTTP回调RegisterAccount接identity.BindOperationResults，确保完整用例已注入才Serve，查询时不临时找全局服务。

一次用户改密调用链现在完整：edge HTTPS → account路由/Protect → Credentials handler严格DTO/Origin/CSRF → logic锁外bcrypt、Begin锁user/session → 同tx新hash+封旧来源+回执/task → 202 PREPARING → Session worker持久领取 → Document Fence→Collaboration/edge输出排空真实ACK → Account finish事务 → 原操作GET终态。任何环节未知都继续原号，跨服务无共享私有库事务。

## 7. 现在构建整个账号包，再执行离线精确测试

命令在自己的工程根目录，沿00已设置的Go1.25.10、锁定依赖、GOWORK=off与本工程GOCACHE；不修改go.mod以吸收遗漏声明。

```powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$env:GOCACHE = Join-Path $lessonRoot 'artifacts/go-cache'
go -C services/account build -mod=readonly ./...
if ($LASTEXITCODE -ne 0) { throw '账号包尚未完整或依赖不一致' }
go -C services/account test -mod=readonly . -run '^(TestMinimalAccountReceiptNeverContainsSensitiveProjection|TestMinimalAccountClosureReceiptBoundToCanonicalOriginalPlan)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '最小输出规则失败' }
node --no-maglev --test --test-reporter=tap tests/unit/account-output-policy.test.mjs
if ($LASTEXITCODE -ne 0) { throw '入口输出规则失败' }
```

构建退出0只是包齐；两条Go具名测试应RUN/PASS，Node精确名为 `sensitive invitation and directory fields cannot hide inside an unleased operation receipt`、`only canonical original closure plans cross the frozen-session output boundary`，应PASS且skip0。它们拒分享链接/邮箱等额外投影、错closure路径/原号/重复key和非法ID。再执行A01–A07登记的离线精确测试，不运行 `go test ./...` 把需要配置的真实集成用例误当离线。

缺symbol先核文件是否写全/包名import及重复追加声明；internal引用失败检查是否仍在services/account模块和本地replace目录；readonly要求更新go.mod时先核00锁定支持，不能直接go get latest。测试显示no tests to run应核00测试支持与精确名字，不能算通过。课程文件完整性另按阶段入口的 `python $coursePrepare --target $lessonRoot --check 02` 检查，素材复原检查不等于真实业务验收。

## 8. 等06再验证实际输出边界及整账号功能

真实私有库反例须显式自己的Account配置；本次编写未执行：

```powershell
$env:COLLAB_TEST_ACCOUNT_CONFIG = Join-Path $lessonRoot '.runtime/services/account.json'
go -C services/account test -mod=readonly . -run '^TestAccountOutputRejectsPausedOldAdministratorResponse$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

该测试暂停在读完旧管理员资格、响应尚未放行的位置，真实SQL将其降级/epoch+1再释放，必须503且不含private-old-admin-data。它需要真实账号库和平台peer配置，不能称离线。

06四服务、开发SMTP、edge/mTLS齐备后，在自己的根目录串行执行：

```powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/account/challenge.test.mjs tests/account/credential.test.mjs tests/account/identity.test.mjs tests/account/organization.test.mjs tests/account/output-boundary.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实账号模块存在失败，保留原操作与实例排查' }
```

精确关口包括 `signup uses real SMTP code and consumed challenge cannot register again`、`password reset closes old epoch and Redis restore cannot replay consumed verification`、`real identity admission binds session, operation, target and purpose then closes on logout`、`invitation means application only, cancellation after application does not invalidate approval`、`account sensitive responses cannot bypass the controlled edge by calling its private HTTP port`、`real account me output persists its exact session credential and identity dependencies`。所有文件真实执行的test应PASS且skip0；测试创建随机学习夹具，不对体验实例执行。通知/注销/恢复按A05–A07额外验收，不能省略后宣称账号全域已实测。

私有直连503是预期边界；edge也503先查X-Edge-Handler、来源捕获、current epoch与Document登记ACK。路由404先核Register调用与cfg.Redis，再看业务；Readiness失败看稳定绑定错误码，不能手改任务成功。当前文档交付只核源码/路径/测试存在性，不声称学生从空目录顺序编译与四服务流程已经跑通。

## 9. 章末核对

[模型](../../source/043-output.md)、[SQL](../../source/055-output.md)、[用例](../../source/066-output.md)、[中间件](../../source/077-outputs-handlers.md)、[svc](../../source/082-identity-wiring.md)、[路由](../../source/079-routes.md)、[RPC](../../source/080-rpc-server.md)、[bootstrap](../../source/084-account.md)、[main](../../source/088-main.md)。

为什么查到新epoch不能更新旧binding再放行？响应内容基于旧管理员资格构造，新epoch代表另一权限事实；给旧结果贴新代次会把撤权绕过去。正确行为是拒绝旧响应，由用户以当前资格重新请求。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
Set-Location -LiteralPath $lessonRoot
$paths = @(
 'services/account/internal/domain/account/output.go',
 'services/account/internal/repository/account/output.go',
 'services/account/internal/logic/account/output.go',
 'services/account/internal/handler/account/outputs_handlers.go',
 'services/account/internal/svc/identity_wiring.go',
 'services/account/internal/handler/routes.go',
 'services/account/internal/server/rpc_server.go',
 'services/account/internal/bootstrap/account.go',
 'services/account/main.go'
)
foreach ($path in $paths) {
 New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
 if (-not (Test-Path -LiteralPath $path)) { New-Item -ItemType File $path | Out-Null }
}
```
