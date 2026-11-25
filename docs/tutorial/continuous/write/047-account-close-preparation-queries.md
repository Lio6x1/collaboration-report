# 分步编写 `services/account/internal/repository/account/account_close_preparation_queries.go`

先按 [A06 · 账号注销准备、职责交接与来源冻结](../lessons/account/06-account-close-preparation.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/047-account-close-preparation-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把注销准备、固定组织快照、来源冻结和各来源Fence子任务原子持久化。

PREPARING记录不代表身份已冻结；Freeze必须与全部gate和恢复任务同事务，避免半注销。

写之前先按这条执行路线理解各段的连接：

1. Prepare只写原回执和account_close；LockOrganizations在user锁后按组织编号锁组织及成员。
2. Freeze重验凭据/身份/组织版本后，把user与session/member变REVOKING，逐scope关闭admission并写authorization_change和ACCOUNT_CLOSE_FENCE。
3. AccountCloseFences.Finish在RPC后按user→receipt→account_close→task重锁，核验租约/代次/原绑定和ACK。

本文件需要守住的边界：

- 完成一个来源屏障不能提前结束父账号注销。
- 不能在网络RPC期间持有SQL行锁；提交前必须重新验证ownership。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/account_close_preparation_queries.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/account_close_preparation_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/account_close_preparation_queries.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/account_close_preparation_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:348 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
)

// account_close.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountClosures

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`type AccountClosures struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 2 bytes=348:391 -->
````go
type AccountClosures struct{ db *sql.DB }

````

## 第 3 步：NewAccountClosures

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func NewAccountClosures(db *sql.DB) *AccountClosures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountClosures</code> | 实际返回表达式：<code>&amp;AccountClosures{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountClosures{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=391:481 -->
````go
func NewAccountClosures(db *sql.DB) *AccountClosures { return &AccountClosures{db: db} }

````

## 第 4 步：AccountClosures.Hash

插入位置：第 3 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func (r *AccountClosures) Hash(ctx context.Context, user uint64) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT COALESCE(password_hash,X&#x27;&#x27;) FROM user WHERE id=?&quot;, user)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT COALESCE(password_hash,X&#x27;&#x27;) FROM user WHERE id=?&quot;, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>hash</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT COALESCE(password_hash,X&#x27;&#x27;) FROM user WHERE id=?&quot;, user).Scan(&amp;hash)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT COALESCE(password_hash,X&#x27;&#x27;) FROM user WHERE id=?&quot;, user)</code>。
- 此处通过 <code>return hash, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=481:713 -->
````go
func (r *AccountClosures) Hash(ctx context.Context, user uint64) ([]byte, error) {
	var hash []byte
	err := r.db.QueryRowContext(ctx, "SELECT COALESCE(password_hash,X'') FROM user WHERE id=?", user).Scan(&hash)
	return hash, err
}

````

## 第 5 步：AccountClosures.LockUser

插入位置：第 4 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func (r *AccountClosures) LockUser(ctx context.Context, tx *sql.Tx, user uint64) (domain.Credential, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(password_hash,X&#x27;&#x27;),CAST(credential_epoch AS CHAR),identity_epoch,state 		FROM user WHERE id=? FOR UPDATE`, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(password_hash,X&#x27;&#x27;),CAST(credential_epoch AS CHAR),identity_epoch,state 		FROM user WHERE id=? FOR UPDATE`, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Credential</code> | 使用 <code>domain.Credential</code> 类型；其定义处的职责：按调用目的承载密码或状态代次快照。<br>实际返回表达式：<code>credential</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(password_hash,X&#x27;&#x27;),CAST(credential_epoch AS CHAR),identity_epoch,state 		FROM user WHERE id=? FOR UPDATE`, user).Scan(&amp;credential.Hash, &amp;cr …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(password_hash,X&#x27;&#x27;),CAST(credential_epoch AS CHAR),identity_epoch,state 		FROM user WHERE id=? FOR UPDATE`, user)</code>。
- 此处通过 <code>return credential, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=713:1134 -->
````go
func (r *AccountClosures) LockUser(ctx context.Context, tx *sql.Tx, user uint64) (domain.Credential, error) {
	var credential domain.Credential
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(password_hash,X''),CAST(credential_epoch AS CHAR),identity_epoch,state
		FROM user WHERE id=? FOR UPDATE`, user).Scan(&credential.Hash, &credential.Epoch, &credential.IdentityEpoch, &credential.State)
	return credential, err
}

````

## 第 6 步：AccountClosures.LockSession

插入位置：第 5 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`func (r *AccountClosures) LockSession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT CAST(user_id AS CHAR),state,CAST(credential_epoch AS CHAR),epoch,expires_at 		FROM session WHERE id=? FOR UPDATE`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>ID: id</code>；<code>tx.QueryRowContext(ctx, `SELECT CAST(user_id AS CHAR),state,CAST(credential_epoch AS CHAR),epoch,expires_at 		FROM session WHERE id=? FOR UPDATE`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>实际返回表达式：<code>session</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT CAST(user_id AS CHAR),state,CAST(credential_epoch AS CHAR),epoch,expires_at 		FROM session WHERE id=? FOR UPDATE`, id).Scan(&amp;session.UserID, &amp;sessio …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT CAST(user_id AS CHAR),state,CAST(credential_epoch AS CHAR),epoch,expires_at 		FROM session WHERE id=? FOR UPDATE`, id)</code>。
- 此处通过 <code>return session, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1134:1558 -->
````go
func (r *AccountClosures) LockSession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error) {
	session := domain.Session{ID: id}
	err := tx.QueryRowContext(ctx, `SELECT CAST(user_id AS CHAR),state,CAST(credential_epoch AS CHAR),epoch,expires_at
		FROM session WHERE id=? FOR UPDATE`, id).Scan(&session.UserID, &session.State, &session.CredentialEpoch, &session.Epoch, &session.Expires)
	return session, err
}

````

## 第 7 步：AccountClosures.Receipt

插入位置：第 6 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`func (r *AccountClosures) Receipt(ctx context.Context, tx *sql.Tx, operation string) (domain.AccountCloseReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT actor_id,kind,state,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;),request_fingerprint,result_json 		FROM operation_receipt W …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT actor_id,kind,state,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;),request_fingerprint,result_json 		FROM operation_receipt W …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseReceipt</code> | 使用 <code>domain.AccountCloseReceipt</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>receipt</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT actor_id,kind,state,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;),request_fingerprint,result_json 		FROM operation_receipt W …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return receipt, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1558:2092 -->
````go
func (r *AccountClosures) Receipt(ctx context.Context, tx *sql.Tx, operation string) (domain.AccountCloseReceipt, error) {
	var receipt domain.AccountCloseReceipt
	err := tx.QueryRowContext(ctx, `SELECT actor_id,kind,state,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),''),request_fingerprint,result_json
		FROM operation_receipt WHERE operation_id=? FOR UPDATE`, operation).
		Scan(&receipt.User, &receipt.Kind, &receipt.State, &receipt.SessionHash, &receipt.Fingerprint, &receipt.Result)
	return receipt, err
}

````

## 第 8 步：AccountClosures.Record

插入位置：第 7 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func (r *AccountClosures) Record(ctx context.Context, tx *sql.Tx, operation string) (domain.AccountCloseRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,retention_plan FROM account_close WHERE operation_id=? FOR UPDATE&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,retention_plan FROM account_close WHERE operation_id=? FOR UPDATE&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseRecord</code> | 使用 <code>domain.AccountCloseRecord</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,retention_plan FROM account_close WHERE operation_id=? FOR UPDATE&quot;, operation)</code>。
- 此处通过 <code>return record, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2092:2461 -->
````go
func (r *AccountClosures) Record(ctx context.Context, tx *sql.Tx, operation string) (domain.AccountCloseRecord, error) {
	var record domain.AccountCloseRecord
	err := tx.QueryRowContext(ctx, "SELECT user_id,state,retention_plan FROM account_close WHERE operation_id=? FOR UPDATE", operation).
		Scan(&record.User, &record.State, &record.Capture)
	return record, err
}

````

## 第 9 步：AccountClosures.HasPending

