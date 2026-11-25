# 分步编写 `services/account/internal/repository/account/operation_receipts_queries.go`

先按 [A04 · 原操作回执、授权证明与平台治理任免](../lessons/account/04-receipts-governance.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/053-operation-receipts-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

查询账号操作原回执与当前凭据，为退出/改密恢复和RPC固定结果提供持久事实。

恢复权限要核对actor与原session_hash，不能按操作号直接返回敏感结果。

写之前先按这条执行路线理解各段的连接：

1. ReadOperationResult 读取actor/kind/state、原会话绑定、generation、terminal与结果字节。
2. OperationReceipts.Lookup/Result/CurrentCredential 分别服务绑定检查、结果读取和当前身份复核。

本文件需要守住的边界：

- 结果查询不得顺便创建操作；generation和terminal必须来自持久回执，不能用worker租约号代替。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/operation_receipts_queries.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/operation_receipts_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/operation_receipts_queries.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/operation_receipts_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:279 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"context"
	"database/sql"
)

// operation_receipt.go：保留原实现，按业务合并文件。

````

## 第 2 步：OperationReceipts

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`type OperationReceipts struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 2 bytes=279:324 -->
````go
type OperationReceipts struct{ db *sql.DB }

````

## 第 3 步：NewOperationReceipts

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func NewOperationReceipts(db *sql.DB) *OperationReceipts`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OperationReceipts</code> | 实际返回表达式：<code>&amp;OperationReceipts{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OperationReceipts{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=324:420 -->
````go
func NewOperationReceipts(db *sql.DB) *OperationReceipts { return &OperationReceipts{db: db} }

````

## 第 4 步：OperationReceipts.Lookup

插入位置：第 3 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`func (r *OperationReceipts) Lookup(ctx context.Context, id string) (domain.OperationReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, ` 		SELECT CAST(actor_id AS CHAR),kind,state, 		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;) 		FROM operation_receipt WHERE operation_id=?`, id)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, ` 		SELECT CAST(actor_id AS CHAR),kind,state, 		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;) 		FROM operation_receipt WHERE operation_id=?`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OperationReceipt</code> | 使用 <code>domain.OperationReceipt</code> 类型；其定义处的职责：表达原操作回执、退出会话选择和只含状态的最小进度。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, ` 		SELECT CAST(actor_id AS CHAR),kind,state, 		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;) 		FROM operation_receipt WHERE operation_id=?`, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=420:861 -->
````go
func (r *OperationReceipts) Lookup(ctx context.Context, id string) (domain.OperationReceipt, error) {
	var value domain.OperationReceipt
	err := r.db.QueryRowContext(ctx, `
		SELECT CAST(actor_id AS CHAR),kind,state,
		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),'')
		FROM operation_receipt WHERE operation_id=?`, id).
		Scan(&value.ActorID, &value.Kind, &value.State, &value.SessionHash)
	return value, err
}

````

## 第 5 步：OperationReceipts.ConcurrentLogout

插入位置：第 4 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func (r *OperationReceipts) ConcurrentLogout(ctx context.Context, tx *sql.Tx, id string) (string, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),state 		FROM operation_receipt WHERE operation_id=?`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, ` 		SELECT JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),state 		FROM operation_receipt WHERE operation_id=?`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>sessionHash</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),state 		FROM operation_receipt WHERE operation_id=?`, id).Scan(&amp;sessionHash, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, ` 		SELECT JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),state 		FROM operation_receipt WHERE operation_id=?`, id)</code>。
- 此处通过 <code>return sessionHash, state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=861:1227 -->
````go
func (r *OperationReceipts) ConcurrentLogout(ctx context.Context, tx *sql.Tx, id string) (string, string, error) {
	var sessionHash, state string
	err := tx.QueryRowContext(ctx, `
		SELECT JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),state
		FROM operation_receipt WHERE operation_id=?`, id).Scan(&sessionHash, &state)
	return sessionHash, state, err
}

````

## 第 6 步：OperationReceipts.CurrentCredential

插入位置：第 5 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`func (r *OperationReceipts) CurrentCredential(ctx context.Context, user string) (domain.Credential, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=?&quot;, user)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=?&quot;, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Credential</code> | 使用 <code>domain.Credential</code> 类型；其定义处的职责：按调用目的承载密码或状态代次快照。<br>实际返回表达式：<code>credential</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=?&quot;, user).Scan(&amp;credential.State, &amp;credential.Epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=?&quot;, user)</code>。
- 此处通过 <code>return credential, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1227:1548 -->
````go
func (r *OperationReceipts) CurrentCredential(ctx context.Context, user string) (domain.Credential, error) {
	var credential domain.Credential
	err := r.db.QueryRowContext(ctx, "SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=?", user).Scan(&credential.State, &credential.Epoch)
	return credential, err
}

