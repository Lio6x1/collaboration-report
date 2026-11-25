# 分步编写 `services/document/internal/repository/document/resources_queries.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/171-resources-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久资源保护、OPERATION/DRAFT引用，以及清理后的分配对账。

正文写入前先保护附件，可防GC在Node提交与Document引用确认之间删除对象。

写之前先按这条执行路线理解各段的连接：

1. LockedProtectionAdmission读取EDIT准入
2. InsertResourceProtection/Binding固定UPDATE
3. InsertOperationResourceRef暂时保留
4. FinalizeResourceProtection落定
5. ReconcileCleanedResourceAllocations在Node CLEANED后清理。

本文件需要守住的边界：

- 配额锁必须先于资源锁
- 恢复要发现迁入但已无引用的分配，不能仅查原上传资源。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/resources_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/resources_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/resources_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/resources_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:357 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"database/sql"
)

// resource_allocation_cleanup.go：保留原实现，按业务合并文件。

````

## 第 2 步：LockDocumentResourceAllocationCleanup

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

调用方已锁清理 Doc/KB；必须在任何资源锁之前调用。原上传墓碑不是消费引用，
但已转入且没有剩余引用的 allocation 仍须发现，以便未知提交后的原号恢复。

本步声明：`func LockDocumentResourceAllocationCleanup(ctx context.Context, tx *sql.Tx, document uint64) ([]domain.TransferResource, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document)</code>；<code>TrashSpaceRow(ctx, tx, space, true)</code>；<code>tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space)</code>；<code>resources.LockQuota(ctx, tx, space, limit)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>TrashSpaceRow(ctx, tx, space, true)</code>；<code>resources.LockQuota(ctx, tx, space, limit)</code>；<code>tx.QueryContext</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document)</code>；<code>tx.QueryContext(ctx, `SELECT DISTINCT id FROM (  SELECT resource_id id FROM resource_reference WHERE document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;)  UNION SELEC …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.TransferResource</code> | 使用 <code>domain.TransferResource</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document).Scan(&amp;space)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space).Scan(&amp;limit)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, `SELECT DISTINCT id FROM (  SELECT resource_id id FROM resource_reference WHERE document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;)  UNION SELEC …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;r.ID)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return items, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=357:2306 -->
````go
// 调用方已锁清理 Doc/KB；必须在任何资源锁之前调用。原上传墓碑不是消费引用，
// 但已转入且没有剩余引用的 allocation 仍须发现，以便未知提交后的原号恢复。
func LockDocumentResourceAllocationCleanup(ctx context.Context, tx *sql.Tx, document uint64) ([]domain.TransferResource, error) {
	var space, limit uint64
	if err := tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document).Scan(&space); err != nil {
		return nil, err
	}
	// Account/组织清理时空间已 CLOSING/PUBLIC_RETAINED；它由上层原清理范围
	// 授权，不能复用只接纳 ACTIVE 空间的普通正文写入口。
	if _, err := TrashSpaceRow(ctx, tx, space, true); err != nil {
		return nil, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space).Scan(&limit); err != nil {
		return nil, err
	}
	if _, err := resources.LockQuota(ctx, tx, space, limit); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT id FROM (
 SELECT resource_id id FROM resource_reference WHERE document_id=? AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD')
 UNION SELECT j.id FROM resource_protection p JOIN JSON_TABLE(p.resource_ids,'$[*]' COLUMNS(id BIGINT UNSIGNED PATH '$')) j WHERE p.document_id=? AND p.state<>'RELEASED'
 UNION SELECT resource_id id FROM document_resource_allocation WHERE document_id=? AND state='ACTIVE') x ORDER BY id`, document, document, document)
	if err != nil {
		return nil, err
	}
	var items []domain.TransferResource
	for rows.Next() {
		var r domain.TransferResource
		if err = rows.Scan(&r.ID); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, r := range items {
		if _, _, err = LockedResourceSpace(ctx, tx, r.ID); err != nil {
			return nil, err
		}
	}
	return items, nil
}

````

## 第 3 步：ReconcileCleanedResourceAllocations

插入位置：第 2 步代码之后，紧接追加；完成稿约第 60 行。

