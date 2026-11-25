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
