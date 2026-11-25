# 分步编写 `services/account/internal/logic/account/account_close_preparation_service.go`

先按 [A06 · 账号注销准备、职责交接与来源冻结](../lessons/account/06-account-close-preparation.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/058-account-close-preparation-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

执行注销密码确认、职责交接检查、下游范围固定和来源冻结。

用户请求可能在Doc预留后超时，原号和固定组织快照是恢复依据；不能再次扩充删除范围。

写之前先按这条执行路线理解各段的连接：

1. Start在原操作事务内记录自动恢复意图；锁外bcrypt后在user锁内复核原hash与epoch。
2. PrepareClose先保存Account组织事实，再锁外调用Doc准备，回到事务复核同一快照。
3. 需要移交职责时保存succession要求；Doc确认同一计划FROZEN后才调用Freeze。

本文件需要守住的边界：

- Begin仅内部准备，不等同用户授权全流程；公开入口使用Start。
- BindingForDocument只读已经持久固定的原范围，不能拿当前身份/组织图补造。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/account_close_preparation_service.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/account_close_preparation_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/account_close_preparation_service.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/account_close_preparation_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:573 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"slices"
	"strconv"
	"time"
)

// account_close.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountClosures

插入位置：第 1 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`type AccountClosures struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.AccountClosures</code> | 使用 <code>repository.AccountClosures</code> 类型；其定义处的职责：把注销准备、固定组织快照、来源冻结和各来源Fence子任务原子持久化。 |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：把SQL事务、只读/锁定查询和限流能力组合成业务用例，HTTP Cookie留给handler。 |

<!-- write-step: 2 bytes=573:706 -->
````go
type AccountClosures struct {
	db             *sql.DB
	repository     *repository.AccountClosures
	authentication *Authentication
}

````

## 第 3 步：NewAccountClosures

插入位置：第 2 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func NewAccountClosures(db *sql.DB, authentication *Authentication) *AccountClosures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code>；<code>repository.NewAccountClosures(db)</code> |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：把SQL事务、只读/锁定查询和限流能力组合成业务用例，HTTP Cookie留给handler。<br>在此声明中的实际使用：<code>authentication: authentication</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountClosures</code> | 实际返回表达式：<code>&amp;AccountClosures{db: db, repository: repository.NewAccountClosures(db), authentication: authentication}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountClosures{db: db, repository: repository.NewAccountClosures(db), authentication: authentication}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=706:908 -->
````go
func NewAccountClosures(db *sql.DB, authentication *Authentication) *AccountClosures {
	return &AccountClosures{db: db, repository: repository.NewAccountClosures(db), authentication: authentication}
}

````

## 第 4 步：accountCloseConflict

插入位置：第 3 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func accountCloseConflict(code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>Code: code</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.StateConflict, Code: code}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.RuleViolation{Kind: domain.StateConflict, Code: code}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=908:1027 -->
````go
func accountCloseConflict(code string) error {
	return domain.RuleViolation{Kind: domain.StateConflict, Code: code}
}

````

## 第 5 步：AccountClosures.Begin

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func (s *AccountClosures) Begin(ctx context.Context, session domain.Session, input dto.AccountClose, fingerprint []byte) (dto.AccountClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.beginClose(ctx, session, input, fingerprint, false)</code> |
| <code>session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>s.beginClose(ctx, session, input, fingerprint, false)</code> |
| <code>input</code> | <code>dto.AccountClose</code> | 在此声明中的实际使用：<code>s.beginClose(ctx, session, input, fingerprint, false)</code> |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>s.beginClose(ctx, session, input, fingerprint, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.AccountClosePlan</code> | 由 <code>s.beginClose(ctx, session, input, fingerprint, false)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.beginClose(ctx, session, input, fingerprint, false)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.beginClose(ctx, session, input, fingerprint, false)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1027:1245 -->
````go
func (s *AccountClosures) Begin(ctx context.Context, session domain.Session, input dto.AccountClose, fingerprint []byte) (dto.AccountClosePlan, error) {
	return s.beginClose(ctx, session, input, fingerprint, false)
}

````

## 第 6 步：AccountClosures.Start

插入位置：第 5 步代码之后，紧接追加；完成稿约第 43 行。

Start是完整用户用例；原操作与自动恢复意图同事务写入。内部Begin仍用于
已有控制协议阶段验证，不将内部准备调用误当成用户已发起整条不可逆流程。

本步声明：`func (s *AccountClosures) Start(ctx context.Context, session domain.Session, input dto.AccountClose, fingerprint []byte) (dto.AccountClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.beginClose(ctx, session, input, fingerprint, true)</code> |
| <code>session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>s.beginClose(ctx, session, input, fingerprint, true)</code> |
| <code>input</code> | <code>dto.AccountClose</code> | 在此声明中的实际使用：<code>s.beginClose(ctx, session, input, fingerprint, true)</code> |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>s.beginClose(ctx, session, input, fingerprint, true)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.AccountClosePlan</code> | 由 <code>s.beginClose(ctx, session, input, fingerprint, true)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.beginClose(ctx, session, input, fingerprint, true)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.beginClose(ctx, session, input, fingerprint, true)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1245:1672 -->
````go
// Start是完整用户用例；原操作与自动恢复意图同事务写入。内部Begin仍用于
// 已有控制协议阶段验证，不将内部准备调用误当成用户已发起整条不可逆流程。
func (s *AccountClosures) Start(ctx context.Context, session domain.Session, input dto.AccountClose, fingerprint []byte) (dto.AccountClosePlan, error) {
	return s.beginClose(ctx, session, input, fingerprint, true)
}

````

## 第 7 步：AccountClosures.beginClose

插入位置：第 6 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func (s *AccountClosures) beginClose(ctx context.Context, session domain.Session, input dto.AccountClose, fingerprint []byte, automatic bool) (dto.AccountClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, nil, automatic)</code>；<code>s.repository.Hash(ctx, user)</code>；<code>s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, hash, automatic)</code> |
| <code>session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>session.UserID</code>；<code>session.ID</code>；<code>s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, nil, automatic)</code>；<code>s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, hash, automatic)</code> |
| <code>input</code> | <code>dto.AccountClose</code> | 在此声明中的实际使用：<code>input.ExpectedIdentityEpoch</code>；<code>input.OperationId</code>；<code>input.CurrentPassword</code> |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(fingerprint)</code>；<code>s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, nil, automatic)</code>；<code>s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, hash, automatic)</code> |
| <code>automatic</code> | <code>bool</code> | 在此声明中的实际使用：<code>s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, nil, automatic)</code>；<code>s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, hash, automatic)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.AccountClosePlan</code> | 实际返回表达式：<code>dto.AccountClosePlan{}</code>；<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;ACCOUNT_CLOSE_INPUT_INVALID&quot;}</code>；<code>err</code>；<code>domain.RuleViolation{Kind: domain.PermissionDenied, Code: &quot;CURRENT_PASSWORD_INVALID&quot;}</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; user == 0 &#124;&#124; strconv.FormatUint(user, 10) != session.UserID &#124;&#124; !valid &#124;&#124; !domain.ValidOperationID(input.OperationId) &#124;&#124; len(session.ID) != 32 &#124;&#124; len(fingerprint) != 32</code>。条件成立时执行 <code>return dto.AccountClosePlan{}, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;ACCOUNT_CLOSE_INPUT_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>bcrypt.CompareHashAndPassword(hash, &#91;&#93;byte(input.CurrentPassword)) != nil</code>。条件成立时执行 <code>return plan, domain.RuleViolation{Kind: domain.PermissionDenied, Code: &quot;CURRENT_PASSWORD_INVALID&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return plan, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1672:3155 -->
````go
func (s *AccountClosures) beginClose(ctx context.Context, session domain.Session, input dto.AccountClose, fingerprint []byte, automatic bool) (dto.AccountClosePlan, error) {
	user, err := strconv.ParseUint(session.UserID, 10, 64)
	epoch, valid := domain.CredentialEpoch(input.ExpectedIdentityEpoch)
	if err != nil || user == 0 || strconv.FormatUint(user, 10) != session.UserID || !valid || !domain.ValidOperationID(input.OperationId) || len(session.ID) != 32 || len(fingerprint) != 32 {
		return dto.AccountClosePlan{}, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "ACCOUNT_CLOSE_INPUT_INVALID"}
	}
	// 原会话只允许读取完全相同的原操作。先查回执再验密码，身份进入REVOKING后
	// 仍能恢复原请求；另一个操作编号必须重新通过当前凭据与会话资格检查。
	plan, found, err := s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, nil, automatic)
	if err != nil || found {
		return plan, err
	}
	hash, err := s.repository.Hash(ctx, user)
	if err != nil {
		return plan, err
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(input.CurrentPassword)) != nil {
		return plan, domain.RuleViolation{Kind: domain.PermissionDenied, Code: "CURRENT_PASSWORD_INVALID"}
	}
	// bcrypt在事务外运行；取得user锁之后比较原hash及两个代次，防止旧密码验证穿越凭据变更。
	plan, _, err = s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, hash, automatic)
	return plan, err
}

