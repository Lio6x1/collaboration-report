# 分步编写 `services/document/internal/repository/document/personal_transfer_queries.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/164-personal-transfer-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存个人转移邀约、接受计划、真实计量和恢复任务。

接受未完成时Doc仍在源空间，父级清理必须同时看到目标接收计划。

写之前先按这条执行路线理解各段的连接：

1. BeginPersonalTransfer固定接受
2. ClaimPersonalTransfer领取
3. SavePersonalTransferMeasurement存计量
4. CompletePersonalTransfer迁移
5. SourcesPending阻止提前重开。

本文件需要守住的边界：

- 关闭子任务成功不等于父迁移完成
- 消费者Doc按ID排序锁，最终资源费用重算。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/personal_transfer_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/personal_transfer_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/personal_transfer_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/personal_transfer_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:328 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// personal_transfer.go：保留原实现，按业务合并文件。

````

## 第 2 步：PersonalTransferTask

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`const PersonalTransferTask = "PERSONAL_TRANSFER_RECOVER"`。

<!-- write-step: 2 bytes=328:386 -->
````go
const PersonalTransferTask = "PERSONAL_TRANSFER_RECOVER"

````

## 第 3 步：PersonalTransferRow

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func PersonalTransferRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.PersonalTransfer, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT t.id,t.document_id,t.document_title,t.source_user_id,t.target_user_id,t.source_space_id,  t.expected_control_revision,t.revision,t.state,t.offer_oper …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT t.id,t.document_id,t.document_title,t.source_user_id,t.target_user_id,t.source_space_id,  t.expected_control_revision,t.revision,t.state,t.offer_oper …</code>（长表达式，按本步完整代码继续） |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.PersonalTransfer</code> | 使用 <code>domain.PersonalTransfer</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>实际返回表达式：<code>p</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;TRANSFER_PLAN_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, `SELECT t.id,t.document_id,t.document_title,t.source_user_id,t.target_user_id,t.source_space_id,  t.expected_control_revision,t.revision,t.state,t.offer_oper …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT t.id,t.document_id,t.document_title,t.source_user_id,t.target_user_id,t.source_space_id,  t.expected_control_revision,t.revision,t.state,t.offer_oper …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return p, err</code>。当前函数在这里结束。
- 检查 <code>len(raw) &gt; 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return p, errors.New(&quot;TRANSFER_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!p.Plan.Valid()</code>。条件成立时执行 <code>return p, errors.New(&quot;TRANSFER_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return p, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=386:1413 -->
````go
func PersonalTransferRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.PersonalTransfer, error) {
	var p domain.PersonalTransfer
	var raw []byte
	var accept sql.NullString
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, `SELECT t.id,t.document_id,t.document_title,t.source_user_id,t.target_user_id,t.source_space_id,
 t.expected_control_revision,t.revision,t.state,t.offer_operation_id,t.accept_operation_id,t.fixed_plan,t.expires_at,UTC_TIMESTAMP(6)
 FROM personal_transfer t WHERE t.id=?`+suffix, id).Scan(&p.ID, &p.Document, &p.Title, &p.SourceUser, &p.TargetUser, &p.SourceSpace, &p.ExpectedControl, &p.Revision, &p.State, &p.OfferOperation, &accept, &raw, &p.Expires, &p.Now)
	if err != nil {
		return p, err
	}
	p.AcceptOperation = accept.String
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &p.Plan); err != nil {
			return p, errors.New("TRANSFER_PLAN_INVALID")
		}
		if !p.Plan.Valid() {
			return p, errors.New("TRANSFER_PLAN_INVALID")
		}
	}
	return p, nil
}

````

## 第 4 步：InsertPersonalTransfer

插入位置：第 3 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func InsertPersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransfer) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO personal_transfer(document_id,document_title,source_user_id,target_user_id,source_space_id,expected_control_revision,state,offer_operation_id,expir …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>p</code> | <code>domain.PersonalTransfer</code> | 使用 <code>domain.PersonalTransfer</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>在此声明中的实际使用：<code>p.Document</code>；<code>p.Title</code>；<code>p.SourceUser</code>；<code>p.TargetUser</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO personal_transfer(document_id,document_title,source_user_id,target_user_id,source_space_id,expected_control_revision,state,offer_operation_id,expir …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return uint64(id), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1413:1997 -->
````go
func InsertPersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransfer) (uint64, error) {
	result, err := tx.ExecContext(ctx, `INSERT INTO personal_transfer(document_id,document_title,source_user_id,target_user_id,source_space_id,expected_control_revision,state,offer_operation_id,expires_at)
 VALUES(?,?,?,?,?,?,'PENDING',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY))`, p.Document, p.Title, p.SourceUser, p.TargetUser, p.SourceSpace, p.ExpectedControl, p.OfferOperation)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

````

## 第 5 步：PersonalTransferIDs

插入位置：第 4 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`func PersonalTransferIDs(ctx context.Context, tx *sql.Tx, user uint64, direction string, last uint64, limit int) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT id FROM personal_transfer WHERE `+field+`=? AND id&gt;? ORDER BY id LIMIT ?`, user, last, limit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT id FROM personal_transfer WHERE `+field+`=? AND id&gt;? ORDER BY id LIMIT ?`, user, last, limit)</code> |
| <code>direction</code> | <code>string</code> | 在此声明中的实际使用：<code>direction == &quot;outgoing&quot;</code> |
| <code>last</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT id FROM personal_transfer WHERE `+field+`=? AND id&gt;? ORDER BY id LIMIT ?`, user, last, limit)</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT id FROM personal_transfer WHERE `+field+`=? AND id&gt;? ORDER BY id LIMIT ?`, user, last, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 检查 <code>direction == &quot;outgoing&quot;</code>。
- 执行 <code>tx.QueryContext(ctx, `SELECT id FROM personal_transfer WHERE `+field+`=? AND id&gt;? ORDER BY id LIMIT ?`, user, last, limit)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1997:2583 -->
````go
func PersonalTransferIDs(ctx context.Context, tx *sql.Tx, user uint64, direction string, last uint64, limit int) ([]uint64, error) {
	field := "target_user_id"
	if direction == "outgoing" {
		field = "source_user_id"
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM personal_transfer WHERE `+field+`=? AND id>? ORDER BY id LIMIT ?`, user, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

````

## 第 6 步：ClosePersonalTransfer

插入位置：第 5 步代码之后，紧接追加；完成稿约第 75 行。

本步声明：`func ClosePersonalTransfer(ctx context.Context, tx *sql.Tx, id, revision uint64, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=?,revision=revision+1 WHERE id=? AND revision=? AND state=&#x27;PENDING&#x27;`, state, id, revision)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=?,revision=revision+1 WHERE id=? AND revision=? AND state=&#x27;PENDING&#x27;`, state, id, revision)</code> |
| <code>revision</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=?,revision=revision+1 WHERE id=? AND revision=? AND state=&#x27;PENDING&#x27;`, state, id, revision)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=?,revision=revision+1 WHERE id=? AND revision=? AND state=&#x27;PENDING&#x27;`, state, id, revision)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRANSFER_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=?,revision=revision+1 WHERE id=? AND revision=? AND state=&#x27;PENDING&#x27;`, state, id, revision)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;TRANSFER_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2583:3028 -->
````go
func ClosePersonalTransfer(ctx context.Context, tx *sql.Tx, id, revision uint64, state string) error {
	result, err := tx.ExecContext(ctx, `UPDATE personal_transfer SET state=?,revision=revision+1 WHERE id=? AND revision=? AND state='PENDING'`, state, id, revision)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "TRANSFER_CHANGED")
	}
	return nil
}

````

## 第 7 步：TransferInterlock

插入位置：第 6 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`func TransferInterlock(ctx context.Context, tx *sql.Tx, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>DocumentWritesPending(ctx, tx, document, &quot;&quot;)</code>；<code>tx.QueryRowContext(ctx, `SELECT  EXISTS(SELECT 1 FROM resource_protection WHERE document_id=? AND state IN (&#x27;PROTECTED&#x27;,&#x27;UNKNOWN&#x27;))  OR EXISTS(SELECT 1 FROM personal_transfer WHERE …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>DocumentWritesPending(ctx, tx, document, &quot;&quot;)</code>；<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>DocumentWritesPending(ctx, tx, document, &quot;&quot;)</code>；<code>tx.QueryRowContext(ctx, `SELECT  EXISTS(SELECT 1 FROM resource_protection WHERE document_id=? AND state IN (&#x27;PROTECTED&#x27;,&#x27;UNKNOWN&#x27;))  OR EXISTS(SELECT 1 FROM personal_transfer WHERE …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;DOCUMENT_OPERATION_PENDING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DOCUMENT_OPERATION_PENDING&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT  EXISTS(SELECT 1 FROM resource_protection WHERE document_id=? AND state IN (&#x27;PROTECTED&#x27;,&#x27;UNKNOWN&#x27;))  OR EXISTS(SELECT 1 FROM personal_transfer WHERE …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT  EXISTS(SELECT 1 FROM resource_protection WHERE document_id=? AND state IN (&#x27;PROTECTED&#x27;,&#x27;UNKNOWN&#x27;))  OR EXISTS(SELECT 1 FROM personal_transfer WHERE …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>blocked != 0</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DOCUMENT_OPERATION_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3028:3851 -->
````go
func TransferInterlock(ctx context.Context, tx *sql.Tx, document uint64) error {
	pending, err := DocumentWritesPending(ctx, tx, document, "")
	if err != nil {
		return err
	}
	if pending {
		return domain.Violation(domain.StateConflict, "DOCUMENT_OPERATION_PENDING")
	}
	var blocked int
	// 已登记的未知正文/结构任务必须先结束；不能固定旧版本后让迟到的 COMMIT 改写转让范围。
	err = tx.QueryRowContext(ctx, `SELECT
 EXISTS(SELECT 1 FROM resource_protection WHERE document_id=? AND state IN ('PROTECTED','UNKNOWN'))
 OR EXISTS(SELECT 1 FROM personal_transfer WHERE document_id=? AND state='ACCEPTING')`, document, document).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked != 0 {
		return domain.Violation(domain.StateConflict, "DOCUMENT_OPERATION_PENDING")
	}
	return nil
}

````

## 第 8 步：TransferBodyCapacity

插入位置：第 7 步代码之后，紧接追加；完成稿约第 108 行。

本步声明：`func TransferBodyCapacity(ctx context.Context, tx *sql.Tx, document uint64) (uint64, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document)</code>；<code>tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document)</code>；<code>tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>total</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>paid</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document).Scan(&amp;total)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document)</code>。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document).Scan(&amp;paid)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document)</code>。
- 检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return total, paid, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3851:4549 -->
````go
func TransferBodyCapacity(ctx context.Context, tx *sql.Tx, document uint64) (uint64, uint64, error) {
	var total, paid uint64
	// 版本 byte_size 由 canonical 编码器写入；计入全部历史，不只当前 manifest。
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document).Scan(&total); err != nil {
		return 0, 0, err
	}
	err := tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document).Scan(&paid)
	if err != nil && err != sql.ErrNoRows {
		return 0, 0, err
	}
	return total, paid, nil
}

````

## 第 9 步：BeginPersonalTransfer

插入位置：第 8 步代码之后，紧接追加；完成稿约第 121 行。

本步声明：`func BeginPersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=&#x27;ACCEPTING&#x27;,revision=revision+1,accept_operation_id=?,fixed_plan=? WHERE id=? AND state=&#x27;PENDING&#x27;`, p.Operation, string(raw), p.Transfer)</code>；<code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,control_revision=?,acl_revision=?,publication_epoch=?,publication_state=&#x27;UNPUBLISHED&#x27;,current_release_id=NULL,  visibil …</code>（长表达式，按本步完整代码继续）；<code>InsertSubmissionTask(ctx, tx, PersonalTransferTask, p.Operation, p.Operation, raw, &quot;QUEUED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>InsertSubmissionTask(ctx, tx, PersonalTransferTask, p.Operation, p.Operation, raw, &quot;QUEUED&quot;)</code> |
| <code>p</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>在此声明中的实际使用：<code>p.Valid</code>；<code>json.Marshal(p)</code>；<code>p.Operation</code>；<code>p.Transfer</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;TRANSFER_PLAN_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRANSFER_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;DOCUMENT_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!p.Valid()</code>。条件成立时执行 <code>return errors.New(&quot;TRANSFER_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=&#x27;ACCEPTING&#x27;,revision=revision+1,accept_operation_id=?,fixed_plan=? WHERE id=? AND state=&#x27;PENDING&#x27;`, p.Operation, string(raw), p.Transfer)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;TRANSFER_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,control_revision=?,acl_revision=?,publication_epoch=?,publication_state=&#x27;UNPUBLISHED&#x27;,current_release_id=NULL,  visibil …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, PersonalTransferTask, p.Operation, p.Operation, raw, &quot;QUEUED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4549:5886 -->
````go
func BeginPersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) error {
	if !p.Valid() {
		return errors.New("TRANSFER_PLAN_INVALID")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE personal_transfer SET state='ACCEPTING',revision=revision+1,accept_operation_id=?,fixed_plan=? WHERE id=? AND state='PENDING'`, p.Operation, string(raw), p.Transfer)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "TRANSFER_CHANGED")
	}
	result, err = tx.ExecContext(ctx, `UPDATE document SET lifecycle='CLOSING',control_revision=?,acl_revision=?,publication_epoch=?,publication_state='UNPUBLISHED',current_release_id=NULL,
 visibility_policy='COLLABORATORS',visibility_revision=visibility_revision+1
 WHERE id=? AND lifecycle='ACTIVE' AND control_revision=? AND space_id=? AND responsible_user_id=?`, p.ClosingControl, p.ACL+1, p.Publication+1, p.Document, p.ExpectedControl, p.SourceSpace, p.SourceUser)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "DOCUMENT_CHANGED")
	}
	return InsertSubmissionTask(ctx, tx, PersonalTransferTask, p.Operation, p.Operation, raw, "QUEUED")
}

````

## 第 10 步：ClaimPersonalTransfer

插入位置：第 9 步代码之后，紧接追加；完成稿约第 148 行。

本步声明：`func ClaimPersonalTransfer(ctx context.Context, tx *sql.Tx, instance string) (PurgeTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task WHERE kind=? AND  ((state IN (&#x27;QUEUED&#x27;,&#x27;RETRY&#x27;,&#x27;UNKNOWN&#x27;) AND retry_at&lt;=UTC_TIMESTAMP(6)) OR (state=&#x27;RU …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?`, instance, task.Generation, task.ID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?`, instance, task.Generation, task.ID)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PurgeTask</code> | 使用 <code>PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>实际返回表达式：<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;TRANSFER_GENERATION_EXHAUSTED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task WHERE kind=? AND  ((state IN (&#x27;QUEUED&#x27;,&#x27;RETRY&#x27;,&#x27;UNKNOWN&#x27;) AND retry_at&lt;=UTC_TIMESTAMP(6)) OR (state=&#x27;RU …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task WHERE kind=? AND  ((state IN (&#x27;QUEUED&#x27;,&#x27;RETRY&#x27;,&#x27;UNKNOWN&#x27;) AND retry_at&lt;=UTC_TIMESTAMP(6)) OR (state=&#x27;RU …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 检查 <code>task.Generation == ^uint64(0)</code>。条件成立时执行 <code>return task, errors.New(&quot;TRANSFER_GENERATION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?`, instance, task.Generation, task.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return task, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5886:6763 -->
````go
func ClaimPersonalTransfer(ctx context.Context, tx *sql.Tx, instance string) (PurgeTask, error) {
	var task PurgeTask
	err := tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task WHERE kind=? AND
 ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6)) OR (state='RUNNING' AND lease_until<=UTC_TIMESTAMP(6)))
 ORDER BY retry_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, PersonalTransferTask).Scan(&task.ID, &task.Operation, &task.Generation)
	if err != nil {
		return task, err
	}
	if task.Generation == ^uint64(0) {
		return task, errors.New("TRANSFER_GENERATION_EXHAUSTED")
	}
	task.Generation++
	_, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='RUNNING',instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?`, instance, task.Generation, task.ID)
	return task, err
}

````

## 第 11 步：LockPersonalTransferTask

插入位置：第 10 步代码之后，紧接追加；完成稿约第 164 行。

本步声明：`func LockPersonalTransferTask(ctx context.Context, tx *sql.Tx, t PurgeTask, instance string) (domain.PersonalTransfer, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? AND kind=? AND operation_id=? FOR UPDATE`, t.ID, PersonalTransferTask, t.Operation)</code>；<code>tx.QueryRowContext(ctx, `SELECT id FROM personal_transfer WHERE accept_operation_id=?`, t.Operation)</code>；<code>PersonalTransferRow(ctx, tx, id, true)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>PersonalTransferRow(ctx, tx, id, true)</code> |
| <code>t</code> | <code>PurgeTask</code> | 使用 <code>PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>在此声明中的实际使用：<code>t.ID</code>；<code>t.Operation</code>；<code>t.Generation</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>owner != instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.PersonalTransfer</code> | 使用 <code>domain.PersonalTransfer</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>实际返回表达式：<code>domain.PersonalTransfer{}</code>；<code>p</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRANSFER_WORKER_CHANGED&quot;)</code>；<code>errors.New(&quot;TRANSFER_PLAN_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? AND kind=? AND operation_id=? FOR UPDATE`, t.ID, PersonalTransferTask, t.Operation).Scan(&amp;generation, &amp;state, &amp;owner)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? AND kind=? AND operation_id=? FOR UPDATE`, t.ID, PersonalTransferTask, t.Operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.PersonalTransfer{}, err</code>。当前函数在这里结束。
- 检查 <code>generation != t.Generation &#124;&#124; state != &quot;RUNNING&quot; &#124;&#124; owner != instance</code>。条件成立时执行 <code>return domain.PersonalTransfer{}, domain.Violation(domain.StateConflict, &quot;TRANSFER_WORKER_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.PersonalTransfer{}, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT id FROM personal_transfer WHERE accept_operation_id=?`, t.Operation).Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT id FROM personal_transfer WHERE accept_operation_id=?`, t.Operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return p, err</code>。当前函数在这里结束。
- 检查 <code>p.State != &quot;ACCEPTING&quot; &#124;&#124; p.Plan.Operation != t.Operation &#124;&#124; p.Plan.Transfer != p.ID &#124;&#124; p.Plan.Document != p.Document</code>。条件成立时执行 <code>return p, errors.New(&quot;TRANSFER_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return p, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6763:7870 -->
````go
func LockPersonalTransferTask(ctx context.Context, tx *sql.Tx, t PurgeTask, instance string) (domain.PersonalTransfer, error) {
	var generation, id uint64
	var state, owner string
	err := tx.QueryRowContext(ctx, `SELECT generation,state,COALESCE(instance_id,'') FROM durable_task WHERE id=? AND kind=? AND operation_id=? FOR UPDATE`, t.ID, PersonalTransferTask, t.Operation).Scan(&generation, &state, &owner)
	if err != nil {
		return domain.PersonalTransfer{}, err
	}
	if generation != t.Generation || state != "RUNNING" || owner != instance {
		return domain.PersonalTransfer{}, domain.Violation(domain.StateConflict, "TRANSFER_WORKER_CHANGED")
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM personal_transfer WHERE accept_operation_id=?`, t.Operation).Scan(&id); err != nil {
		return domain.PersonalTransfer{}, err
	}
	p, err := PersonalTransferRow(ctx, tx, id, true)
	if err != nil {
		return p, err
	}
	if p.State != "ACCEPTING" || p.Plan.Operation != t.Operation || p.Plan.Transfer != p.ID || p.Plan.Document != p.Document {
		return p, errors.New("TRANSFER_PLAN_CHANGED")
	}
	return p, nil
}

````

## 第 12 步：RetryPersonalTransfer

插入位置：第 11 步代码之后，紧接追加；完成稿约第 187 行。

本步声明：`func RetryPersonalTransfer(ctx context.Context, db *sql.DB, t PurgeTask, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;TRANSFER_PENDING&#x27; WHERE id=? AND gener …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.ExecContext</code> |
| <code>t</code> | <code>PurgeTask</code> | 使用 <code>PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>在此声明中的实际使用：<code>t.ID</code>；<code>t.Generation</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;TRANSFER_PENDING&#x27; WHERE id=? AND gener …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;TRANSFER_PENDING&#x27; WHERE id=? AND gener …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=7870:8252 -->
````go
func RetryPersonalTransfer(ctx context.Context, db *sql.DB, t PurgeTask, instance string) error {
	_, err := db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error='TRANSFER_PENDING' WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'`, t.ID, t.Generation, instance)
	return err
}

````

## 第 13 步：CompletePersonalTransfer

插入位置：第 12 步代码之后，紧接追加；完成稿约第 192 行。

本步声明：`func CompletePersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE draft_body_capacity SET space_id=? WHERE document_id=? AND space_id=?&quot;, p.TargetSpace, p.Document, p.SourceSpace)</code>；<code>tx.ExecContext(ctx, `UPDATE document SET space_id=?,knowledge_base_id=?,responsible_user_id=?,lifecycle=&#x27;ACTIVE&#x27;,control_revision=control_revision+1  WHERE id=? AND lifecycle=&#x27;CLOS …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, statement, p.Document)</code>；<code>tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,node_bytes,operation_id) VALUES(?,?,?,?,?)  ON DUPLICATE KEY UPDATE space_id=VALUES(space_id),bytes=VA …</code>（长表达式，按本步完整代码继续）<br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>NotifyResponsibilityTransfer(ctx, tx, p.TargetUser, p.Operation, &#91;&#93;byte(`{&quot;document_id&quot;:&quot;`+domain.TextID(p.Document)+`&quot;}`))</code> |
| <code>p</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>在此声明中的实际使用：<code>p.TargetSpace</code>；<code>p.Document</code>；<code>p.SourceSpace</code>；<code>p.TargetKB</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRANSFER_DOCUMENT_CHANGED&quot;)</code>；<code>errors.New(&quot;TRANSFER_RECEIPT_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE draft_body_capacity SET space_id=? WHERE document_id=? AND space_id=?&quot;, p.TargetSpace, p.Document, p.SourceSpace)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, `UPDATE document SET space_id=?,knowledge_base_id=?,responsible_user_id=?,lifecycle=&#x27;ACTIVE&#x27;,control_revision=control_revision+1  WHERE id=? AND lifecycle=&#x27;CLOS …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;TRANSFER_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, statement, p.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,node_bytes,operation_id) VALUES(?,?,?,?,?)  ON DUPLICATE KEY UPDATE space_id=VALUES(space_id),bytes=VA …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=&#x27;ACCEPTED&#x27;,revision=revision+1 WHERE id=? AND state=&#x27;ACCEPTING&#x27; AND accept_operation_id=?`, p.Transfer, p.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=&#x27;CANCELLED&#x27;,revision=revision+1 WHERE document_id=? AND state=&#x27;PENDING&#x27;`, p.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind=&#x27;PERSONAL_TRANSFER_ACCEP …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;TRANSFER_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=8252:11513 -->
````go
func CompletePersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) error {
	// 排空与 TransferInterlock 已确保所有正文预留结算。移动高水位归属，
	// 下一次编辑只预留新的差额，不给原房间再次扣整篇正文。
	if _, err := tx.ExecContext(ctx, "UPDATE draft_body_capacity SET space_id=? WHERE document_id=? AND space_id=?", p.TargetSpace, p.Document, p.SourceSpace); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE document SET space_id=?,knowledge_base_id=?,responsible_user_id=?,lifecycle='ACTIVE',control_revision=control_revision+1
 WHERE id=? AND lifecycle='CLOSING' AND space_id=? AND responsible_user_id=? AND control_revision=? AND acl_revision=? AND publication_epoch=?`, p.TargetSpace, p.TargetKB, p.TargetUser, p.Document, p.SourceSpace, p.SourceUser, p.ClosingControl, p.ACL+1, p.Publication+1)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "TRANSFER_DOCUMENT_CHANGED")
	}
	for _, statement := range []string{
		`UPDATE content_grant SET state='CLOSED',revision=revision+1 WHERE document_id=? AND state<>'CLOSED'`,
		`UPDATE document_invitation SET state='CLOSED',revision=revision+1 WHERE document_id=? AND state='OPEN'`,
		`UPDATE document_application SET state='CANCELLED',revision=revision+1 WHERE document_id=? AND state='PENDING'`,
	} {
		if _, err = tx.ExecContext(ctx, statement, p.Document); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,node_bytes,operation_id) VALUES(?,?,?,?,?)
 ON DUPLICATE KEY UPDATE space_id=VALUES(space_id),bytes=VALUES(bytes),node_bytes=VALUES(node_bytes),operation_id=VALUES(operation_id)`, p.Document, p.TargetSpace, p.BodyBytes+p.NodeBytes, p.NodeBytes, p.Operation); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE personal_transfer SET state='ACCEPTED',revision=revision+1 WHERE id=? AND state='ACCEPTING' AND accept_operation_id=?`, p.Transfer, p.Operation); err != nil {
		return err
	}
	// 旧 owner 的其它申请不随文档归属继续生效；标题快照也不泄漏新 owner 的后续改名。
	if _, err = tx.ExecContext(ctx, `UPDATE personal_transfer SET state='CANCELLED',revision=revision+1 WHERE document_id=? AND state='PENDING'`, p.Document); err != nil {
		return err
	}
	// 负责人通知的既定含义是已经完成；申请阶段不能复用该通知冒充迁移。
	if err = NotifyResponsibilityTransfer(ctx, tx, p.TargetUser, p.Operation, []byte(`{"document_id":"`+domain.TextID(p.Document)+`"}`)); err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]any{"operation_id": p.Operation, "state": "SUCCEEDED", "result": map[string]string{"entity_id": domain.TextID(p.Document)}})
	result, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind='PERSONAL_TRANSFER_ACCEPT' AND state='PREPARING'`, string(raw), p.Operation, p.TargetUser)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("TRANSFER_RECEIPT_CHANGED")
	}