````

## 第 7 步：OperationReceipts.Result

插入位置：第 6 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`func (r *OperationReceipts) Result(ctx context.Context, id string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT result_json FROM operation_receipt WHERE operation_id=?&quot;, id)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT result_json FROM operation_receipt WHERE operation_id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT result_json FROM operation_receipt WHERE operation_id=?&quot;, id).Scan(&amp;result)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT result_json FROM operation_receipt WHERE operation_id=?&quot;, id)</code>。
- 此处通过 <code>return result, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1548:1862 -->
````go
func (r *OperationReceipts) Result(ctx context.Context, id string) ([]byte, error) {
	var result []byte
	err := r.db.QueryRowContext(ctx, "SELECT result_json FROM operation_receipt WHERE operation_id=?", id).Scan(&result)
	return result, err
}

// operation_result.go：保留原实现，按业务合并文件。

````

## 第 8 步：OperationResultSnapshot

插入位置：第 7 步代码之后，紧接追加；完成稿约第 48 行。

OperationResultSnapshot包括原不可变绑定和同一行的版本/结果，不能把多次查询的
state与generation拼接；授权前后由用例核验该快照仍为同一个持久事实。

本步声明：`type OperationResultSnapshot struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>string</code> | 原回执操作者账号字符串。 |
| <code>Kind</code> | <code>string</code> | 原操作业务种类。 |
| <code>State</code> | <code>string</code> | 原回执当前状态。 |
| <code>SessionHash</code> | <code>string</code> | 会话令牌的摘要表示，用于核对会话；不能把可直接登录的原文令牌写入这类身份字段。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 已保存的结果JSON。 |
| <code>Generation</code> | <code>uint64</code> | 回执推进代次。 |
| <code>Terminal</code> | <code>bool</code> | 是否已记录终态时间；不能单凭字符串看起来成功就漏核对终态事实。 |

<!-- write-step: 8 bytes=1862:2264 -->
````go
// OperationResultSnapshot包括原不可变绑定和同一行的版本/结果，不能把多次查询的
// state与generation拼接；授权前后由用例核验该快照仍为同一个持久事实。
type OperationResultSnapshot struct {
	Actor, Kind, State, SessionHash string
	Fingerprint, Result             []byte
	Generation                      uint64
	Terminal                        bool
}

````

## 第 9 步：ReadOperationResult

插入位置：第 8 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func ReadOperationResult(ctx context.Context, db *sql.DB, operation string) (OperationResultSnapshot, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, `SELECT CAST(actor_id AS CHAR),kind,state, 	 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;), 	 request_fingerprint,generation,result_ …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>db.QueryRowContext(ctx, `SELECT CAST(actor_id AS CHAR),kind,state, 	 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;), 	 request_fingerprint,generation,result_ …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>OperationResultSnapshot</code> | 使用 <code>OperationResultSnapshot</code> 类型；其定义处的职责：OperationResultSnapshot包括原不可变绑定和同一行的版本/结果，不能把多次查询的 state与generation拼接；授权前后由用例核验该快照仍为同一个持久事实。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryRowContext(ctx, `SELECT CAST(actor_id AS CHAR),kind,state, 	 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;), 	 request_fingerprint,generation,result_ …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2264:2847 -->
````go
func ReadOperationResult(ctx context.Context, db *sql.DB, operation string) (OperationResultSnapshot, error) {
	var value OperationResultSnapshot
	err := db.QueryRowContext(ctx, `SELECT CAST(actor_id AS CHAR),kind,state,
	 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),''),
	 request_fingerprint,generation,result_json,terminal_at IS NOT NULL
	 FROM operation_receipt WHERE operation_id=?`, operation).
		Scan(&value.Actor, &value.Kind, &value.State, &value.SessionHash, &value.Fingerprint, &value.Generation, &value.Result, &value.Terminal)
	return value, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/operation_receipts_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/04-receipts-governance.md)。
