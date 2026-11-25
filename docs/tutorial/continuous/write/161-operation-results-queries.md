# 分步编写 `services/document/internal/repository/document/operation_results_queries.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/161-operation-results-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现幂等操作门闩、固定回执、准入来源索引及事务内搜索outbox。

相同操作号只能对应同一发起人、动作和请求指纹；来源与结果必须同事务持久化。

写之前先按这条执行路线理解各段的连接：

1. LockOperationGate串行化原号
2. LockedOperation查固定结果
3. InsertOperation登记
4. InsertAdmissionSource建立反向来源
5. InsertIndexEvent写入业务事务。

本文件需要守住的边界：

- Kafka发送由Canal读取已提交binlog完成，不能在COMMIT前发送
- Queryer不能暗自另开事务。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/operation_results_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/operation_results_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/operation_results_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/operation_results_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:313 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// operation.go：保留原实现，按业务合并文件。

````

## 第 2 步：Operations

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`type Operations struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 2 bytes=313:351 -->
````go
type Operations struct{ db *sql.DB }

````

## 第 3 步：NewOperations

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`func NewOperations(db *sql.DB) *Operations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Operations</code> | 实际返回表达式：<code>&amp;Operations{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Operations{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=351:425 -->
````go
func NewOperations(db *sql.DB) *Operations { return &Operations{db: db} }
````

## 第 4 步：Operations.Exists

插入位置：第 3 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`func (r *Operations) Exists(ctx context.Context, id string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>OperationExists(ctx, r.db, id)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>OperationExists(ctx, r.db, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>OperationExists(ctx, r.db, id)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>OperationExists(ctx, r.db, id)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return OperationExists(ctx, r.db, id)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=425:542 -->
````go
func (r *Operations) Exists(ctx context.Context, id string) (bool, error) {
	return OperationExists(ctx, r.db, id)
}
````

## 第 5 步：OperationExists

插入位置：第 4 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func OperationExists(ctx context.Context, q Queryer, id string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT 1 FROM operation_receipt WHERE operation_id=?&quot;, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT 1 FROM operation_receipt WHERE operation_id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT 1 FROM operation_receipt WHERE operation_id=?&quot;, id).Scan(&amp;marker)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT 1 FROM operation_receipt WHERE operation_id=?&quot;, id)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=542:819 -->
````go
func OperationExists(ctx context.Context, q Queryer, id string) (bool, error) {
	var marker int
	err := q.QueryRowContext(ctx, "SELECT 1 FROM operation_receipt WHERE operation_id=?", id).Scan(&marker)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
````

## 第 6 步：LockOperationGate

插入位置：第 5 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func LockOperationGate(ctx context.Context, tx *sql.Tx, id string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, &quot;operation:&quot;+id)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, &quot;operation:&quot;+id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;operation:&quot;+id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;OPERATION_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, &quot;operation:&quot;+id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, &quot;operation:&quot;+id).Scan(&amp;revision) != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;OPERATION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, &quot;operation:&quot;+id).Scan(&amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, &quot;operation:&quot;+id)</code>。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=819:1458 -->
````go
func LockOperationGate(ctx context.Context, tx *sql.Tx, id string) error {
	// 同号空receipt也需串行：唯一gate行提供互斥，不能依赖不存在行的间隙锁。
	_, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)", "operation:"+id)
	if err != nil {
		return err
	}
	var revision uint64
	if tx.QueryRowContext(ctx, "SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE", "operation:"+id).Scan(&revision) != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "OPERATION_UNAVAILABLE")
	}
	return nil
}
````

## 第 7 步：LockedOperation

插入位置：第 6 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func LockedOperation(ctx context.Context, tx *sql.Tx, id string) (domain.OperationReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OperationReceipt</code> | 使用 <code>domain.OperationReceipt</code> 类型；其定义处的职责：OperationReceipt保存原号/发起者/请求指纹与固定结果，原结果不等于新动作资格。<br>实际返回表达式：<code>receipt</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, id).Scan(&amp;receipt.ActorID, &amp;receipt.Kind, &amp;receipt.Fingerprint, &amp;receipt.Result)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return receipt, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1458:1842 -->
````go
func LockedOperation(ctx context.Context, tx *sql.Tx, id string) (domain.OperationReceipt, error) {
	var receipt domain.OperationReceipt
	err := tx.QueryRowContext(ctx, "SELECT actor_id,kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE", id).Scan(&receipt.ActorID, &receipt.Kind, &receipt.Fingerprint, &receipt.Result)
	return receipt, err
}
````

## 第 8 步：InsertOperation

插入位置：第 7 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func InsertOperation(ctx context.Context, tx *sql.Tx, record domain.OperationRecord) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUE …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(?,?,1,?,?,?)&quot;, record.ActorID, record.ID, record.Kind, string(record.TargetJSON), string(record.ResultJSON))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>record</code> | <code>domain.OperationRecord</code> | 使用 <code>domain.OperationRecord</code> 类型；其定义处的职责：定义操作回执和读取准入，并计算结构改变后历史单元范围是否仍完整受权。<br>在此声明中的实际使用：<code>record.ID</code>；<code>record.ActorID</code>；<code>record.Kind</code>；<code>record.TargetJSON</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUE …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(?,?,1,?,?,?)&quot;, record.ActorID, record.ID, record.Kind, string(record.TargetJSON), string(record.ResultJSON))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 8 bytes=1842:2677 -->
````go
func InsertOperation(ctx context.Context, tx *sql.Tx, record domain.OperationRecord) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUES(?,?,?,?,?,?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY),IF(?='SUCCEEDED',UTC_TIMESTAMP(6),NULL))", record.ID, record.ActorID, record.Kind, string(record.TargetJSON), record.Fingerprint, record.State, record.IdentityAdmissionRef, string(record.ResultJSON), record.State)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(?,?,1,?,?,?)", record.ActorID, record.ID, record.Kind, string(record.TargetJSON), string(record.ResultJSON))
	return err
}
````

## 第 9 步：InsertReadAdmission

插入位置：第 8 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func InsertReadAdmission(ctx context.Context, tx *sql.Tx, record domain.ReadAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO document_admission(admission_id,identity_admission_ref,actor_id,session_ref,document_id,purpose,capabilities,target_json,operation_id,state,expires …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>record</code> | <code>domain.ReadAdmission</code> | 使用 <code>domain.ReadAdmission</code> 类型；其定义处的职责：ReadAdmission只保存固定证明绑定；权限来源由另一个反向索引在原事务内登记。<br>在此声明中的实际使用：<code>record.ID</code>；<code>record.IdentityAdmissionRef</code>；<code>record.ActorID</code>；<code>record.SessionRef</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO document_admission(admission_id,identity_admission_ref,actor_id,session_ref,document_id,purpose,capabilities,target_json,operation_id,state,expires …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2677:3252 -->
````go
func InsertReadAdmission(ctx context.Context, tx *sql.Tx, record domain.ReadAdmission) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO document_admission(admission_id,identity_admission_ref,actor_id,session_ref,document_id,purpose,capabilities,target_json,operation_id,state,expires_at) VALUES(?,?,?,?,?,?,?,?,?,'OPEN',FROM_UNIXTIME(?/1000))", record.ID, record.IdentityAdmissionRef, record.ActorID, record.SessionRef, record.DocumentID, record.Purpose, string(record.CapabilitiesJSON), string(record.TargetJSON), record.OperationID, record.ExpiresUnixMs)
	return err
}

````

## 第 10 步：UpdateOperationResult

插入位置：第 9 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func UpdateOperationResult(ctx context.Context, tx *sql.Tx, id, state string, result []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=?,result_json=?,terminal_at=IF(?=&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6),NULL) WHERE operation_id=?&quot;, state, string(result), state, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=?,result_json=?,terminal_at=IF(?=&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6),NULL) WHERE operation_id=?&quot;, state, string(result), state, id)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=?,result_json=?,terminal_at=IF(?=&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6),NULL) WHERE operation_id=?&quot;, state, string(result), state, id)</code> |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=?,result_json=?,terminal_at=IF(?=&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6),NULL) WHERE operation_id=?&quot;, state, string(result), state, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3252:3559 -->
````go
func UpdateOperationResult(ctx context.Context, tx *sql.Tx, id, state string, result []byte) error {
	_, err := tx.ExecContext(ctx, "UPDATE operation_receipt SET state=?,result_json=?,terminal_at=IF(?='SUCCEEDED',UTC_TIMESTAMP(6),NULL) WHERE operation_id=?", state, string(result), state, id)
	return err
}
````

## 第 11 步：InsertAdmissionSource

插入位置：第 10 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`func InsertAdmissionSource(ctx context.Context, tx *sql.Tx, id, key string, epoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)&quot;, id, key, epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)&quot;, id, key, epoch)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)&quot;, id, key, epoch)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)&quot;, id, key, epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)&quot;, id, key, epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3559:3854 -->
````go
func InsertAdmissionSource(ctx context.Context, tx *sql.Tx, id, key string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)", id, key, epoch)
	return err
}