插入位置：第 8 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`func (r *AccountClosures) HasPending(ctx context.Context, tx *sql.Tx, user uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT operation_id FROM account_close WHERE user_id=? 		AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) ORDER BY operation_id LIMIT 1 FOR UPDATE`, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT operation_id FROM account_close WHERE user_id=? 		AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) ORDER BY operation_id LIMIT 1 FOR UPDATE`, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT operation_id FROM account_close WHERE user_id=? 		AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) ORDER BY operation_id LIMIT 1 FOR UPDATE`, user).Scan(&amp;operation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT operation_id FROM account_close WHERE user_id=? 		AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) ORDER BY operation_id LIMIT 1 FOR UPDATE`, user)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2461:2866 -->
````go
func (r *AccountClosures) HasPending(ctx context.Context, tx *sql.Tx, user uint64) (bool, error) {
	var operation string
	err := tx.QueryRowContext(ctx, `SELECT operation_id FROM account_close WHERE user_id=?
		AND state NOT IN ('SUCCEEDED','REJECTED') ORDER BY operation_id LIMIT 1 FOR UPDATE`, user).Scan(&operation)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

````

## 第 10 步：AccountClosures.Prepare

插入位置：第 9 步代码之后，紧接追加；完成稿约第 66 行。

Prepare只写入准备记录和原回执，同一事务回滚时两者一起撤销；这里不冻结身份来源。

本步声明：`func (r *AccountClosures) Prepare(ctx context.Context, tx *sql.Tx, user uint64, operation string, capture, target, fingerprint, result []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at) 		VALUES(?,?,&#x27;ACCOUNT_CLOSE&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `INSERT INTO account_close(operation_id,user_id,state,retention_plan,retry_at) 		VALUES(?,?,&#x27;PREPARING&#x27;,?,UTC_TIMESTAMP(6))`, operation, user, string(capture))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at) 		VALUES(?,?,&#x27;ACCOUNT_CLOSE&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `INSERT INTO account_close(operation_id,user_id,state,retention_plan,retry_at) 		VALUES(?,?,&#x27;PREPARING&#x27;,?,UTC_TIMESTAMP(6))`, operation, user, string(capture))</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at) 		VALUES(?,?,&#x27;ACCOUNT_CLOSE&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `INSERT INTO account_close(operation_id,user_id,state,retention_plan,retry_at) 		VALUES(?,?,&#x27;PREPARING&#x27;,?,UTC_TIMESTAMP(6))`, operation, user, string(capture))</code> |
| <code>capture</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(capture)</code> |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at) 		VALUES(?,?,&#x27;ACCOUNT_CLOSE&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续） |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at) 		VALUES(?,?,&#x27;ACCOUNT_CLOSE&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO account_close(operation_id,user_id,state,retention_plan,retry_at) 		VALUES(?,?,&#x27;PREPARING&#x27;,?,UTC_TIMESTAMP(6))`, operation, user, string(capture))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 10 bytes=2866:3679 -->
````go
// Prepare只写入准备记录和原回执，同一事务回滚时两者一起撤销；这里不冻结身份来源。
func (r *AccountClosures) Prepare(ctx context.Context, tx *sql.Tx, user uint64, operation string, capture, target, fingerprint, result []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at)
		VALUES(?,?,'ACCOUNT_CLOSE',?,?,'PREPARING',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY))`, operation, user, string(target), fingerprint, string(result))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO account_close(operation_id,user_id,state,retention_plan,retry_at)
		VALUES(?,?,'PREPARING',?,UTC_TIMESTAMP(6))`, operation, user, string(capture))
	return err
}

````

## 第 11 步：AccountClosures.Bind

插入位置：第 10 步代码之后，紧接追加；完成稿约第 78 行。