仅真实 Node CLEANED 后的历史/保留 pin 清理调用；事务为 READ COMMITTED。

本步声明：`func ReconcileCleanedResourceAllocations(ctx context.Context, tx *sql.Tx, document, space uint64, items []domain.TransferResource) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space)</code>；<code>resources.ReleaseUnusedDocumentAllocations(ctx, tx, document, space, limit, items)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>resources.ReleaseUnusedDocumentAllocations(ctx, tx, document, space, limit, items)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>resources.ReleaseUnusedDocumentAllocations(ctx, tx, document, space, limit, items)</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space)</code>；<code>resources.ReleaseUnusedDocumentAllocations(ctx, tx, document, space, limit, items)</code> |
| <code>items</code> | <code>&#91;&#93;domain.TransferResource</code> | 使用 <code>domain.TransferResource</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>在此声明中的实际使用：<code>resources.ReleaseUnusedDocumentAllocations(ctx, tx, document, space, limit, items)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>resources.ReleaseUnusedDocumentAllocations(ctx, tx, document, space, limit, items)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space).Scan(&amp;limit)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space)</code>。
- 此处通过 <code>return resources.ReleaseUnusedDocumentAllocations(ctx, tx, document, space, limit, items)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2306:2865 -->
````go
// 仅真实 Node CLEANED 后的历史/保留 pin 清理调用；事务为 READ COMMITTED。
func ReconcileCleanedResourceAllocations(ctx context.Context, tx *sql.Tx, document, space uint64, items []domain.TransferResource) error {
	var limit uint64
	if err := tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space).Scan(&limit); err != nil {
		return err
	}
	return resources.ReleaseUnusedDocumentAllocations(ctx, tx, document, space, limit, items)
}

// resource_protection.go：保留原实现，按业务合并文件。

````

## 第 4 步：ResourceProtectionBinding

插入位置：第 3 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func ResourceProtectionBinding(ctx context.Context, q Queryer, operation string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;RESOURCE_PROTECTION_BINDING&#x27; AND business_key=?&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;RESOURCE_PROTECTION_BINDING&#x27; AND business_key=?&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;RESOURCE_PROTECTION_BINDING&#x27; AND business_key=?&quot;, operation).Scan(&amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;RESOURCE_PROTECTION_BINDING&#x27; AND business_key=?&quot;, operation)</code>。
- 此处通过 <code>return raw, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2865:3149 -->
````go
func ResourceProtectionBinding(ctx context.Context, q Queryer, operation string) ([]byte, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='RESOURCE_PROTECTION_BINDING' AND business_key=?", operation).Scan(&raw)
	return raw, err
}

````

## 第 5 步：LockedProtectionAdmission

插入位置：第 4 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func LockedProtectionAdmission(ctx context.Context, tx *sql.Tx, admission string) (domain.EditingAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.EditingAdmission</code> | 使用 <code>domain.EditingAdmission</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission).Scan(&amp;current.Actor, &amp;current.TargetJSON, &amp;current.State, &amp;current.Expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission)</code>。
- 此处通过 <code>return current, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3149:3549 -->
````go
func LockedProtectionAdmission(ctx context.Context, tx *sql.Tx, admission string) (domain.EditingAdmission, error) {
	var current domain.EditingAdmission
	err := tx.QueryRowContext(ctx, "SELECT actor_id,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE", admission).Scan(&current.Actor, &current.TargetJSON, &current.State, &current.Expires)
	return current, err
}
````

## 第 6 步：LockedResourceProtection

插入位置：第 5 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func LockedResourceProtection(ctx context.Context, tx *sql.Tx, operation string) (domain.ProtectionState, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT request_fingerprint,state FROM resource_protection WHERE operation_id=? FOR UPDATE&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT request_fingerprint,state FROM resource_protection WHERE operation_id=? FOR UPDATE&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ProtectionState</code> | 使用 <code>domain.ProtectionState</code> 类型；其定义处的职责：表达草稿正文计量、资源保护绑定及Node提交证明。<br>实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT request_fingerprint,state FROM resource_protection WHERE operation_id=? FOR UPDATE&quot;, operation).Scan(&amp;current.Fingerprint, &amp;current.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT request_fingerprint,state FROM resource_protection WHERE operation_id=? FOR UPDATE&quot;, operation)</code>。
- 此处通过 <code>return current, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3549:3902 -->
````go
func LockedResourceProtection(ctx context.Context, tx *sql.Tx, operation string) (domain.ProtectionState, error) {
	var current domain.ProtectionState
	err := tx.QueryRowContext(ctx, "SELECT request_fingerprint,state FROM resource_protection WHERE operation_id=? FOR UPDATE", operation).Scan(&current.Fingerprint, &current.State)
	return current, err
}
````

## 第 7 步：LockedResourceProtectionState

插入位置：第 6 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`func LockedResourceProtectionState(ctx context.Context, tx *sql.Tx, operation string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM resource_protection WHERE operation_id=? FOR UPDATE&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM resource_protection WHERE operation_id=? FOR UPDATE&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM resource_protection WHERE operation_id=? FOR UPDATE&quot;, operation).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM resource_protection WHERE operation_id=? FOR UPDATE&quot;, operation)</code>。
- 此处通过 <code>return state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3902:4174 -->
````go
func LockedResourceProtectionState(ctx context.Context, tx *sql.Tx, operation string) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM resource_protection WHERE operation_id=? FOR UPDATE", operation).Scan(&state)
	return state, err
}
````