// outbox.go：保留原实现，按业务合并文件。

````

## 第 12 步：InsertIndexEvent

插入位置：第 11 步代码之后，紧接追加；完成稿约第 68 行。

只写调用方的原业务事务。发送由Canal读取已提交binlog完成，不能在提交前发送Kafka。

本步声明：`func InsertIndexEvent(ctx context.Context, tx *sql.Tx, document, sequence uint64, eventID, eventType string, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO outbox(event_id,aggregate_kind,aggregate_id,aggregate_seq,event_type,schema_version,payload) VALUES(?,&#x27;DOCUMENT&#x27;,?,?,?,&#x27;1&#x27;,?)&quot;, eventID, document, sequence, eventType, string(payload))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO outbox(event_id,aggregate_kind,aggregate_id,aggregate_seq,event_type,schema_version,payload) VALUES(?,&#x27;DOCUMENT&#x27;,?,?,?,&#x27;1&#x27;,?)&quot;, eventID, document, sequence, eventType, string(payload))</code> |
| <code>sequence</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO outbox(event_id,aggregate_kind,aggregate_id,aggregate_seq,event_type,schema_version,payload) VALUES(?,&#x27;DOCUMENT&#x27;,?,?,?,&#x27;1&#x27;,?)&quot;, eventID, document, sequence, eventType, string(payload))</code> |
| <code>eventID</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO outbox(event_id,aggregate_kind,aggregate_id,aggregate_seq,event_type,schema_version,payload) VALUES(?,&#x27;DOCUMENT&#x27;,?,?,?,&#x27;1&#x27;,?)&quot;, eventID, document, sequence, eventType, string(payload))</code> |
| <code>eventType</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO outbox(event_id,aggregate_kind,aggregate_id,aggregate_seq,event_type,schema_version,payload) VALUES(?,&#x27;DOCUMENT&#x27;,?,?,?,&#x27;1&#x27;,?)&quot;, eventID, document, sequence, eventType, string(payload))</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO outbox(event_id,aggregate_kind,aggregate_id,aggregate_seq,event_type,schema_version,payload) VALUES(?,&#x27;DOCUMENT&#x27;,?,?,?,&#x27;1&#x27;,?)&quot;, eventID, document, sequence, eventType, string(payload))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3854:4404 -->
````go
// 只写调用方的原业务事务。发送由Canal读取已提交binlog完成，不能在提交前发送Kafka。
func InsertIndexEvent(ctx context.Context, tx *sql.Tx, document, sequence uint64, eventID, eventType string, payload []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO outbox(event_id,aggregate_kind,aggregate_id,aggregate_seq,event_type,schema_version,payload) VALUES(?,'DOCUMENT',?,?,?,'1',?)", eventID, document, sequence, eventType, string(payload))
	return err
}

// rows.go：保留原实现，按业务合并文件。

````

## 第 13 步：Queryer

插入位置：第 12 步代码之后，紧接追加；完成稿约第 76 行。

Queryer让快照与事务复核使用原连接；repository不创建或提交事务。

本步声明：`type Queryer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>QueryRowContext</code> | <code>(context.Context, string, ...any) *sql.Row</code> | 依次接收ctx、带占位符的SQL和占位符参数；返回单行句柄，实际查询/无行错误在Scan时处理。接口让连接池或原事务都可注入，但涉及锁时必须沿用原事务。 |
| <code>QueryContext</code> | <code>(context.Context, string, ...any) (*sql.Rows, error)</code> | 依次接收ctx、SQL和占位符参数，返回多行游标及错误；调用方关闭Rows、逐行Scan，并在循环后检查Rows.Err。 |