````

## 第 14 步：CompletePersonalTransfer（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 238 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL WHERE kind=? AND operation_id=?`, PersonalTransferTask, p.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=11513:11703 -->
````go
	_, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE kind=? AND operation_id=?`, PersonalTransferTask, p.Operation)
	return err
}

````

## 第 15 步：PersonalTransferAccountPending

插入位置：第 14 步代码之后，紧接追加；完成稿约第 242 行。

Account 固定父空间范围之前调用；源或目标的已接受固定关闭都必须先完成真实迁移。

本步声明：`func PersonalTransferAccountPending(ctx context.Context, q Queryer, user uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT COUNT(*) FROM personal_transfer WHERE state=&#x27;ACCEPTING&#x27; AND (source_user_id=? OR target_user_id=?)`, user, user)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT COUNT(*) FROM personal_transfer WHERE state=&#x27;ACCEPTING&#x27; AND (source_user_id=? OR target_user_id=?)`, user, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>n != 0</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT COUNT(*) FROM personal_transfer WHERE state=&#x27;ACCEPTING&#x27; AND (source_user_id=? OR target_user_id=?)`, user, user).Scan(&amp;n)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT COUNT(*) FROM personal_transfer WHERE state=&#x27;ACCEPTING&#x27; AND (source_user_id=? OR target_user_id=?)`, user, user)</code>。
- 此处通过 <code>return n != 0, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=11703:12112 -->
````go
// Account 固定父空间范围之前调用；源或目标的已接受固定关闭都必须先完成真实迁移。
func PersonalTransferAccountPending(ctx context.Context, q Queryer, user uint64) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM personal_transfer WHERE state='ACCEPTING' AND (source_user_id=? OR target_user_id=?)`, user, user).Scan(&n)
	return n != 0, err
}

````

## 第 16 步：TransferFenceConfirmed

插入位置：第 15 步代码之后，紧接追加；完成稿约第 249 行。

本步声明：`func TransferFenceConfirmed(ctx context.Context, tx *sql.Tx, scope domain.PurgeScope) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c WHERE revocation_id=? AND scope_key=? AND epoch=? AND state=&#x27;SUCCEEDED&#x27;  AND EXISTS(SELECT 1 FROM authorization …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>scope</code> | <code>domain.PurgeScope</code> | 使用 <code>domain.PurgeScope</code> 类型；其定义处的职责：定义回收事实、恢复资格和单文档永久删除计划。<br>在此声明中的实际使用：<code>scope.Operation</code>；<code>scope.Key</code>；<code>scope.Epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;TRANSFER_FENCE_NOT_CONFIRMED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c WHERE revocation_id=? AND scope_key=? AND epoch=? AND state=&#x27;SUCCEEDED&#x27;  AND EXISTS(SELECT 1 FROM authorization …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c WHERE revocation_id=? AND scope_key=? AND epoch=? AND state=&#x27;SUCCEEDED&#x27;  AND EXISTS(SELECT 1 FROM authorization …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>n != 1</code>。条件成立时执行 <code>return errors.New(&quot;TRANSFER_FENCE_NOT_CONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=12112:12810 -->
````go
func TransferFenceConfirmed(ctx context.Context, tx *sql.Tx, scope domain.PurgeScope) error {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c WHERE revocation_id=? AND scope_key=? AND epoch=? AND state='SUCCEEDED'
 AND EXISTS(SELECT 1 FROM authorization_ack a WHERE a.revocation_id=c.revocation_id AND a.scope_key=c.scope_key AND a.epoch=c.epoch AND a.participant='collaboration' AND a.phase='OUTPUT_STOPPED')`, scope.Operation, scope.Key, scope.Epoch).Scan(&n)
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("TRANSFER_FENCE_NOT_CONFIRMED")
	}
	return nil
}

// personal_transfer_interlock.go：保留原实现，按业务合并文件。

````

## 第 17 步：PersonalTransferKnowledgeBasePending

插入位置：第 16 步代码之后，紧接追加；完成稿约第 264 行。

父 KB 删除必须同时看原范围与接收计划，不能只看当前 children：接受尚未完成
时 Doc 仍在源 KB，接收目标 KB 没有这个子文档。调用方先锁 KB 来源 gate。

本步声明：`func PersonalTransferKnowledgeBasePending(ctx context.Context, q Queryer, kb uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM personal_transfer t JOIN document d ON d.id=t.document_id  WHERE t.state=&#x27;ACCEPTING&#x27; AND (d.knowledge_base_id=? OR JSON_UNQUOTE( …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM personal_transfer t JOIN document d ON d.id=t.document_id  WHERE t.state=&#x27;ACCEPTING&#x27; AND (d.knowledge_base_id=? OR JSON_UNQUOTE( …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>pending</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM personal_transfer t JOIN document d ON d.id=t.document_id  WHERE t.state=&#x27;ACCEPTING&#x27; AND (d.knowledge_base_id=? OR JSON_UNQUOTE( …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM personal_transfer t JOIN document d ON d.id=t.document_id  WHERE t.state=&#x27;ACCEPTING&#x27; AND (d.knowledge_base_id=? OR JSON_UNQUOTE( …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return pending, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=12810:13418 -->
````go
// 父 KB 删除必须同时看原范围与接收计划，不能只看当前 children：接受尚未完成
// 时 Doc 仍在源 KB，接收目标 KB 没有这个子文档。调用方先锁 KB 来源 gate。
func PersonalTransferKnowledgeBasePending(ctx context.Context, q Queryer, kb uint64) (bool, error) {
	var pending bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM personal_transfer t JOIN document d ON d.id=t.document_id
 WHERE t.state='ACCEPTING' AND (d.knowledge_base_id=? OR JSON_UNQUOTE(JSON_EXTRACT(t.fixed_plan,'$.TargetKB'))=?))`, kb, kb).Scan(&pending)
	return pending, err
}

````

## 第 18 步：PersonalTransferSourcePending

插入位置：第 17 步代码之后，紧接追加；完成稿约第 273 行。

只有固定 transfer driver 的真实终态才能开放更高 Doc 来源；关闭子任务已经
SUCCEEDED 不能代表父迁移已经完成。按当前来源锁序调用，以免提前重开旧范围。

本步声明：`func PersonalTransferSourcePending(ctx context.Context, tx *sql.Tx, key string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>PersonalTransferSourcesPending(ctx, tx, &#91;&#93;string{key})</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>PersonalTransferSourcesPending(ctx, tx, &#91;&#93;string{key})</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>PersonalTransferSourcesPending(ctx, tx, &#91;&#93;string{key})</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>PersonalTransferSourcesPending(ctx, tx, &#91;&#93;string{key})</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>PersonalTransferSourcesPending(ctx, tx, &#91;&#93;string{key})</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return PersonalTransferSourcesPending(ctx, tx, &#91;&#93;string{key})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=13418:13791 -->
````go
// 只有固定 transfer driver 的真实终态才能开放更高 Doc 来源；关闭子任务已经
// SUCCEEDED 不能代表父迁移已经完成。按当前来源锁序调用，以免提前重开旧范围。
func PersonalTransferSourcePending(ctx context.Context, tx *sql.Tx, key string) (bool, error) {
	return PersonalTransferSourcesPending(ctx, tx, []string{key})
}

````

## 第 19 步：PersonalTransferSourcesPending

插入位置：第 18 步代码之后，紧接追加；完成稿约第 279 行。

来源 gate 已由调用方排序锁定。提取 Doc 后按 transfer_document 索引做一次
当前读，既不让 RR 旧快照遗漏原迁移，也不把搜索的一页退化成逐来源 SQL。

本步声明：`func PersonalTransferSourcesPending(ctx context.Context, tx *sql.Tx, keys []string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT state FROM personal_transfer FORCE INDEX(transfer_document)  WHERE document_id IN (`+marks+`) AND state=&#x27;ACCEPTING&#x27; ORDER BY document_id,id LIMIT 1 FOR UPDATE`, args...)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>keys</code> | <code>&#91;&#93;string</code> | 迁移绑定的权限来源键；只从doc-acl/publication键提取文档，去重排序后查询未落定操作。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!found &#124;&#124; (prefix != &quot;doc-acl&quot; &amp;&amp; prefix != &quot;publication&quot;)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>len(documents) == 0</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT state FROM personal_transfer FORCE INDEX(transfer_document)  WHERE document_id IN (`+marks+`) AND state=&#x27;ACCEPTING&#x27; ORDER BY document_id,id LIMIT 1 FOR UPDATE`, args...).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT state FROM personal_transfer FORCE INDEX(transfer_document)  WHERE document_id IN (`+marks+`) AND state=&#x27;ACCEPTING&#x27; ORDER BY document_id,id LIMIT 1 FOR UPDATE`, args...)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=13791:15152 -->
````go
// 来源 gate 已由调用方排序锁定。提取 Doc 后按 transfer_document 索引做一次
// 当前读，既不让 RR 旧快照遗漏原迁移，也不把搜索的一页退化成逐来源 SQL。
func PersonalTransferSourcesPending(ctx context.Context, tx *sql.Tx, keys []string) (bool, error) {
	documents := map[uint64]bool{}
	for _, key := range keys {
		prefix, number, found := strings.Cut(key, ":")
		if !found || (prefix != "doc-acl" && prefix != "publication") {
			continue
		}
		id, err := domain.CanonicalID(number)
		if err != nil {
			return false, err
		}
		documents[id] = true
	}
	if len(documents) == 0 {
		return false, nil
	}
	ids := make([]uint64, 0, len(documents))
	for id := range documents {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	var state string
	err := tx.QueryRowContext(ctx, `SELECT state FROM personal_transfer FORCE INDEX(transfer_document)
 WHERE document_id IN (`+marks+`) AND state='ACCEPTING' ORDER BY document_id,id LIMIT 1 FOR UPDATE`, args...).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// personal_transfer_terminal.go：保留原实现，按业务合并文件。

````

## 第 20 步：SavePersonalTransferMeasurement

插入位置：第 19 步代码之后，紧接追加；完成稿约第 318 行。

本步声明：`func SavePersonalTransferMeasurement(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE personal_transfer SET fixed_plan=? WHERE id=? AND accept_operation_id=?`, string(raw), p.Transfer, p.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>p</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>在此声明中的实际使用：<code>json.Marshal(p)</code>；<code>p.Transfer</code>；<code>p.Operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE personal_transfer SET fixed_plan=? WHERE id=? AND accept_operation_id=?`, string(raw), p.Transfer, p.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 20 bytes=15152:15488 -->
````go
func SavePersonalTransferMeasurement(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE personal_transfer SET fixed_plan=? WHERE id=? AND accept_operation_id=?`, string(raw), p.Transfer, p.Operation)
	return err
}

````

## 第 21 步：RejectPersonalTransfer

插入位置：第 20 步代码之后，紧接追加；完成稿约第 327 行。

本步声明：`func RejectPersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan, reason string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;ACTIVE&#x27;,control_revision=control_revision+1 WHERE id=? AND lifecycle=&#x27;CLOSING&#x27; AND space_id=? AND responsible_user_id=? AND cont …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=&#x27;REJECTED&#x27;,revision=revision+1 WHERE id=? AND state=&#x27;ACCEPTING&#x27; AND accept_operation_id=?`, p.Transfer, p.Operation)</code>；<code>tx.ExecContext(ctx, `UPDATE operation_receipt SET state=&#x27;REJECTED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind=&#x27;PERSONAL_TRANSFER_ACCEPT …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;REJECTED&#x27;,last_error=?,lease_until=NULL WHERE kind=? AND operation_id=?`, reason, PersonalTransferTask, p.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>p</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>在此声明中的实际使用：<code>p.Document</code>；<code>p.SourceSpace</code>；<code>p.SourceUser</code>；<code>p.ClosingControl</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>reason</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;code&quot;: reason</code>；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;REJECTED&#x27;,last_error=?,lease_until=NULL WHERE kind=? AND operation_id=?`, reason, PersonalTransferTask, p.Operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;TRANSFER_COMPENSATION_CHANGED&quot;)</code>；<code>errors.New(&quot;TRANSFER_COMPENSATION_RECEIPT_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;ACTIVE&#x27;,control_revision=control_revision+1 WHERE id=? AND lifecycle=&#x27;CLOSING&#x27; AND space_id=? AND responsible_user_id=? AND cont …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;TRANSFER_COMPENSATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE personal_transfer SET state=&#x27;REJECTED&#x27;,revision=revision+1 WHERE id=? AND state=&#x27;ACCEPTING&#x27; AND accept_operation_id=?`, p.Transfer, p.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE operation_receipt SET state=&#x27;REJECTED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind=&#x27;PERSONAL_TRANSFER_ACCEPT …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;TRANSFER_COMPENSATION_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;REJECTED&#x27;,last_error=?,lease_until=NULL WHERE kind=? AND operation_id=?`, reason, PersonalTransferTask, p.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=15488:17312 -->
````go
func RejectPersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan, reason string) error {
	result, err := tx.ExecContext(ctx, `UPDATE document SET lifecycle='ACTIVE',control_revision=control_revision+1 WHERE id=? AND lifecycle='CLOSING' AND space_id=? AND responsible_user_id=? AND control_revision=? AND acl_revision=? AND publication_epoch=?`, p.Document, p.SourceSpace, p.SourceUser, p.ClosingControl, p.ACL+1, p.Publication+1)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("TRANSFER_COMPENSATION_CHANGED")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE personal_transfer SET state='REJECTED',revision=revision+1 WHERE id=? AND state='ACCEPTING' AND accept_operation_id=?`, p.Transfer, p.Operation); err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]any{"operation_id": p.Operation, "state": "REJECTED", "error": map[string]string{"code": reason, "message": "接收空间容量不足，原文档归属已恢复", "trace_id": p.Operation, "operation_id": p.Operation, "retry_action": "REFRESH_STATE"}, "result": map[string]string{"entity_id": domain.TextID(p.Document)}})
	if err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET state='REJECTED',result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind='PERSONAL_TRANSFER_ACCEPT' AND state='PREPARING'`, string(raw), p.Operation, p.TargetUser)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("TRANSFER_COMPENSATION_RECEIPT_CHANGED")
	}
	_, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='REJECTED',last_error=?,lease_until=NULL WHERE kind=? AND operation_id=?`, reason, PersonalTransferTask, p.Operation)
	return err
}

````

## 第 22 步：ActiveTransferTargetACL

插入位置：第 21 步代码之后，紧接追加；完成稿约第 353 行。

本步声明：`func ActiveTransferTargetACL(ctx context.Context, q Queryer, kb, space uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=? AND space_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, kb, space)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=? AND space_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, kb, space)</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=? AND space_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, kb, space)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>revision</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=? AND space_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, kb, space).Scan(&amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=? AND space_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, kb, space)</code>。
- 此处通过 <code>return revision, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=17312:17605 -->
````go
func ActiveTransferTargetACL(ctx context.Context, q Queryer, kb, space uint64) (uint64, error) {
	var revision uint64
	err := q.QueryRowContext(ctx, "SELECT acl_revision FROM knowledge_base WHERE id=? AND space_id=? AND lifecycle='ACTIVE'", kb, space).Scan(&revision)
	return revision, err
}

````

## 第 23 步：LockTransferConsumerDocuments

插入位置：第 22 步代码之后，紧接追加；完成稿约第 359 行。

调用方已先排序锁定来源；文档锁继续按编号递增取得，避免共享资源消费者互锁。

本步声明：`func LockTransferConsumerDocuments(ctx context.Context, tx *sql.Tx, documents []uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM document WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>documents</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>append(&#91;&#93;uint64{}, documents...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>i &gt; 0 &amp;&amp; id == previous</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM document WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;current)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM document WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=17605:18206 -->
````go
// 调用方已先排序锁定来源；文档锁继续按编号递增取得，避免共享资源消费者互锁。
func LockTransferConsumerDocuments(ctx context.Context, tx *sql.Tx, documents []uint64) error {
	ids := append([]uint64{}, documents...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var previous uint64
	for i, id := range ids {
		if i > 0 && id == previous {
			continue
		}
		var current uint64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM document WHERE id=? FOR UPDATE", id).Scan(&current); err != nil {
			return err
		}
		previous = id
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/personal_transfer_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
