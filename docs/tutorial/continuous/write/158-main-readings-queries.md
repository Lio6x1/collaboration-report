# 分步编写 `services/document/internal/repository/document/main_readings_queries.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/158-main-readings-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存只读MAIN捕获上下文并读取当前固定安装的工作区代次。

只读捕获要可验证来源，却不能伪装成主线控制操作或公开成功回执。

写之前先按这条执行路线理解各段的连接：

1. InsertMainReadContext写私有捕获记录
2. BindMainReadAdmission固定目标
3. MainWorkspaceEpoch使用调用者已持有的文档锁读取标量。

本文件需要守住的边界：

- 短期准入到期后私有source也必须拒绝
- 不要为取epoch拉整篇正文或读取Node私库。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/main_readings_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/main_readings_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/main_readings_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/main_readings_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:274 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// main_read.go：保留原实现，按业务合并文件。

````

## 第 2 步：InsertMainReadContext

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

该记录只表示只读捕获依据已持久化，不是用户控制动作或可公开查询的业务成功回执。
读失败不会切MAIN指针；原短期准入到期后，私有source和输出都必须拒绝继续使用。

本步声明：`func InsertMainReadContext(ctx context.Context, tx *sql.Tx, operation string, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_READ_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_READ_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_READ_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_READ_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_READ_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;MAIN_READ_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=274:716 -->
````go
// 该记录只表示只读捕获依据已持久化，不是用户控制动作或可公开查询的业务成功回执。
// 读失败不会切MAIN指针；原短期准入到期后，私有source和输出都必须拒绝继续使用。
func InsertMainReadContext(ctx context.Context, tx *sql.Tx, operation string, payload []byte) error {
	return InsertSubmissionTask(ctx, tx, "MAIN_READ_CONTEXT", operation, operation, payload, "SUCCEEDED")
}

````

## 第 3 步：MainReadContext

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func MainReadContext(ctx context.Context, query Queryer, operation string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>MergeArtifact(ctx, query, &quot;MAIN_READ_CONTEXT&quot;, operation)</code> |
| <code>query</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>MergeArtifact(ctx, query, &quot;MAIN_READ_CONTEXT&quot;, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>MergeArtifact(ctx, query, &quot;MAIN_READ_CONTEXT&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>MergeArtifact(ctx, query, &quot;MAIN_READ_CONTEXT&quot;, operation)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>MergeArtifact(ctx, query, &quot;MAIN_READ_CONTEXT&quot;, operation)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return MergeArtifact(ctx, query, &quot;MAIN_READ_CONTEXT&quot;, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=716:878 -->
````go
func MainReadContext(ctx context.Context, query Queryer, operation string) ([]byte, error) {
	return MergeArtifact(ctx, query, "MAIN_READ_CONTEXT", operation)
}

````

## 第 4 步：BindMainReadAdmission

插入位置：第 3 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func BindMainReadAdmission(ctx context.Context, tx *sql.Tx, admission, operation string, target []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET target_json=? WHERE admission_id=? AND operation_id=? AND purpose=&#x27;READ_MAIN&#x27; AND state=&#x27;OPEN&#x27;&quot;, string(target), admission, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET target_json=? WHERE admission_id=? AND operation_id=? AND purpose=&#x27;READ_MAIN&#x27; AND state=&#x27;OPEN&#x27;&quot;, string(target), admission, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET target_json=? WHERE admission_id=? AND operation_id=? AND purpose=&#x27;READ_MAIN&#x27; AND state=&#x27;OPEN&#x27;&quot;, string(target), admission, operation)</code> |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>sql.ErrNoRows</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET target_json=? WHERE admission_id=? AND operation_id=? AND purpose=&#x27;READ_MAIN&#x27; AND state=&#x27;OPEN&#x27;&quot;, string(target), admission, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>count != 1</code>。条件成立时执行 <code>return sql.ErrNoRows</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=878:1420 -->
````go
func BindMainReadAdmission(ctx context.Context, tx *sql.Tx, admission, operation string, target []byte) error {
	result, err := tx.ExecContext(ctx, "UPDATE document_admission SET target_json=? WHERE admission_id=? AND operation_id=? AND purpose='READ_MAIN' AND state='OPEN'", string(target), admission, operation)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return sql.ErrNoRows
	}
	return nil
}

// main_workspace.go：保留原实现，按业务合并文件。

````

## 第 5 步：MainWorkspaceEpoch

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

MainWorkspaceEpoch复用调用方已经持有的Doc锁和事务，仅取原固定安装的标量。
不能为取得一个epoch再次拉出整篇正文，也不能查询协作服务私有数据库。

本步声明：`func MainWorkspaceEpoch(ctx context.Context, tx *sql.Tx, document uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT m.id,m.structure_id,m.style_id,d.control_revision,m.product_key,m.parent_id IS NOT NULL, 		JSON_UNQUOTE(JSON_EXTRACT(f.payload,&#x27;$.manifest&#x27;)), 		JSO …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT m.id,m.structure_id,m.style_id,d.control_revision,m.product_key,m.parent_id IS NOT NULL, 		JSON_UNQUOTE(JSON_EXTRACT(f.payload,&#x27;$.manifest&#x27;)), 		JSO …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>domain.ResolveMainWorkspace(fact)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>0</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>domain.ResolveMainWorkspace(fact)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_WORKSPACE_BINDING_UNKNOWN&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT m.id,m.structure_id,m.style_id,d.control_revision,m.product_key,m.parent_id IS NOT NULL, 		JSON_UNQUOTE(JSON_EXTRACT(f.payload,&#x27;$.manifest&#x27;)), 		JSO …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT m.id,m.structure_id,m.style_id,d.control_revision,m.product_key,m.parent_id IS NOT NULL, 		JSON_UNQUOTE(JSON_EXTRACT(f.payload,&#x27;$.manifest&#x27;)), 		JSO …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 检查 <code>!value.raw.Valid</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_WORKSPACE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return domain.ResolveMainWorkspace(fact)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1420:4098 -->
````go
// MainWorkspaceEpoch复用调用方已经持有的Doc锁和事务，仅取原固定安装的标量。
// 不能为取得一个epoch再次拉出整篇正文，也不能查询协作服务私有数据库。
func MainWorkspaceEpoch(ctx context.Context, tx *sql.Tx, document uint64) (uint64, error) {
	var fact domain.MainWorkspaceFact
	var manifest, structure, style, control sql.NullString
	var fixed, succeeded, installed, matched sql.NullBool
	err := tx.QueryRowContext(ctx, `SELECT m.id,m.structure_id,m.style_id,d.control_revision,m.product_key,m.parent_id IS NOT NULL,
		JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.manifest')),
		JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.Structure')),
		JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.Style')),
		JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.Control')),
		f.state='SUCCEEDED',o.state='SUCCEEDED',
		completed.state='SUCCEEDED' AND JSON_UNQUOTE(JSON_EXTRACT(completed.payload,'$.manifest_id'))=CAST(m.id AS CHAR),
		JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.context.operation_id'))=m.source_operation_id
		AND JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.context.target.document_id'))=CAST(d.id AS CHAR)
		AND FROM_BASE64(JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.context.request_fingerprint')))=o.request_fingerprint
		FROM document d JOIN main_manifest m ON m.id=d.current_manifest_id AND m.document_id=d.id
		LEFT JOIN durable_task f ON f.kind='MAIN_COMMIT_FIXED' AND f.business_key=m.source_operation_id AND f.operation_id=m.source_operation_id
		LEFT JOIN operation_receipt o ON o.operation_id=m.source_operation_id
		LEFT JOIN durable_task completed ON completed.kind='MAIN_INSTALL_COMPLETED' AND completed.business_key=m.source_operation_id AND completed.operation_id=m.source_operation_id
		WHERE d.id=?`, document).Scan(&fact.Manifest, &fact.Structure, &fact.Style, &fact.MetadataControl, &fact.Product, &fact.Parent,
		&manifest, &structure, &style, &control, &fixed, &succeeded, &installed, &matched)
	if err != nil {
		return 0, err
	}
	values := []struct {
		raw    sql.NullString
		target *uint64
	}{
		{manifest, &fact.RecordedManifest}, {structure, &fact.RecordedStructure}, {style, &fact.RecordedStyle}, {control, &fact.Control},
	}
	for _, value := range values {
		if !value.raw.Valid {
			continue
		}
		parsed, err := domain.CanonicalID(value.raw.String)
		if err != nil {
			return 0, domain.Violation(domain.TemporarilyUnavailable, "MAIN_WORKSPACE_BINDING_UNKNOWN")
		}
		*value.target = parsed
	}
	fact.Fixed, fact.Succeeded, fact.Installed, fact.OriginalMatched = fixed.Valid && fixed.Bool, succeeded.Valid && succeeded.Bool, installed.Valid && installed.Bool, matched.Valid && matched.Bool
	return domain.ResolveMainWorkspace(fact)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/main_readings_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