## 第 8 步：LockedResourceDocument

插入位置：第 7 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`func LockedResourceDocument(ctx context.Context, tx *sql.Tx, document uint64) (uint64, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle FROM document WHERE id=? FOR UPDATE&quot;, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle FROM document WHERE id=? FOR UPDATE&quot;, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>space</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>lifecycle</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle FROM document WHERE id=? FOR UPDATE&quot;, document).Scan(&amp;space, &amp;lifecycle)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle FROM document WHERE id=? FOR UPDATE&quot;, document)</code>。
- 此处通过 <code>return space, lifecycle, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4174:4482 -->
````go
func LockedResourceDocument(ctx context.Context, tx *sql.Tx, document uint64) (uint64, string, error) {
	var space uint64
	var lifecycle string
	err := tx.QueryRowContext(ctx, "SELECT space_id,lifecycle FROM document WHERE id=? FOR UPDATE", document).Scan(&space, &lifecycle)
	return space, lifecycle, err
}
````

## 第 9 步：LockedResourceSpace

插入位置：第 8 步代码之后，紧接追加；完成稿约第 98 行。

本步声明：`func LockedResourceSpace(ctx context.Context, tx *sql.Tx, resource uint64) (uint64, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT space_id,state FROM resource WHERE id=? FOR UPDATE&quot;, resource)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT space_id,state FROM resource WHERE id=? FOR UPDATE&quot;, resource)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>space</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT space_id,state FROM resource WHERE id=? FOR UPDATE&quot;, resource).Scan(&amp;space, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT space_id,state FROM resource WHERE id=? FOR UPDATE&quot;, resource)</code>。
- 此处通过 <code>return space, state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4482:4771 -->
````go
func LockedResourceSpace(ctx context.Context, tx *sql.Tx, resource uint64) (uint64, string, error) {
	var space uint64
	var state string
	err := tx.QueryRowContext(ctx, "SELECT space_id,state FROM resource WHERE id=? FOR UPDATE", resource).Scan(&space, &state)
	return space, state, err
}
````

## 第 10 步：LockedResourceState

插入位置：第 9 步代码之后，紧接追加；完成稿约第 104 行。