<!-- write-step: 13 bytes=4404:4647 -->
````go
// Queryer让快照与事务复核使用原连接；repository不创建或提交事务。
type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

````

## 第 14 步：DocumentRow

插入位置：第 13 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func DocumentRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.Document, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),title,CAST(space_id AS CHAR),CAST(knowledge_base_id AS CHAR),CAST(responsible_user_id AS CHAR),lifecycle,main_locked,CAST(current_st …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),title,CAST(space_id AS CHAR),CAST(knowledge_base_id AS CHAR),CAST(responsible_user_id AS CHAR),lifecycle,main_locked,CAST(current_st …</code>（长表达式，按本步完整代码继续） |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Document</code> | 实际返回表达式：<code>d</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;DOCUMENT_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),title,CAST(space_id AS CHAR),CAST(knowledge_base_id AS CHAR),CAST(responsible_user_id AS CHAR),lifecycle,main_locked,CAST(current_st …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),title,CAST(space_id AS CHAR),CAST(knowledge_base_id AS CHAR),CAST(responsible_user_id AS CHAR),lifecycle,main_locked,CAST(current_st …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil &#124;&#124; d.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return d, domain.Violation(domain.EntityNotFound, &quot;DOCUMENT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return d, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=4647:5769 -->
````go
func DocumentRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.Document, error) {
	var d dto.Document
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),title,CAST(space_id AS CHAR),CAST(knowledge_base_id AS CHAR),CAST(responsible_user_id AS CHAR),lifecycle,main_locked,CAST(current_structure_id AS CHAR),CAST(current_style_id AS CHAR),CAST(current_manifest_id AS CHAR),CAST(current_release_id AS CHAR),publication_state,CAST(control_revision AS CHAR),CAST(acl_revision AS CHAR),CAST(publication_epoch AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR) FROM document WHERE id=?"+suffix, id).Scan(&d.Id, &d.Title, &d.SpaceId, &d.KnowledgeBaseId, &d.ResponsibleUserId, &d.Lifecycle, &d.MainLocked, &d.CurrentStructureId, &d.CurrentStyleId, &d.CurrentManifestId, &d.CurrentReleaseId, &d.PublicationState, &d.ControlRevision, &d.AclRevision, &d.PublicationEpoch, &d.VisibilityPolicy, &d.VisibilityRevision)
	if err != nil || d.Lifecycle != "ACTIVE" {
		return d, domain.Violation(domain.EntityNotFound, "DOCUMENT_NOT_FOUND")
	}
	return d, nil
}
````

