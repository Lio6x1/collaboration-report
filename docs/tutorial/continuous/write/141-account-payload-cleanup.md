# 分步编写 `services/document/internal/repository/document/account_payload_cleanup.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/141-account-payload-cleanup.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

精确清除已终态任务中保存的私有正文、标题和计算输入。

仅删除关系历史还会留下任务payload副本；清理又不能毁掉通知、屏障和上传恢复凭据。

写之前先按这条执行路线理解各段的连接：

1. accountPayloadTasks取得指定类型
2. RemoveAccountPayloads先由context定位文档，再沿精确operation_id找子工件，事务内删除并记回执。

本文件需要守住的边界：

- 不按business_key前缀猜归属
- 跨文档或未终态任务必须拒绝清理。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/account_payload_cleanup.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/account_payload_cleanup.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/account_payload_cleanup.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/account_payload_cleanup.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:179 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

````

## 第 2 步：accountBodyTaskKinds

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

只覆盖实际保存私有内容/标题/固定输入的Doc任务。通知、原授权屏障、
房间编号与上传恢复凭据另有职责，不能为了清正文把原恢复依据全部删除。

本步声明：`var accountBodyTaskKinds = []string{`。

<!-- write-step: 2 bytes=179:1442 -->
````go
// 只覆盖实际保存私有内容/标题/固定输入的Doc任务。通知、原授权屏障、
// 房间编号与上传恢复凭据另有职责，不能为了清正文把原恢复依据全部删除。
var accountBodyTaskKinds = []string{
	"SUBMISSION_RECOVER", "CAPTURE_FINALIZE", "MERGE_RECOVER", "MAIN_COMMIT_FIXED", "MAIN_RECONCILE",
	"MAIN_CONTROL_CONTEXT", "MAIN_CONTROL_RECOVER", "MAIN_CONTROL_MAIN_FIXED", "MAIN_CONTROL_RECONCILE",
	"PUBLICATION_CONTEXT", "PUBLICATION_RECOVER", "PULL_MAIN_CONTEXT", "PULL_MAIN_RECOVER", "PULL_MAIN_COMMITTED", "PULL_MAIN_RECONCILE",
	"STRUCTURE_PREVIEW_CONTEXT", "STRUCTURE_PREVIEW_RECOVER", "STRUCTURE_PREVIEW_FIXED",
	"DISCARD_PREVIEW_CONTEXT", "DISCARD_PREVIEW_RECOVER", "DISCARD_PREVIEW_FIXED",
	"DISCARD_DRAFT_CONTEXT", "DISCARD_DRAFT_RECOVER", "DISCARD_DRAFT_COMMITTED", "DISCARD_DRAFT_RECONCILE",
	"REVERSE_CONTEXT", "REVERSE_RECOVER", "REVERSE_FIXED", "REVERSE_COMMITTED", "REVERSE_RECONCILE", "REVERSE_REVIEW_SOURCE",
	"RESOLUTION_CONTEXT", "RESOLUTION_RECOVER", "RESOLUTION_FIXED", "RESOLUTION_COMMITTED", "RESOLUTION_RECONCILE",
	"MERGE_ATTEMPT_CONTEXT", "MERGE_ATTEMPT_FIXED", "MERGE_ATTEMPT_RANGE_FIXED", "CONFLICT_CONTEXT_VERSIONS",
	"REVIEW_ROUND_CONTEXT", "REVIEW_CAPTURE_CUT", "MAIN_READ_CONTEXT",
}

````