本步声明：`func (r *AccountClosures) Bind(ctx context.Context, tx *sql.Tx, operation string, capture []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE account_close SET retention_plan=? WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, string(capture), operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE account_close SET retention_plan=? WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, string(capture), operation)</code> |
| <code>capture</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(capture)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>accountCloseChanged(result, err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE account_close SET retention_plan=? WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, string(capture), operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(result, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3679:3981 -->
````go
func (r *AccountClosures) Bind(ctx context.Context, tx *sql.Tx, operation string, capture []byte) error {
	result, err := tx.ExecContext(ctx, "UPDATE account_close SET retention_plan=? WHERE operation_id=? AND state='PREPARING'", string(capture), operation)
	return accountCloseChanged(result, err)
}

````

## 第 12 步：AccountClosures.Reject

插入位置：第 11 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`func (r *AccountClosures) Reject(ctx context.Context, tx *sql.Tx, operation, reason string, result []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE account_close SET state=&#x27;REJECTED&#x27;,last_error=?,retry_at=NULL 		WHERE operation_id=? AND state=&#x27;PREPARING&#x27;`, reason, operation)</code>；<code>tx.ExecContext(ctx, `UPDATE operation_receipt SET state=&#x27;REJECTED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) 		WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PREPARING&#x27;`, string(result), operation)</code>；<code>r.Record(ctx, tx, operation)</code>；<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;ACCOUNT_CLOSE_CANCEL&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))`, operation, operation, string(payload))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>r.Record(ctx, tx, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE account_close SET state=&#x27;REJECTED&#x27;,last_error=?,retry_at=NULL 		WHERE operation_id=? AND state=&#x27;PREPARING&#x27;`, reason, operation)</code>；<code>tx.ExecContext(ctx, `UPDATE operation_receipt SET state=&#x27;REJECTED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) 		WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PREPARING&#x27;`, string(result), operation)</code>；<code>r.Record(ctx, tx, operation)</code>；<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;ACCOUNT_CLOSE_CANCEL&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))`, operation, operation, string(payload))</code> |
| <code>reason</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE account_close SET state=&#x27;REJECTED&#x27;,last_error=?,retry_at=NULL 		WHERE operation_id=? AND state=&#x27;PREPARING&#x27;`, reason, operation)</code> |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_BINDING_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE account_close SET state=&#x27;REJECTED&#x27;,last_error=?,retry_at=NULL 		WHERE operation_id=? AND state=&#x27;PREPARING&#x27;`, reason, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE operation_receipt SET state=&#x27;REJECTED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) 		WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PREPARING&#x27;`, string(result), operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_BINDING_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>capture.Binding == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;ACCOUNT_CLOSE_CANCEL&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))`, operation, operation, string(payload))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 12 bytes=3981:5515 -->
````go
func (r *AccountClosures) Reject(ctx context.Context, tx *sql.Tx, operation, reason string, result []byte) error {
	changed, err := tx.ExecContext(ctx, `UPDATE account_close SET state='REJECTED',last_error=?,retry_at=NULL
		WHERE operation_id=? AND state='PREPARING'`, reason, operation)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET state='REJECTED',result_json=?,terminal_at=UTC_TIMESTAMP(6)
		WHERE operation_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING'`, string(result), operation)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	record, err := r.Record(ctx, tx, operation)
	if err != nil {
		return err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return errors.New("ACCOUNT_CLOSE_BINDING_REQUIRED")
	}
	if capture.Binding == nil {
		// 未向Doc固定任何绑定时只有本地准备记录，拒绝即可终结，不登记虚构取消范围。
		return nil
	}
	payload, err := json.Marshal(capture.Binding)
	if err != nil {
		return err
	}
	// 拒绝证明与取消意图一起提交。进程退出或Doc回复丢失后仍有原号任务，
	// 不依赖用户再次点击，也不靠超时猜测下游预留是否存在。
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('ACCOUNT_CLOSE_CANCEL',?,?,?,'QUEUED',UTC_TIMESTAMP(6))`, operation, operation, string(payload))
	return err
}

````

## 第 13 步：accountCloseChanged

插入位置：第 12 步代码之后，紧接追加；完成稿约第 117 行。

本步声明：`func accountCloseChanged(result sql.Result, err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>result</code> | <code>sql.Result</code> | 在此声明中的实际使用：<code>result.RowsAffected</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err != nil</code>；<code>return err</code>；<code>count, err := result.RowsAffected()</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>count != 1</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5515:5833 -->
````go
func accountCloseChanged(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("ACCOUNT_CLOSE_CHANGED")
	}
	return nil
}

// account_close_fence.go：保留原实现，按业务合并文件。

````

## 第 14 步：AccountCloseFences

插入位置：第 13 步代码之后，紧接追加；完成稿约第 133 行。

本步声明：`type AccountCloseFences struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 14 bytes=5833:5879 -->
````go
type AccountCloseFences struct{ db *sql.DB }

````

## 第 15 步：NewAccountCloseFences

插入位置：第 14 步代码之后，紧接追加；完成稿约第 135 行。

本步声明：`func NewAccountCloseFences(db *sql.DB) *AccountCloseFences`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountCloseFences</code> | 实际返回表达式：<code>&amp;AccountCloseFences{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountCloseFences{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=5879:5978 -->
````go
func NewAccountCloseFences(db *sql.DB) *AccountCloseFences { return &AccountCloseFences{db: db} }

````

## 第 16 步：accountCloseFenceTaskColumns

插入位置：第 15 步代码之后，紧接追加；完成稿约第 137 行。

本步声明：`const accountCloseFenceTaskColumns = \`t.id,t.generation,COALESCE(t.operation_id,''),t.business_key,t.state,`。

<!-- write-step: 16 bytes=5978:6173 -->
````go
const accountCloseFenceTaskColumns = `t.id,t.generation,COALESCE(t.operation_id,''),t.business_key,t.state,
	COALESCE(t.instance_id,''),t.payload,COALESCE(t.lease_until>UTC_TIMESTAMP(6),FALSE)`

````

## 第 17 步：scanAccountCloseFenceTask

插入位置：第 16 步代码之后，紧接追加；完成稿约第 140 行。

本步声明：`func scanAccountCloseFenceTask(row interface{ Scan(...any) error }) (domain.AccountCloseFenceTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>interface{ Scan(...any) error }</code> | 在此声明中的实际使用：<code>row.Scan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>row.Scan(&amp;task.ID, &amp;task.Generation, &amp;task.Operation, &amp;task.Revocation, &amp;task.State, &amp;task.Instance, &amp;task.Payload, &amp;task.LeaseValid)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return task, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=6173:6483 -->
````go
func scanAccountCloseFenceTask(row interface{ Scan(...any) error }) (domain.AccountCloseFenceTask, error) {
	var task domain.AccountCloseFenceTask
	err := row.Scan(&task.ID, &task.Generation, &task.Operation, &task.Revocation, &task.State, &task.Instance, &task.Payload, &task.LeaseValid)
	return task, err
}

````

## 第 18 步：AccountCloseFences.Claim

插入位置：第 17 步代码之后，紧接追加；完成稿约第 146 行。

本步声明：`func (r *AccountCloseFences) Claim(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimClosureTask(ctx, tx, &#91;&#93;string{domain.AccountCloseFenceKind})</code>；<code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE t.id=? AND t.kind=? FOR UPDATE`, claimed.ID, domain.AccountCloseFenceKind)</code>；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 		lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimClosureTask(ctx, tx, &#91;&#93;string{domain.AccountCloseFenceKind})</code>；<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance == &quot;&quot;</code>；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 		lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续）；<code>task.Instance, task.State, task.LeaseValid = instance, &quot;RUNNING&quot;, true</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>domain.AccountCloseFenceTask{}</code>；<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_CLOSE_FENCE_INSTANCE_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>instance == &quot;&quot;</code>。条件成立时执行 <code>return domain.AccountCloseFenceTask{}, errors.New(&quot;ACCOUNT_CLOSE_FENCE_INSTANCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.AccountCloseFenceTask{}, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE t.id=? AND t.kind=? FOR UPDATE`, claimed.ID, domain.AccountCloseFenceKind)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 		lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 此处通过 <code>return task, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=6483:7785 -->
````go
func (r *AccountCloseFences) Claim(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error) {
	if instance == "" {
		return domain.AccountCloseFenceTask{}, errors.New("ACCOUNT_CLOSE_FENCE_INSTANCE_INVALID")
	}
	claimed, err := claimClosureTask(ctx, tx, []string{domain.AccountCloseFenceKind})
	if err != nil {
		return domain.AccountCloseFenceTask{}, err
	}
	task, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE t.id=? AND t.kind=? FOR UPDATE`, claimed.ID, domain.AccountCloseFenceKind))
	if err != nil {
		return task, err
	}
	// 每次领取推进代次，旧worker即使复用实例编号也不能提交新租约的结果。
	changed, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='RUNNING',instance_id=?,generation=generation+1,
		lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts<4294967295,attempts+1,attempts)
		WHERE id=? AND kind=? AND generation=? AND generation<18446744073709551615`, instance, task.ID, domain.AccountCloseFenceKind, task.Generation)
	if err = accountCloseChanged(changed, err); err != nil {
		return task, err
	}
	task.Generation++
	task.Instance, task.State, task.LeaseValid = instance, "RUNNING", true
	return task, nil
}

````

## 第 19 步：accountCloseFenceFacts

插入位置：第 18 步代码之后，紧接追加；完成稿约第 170 行。

本步声明：`type accountCloseFenceFacts struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>User</code> | <code>uint64</code> | account_close记录中的被注销账号。 |
| <code>Actor</code> | <code>uint64</code> | 操作回执中的操作者。 |
| <code>AuthorizationActor</code> | <code>uint64</code> | authorization_change中的操作者，需与前两者一致。 |
| <code>Epoch</code> | <code>uint64</code> | 本次身份来源关闭代次。 |
| <code>GateEpoch</code> | <code>uint64</code> | access_gate保存的最高关闭代次。 |
| <code>CloseState</code> | <code>string</code> | 账号注销状态。 |
| <code>ReceiptKind</code> | <code>string</code> | 原回执种类，须为对应注销业务。 |
| <code>ReceiptState</code> | <code>string</code> | 原操作回执所处状态，决定是返回既有结果、继续原操作还是等待核验；未知状态不能伪报成功。 |
| <code>AuthorizationState</code> | <code>string</code> | 权限关闭流程状态。 |
| <code>Scope</code> | <code>string</code> | 关闭的身份来源键。 |
| <code>GateState</code> | <code>string</code> | 来源门状态。 |
| <code>GateRevocation</code> | <code>string</code> | 来源门最后关闭操作号。 |
| <code>Capture</code> | <code>&#91;&#93;byte</code> | 账号注销记录的固定留存计划字节。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 权限关闭固定目标字节。 |
| <code>Participants</code> | <code>&#91;&#93;byte</code> | 权限关闭参与者计划JSON，不能只收到一个任意ACK就算完成。 |

<!-- write-step: 19 bytes=7785:8125 -->
````go
type accountCloseFenceFacts struct {
	User, Actor, AuthorizationActor, Epoch, GateEpoch                                           uint64
	CloseState, ReceiptKind, ReceiptState, AuthorizationState, Scope, GateState, GateRevocation string
	Capture, Target, Participants                                                               []byte
}

````

## 第 20 步：accountCloseFenceFactsColumns

插入位置：第 19 步代码之后，紧接追加；完成稿约第 176 行。

本步声明：`const accountCloseFenceFactsColumns = \`COALESCE(c.user_id,0),COALESCE(c.state,''),c.retention_plan,`。

<!-- write-step: 20 bytes=8125:8501 -->
````go
const accountCloseFenceFactsColumns = `COALESCE(c.user_id,0),COALESCE(c.state,''),c.retention_plan,
	COALESCE(o.actor_id,0),COALESCE(o.kind,''),COALESCE(o.state,''),COALESCE(a.actor_id,0),COALESCE(a.scope_key,''),COALESCE(a.epoch,0),
	COALESCE(a.state,''),a.target_permission,a.participant_plan,COALESCE(g.max_epoch,0),COALESCE(g.state,''),COALESCE(g.last_revocation_id,'')`

````

## 第 21 步：accountCloseFenceFactsJoins

插入位置：第 20 步代码之后，紧接追加；完成稿约第 180 行。

本步声明：`const accountCloseFenceFactsJoins = \` LEFT JOIN account_close c ON c.operation_id=t.operation_id`。

<!-- write-step: 21 bytes=8501:8785 -->
````go
const accountCloseFenceFactsJoins = ` LEFT JOIN account_close c ON c.operation_id=t.operation_id
	LEFT JOIN operation_receipt o ON o.operation_id=c.operation_id
	LEFT JOIN authorization_change a ON a.revocation_id=t.business_key
	LEFT JOIN access_gate g ON g.scope_key=a.scope_key `

````

## 第 22 步：accountCloseFenceFactDestinations

插入位置：第 21 步代码之后，紧接追加；完成稿约第 185 行。

本步声明：`func accountCloseFenceFactDestinations(f *accountCloseFenceFacts) []any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>f</code> | <code>*accountCloseFenceFacts</code> | 在此声明中的实际使用：<code>f.User</code>；<code>f.CloseState</code>；<code>f.Capture</code>；<code>f.Actor</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;any</code> | 实际返回表达式：<code>&#91;&#93;any{&amp;f.User, &amp;f.CloseState, &amp;f.Capture, &amp;f.Actor, &amp;f.ReceiptKind, &amp;f.ReceiptState, &amp;f.AuthorizationActor, &amp;f.Scope, &amp;f.Epoch, &amp;f.AuthorizationState, &amp;f.Target, &amp;f.Participants, &amp; …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return &#91;&#93;any{&amp;f.User, &amp;f.CloseState, &amp;f.Capture, &amp;f.Actor, &amp;f.ReceiptKind, &amp;f.ReceiptState, &amp;f.AuthorizationActor, &amp;f.Scope, &amp;f.Epoch, &amp;f.AuthorizationState, &amp;f.Target, &amp;f.Particip …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=8785:9096 -->
````go
func accountCloseFenceFactDestinations(f *accountCloseFenceFacts) []any {
	return []any{&f.User, &f.CloseState, &f.Capture, &f.Actor, &f.ReceiptKind, &f.ReceiptState, &f.AuthorizationActor, &f.Scope, &f.Epoch, &f.AuthorizationState, &f.Target, &f.Participants, &f.GateEpoch, &f.GateState, &f.GateRevocation}
}

````

## 第 23 步：validateAccountCloseFence

插入位置：第 22 步代码之后，紧接追加；完成稿约第 189 行。

本步声明：`func validateAccountCloseFence(task domain.AccountCloseFenceTask, facts accountCloseFenceFacts) (domain.AccountCloseFencePayload, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.Payload</code>；<code>task.Operation</code>；<code>task.Revocation</code> |
| <code>facts</code> | <code>accountCloseFenceFacts</code> | 在此声明中的实际使用：<code>facts.User</code>；<code>facts.Actor</code>；<code>facts.AuthorizationActor</code>；<code>facts.CloseState</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseFencePayload</code> | 使用 <code>domain.AccountCloseFencePayload</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>payload</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return payload, err</code>。当前函数在这里结束。
- 检查 <code>task.Operation != payload.Operation &#124;&#124; task.Revocation != payload.Revocation &#124;&#124; facts.User != payload.User &#124;&#124; facts.Actor != payload.User &#124;&#124; facts.AuthorizationActor != payload.Use …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return payload, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; capture.Binding == nil &#124;&#124; capture.Binding.Operation != payload.Operation &#124;&#124; capture.Binding.User != payload.User</code>。条件成立时执行 <code>return payload, invalid</code>。当前函数在这里结束。
- 检查 <code>scope.Scope == payload.Scope &#124;&#124; scope.Revocation == payload.Revocation</code>。
- 检查 <code>scope.Scope != payload.Scope &#124;&#124; scope.Revocation != payload.Revocation &#124;&#124; scope.Epoch != payload.Epoch</code>。条件成立时执行 <code>return payload, invalid</code>。当前函数在这里结束。
- 检查 <code>matched != 1 &#124;&#124; json.Unmarshal(facts.Target, &amp;target) != nil &#124;&#124; target.Operation != payload.Operation &#124;&#124; target.Scope != payload.Scope &#124;&#124; target.Epoch != payload.Epoch &#124;&#124; json.Unma …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return payload, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return payload, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=9096:10990 -->
````go
func validateAccountCloseFence(task domain.AccountCloseFenceTask, facts accountCloseFenceFacts) (domain.AccountCloseFencePayload, error) {
	payload, err := domain.DecodeAccountCloseFence(task.Payload)
	if err != nil {
		return payload, err
	}
	invalid := errors.New("ACCOUNT_CLOSE_FENCE_BINDING_INVALID")
	if task.Operation != payload.Operation || task.Revocation != payload.Revocation || facts.User != payload.User || facts.Actor != payload.User || facts.AuthorizationActor != payload.User || facts.CloseState != "FROZEN" || facts.ReceiptKind != "ACCOUNT_CLOSE" || facts.ReceiptState != "PREPARING" || facts.AuthorizationState != "FENCING" || facts.Scope != payload.Scope || facts.Epoch != payload.Epoch || facts.GateEpoch != payload.Epoch || facts.GateState != "BLOCKED" || facts.GateRevocation != payload.Revocation {
		return payload, invalid
	}
	capture, err := domain.DecodeAccountCloseCapture(facts.Capture)
	if err != nil || capture.Binding == nil || capture.Binding.Operation != payload.Operation || capture.Binding.User != payload.User {
		return payload, invalid
	}
	matched := 0
	for _, scope := range capture.Scopes {
		if scope.Scope == payload.Scope || scope.Revocation == payload.Revocation {
			if scope.Scope != payload.Scope || scope.Revocation != payload.Revocation || scope.Epoch != payload.Epoch {
				return payload, invalid
			}
			matched++
		}
	}
	var target struct {
		Operation string `json:"account_close_operation"`
		Scope     string `json:"scope_key"`
		Epoch     uint64 `json:"epoch"`
	}
	var participants []string
	if matched != 1 || json.Unmarshal(facts.Target, &target) != nil || target.Operation != payload.Operation || target.Scope != payload.Scope || target.Epoch != payload.Epoch || json.Unmarshal(facts.Participants, &participants) != nil || len(participants) != 1 || participants[0] != "document" {
		return payload, invalid
	}
	return payload, nil
}

````

## 第 24 步：AccountCloseFences.Binding

插入位置：第 23 步代码之后，紧接追加；完成稿约第 223 行。

本步声明：`func (r *AccountCloseFences) Binding(ctx context.Context, q Queryer, task domain.AccountCloseFenceTask) (domain.AccountCloseFencePayload, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT `+accountCloseFenceFactsColumns+` FROM durable_task t `+accountCloseFenceFactsJoins+` 		WHERE t.id=? AND t.kind=? AND t.operation_id=? AND t.business …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Operation</code>；<code>task.Revocation</code>；<code>task.Generation</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseFencePayload</code> | 使用 <code>domain.AccountCloseFencePayload</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>由 <code>validateAccountCloseFence(task, facts)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>domain.AccountCloseFencePayload{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>validateAccountCloseFence(task, facts)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT `+accountCloseFenceFactsColumns+` FROM durable_task t `+accountCloseFenceFactsJoins+` 		WHERE t.id=? AND t.kind=? AND t.operation_id=? AND t.business …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.AccountCloseFencePayload{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return validateAccountCloseFence(task, facts)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=10990:11740 -->
````go
func (r *AccountCloseFences) Binding(ctx context.Context, q Queryer, task domain.AccountCloseFenceTask) (domain.AccountCloseFencePayload, error) {
	var facts accountCloseFenceFacts
	err := q.QueryRowContext(ctx, `SELECT `+accountCloseFenceFactsColumns+` FROM durable_task t `+accountCloseFenceFactsJoins+`
		WHERE t.id=? AND t.kind=? AND t.operation_id=? AND t.business_key=? AND t.generation=? AND t.state='RUNNING' AND t.instance_id=? AND t.lease_until>UTC_TIMESTAMP(6)`, task.ID, domain.AccountCloseFenceKind, task.Operation, task.Revocation, task.Generation, task.Instance).
		Scan(accountCloseFenceFactDestinations(&facts)...)
	if err != nil {
		return domain.AccountCloseFencePayload{}, err
	}
	return validateAccountCloseFence(task, facts)
}

````

## 第 25 步：AccountCloseFences.Retry

插入位置：第 24 步代码之后，紧接追加；完成稿约第 234 行。

本步声明：`func (r *AccountCloseFences) Retry(ctx context.Context, task domain.AccountCloseFenceTask, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,instance_id=NULL, 		retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND  …</code>（长表达式，按本步完整代码继续） |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Operation</code>；<code>task.Revocation</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,instance_id=NULL, 		retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,instance_id=NULL, 		retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=11740:12272 -->
````go
func (r *AccountCloseFences) Retry(ctx context.Context, task domain.AccountCloseFenceTask, code string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',lease_until=NULL,instance_id=NULL,
		retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND kind=? AND generation=?
		AND operation_id=? AND business_key=? AND instance_id=? AND state='RUNNING'`, code, task.ID, domain.AccountCloseFenceKind, task.Generation, task.Operation, task.Revocation, task.Instance)
	return err
}

````

## 第 26 步：AccountCloseFences.Finish

插入位置：第 25 步代码之后，紧接追加；完成稿约第 241 行。

Finish锁序沿用user→receipt→account_close→task。RPC期间不持SQL锁，提交前重新核验
租约代次和固定绑定；这里只结束一个来源屏障，不结束账号注销或全局操作回执。

本步声明：`func (r *AccountCloseFences) Finish(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, payload domain.AccountCloseFencePayload, digest []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>closures.LockUser(ctx, tx, payload.User)</code>；<code>closures.Receipt(ctx, tx, task.Operation)</code>；<code>closures.Record(ctx, tx, task.Operation)</code>；<code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE t.id=? AND t.kind=? FOR UPDATE`, task.ID, domain.AccountCloseFenceKind)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>closures.LockUser(ctx, tx, payload.User)</code>；<code>closures.Receipt(ctx, tx, task.Operation)</code>；<code>closures.Record(ctx, tx, task.Operation)</code>；<code>tx.QueryRowContext</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>payload</code> | <code>domain.AccountCloseFencePayload</code> | 使用 <code>domain.AccountCloseFencePayload</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>payload.User</code>；<code>actual != payload</code>；<code>payload.Revocation</code>；<code>payload.Scope</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>digest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(digest)</code>；<code>tx.ExecContext(ctx, `INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) 		VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIME …</code>（长表达式，按本步完整代码继续）；<code>bytes.Equal(existing, digest)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_FENCE_OWNERSHIP_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_FENCE_ACK_INVALID&quot;)</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_FENCE_ACK_CONFLICT&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE t.id=? AND t.kind=? FOR UPDATE`, task.ID, domain.AccountCloseFenceKind)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.Generation != task.Generation &#124;&#124; current.Instance != task.Instance &#124;&#124; current.State != &quot;RUNNING&quot; &#124;&#124; !current.LeaseValid &#124;&#124; current.Operation != task.Operation &#124;&#124; current.Re …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FENCE_OWNERSHIP_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>actual != payload &#124;&#124; len(digest) != 32</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FENCE_ACK_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) 		VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIME …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=&#x27;document&#x27; AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27; FOR UPD …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=&#x27;document&#x27; AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27; FOR UPD …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(existing, digest)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FENCE_ACK_CONFLICT&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE authorization_change SET state=&#x27;SUCCEEDED&#x27;,last_error=NULL,retry_at=NULL 		WHERE revocation_id=? AND actor_id=? AND scope_key=? AND epoch=? AND state=&#x27;F …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 26 bytes=12272:14800 -->
````go
// Finish锁序沿用user→receipt→account_close→task。RPC期间不持SQL锁，提交前重新核验
// 租约代次和固定绑定；这里只结束一个来源屏障，不结束账号注销或全局操作回执。
func (r *AccountCloseFences) Finish(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, payload domain.AccountCloseFencePayload, digest []byte) error {
	closures := NewAccountClosures(r.db)
	if _, err := closures.LockUser(ctx, tx, payload.User); err != nil {
		return err
	}
	if _, err := closures.Receipt(ctx, tx, task.Operation); err != nil {
		return err
	}
	if _, err := closures.Record(ctx, tx, task.Operation); err != nil {
		return err
	}
	current, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE t.id=? AND t.kind=? FOR UPDATE`, task.ID, domain.AccountCloseFenceKind))
	if err != nil {
		return err
	}
	if current.Generation != task.Generation || current.Instance != task.Instance || current.State != "RUNNING" || !current.LeaseValid || current.Operation != task.Operation || current.Revocation != task.Revocation || !bytes.Equal(current.Payload, task.Payload) {
		return errors.New("ACCOUNT_CLOSE_FENCE_OWNERSHIP_CHANGED")
	}
	actual, err := r.Binding(ctx, tx, current)
	if err != nil {
		return err
	}
	if actual != payload || len(digest) != 32 {
		return errors.New("ACCOUNT_CLOSE_FENCE_ACK_INVALID")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at)
		VALUES(?,'document',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE revocation_id=revocation_id`, payload.Revocation, payload.Scope, payload.Epoch, digest)
	if err != nil {
		return err
	}
	var existing []byte
	err = tx.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant='document' AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED' FOR UPDATE`, payload.Revocation, payload.Scope, payload.Epoch).Scan(&existing)
	if err != nil {
		return err
	}
	if !bytes.Equal(existing, digest) {
		return errors.New("ACCOUNT_CLOSE_FENCE_ACK_CONFLICT")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE authorization_change SET state='SUCCEEDED',last_error=NULL,retry_at=NULL
		WHERE revocation_id=? AND actor_id=? AND scope_key=? AND epoch=? AND state='FENCING'`, payload.Revocation, payload.User, payload.Scope, payload.Epoch)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
````

## 第 27 步：AccountCloseFences.Finish（接着实现）

插入位置：第 26 步代码之后，紧接追加；完成稿约第 286 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,instance_id=NULL,last_error=NULL 		WHERE id=? AND kind=? AND operation_id=? AND business_key=? AND g …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(changed, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=14800:15230 -->
````go
	changed, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,instance_id=NULL,last_error=NULL
		WHERE id=? AND kind=? AND operation_id=? AND business_key=? AND generation=? AND instance_id=? AND state='RUNNING' AND lease_until>UTC_TIMESTAMP(6)`, task.ID, domain.AccountCloseFenceKind, task.Operation, task.Revocation, task.Generation, task.Instance)
	return accountCloseChanged(changed, err)
}

````

## 第 28 步：AccountCloseFences.InvalidTasks

插入位置：第 27 步代码之后，紧接追加；完成稿约第 291 行。

InvalidTasks只分页联查本worker接管的未完成任务，既不放过绑定损坏，也不逐行查询旧任务队列。

本步声明：`func (r *AccountCloseFences) InvalidTasks(ctx context.Context) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, `SELECT `+accountCloseFenceTaskColumns+`,`+accountCloseFenceFactsColumns+` 			FROM durable_task t `+accountCloseFenceFactsJoins+` WHERE t.kind=? AND t.state  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryContext(ctx, `SELECT `+accountCloseFenceTaskColumns+`,`+accountCloseFenceFactsColumns+` 			FROM durable_task t `+accountCloseFenceFactsJoins+` WHERE t.kind=? AND t.state  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(destinations...)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return false, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>task.State != &quot;QUEUED&quot; &amp;&amp; task.State != &quot;RETRY&quot; &amp;&amp; task.State != &quot;UNKNOWN&quot; &amp;&amp; task.State != &quot;RUNNING&quot;</code>。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>count &lt; 100</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。

<!-- write-step: 28 bytes=15230:16755 -->
````go
// InvalidTasks只分页联查本worker接管的未完成任务，既不放过绑定损坏，也不逐行查询旧任务队列。
func (r *AccountCloseFences) InvalidTasks(ctx context.Context) (bool, error) {
	after := uint64(0)
	for {
		rows, err := r.db.QueryContext(ctx, `SELECT `+accountCloseFenceTaskColumns+`,`+accountCloseFenceFactsColumns+`
			FROM durable_task t `+accountCloseFenceFactsJoins+` WHERE t.kind=? AND t.state NOT IN ('SUCCEEDED','REJECTED') AND t.id>? ORDER BY t.id LIMIT 100`, domain.AccountCloseFenceKind, after)
		if err != nil {
			return false, err
		}
		count := 0
		for rows.Next() {
			var task domain.AccountCloseFenceTask
			var facts accountCloseFenceFacts
			destinations := []any{&task.ID, &task.Generation, &task.Operation, &task.Revocation, &task.State, &task.Instance, &task.Payload, &task.LeaseValid}
			destinations = append(destinations, accountCloseFenceFactDestinations(&facts)...)
			if err = rows.Scan(destinations...); err != nil {
				rows.Close()
				return false, err
			}
			if task.State != "QUEUED" && task.State != "RETRY" && task.State != "UNKNOWN" && task.State != "RUNNING" {
				rows.Close()
				return true, nil
			}
			if _, err = validateAccountCloseFence(task, facts); err != nil {
				rows.Close()
				return true, nil
			}
			after, count = task.ID, count+1
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return false, err
		}
		if count < 100 {
			return false, nil
		}
	}
}

// account_close_freeze.go：保留原实现，按业务合并文件。

````

## 第 29 步：AccountClosures.SavePreparation

插入位置：第 28 步代码之后，紧接追加；完成稿约第 333 行。

SavePreparation允许在保持PREPARING的前提下持久化交接要求或下游固定计划。
代次每次推进让相同JSON重存仍有明确CAS结果，不把MySQL的零变更误判为不存在。

本步声明：`func (r *AccountClosures) SavePreparation(ctx context.Context, tx *sql.Tx, operation string, captureJSON, resultJSON []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE account_close SET retention_plan=?,generation=generation+1 		WHERE operation_id=? AND state=&#x27;PREPARING&#x27; AND generation&lt;18446744073709551615`, string(captureJSON), operation)</code>；<code>tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1 		WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PREPARING&#x27; AND generation&lt;1844674 …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE account_close SET retention_plan=?,generation=generation+1 		WHERE operation_id=? AND state=&#x27;PREPARING&#x27; AND generation&lt;18446744073709551615`, string(captureJSON), operation)</code>；<code>tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1 		WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PREPARING&#x27; AND generation&lt;1844674 …</code>（长表达式，按本步完整代码继续） |
| <code>captureJSON</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(captureJSON)</code> |
| <code>resultJSON</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(resultJSON)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>accountCloseChanged(changed, err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE account_close SET retention_plan=?,generation=generation+1 		WHERE operation_id=? AND state=&#x27;PREPARING&#x27; AND generation&lt;18446744073709551615`, string(captureJSON), operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1 		WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PREPARING&#x27; AND generation&lt;1844674 …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(changed, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=16755:17678 -->
````go
// SavePreparation允许在保持PREPARING的前提下持久化交接要求或下游固定计划。
// 代次每次推进让相同JSON重存仍有明确CAS结果，不把MySQL的零变更误判为不存在。
func (r *AccountClosures) SavePreparation(ctx context.Context, tx *sql.Tx, operation string, captureJSON, resultJSON []byte) error {
	changed, err := tx.ExecContext(ctx, `UPDATE account_close SET retention_plan=?,generation=generation+1
		WHERE operation_id=? AND state='PREPARING' AND generation<18446744073709551615`, string(captureJSON), operation)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1
		WHERE operation_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING' AND generation<18446744073709551615`, string(resultJSON), operation)
	return accountCloseChanged(changed, err)
}

````

## 第 30 步：AccountClosures.Freeze

插入位置：第 29 步代码之后，紧接追加；完成稿约第 346 行。

Freeze必须由已复核凭据/组织快照、且已确认Doc同一固定计划FROZEN的用例调用。
全部来源冻结、通知下游的持久任务与状态推进使用同一事务；不在此宣称注销完成。

本步声明：`func (r *AccountClosures) Freeze(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture, result []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE user SET state=&#x27;REVOKING&#x27;,identity_epoch=identity_epoch+1 		WHERE id=? AND state=&#x27;ACTIVE&#x27; AND identity_epoch=? AND credential_epoch=? AND identity_epoch …</code>（长表达式，按本步完整代码继续）；<code>freezeAccountCloseSessions(ctx, tx, binding.User)</code>；<code>tx.ExecContext(ctx, `UPDATE organization_member SET state=&#x27;REVOKING&#x27;,epoch=epoch+1,revision=revision+1 			WHERE organization_id=? AND user_id=? AND role=? AND state=&#x27;ACTIVE&#x27; AND ep …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE organization SET lifecycle=&#x27;CLOSING&#x27;,revision=revision+1,member_revision=member_revision+1 				WHERE id=? AND owner_id=? AND lifecycle=? AND revision=?  …</code>（长表达式，按本步完整代码继续）<br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>freezeAccountCloseSessions(ctx, tx, binding.User)</code>；<code>freezeAccountCloseScope(ctx, tx, binding, scope)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>validateAccountCloseFreeze(binding, capture)</code>；<code>binding.User</code>；<code>binding.IdentityEpoch</code>；<code>binding.ClosingOrganizations</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>capture</code> | <code>domain.AccountCloseCapture</code> | 使用 <code>domain.AccountCloseCapture</code> 类型；其定义处的职责：AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。 Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。<br>在此声明中的实际使用：<code>validateAccountCloseFreeze(binding, capture)</code>；<code>capture.CredentialEpoch</code>；<code>capture.Organizations</code>；<code>capture.Scopes</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_MEMBERSHIP_CHANGED&quot;)</code>；<code>accountCloseChanged(changed, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE user SET state=&#x27;REVOKING&#x27;,identity_epoch=identity_epoch+1 		WHERE id=? AND state=&#x27;ACTIVE&#x27; AND identity_epoch=? AND credential_epoch=? AND identity_epoch …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>org.MemberState != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_MEMBERSHIP_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE organization_member SET state=&#x27;REVOKING&#x27;,epoch=epoch+1,revision=revision+1 			WHERE organization_id=? AND user_id=? AND role=? AND state=&#x27;ACTIVE&#x27; AND ep …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>closing&#91;org.ID&#93;</code>。
- 执行 <code>tx.ExecContext(ctx, `UPDATE organization SET lifecycle=&#x27;CLOSING&#x27;,revision=revision+1,member_revision=member_revision+1 				WHERE id=? AND owner_id=? AND lifecycle=? AND revision=?  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, `UPDATE organization SET member_revision=member_revision+1 				WHERE id=? AND owner_id=? AND lifecycle=? AND revision=? AND member_revision=? AND member_revisio …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 30 bytes=17678:20192 -->
````go
// Freeze必须由已复核凭据/组织快照、且已确认Doc同一固定计划FROZEN的用例调用。
// 全部来源冻结、通知下游的持久任务与状态推进使用同一事务；不在此宣称注销完成。
func (r *AccountClosures) Freeze(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture, result []byte) error {
	if err := validateAccountCloseFreeze(binding, capture); err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, `UPDATE user SET state='REVOKING',identity_epoch=identity_epoch+1
		WHERE id=? AND state='ACTIVE' AND identity_epoch=? AND credential_epoch=? AND identity_epoch<18446744073709551615`, binding.User, binding.IdentityEpoch, capture.CredentialEpoch)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	if err = freezeAccountCloseSessions(ctx, tx, binding.User); err != nil {
		return err
	}
	closing := map[uint64]bool{}
	for _, id := range binding.ClosingOrganizations {
		closing[id] = true
	}
	for _, org := range capture.Organizations {
		if org.MemberState != "ACTIVE" {
			return errors.New("ACCOUNT_CLOSE_MEMBERSHIP_CHANGED")
		}
		changed, err = tx.ExecContext(ctx, `UPDATE organization_member SET state='REVOKING',epoch=epoch+1,revision=revision+1
			WHERE organization_id=? AND user_id=? AND role=? AND state='ACTIVE' AND epoch=? AND revision=?
			AND epoch<18446744073709551615 AND revision<18446744073709551615`, org.ID, binding.User, org.Role, org.Epoch, org.MembershipRevision)
		if err = accountCloseChanged(changed, err); err != nil {
			return err
		}
		if closing[org.ID] {
			changed, err = tx.ExecContext(ctx, `UPDATE organization SET lifecycle='CLOSING',revision=revision+1,member_revision=member_revision+1
				WHERE id=? AND owner_id=? AND lifecycle=? AND revision=? AND member_revision=?
				AND revision<18446744073709551615 AND member_revision<18446744073709551615`, org.ID, org.Owner, org.Lifecycle, org.Revision, org.MemberRevision)
		} else {
			changed, err = tx.ExecContext(ctx, `UPDATE organization SET member_revision=member_revision+1
				WHERE id=? AND owner_id=? AND lifecycle=? AND revision=? AND member_revision=? AND member_revision<18446744073709551615`, org.ID, org.Owner, org.Lifecycle, org.Revision, org.MemberRevision)
		}
		if err = accountCloseChanged(changed, err); err != nil {
			return err
		}
	}
	for _, scope := range capture.Scopes {
		if err = freezeAccountCloseScope(ctx, tx, binding, scope); err != nil {
			return err
		}
	}
````

## 第 31 步：AccountClosures.Freeze（接着实现）

插入位置：第 30 步代码之后，紧接追加；完成稿约第 391 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE account_close SET state=&#x27;FROZEN&#x27;,retention_plan=?,generation=generation+1,retry_at=UTC_TIMESTAMP(6) 		WHERE operation_id=? AND user_id=? AND state=&#x27;PREP …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1 		WHERE operation_id=? AND actor_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PREPARING&#x27; AND gen …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(changed, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=20192:20942 -->
````go
	raw, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE account_close SET state='FROZEN',retention_plan=?,generation=generation+1,retry_at=UTC_TIMESTAMP(6)
		WHERE operation_id=? AND user_id=? AND state='PREPARING' AND generation<18446744073709551615`, string(raw), binding.Operation, binding.User)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1
		WHERE operation_id=? AND actor_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING' AND generation<18446744073709551615`, string(result), binding.Operation, binding.User)
	return accountCloseChanged(changed, err)
}

````

## 第 32 步：validateAccountCloseFreeze

插入位置：第 31 步代码之后，紧接追加；完成稿约第 405 行。

本步声明：`func validateAccountCloseFreeze(binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>platform.ValidateAccountClosureBinding(binding)</code>；<code>platform.EqualAccountClosureBinding(*capture.Binding, binding)</code>；<code>binding.IdentityEpoch</code>；<code>binding.User</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>capture</code> | <code>domain.AccountCloseCapture</code> | 使用 <code>domain.AccountCloseCapture</code> 类型；其定义处的职责：AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。 Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。<br>在此声明中的实际使用：<code>capture.Binding</code>；<code>capture.IdentityEpoch</code>；<code>capture.CredentialEpoch</code>；<code>capture.Organizations</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.ValidateAccountClosureBinding(binding) &#124;&#124; capture.Binding == nil &#124;&#124; !platform.EqualAccountClosureBinding(*capture.Binding, binding) &#124;&#124; capture.IdentityEpoch != binding.Id …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>org.ID &lt;= previous &#124;&#124; org.Epoch == 0 &#124;&#124; org.Revision == 0 &#124;&#124; org.MemberRevision == 0 &#124;&#124; org.MembershipRevision == 0 &#124;&#124; org.MemberState != &quot;ACTIVE&quot; &#124;&#124; (org.Lifecycle != &quot;ACTIVE&quot; &amp;&amp; org.Lifecycle != &quot;CLOSING&quot;)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!organizations&#91;id&#93;</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>len(capture.Scopes) != len(expected)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(scope.Revocation) &#124;&#124; revocations&#91;scope.Revocation&#93; &#124;&#124; expected&#91;scope.Scope&#93; == 0 &#124;&#124; expected&#91;scope.Scope&#93; != scope.Epoch</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>scope.Scope == &quot;identity:&quot;+user &amp;&amp; scope.Revocation != binding.Operation</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=20942:22569 -->
````go
func validateAccountCloseFreeze(binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) error {
	invalid := errors.New("ACCOUNT_CLOSE_FREEZE_BINDING_INVALID")
	if !platform.ValidateAccountClosureBinding(binding) || capture.Binding == nil || !platform.EqualAccountClosureBinding(*capture.Binding, binding) || capture.IdentityEpoch != binding.IdentityEpoch || capture.CredentialEpoch == 0 {
		return invalid
	}
	user := strconv.FormatUint(binding.User, 10)
	expected := map[string]uint64{"identity:" + user: binding.IdentityEpoch}
	organizations := map[uint64]bool{}
	previous := uint64(0)
	for _, org := range capture.Organizations {
		if org.ID <= previous || org.Epoch == 0 || org.Revision == 0 || org.MemberRevision == 0 || org.MembershipRevision == 0 || org.MemberState != "ACTIVE" || (org.Lifecycle != "ACTIVE" && org.Lifecycle != "CLOSING") {
			return invalid
		}
		previous = org.ID
		organizations[org.ID] = true
		expected["org-member:"+strconv.FormatUint(org.ID, 10)+":"+user] = org.Epoch
	}
	for _, id := range binding.ClosingOrganizations {
		if !organizations[id] {
			return invalid
		}
	}
	if len(capture.Scopes) != len(expected) {
		return invalid
	}
	revocations := map[string]bool{}
	for _, scope := range capture.Scopes {
		if !domain.ValidOperationID(scope.Revocation) || revocations[scope.Revocation] || expected[scope.Scope] == 0 || expected[scope.Scope] != scope.Epoch {
			return invalid
		}
		if scope.Scope == "identity:"+user && scope.Revocation != binding.Operation {
			return invalid
		}
		revocations[scope.Revocation] = true
		delete(expected, scope.Scope)
	}
	return nil
}

````

## 第 33 步：freezeAccountCloseSessions

插入位置：第 32 步代码之后，紧接追加；完成稿约第 444 行。

本步声明：`func freezeAccountCloseSessions(ctx context.Context, tx *sql.Tx, user uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT epoch FROM session WHERE user_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY id FOR UPDATE&quot;, user)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE session SET state=&#x27;REVOKING&#x27;,epoch=epoch+1 WHERE user_id=? AND state=&#x27;ACTIVE&#x27; AND epoch&lt;18446744073709551615&quot;, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code>；<code>tx.ExecContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT epoch FROM session WHERE user_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY id FOR UPDATE&quot;, user)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE session SET state=&#x27;REVOKING&#x27;,epoch=epoch+1 WHERE user_id=? AND state=&#x27;ACTIVE&#x27; AND epoch&lt;18446744073709551615&quot;, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_SESSION_EPOCH_EXHAUSTED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_SESSION_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, &quot;SELECT epoch FROM session WHERE user_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY id FOR UPDATE&quot;, user)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>epoch == ^uint64(0)</code>。
- 此处通过 <code>return errors.New(&quot;ACCOUNT_CLOSE_SESSION_EPOCH_EXHAUSTED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE session SET state=&#x27;REVOKING&#x27;,epoch=epoch+1 WHERE user_id=? AND state=&#x27;ACTIVE&#x27; AND epoch&lt;18446744073709551615&quot;, user)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>actual != count</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_SESSION_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=22569:23497 -->
````go
func freezeAccountCloseSessions(ctx context.Context, tx *sql.Tx, user uint64) error {
	rows, err := tx.QueryContext(ctx, "SELECT epoch FROM session WHERE user_id=? AND state='ACTIVE' ORDER BY id FOR UPDATE", user)
	if err != nil {
		return err
	}
	count := int64(0)
	for rows.Next() {
		var epoch uint64
		if err = rows.Scan(&epoch); err != nil {
			rows.Close()
			return err
		}
		if epoch == ^uint64(0) {
			rows.Close()
			return errors.New("ACCOUNT_CLOSE_SESSION_EPOCH_EXHAUSTED")
		}
		count++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, "UPDATE session SET state='REVOKING',epoch=epoch+1 WHERE user_id=? AND state='ACTIVE' AND epoch<18446744073709551615", user)
	if err != nil {
		return err
	}
	actual, err := changed.RowsAffected()
	if err != nil {
		return err
	}
	if actual != count {
		return errors.New("ACCOUNT_CLOSE_SESSION_CHANGED")
	}
	return nil
}

````

## 第 34 步：freezeAccountCloseScope

插入位置：第 33 步代码之后，紧接追加；完成稿约第 481 行。

本步声明：`func freezeAccountCloseScope(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, scope domain.AccountCloseScope) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) 		ON DUPLICATE KEY UPDATE scope_key=scope_key`, scope.Scope, scope.Epoch)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state,COALESCE(last_revocation_id,&#x27;&#x27;) FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope.Scope)</code>；<code>tx.ExecContext(ctx, `UPDATE access_gate SET max_epoch=?,state=&#x27;BLOCKED&#x27;,last_revocation_id=?,revision=revision+1 		WHERE scope_key=? AND revision&lt;18446744073709551615`, scope.Epoch, scope.Revocation, scope.Scope)</code>；<code>tx.ExecContext(ctx, `UPDATE identity_admission i JOIN admission_source s ON s.admission_id=i.admission_id 		SET i.state=&#x27;CLOSED&#x27; WHERE s.scope_key=? AND s.epoch&lt;=? AND i.state=&#x27;OPEN&#x27;`, scope.Scope, scope.Epoch)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>binding.Operation</code>；<code>binding.User</code> |
| <code>scope</code> | <code>domain.AccountCloseScope</code> | 使用 <code>domain.AccountCloseScope</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>scope.Scope</code>；<code>scope.Epoch</code>；<code>scope.Revocation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_SOURCE_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) 		ON DUPLICATE KEY UPDATE scope_key=scope_key`, scope.Scope, scope.Epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state,COALESCE(last_revocation_id,&#x27;&#x27;) FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope.Scope).Scan(&amp;epoch, &amp;state, &amp;revocation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state,COALESCE(last_revocation_id,&#x27;&#x27;) FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope.Scope)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>epoch &gt; scope.Epoch &#124;&#124; (epoch == scope.Epoch &amp;&amp; state == &quot;BLOCKED&quot; &amp;&amp; revocation != scope.Revocation)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE access_gate SET max_epoch=?,state=&#x27;BLOCKED&#x27;,last_revocation_id=?,revision=revision+1 		WHERE scope_key=? AND revision&lt;18446744073709551615`, scope.Epoch, scope.Revocation, scope.Scope)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE identity_admission i JOIN admission_source s ON s.admission_id=i.admission_id 		SET i.state=&#x27;CLOSED&#x27; WHERE s.scope_key=? AND s.epoch&lt;=? AND i.state=&#x27;OPEN&#x27;`, scope.Scope, scope.Epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at) 		VALUES(?,?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&quot;docu …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;ACCOUNT_CLOSE_FENCE&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))`, scope.Revocation, binding.Operation, string(payload))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 34 bytes=23497:26099 -->
````go
func freezeAccountCloseScope(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, scope domain.AccountCloseScope) error {
	// 先建立门禁占位再当前读核对，绝不盲目覆盖另一个相同代次的冻结操作。
	_, err := tx.ExecContext(ctx, `INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'OPEN')
		ON DUPLICATE KEY UPDATE scope_key=scope_key`, scope.Scope, scope.Epoch)
	if err != nil {
		return err
	}
	var epoch uint64
	var state, revocation string
	err = tx.QueryRowContext(ctx, "SELECT max_epoch,state,COALESCE(last_revocation_id,'') FROM access_gate WHERE scope_key=? FOR UPDATE", scope.Scope).Scan(&epoch, &state, &revocation)
	if err != nil {
		return err
	}
	if epoch > scope.Epoch || (epoch == scope.Epoch && state == "BLOCKED" && revocation != scope.Revocation) {
		return errors.New("ACCOUNT_CLOSE_SOURCE_CHANGED")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE access_gate SET max_epoch=?,state='BLOCKED',last_revocation_id=?,revision=revision+1
		WHERE scope_key=? AND revision<18446744073709551615`, scope.Epoch, scope.Revocation, scope.Scope)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	// 反向来源覆盖作为操作者和作为目标人的证明；目标身份来源的登记由准入路径负责。
	_, err = tx.ExecContext(ctx, `UPDATE identity_admission i JOIN admission_source s ON s.admission_id=i.admission_id
		SET i.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=? AND i.state='OPEN'`, scope.Scope, scope.Epoch)
	if err != nil {
		return err
	}
	target, err := json.Marshal(map[string]any{"account_close_operation": binding.Operation, "scope_key": scope.Scope, "epoch": scope.Epoch})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at)
		VALUES(?,?,?,?,?,'FENCING','["document"]',UTC_TIMESTAMP(6))`, scope.Revocation, binding.User, scope.Scope, scope.Epoch, string(target))
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"operation_id": binding.Operation, "user_id": binding.User, "revocation_id": scope.Revocation, "scope_key": scope.Scope, "epoch": scope.Epoch})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('ACCOUNT_CLOSE_FENCE',?,?,?,'QUEUED',UTC_TIMESTAMP(6))`, scope.Revocation, binding.Operation, string(payload))
	return err
}

// account_close_organization.go：保留原实现，按业务合并文件。

````

## 第 35 步：AccountClosures.UserForOperation

插入位置：第 34 步代码之后，紧接追加；完成稿约第 528 行。

UserForOperation仅用于找到串行锁的用户编号；调用方持user锁后必须重新读取原关闭记录。

本步声明：`func (r *AccountClosures) UserForOperation(ctx context.Context, operation string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT user_id FROM account_close WHERE operation_id=?&quot;, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT user_id FROM account_close WHERE operation_id=?&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>user</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT user_id FROM account_close WHERE operation_id=?&quot;, operation).Scan(&amp;user)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT user_id FROM account_close WHERE operation_id=?&quot;, operation)</code>。
- 此处通过 <code>return user, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=26099:26472 -->
````go
// UserForOperation仅用于找到串行锁的用户编号；调用方持user锁后必须重新读取原关闭记录。
func (r *AccountClosures) UserForOperation(ctx context.Context, operation string) (uint64, error) {
	var user uint64
	err := r.db.QueryRowContext(ctx, "SELECT user_id FROM account_close WHERE operation_id=?", operation).Scan(&user)
	return user, err
}

````

## 第 36 步：AccountClosures.LockOrganizations

插入位置：第 35 步代码之后，紧接追加；完成稿约第 535 行。

LockOrganizations由已持有user锁的调用方使用。发现阶段不锁成员行，随后逐个
按组织编号取组织锁，避免两个成员分别持有自身成员行后再交叉等待组织锁。

本步声明：`func (r *AccountClosures) LockOrganizations(ctx context.Context, tx *sql.Tx, user uint64) ([]domain.AccountCloseOrganization, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT id FROM organization WHERE lifecycle IN (&#x27;ACTIVE&#x27;,&#x27;CLOSING&#x27;) AND 		(owner_id=? OR id IN (SELECT organization_id FROM organization_member WHERE user_id= …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT owner_id,revision,member_revision,lifecycle FROM organization WHERE id=? FOR UPDATE`, id)</code>；<code>tx.QueryContext(ctx, `SELECT user_id,role,state,epoch,revision FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code>；<code>tx.QueryRowContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT id FROM organization WHERE lifecycle IN (&#x27;ACTIVE&#x27;,&#x27;CLOSING&#x27;) AND 		(owner_id=? OR id IN (SELECT organization_id FROM organization_member WHERE user_id= …</code>（长表达式，按本步完整代码继续）；<code>member == user</code>；<code>org.Owner == user</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.AccountCloseOrganization</code> | 使用 <code>domain.AccountCloseOrganization</code> 类型；其定义处的职责：MemberRevision是组织成员集合水位，MembershipRevision是本人那一行的水位， 两者都固定才能在关闭前发现其他成员加入与本人角色变化。<br>实际返回表达式：<code>nil</code>；<code>organizations</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT id FROM organization WHERE lifecycle IN (&#x27;ACTIVE&#x27;,&#x27;CLOSING&#x27;) AND 		(owner_id=? OR id IN (SELECT organization_id FROM organization_member WHERE user_id= …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT owner_id,revision,member_revision,lifecycle FROM organization WHERE id=? FOR UPDATE`, id)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>org.Lifecycle != &quot;ACTIVE&quot; &amp;&amp; org.Lifecycle != &quot;CLOSING&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 执行 <code>tx.QueryContext(ctx, `SELECT user_id,role,state,epoch,revision FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE`, id)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 36 bytes=26472:28260 -->
````go
// LockOrganizations由已持有user锁的调用方使用。发现阶段不锁成员行，随后逐个
// 按组织编号取组织锁，避免两个成员分别持有自身成员行后再交叉等待组织锁。
func (r *AccountClosures) LockOrganizations(ctx context.Context, tx *sql.Tx, user uint64) ([]domain.AccountCloseOrganization, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM organization WHERE lifecycle IN ('ACTIVE','CLOSING') AND
		(owner_id=? OR id IN (SELECT organization_id FROM organization_member WHERE user_id=? AND state IN ('ACTIVE','REVOKING')))
		ORDER BY id`, user, user)
	if err != nil {
		return nil, err
	}
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	organizations := []domain.AccountCloseOrganization{}
	for _, id := range ids {
		org := domain.AccountCloseOrganization{ID: id}
		err = tx.QueryRowContext(ctx, `SELECT owner_id,revision,member_revision,lifecycle FROM organization WHERE id=? FOR UPDATE`, id).
			Scan(&org.Owner, &org.Revision, &org.MemberRevision, &org.Lifecycle)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if org.Lifecycle != "ACTIVE" && org.Lifecycle != "CLOSING" {
			continue
		}
		// 当前锁定读不受发现阶段RR快照影响，等待组织锁后必须看到刚提交的角色与成员状态。
		rows, err = tx.QueryContext(ctx, `SELECT user_id,role,state,epoch,revision FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE`, id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var member, epoch, revision uint64
			var role, state string
````

## 第 37 步：AccountClosures.LockOrganizations（接着实现）

插入位置：第 36 步代码之后，紧接追加；完成稿约第 580 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;member, &amp;role, &amp;state, &amp;epoch, &amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>state == &quot;ACTIVE&quot; &#124;&#124; state == &quot;REVOKING&quot;</code>。
- 检查 <code>state == &quot;ACTIVE&quot; &amp;&amp; (role == &quot;ADMIN&quot; &#124;&#124; member == org.Owner)</code>。
- 检查 <code>member == user</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>org.Owner == user &#124;&#124; org.MemberState == &quot;ACTIVE&quot; &#124;&#124; org.MemberState == &quot;REVOKING&quot;</code>。
- 此处通过 <code>return organizations, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=28260:28940 -->
````go
			if err = rows.Scan(&member, &role, &state, &epoch, &revision); err != nil {
				rows.Close()
				return nil, err
			}
			if state == "ACTIVE" || state == "REVOKING" {
				org.LiveMembers++
			}
			if state == "ACTIVE" && (role == "ADMIN" || member == org.Owner) {
				org.AvailableAdministrators++
			}
			if member == user {
				org.Role, org.MemberState, org.Epoch, org.MembershipRevision = role, state, epoch, revision
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if org.Owner == user || org.MemberState == "ACTIVE" || org.MemberState == "REVOKING" {
			organizations = append(organizations, org)
		}
	}
	return organizations, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/account_close_preparation_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/06-account-close-preparation.md)。