## 第 15 步：SpaceRow

插入位置：第 14 步代码之后，紧接追加；完成稿约第 94 行。

本步声明：`func SpaceRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.Space, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),kind,CAST(account_owner_id AS CHAR),CAST(organization_id AS CHAR),lifecycle FROM space WHERE id=?&quot;+suffix, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),kind,CAST(account_owner_id AS CHAR),CAST(organization_id AS CHAR),lifecycle FROM space WHERE id=?&quot;+suffix, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Space</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;SPACE_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),kind,CAST(account_owner_id AS CHAR),CAST(organization_id AS CHAR),lifecycle FROM space WHERE id=?&quot;+suffix, id).Scan(&amp;value.Id, &amp;valu …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),kind,CAST(account_owner_id AS CHAR),CAST(organization_id AS CHAR),lifecycle FROM space WHERE id=?&quot;+suffix, id)</code>。
- 检查 <code>err != nil &#124;&#124; value.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return value, domain.Violation(domain.EntityNotFound, &quot;SPACE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=5769:6364 -->
````go
func SpaceRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.Space, error) {
	var value dto.Space
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),kind,CAST(account_owner_id AS CHAR),CAST(organization_id AS CHAR),lifecycle FROM space WHERE id=?"+suffix, id).Scan(&value.Id, &value.Kind, &value.OwnerUserId, &value.OrganizationId, &value.Lifecycle)
	value.Actions = []string{}
	if err != nil || value.Lifecycle != "ACTIVE" {
		return value, domain.Violation(domain.EntityNotFound, "SPACE_NOT_FOUND")
	}
	return value, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/operation_results_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
