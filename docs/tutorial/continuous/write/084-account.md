# 分步编写 `services/account/internal/bootstrap/account.go`

先按 [A08 · 输出保护与HTTP、RPC、worker完整装配](../lessons/account/08-output-and-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/084-account.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把Account各用例、路由、恢复worker和就绪检查装成可运行服务。

只有路由能启动并不代表后台可以安全接管任务；ReadyExtra必须验证持久恢复绑定。

写之前先按这条执行路线理解各段的连接：

1. RegisterAccount建立Redis与Document RPC/HTTP客户端，注入Secrets和各业务。
2. 登记ManagedTasks，ReadyExtra检查Redis以及通知、Fence、取消、治理、组织关闭和注销恢复。
3. 先安装输出保护并注册全部路由，再用Store.Context启动各独立worker。

本文件需要守住的边界：

- SMTP启用时ChallengeKey必须32字节；Redis缺失的基础启动分支不代表登录功能已装好。
- 所有worker必须随服务停止，RPC连接在会话关闭worker退出时释放。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/bootstrap' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/bootstrap/account.go') -and (Get-Item -LiteralPath 'services/account/internal/bootstrap/account.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/bootstrap/account.go')) { New-Item -ItemType File -Path 'services/account/internal/bootstrap/account.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:562 -->
````go
package bootstrap

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"collaboration/account/internal/client"
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler"
	account "collaboration/account/internal/handler/account"
	logic "collaboration/account/internal/logic/account"
	"collaboration/account/internal/svc"
	"collaboration/account/internal/worker"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

````

## 第 2 步：RegisterAccount

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

RegisterAccount是组合入口：显式装配依赖和路由，不承载账号业务规则或SQL。

本步声明：`func RegisterAccount(server *rest.Server, store *platform.Store, cfg platform.Config, bindOperations func(*logic.OperationResults)) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.Use</code>；<code>handler.RegisterAuthentication(server, authenticationHTTP)</code>；<code>handler.RegisterIdentityRenewal(server, account.NewIdentityRenewal(svc.NewIdentity(store)))</code>；<code>handler.RegisterNotificationRecipients(server, account.NewNotificationRecipients(logic.NewNotificationRecipients(store.DB)))</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>svc.NewAuthentication(store, limiter)</code>；<code>svc.NewMutations(store)</code>；<code>svc.NewChallenges(store, limiter, secrets, cfg.SMTP)</code>；<code>svc.NewSignups(store, challenges)</code><br>另有 14 处使用，继续按下方完整代码的顺序处理。 |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg.Redis</code>；<code>platform.DialPeer(cfg, &quot;document&quot;)</code>；<code>cfg.ChallengeKey</code>；<code>cfg.Origin</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>bindOperations</code> | <code>func(*logic.OperationResults)</code> | 在此声明中的实际使用：<code>bindOperations(logic.NewOperationResults(store.DB, logout, accountClosures))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;LOGIN_LIMITER_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;CHALLENGE_KEY_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>cfg.Redis == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;LOGIN_LIMITER_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>cfg.SMTP != nil</code>。
- 检查 <code>err != nil &#124;&#124; len(key) != 32</code>。条件成立时执行 <code>return errors.New(&quot;CHALLENGE_KEY_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=562:2333 -->
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
````

## 第 3 步：RegisterAccount（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 68 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>!limiter.PingCtx(ctx)</code>。条件成立时执行 <code>return errors.New(&quot;LOGIN_LIMITER_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return closeRecoveries.Ready(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2333:4932 -->
````go
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
````

## 第 4 步：RegisterAccount（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 116 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 用独立 goroutine 执行 <code>worker.RunAccountCloseRecovery(store.Context, closeFences)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 用独立 goroutine 执行 <code>worker.RunAccountCloseRecovery(store.Context, closeCancellations)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 用独立 goroutine 执行 <code>worker.RunAccountCloseRecovery(store.Context, closeRecoveries)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。

<!-- write-step: 4 bytes=4932:6692 -->
````go
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
````

## 第 5 步：RegisterAccount（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 161 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 用独立 goroutine 执行 <code>worker.RunOrganizationCloseRecovery(store.Context, organizationClosures)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 用独立 goroutine 执行 <code>func() { 		defer peer.Close() 		worker.RunSessionClosures(store.Context, closures) 	}()</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=6692:6875 -->
````go
	go worker.RunOrganizationCloseRecovery(store.Context, organizationClosures)
	go func() {
		defer peer.Close()
		worker.RunSessionClosures(store.Context, closures)
	}()
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/bootstrap/account.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/08-output-and-wiring.md)。