本步声明：`func LockedResourceState(ctx context.Context, tx *sql.Tx, resource uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM resource WHERE id=? FOR UPDATE&quot;, resource)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM resource WHERE id=? FOR UPDATE&quot;, resource)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM resource WHERE id=? FOR UPDATE&quot;, resource).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM resource WHERE id=? FOR UPDATE&quot;, resource)</code>。
- 此处通过 <code>return state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4771:5010 -->
````go
func LockedResourceState(ctx context.Context, tx *sql.Tx, resource uint64) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM resource WHERE id=? FOR UPDATE", resource).Scan(&state)
	return state, err
}
````

## 第 11 步：InsertResourceProtection

插入位置：第 10 步代码之后，紧接追加；完成稿约第 109 行。

本步声明：`func InsertResourceProtection(ctx context.Context, tx *sql.Tx, record domain.ProtectionRecord) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_protection(operation_id,document_id,room_id,grant_id,workspace_epoch,generation,request_fingerprint,resource_ids,source_epochs,state) VALU …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>record</code> | <code>domain.ProtectionRecord</code> | 使用 <code>domain.ProtectionRecord</code> 类型；其定义处的职责：表达草稿正文计量、资源保护绑定及Node提交证明。<br>在此声明中的实际使用：<code>record.Operation</code>；<code>record.Document</code>；<code>record.Room</code>；<code>record.Grant</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO resource_protection(operation_id,document_id,room_id,grant_id,workspace_epoch,generation,request_fingerprint,resource_ids,source_epochs,state) VALU …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5010:5509 -->
````go
func InsertResourceProtection(ctx context.Context, tx *sql.Tx, record domain.ProtectionRecord) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO resource_protection(operation_id,document_id,room_id,grant_id,workspace_epoch,generation,request_fingerprint,resource_ids,source_epochs,state) VALUES(?,?,?,?,?,?,?,?,?,'PROTECTED')", record.Operation, record.Document, record.Room, record.Grant, record.Epoch, record.Generation, record.Fingerprint, record.ResourceJSON, record.SourceJSON)
	return err
}
````

## 第 12 步：InsertResourceProtectionBinding

插入位置：第 11 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`func InsertResourceProtectionBinding(ctx context.Context, tx *sql.Tx, operation string, binding []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;RESOURCE_PROTECTION_BINDING&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;, operation, string(binding))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;RESOURCE_PROTECTION_BINDING&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;, operation, string(binding))</code> |
| <code>binding</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;RESOURCE_PROTECTION_BINDING&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;, operation, string(binding))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5509:5835 -->
````go
func InsertResourceProtectionBinding(ctx context.Context, tx *sql.Tx, operation string, binding []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('RESOURCE_PROTECTION_BINDING',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))", operation, string(binding))
	return err
}
````

## 第 13 步：InsertOperationResourceRef

插入位置：第 12 步代码之后，紧接追加；完成稿约第 117 行。

本步声明：`func InsertOperationResourceRef(ctx context.Context, tx *sql.Tx, resource, document uint64, operation string, generation uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;OPERATION&#x27;,?,&#x27;DRAFT_UPDATE&#x27;,?)&quot;, resource, document, operation, generation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;OPERATION&#x27;,?,&#x27;DRAFT_UPDATE&#x27;,?)&quot;, resource, document, operation, generation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;OPERATION&#x27;,?,&#x27;DRAFT_UPDATE&#x27;,?)&quot;, resource, document, operation, generation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;OPERATION&#x27;,?,&#x27;DRAFT_UPDATE&#x27;,?)&quot;, resource, document, operation, generation)</code> |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;OPERATION&#x27;,?,&#x27;DRAFT_UPDATE&#x27;,?)&quot;, resource, document, operation, generation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;OPERATION&#x27;,?,&#x27;DRAFT_UPDATE&#x27;,?)&quot;, resource, document, operation, generation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5835:6202 -->
````go
func InsertOperationResourceRef(ctx context.Context, tx *sql.Tx, resource, document uint64, operation string, generation uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,'OPERATION',?,'DRAFT_UPDATE',?)", resource, document, operation, generation)
	return err
}
````

## 第 14 步：InsertDraftResourceRef

插入位置：第 13 步代码之后，紧接追加；完成稿约第 121 行。

