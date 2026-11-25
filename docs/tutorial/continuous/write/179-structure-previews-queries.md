# 分步编写 `services/document/internal/repository/document/structure_previews_queries.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/179-structure-previews-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久结构预览上下文与恢复任务，列出当前结构的单元和活动支线。

整份预览必须基于完整单元集合，后台恢复要继续相同范围。

写之前先按这条执行路线理解各段的连接：

1. CurrentStructurePreviewUnits固定当前unit/chapter/branch
2. RegisterStructurePreview保存原payload
3. ClaimStructurePreviewTask领取
4. StructurePreviewResult读取原回执。

本文件需要守住的边界：

- 重新领取任务不能重新选择当时最新目录作为预览范围。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/structure_previews_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/structure_previews_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/structure_previews_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/structure_previews_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:292 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// structure_preview_readiness.go：保留原实现，按业务合并文件。

````

## 第 2 步：PendingStructurePreviewBinding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`type PendingStructurePreviewBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原业务操作号，恢复继续同一号。 |
| <code>State</code> | <code>string</code> | 与此操作关联的回执状态。 |
| <code>Actor</code> | <code>uint64</code> | 原回执绑定的操作者。 |
| <code>Original</code> | <code>&#91;&#93;byte</code> | 原上下文任务中保存的载荷；不能由当前请求替换。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 当前恢复任务的载荷，与原上下文互相核对。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 原回执的目标JSON。 |

<!-- write-step: 2 bytes=292:481 -->
````go
type PendingStructurePreviewBinding struct {
	Operation, State                       string
	Actor                                  uint64
	Original, Payload, Fingerprint, Target []byte
}

````

