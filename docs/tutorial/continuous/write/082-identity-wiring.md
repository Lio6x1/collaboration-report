# 分步编写 `services/account/internal/svc/identity_wiring.go`

先按 [A08 · 输出保护与HTTP、RPC、worker完整装配](../lessons/account/08-output-and-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/082-identity-wiring.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

装配认证、验证码、身份准入、幂等写入和输出保护的实现依赖。

接口由logic消费，构造层把同Store SQL适配器与Redis限流器接起来，业务不持通用Redis命令。

写之前先按这条执行路线理解各段的连接：

1. NewAuthentication启动时生成一次dummy bcrypt hash，再注入repository.Authentication和LoginLimit。
2. NewChallenges连接Secrets/SMTP/Challenges仓库；NewAuthorizationResults只读Account原库，避免向Doc反向循环。

本文件需要守住的边界：

- 不能每次登录临时生成dummyHash；输出registrar必须用可信Document客户端。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/svc' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/svc/identity_wiring.go') -and (Get-Item -LiteralPath 'services/account/internal/svc/identity_wiring.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/svc/identity_wiring.go')) { New-Item -ItemType File -Path 'services/account/internal/svc/identity_wiring.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:522 -->
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

````

## 第 2 步：NewAuthentication

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func NewAuthentication(store *platform.Store, limit *redis.Redis) (*logic.Authentication, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.Conn</code>；<code>store.DB</code> |
| <code>limit</code> | <code>*redis.Redis</code> | 在此声明中的实际使用：<code>repository.NewLoginLimit(limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Authentication</code> | 使用 <code>logic.Authentication</code> 类型；其定义处的职责：把SQL事务、只读/锁定查询和限流能力组合成业务用例，HTTP Cookie留给handler。<br>实际返回表达式：<code>nil</code>；<code>logic.NewAuthentication(store.DB, repo, repository.NewLoginLimit(limit), dummy)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewAuthentication(store.DB, repo, repository.NewLoginLimit(limit), dummy), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=522:1072 -->
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

// challenge.go：保留原实现，按业务合并文件。

````

## 第 3 步：NewChallenges

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func NewChallenges(store *platform.Store, limiter *redis.Redis, secrets *domain.Secrets, smtp *platform.SMTPConfig) *logic.Challenges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>limiter</code> | <code>*redis.Redis</code> | 在此声明中的实际使用：<code>repository.NewChallenges(store.DB, limiter)</code> |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。<br>在此声明中的实际使用：<code>logic.NewChallenges(repository.NewChallenges(store.DB, limiter), secrets, client.NewSMTP(smtp))</code> |
| <code>smtp</code> | <code>*platform.SMTPConfig</code> | 使用 <code>platform.SMTPConfig</code> 类型；其定义处的职责：加载服务自身配置，创建有界数据库池，并以迁移和恢复事实判定就绪。<br>在此声明中的实际使用：<code>client.NewSMTP(smtp)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Challenges</code> | 使用 <code>logic.Challenges</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。<br>实际返回表达式：<code>logic.NewChallenges(repository.NewChallenges(store.DB, limiter), secrets, client.NewSMTP(smtp))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewChallenges(repository.NewChallenges(store.DB, limiter), secrets, client.NewSMTP(smtp))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1072:1315 -->
````go
func NewChallenges(store *platform.Store, limiter *redis.Redis, secrets *domain.Secrets, smtp *platform.SMTPConfig) *logic.Challenges {
	return logic.NewChallenges(repository.NewChallenges(store.DB, limiter), secrets, client.NewSMTP(smtp))
}

````

## 第 4 步：NewSignups

插入位置：第 3 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`func NewSignups(store *platform.Store, challenges *logic.Challenges) *logic.Signups`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>challenges</code> | <code>*logic.Challenges</code> | 使用 <code>logic.Challenges</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。<br>在此声明中的实际使用：<code>logic.NewSignups(store.DB, repository.NewSignups(store.DB), challenges)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Signups</code> | 使用 <code>logic.Signups</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。<br>实际返回表达式：<code>logic.NewSignups(store.DB, repository.NewSignups(store.DB), challenges)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewSignups(store.DB, repository.NewSignups(store.DB), challenges)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1315:1547 -->
````go
func NewSignups(store *platform.Store, challenges *logic.Challenges) *logic.Signups {
	return logic.NewSignups(store.DB, repository.NewSignups(store.DB), challenges)
}

// credential.go：保留原实现，按业务合并文件。

````

## 第 5 步：NewCredentials

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func NewCredentials(store *platform.Store, authentication *logic.Authentication, challenges *logic.Challenges) *logic.Credentials`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>authentication</code> | <code>*logic.Authentication</code> | 使用 <code>logic.Authentication</code> 类型；其定义处的职责：把SQL事务、只读/锁定查询和限流能力组合成业务用例，HTTP Cookie留给handler。<br>在此声明中的实际使用：<code>logic.NewCredentials(store.DB, repository.NewCredentials(store.DB), authentication, challenges)</code> |
| <code>challenges</code> | <code>*logic.Challenges</code> | 使用 <code>logic.Challenges</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。<br>在此声明中的实际使用：<code>logic.NewCredentials(store.DB, repository.NewCredentials(store.DB), authentication, challenges)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Credentials</code> | 使用 <code>logic.Credentials</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。<br>实际返回表达式：<code>logic.NewCredentials(store.DB, repository.NewCredentials(store.DB), authentication, challenges)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewCredentials(store.DB, repository.NewCredentials(store.DB), authentication, challenges)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1547:1847 -->
````go
func NewCredentials(store *platform.Store, authentication *logic.Authentication, challenges *logic.Challenges) *logic.Credentials {
	return logic.NewCredentials(store.DB, repository.NewCredentials(store.DB), authentication, challenges)
}

// identity.go：保留原实现，按业务合并文件。

````

## 第 6 步：NewIdentity

插入位置：第 5 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func NewIdentity(store *platform.Store) *logic.Identity`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>实际返回表达式：<code>logic.NewIdentity(store.DB, repository.NewIdentity())</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewIdentity(store.DB, repository.NewIdentity())</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1847:1970 -->
````go
func NewIdentity(store *platform.Store) *logic.Identity {
	return logic.NewIdentity(store.DB, repository.NewIdentity())
}

````

## 第 7 步：NewAuthorizationResults

插入位置：第 6 步代码之后，紧接追加；完成稿约第 49 行。

授权证明查询只读Account原库，依赖与原向Doc发出的关闭协调器隔离，防止反向循环。

本步声明：`func NewAuthorizationResults(store *platform.Store) *logic.AuthorizationResults`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.AuthorizationResults</code> | 使用 <code>logic.AuthorizationResults</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>实际返回表达式：<code>logic.NewAuthorizationResults(store.DB)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewAuthorizationResults(store.DB)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1970:2280 -->
````go
// 授权证明查询只读Account原库，依赖与原向Doc发出的关闭协调器隔离，防止反向循环。
func NewAuthorizationResults(store *platform.Store) *logic.AuthorizationResults {
	return logic.NewAuthorizationResults(store.DB)
}

// mutation.go：保留原实现，按业务合并文件。

````

## 第 8 步：NewMutations

插入位置：第 7 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`func NewMutations(store *platform.Store) *logic.Mutations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Mutations</code> | 使用 <code>logic.Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>实际返回表达式：<code>logic.NewMutations(store.DB, repository.NewMutation())</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewMutations(store.DB, repository.NewMutation())</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2280:2465 -->
````go
func NewMutations(store *platform.Store) *logic.Mutations {
	return logic.NewMutations(store.DB, repository.NewMutation())
}

// output.go：保留原实现，按业务合并文件。

````

## 第 9 步：NewOutputs

插入位置：第 8 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func NewOutputs(store *platform.Store, httpClient *http.Client, endpoint string) *logic.Outputs`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>client.NewAccountOutput(httpClient, endpoint)</code> |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>client.NewAccountOutput(httpClient, endpoint)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：核对响应捕获的全部来源，再到Doc注册该入口handler的输出。<br>实际返回表达式：<code>logic.NewOutputs(repository.NewOutputs(store.DB), client.NewAccountOutput(httpClient, endpoint))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewOutputs(repository.NewOutputs(store.DB), client.NewAccountOutput(httpClient, endpoint))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2465:2670 -->
````go
func NewOutputs(store *platform.Store, httpClient *http.Client, endpoint string) *logic.Outputs {
	return logic.NewOutputs(repository.NewOutputs(store.DB), client.NewAccountOutput(httpClient, endpoint))
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/svc/identity_wiring.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/08-output-and-wiring.md)。