本步声明：`func InsertDraftResourceRef(ctx context.Context, tx *sql.Tx, resource, document uint64, owner string, generation uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;DRAFT&#x27;,?,&#x27;ROOM_DRAFT&#x27;,?) ON DUPLICATE KEY UPDATE ge …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;DRAFT&#x27;,?,&#x27;ROOM_DRAFT&#x27;,?) ON DUPLICATE KEY UPDATE ge …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;DRAFT&#x27;,?,&#x27;ROOM_DRAFT&#x27;,?) ON DUPLICATE KEY UPDATE ge …</code>（长表达式，按本步完整代码继续） |
| <code>owner</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;DRAFT&#x27;,?,&#x27;ROOM_DRAFT&#x27;,?) ON DUPLICATE KEY UPDATE ge …</code>（长表达式，按本步完整代码继续） |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;DRAFT&#x27;,?,&#x27;ROOM_DRAFT&#x27;,?) ON DUPLICATE KEY UPDATE ge …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,&#x27;DRAFT&#x27;,?,&#x27;ROOM_DRAFT&#x27;,?) ON DUPLICATE KEY UPDATE ge …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6202:6626 -->
````go
func InsertDraftResourceRef(ctx context.Context, tx *sql.Tx, resource, document uint64, owner string, generation uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,'DRAFT',?,'ROOM_DRAFT',?) ON DUPLICATE KEY UPDATE generation=GREATEST(generation,VALUES(generation))", resource, document, owner, generation)
	return err
}
````

## 第 15 步：DeleteOperationResourceRefs

插入位置：第 14 步代码之后，紧接追加；完成稿约第 125 行。

本步声明：`func DeleteOperationResourceRefs(ctx context.Context, tx *sql.Tx, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;DELETE FROM resource_reference WHERE owner_kind=&#x27;OPERATION&#x27; AND owner_ref=? AND purpose=&#x27;DRAFT_UPDATE&#x27;&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;DELETE FROM resource_reference WHERE owner_kind=&#x27;OPERATION&#x27; AND owner_ref=? AND purpose=&#x27;DRAFT_UPDATE&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM resource_reference WHERE owner_kind=&#x27;OPERATION&#x27; AND owner_ref=? AND purpose=&#x27;DRAFT_UPDATE&#x27;&quot;, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6626:6880 -->
````go
func DeleteOperationResourceRefs(ctx context.Context, tx *sql.Tx, operation string) error {
	_, err := tx.ExecContext(ctx, "DELETE FROM resource_reference WHERE owner_kind='OPERATION' AND owner_ref=? AND purpose='DRAFT_UPDATE'", operation)
	return err
}
````

## 第 16 步：FinalizeResourceProtection

插入位置：第 15 步代码之后，紧接追加；完成稿约第 129 行。

本步声明：`func FinalizeResourceProtection(ctx context.Context, tx *sql.Tx, operation, state, receipt string, seq uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE resource_protection SET state=?,node_receipt_ref=?,durable_seq=? WHERE operation_id=?&quot;, state, receipt, seq, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE resource_protection SET state=?,node_receipt_ref=?,durable_seq=? WHERE operation_id=?&quot;, state, receipt, seq, operation)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE resource_protection SET state=?,node_receipt_ref=?,durable_seq=? WHERE operation_id=?&quot;, state, receipt, seq, operation)</code> |
| <code>receipt</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE resource_protection SET state=?,node_receipt_ref=?,durable_seq=? WHERE operation_id=?&quot;, state, receipt, seq, operation)</code> |
| <code>seq</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE resource_protection SET state=?,node_receipt_ref=?,durable_seq=? WHERE operation_id=?&quot;, state, receipt, seq, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE resource_protection SET state=?,node_receipt_ref=?,durable_seq=? WHERE operation_id=?&quot;, state, receipt, seq, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6880:7173 -->
````go
func FinalizeResourceProtection(ctx context.Context, tx *sql.Tx, operation, state, receipt string, seq uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE resource_protection SET state=?,node_receipt_ref=?,durable_seq=? WHERE operation_id=?", state, receipt, seq, operation)
	return err
}

````

## 第 17 步：ResourceState

插入位置：第 16 步代码之后，紧接追加；完成稿约第 134 行。

本步声明：`func ResourceState(ctx context.Context, q Queryer, resource uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT state FROM resource WHERE id=?&quot;, resource)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT state FROM resource WHERE id=?&quot;, resource)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT state FROM resource WHERE id=?&quot;, resource).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT state FROM resource WHERE id=?&quot;, resource)</code>。
- 此处通过 <code>return state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=7173:7393 -->
````go
func ResourceState(ctx context.Context, q Queryer, resource uint64) (string, error) {
	var state string
	err := q.QueryRowContext(ctx, "SELECT state FROM resource WHERE id=?", resource).Scan(&state)
	return state, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/resources_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
