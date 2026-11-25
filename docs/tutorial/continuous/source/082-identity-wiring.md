# services/account/internal/svc/identity_wiring.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/082-identity-wiring.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：创建依赖、注入构造函数；同一私有Store贯穿本服务。

主要声明：`NewAuthentication`、`NewChallenges`、`NewSignups`、`NewCredentials`、`NewIdentity`、`NewAuthorizationResults`、`NewMutations`、`NewOutputs`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	"collaboration/account/internal/client"
	domain "collaboration/account/internal/domain/account"
	logic "collaboration/account/internal/logic/account"
	repository "collaboration/account/internal/repository/account"
	"collaboration/platform"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"golang.org/x/crypto/bcrypt"
	"net/http"
)

// authentication.go：保留原实现，按业务合并文件。

func NewAuthentication(store *platform.Store, limit *redis.Redis) (*logic.Authentication, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy-not-a-user-credential"), 10)
	if err != nil {
		return nil, err
	}
	// Store拥有池的生命周期；装配只传递原池，业务事务不会跨连接池拆成两部分。
	repo := repository.NewAuthentication(store.Conn, store.DB)
	return logic.NewAuthentication(store.DB, repo, repository.NewLoginLimit(limit), dummy), nil
}

// challenge.go：保留原实现，按业务合并文件。

func NewChallenges(store *platform.Store, limiter *redis.Redis, secrets *domain.Secrets, smtp *platform.SMTPConfig) *logic.Challenges {
	return logic.NewChallenges(repository.NewChallenges(store.DB, limiter), secrets, client.NewSMTP(smtp))
}

func NewSignups(store *platform.Store, challenges *logic.Challenges) *logic.Signups {
	return logic.NewSignups(store.DB, repository.NewSignups(store.DB), challenges)
}

// credential.go：保留原实现，按业务合并文件。

func NewCredentials(store *platform.Store, authentication *logic.Authentication, challenges *logic.Challenges) *logic.Credentials {
	return logic.NewCredentials(store.DB, repository.NewCredentials(store.DB), authentication, challenges)
}

// identity.go：保留原实现，按业务合并文件。

func NewIdentity(store *platform.Store) *logic.Identity {
	return logic.NewIdentity(store.DB, repository.NewIdentity())
}

// 授权证明查询只读Account原库，依赖与原向Doc发出的关闭协调器隔离，防止反向循环。
func NewAuthorizationResults(store *platform.Store) *logic.AuthorizationResults {
	return logic.NewAuthorizationResults(store.DB)
}

// mutation.go：保留原实现，按业务合并文件。

func NewMutations(store *platform.Store) *logic.Mutations {
	return logic.NewMutations(store.DB, repository.NewMutation())
}

// output.go：保留原实现，按业务合并文件。

func NewOutputs(store *platform.Store, httpClient *http.Client, endpoint string) *logic.Outputs {
	return logic.NewOutputs(repository.NewOutputs(store.DB), client.NewAccountOutput(httpClient, endpoint))
}
````

UTF-8/LF全文SHA-256：`3060f065a17ac060bf0c27cb49e372658c5db356b14a18c39e3aa3b93eb4cff0`。[参考实现](../../../../services/account/internal/svc/identity_wiring.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