## 第 3 步：accountPayloadTask

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`type accountPayloadTask struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>uint64</code> | 待清理持久任务的主键。 |
| <code>operation</code> | <code>sql.NullString</code> | 任务原操作号，允许SQL NULL；不能把缺失号当成可恢复的操作。 |
| <code>owner</code> | <code>sql.NullString</code> | 任务实例所有者，允许SQL NULL；用于判断是否仍被执行实例持有。 |
| <code>state</code> | <code>string</code> | 任务当前状态；不能清掉仍会继续执行所需的载荷。 |

<!-- write-step: 3 bytes=1442:1561 -->
````go
type accountPayloadTask struct {
	id               uint64
	operation, owner sql.NullString
	state            string
}

````

## 第 4 步：AccountPayloadCleanupReceipt

插入位置：第 3 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`func AccountPayloadCleanupReceipt(ctx context.Context, q Queryer, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;ACCOUNT_PAYLOAD_CLEANUP&#x27; AND business_key=?&quot;, request.Document)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Document</code>；<code>request.Operation</code>；<code>result.AccountDocumentCleanupRequest != request</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_PAYLOAD_CLEANUP_BINDING_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;ACCOUNT_PAYLOAD_CLEANUP&#x27; AND business_key=?&quot;, request.Document).Scan(&amp;operation, &amp;state, &amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;ACCOUNT_PAYLOAD_CLEANUP&#x27; AND business_key=?&quot;, request.Document)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return result, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, false, err</code>。当前函数在这里结束。
- 检查 <code>operation != request.Operation &#124;&#124; state != &quot;SUCCEEDED&quot; &#124;&#124; json.Unmarshal(raw, &amp;result) != nil &#124;&#124; !result.Valid() &#124;&#124; result.AccountDocumentCleanupRequest != request</code>。条件成立时执行 <code>return result, false, errors.New(&quot;ACCOUNT_PAYLOAD_CLEANUP_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1561:2413 -->
````go
func AccountPayloadCleanupReceipt(ctx context.Context, q Queryer, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, bool, error) {
	var result platform.AccountDocumentCleanupContext
	var operation, state string
	var raw []byte
	err := q.QueryRowContext(ctx, "SELECT operation_id,state,payload FROM durable_task WHERE kind='ACCOUNT_PAYLOAD_CLEANUP' AND business_key=?", request.Document).Scan(&operation, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if operation != request.Operation || state != "SUCCEEDED" || json.Unmarshal(raw, &result) != nil || !result.Valid() || result.AccountDocumentCleanupRequest != request {
		return result, false, errors.New("ACCOUNT_PAYLOAD_CLEANUP_BINDING_CHANGED")
	}
	return result, true, nil
}

````

## 第 5 步：accountPayloadTasks

插入位置：第 4 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func accountPayloadTasks(ctx context.Context, tx *sql.Tx, predicate string, args []any) ([]accountPayloadTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id,operation_id,state,cleanup_document FROM durable_task WHERE kind IN (&quot;+marks+&quot;) AND (&quot;+predicate+&quot;) ORDER BY id&quot;, all...)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>predicate</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;SELECT id,operation_id,state,cleanup_document FROM durable_task WHERE kind IN (&quot;+marks+&quot;) AND (&quot;+predicate</code> |
| <code>args</code> | <code>&#91;&#93;any</code> | 在此声明中的实际使用：<code>len(args)</code>；<code>append(all, args...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;accountPayloadTask</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, &quot;SELECT id,operation_id,state,cleanup_document FROM durable_task WHERE kind IN (&quot;+marks+&quot;) AND (&quot;+predicate+&quot;) ORDER BY id&quot;, all...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;row.id, &amp;row.operation, &amp;row.state, &amp;row.owner)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return result, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2413:3448 -->
````go
func accountPayloadTasks(ctx context.Context, tx *sql.Tx, predicate string, args []any) ([]accountPayloadTask, error) {
	marks := strings.TrimSuffix(strings.Repeat("?,", len(accountBodyTaskKinds)), ",")
	all := make([]any, 0, len(accountBodyTaskKinds)+len(args))
	for _, kind := range accountBodyTaskKinds {
		all = append(all, kind)
	}
	all = append(all, args...)
	// 007迁移从原payload派生归属并建索引，正文更新时由MySQL同步维护；
	// 避免每次注销逐行解析全部历史任务JSON，仍使用同一事务的真实绑定。
	rows, err := tx.QueryContext(ctx, "SELECT id,operation_id,state,cleanup_document FROM durable_task WHERE kind IN ("+marks+") AND ("+predicate+") ORDER BY id", all...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []accountPayloadTask{}
	for rows.Next() {
		var row accountPayloadTask
		if err = rows.Scan(&row.id, &row.operation, &row.state, &row.owner); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

````

## 第 6 步：RemoveAccountPayloads

插入位置：第 5 步代码之后，紧接追加；完成稿约第 76 行。

先用原context确定文档，再沿精确operation_id扩展无doc字段的子固定任务。
不读取/输出正文，不用business_key前缀猜归属；跨文档或未终态必须拒绝。

本步声明：`func RemoveAccountPayloads(ctx context.Context, tx *sql.Tx, authority platform.AccountDocumentCleanupContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>accountPayloadTasks(ctx, tx, &quot;cleanup_document=?&quot;, &#91;&#93;any{authority.Document})</code>；<code>accountPayloadTasks(ctx, tx, &quot;operation_id=?&quot;, &#91;&#93;any{operation})</code>；<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET payload=? WHERE id=? AND state=?&quot;, string(raw), task.id, task.state)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>accountPayloadTasks(ctx, tx, &quot;cleanup_document=?&quot;, &#91;&#93;any{authority.Document})</code>；<code>accountPayloadTasks(ctx, tx, &quot;operation_id=?&quot;, &#91;&#93;any{operation})</code>；<code>tx.ExecContext</code> |
| <code>authority</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>在此声明中的实际使用：<code>authority.Document</code>；<code>&quot;cleanup_context&quot;: authority</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_PAYLOAD_DOCUMENT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_PAYLOAD_RESULT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_PAYLOAD_TASK_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>root.operation.Valid &amp;&amp; root.operation.String != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>child.owner.Valid &amp;&amp; child.owner.String != authority.Document</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_PAYLOAD_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>task.state != &quot;SUCCEEDED&quot; &amp;&amp; task.state != &quot;REJECTED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_PAYLOAD_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET payload=? WHERE id=? AND state=?&quot;, string(raw), task.id, task.state)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=3448:5284 -->
````go
// 先用原context确定文档，再沿精确operation_id扩展无doc字段的子固定任务。
// 不读取/输出正文，不用business_key前缀猜归属；跨文档或未终态必须拒绝。
func RemoveAccountPayloads(ctx context.Context, tx *sql.Tx, authority platform.AccountDocumentCleanupContext) error {
	roots, err := accountPayloadTasks(ctx, tx, "cleanup_document=?", []any{authority.Document})
	if err != nil {
		return err
	}
	operations := map[string]bool{}
	tasks := map[uint64]accountPayloadTask{}
	for _, root := range roots {
		tasks[root.id] = root
		if root.operation.Valid && root.operation.String != "" {
			operations[root.operation.String] = true
		}
	}
	for operation := range operations {
		children, err := accountPayloadTasks(ctx, tx, "operation_id=?", []any{operation})
		if err != nil {
			return err
		}
		for _, child := range children {
			if child.owner.Valid && child.owner.String != authority.Document {
				return errors.New("ACCOUNT_PAYLOAD_DOCUMENT_CHANGED")
			}
			tasks[child.id] = child
		}
	}
	for _, task := range tasks {
		if task.state != "SUCCEEDED" && task.state != "REJECTED" {
			return domain.Violation(domain.StateConflict, "ACCOUNT_PAYLOAD_RESULT_UNKNOWN")
		}
	}
	// 原任务终态/原号保留，原正文不可再提供给迟到安装器；Node永久墓碑
	// 已阻止物化恢复。保留规范doc context，后续排空核验仍可精确定位。
	raw, err := json.Marshal(map[string]any{
		"body_removed": true, "cleanup_context": authority,
		"context": map[string]any{"target": map[string]string{"document_id": authority.Document}},
	})
	if err != nil {
		return err
	}
	for _, task := range tasks {
		result, err := tx.ExecContext(ctx, "UPDATE durable_task SET payload=? WHERE id=? AND state=?", string(raw), task.id, task.state)
		if err != nil {
			return err
		}
````

## 第 7 步：RemoveAccountPayloads（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 122 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_PAYLOAD_TASK_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=5284:5482 -->
````go
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return domain.Violation(domain.StateConflict, "ACCOUNT_PAYLOAD_TASK_CHANGED")
		}
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/account_payload_cleanup.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