## 第 3 步：PendingStructurePreviewBindings

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func PendingStructurePreviewBindings(ctx context.Context, db *sql.DB) ([]PendingStructurePreviewBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT t.operation_id,COALESCE(o.state,&#x27;&#x27;),COALESCE(o.actor_id,0),  original.payload,t.payload,o.request_fingerprint,o.target_json FROM durable_task t  LEFT J …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingStructurePreviewBinding</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, `SELECT t.operation_id,COALESCE(o.state,&#x27;&#x27;),COALESCE(o.actor_id,0),  original.payload,t.payload,o.request_fingerprint,o.target_json FROM durable_task t  LEFT J …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;item.Operation, &amp;item.State, &amp;item.Actor, &amp;item.Original, &amp;item.Payload, &amp;item.Fingerprint, &amp;item.Target)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return result, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=481:1605 -->
````go
func PendingStructurePreviewBindings(ctx context.Context, db *sql.DB) ([]PendingStructurePreviewBinding, error) {
	rows, err := db.QueryContext(ctx, `SELECT t.operation_id,COALESCE(o.state,''),COALESCE(o.actor_id,0),
 original.payload,t.payload,o.request_fingerprint,o.target_json FROM durable_task t
 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id AND o.kind='STRUCTURE_PREVIEW'
 LEFT JOIN durable_task original ON original.kind='STRUCTURE_PREVIEW_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
 WHERE t.kind='STRUCTURE_PREVIEW_RECOVER' AND t.state NOT IN ('SUCCEEDED','REJECTED') ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PendingStructurePreviewBinding{}
	for rows.Next() {
		var item PendingStructurePreviewBinding
		if err = rows.Scan(&item.Operation, &item.State, &item.Actor, &item.Original, &item.Payload, &item.Fingerprint, &item.Target); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// structure_previews.go：保留原实现，按业务合并文件。

````

## 第 4 步：StructurePreviewUnit

插入位置：第 3 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`type StructurePreviewUnit struct{ Unit, Chapter, Branch uint64 }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Unit</code> | <code>uint64</code> | 纳入结构预览的协作单元。 |
| <code>Chapter</code> | <code>uint64</code> | 此单元原属章节；拆合不能丢掉父章节。 |
| <code>Branch</code> | <code>uint64</code> | 该单元当前共同支线编号。 |

<!-- write-step: 4 bytes=1605:1671 -->
````go
type StructurePreviewUnit struct{ Unit, Chapter, Branch uint64 }

````

## 第 5 步：CurrentStructurePreviewUnits

插入位置：第 4 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`func CurrentStructurePreviewUnits(ctx context.Context, tx *sql.Tx, document, structure uint64) ([]StructurePreviewUnit, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT u.id,u.chapter_id,COALESCE(u.active_branch_id,0) FROM collaboration_unit u  JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AN …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT u.id,u.chapter_id,COALESCE(u.active_branch_id,0) FROM collaboration_unit u  JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AN …</code>（长表达式，按本步完整代码继续） |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT u.id,u.chapter_id,COALESCE(u.active_branch_id,0) FROM collaboration_unit u  JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AN …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;StructurePreviewUnit</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT u.id,u.chapter_id,COALESCE(u.active_branch_id,0) FROM collaboration_unit u  JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AN …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;unit.Unit, &amp;unit.Chapter, &amp;unit.Branch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return result, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1671:2417 -->
````go
func CurrentStructurePreviewUnits(ctx context.Context, tx *sql.Tx, document, structure uint64) ([]StructurePreviewUnit, error) {
	rows, err := tx.QueryContext(ctx, `SELECT u.id,u.chapter_id,COALESCE(u.active_branch_id,0) FROM collaboration_unit u
 JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=?
 WHERE u.document_id=? AND u.retired_structure_id IS NULL ORDER BY u.id`, structure, document)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []StructurePreviewUnit{}
	for rows.Next() {
		var unit StructurePreviewUnit
		if err = rows.Scan(&unit.Unit, &unit.Chapter, &unit.Branch); err != nil {
			return nil, err
		}
		result = append(result, unit)
	}
	return result, rows.Err()
}

````

## 第 6 步：StructurePreviewResult

插入位置：第 5 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func StructurePreviewResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at  FROM operation_receipt WHERE operation_id=? AND kind=&#x27;STRUCTURE_PREVIEW&#x27;`, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at  FROM operation_receipt WHERE operation_id=? AND kind=&#x27;STRUCTURE_PREVIEW&#x27;`, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlReceipt</code> | 使用 <code>MainControlReceipt</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at  FROM operation_receipt WHERE operation_id=? AND kind=&#x27;STRUCTURE_PREVIEW&#x27;`, …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at  FROM operation_receipt WHERE operation_id=? AND kind=&#x27;STRUCTURE_PREVIEW&#x27;`, operation)</code>。
- 此处通过 <code>return result, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2417:2902 -->
````go
func StructurePreviewResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var result MainControlReceipt
	err := q.QueryRowContext(ctx, `SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at
 FROM operation_receipt WHERE operation_id=? AND kind='STRUCTURE_PREVIEW'`, operation).Scan(&result.Actor, &result.Kind, &result.State, &result.Fingerprint, &result.Result, &result.Generation, &result.Expires)
	return result, err
}

````

## 第 7 步：RegisterStructurePreview

插入位置：第 6 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func RegisterStructurePreview(ctx context.Context, tx *sql.Tx, operation string, raw []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;STRUCTURE_PREVIEW_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;STRUCTURE_PREVIEW_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;STRUCTURE_PREVIEW_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;STRUCTURE_PREVIEW_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2902:3376 -->
````go
func RegisterStructurePreview(ctx context.Context, tx *sql.Tx, operation string, raw []byte) error {
	// 原上下文与后台任务依赖已登记receipt，使用调用方同一事务，不把未知RPC调用塞进SQL锁内。
	if err := InsertSubmissionTask(ctx, tx, "STRUCTURE_PREVIEW_CONTEXT", operation, operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "STRUCTURE_PREVIEW_RECOVER", operation, operation, raw, "QUEUED")
}

````

## 第 8 步：ClaimStructurePreviewTask

插入位置：第 7 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func ClaimStructurePreviewTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, &quot;STRUCTURE_PREVIEW_RECOVER&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, &quot;STRUCTURE_PREVIEW_RECOVER&quot;)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, &quot;STRUCTURE_PREVIEW_RECOVER&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, &quot;STRUCTURE_PREVIEW_RECOVER&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, &quot;STRUCTURE_PREVIEW_RECOVER&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return claimOperationTask(ctx, tx, instance, &quot;STRUCTURE_PREVIEW_RECOVER&quot;, &quot;STRUCTURE_PREVIEW_RECOVER&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3376:3596 -->
````go
func ClaimStructurePreviewTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "STRUCTURE_PREVIEW_RECOVER", "STRUCTURE_PREVIEW_RECOVER")
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/structure_previews_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