````

## 第 8 步：AccountClosures.begin

插入位置：第 7 步代码之后，紧接追加；完成稿约第 73 行。

本步声明：`func (s *AccountClosures) begin(ctx context.Context, user, epoch uint64, session domain.Session, operation string, fingerprint, verifiedHash []byte, automatic bool) (dto.AccountClosePlan, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.repository.LockUser(ctx, tx, user)</code>；<code>s.repository.Receipt(ctx, tx, operation)</code>；<code>s.repository.Record(ctx, tx, operation)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.repository.LockUser(ctx, tx, user)</code>；<code>receipt.User != user</code>；<code>record.User != user</code>；<code>capture.Binding.User != user</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>capture.IdentityEpoch != epoch</code>；<code>credential.IdentityEpoch != epoch</code>；<code>IdentityEpoch: epoch</code> |
| <code>session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>session.ID</code>；<code>session.CredentialEpoch</code>；<code>session.UserID</code>；<code>session.Epoch</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.Receipt(ctx, tx, operation)</code>；<code>s.repository.Record(ctx, tx, operation)</code>；<code>capture.Binding.Operation != operation</code>；<code>accountCloseSavedPlan(operation, record, receipt)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.Equal(receipt.Fingerprint, fingerprint)</code>；<code>s.repository.Prepare(ctx, tx, user, operation, rawCapture, target, fingerprint, rawPlan)</code> |
| <code>verifiedHash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>verifiedHash == nil</code>；<code>bytes.Equal(credential.Hash, verifiedHash)</code> |
| <code>automatic</code> | <code>bool</code> | 在此声明中的实际使用：<code>err == nil &amp;&amp; automatic</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.AccountClosePlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>accountCloseConflict(&quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_CONFLICT&quot;)</code>；<code>nil</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, false, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, false, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>receipt.User != user &#124;&#124; receipt.Kind != &quot;ACCOUNT_CLOSE&quot; &#124;&#124; receipt.SessionHash != hex.EncodeToString(session.ID) &#124;&#124; !bytes.Equal(receipt.Fingerprint, fingerprint)</code>。条件成立时执行 <code>return plan, true, accountCloseConflict(&quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, true, err</code>。当前函数在这里结束。
- 检查 <code>record.User != user &#124;&#124; capture.SessionHash != receipt.SessionHash &#124;&#124; capture.IdentityEpoch != epoch &#124;&#124; strconv.FormatUint(capture.CredentialEpoch, 10) != session.CredentialEpoch</code>。条件成立时执行 <code>return plan, true, accountCloseConflict(&quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>capture.Binding != nil &amp;&amp; (capture.Binding.User != user &#124;&#124; capture.Binding.Operation != operation)</code>。条件成立时执行 <code>return plan, true, accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; automatic</code>。
- 检查 <code>err == nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return plan, true, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return plan, false, err</code>。当前函数在这里结束。
- 检查 <code>verifiedHash == nil</code>。条件成立时执行 <code>return plan, false, nil</code>。当前函数在这里结束。
- 检查 <code>credential.State != &quot;ACTIVE&quot; &#124;&#124; credential.IdentityEpoch != epoch &#124;&#124; credential.Epoch != session.CredentialEpoch &#124;&#124; !bytes.Equal(credential.Hash, verifiedHash)</code>。条件成立时执行 <code>return plan, false, accountCloseConflict(&quot;ACCOUNT_CLOSE_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=3155:5105 -->
````go
func (s *AccountClosures) begin(ctx context.Context, user, epoch uint64, session domain.Session, operation string, fingerprint, verifiedHash []byte, automatic bool) (dto.AccountClosePlan, bool, error) {
	var plan dto.AccountClosePlan
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return plan, false, err
	}
	defer tx.Rollback()
	credential, err := s.repository.LockUser(ctx, tx, user)
	if err != nil {
		return plan, false, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, operation)
	if err == nil {
		if receipt.User != user || receipt.Kind != "ACCOUNT_CLOSE" || receipt.SessionHash != hex.EncodeToString(session.ID) || !bytes.Equal(receipt.Fingerprint, fingerprint) {
			return plan, true, accountCloseConflict("OPERATION_ID_CONFLICT")
		}
		record, err := s.repository.Record(ctx, tx, operation)
		if err != nil {
			return plan, true, err
		}
		capture, err := domain.DecodeAccountCloseCapture(record.Capture)
		if err != nil {
			return plan, true, err
		}
		if record.User != user || capture.SessionHash != receipt.SessionHash || capture.IdentityEpoch != epoch || strconv.FormatUint(capture.CredentialEpoch, 10) != session.CredentialEpoch {
			return plan, true, accountCloseConflict("OPERATION_ID_CONFLICT")
		}
		if capture.Binding != nil && (capture.Binding.User != user || capture.Binding.Operation != operation) {
			return plan, true, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
		}
		plan, err = accountCloseSavedPlan(operation, record, receipt)
		if err == nil && automatic {
			if err = repository.QueueAccountCloseRecovery(ctx, tx, operation, user); err == nil {
				err = tx.Commit()
			}
		}
		return plan, true, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return plan, false, err
	}
	if verifiedHash == nil {
		return plan, false, nil
	}
	if credential.State != "ACTIVE" || credential.IdentityEpoch != epoch || credential.Epoch != session.CredentialEpoch || !bytes.Equal(credential.Hash, verifiedHash) {
````

## 第 9 步：AccountClosures.begin（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 118 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return plan, false, accountCloseConflict(&quot;ACCOUNT_CLOSE_IDENTITY_CHANGED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, false, err</code>。当前函数在这里结束。
- 检查 <code>current.UserID != session.UserID &#124;&#124; current.State != &quot;ACTIVE&quot; &#124;&#124; current.CredentialEpoch != credential.Epoch &#124;&#124; current.Epoch != session.Epoch &#124;&#124; !current.Expires.After(time.Now())</code>。条件成立时执行 <code>return plan, false, domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, false, err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return plan, false, accountCloseConflict(&quot;ACCOUNT_CLOSE_IN_PROGRESS&quot;)</code>。当前函数在这里结束。
- 检查 <code>!valid</code>。条件成立时执行 <code>return plan, false, accountCloseConflict(&quot;ACCOUNT_CLOSE_CREDENTIAL_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, false, err</code>。当前函数在这里结束。
- 检查 <code>automatic</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, false, err</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=5105:6896 -->
````go
		return plan, false, accountCloseConflict("ACCOUNT_CLOSE_IDENTITY_CHANGED")
	}
	current, err := s.repository.LockSession(ctx, tx, session.ID)
	if err != nil {
		return plan, false, err
	}
	if current.UserID != session.UserID || current.State != "ACTIVE" || current.CredentialEpoch != credential.Epoch || current.Epoch != session.Epoch || !current.Expires.After(time.Now()) {
		return plan, false, domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	pending, err := s.repository.HasPending(ctx, tx, user)
	if err != nil {
		return plan, false, err
	}
	if pending {
		return plan, false, accountCloseConflict("ACCOUNT_CLOSE_IN_PROGRESS")
	}
	credentialEpoch, valid := domain.CredentialEpoch(credential.Epoch)
	if !valid {
		return plan, false, accountCloseConflict("ACCOUNT_CLOSE_CREDENTIAL_INVALID")
	}
	capture := domain.AccountCloseCapture{SessionHash: hex.EncodeToString(session.ID), CredentialEpoch: credentialEpoch, IdentityEpoch: epoch}
	plan = dto.AccountClosePlan{OperationId: operation, State: "PREPARING", RetainedReleaseIds: []string{}, RequiredSuccessions: []struct {
		Kind     string `json:"kind"`
		TargetId string `json:"target_id"`
	}{}}
	rawCapture, err := json.Marshal(capture)
	if err != nil {
		return plan, false, err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return plan, false, err
	}
	target, err := json.Marshal(map[string]string{"user_id": session.UserID, "session_hash": capture.SessionHash})
	if err != nil {
		return plan, false, err
	}
	if err = s.repository.Prepare(ctx, tx, user, operation, rawCapture, target, fingerprint, rawPlan); err != nil {
		return plan, false, err
	}
	if automatic {
		if err = repository.QueueAccountCloseRecovery(ctx, tx, operation, user); err != nil {
			return plan, false, err
		}
	}
````

## 第 10 步：AccountClosures.begin（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 163 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, false, errors.New(&quot;ACCOUNT_CLOSE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return plan, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6896:7027 -->
````go
	if err = tx.Commit(); err != nil {
		return plan, false, errors.New("ACCOUNT_CLOSE_COMMIT_UNKNOWN")
	}
	return plan, true, nil
}

````

## 第 11 步：accountCloseSavedPlan

插入位置：第 10 步代码之后，紧接追加；完成稿约第 169 行。

本步声明：`func accountCloseSavedPlan(operation string, record domain.AccountCloseRecord, receipt domain.AccountCloseReceipt) (dto.AccountClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>plan.OperationId != operation</code> |
| <code>record</code> | <code>domain.AccountCloseRecord</code> | 使用 <code>domain.AccountCloseRecord</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>record.User</code>；<code>record.State</code> |
| <code>receipt</code> | <code>domain.AccountCloseReceipt</code> | 使用 <code>domain.AccountCloseReceipt</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>receipt.Kind</code>；<code>receipt.User</code>；<code>receipt.Result</code>；<code>receipt.State</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.AccountClosePlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_CLOSE_RECEIPT_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>receipt.Kind != &quot;ACCOUNT_CLOSE&quot; &#124;&#124; receipt.User != record.User &#124;&#124; json.Unmarshal(receipt.Result, &amp;plan) != nil &#124;&#124; plan.OperationId != operation &#124;&#124; plan.State != record.State &#124;&#124; pla …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return plan, errors.New(&quot;ACCOUNT_CLOSE_RECEIPT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>record.State == &quot;SUCCEEDED&quot; &#124;&#124; record.State == &quot;REJECTED&quot;</code>。
- 检查 <code>receipt.State != expected</code>。条件成立时执行 <code>return plan, errors.New(&quot;ACCOUNT_CLOSE_RECEIPT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=7027:7749 -->
````go
func accountCloseSavedPlan(operation string, record domain.AccountCloseRecord, receipt domain.AccountCloseReceipt) (dto.AccountClosePlan, error) {
	var plan dto.AccountClosePlan
	if receipt.Kind != "ACCOUNT_CLOSE" || receipt.User != record.User || json.Unmarshal(receipt.Result, &plan) != nil || plan.OperationId != operation || plan.State != record.State || plan.RequiredSuccessions == nil || plan.RetainedReleaseIds == nil {
		return plan, errors.New("ACCOUNT_CLOSE_RECEIPT_INVALID")
	}
	expected := "PREPARING"
	if record.State == "SUCCEEDED" || record.State == "REJECTED" {
		expected = record.State
	}
	if receipt.State != expected {
		return plan, errors.New("ACCOUNT_CLOSE_RECEIPT_INVALID")
	}
	return plan, nil
}

````

## 第 12 步：AccountClosures.lockBinding

插入位置：第 11 步代码之后，紧接追加；完成稿约第 184 行。

lockBinding沿用user→receipt→account_close顺序。原关闭组织资格由编排器在固定下游
计划前核验，本层仅维护原用户/身份代次以及绑定不可替换的持久约束。

本步声明：`func (s *AccountClosures) lockBinding(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding) (domain.AccountCloseRecord, domain.AccountCloseCapture, dto.AccountClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.LockUser(ctx, tx, binding.User)</code>；<code>s.repository.Receipt(ctx, tx, binding.Operation)</code>；<code>s.repository.Record(ctx, tx, binding.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.repository.LockUser(ctx, tx, binding.User)</code>；<code>s.repository.Receipt(ctx, tx, binding.Operation)</code>；<code>s.repository.Record(ctx, tx, binding.Operation)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>platform.ValidateAccountClosureBinding(binding)</code>；<code>binding.User</code>；<code>binding.Operation</code>；<code>binding.IdentityEpoch</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseRecord</code> | 使用 <code>domain.AccountCloseRecord</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>record</code> |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseCapture</code> | 使用 <code>domain.AccountCloseCapture</code> 类型；其定义处的职责：AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。 Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。<br>实际返回表达式：<code>capture</code> |
| <code>(匿名/未命名)</code> | <code>dto.AccountClosePlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_CONFLICT&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.ValidateAccountClosureBinding(binding)</code>。条件成立时执行 <code>return record, capture, plan, accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return record, capture, plan, err</code>。当前函数在这里结束。
- 检查 <code>record.User != binding.User &#124;&#124; receipt.User != binding.User &#124;&#124; receipt.SessionHash != capture.SessionHash &#124;&#124; capture.IdentityEpoch != binding.IdentityEpoch</code>。条件成立时执行 <code>return record, capture, plan, accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>capture.Binding != nil &amp;&amp; !platform.EqualAccountClosureBinding(*capture.Binding, binding)</code>。条件成立时执行 <code>return record, capture, plan, accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return record, capture, plan, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=7749:9430 -->
````go
// lockBinding沿用user→receipt→account_close顺序。原关闭组织资格由编排器在固定下游
// 计划前核验，本层仅维护原用户/身份代次以及绑定不可替换的持久约束。
func (s *AccountClosures) lockBinding(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding) (domain.AccountCloseRecord, domain.AccountCloseCapture, dto.AccountClosePlan, error) {
	var record domain.AccountCloseRecord
	var capture domain.AccountCloseCapture
	var plan dto.AccountClosePlan
	if !platform.ValidateAccountClosureBinding(binding) {
		return record, capture, plan, accountCloseConflict("ACCOUNT_CLOSE_BINDING_INVALID")
	}
	if _, err := s.repository.LockUser(ctx, tx, binding.User); err != nil {
		return record, capture, plan, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, binding.Operation)
	if err != nil {
		return record, capture, plan, err
	}
	record, err = s.repository.Record(ctx, tx, binding.Operation)
	if err != nil {
		return record, capture, plan, err
	}
	capture, err = domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return record, capture, plan, err
	}
	if record.User != binding.User || receipt.User != binding.User || receipt.SessionHash != capture.SessionHash || capture.IdentityEpoch != binding.IdentityEpoch {
		return record, capture, plan, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
	}
	if capture.Binding != nil && !platform.EqualAccountClosureBinding(*capture.Binding, binding) {
		return record, capture, plan, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
	}
	plan, err = accountCloseSavedPlan(binding.Operation, record, receipt)
	return record, capture, plan, err
}

````

## 第 13 步：AccountClosures.Bind

插入位置：第 12 步代码之后，紧接追加；完成稿约第 218 行。

本步声明：`func (s *AccountClosures) Bind(ctx context.Context, binding platform.AccountClosureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.lockBinding(ctx, tx, binding)</code>；<code>s.repository.Bind(ctx, tx, binding.Operation, raw)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>s.lockBinding(ctx, tx, binding)</code>；<code>&amp;binding</code>；<code>binding.Operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>accountCloseConflict(&quot;ACCOUNT_CLOSE_NOT_PREPARING&quot;)</code>；<code>nil</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_BIND_UNKNOWN&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>record.State != &quot;PREPARING&quot;</code>。条件成立时执行 <code>return accountCloseConflict(&quot;ACCOUNT_CLOSE_NOT_PREPARING&quot;)</code>。当前函数在这里结束。
- 检查 <code>capture.Binding != nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_BIND_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=9430:10146 -->
````go
func (s *AccountClosures) Bind(ctx context.Context, binding platform.AccountClosureBinding) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	record, capture, _, err := s.lockBinding(ctx, tx, binding)
	if err != nil {
		return err
	}
	if record.State != "PREPARING" {
		return accountCloseConflict("ACCOUNT_CLOSE_NOT_PREPARING")
	}
	if capture.Binding != nil {
		return nil
	}
	capture.Binding = &binding
	raw, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	if err = s.repository.Bind(ctx, tx, binding.Operation, raw); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_CLOSE_BIND_UNKNOWN")
	}
	return nil
}

````

## 第 14 步：AccountClosures.Reject

插入位置：第 13 步代码之后，紧接追加；完成稿约第 248 行。

本步声明：`func (s *AccountClosures) Reject(ctx context.Context, binding platform.AccountClosureBinding, reason string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.lockBinding(ctx, tx, binding)</code>；<code>s.repository.Reject(ctx, tx, binding.Operation, reason, raw)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>s.lockBinding(ctx, tx, binding)</code>；<code>binding.Operation</code> |
| <code>reason</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidIdentityPurpose(reason)</code>；<code>s.repository.Reject(ctx, tx, binding.Operation, reason, raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;ACCOUNT_CLOSE_REASON_INVALID&quot;}</code>；<code>err</code>；<code>accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_REQUIRED&quot;)</code>；<code>nil</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidIdentityPurpose(reason)</code>。条件成立时执行 <code>return domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;ACCOUNT_CLOSE_REASON_INVALID&quot;}</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>capture.Binding == nil</code>。条件成立时执行 <code>return accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>record.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>record.State != &quot;PREPARING&quot;</code>。条件成立时执行 <code>return accountCloseConflict(&quot;ACCOUNT_CLOSE_NOT_PREPARING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_REJECTION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=10146:11238 -->
````go
func (s *AccountClosures) Reject(ctx context.Context, binding platform.AccountClosureBinding, reason string) error {
	if !domain.ValidIdentityPurpose(reason) {
		return domain.RuleViolation{Kind: domain.InvalidRequest, Code: "ACCOUNT_CLOSE_REASON_INVALID"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	record, capture, plan, err := s.lockBinding(ctx, tx, binding)
	if err != nil {
		return err
	}
	if capture.Binding == nil {
		return accountCloseConflict("ACCOUNT_CLOSE_BINDING_REQUIRED")
	}
	if record.State == "REJECTED" {
		return nil
	}
	// FROZEN及之后阶段可能已产生不可逆事实，不能用准备期拒绝释放下游预留。
	if record.State != "PREPARING" {
		return accountCloseConflict("ACCOUNT_CLOSE_NOT_PREPARING")
	}
	plan.State = "REJECTED"
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = s.repository.Reject(ctx, tx, binding.Operation, reason, raw); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_CLOSE_REJECTION_UNKNOWN")
	}
	return nil
}

````

## 第 15 步：AccountClosures.Rejected

插入位置：第 14 步代码之后，紧接追加；完成稿约第 285 行。

本步声明：`func (s *AccountClosures) Rejected(ctx context.Context, binding platform.AccountClosureBinding) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.lockBinding(ctx, tx, binding)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>s.lockBinding(ctx, tx, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>record.State == &quot;REJECTED&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>capture.Binding == nil</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return record.State == &quot;REJECTED&quot;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=11238:11875 -->
````go
func (s *AccountClosures) Rejected(ctx context.Context, binding platform.AccountClosureBinding) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	record, capture, _, err := s.lockBinding(ctx, tx, binding)
	// 不存在只表示没有拒绝证明；查询失败、超时及失配不能推断为已拒绝。
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if capture.Binding == nil {
		return false, nil
	}
	return record.State == "REJECTED", nil
}

// account_close_binding.go：保留原实现，按业务合并文件。

````

## 第 16 步：AccountClosures.BindingForDocument

插入位置：第 15 步代码之后，紧接追加；完成稿约第 307 行。

BindingForDocument只交付原操作已经持久固定的关闭范围；不能用当前身份epoch或
当前组织成员图补造绑定。它不授权注销成功，也不创建/推进/取消任何操作。

本步声明：`func (s *AccountClosures) BindingForDocument(ctx context.Context, operation string) (platform.AccountClosureBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.UserForOperation(ctx, operation)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.repository.LockUser(ctx, tx, user)</code>；<code>s.repository.Receipt(ctx, tx, operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidOperationID(operation)</code>；<code>s.repository.UserForOperation(ctx, operation)</code>；<code>s.repository.Receipt(ctx, tx, operation)</code>；<code>s.repository.Record(ctx, tx, operation)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>实际返回表达式：<code>empty</code>；<code>binding</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;OPERATION_ID_INVALID&quot;}</code>；<code>accountCloseNotFound()</code>；<code>err</code>；<code>accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_REQUIRED&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(operation)</code>。条件成立时执行 <code>return empty, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;OPERATION_ID_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return empty, accountCloseNotFound()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>capture.Binding == nil</code>。条件成立时执行 <code>return empty, accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>binding.Operation != operation &#124;&#124; binding.User != user &#124;&#124; record.User != user &#124;&#124; receipt.User != user &#124;&#124; 		receipt.SessionHash != capture.SessionHash &#124;&#124; binding.IdentityEpoch != capture.IdentityEpoch</code>。条件成立时执行 <code>return empty, accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_CONFLICT&quot;)</code>。当前函数在这里结束。

<!-- write-step: 16 bytes=11875:13616 -->
````go
// BindingForDocument只交付原操作已经持久固定的关闭范围；不能用当前身份epoch或
// 当前组织成员图补造绑定。它不授权注销成功，也不创建/推进/取消任何操作。
func (s *AccountClosures) BindingForDocument(ctx context.Context, operation string) (platform.AccountClosureBinding, error) {
	var empty platform.AccountClosureBinding
	if !domain.ValidOperationID(operation) {
		return empty, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "OPERATION_ID_INVALID"}
	}
	user, err := s.repository.UserForOperation(ctx, operation)
	if errors.Is(err, sql.ErrNoRows) {
		return empty, accountCloseNotFound()
	}
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	if _, err = s.repository.LockUser(ctx, tx, user); err != nil {
		return empty, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, operation)
	if err != nil {
		return empty, err
	}
	record, err := s.repository.Record(ctx, tx, operation)
	if err != nil {
		return empty, err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return empty, err
	}
	if _, err = accountCloseSavedPlan(operation, record, receipt); err != nil {
		return empty, err
	}
	if capture.Binding == nil {
		return empty, accountCloseConflict("ACCOUNT_CLOSE_BINDING_REQUIRED")
	}
	binding := *capture.Binding
	if binding.Operation != operation || binding.User != user || record.User != user || receipt.User != user ||
		receipt.SessionHash != capture.SessionHash || binding.IdentityEpoch != capture.IdentityEpoch {
		return empty, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
	}
````

## 第 17 步：AccountClosures.BindingForDocument（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 352 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>record.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return empty, accountCloseConflict(&quot;ACCOUNT_CLOSE_REJECTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_CLOSE_BINDING_READ_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return binding, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=13616:13915 -->
````go
	if record.State == "REJECTED" {
		return empty, accountCloseConflict("ACCOUNT_CLOSE_REJECTED")
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_CLOSE_BINDING_READ_UNKNOWN")
	}
	return binding, nil
}

// account_close_fence.go：保留原实现，按业务合并文件。

````

## 第 18 步：AccountCloseFencePeer

插入位置：第 17 步代码之后，紧接追加；完成稿约第 363 行。

本步声明：`type AccountCloseFencePeer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>FenceAccess</code> | <code>(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)</code> | 接收ctx及固定来源关闭请求，返回真实参与者关闭结果；超时/未知不能等同输出已停止。 |

<!-- write-step: 18 bytes=13915:14030 -->
````go
type AccountCloseFencePeer interface {
	FenceAccess(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)
}

````

## 第 19 步：AccountCloseFences

插入位置：第 18 步代码之后，紧接追加；完成稿约第 367 行。

本步声明：`type AccountCloseFences struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.AccountCloseFences</code> | 使用 <code>repository.AccountCloseFences</code> 类型；其定义处的职责：把注销准备、固定组织快照、来源冻结和各来源Fence子任务原子持久化。 |
| <code>peer</code> | <code>AccountCloseFencePeer</code> | 负责让Doc及其下游停止旧来源输出的参与者接口。 |

<!-- write-step: 19 bytes=14030:14163 -->
````go
type AccountCloseFences struct {
	db         *sql.DB
	repository *repository.AccountCloseFences
	peer       AccountCloseFencePeer
}

````

## 第 20 步：NewAccountCloseFences

插入位置：第 19 步代码之后，紧接追加；完成稿约第 373 行。

本步声明：`func NewAccountCloseFences(db *sql.DB, peer AccountCloseFencePeer) *AccountCloseFences`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code>；<code>repository.NewAccountCloseFences(db)</code> |
| <code>peer</code> | <code>AccountCloseFencePeer</code> | 在此声明中的实际使用：<code>peer: peer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountCloseFences</code> | 实际返回表达式：<code>&amp;AccountCloseFences{db: db, repository: repository.NewAccountCloseFences(db), peer: peer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountCloseFences{db: db, repository: repository.NewAccountCloseFences(db), peer: peer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=14163:14353 -->
````go
func NewAccountCloseFences(db *sql.DB, peer AccountCloseFencePeer) *AccountCloseFences {
	return &AccountCloseFences{db: db, repository: repository.NewAccountCloseFences(db), peer: peer}
}

````

## 第 21 步：AccountCloseFences.Ready

插入位置：第 20 步代码之后，紧接追加；完成稿约第 377 行。

本步声明：`func (s *AccountCloseFences) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.InvalidTasks(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_FENCE_BINDING_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>invalid</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FENCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=14353:14589 -->
````go
func (s *AccountCloseFences) Ready(ctx context.Context) error {
	invalid, err := s.repository.InvalidTasks(ctx)
	if err != nil {
		return err
	}
	if invalid {
		return errors.New("ACCOUNT_CLOSE_FENCE_BINDING_INVALID")
	}
	return nil
}

````

## 第 22 步：AccountCloseFences.RecoverOne

插入位置：第 21 步代码之后，紧接追加；完成稿约第 388 行。

本步声明：`func (s *AccountCloseFences) RecoverOne(parent context.Context, instance string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 4*time.Second)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.Claim(ctx, tx, instance)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, 2*time.Second)</code>。
- 检查 <code>err != nil &#124;&#124; !platform.MatchesBootstrapAck(request, ack, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 执行 <code>finished.Rollback()</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 执行 <code>finished.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 22 bytes=14589:16470 -->
````go
func (s *AccountCloseFences) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	task, err := s.repository.Claim(ctx, tx, instance)
	if err != nil {
		_ = tx.Rollback()
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	payload, err := s.repository.Binding(ctx, s.db, task)
	if err != nil {
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_BINDING_INVALID")
		return
	}
	request := &pb.FenceRequest{RevocationId: payload.Revocation, Generation: 1,
		ClosedScopes: []*pb.SourceEpoch{{ScopeKey: payload.Scope, Epoch: payload.Epoch}},
		TargetDigest: platform.SourceClosureDigest(payload.Scope, payload.Epoch)}
	// 持久租约已提交后才调用真实Doc屏障。协议generation固定为1，不能把任务领取代次
	// 填入原屏障请求，否则恢复会悄悄变成另一份业务操作。
	rpcCtx, done := context.WithTimeout(ctx, 2*time.Second)
	ack, err := s.peer.FenceAccess(rpcCtx, request)
	done()
	if err != nil || !platform.MatchesBootstrapAck(request, ack, "document") {
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_NOT_CONFIRMED")
		return
	}
	finished, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_FINISH_UNKNOWN")
		return
	}
	if err = s.repository.Finish(ctx, finished, task, payload, ack.ResultDigest); err != nil {
		_ = finished.Rollback()
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_FINISH_UNKNOWN")
		return
	}
	if err = finished.Commit(); err != nil {
		// 条件重试不会改写已提交成功的任务；提交响应丢失仍按未知结果处理。
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_FINISH_UNKNOWN")
	}
}

// account_close_prepare.go：保留原实现，按业务合并文件。

````

## 第 23 步：AccountClosurePreparer

插入位置：第 22 步代码之后，紧接追加；完成稿约第 438 行。

本步声明：`type AccountClosurePreparer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Prepare</code> | <code>(context.Context, platform.AccountClosureBinding) (platform.AccountClosurePreparation, error)</code> | 接收ctx与原账号关闭绑定，返回Doc固定的准备事实；准备还不是最终注销。 |
| <code>Cancel</code> | <code>(context.Context, platform.AccountClosureBinding) error</code> | 按同一原绑定取消Doc准备，返回可重试错误或明确完成。 |

<!-- write-step: 23 bytes=16470:16678 -->
````go
type AccountClosurePreparer interface {
	Prepare(context.Context, platform.AccountClosureBinding) (platform.AccountClosurePreparation, error)
	Cancel(context.Context, platform.AccountClosureBinding) error
}

````

## 第 24 步：AccountClosures.PrepareClose

插入位置：第 23 步代码之后，紧接追加；完成稿约第 443 行。

PrepareClose先固定Account组织事实，再取得Doc预留，最后复核同一快照并原子冻结。
网络调用不持SQL锁；中间事实改变时只能拒绝原计划，不能静默替换关闭组织集合。

本步声明：`func (s *AccountClosures) PrepareClose(ctx context.Context, operation string, peer AccountClosurePreparer) (dto.AccountClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.prepareCloseStage(ctx, operation, nil)</code>；<code>peer.Cancel(ctx, *binding)</code>；<code>peer.Prepare(ctx, *binding)</code>；<code>s.prepareCloseStage(ctx, operation, &amp;prepared)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.prepareCloseStage(ctx, operation, nil)</code>；<code>s.prepareCloseStage(ctx, operation, &amp;prepared)</code> |
| <code>peer</code> | <code>AccountClosurePreparer</code> | 在此声明中的实际使用：<code>peer == nil</code>；<code>peer.Cancel</code>；<code>peer.Prepare</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.AccountClosePlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_CONTROL_REQUIRED&quot;)</code>；<code>peer.Cancel(ctx, *binding)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; binding == nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>peer == nil</code>。条件成立时执行 <code>return plan, errors.New(&quot;ACCOUNT_CLOSE_CONTROL_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>plan.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return plan, peer.Cancel(ctx, *binding)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; plan.State == &quot;REJECTED&quot; &amp;&amp; binding != nil</code>。
- 此处通过 <code>return plan, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=16678:17692 -->
````go
// PrepareClose先固定Account组织事实，再取得Doc预留，最后复核同一快照并原子冻结。
// 网络调用不持SQL锁；中间事实改变时只能拒绝原计划，不能静默替换关闭组织集合。
func (s *AccountClosures) PrepareClose(ctx context.Context, operation string, peer AccountClosurePreparer) (dto.AccountClosePlan, error) {
	plan, binding, err := s.prepareCloseStage(ctx, operation, nil)
	if err != nil || binding == nil {
		return plan, err
	}
	if peer == nil {
		return plan, errors.New("ACCOUNT_CLOSE_CONTROL_REQUIRED")
	}
	if plan.State == "REJECTED" {
		return plan, peer.Cancel(ctx, *binding)
	}
	prepared, err := peer.Prepare(ctx, *binding)
	if err != nil {
		// 回复未知仍保持同一PREPARING记录，重试原号向Doc取回已保存的预留。
		return plan, err
	}
	plan, binding, err = s.prepareCloseStage(ctx, operation, &prepared)
	if err == nil && plan.State == "REJECTED" && binding != nil {
		err = peer.Cancel(ctx, *binding)
	}
	return plan, err
}

````

## 第 25 步：AccountClosures.prepareCloseStage

插入位置：第 24 步代码之后，紧接追加；完成稿约第 468 行。

本步声明：`func (s *AccountClosures) prepareCloseStage(ctx context.Context, operation string, prepared *platform.AccountClosurePreparation) (dto.AccountClosePlan, *platform.AccountClosureBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.UserForOperation(ctx, operation)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.repository.LockUser(ctx, tx, user)</code>；<code>s.repository.Receipt(ctx, tx, operation)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.UserForOperation(ctx, operation)</code>；<code>s.repository.Receipt(ctx, tx, operation)</code>；<code>s.repository.Record(ctx, tx, operation)</code>；<code>accountCloseSavedPlan(operation, record, receipt)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>prepared</code> | <code>*platform.AccountClosurePreparation</code> | 使用 <code>platform.AccountClosurePreparation</code> 类型；其定义处的职责：定义Account与Document之间的固定注销绑定和有界清理分页协议。<br>在此声明中的实际使用：<code>prepared != nil</code>；<code>prepared.RequiredKnowledgeBases</code>；<code>prepared.ReservedKnowledgeBases</code>；<code>prepared.State</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.AccountClosePlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>*platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>实际返回表达式：<code>nil</code>；<code>capture.Binding</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_CONFLICT&quot;)</code>；<code>nil</code>；<code>marshalErr</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, nil, err</code>。当前函数在这里结束。
- 检查 <code>record.User != user &#124;&#124; receipt.SessionHash != capture.SessionHash &#124;&#124; (capture.Binding != nil &amp;&amp; (capture.Binding.User != user &#124;&#124; capture.Binding.Operation != operation))</code>。条件成立时执行 <code>return plan, nil, accountCloseConflict(&quot;ACCOUNT_CLOSE_BINDING_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>record.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return plan, capture.Binding, nil</code>。当前函数在这里结束。
- 检查 <code>record.State != &quot;PREPARING&quot;</code>。条件成立时执行 <code>return plan, nil, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, nil, err</code>。当前函数在这里结束。

<!-- write-step: 25 bytes=17692:19360 -->
````go
func (s *AccountClosures) prepareCloseStage(ctx context.Context, operation string, prepared *platform.AccountClosurePreparation) (dto.AccountClosePlan, *platform.AccountClosureBinding, error) {
	var plan dto.AccountClosePlan
	user, err := s.repository.UserForOperation(ctx, operation)
	if err != nil {
		return plan, nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return plan, nil, err
	}
	defer tx.Rollback()
	credential, err := s.repository.LockUser(ctx, tx, user)
	if err != nil {
		return plan, nil, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, operation)
	if err != nil {
		return plan, nil, err
	}
	record, err := s.repository.Record(ctx, tx, operation)
	if err != nil {
		return plan, nil, err
	}
	plan, err = accountCloseSavedPlan(operation, record, receipt)
	if err != nil {
		return plan, nil, err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return plan, nil, err
	}
	if record.User != user || receipt.SessionHash != capture.SessionHash || (capture.Binding != nil && (capture.Binding.User != user || capture.Binding.Operation != operation)) {
		return plan, nil, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
	}
	if record.State == "REJECTED" {
		return plan, capture.Binding, nil
	}
	if record.State != "PREPARING" {
		return plan, nil, nil
	}
	organizations, err := s.repository.LockOrganizations(ctx, tx, user)
	if err != nil {
		return plan, nil, err
	}
	identityMatches := credential.State == "ACTIVE" && credential.IdentityEpoch == capture.IdentityEpoch && credential.Epoch == strconv.FormatUint(capture.CredentialEpoch, 10)
````

## 第 26 步：AccountClosures.prepareCloseStage（接着实现）

插入位置：第 25 步代码之后，紧接追加；完成稿约第 513 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!identityMatches &#124;&#124; (capture.Binding != nil &amp;&amp; !slices.Equal(capture.Organizations, organizations))</code>。
- 检查 <code>marshalErr != nil</code>。条件成立时执行 <code>return plan, nil, marshalErr</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, nil, errors.New(&quot;ACCOUNT_CLOSE_REJECTION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return plan, capture.Binding, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>org.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; org.MemberState != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return plan, nil, accountCloseConflict(&quot;ACCOUNT_CLOSE_ORGANIZATION_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>org.Owner == user</code>。
- 检查 <code>org.LiveMembers == 1</code>。
- 检查 <code>org.Role == &quot;ADMIN&quot; &amp;&amp; org.AvailableAdministrators &lt;= 1</code>。
- 检查 <code>capture.Binding == nil &amp;&amp; len(plan.RequiredSuccessions) == 0</code>。
- 检查 <code>prepared != nil</code>。
- 检查 <code>capture.Binding == nil &#124;&#124; prepared.RequiredKnowledgeBases == nil &#124;&#124; prepared.ReservedKnowledgeBases == nil</code>。条件成立时执行 <code>return plan, nil, errors.New(&quot;ACCOUNT_CLOSE_PREPARATION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(prepared.RequiredKnowledgeBases) == 0 &#124;&#124; len(prepared.ReservedKnowledgeBases) != 0</code>。条件成立时执行 <code>return plan, nil, errors.New(&quot;ACCOUNT_CLOSE_PREPARATION_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 26 bytes=19360:21222 -->
````go
	if !identityMatches || (capture.Binding != nil && !slices.Equal(capture.Organizations, organizations)) {
		plan.State = "REJECTED"
		raw, marshalErr := json.Marshal(plan)
		if marshalErr != nil {
			return plan, nil, marshalErr
		}
		if err = s.repository.Reject(ctx, tx, operation, "PREPARATION_CHANGED", raw); err != nil {
			return plan, nil, err
		}
		if err = tx.Commit(); err != nil {
			return plan, nil, errors.New("ACCOUNT_CLOSE_REJECTION_UNKNOWN")
		}
		return plan, capture.Binding, nil
	}
	plan.RequiredSuccessions = plan.RequiredSuccessions[:0]
	closing := []uint64{}
	for _, org := range organizations {
		if org.Lifecycle != "ACTIVE" || org.MemberState != "ACTIVE" {
			return plan, nil, accountCloseConflict("ACCOUNT_CLOSE_ORGANIZATION_PENDING")
		}
		if org.Owner == user {
			if org.LiveMembers == 1 {
				closing = append(closing, org.ID)
			} else {
				appendAccountCloseSuccession(&plan, "ORGANIZATION_OWNER", org.ID)
			}
		} else if org.Role == "ADMIN" && org.AvailableAdministrators <= 1 {
			appendAccountCloseSuccession(&plan, "ORGANIZATION_ADMIN", org.ID)
		}
	}
	if capture.Binding == nil && len(plan.RequiredSuccessions) == 0 {
		capture.Binding = &platform.AccountClosureBinding{Operation: operation, User: user, IdentityEpoch: capture.IdentityEpoch, ClosingOrganizations: closing}
		capture.Organizations = organizations
	}
	freeze := false
	if prepared != nil {
		if capture.Binding == nil || prepared.RequiredKnowledgeBases == nil || prepared.ReservedKnowledgeBases == nil {
			return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
		}
		switch prepared.State {
		case "HANDOFF_REQUIRED":
			if len(prepared.RequiredKnowledgeBases) == 0 || len(prepared.ReservedKnowledgeBases) != 0 {
				return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
			}
			for _, kb := range prepared.RequiredKnowledgeBases {
````

## 第 27 步：AccountClosures.prepareCloseStage（接着实现）

插入位置：第 26 步代码之后，紧接追加；完成稿约第 558 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>kb == 0</code>。条件成立时执行 <code>return plan, nil, errors.New(&quot;ACCOUNT_CLOSE_PREPARATION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(plan.RequiredSuccessions) != 0 &#124;&#124; len(prepared.RequiredKnowledgeBases) != 0</code>。条件成立时执行 <code>return plan, nil, errors.New(&quot;ACCOUNT_CLOSE_PREPARATION_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return plan, nil, errors.New(&quot;ACCOUNT_CLOSE_PREPARATION_CONFLICT&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, nil, err</code>。当前函数在这里结束。
- 检查 <code>freeze</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, nil, errors.New(&quot;ACCOUNT_CLOSE_PREPARATION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return plan, capture.Binding, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=21222:22303 -->
````go
				if kb == 0 {
					return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
				}
				appendAccountCloseSuccession(&plan, "KNOWLEDGE_BASE_ADMIN", kb)
			}
		case "FROZEN":
			if len(plan.RequiredSuccessions) != 0 || len(prepared.RequiredKnowledgeBases) != 0 {
				return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
			}
			freeze = true
			plan.State = "FROZEN"
			capture.Scopes = accountCloseScopes(*capture.Binding, organizations)
		default:
			return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_CONFLICT")
		}
	}
	rawCapture, err := json.Marshal(capture)
	if err != nil {
		return plan, nil, err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return plan, nil, err
	}
	if freeze {
		err = s.repository.Freeze(ctx, tx, *capture.Binding, capture, rawPlan)
	} else {
		err = s.repository.SavePreparation(ctx, tx, operation, rawCapture, rawPlan)
	}
	if err != nil {
		return plan, nil, err
	}
	if err = tx.Commit(); err != nil {
		return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_UNKNOWN")
	}
	return plan, capture.Binding, nil
}

````

## 第 28 步：appendAccountCloseSuccession

插入位置：第 27 步代码之后，紧接追加；完成稿约第 596 行。

本步声明：`func appendAccountCloseSuccession(plan *dto.AccountClosePlan, kind string, target uint64)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>plan</code> | <code>*dto.AccountClosePlan</code> | 在此声明中的实际使用：<code>plan.RequiredSuccessions</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>Kind: kind</code> |
| <code>target</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(target, 10)</code> |

<!-- write-step: 28 bytes=22303:22595 -->
````go
func appendAccountCloseSuccession(plan *dto.AccountClosePlan, kind string, target uint64) {
	plan.RequiredSuccessions = append(plan.RequiredSuccessions, struct {
		Kind     string `json:"kind"`
		TargetId string `json:"target_id"`
	}{Kind: kind, TargetId: strconv.FormatUint(target, 10)})
}

````

## 第 29 步：accountCloseScopes

插入位置：第 28 步代码之后，紧接追加；完成稿约第 603 行。

本步声明：`func accountCloseScopes(binding platform.AccountClosureBinding, organizations []domain.AccountCloseOrganization) []domain.AccountCloseScope`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>binding.User</code>；<code>binding.Operation</code>；<code>binding.IdentityEpoch</code> |
| <code>organizations</code> | <code>&#91;&#93;domain.AccountCloseOrganization</code> | 使用 <code>domain.AccountCloseOrganization</code> 类型；其定义处的职责：MemberRevision是组织成员集合水位，MembershipRevision是本人那一行的水位， 两者都固定才能在关闭前发现其他成员加入与本人角色变化。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.AccountCloseScope</code> | 使用 <code>domain.AccountCloseScope</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>scopes</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return scopes</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=22595:23349 -->
````go
func accountCloseScopes(binding platform.AccountClosureBinding, organizations []domain.AccountCloseOrganization) []domain.AccountCloseScope {
	user := strconv.FormatUint(binding.User, 10)
	scopes := []domain.AccountCloseScope{{Revocation: binding.Operation, Scope: "identity:" + user, Epoch: binding.IdentityEpoch}}
	for _, org := range organizations {
		scope := "org-member:" + strconv.FormatUint(org.ID, 10) + ":" + user
		// 子撤权号固定且有界，不因原操作号接近最大长度而截断产生碰撞。
		digest := sha256.Sum256([]byte(binding.Operation + "\n" + scope))
		scopes = append(scopes, domain.AccountCloseScope{Revocation: "account-close-" + hex.EncodeToString(digest[:]), Scope: scope, Epoch: org.Epoch})
	}
	return scopes
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/account_close_preparation_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/06-account-close-preparation.md)。
