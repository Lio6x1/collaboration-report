# services/account/internal/bootstrap/account.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/084-account.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：注册路由、RPC及恢复器，连接入口和完整业务。

主要声明：`RegisterAccount`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

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

UTF-8/LF全文SHA-256：`c0ffb83674f5c01ce60520f1cd2d8947fc5aadadedfeb434d0dab13b807ba62f`。[参考实现](../../../../services/account/internal/bootstrap/account.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
