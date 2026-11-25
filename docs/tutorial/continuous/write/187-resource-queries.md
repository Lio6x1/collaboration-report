# 分步编写 `services/document/internal/repository/resource/resource_queries.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/187-resource-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现上传、引用、跨空间分配、正文高水位及配额结算的SQL规则。

真实字节、空间计费归属与引用是三种事实；同一共享对象不能因一次GC或转让重复退款。

写之前先按这条执行路线理解各段的连接：

1. LockQuota后ReserveQuota
2. READY转换与SettleQuota共同提交
3. ReserveDraftBody按room/epoch高水位预留
4. MarkReadyDeleting持锁重验无引用
5. Transfer资源按空间编号锁配额后重新计量。

本文件需要守住的边界：

- UNKNOWN不按TTL退款
- 候选列表不是删除授权
- RecordCommittedAllocation不得复活已迁出的RELEASED分配。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/resource' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/resource/resource_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/resource/resource_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/resource/resource_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/resource/resource_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:388 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package resource

import (
	"bytes"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/go-sql-driver/mysql"
	"sort"
	"strconv"
	"strings"
	"time"
)

// allocated_quota.go：保留原实现，按业务合并文件。

````

## 第 2 步：RecordCommittedAllocation

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

READY 与真实 COMMITTED 上传结算同事务建立原分配，不增加第二笔 used。
已迁出的 RELEASED 分配不能由旧上传重试重新激活。

本步声明：`func RecordCommittedAllocation(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id)  SELECT r.id,r.space_id,r.byte_size,&#x27;ACTIVE&#x27;,r.upload_operation_id FROM re …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id)  SELECT r.id,r.space_id,r.byte_size,&#x27;ACTIVE&#x27;,r.upload_operation_id FROM re …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id)  SELECT r.id,r.space_id,r.byte_size,&#x27;ACTIVE&#x27;,r.upload_operation_id FROM re …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=388:1105 -->
````go
// READY 与真实 COMMITTED 上传结算同事务建立原分配，不增加第二笔 used。
// 已迁出的 RELEASED 分配不能由旧上传重试重新激活。
func RecordCommittedAllocation(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id)
 SELECT r.id,r.space_id,r.byte_size,'ACTIVE',r.upload_operation_id FROM resource r
 JOIN quota_reservation q ON q.operation_id=r.upload_operation_id AND q.space_id=r.space_id AND q.bytes=r.byte_size AND q.state='COMMITTED'
 WHERE r.id=? AND r.state IN ('READY','DELETING')
 ON DUPLICATE KEY UPDATE resource_id=VALUES(resource_id)`, id)
	return err
}

````

## 第 3 步：LockResourceAllocationQuotas

插入位置：第 2 步代码之后，紧接追加；完成稿约第 31 行。

当前资源状态转换之前按空间编号锁全部真实分配；物理原空间也需保留上传原收据锁序。
后续资源锁下再次核集合，新增跨空间分配不能绕过尚未持有的 quota 锁。

本步声明：`func LockResourceAllocationQuotas(ctx context.Context, tx *sql.Tx, id, physical, limit uint64) (map[uint64]Quota, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ResourceAllocationSpaces(ctx, tx, id)</code>；<code>LockQuota(ctx, tx, space, limit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ResourceAllocationSpaces(ctx, tx, id)</code>；<code>LockQuota(ctx, tx, space, limit)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>ResourceAllocationSpaces(ctx, tx, id)</code> |
| <code>physical</code> | <code>uint64</code> | 在此声明中的实际使用：<code>append(ids, physical)</code> |
| <code>limit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockQuota(ctx, tx, space, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;uint64&#93;Quota</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>exists</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1105:1885 -->
````go
// 当前资源状态转换之前按空间编号锁全部真实分配；物理原空间也需保留上传原收据锁序。
// 后续资源锁下再次核集合，新增跨空间分配不能绕过尚未持有的 quota 锁。
func LockResourceAllocationQuotas(ctx context.Context, tx *sql.Tx, id, physical, limit uint64) (map[uint64]Quota, error) {
	ids, err := ResourceAllocationSpaces(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	ids = append(ids, physical)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	result := map[uint64]Quota{}
	for _, space := range ids {
		if _, exists := result[space]; exists {
			continue
		}
		q, err := LockQuota(ctx, tx, space, limit)
		if err != nil {
			return nil, err
		}
		result[space] = q
	}
	return result, nil
}

````

## 第 4 步：ValidateResourceAllocationQuotas

插入位置：第 3 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`func ValidateResourceAllocationQuotas(ctx context.Context, tx *sql.Tx, id uint64, quotas map[uint64]Quota) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT space_id FROM resource_space_allocation WHERE resource_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY space_id FOR UPDATE`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT space_id FROM resource_space_allocation WHERE resource_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY space_id FOR UPDATE`, id)</code> |
| <code>quotas</code> | <code>map&#91;uint64&#93;Quota</code> | 在此声明中的实际使用：<code>_, exists := quotas&#91;space&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;RESOURCE_ALLOCATION_SET_CHANGED&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT space_id FROM resource_space_allocation WHERE resource_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY space_id FOR UPDATE`, id)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;space)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>!exists</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_ALLOCATION_SET_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1885:2437 -->
````go
func ValidateResourceAllocationQuotas(ctx context.Context, tx *sql.Tx, id uint64, quotas map[uint64]Quota) error {
	rows, err := tx.QueryContext(ctx, `SELECT space_id FROM resource_space_allocation WHERE resource_id=? AND state='ACTIVE' ORDER BY space_id FOR UPDATE`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var space uint64
		if err = rows.Scan(&space); err != nil {
			return err
		}
		if _, exists := quotas[space]; !exists {
			return errors.New("RESOURCE_ALLOCATION_SET_CHANGED")
		}
	}
	return rows.Err()
}

````

## 第 5 步：ResourceTransferEvidence

插入位置：第 4 步代码之后，紧接追加；完成稿约第 72 行。

清理资格可沿已接受的固定转让追溯，包含最终物理 DELETED 后的同号重试；
普通读取/新增保护另用 ResourceAllowedForDocument，仍须 ACTIVE 真实计费分配。

本步声明：`func ResourceTransferEvidence(ctx context.Context, tx *sql.Tx, id, document, space uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM document_resource_allocation a  JOIN personal_transfer t ON t.id=a.transfer_id AND t.document_id=a.document_id AND t.state=&#x27;ACC …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM document_resource_allocation a  JOIN personal_transfer t ON t.id=a.transfer_id AND t.document_id=a.document_id AND t.state=&#x27;ACC …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM document_resource_allocation a  JOIN personal_transfer t ON t.id=a.transfer_id AND t.document_id=a.document_id AND t.state=&#x27;ACC …</code>（长表达式，按本步完整代码继续） |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM document_resource_allocation a  JOIN personal_transfer t ON t.id=a.transfer_id AND t.document_id=a.document_id AND t.state=&#x27;ACC …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>valid</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM document_resource_allocation a  JOIN personal_transfer t ON t.id=a.transfer_id AND t.document_id=a.document_id AND t.state=&#x27;ACC …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM document_resource_allocation a  JOIN personal_transfer t ON t.id=a.transfer_id AND t.document_id=a.document_id AND t.state=&#x27;ACC …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return valid, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2437:3148 -->
````go
// 清理资格可沿已接受的固定转让追溯，包含最终物理 DELETED 后的同号重试；
// 普通读取/新增保护另用 ResourceAllowedForDocument，仍须 ACTIVE 真实计费分配。
func ResourceTransferEvidence(ctx context.Context, tx *sql.Tx, id, document, space uint64) (bool, error) {
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM document_resource_allocation a
 JOIN personal_transfer t ON t.id=a.transfer_id AND t.document_id=a.document_id AND t.state='ACCEPTED'
 WHERE a.resource_id=? AND a.document_id=? AND a.space_id=?)`, id, document, space).Scan(&valid)
	return valid, err
}

// allocation_cleanup.go：保留原实现，按业务合并文件。

````

## 第 6 步：ReleaseUnusedDocumentAllocations

插入位置：第 5 步代码之后，紧接追加；完成稿约第 84 行。

调用方在真实 CLEANED 后持清理 Doc、Space/quota 与排序资源锁，且使用 READ
COMMITTED。迁入/迁出在改 Doc.space 之前均锁双方 quota；持此空间 quota 就
固定了消费者经过此空间的范围。资源锁又阻止新增 pin，无需在 quota 后锁其他
Doc。引用、保护与 Doc.space 使用 READ COMMITTED 的新语句快照；不能在资源
之后加外文档 protection 锁，否则与其保护确认的 protection→resource 锁序相反。

本步声明：`func ReleaseUnusedDocumentAllocations(ctx context.Context, tx *sql.Tx, document, space, limit uint64, items []domain.TransferResource) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockQuota(ctx, tx, space, limit)</code>；<code>tx.QueryRowContext(ctx, `SELECT state FROM resource WHERE id=? FOR UPDATE`, item.ID)</code>；<code>TransferResourceConsumerDocuments(ctx, tx, &#91;&#93;domain.TransferResource{item}, false)</code>；<code>tx.QueryRowContext(ctx, `SELECT operation_id FROM resource_protection WHERE state=&#x27;UNKNOWN&#x27; AND JSON_CONTAINS(resource_ids,CAST(? AS JSON)) LIMIT 1`, domain.TextID(item.ID))</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockQuota(ctx, tx, space, limit)</code>；<code>tx.QueryRowContext</code>；<code>TransferResourceConsumerDocuments(ctx, tx, &#91;&#93;domain.TransferResource{item}, false)</code>；<code>tx.ExecContext</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>consumer == document</code>；<code>tx.ExecContext(ctx, `UPDATE document_resource_allocation SET state=&#x27;RELEASED&#x27; WHERE resource_id=? AND document_id=? AND space_id=? AND state=&#x27;ACTIVE&#x27;`, item.ID, document, space)</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockQuota(ctx, tx, space, limit)</code>；<code>currentSpace == space</code>；<code>tx.ExecContext(ctx, `UPDATE document_resource_allocation SET state=&#x27;RELEASED&#x27; WHERE resource_id=? AND document_id=? AND space_id=? AND state=&#x27;ACTIVE&#x27;`, item.ID, document, space)</code>；<code>tx.QueryRowContext(ctx, `SELECT bytes FROM resource_space_allocation WHERE resource_id=? AND space_id=? AND state=&#x27;ACTIVE&#x27; FOR UPDATE`, item.ID, space)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>limit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockQuota(ctx, tx, space, limit)</code> |
| <code>items</code> | <code>&#91;&#93;domain.TransferResource</code> | 使用 <code>domain.TransferResource</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_ALLOCATION_PROTECTION_UNKNOWN&quot;)</code>；<code>errors.New(&quot;RESOURCE_ALLOCATION_QUOTA_INCONSISTENT&quot;)</code>；<code>errors.New(&quot;RESOURCE_ALLOCATION_CLEANUP_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT state FROM resource WHERE id=? FOR UPDATE`, item.ID).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT state FROM resource WHERE id=? FOR UPDATE`, item.ID)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT operation_id FROM resource_protection WHERE state=&#x27;UNKNOWN&#x27; AND JSON_CONTAINS(resource_ids,CAST(? AS JSON)) LIMIT 1`, domain.TextID(item.ID)).Scan(&amp;unknown)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT operation_id FROM resource_protection WHERE state=&#x27;UNKNOWN&#x27; AND JSON_CONTAINS(resource_ids,CAST(? AS JSON)) LIMIT 1`, domain.TextID(item.ID))</code>。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RESOURCE_ALLOCATION_PROTECTION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, consumer).Scan(&amp;currentSpace)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, consumer)</code>。
- 检查 <code>own</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE document_resource_allocation SET state=&#x27;RELEASED&#x27; WHERE resource_id=? AND document_id=? AND space_id=? AND state=&#x27;ACTIVE&#x27;`, item.ID, document, space)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>scoped &#124;&#124; len(consumers) == 0 &#124;&#124; state != &quot;READY&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。

<!-- write-step: 6 bytes=3148:5376 -->
````go
// 调用方在真实 CLEANED 后持清理 Doc、Space/quota 与排序资源锁，且使用 READ
// COMMITTED。迁入/迁出在改 Doc.space 之前均锁双方 quota；持此空间 quota 就
// 固定了消费者经过此空间的范围。资源锁又阻止新增 pin，无需在 quota 后锁其他
// Doc。引用、保护与 Doc.space 使用 READ COMMITTED 的新语句快照；不能在资源
// 之后加外文档 protection 锁，否则与其保护确认的 protection→resource 锁序相反。
func ReleaseUnusedDocumentAllocations(ctx context.Context, tx *sql.Tx, document, space, limit uint64, items []domain.TransferResource) error {
	for _, item := range items {
		q, err := LockQuota(ctx, tx, space, limit)
		if err != nil {
			return err
		}
		var state string
		if err = tx.QueryRowContext(ctx, `SELECT state FROM resource WHERE id=? FOR UPDATE`, item.ID).Scan(&state); err != nil {
			return err
		}
		consumers, err := TransferResourceConsumerDocuments(ctx, tx, []domain.TransferResource{item}, false)
		if err != nil {
			return err
		}
		var unknown string
		err = tx.QueryRowContext(ctx, `SELECT operation_id FROM resource_protection WHERE state='UNKNOWN' AND JSON_CONTAINS(resource_ids,CAST(? AS JSON)) LIMIT 1`, domain.TextID(item.ID)).Scan(&unknown)
		if err == nil {
			return domain.Violation(domain.StateConflict, "RESOURCE_ALLOCATION_PROTECTION_UNKNOWN")
		}
		if err != sql.ErrNoRows {
			return err
		}
		own, scoped := false, false
		for _, consumer := range consumers {
			var currentSpace uint64
			if err = tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, consumer).Scan(&currentSpace); err != nil {
				return err
			}
			own = own || consumer == document
			scoped = scoped || currentSpace == space
		}
		if own {
			continue
		}
		if _, err = tx.ExecContext(ctx, `UPDATE document_resource_allocation SET state='RELEASED' WHERE resource_id=? AND document_id=? AND space_id=? AND state='ACTIVE'`, item.ID, document, space); err != nil {
			return err
		}
		// 无消费者的物理对象仍沿原 GC 真正 DELETED 后结算全部分配。
		// 本分支只释放其他空间真实共享保留下的空计费范围。
		if scoped || len(consumers) == 0 || state != "READY" {
````

## 第 7 步：ReleaseUnusedDocumentAllocations（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 129 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT bytes FROM resource_space_allocation WHERE resource_id=? AND space_id=? AND state=&#x27;ACTIVE&#x27; FOR UPDATE`, item.ID, space).Scan(&amp;paid)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT bytes FROM resource_space_allocation WHERE resource_id=? AND space_id=? AND state=&#x27;ACTIVE&#x27; FOR UPDATE`, item.ID, space)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>paid &gt; q.Used</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_ALLOCATION_QUOTA_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state=&#x27;RELEASED&#x27; WHERE resource_id=? AND space_id=? AND state=&#x27;ACTIVE&#x27; AND bytes=?`, item.ID, space, paid)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; changed != 1</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_ALLOCATION_CLEANUP_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=5376:6275 -->
````go
			continue
		}
		var paid uint64
		err = tx.QueryRowContext(ctx, `SELECT bytes FROM resource_space_allocation WHERE resource_id=? AND space_id=? AND state='ACTIVE' FOR UPDATE`, item.ID, space).Scan(&paid)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		if paid > q.Used {
			return errors.New("RESOURCE_ALLOCATION_QUOTA_INCONSISTENT")
		}
		result, err := tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state='RELEASED' WHERE resource_id=? AND space_id=? AND state='ACTIVE' AND bytes=?`, item.ID, space, paid)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return errors.New("RESOURCE_ALLOCATION_CLEANUP_CHANGED")
		}
		if err = writeQuota(ctx, tx, q, q.Used-paid, q.Reserved); err != nil {
			return err
		}
	}
	return nil
}

// cleanup.go：保留原实现，按业务合并文件。

````

## 第 8 步：CleanupCandidates

插入位置：第 7 步代码之后，紧接追加；完成稿约第 159 行。

CleanupCandidates 只发现未完成上传的过期回收任务，不承担 READY 共享资源 GC。
候选读取不是删除授权；业务层持上传互斥锁，再在事务内锁配额和资源重新核验。

本步声明：`func CleanupCandidates(ctx context.Context, db *sql.DB, limit int) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT r.id FROM resource r JOIN quota_reservation q ON q.operation_id=r.upload_operation_id 		WHERE q.state=&#x27;RESERVED&#x27; AND ((r.state=&#x27;UPLOADING&#x27; AND q.expire …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>limit &lt; 1</code>；<code>limit = 1</code>；<code>limit &gt; 32</code>；<code>limit = 32</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RESOURCE_CLEANUP_READ_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>limit &lt; 1</code>。
- 检查 <code>limit &gt; 32</code>。
- 执行 <code>db.QueryContext(ctx, `SELECT r.id FROM resource r JOIN quota_reservation q ON q.operation_id=r.upload_operation_id 		WHERE q.state=&#x27;RESERVED&#x27; AND ((r.state=&#x27;UPLOADING&#x27; AND q.expire …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;RESOURCE_CLEANUP_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;RESOURCE_CLEANUP_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;RESOURCE_CLEANUP_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return ids, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=6275:7316 -->
````go
// CleanupCandidates 只发现未完成上传的过期回收任务，不承担 READY 共享资源 GC。
// 候选读取不是删除授权；业务层持上传互斥锁，再在事务内锁配额和资源重新核验。
func CleanupCandidates(ctx context.Context, db *sql.DB, limit int) ([]uint64, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 32 {
		limit = 32
	}
	rows, err := db.QueryContext(ctx, `SELECT r.id FROM resource r JOIN quota_reservation q ON q.operation_id=r.upload_operation_id
		WHERE q.state='RESERVED' AND ((r.state='UPLOADING' AND q.expires_at<=UTC_TIMESTAMP(6)) OR r.state='DELETING') ORDER BY r.id LIMIT ?`, limit)
	if err != nil {
		return nil, errors.New("RESOURCE_CLEANUP_READ_FAILED")
	}
	defer rows.Close()
	ids := make([]uint64, 0, limit)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, errors.New("RESOURCE_CLEANUP_READ_FAILED")
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		return nil, errors.New("RESOURCE_CLEANUP_READ_FAILED")
	}
	return ids, nil
}

````

## 第 9 步：MarkDeleting

插入位置：第 8 步代码之后，紧接追加；完成稿约第 188 行。

MarkDeleting 的 u 必须来自同一事务加锁的 UploadRow，并已锁住空间配额。
只允许数据库时间已过期且仍 RESERVED 的 UPLOADING 转换；任何共享引用或
未释放保护均阻止删除。原 UPLOAD 引用保留供恢复核对，不视作共享引用。

本步声明：`func MarkDeleting(ctx context.Context, tx *sql.Tx, u Upload) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE resource r SET r.state=&#x27;DELETING&#x27;,r.revision=r.revision+1 		WHERE r.id=? AND r.state=&#x27;UPLOADING&#x27; AND r.revision=? AND r.revision&lt;18446744073709551615 		 …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>u</code> | <code>Upload</code> | 在此声明中的实际使用：<code>u.State</code>；<code>u.ID</code>；<code>u.Revision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RESOURCE_UPLOAD_CHANGED&quot;)</code>；<code>errors.New(&quot;RESOURCE_DELETING_WRITE_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>u.State != &quot;UPLOADING&quot;</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_UPLOAD_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE resource r SET r.state=&#x27;DELETING&#x27;,r.revision=r.revision+1 		WHERE r.id=? AND r.state=&#x27;UPLOADING&#x27; AND r.revision=? AND r.revision&lt;18446744073709551615 		 …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_DELETING_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_UPLOAD_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=7316:8658 -->
````go
// MarkDeleting 的 u 必须来自同一事务加锁的 UploadRow，并已锁住空间配额。
// 只允许数据库时间已过期且仍 RESERVED 的 UPLOADING 转换；任何共享引用或
// 未释放保护均阻止删除。原 UPLOAD 引用保留供恢复核对，不视作共享引用。
func MarkDeleting(ctx context.Context, tx *sql.Tx, u Upload) error {
	if u.State != "UPLOADING" {
		return errors.New("RESOURCE_UPLOAD_CHANGED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE resource r SET r.state='DELETING',r.revision=r.revision+1
		WHERE r.id=? AND r.state='UPLOADING' AND r.revision=? AND r.revision<18446744073709551615
		AND EXISTS (SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id AND q.state='RESERVED' AND q.expires_at<=UTC_TIMESTAMP(6))
		AND NOT EXISTS (SELECT 1 FROM resource_reference x WHERE x.resource_id=r.id AND NOT (x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'))
		AND NOT EXISTS (SELECT 1 FROM resource_protection p WHERE p.state<>'RELEASED' AND JSON_CONTAINS(p.resource_ids,CAST(? AS JSON)))`, u.ID, u.Revision, strconv.FormatUint(u.ID, 10))
	if err != nil {
		return errors.New("RESOURCE_DELETING_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("RESOURCE_UPLOAD_CHANGED")
	}
	return nil
}

````

## 第 10 步：MarkDeleted

插入位置：第 9 步代码之后，紧接追加；完成稿约第 209 行。

MarkDeleted 仅在业务层已删除完整文件及临时文件后调用；与配额和暂存预算释放
同事务提交。保留原 UPLOAD 引用与预留记录，使提交结果未知时仍可恢复、审计。

本步声明：`func MarkDeleted(ctx context.Context, tx *sql.Tx, u Upload) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE resource SET state=&#x27;DELETED&#x27;,revision=revision+1 WHERE id=? AND state=&#x27;DELETING&#x27; AND revision=? AND revision&lt;18446744073709551615`, u.ID, u.Revision)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>u</code> | <code>Upload</code> | 在此声明中的实际使用：<code>u.State</code>；<code>u.ID</code>；<code>u.Revision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RESOURCE_UPLOAD_CHANGED&quot;)</code>；<code>errors.New(&quot;RESOURCE_DELETED_WRITE_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>u.State != &quot;DELETING&quot;</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_UPLOAD_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE resource SET state=&#x27;DELETED&#x27;,revision=revision+1 WHERE id=? AND state=&#x27;DELETING&#x27; AND revision=? AND revision&lt;18446744073709551615`, u.ID, u.Revision)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_DELETED_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_UPLOAD_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=8658:9484 -->
````go
// MarkDeleted 仅在业务层已删除完整文件及临时文件后调用；与配额和暂存预算释放
// 同事务提交。保留原 UPLOAD 引用与预留记录，使提交结果未知时仍可恢复、审计。
func MarkDeleted(ctx context.Context, tx *sql.Tx, u Upload) error {
	if u.State != "DELETING" {
		return errors.New("RESOURCE_UPLOAD_CHANGED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE resource SET state='DELETED',revision=revision+1 WHERE id=? AND state='DELETING' AND revision=? AND revision<18446744073709551615`, u.ID, u.Revision)
	if err != nil {
		return errors.New("RESOURCE_DELETED_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("RESOURCE_UPLOAD_CHANGED")
	}
	return nil
}

// comment_context.go：保留原实现，按业务合并文件。

````

## 第 11 步：BranchCommentResourceVersions

插入位置：第 10 步代码之后，紧接追加；完成稿约第 227 行。

BranchCommentResourceVersions 只列出未删除讨论原版本中的图片引用。
分支谱系归属仍由批注仓库的 ContextVersion 统一核验，不能将同单元任意版本放行。

本步声明：`func BranchCommentResourceVersions(ctx context.Context, tx *sql.Tx, document, branch, resource uint64) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT DISTINCT v.id FROM comment_thread h 		JOIN content_version v ON v.id=h.content_version_id AND v.document_id=h.document_id 		JOIN resource_reference r O …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT DISTINCT v.id FROM comment_thread h 		JOIN content_version v ON v.id=h.content_version_id AND v.document_id=h.document_id 		JOIN resource_reference r O …</code>（长表达式，按本步完整代码继续） |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT DISTINCT v.id FROM comment_thread h 		JOIN content_version v ON v.id=h.content_version_id AND v.document_id=h.document_id 		JOIN resource_reference r O …</code>（长表达式，按本步完整代码继续） |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT DISTINCT v.id FROM comment_thread h 		JOIN content_version v ON v.id=h.content_version_id AND v.document_id=h.document_id 		JOIN resource_reference r O …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>versions</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>contextReadError(err)</code>；<code>contextReadError(rows.Err())</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT DISTINCT v.id FROM comment_thread h 		JOIN content_version v ON v.id=h.content_version_id AND v.document_id=h.document_id 		JOIN resource_reference r O …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, contextReadError(err)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, contextReadError(err)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;version)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return versions, contextReadError(rows.Err())</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=9484:10599 -->
````go
// BranchCommentResourceVersions 只列出未删除讨论原版本中的图片引用。
// 分支谱系归属仍由批注仓库的 ContextVersion 统一核验，不能将同单元任意版本放行。
func BranchCommentResourceVersions(ctx context.Context, tx *sql.Tx, document, branch, resource uint64) ([]uint64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT v.id FROM comment_thread h
		JOIN content_version v ON v.id=h.content_version_id AND v.document_id=h.document_id
		JOIN resource_reference r ON r.document_id=v.document_id AND r.owner_kind='VERSION'
		AND r.owner_ref=CAST(v.id AS CHAR) AND r.purpose='CONTENT' AND r.resource_id=?
		WHERE h.document_id=? AND h.context_kind='BRANCH' AND h.context_ref=CAST(? AS CHAR) AND h.status<>'DELETED'`, resource, document, branch)
	if err != nil {
		return nil, contextReadError(err)
	}
	defer rows.Close()
	versions := []uint64{}
	for rows.Next() {
		var version uint64
		if err := rows.Scan(&version); err != nil {
			return nil, contextReadError(err)
		}
		versions = append(versions, version)
	}
	return versions, contextReadError(rows.Err())
}

````

## 第 12 步：MainCommentResourceUnits

插入位置：第 11 步代码之后，紧接追加；完成稿约第 250 行。

历史MAIN图片必须仍被一个未删除讨论的原锚点版本实际引用；同清单中的
其他单元和后来版本不能借这个讨论获得下载资格。

本步声明：`func MainCommentResourceUnits(ctx context.Context, tx *sql.Tx, document, manifest, resource uint64) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT DISTINCT v.unit_id FROM comment_thread h 		JOIN manifest_entry e ON e.document_id=h.document_id AND CAST(e.manifest_id AS CHAR)=h.context_ref AND e.con …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT DISTINCT v.unit_id FROM comment_thread h 		JOIN manifest_entry e ON e.document_id=h.document_id AND CAST(e.manifest_id AS CHAR)=h.context_ref AND e.con …</code>（长表达式，按本步完整代码继续） |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT DISTINCT v.unit_id FROM comment_thread h 		JOIN manifest_entry e ON e.document_id=h.document_id AND CAST(e.manifest_id AS CHAR)=h.context_ref AND e.con …</code>（长表达式，按本步完整代码继续） |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT DISTINCT v.unit_id FROM comment_thread h 		JOIN manifest_entry e ON e.document_id=h.document_id AND CAST(e.manifest_id AS CHAR)=h.context_ref AND e.con …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>units</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>contextReadError(err)</code>；<code>contextReadError(rows.Err())</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT DISTINCT v.unit_id FROM comment_thread h 		JOIN manifest_entry e ON e.document_id=h.document_id AND CAST(e.manifest_id AS CHAR)=h.context_ref AND e.con …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, contextReadError(err)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, contextReadError(err)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;unit)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return units, contextReadError(rows.Err())</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=10599:11908 -->
````go
// 历史MAIN图片必须仍被一个未删除讨论的原锚点版本实际引用；同清单中的
// 其他单元和后来版本不能借这个讨论获得下载资格。
func MainCommentResourceUnits(ctx context.Context, tx *sql.Tx, document, manifest, resource uint64) ([]uint64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT v.unit_id FROM comment_thread h
		JOIN manifest_entry e ON e.document_id=h.document_id AND CAST(e.manifest_id AS CHAR)=h.context_ref AND e.content_version_id=h.content_version_id
		JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit_id
		JOIN resource_reference r ON r.document_id=v.document_id AND r.owner_kind='VERSION'
		AND r.owner_ref=CAST(v.id AS CHAR) AND r.purpose='CONTENT' AND r.resource_id=?
		WHERE h.document_id=? AND h.context_kind='MAIN' AND h.context_ref=CAST(? AS CHAR) AND h.status<>'DELETED'`, resource, document, manifest)
	if err != nil {
		return nil, contextReadError(err)
	}
	defer rows.Close()
	units := []uint64{}
	for rows.Next() {
		var unit uint64
		if err := rows.Scan(&unit); err != nil {
			return nil, contextReadError(err)
		}
		units = append(units, unit)
	}
	return units, contextReadError(rows.Err())
}

// conflict_context.go：保留原实现，按业务合并文件。

````

## 第 13 步：ConflictResourceOperation

插入位置：第 12 步代码之后，紧接追加；完成稿约第 276 行。

ConflictResourceOperation 只定位同轮次已物化的冲突版本候选，不以历史正文扫描
或任意旧合并记录授权下载。业务层还须验证原 attempt、固定输入和版本摘要。
JSON 数字以十进制字符串比较，避免大 uint64 被浮点或隐式数值比较截断。

本步声明：`func ConflictResourceOperation(ctx context.Context, tx *sql.Tx, document, round, resource uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT m.operation_id FROM merge_operation m 		JOIN review_request r ON r.id=m.request_id AND r.document_id=? 		JOIN review_round v ON v.id=m.round_id AND  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT m.operation_id FROM merge_operation m 		JOIN review_request r ON r.id=m.request_id AND r.document_id=? 		JOIN review_round v ON v.id=m.round_id AND  …</code>（长表达式，按本步完整代码继续） |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT m.operation_id FROM merge_operation m 		JOIN review_request r ON r.id=m.request_id AND r.document_id=? 		JOIN review_round v ON v.id=m.round_id AND  …</code>（长表达式，按本步完整代码继续） |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT m.operation_id FROM merge_operation m 		JOIN review_request r ON r.id=m.request_id AND r.document_id=? 		JOIN review_round v ON v.id=m.round_id AND  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>operation</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>contextReadError(err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT m.operation_id FROM merge_operation m 		JOIN review_request r ON r.id=m.request_id AND r.document_id=? 		JOIN review_round v ON v.id=m.round_id AND  …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT m.operation_id FROM merge_operation m 		JOIN review_request r ON r.id=m.request_id AND r.document_id=? 		JOIN review_round v ON v.id=m.round_id AND  …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return operation, contextReadError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=11908:13409 -->
````go
// ConflictResourceOperation 只定位同轮次已物化的冲突版本候选，不以历史正文扫描
// 或任意旧合并记录授权下载。业务层还须验证原 attempt、固定输入和版本摘要。
// JSON 数字以十进制字符串比较，避免大 uint64 被浮点或隐式数值比较截断。
func ConflictResourceOperation(ctx context.Context, tx *sql.Tx, document, round, resource uint64) (string, error) {
	var operation string
	err := tx.QueryRowContext(ctx, `SELECT m.operation_id FROM merge_operation m
		JOIN review_request r ON r.id=m.request_id AND r.document_id=?
		JOIN review_round v ON v.id=m.round_id AND v.request_id=r.id AND v.document_id=r.document_id
		JOIN operation_receipt o ON o.operation_id=m.operation_id AND o.kind='MERGE'
			AND o.state='REJECTED' AND o.expires_at>UTC_TIMESTAMP(6)
		JOIN durable_task t ON t.operation_id=m.operation_id AND t.kind='CONFLICT_CONTEXT_VERSIONS' AND t.state='SUCCEEDED'
		JOIN resource_reference x ON x.document_id=r.document_id AND x.resource_id=?
			AND x.owner_kind='VERSION' AND x.purpose='CONTENT'
			AND x.owner_ref IN (JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.Base')),
				JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.Main')),JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.Submitted')))
		WHERE m.round_id=? AND m.state='REJECTED'
		ORDER BY m.created_at DESC,m.operation_id LIMIT 1`, document, resource, round).Scan(&operation)
	return operation, contextReadError(err)
}

// context.go：保留原实现，按业务合并文件。

````

## 第 14 步：Object

插入位置：第 13 步代码之后，紧接追加；完成稿约第 298 行。

本步声明：`type Object struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 资源对象编号，正文引用此编号。 |
| <code>Space</code> | <code>uint64</code> | 资源记录的物理归属空间。 |
| <code>Size</code> | <code>uint64</code> | 实际对象字节数。 |
| <code>Revision</code> | <code>uint64</code> | 资源记录修订号。 |
| <code>Key</code> | <code>string</code> | 存储后端对象键，不是可公开访问的URL。 |
| <code>MediaType</code> | <code>string</code> | 经过校验的媒体类型，用于下载响应。 |
| <code>State</code> | <code>string</code> | 资源上传、可用或清理等生命周期状态。 |
| <code>Checksum</code> | <code>&#91;32&#93;byte</code> | 完整对象SHA-256摘要，读取和提交时防止内容被替换。 |

<!-- write-step: 14 bytes=13409:13537 -->
````go
type Object struct {
	ID, Space, Size, Revision uint64
	Key, MediaType, State     string
	Checksum                  [32]byte
}

````

## 第 15 步：ObjectRow

插入位置：第 14 步代码之后，紧接追加；完成稿约第 304 行。

ObjectRow 只读取对象事实；上传来源不能替代下载上下文的引用与权限证明。
下载持共享锁，防止检查 READY 后资源状态在同一事务内被回收方改变。

本步声明：`func ObjectRow(ctx context.Context, q Queryer, id uint64, lock bool) (Object, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR SHARE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Object</code> | 实际返回表达式：<code>object</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>contextReadError(err)</code>；<code>errors.New(&quot;RESOURCE_OBJECT_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, query, id).Scan(&amp;object.ID, &amp;object.Space, &amp;object.Size, &amp;object.Revision, &amp;object.Key, &amp;object.MediaType, &amp;object.State, &amp;checksum)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, query, id)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return object, contextReadError(err)</code>。当前函数在这里结束。
- 检查 <code>len(checksum) != len(object.Checksum) &#124;&#124; object.Revision == 0</code>。条件成立时执行 <code>return object, errors.New(&quot;RESOURCE_OBJECT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return object, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=13537:14425 -->
````go
// ObjectRow 只读取对象事实；上传来源不能替代下载上下文的引用与权限证明。
// 下载持共享锁，防止检查 READY 后资源状态在同一事务内被回收方改变。
func ObjectRow(ctx context.Context, q Queryer, id uint64, lock bool) (Object, error) {
	var object Object
	var checksum []byte
	query := `SELECT id,space_id,byte_size,revision,storage_key,media_type,state,checksum FROM resource WHERE id=?`
	if lock {
		query += " FOR SHARE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&object.ID, &object.Space, &object.Size, &object.Revision, &object.Key, &object.MediaType, &object.State, &checksum)
	if err != nil {
		return object, contextReadError(err)
	}
	if len(checksum) != len(object.Checksum) || object.Revision == 0 {
		return object, errors.New("RESOURCE_OBJECT_INVALID")
	}
	copy(object.Checksum[:], checksum)
	return object, nil
}

````

## 第 16 步：ManifestContains

插入位置：第 15 步代码之后，紧接追加；完成稿约第 324 行。

ManifestContains 只承认清单固定版本的 CONTENT 引用，避免把同文档的草稿或
其他历史版本附件泄露到当前阅读上下文。字符串比较避免 owner_ref 数字隐式转换。

本步声明：`func ManifestContains(ctx context.Context, tx *sql.Tx, document, manifest, resource uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM manifest_entry e 		JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM manifest_entry e 		JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit …</code>（长表达式，按本步完整代码继续） |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM manifest_entry e 		JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit …</code>（长表达式，按本步完整代码继续） |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM manifest_entry e 		JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>found</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>contextReadError(err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM manifest_entry e 		JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM manifest_entry e 		JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return found, contextReadError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=14425:15252 -->
````go
// ManifestContains 只承认清单固定版本的 CONTENT 引用，避免把同文档的草稿或
// 其他历史版本附件泄露到当前阅读上下文。字符串比较避免 owner_ref 数字隐式转换。
func ManifestContains(ctx context.Context, tx *sql.Tx, document, manifest, resource uint64) (bool, error) {
	var found bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM manifest_entry e
		JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit_id
		JOIN resource_reference r ON r.document_id=e.document_id AND r.owner_kind='VERSION'
			AND r.owner_ref=CAST(v.id AS CHAR) AND r.purpose='CONTENT' AND r.resource_id=?
		WHERE e.document_id=? AND e.manifest_id=?)`, resource, document, manifest).Scan(&found)
	return found, contextReadError(err)
}

````

## 第 17 步：VersionContains

插入位置：第 16 步代码之后，紧接追加；完成稿约第 337 行。

本步声明：`func VersionContains(ctx context.Context, tx *sql.Tx, document, version, resource uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM content_version v JOIN resource_reference r 		ON r.document_id=v.document_id AND r.owner_kind=&#x27;VERSION&#x27; AND r.owner_ref=CAS …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM content_version v JOIN resource_reference r 		ON r.document_id=v.document_id AND r.owner_kind=&#x27;VERSION&#x27; AND r.owner_ref=CAS …</code>（长表达式，按本步完整代码继续） |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM content_version v JOIN resource_reference r 		ON r.document_id=v.document_id AND r.owner_kind=&#x27;VERSION&#x27; AND r.owner_ref=CAS …</code>（长表达式，按本步完整代码继续） |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM content_version v JOIN resource_reference r 		ON r.document_id=v.document_id AND r.owner_kind=&#x27;VERSION&#x27; AND r.owner_ref=CAS …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>found</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>contextReadError(err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM content_version v JOIN resource_reference r 		ON r.document_id=v.document_id AND r.owner_kind=&#x27;VERSION&#x27; AND r.owner_ref=CAS …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS ( 		SELECT 1 FROM content_version v JOIN resource_reference r 		ON r.document_id=v.document_id AND r.owner_kind=&#x27;VERSION&#x27; AND r.owner_ref=CAS …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return found, contextReadError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=15252:15742 -->
````go
func VersionContains(ctx context.Context, tx *sql.Tx, document, version, resource uint64) (bool, error) {
	var found bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM content_version v JOIN resource_reference r
		ON r.document_id=v.document_id AND r.owner_kind='VERSION' AND r.owner_ref=CAST(v.id AS CHAR)
		AND r.purpose='CONTENT' AND r.resource_id=? WHERE v.document_id=? AND v.id=?)`, resource, document, version).Scan(&found)
	return found, contextReadError(err)
}

````

## 第 18 步：MainManifestAccessible

插入位置：第 17 步代码之后，紧接追加；完成稿约第 346 行。

MainManifestAccessible 只证明调用者仍持有该固定清单的有效阅读视图。
当前 MAIN 指针与访问权限由业务层在文档锁下判断；这里不能放宽为任意历史清单。

本步声明：`func MainManifestAccessible(ctx context.Context, tx *sql.Tx, document, manifest, actor uint64, session []byte) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT 1 FROM read_view v 		JOIN document_admission a ON a.admission_id=v.admission_id AND a.document_id=v.document_id 		WHERE v.document_id=? AND v.manife …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT 1 FROM read_view v 		JOIN document_admission a ON a.admission_id=v.admission_id AND a.document_id=v.document_id 		WHERE v.document_id=? AND v.manife …</code>（长表达式，按本步完整代码继续） |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT 1 FROM read_view v 		JOIN document_admission a ON a.admission_id=v.admission_id AND a.document_id=v.document_id 		WHERE v.document_id=? AND v.manife …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT 1 FROM read_view v 		JOIN document_admission a ON a.admission_id=v.admission_id AND a.document_id=v.document_id 		WHERE v.document_id=? AND v.manife …</code>（长表达式，按本步完整代码继续） |
| <code>session</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT 1 FROM read_view v 		JOIN document_admission a ON a.admission_id=v.admission_id AND a.document_id=v.document_id 		WHERE v.document_id=? AND v.manife …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>contextReadError(err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT 1 FROM read_view v 		JOIN document_admission a ON a.admission_id=v.admission_id AND a.document_id=v.document_id 		WHERE v.document_id=? AND v.manife …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT 1 FROM read_view v 		JOIN document_admission a ON a.admission_id=v.admission_id AND a.document_id=v.document_id 		WHERE v.document_id=? AND v.manife …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil, contextReadError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=15742:16599 -->
````go
// MainManifestAccessible 只证明调用者仍持有该固定清单的有效阅读视图。
// 当前 MAIN 指针与访问权限由业务层在文档锁下判断；这里不能放宽为任意历史清单。
func MainManifestAccessible(ctx context.Context, tx *sql.Tx, document, manifest, actor uint64, session []byte) (bool, error) {
	var found int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM read_view v
		JOIN document_admission a ON a.admission_id=v.admission_id AND a.document_id=v.document_id
		WHERE v.document_id=? AND v.manifest_id=? AND v.state='OPEN' AND v.expires_at>UTC_TIMESTAMP(6)
		AND a.state='OPEN' AND a.expires_at>UTC_TIMESTAMP(6) AND a.actor_id=? AND a.session_ref=?
		LIMIT 1 FOR SHARE`, document, manifest, actor, session).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, contextReadError(err)
}

````

## 第 19 步：ReviewContext

插入位置：第 18 步代码之后，紧接追加；完成稿约第 361 行。

本步声明：`func ReviewContext(ctx context.Context, tx *sql.Tx, document, round uint64) (request, version, baseVersion uint64, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT request_id,content_version_id,base_version_id FROM review_round WHERE document_id=? AND id=?`, document, round)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT request_id,content_version_id,base_version_id FROM review_round WHERE document_id=? AND id=?`, document, round)</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT request_id,content_version_id,base_version_id FROM review_round WHERE document_id=? AND id=?`, document, round)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>request</code> | <code>uint64</code> | 实际返回表达式：<code>request</code> |
| <code>version</code> | <code>uint64</code> | 实际返回表达式：<code>version</code> |
| <code>baseVersion</code> | <code>uint64</code> | 实际返回表达式：<code>baseVersion</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>contextReadError(err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT request_id,content_version_id,base_version_id FROM review_round WHERE document_id=? AND id=?`, document, round).Scan(&amp;request, &amp;version, &amp;baseVersion)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT request_id,content_version_id,base_version_id FROM review_round WHERE document_id=? AND id=?`, document, round)</code>。
- 此处通过 <code>return request, version, baseVersion, contextReadError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=16599:16981 -->
````go
func ReviewContext(ctx context.Context, tx *sql.Tx, document, round uint64) (request, version, baseVersion uint64, err error) {
	err = tx.QueryRowContext(ctx, `SELECT request_id,content_version_id,base_version_id FROM review_round WHERE document_id=? AND id=?`, document, round).Scan(&request, &version, &baseVersion)
	return request, version, baseVersion, contextReadError(err)
}

````

## 第 20 步：Branch

插入位置：第 19 步代码之后，紧接追加；完成稿约第 366 行。

本步声明：`type Branch struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Unit</code> | <code>uint64</code> | 支线所属协作单元。 |
| <code>Epoch</code> | <code>uint64</code> | 支线工作区代次，旧房间不能继续写。 |
| <code>BaseVersion</code> | <code>uint64</code> | 支线基线正文版本，用于判断图片引用关系。 |
| <code>Room</code> | <code>uint64</code> | 真实协作房间编号，由已成功房间任务核验。 |

<!-- write-step: 20 bytes=16981:17071 -->
````go
type Branch struct {
	Unit, Epoch, BaseVersion uint64
	Room                     uint64
}

````

## 第 21 步：BranchContext

插入位置：第 20 步代码之后，紧接追加；完成稿约第 371 行。

BranchContext 在业务层文档锁之后锁当前分支事实，严格绑定现行谱系与单元。
房间尚未创建不影响基线附件读取；存在但损坏的登记必须报错，不能当作没有草稿。

本步声明：`func BranchContext(ctx context.Context, tx *sql.Tx, document, branch uint64) (Branch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT b.unit_id,b.workspace_epoch,l.base_version_id 		FROM branch b JOIN collaboration_unit u ON u.id=b.unit_id AND u.document_id=b.document_id AND u.acti …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind=&#x27;COLLABORATION_ROOM&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27; FOR SHARE`, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT b.unit_id,b.workspace_epoch,l.base_version_id 		FROM branch b JOIN collaboration_unit u ON u.id=b.unit_id AND u.document_id=b.document_id AND u.acti …</code>（长表达式，按本步完整代码继续） |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT b.unit_id,b.workspace_epoch,l.base_version_id 		FROM branch b JOIN collaboration_unit u ON u.id=b.unit_id AND u.document_id=b.document_id AND u.acti …</code>（长表达式，按本步完整代码继续）；<code>strconv.FormatUint(branch, 10)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Branch</code> | 实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>contextReadError(err)</code>；<code>nil</code>；<code>errors.New(&quot;RESOURCE_ROOM_BINDING_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT b.unit_id,b.workspace_epoch,l.base_version_id 		FROM branch b JOIN collaboration_unit u ON u.id=b.unit_id AND u.document_id=b.document_id AND u.acti …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT b.unit_id,b.workspace_epoch,l.base_version_id 		FROM branch b JOIN collaboration_unit u ON u.id=b.unit_id AND u.document_id=b.document_id AND u.acti …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, contextReadError(err)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind=&#x27;COLLABORATION_ROOM&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27; FOR SHARE`, key).Scan(&amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind=&#x27;COLLABORATION_ROOM&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27; FOR SHARE`, key)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return current, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, contextReadError(err)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;payload) != nil</code>。条件成立时执行 <code>return current, errors.New(&quot;RESOURCE_ROOM_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; room == 0 &#124;&#124; strconv.FormatUint(room, 10) != payload.Room</code>。条件成立时执行 <code>return current, errors.New(&quot;RESOURCE_ROOM_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return current, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=17071:18830 -->
````go
// BranchContext 在业务层文档锁之后锁当前分支事实，严格绑定现行谱系与单元。
// 房间尚未创建不影响基线附件读取；存在但损坏的登记必须报错，不能当作没有草稿。
func BranchContext(ctx context.Context, tx *sql.Tx, document, branch uint64) (Branch, error) {
	var current Branch
	err := tx.QueryRowContext(ctx, `SELECT b.unit_id,b.workspace_epoch,l.base_version_id
		FROM branch b JOIN collaboration_unit u ON u.id=b.unit_id AND u.document_id=b.document_id AND u.active_branch_id=b.id
		JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id AND l.document_id=b.document_id AND l.unit_id=b.unit_id
		WHERE b.id=? AND b.document_id=? AND b.lifecycle='ACTIVE' AND u.retired_structure_id IS NULL FOR SHARE`, branch, document).Scan(&current.Unit, &current.Epoch, &current.BaseVersion)
	if err != nil {
		return current, contextReadError(err)
	}
	key := "BRANCH:" + strconv.FormatUint(branch, 10) + ":" + strconv.FormatUint(current.Unit, 10) + ":" + strconv.FormatUint(current.Epoch, 10)
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind='COLLABORATION_ROOM' AND business_key=? AND state='SUCCEEDED' FOR SHARE`, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return current, nil
	}
	if err != nil {
		return current, contextReadError(err)
	}
	var payload struct {
		Room string `json:"room_id"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return current, errors.New("RESOURCE_ROOM_BINDING_INVALID")
	}
	room, err := strconv.ParseUint(payload.Room, 10, 64)
	if err != nil || room == 0 || strconv.FormatUint(room, 10) != payload.Room {
		return current, errors.New("RESOURCE_ROOM_BINDING_INVALID")
	}
	current.Room = room
	return current, nil
}

````

## 第 22 步：DraftContains

插入位置：第 21 步代码之后，紧接追加；完成稿约第 405 行。

DraftContains 必须使用持久房间身份和当前 epoch，旧房间引用不能授权新工作区。

本步声明：`func DraftContains(ctx context.Context, tx *sql.Tx, document, room, epoch, resource uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM resource_reference 		WHERE document_id=? AND resource_id=? AND owner_kind=&#x27;DRAFT&#x27; AND owner_ref=? AND purpose=&#x27;ROOM_DRAFT&#x27;)`, document, resource, owner)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM resource_reference 		WHERE document_id=? AND resource_id=? AND owner_kind=&#x27;DRAFT&#x27; AND owner_ref=? AND purpose=&#x27;ROOM_DRAFT&#x27;)`, document, resource, owner)</code> |
| <code>room</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(room, 10)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(epoch, 10)</code> |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM resource_reference 		WHERE document_id=? AND resource_id=? AND owner_kind=&#x27;DRAFT&#x27; AND owner_ref=? AND purpose=&#x27;ROOM_DRAFT&#x27;)`, document, resource, owner)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>found</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>contextReadError(err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM resource_reference 		WHERE document_id=? AND resource_id=? AND owner_kind=&#x27;DRAFT&#x27; AND owner_ref=? AND purpose=&#x27;ROOM_DRAFT&#x27;)`, document, resource, owner).Scan(&amp;found)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM resource_reference 		WHERE document_id=? AND resource_id=? AND owner_kind=&#x27;DRAFT&#x27; AND owner_ref=? AND purpose=&#x27;ROOM_DRAFT&#x27;)`, document, resource, owner)</code>。
- 此处通过 <code>return found, contextReadError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=18830:19407 -->
````go
// DraftContains 必须使用持久房间身份和当前 epoch，旧房间引用不能授权新工作区。
func DraftContains(ctx context.Context, tx *sql.Tx, document, room, epoch, resource uint64) (bool, error) {
	owner := strconv.FormatUint(room, 10) + ":" + strconv.FormatUint(epoch, 10)
	var found bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM resource_reference
		WHERE document_id=? AND resource_id=? AND owner_kind='DRAFT' AND owner_ref=? AND purpose='ROOM_DRAFT')`, document, resource, owner).Scan(&found)
	return found, contextReadError(err)
}

````

## 第 23 步：contextReadError

插入位置：第 22 步代码之后，紧接追加；完成稿约第 414 行。

不把数据库故障误报为不存在；仅真实缺行保留 ErrNoRows 供业务层区分。

本步声明：`func contextReadError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err == nil</code>；<code>err == sql.ErrNoRows</code>；<code>return err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;RESOURCE_CONTEXT_READ_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil &#124;&#124; err == sql.ErrNoRows</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;RESOURCE_CONTEXT_READ_FAILED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=19407:19728 -->
````go
// 不把数据库故障误报为不存在；仅真实缺行保留 ErrNoRows 供业务层区分。
func contextReadError(err error) error {
	if err == nil || err == sql.ErrNoRows {
		return err
	}
	return errors.New("RESOURCE_CONTEXT_READ_FAILED")
}

// document_capacity.go：保留原实现，按业务合并文件。

````

## 第 24 步：ChargeDocumentCapacity

插入位置：第 23 步代码之后，紧接追加；完成稿约第 424 行。

新 canonical 历史正文必须和 INSERT content_version 同事务记账。旧库第一次
遇到写入时先物化已有历史，因此不能用零字节文档代表完整容量已经接通。
调用方已锁 Doc，随后按现有权限→Doc→KB→Space→Quota 顺序调用。

本步声明：`func ChargeDocumentCapacity(ctx context.Context, tx *sql.Tx, document, space, added, limit uint64, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockQuota(ctx, tx, space, limit)</code>；<code>tx.QueryRowContext(ctx, `SELECT space_id,bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document)</code>；<code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document)</code>；<code>writeQuota(ctx, tx, q, q.Used+charge, q.Reserved)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockQuota(ctx, tx, space, limit)</code>；<code>tx.QueryRowContext</code>；<code>writeQuota(ctx, tx, q, q.Used+charge, q.Reserved)</code>；<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id,bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document)</code>；<code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document)</code>；<code>tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=VALUES(bytes),operation_id=VALUES(operati …</code>（长表达式，按本步完整代码继续） |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockQuota(ctx, tx, space, limit)</code>；<code>owner = space</code>；<code>owner != space</code>；<code>tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=VALUES(bytes),operation_id=VALUES(operati …</code>（长表达式，按本步完整代码继续） |
| <code>added</code> | <code>uint64</code> | 在此声明中的实际使用：<code>charge := added</code> |
| <code>limit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockQuota(ctx, tx, space, limit)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=VALUES(bytes),operation_id=VALUES(operati …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;DOCUMENT_CAPACITY_SPACE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;QUOTA_EXCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document).Scan(&amp;owner, &amp;paid)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document).Scan(&amp;paid)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>owner != space</code>。条件成立时执行 <code>return errors.New(&quot;DOCUMENT_CAPACITY_SPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>q.Used &gt; q.Limit &#124;&#124; q.Reserved &gt; q.Limit-q.Used &#124;&#124; charge &gt; q.Limit-q.Used-q.Reserved</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=VALUES(bytes),operation_id=VALUES(operati …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 24 bytes=19728:21516 -->
````go
// 新 canonical 历史正文必须和 INSERT content_version 同事务记账。旧库第一次
// 遇到写入时先物化已有历史，因此不能用零字节文档代表完整容量已经接通。
// 调用方已锁 Doc，随后按现有权限→Doc→KB→Space→Quota 顺序调用。
func ChargeDocumentCapacity(ctx context.Context, tx *sql.Tx, document, space, added, limit uint64, operation string) error {
	q, err := LockQuota(ctx, tx, space, limit)
	if err != nil {
		return err
	}
	var paid, owner uint64
	err = tx.QueryRowContext(ctx, `SELECT space_id,bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document).Scan(&owner, &paid)
	charge := added
	if err == sql.ErrNoRows {
		// helper 在正文 INSERT 后执行，所以历史总量已经包含本次 added。
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document).Scan(&paid); err != nil {
			return err
		}
		charge = paid
		paid = 0
		owner = space
	} else if err != nil {
		return err
	}
	if owner != space {
		return errors.New("DOCUMENT_CAPACITY_SPACE_CHANGED")
	}
	next, err := domain.TransferCapacity(paid, charge)
	if err != nil {
		return err
	}
	if q.Used > q.Limit || q.Reserved > q.Limit-q.Used || charge > q.Limit-q.Used-q.Reserved {
		return domain.Violation(domain.StateConflict, "QUOTA_EXCEEDED")
	}
	if err = writeQuota(ctx, tx, q, q.Used+charge, q.Reserved); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)
 ON DUPLICATE KEY UPDATE bytes=VALUES(bytes),operation_id=VALUES(operation_id)`, document, space, next, operation)
	return err
}

````

## 第 25 步：ReleaseDocumentCapacity

插入位置：第 24 步代码之后，紧接追加；完成稿约第 464 行。

仅在真实永久删除/Account 清理已确认后的原清理事务释放；TRASHED 不释放历史。

本步声明：`func ReleaseDocumentCapacity(ctx context.Context, tx *sql.Tx, document, limit uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id FROM document_capacity WHERE document_id=?`, document)</code>；<code>LockQuota(ctx, tx, space, limit)</code>；<code>tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE`, document, space)</code>；<code>writeQuota(ctx, tx, q, q.Used-bytes, q.Reserved)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>LockQuota(ctx, tx, space, limit)</code>；<code>writeQuota(ctx, tx, q, q.Used-bytes, q.Reserved)</code>；<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id FROM document_capacity WHERE document_id=?`, document)</code>；<code>tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE`, document, space)</code>；<code>tx.ExecContext(ctx, `DELETE FROM document_capacity WHERE document_id=?`, document)</code> |
| <code>limit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockQuota(ctx, tx, space, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>errors.New(&quot;DOCUMENT_CAPACITY_INCONSISTENT&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id FROM document_capacity WHERE document_id=?`, document).Scan(&amp;space)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id FROM document_capacity WHERE document_id=?`, document)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE`, document, space).Scan(&amp;bytes)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE`, document, space)</code>。
- 检查 <code>bytes &gt; q.Used</code>。条件成立时执行 <code>return errors.New(&quot;DOCUMENT_CAPACITY_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `DELETE FROM document_capacity WHERE document_id=?`, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 25 bytes=21516:22550 -->
````go
// 仅在真实永久删除/Account 清理已确认后的原清理事务释放；TRASHED 不释放历史。
func ReleaseDocumentCapacity(ctx context.Context, tx *sql.Tx, document, limit uint64) error {
	var space uint64
	err := tx.QueryRowContext(ctx, `SELECT space_id FROM document_capacity WHERE document_id=?`, document).Scan(&space)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	q, err := LockQuota(ctx, tx, space, limit)
	if err != nil {
		return err
	}
	var bytes uint64
	if err = tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE`, document, space).Scan(&bytes); err != nil {
		return err
	}
	if bytes > q.Used {
		return errors.New("DOCUMENT_CAPACITY_INCONSISTENT")
	}
	if err = writeQuota(ctx, tx, q, q.Used-bytes, q.Reserved); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM document_capacity WHERE document_id=?`, document)
	return err
}

// draft_body_capacity.go：保留原实现，按业务合并文件。

````

## 第 26 步：ReserveDraftBody

插入位置：第 25 步代码之后，紧接追加；完成稿约第 494 行。

调用方先锁来源、准入与 Doc/Space。每个 room/epoch 只增长一次正文高水位；
尚未取得 Node 结算证明的差额也保留在高水位中，多人或迟到确认不会重复预留。

本步声明：`func ReserveDraftBody(ctx context.Context, tx *sql.Tx, q Quota, document, room, epoch uint64, operation string, body domain.DraftBodyMeasure, prior bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,state  FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>；<code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,bytes FROM draft_body_capacity WHERE room_id=? AND workspace_epoch=? FOR UPDATE`, room, epoch)</code>；<code>writeQuota(ctx, tx, q, q.Used, q.Reserved+added)</code>；<code>tx.ExecContext(ctx, `INSERT INTO draft_body_capacity(room_id,workspace_epoch,document_id,space_id,bytes) VALUES(?,?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=VALUES(bytes)`, room, epoch, document, q.Space, high)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>writeQuota(ctx, tx, q, q.Used, q.Reserved+added)</code>；<code>tx.ExecContext</code> |
| <code>q</code> | <code>Quota</code> | 在此声明中的实际使用：<code>q.Space</code>；<code>q.Used</code>；<code>q.Limit</code>；<code>q.Reserved</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>savedDoc != document</code>；<code>owner != document</code>；<code>tx.ExecContext(ctx, `INSERT INTO draft_body_capacity(room_id,workspace_epoch,document_id,space_id,bytes) VALUES(?,?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=VALUES(bytes)`, room, epoch, document, q.Space, high)</code>；<code>tx.ExecContext(ctx, `INSERT INTO draft_body_reservation(operation_id,document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,bytes,state)  VALUES(?,?,?,?,?,?,?,?,&#x27;RESERVE …</code>（长表达式，按本步完整代码继续） |
| <code>room</code> | <code>uint64</code> | 在此声明中的实际使用：<code>savedRoom != room</code>；<code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,bytes FROM draft_body_capacity WHERE room_id=? AND workspace_epoch=? FOR UPDATE`, room, epoch)</code>；<code>tx.ExecContext(ctx, `INSERT INTO draft_body_capacity(room_id,workspace_epoch,document_id,space_id,bytes) VALUES(?,?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=VALUES(bytes)`, room, epoch, document, q.Space, high)</code>；<code>tx.ExecContext(ctx, `INSERT INTO draft_body_reservation(operation_id,document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,bytes,state)  VALUES(?,?,?,?,?,?,?,?,&#x27;RESERVE …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>savedEpoch != epoch</code>；<code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,bytes FROM draft_body_capacity WHERE room_id=? AND workspace_epoch=? FOR UPDATE`, room, epoch)</code>；<code>tx.ExecContext(ctx, `INSERT INTO draft_body_capacity(room_id,workspace_epoch,document_id,space_id,bytes) VALUES(?,?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=VALUES(bytes)`, room, epoch, document, q.Space, high)</code>；<code>tx.ExecContext(ctx, `INSERT INTO draft_body_reservation(operation_id,document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,bytes,state)  VALUES(?,?,?,?,?,?,?,?,&#x27;RESERVE …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,state  FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>；<code>tx.ExecContext(ctx, `INSERT INTO draft_body_reservation(operation_id,document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,bytes,state)  VALUES(?,?,?,?,?,?,?,?,&#x27;RESERVE …</code>（长表达式，按本步完整代码继续） |
| <code>body</code> | <code>domain.DraftBodyMeasure</code> | 使用 <code>domain.DraftBodyMeasure</code> 类型；其定义处的职责：表达草稿正文计量、资源保护绑定及Node提交证明。<br>在此声明中的实际使用：<code>body.Bytes</code>；<code>body.Hash</code> |
| <code>prior</code> | <code>bool</code> | 调用方是否声明此原号已有预留；若true却查不到原预留则报冲突，不能偷偷再创建一份。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;DRAFT_BODY_BINDING_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;DRAFT_BODY_RESERVATION_CLOSED&quot;)</code>；<code>nil</code>；<code>err</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,state  FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation).Sc …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,state  FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>savedDoc != document &#124;&#124; savedSpace != q.Space &#124;&#124; savedRoom != room &#124;&#124; savedEpoch != epoch &#124;&#124; savedBytes != body.Bytes &#124;&#124; !bytes.Equal(savedHash, body.Hash)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DRAFT_BODY_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>state == &quot;RELEASED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DRAFT_BODY_RESERVATION_CLOSED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>prior</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DRAFT_BODY_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,bytes FROM draft_body_capacity WHERE room_id=? AND workspace_epoch=? FOR UPDATE`, room, epoch).Scan(&amp;owner, &amp;scope, &amp;high)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,bytes FROM draft_body_capacity WHERE room_id=? AND workspace_epoch=? FOR UPDATE`, room, epoch)</code>。
- 检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; (owner != document &#124;&#124; scope != q.Space)</code>。条件成立时执行 <code>return errors.New(&quot;DRAFT_BODY_OWNER_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>body.Bytes &gt; high</code>。
- 检查 <code>added &gt; 0 &amp;&amp; (q.Used &gt; q.Limit &#124;&#124; q.Reserved &gt; q.Limit-q.Used &#124;&#124; added &gt; q.Limit-q.Used-q.Reserved)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>body.Bytes &gt; high</code>。

<!-- write-step: 26 bytes=22550:24716 -->
````go
// 调用方先锁来源、准入与 Doc/Space。每个 room/epoch 只增长一次正文高水位；
// 尚未取得 Node 结算证明的差额也保留在高水位中，多人或迟到确认不会重复预留。
func ReserveDraftBody(ctx context.Context, tx *sql.Tx, q Quota, document, room, epoch uint64, operation string, body domain.DraftBodyMeasure, prior bool) error {
	var savedDoc, savedSpace, savedRoom, savedEpoch, savedBytes uint64
	var savedHash []byte
	var state string
	err := tx.QueryRowContext(ctx, `SELECT document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,state
 FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&savedDoc, &savedSpace, &savedRoom, &savedEpoch, &savedBytes, &savedHash, &state)
	if err == nil {
		if savedDoc != document || savedSpace != q.Space || savedRoom != room || savedEpoch != epoch || savedBytes != body.Bytes || !bytes.Equal(savedHash, body.Hash) {
			return domain.Violation(domain.StateConflict, "DRAFT_BODY_BINDING_CHANGED")
		}
		if state == "RELEASED" {
			return domain.Violation(domain.StateConflict, "DRAFT_BODY_RESERVATION_CLOSED")
		}
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	if prior {
		return domain.Violation(domain.StateConflict, "DRAFT_BODY_BINDING_CHANGED")
	}
	var owner, scope, high uint64
	err = tx.QueryRowContext(ctx, `SELECT document_id,space_id,bytes FROM draft_body_capacity WHERE room_id=? AND workspace_epoch=? FOR UPDATE`, room, epoch).Scan(&owner, &scope, &high)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil && (owner != document || scope != q.Space) {
		return errors.New("DRAFT_BODY_OWNER_CHANGED")
	}
	added := uint64(0)
	if body.Bytes > high {
		added = body.Bytes - high
	}
	// 满额/旧库超额仍允许不增加占用的删除编辑；旧日志尚在，不以缩短正文假退款。
	if added > 0 && (q.Used > q.Limit || q.Reserved > q.Limit-q.Used || added > q.Limit-q.Used-q.Reserved) {
		return domain.Violation(domain.StateConflict, "QUOTA_EXCEEDED")
	}
	if err = writeQuota(ctx, tx, q, q.Used, q.Reserved+added); err != nil {
		return err
	}
	if body.Bytes > high {
		high = body.Bytes
	}
````

## 第 27 步：ReserveDraftBody（接着实现）

插入位置：第 26 步代码之后，紧接追加；完成稿约第 539 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO draft_body_capacity(room_id,workspace_epoch,document_id,space_id,bytes) VALUES(?,?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=VALUES(bytes)`, room, epoch, document, q.Space, high)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO draft_body_reservation(operation_id,document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,bytes,state)  VALUES(?,?,?,?,?,?,?,?,&#x27;RESERVE …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 27 bytes=24716:25251 -->
````go
	if _, err = tx.ExecContext(ctx, `INSERT INTO draft_body_capacity(room_id,workspace_epoch,document_id,space_id,bytes) VALUES(?,?,?,?,?)
 ON DUPLICATE KEY UPDATE bytes=VALUES(bytes)`, room, epoch, document, q.Space, high); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO draft_body_reservation(operation_id,document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,bytes,state)
 VALUES(?,?,?,?,?,?,?,?,'RESERVED')`, operation, document, q.Space, room, epoch, body.Bytes, body.Hash, added)
	return err
}

````

## 第 28 步：SettleDraftBody

插入位置：第 27 步代码之后，紧接追加；完成稿约第 548 行。

只在上层逐项核对真实 Node COMMITTED 后调用。每笔原差额独立落账，
既不依赖确认次序，也不把过期准入或一个自报 seq 当成释放/成功证明。

本步声明：`func SettleDraftBody(ctx context.Context, tx *sql.Tx, q Quota, operation, size, hash string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,body_bytes,body_hash,bytes,state FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>；<code>writeQuota(ctx, tx, q, q.Used+amount, q.Reserved-amount)</code>；<code>tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=bytes+VALUES(bytes),operation_id=VALUES(o …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE draft_body_reservation SET state=&#x27;COMMITTED&#x27; WHERE operation_id=?`, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>writeQuota(ctx, tx, q, q.Used+amount, q.Reserved-amount)</code>；<code>tx.ExecContext</code> |
| <code>q</code> | <code>Quota</code> | 在此声明中的实际使用：<code>q.Space</code>；<code>q.Reserved</code>；<code>q.Used</code>；<code>writeQuota(ctx, tx, q, q.Used+amount, q.Reserved-amount)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,body_bytes,body_hash,bytes,state FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>；<code>tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=bytes+VALUES(bytes),operation_id=VALUES(o …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE draft_body_reservation SET state=&#x27;COMMITTED&#x27; WHERE operation_id=?`, operation)</code> |
| <code>size</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ParseDraftBodyMeasure(size, hash)</code> |
| <code>hash</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ParseDraftBodyMeasure(size, hash)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>errors.New(&quot;DRAFT_BODY_PROOF_CHANGED&quot;)</code>；<code>errors.New(&quot;DRAFT_BODY_SETTLEMENT_INCONSISTENT&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,body_bytes,body_hash,bytes,state FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&amp;document, &amp;space, &amp;bodyBytes, &amp;digest, &amp;amount, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT document_id,space_id,body_bytes,body_hash,bytes,state FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; body.Bytes != bodyBytes &#124;&#124; !bytes.Equal(body.Hash, digest) &#124;&#124; space != q.Space</code>。条件成立时执行 <code>return errors.New(&quot;DRAFT_BODY_PROOF_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>state == &quot;COMMITTED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>state != &quot;RESERVED&quot; &#124;&#124; amount &gt; q.Reserved &#124;&#124; amount &gt; ^uint64(0)-q.Used</code>。条件成立时执行 <code>return errors.New(&quot;DRAFT_BODY_SETTLEMENT_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)  ON DUPLICATE KEY UPDATE bytes=bytes+VALUES(bytes),operation_id=VALUES(o …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, `UPDATE draft_body_reservation SET state=&#x27;COMMITTED&#x27; WHERE operation_id=?`, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 28 bytes=25251:27001 -->
````go
// 只在上层逐项核对真实 Node COMMITTED 后调用。每笔原差额独立落账，
// 既不依赖确认次序，也不把过期准入或一个自报 seq 当成释放/成功证明。
func SettleDraftBody(ctx context.Context, tx *sql.Tx, q Quota, operation, size, hash string) error {
	var document, space, bodyBytes, amount uint64
	var digest []byte
	var state string
	err := tx.QueryRowContext(ctx, `SELECT document_id,space_id,body_bytes,body_hash,bytes,state FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&document, &space, &bodyBytes, &digest, &amount, &state)
	if err == sql.ErrNoRows {
		return nil
	} // 旧冻结资源保护没有正文侧账，保留原兼容路径。
	if err != nil {
		return err
	}
	body, err := domain.ParseDraftBodyMeasure(size, hash)
	if err != nil || body.Bytes != bodyBytes || !bytes.Equal(body.Hash, digest) || space != q.Space {
		return errors.New("DRAFT_BODY_PROOF_CHANGED")
	}
	if state == "COMMITTED" {
		return nil
	}
	if state != "RESERVED" || amount > q.Reserved || amount > ^uint64(0)-q.Used {
		return errors.New("DRAFT_BODY_SETTLEMENT_INCONSISTENT")
	}
	// 接受之前已检查容量；管理员后来降低上限也不能推翻实际已持久的原承诺。
	if err = writeQuota(ctx, tx, q, q.Used+amount, q.Reserved-amount); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)
 ON DUPLICATE KEY UPDATE bytes=bytes+VALUES(bytes),operation_id=VALUES(operation_id)`, document, space, amount, operation); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE draft_body_reservation SET state='COMMITTED' WHERE operation_id=?`, operation)
	return err
}

````

## 第 29 步：ReleaseCleanedDraftBodies

插入位置：第 28 步代码之后，紧接追加；完成稿约第 583 行。

只有已核验 Node CLEANED 的生命周期事务调用：未落定操作也已不可能迟到写入。
已结算占用由 document_capacity 的实际保留版本统一退款，不能再减一次 used。

本步声明：`func ReleaseCleanedDraftBodies(ctx context.Context, tx *sql.Tx, q Quota, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(bytes),0) FROM draft_body_reservation WHERE document_id=? AND space_id=? AND state=&#x27;RESERVED&#x27;`, document, q.Space)</code>；<code>writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved)</code>；<code>tx.ExecContext(ctx, `UPDATE draft_body_reservation SET state=&#x27;RELEASED&#x27; WHERE document_id=? AND state&lt;&gt;&#x27;RELEASED&#x27;`, document)</code>；<code>tx.ExecContext(ctx, `DELETE FROM draft_body_capacity WHERE document_id=?`, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved)</code>；<code>tx.ExecContext</code> |
| <code>q</code> | <code>Quota</code> | 在此声明中的实际使用：<code>q.Space</code>；<code>q.Reserved</code>；<code>writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved)</code>；<code>q.Used</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(bytes),0) FROM draft_body_reservation WHERE document_id=? AND space_id=? AND state=&#x27;RESERVED&#x27;`, document, q.Space)</code>；<code>tx.ExecContext(ctx, `UPDATE draft_body_reservation SET state=&#x27;RELEASED&#x27; WHERE document_id=? AND state&lt;&gt;&#x27;RELEASED&#x27;`, document)</code>；<code>tx.ExecContext(ctx, `DELETE FROM draft_body_capacity WHERE document_id=?`, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;DRAFT_BODY_RELEASE_INCONSISTENT&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(bytes),0) FROM draft_body_reservation WHERE document_id=? AND space_id=? AND state=&#x27;RESERVED&#x27;`, document, q.Space).Scan(&amp;reserved)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(bytes),0) FROM draft_body_reservation WHERE document_id=? AND space_id=? AND state=&#x27;RESERVED&#x27;`, document, q.Space)</code>。
- 检查 <code>reserved &gt; q.Reserved</code>。条件成立时执行 <code>return errors.New(&quot;DRAFT_BODY_RELEASE_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE draft_body_reservation SET state=&#x27;RELEASED&#x27; WHERE document_id=? AND state&lt;&gt;&#x27;RELEASED&#x27;`, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, `DELETE FROM draft_body_capacity WHERE document_id=?`, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 29 bytes=27001:28066 -->
````go
// 只有已核验 Node CLEANED 的生命周期事务调用：未落定操作也已不可能迟到写入。
// 已结算占用由 document_capacity 的实际保留版本统一退款，不能再减一次 used。
func ReleaseCleanedDraftBodies(ctx context.Context, tx *sql.Tx, q Quota, document uint64) error {
	var reserved uint64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(bytes),0) FROM draft_body_reservation WHERE document_id=? AND space_id=? AND state='RESERVED'`, document, q.Space).Scan(&reserved); err != nil {
		return err
	}
	if reserved > q.Reserved {
		return errors.New("DRAFT_BODY_RELEASE_INCONSISTENT")
	}
	if err := writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE draft_body_reservation SET state='RELEASED' WHERE document_id=? AND state<>'RELEASED'`, document); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM draft_body_capacity WHERE document_id=?`, document)
	return err
}

// quota.go：保留原实现，按业务合并文件。

````

## 第 30 步：Quota

插入位置：第 29 步代码之后，紧接追加；完成稿约第 605 行。

本步声明：`type Quota struct{ Space, Limit, Used, Reserved, Revision uint64 }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Space</code> | <code>uint64</code> | 容量账户所属空间。 |
| <code>Limit</code> | <code>uint64</code> | 空间允许的最大字节数。 |
| <code>Used</code> | <code>uint64</code> | 已记账使用的字节数。 |
| <code>Reserved</code> | <code>uint64</code> | 尚未转为实际使用的预留字节数；并发上传也要计入。 |
| <code>Revision</code> | <code>uint64</code> | 容量账户并发修订号。 |

<!-- write-step: 30 bytes=28066:28134 -->
````go
type Quota struct{ Space, Limit, Used, Reserved, Revision uint64 }

````

## 第 31 步：LockQuota

插入位置：第 30 步代码之后，紧接追加；完成稿约第 607 行。

LockQuota在权限/文档/操作锁之后调用。所有扣减均持有同一配额行，
不以事务外余额决定上传是否可接受，也不使用会回绕的无符号加法。

本步声明：`func LockQuota(ctx context.Context, tx *sql.Tx, space, initialLimit uint64) (Quota, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)  SELECT ?,?,COALESCE((SELECT SUM(bytes) FROM document_capacity WHERE space_id=?),0)   …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT space_id,limit_bytes,used_bytes,reserved_bytes,revision FROM quota_account WHERE space_id=? FOR UPDATE`, space)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>space == 0</code>；<code>tx.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)  SELECT ?,?,COALESCE((SELECT SUM(bytes) FROM document_capacity WHERE space_id=?),0)   …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT space_id,limit_bytes,used_bytes,reserved_bytes,revision FROM quota_account WHERE space_id=? FOR UPDATE`, space)</code> |
| <code>initialLimit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>initialLimit == 0</code>；<code>tx.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)  SELECT ?,?,COALESCE((SELECT SUM(bytes) FROM document_capacity WHERE space_id=?),0)   …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Quota</code> | 实际返回表达式：<code>q</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;QUOTA_CONFIG_INVALID&quot;)</code>；<code>errors.New(&quot;QUOTA_ACCOUNT_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;QUOTA_ACCOUNT_INCONSISTENT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>space == 0 &#124;&#124; initialLimit == 0</code>。条件成立时执行 <code>return q, domain.Violation(domain.InvalidRequest, &quot;QUOTA_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)  SELECT ?,?,COALESCE((SELECT SUM(bytes) FROM document_capacity WHERE space_id=?),0)   …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>!errors.As(err, &amp;duplicate) &#124;&#124; duplicate.Number != 1062</code>。条件成立时执行 <code>return q, errors.New(&quot;QUOTA_ACCOUNT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,limit_bytes,used_bytes,reserved_bytes,revision FROM quota_account WHERE space_id=? FOR UPDATE`, space).Scan(&amp;q.Space, &amp;q.Limit, &amp;q.Used, &amp;q.Reserved, &amp;q.Revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,limit_bytes,used_bytes,reserved_bytes,revision FROM quota_account WHERE space_id=? FOR UPDATE`, space)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return q, errors.New(&quot;QUOTA_ACCOUNT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>q.Revision == 0</code>。条件成立时执行 <code>return q, errors.New(&quot;QUOTA_ACCOUNT_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return q, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=28134:29722 -->
````go
// LockQuota在权限/文档/操作锁之后调用。所有扣减均持有同一配额行，
// 不以事务外余额决定上传是否可接受，也不使用会回绕的无符号加法。
func LockQuota(ctx context.Context, tx *sql.Tx, space, initialLimit uint64) (Quota, error) {
	var q Quota
	if space == 0 || initialLimit == 0 {
		return q, domain.Violation(domain.InvalidRequest, "QUOTA_CONFIG_INVALID")
	}
	// 迁移后的首次空间访问从已落定的分配计费，不能为已有历史生成零余额。
	_, err := tx.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)
 SELECT ?,?,COALESCE((SELECT SUM(bytes) FROM document_capacity WHERE space_id=?),0)
 +COALESCE((SELECT SUM(bytes) FROM resource_space_allocation WHERE space_id=? AND state='ACTIVE'),0),0`, space, initialLimit, space, space)
	if err != nil {
		var duplicate *mysql.MySQLError
		if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
			return q, errors.New("QUOTA_ACCOUNT_UNAVAILABLE")
		}
	}
	err = tx.QueryRowContext(ctx, `SELECT space_id,limit_bytes,used_bytes,reserved_bytes,revision FROM quota_account WHERE space_id=? FOR UPDATE`, space).Scan(&q.Space, &q.Limit, &q.Used, &q.Reserved, &q.Revision)
	if err != nil {
		return q, errors.New("QUOTA_ACCOUNT_UNAVAILABLE")
	}
	// 历史回填可能如实呈现超限；仍允许查询与确定的删除退款。
	// 每个新增预留/正文写入在自己的边界检查可用额度，绝不提高用户上限。
	if q.Revision == 0 {
		return q, errors.New("QUOTA_ACCOUNT_INCONSISTENT")
	}
	return q, nil
}

````

## 第 32 步：ReserveQuota

插入位置：第 31 步代码之后，紧接追加；完成稿约第 636 行。

ReserveQuota的q必须来自本事务LockQuota。原操作重试只能核对原金额，
不能延长expires_at或把已释放预留恢复为可上传。

本步声明：`func ReserveQuota(ctx context.Context, tx *sql.Tx, q Quota, operation string, size, expected uint64, expires time.Time) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at&gt;UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>；<code>tx.ExecContext(ctx, `INSERT INTO quota_reservation(operation_id,space_id,bytes,state,expires_at) VALUES(?,?,?,&#x27;RESERVED&#x27;,?)`, operation, q.Space, size, expires.UTC())</code>；<code>writeQuota(ctx, tx, q, q.Used, q.Reserved+size)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code>；<code>writeQuota(ctx, tx, q, q.Used, q.Reserved+size)</code> |
| <code>q</code> | <code>Quota</code> | 在此声明中的实际使用：<code>q.Space</code>；<code>q.Revision</code>；<code>q.Used</code>；<code>q.Limit</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at&gt;UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>；<code>tx.ExecContext(ctx, `INSERT INTO quota_reservation(operation_id,space_id,bytes,state,expires_at) VALUES(?,?,?,&#x27;RESERVED&#x27;,?)`, operation, q.Space, size, expires.UTC())</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>amount != size</code>；<code>size &gt; q.Limit-q.Used-q.Reserved</code>；<code>tx.ExecContext(ctx, `INSERT INTO quota_reservation(operation_id,space_id,bytes,state,expires_at) VALUES(?,?,?,&#x27;RESERVED&#x27;,?)`, operation, q.Space, size, expires.UTC())</code>；<code>q.Reserved+size</code> |
| <code>expected</code> | <code>uint64</code> | 在此声明中的实际使用：<code>expected != q.Revision</code> |
| <code>expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>expires.IsZero</code>；<code>expires.UTC</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;OPERATION_INPUT_CONFLICT&quot;)</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;QUOTA_RESERVATION_CLOSED&quot;)</code>；<code>errors.New(&quot;QUOTA_RESERVATION_UNAVAILABLE&quot;)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at&gt;UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&amp;space, &amp;amount, &amp;state, &amp;alive)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at&gt;UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>space != q.Space &#124;&#124; amount != size</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;OPERATION_INPUT_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>state == &quot;COMMITTED&quot; &#124;&#124; (state == &quot;RESERVED&quot; &amp;&amp; alive)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_RESERVATION_CLOSED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return errors.New(&quot;QUOTA_RESERVATION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>expected != q.Revision</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_REVISION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>q.Revision == ^uint64(0)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_REVISION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>q.Used &gt; q.Limit &#124;&#124; q.Reserved &gt; q.Limit-q.Used &#124;&#124; size &gt; q.Limit-q.Used-q.Reserved</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>expires.IsZero()</code>。条件成立时执行 <code>return domain.Violation(domain.InvalidRequest, &quot;QUOTA_EXPIRY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;QUOTA_RESERVATION_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO quota_reservation(operation_id,space_id,bytes,state,expires_at) VALUES(?,?,?,&#x27;RESERVED&#x27;,?)`, operation, q.Space, size, expires.UTC())</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return writeQuota(ctx, tx, q, q.Used, q.Reserved+size)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=29722:31433 -->
````go
// ReserveQuota的q必须来自本事务LockQuota。原操作重试只能核对原金额，
// 不能延长expires_at或把已释放预留恢复为可上传。
func ReserveQuota(ctx context.Context, tx *sql.Tx, q Quota, operation string, size, expected uint64, expires time.Time) error {
	var space, amount uint64
	var state string
	var alive bool
	err := tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at>UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&space, &amount, &state, &alive)
	if err == nil {
		if space != q.Space || amount != size {
			return domain.Violation(domain.StateConflict, "OPERATION_INPUT_CONFLICT")
		}
		if state == "COMMITTED" || (state == "RESERVED" && alive) {
			return nil
		}
		return domain.Violation(domain.StateConflict, "QUOTA_RESERVATION_CLOSED")
	}
	if err != sql.ErrNoRows {
		return errors.New("QUOTA_RESERVATION_UNAVAILABLE")
	}
	if expected != q.Revision {
		return domain.Violation(domain.StateConflict, "QUOTA_REVISION_CHANGED")
	}
	if q.Revision == ^uint64(0) {
		return domain.Violation(domain.StateConflict, "QUOTA_REVISION_EXHAUSTED")
	}
	if q.Used > q.Limit || q.Reserved > q.Limit-q.Used || size > q.Limit-q.Used-q.Reserved {
		return domain.Violation(domain.StateConflict, "QUOTA_EXCEEDED")
	}
	if expires.IsZero() {
		return domain.Violation(domain.InvalidRequest, "QUOTA_EXPIRY_REQUIRED")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO quota_reservation(operation_id,space_id,bytes,state,expires_at) VALUES(?,?,?,'RESERVED',?)`, operation, q.Space, size, expires.UTC()); err != nil {
		return errors.New("QUOTA_RESERVATION_WRITE_FAILED")
	}
	return writeQuota(ctx, tx, q, q.Used, q.Reserved+size)
}

````

## 第 33 步：SettleQuota

插入位置：第 32 步代码之后，紧接追加；完成稿约第 673 行。

SettleQuota与resource READY/DELETED转换由同一业务事务提交。
UNKNOWN不自动回收，只有调用方证明文件写入结束/删除完成后才允许结算。

本步声明：`func SettleQuota(ctx context.Context, tx *sql.Tx, q Quota, operation string, commit bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at&gt;UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>；<code>writeQuota(ctx, tx, q, used, q.Reserved-size)</code>；<code>tx.ExecContext(ctx, `UPDATE quota_reservation SET state=? WHERE operation_id=? AND state=&#x27;RESERVED&#x27;`, target, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>writeQuota(ctx, tx, q, used, q.Reserved-size)</code>；<code>tx.ExecContext</code> |
| <code>q</code> | <code>Quota</code> | 在此声明中的实际使用：<code>q.Space</code>；<code>q.Reserved</code>；<code>q.Revision</code>；<code>q.Used</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at&gt;UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>；<code>tx.ExecContext(ctx, `UPDATE quota_reservation SET state=? WHERE operation_id=? AND state=&#x27;RESERVED&#x27;`, target, operation)</code> |
| <code>commit</code> | <code>bool</code> | 在此声明中的实际使用：<code>commit &amp;&amp; !alive</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;QUOTA_RESERVATION_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;QUOTA_TARGET_CHANGED&quot;)</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;QUOTA_RESERVATION_CLOSED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at&gt;UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&amp;space, &amp;size, &amp;state, &amp;alive) != nil</code>。条件成立时执行 <code>return errors.New(&quot;QUOTA_RESERVATION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at&gt;UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&amp;space, &amp;size, &amp;state, &amp;alive)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at&gt;UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation)</code>。
- 检查 <code>commit</code>。
- 检查 <code>space != q.Space</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_TARGET_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>state == target</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>state != &quot;RESERVED&quot; &#124;&#124; (commit &amp;&amp; !alive)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_RESERVATION_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>size &gt; q.Reserved &#124;&#124; q.Revision == ^uint64(0)</code>。条件成立时执行 <code>return errors.New(&quot;QUOTA_ACCOUNT_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>commit</code>。
- 检查 <code>used &gt; q.Limit &#124;&#124; size &gt; q.Limit-used</code>。条件成立时执行 <code>return errors.New(&quot;QUOTA_ACCOUNT_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;QUOTA_RESERVATION_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE quota_reservation SET state=? WHERE operation_id=? AND state=&#x27;RESERVED&#x27;`, target, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=31433:32905 -->
````go
// SettleQuota与resource READY/DELETED转换由同一业务事务提交。
// UNKNOWN不自动回收，只有调用方证明文件写入结束/删除完成后才允许结算。
func SettleQuota(ctx context.Context, tx *sql.Tx, q Quota, operation string, commit bool) error {
	var space, size uint64
	var state string
	var alive bool
	if tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at>UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&space, &size, &state, &alive) != nil {
		return errors.New("QUOTA_RESERVATION_UNAVAILABLE")
	}
	target := "RELEASED"
	if commit {
		target = "COMMITTED"
	}
	if space != q.Space {
		return domain.Violation(domain.StateConflict, "QUOTA_TARGET_CHANGED")
	}
	if state == target {
		return nil
	}
	if state != "RESERVED" || (commit && !alive) {
		return domain.Violation(domain.StateConflict, "QUOTA_RESERVATION_CLOSED")
	}
	if size > q.Reserved || q.Revision == ^uint64(0) {
		return errors.New("QUOTA_ACCOUNT_INCONSISTENT")
	}
	used := q.Used
	if commit {
		if used > q.Limit || size > q.Limit-used {
			return errors.New("QUOTA_ACCOUNT_INCONSISTENT")
		}
		used += size
	}
	if err := writeQuota(ctx, tx, q, used, q.Reserved-size); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE quota_reservation SET state=? WHERE operation_id=? AND state='RESERVED'`, target, operation); err != nil {
		return errors.New("QUOTA_RESERVATION_WRITE_FAILED")
	}
	return nil
}

````

## 第 34 步：writeQuota

插入位置：第 33 步代码之后，紧接追加；完成稿约第 714 行。

本步声明：`func writeQuota(ctx context.Context, tx *sql.Tx, q Quota, used, reserved uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE quota_account SET used_bytes=?,reserved_bytes=?,revision=revision+1 WHERE space_id=? AND revision=? AND revision&lt;18446744073709551615`, used, reserved, q.Space, q.Revision)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>q</code> | <code>Quota</code> | 在此声明中的实际使用：<code>q.Space</code>；<code>q.Revision</code> |
| <code>used</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE quota_account SET used_bytes=?,reserved_bytes=?,revision=revision+1 WHERE space_id=? AND revision=? AND revision&lt;18446744073709551615`, used, reserved, q.Space, q.Revision)</code> |
| <code>reserved</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE quota_account SET used_bytes=?,reserved_bytes=?,revision=revision+1 WHERE space_id=? AND revision=? AND revision&lt;18446744073709551615`, used, reserved, q.Space, q.Revision)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;QUOTA_ACCOUNT_WRITE_FAILED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;QUOTA_REVISION_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE quota_account SET used_bytes=?,reserved_bytes=?,revision=revision+1 WHERE space_id=? AND revision=? AND revision&lt;18446744073709551615`, used, reserved, q.Space, q.Revision)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;QUOTA_ACCOUNT_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_REVISION_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=32905:33506 -->
````go
func writeQuota(ctx context.Context, tx *sql.Tx, q Quota, used, reserved uint64) error {
	result, err := tx.ExecContext(ctx, `UPDATE quota_account SET used_bytes=?,reserved_bytes=?,revision=revision+1 WHERE space_id=? AND revision=? AND revision<18446744073709551615`, used, reserved, q.Space, q.Revision)
	if err != nil {
		return errors.New("QUOTA_ACCOUNT_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "QUOTA_REVISION_CHANGED")
	}
	return nil
}

// quota_initialization.go：保留原实现，按业务合并文件。

````

## 第 35 步：InitializeQuotaAccounts

插入位置：第 34 步代码之后，紧接追加；完成稿约第 727 行。

启动时为迁移前尚未上传过图片的空间物化真实上限。已存在的用户上限和费用
完全沿用；历史 backfill 由不可变迁移完成，不能在每次启动重复追加。

本步声明：`func InitializeQuotaAccounts(ctx context.Context, db *sql.DB, limit uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)  SELECT s.id,?,COALESCE(d.bytes,0)+COALESCE(r.bytes,0),0 FROM space s  LEFT JOIN (SEL …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.ExecContext</code> |
| <code>limit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>limit == 0</code>；<code>db.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)  SELECT s.id,?,COALESCE(d.bytes,0)+COALESCE(r.bytes,0),0 FROM space s  LEFT JOIN (SEL …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;QUOTA_CONFIG_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>limit == 0</code>。条件成立时执行 <code>return errors.New(&quot;QUOTA_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>db.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)  SELECT s.id,?,COALESCE(d.bytes,0)+COALESCE(r.bytes,0),0 FROM space s  LEFT JOIN (SEL …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=33506:34494 -->
````go
// 启动时为迁移前尚未上传过图片的空间物化真实上限。已存在的用户上限和费用
// 完全沿用；历史 backfill 由不可变迁移完成，不能在每次启动重复追加。
func InitializeQuotaAccounts(ctx context.Context, db *sql.DB, limit uint64) error {
	if limit == 0 {
		return errors.New("QUOTA_CONFIG_INVALID")
	}
	_, err := db.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)
 SELECT s.id,?,COALESCE(d.bytes,0)+COALESCE(r.bytes,0),0 FROM space s
 LEFT JOIN (SELECT space_id,SUM(bytes) bytes FROM document_capacity GROUP BY space_id) d ON d.space_id=s.id
 LEFT JOIN (SELECT space_id,SUM(bytes) bytes FROM resource_space_allocation WHERE state='ACTIVE' GROUP BY space_id) r ON r.space_id=s.id
 WHERE NOT EXISTS(SELECT 1 FROM quota_account q WHERE q.space_id=s.id)
 ON DUPLICATE KEY UPDATE space_id=VALUES(space_id)`, limit)
	return err
}

// ready_cleanup.go：保留原实现，按业务合并文件。

````

## 第 36 步：ReadyCleanupCandidates

插入位置：第 35 步代码之后，紧接追加；完成稿约第 744 行。

ReadyCleanupCandidates 只发现候选，不能授权物理删除。原上传期限后额外保留
一小时插入正文的宽限，保证临近上传期限完成的对象仍有时间建立草稿引用。
该宽限仅用于 GC 判断，不修改原上传 expires 收据，也不为上传续期。
保留所有历史、草稿及未释放保护；原 UPLOAD 引用仅为恢复凭据，不阻止 GC。

本步声明：`func ReadyCleanupCandidates(ctx context.Context, db *sql.DB, limit int) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT r.id FROM resource r 		JOIN quota_reservation q ON q.operation_id=r.upload_operation_id 		WHERE q.state=&#x27;COMMITTED&#x27; AND q.space_id=r.space_id AND q.byt …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>limit &lt; 1</code>；<code>limit = 1</code>；<code>limit &gt; 32</code>；<code>limit = 32</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RESOURCE_READY_CLEANUP_READ_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>limit &lt; 1</code>。
- 检查 <code>limit &gt; 32</code>。
- 执行 <code>db.QueryContext(ctx, `SELECT r.id FROM resource r 		JOIN quota_reservation q ON q.operation_id=r.upload_operation_id 		WHERE q.state=&#x27;COMMITTED&#x27; AND q.space_id=r.space_id AND q.byt …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;RESOURCE_READY_CLEANUP_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;RESOURCE_READY_CLEANUP_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;RESOURCE_READY_CLEANUP_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return ids, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=34494:36158 -->
````go
// ReadyCleanupCandidates 只发现候选，不能授权物理删除。原上传期限后额外保留
// 一小时插入正文的宽限，保证临近上传期限完成的对象仍有时间建立草稿引用。
// 该宽限仅用于 GC 判断，不修改原上传 expires 收据，也不为上传续期。
// 保留所有历史、草稿及未释放保护；原 UPLOAD 引用仅为恢复凭据，不阻止 GC。
func ReadyCleanupCandidates(ctx context.Context, db *sql.DB, limit int) ([]uint64, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 32 {
		limit = 32
	}
	rows, err := db.QueryContext(ctx, `SELECT r.id FROM resource r
		JOIN quota_reservation q ON q.operation_id=r.upload_operation_id
		WHERE q.state='COMMITTED' AND q.space_id=r.space_id AND q.bytes=r.byte_size
		AND ((r.state='READY' AND DATE_ADD(q.expires_at,INTERVAL 1 HOUR)<=UTC_TIMESTAMP(6)) OR r.state='DELETING')
		AND NOT EXISTS (SELECT 1 FROM resource_reference x WHERE x.resource_id=r.id
			AND NOT (x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'))
		AND NOT EXISTS (SELECT 1 FROM resource_protection p WHERE p.state<>'RELEASED'
			AND JSON_CONTAINS(p.resource_ids,CAST(CAST(r.id AS CHAR) AS JSON)))
		ORDER BY r.id LIMIT ?`, limit)
	if err != nil {
		return nil, errors.New("RESOURCE_READY_CLEANUP_READ_FAILED")
	}
	defer rows.Close()
	ids := make([]uint64, 0, limit)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, errors.New("RESOURCE_READY_CLEANUP_READ_FAILED")
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		return nil, errors.New("RESOURCE_READY_CLEANUP_READ_FAILED")
	}
	return ids, nil
}

````

## 第 37 步：MarkReadyDeleting

插入位置：第 36 步代码之后，紧接追加；完成稿约第 782 行。

MarkReadyDeleting 由业务层先持文件排他锁，再依次锁配额、资源后调用。
候选发现与加锁之间可能新增引用，因此转换时重查全部约束；竞争返回 false，
只有成功进入 DELETING 后业务层才可删除文件，后续恢复沿用原上传操作身份。

本步声明：`func MarkReadyDeleting(ctx context.Context, tx *sql.Tx, u Upload) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE resource r SET r.state=&#x27;DELETING&#x27;,r.revision=r.revision+1 		WHERE r.id=? AND r.state=&#x27;READY&#x27; AND r.revision=? AND r.revision&lt;18446744073709551615 		AND  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>u</code> | <code>Upload</code> | 在此声明中的实际使用：<code>u.State</code>；<code>u.Revision</code>；<code>u.ID</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>n == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;RESOURCE_REVISION_EXHAUSTED&quot;)</code>；<code>errors.New(&quot;RESOURCE_READY_DELETING_WRITE_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>u.State != &quot;READY&quot;</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>u.Revision == ^uint64(0)</code>。条件成立时执行 <code>return false, errors.New(&quot;RESOURCE_REVISION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE resource r SET r.state=&#x27;DELETING&#x27;,r.revision=r.revision+1 		WHERE r.id=? AND r.state=&#x27;READY&#x27; AND r.revision=? AND r.revision&lt;18446744073709551615 		AND  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, errors.New(&quot;RESOURCE_READY_DELETING_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n &gt; 1</code>。条件成立时执行 <code>return false, errors.New(&quot;RESOURCE_READY_DELETING_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return n == 1, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=36158:37717 -->
````go
// MarkReadyDeleting 由业务层先持文件排他锁，再依次锁配额、资源后调用。
// 候选发现与加锁之间可能新增引用，因此转换时重查全部约束；竞争返回 false，
// 只有成功进入 DELETING 后业务层才可删除文件，后续恢复沿用原上传操作身份。
func MarkReadyDeleting(ctx context.Context, tx *sql.Tx, u Upload) (bool, error) {
	if u.State != "READY" {
		return false, nil
	}
	if u.Revision == ^uint64(0) {
		return false, errors.New("RESOURCE_REVISION_EXHAUSTED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE resource r SET r.state='DELETING',r.revision=r.revision+1
		WHERE r.id=? AND r.state='READY' AND r.revision=? AND r.revision<18446744073709551615
		AND EXISTS (SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id
			AND q.state='COMMITTED' AND q.space_id=r.space_id AND q.bytes=r.byte_size
			AND DATE_ADD(q.expires_at,INTERVAL 1 HOUR)<=UTC_TIMESTAMP(6))
		AND NOT EXISTS (SELECT 1 FROM resource_reference x WHERE x.resource_id=r.id
			AND NOT (x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'))
		AND NOT EXISTS (SELECT 1 FROM resource_protection p WHERE p.state<>'RELEASED'
			AND JSON_CONTAINS(p.resource_ids,CAST(? AS JSON)))`, u.ID, u.Revision, strconv.FormatUint(u.ID, 10))
	if err != nil {
		return false, errors.New("RESOURCE_READY_DELETING_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil || n > 1 {
		return false, errors.New("RESOURCE_READY_DELETING_WRITE_FAILED")
	}
	return n == 1, nil
}

````

## 第 38 步：ReadyCleanupCommitted

插入位置：第 37 步代码之后，紧接追加；完成稿约第 811 行。

ReadyCleanupCommitted 在删除文件前区分两类 DELETING 恢复路径，不能只靠资源
状态判断它来自 READY GC。调用方仍须先按配额、资源顺序加锁。

本步声明：`func ReadyCleanupCommitted(ctx context.Context, tx *sql.Tx, u Upload) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state FROM quota_reservation WHERE operation_id=? FOR UPDATE`, u.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>u</code> | <code>Upload</code> | 在此声明中的实际使用：<code>u.Operation</code>；<code>u.Space</code>；<code>u.Size</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>state == &quot;COMMITTED&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;QUOTA_RESERVATION_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;RESOURCE_USED_QUOTA_INCONSISTENT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state FROM quota_reservation WHERE operation_id=? FOR UPDATE`, u.Operation).Scan(&amp;space, &amp;size, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,bytes,state FROM quota_reservation WHERE operation_id=? FOR UPDATE`, u.Operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, errors.New(&quot;QUOTA_RESERVATION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>space != u.Space &#124;&#124; size != u.Size</code>。条件成立时执行 <code>return false, errors.New(&quot;RESOURCE_USED_QUOTA_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return state == &quot;COMMITTED&quot;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=37717:38415 -->
````go
// ReadyCleanupCommitted 在删除文件前区分两类 DELETING 恢复路径，不能只靠资源
// 状态判断它来自 READY GC。调用方仍须先按配额、资源顺序加锁。
func ReadyCleanupCommitted(ctx context.Context, tx *sql.Tx, u Upload) (bool, error) {
	var space, size uint64
	var state string
	err := tx.QueryRowContext(ctx, `SELECT space_id,bytes,state FROM quota_reservation WHERE operation_id=? FOR UPDATE`, u.Operation).Scan(&space, &size, &state)
	if err != nil {
		return false, errors.New("QUOTA_RESERVATION_UNAVAILABLE")
	}
	if space != u.Space || size != u.Size {
		return false, errors.New("RESOURCE_USED_QUOTA_INCONSISTENT")
	}
	return state == "COMMITTED", nil
}

````

## 第 39 步：ReleaseUsedQuota

插入位置：第 38 步代码之后，紧接追加；完成稿约第 826 行。

ReleaseUsedQuota 必须紧随同事务成功的 MarkDeleted；调用方传入转换前加锁的 u。
MarkDeleted 的 CAS 保证每个资源只结算一次，不能单独重试本函数。COMMITTED
收据保留原样供上传幂等核对，READY GC 只减少 used，不再次释放暂存/预留预算。

本步声明：`func ReleaseUsedQuota(ctx context.Context, tx *sql.Tx, q Quota, u Upload) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ReadyCleanupCommitted(ctx, tx, u)</code>；<code>writeQuota(ctx, tx, q, q.Used-u.Size, q.Reserved)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ReadyCleanupCommitted(ctx, tx, u)</code>；<code>writeQuota(ctx, tx, q, q.Used-u.Size, q.Reserved)</code> |
| <code>q</code> | <code>Quota</code> | 在此声明中的实际使用：<code>q.Space</code>；<code>q.Used</code>；<code>q.Revision</code>；<code>writeQuota(ctx, tx, q, q.Used-u.Size, q.Reserved)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>u</code> | <code>Upload</code> | 在此声明中的实际使用：<code>u.State</code>；<code>u.Space</code>；<code>u.Size</code>；<code>ReadyCleanupCommitted(ctx, tx, u)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RESOURCE_USED_QUOTA_INCONSISTENT&quot;)</code>；<code>err</code>；<code>writeQuota(ctx, tx, q, q.Used-u.Size, q.Reserved)</code> |

**跟着本段代码走一遍：**

- 检查 <code>u.State != &quot;DELETING&quot; &#124;&#124; u.Space != q.Space &#124;&#124; u.Size &gt; q.Used &#124;&#124; q.Revision == ^uint64(0)</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_USED_QUOTA_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!committed</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_USED_QUOTA_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return writeQuota(ctx, tx, q, q.Used-u.Size, q.Reserved)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=38415:39257 -->
````go
// ReleaseUsedQuota 必须紧随同事务成功的 MarkDeleted；调用方传入转换前加锁的 u。
// MarkDeleted 的 CAS 保证每个资源只结算一次，不能单独重试本函数。COMMITTED
// 收据保留原样供上传幂等核对，READY GC 只减少 used，不再次释放暂存/预留预算。
func ReleaseUsedQuota(ctx context.Context, tx *sql.Tx, q Quota, u Upload) error {
	if u.State != "DELETING" || u.Space != q.Space || u.Size > q.Used || q.Revision == ^uint64(0) {
		return errors.New("RESOURCE_USED_QUOTA_INCONSISTENT")
	}
	committed, err := ReadyCleanupCommitted(ctx, tx, u)
	if err != nil {
		return err
	}
	if !committed {
		return errors.New("RESOURCE_USED_QUOTA_INCONSISTENT")
	}
	return writeQuota(ctx, tx, q, q.Used-u.Size, q.Reserved)
}

// retained_cleanup.go：保留原实现，按业务合并文件。

````

## 第 40 步：MarkRetainedReadyDeleting

插入位置：第 39 步代码之后，紧接追加；完成稿约第 845 行。

只供已取得Node墓碑并清除正文的孤立留存治理使用。与普通GC相同，最终
许可在资源锁下重查引用/保护；区别仅是此明确删除不等上传一小时宽限。

本步声明：`func MarkRetainedReadyDeleting(ctx context.Context, tx *sql.Tx, u Upload) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE resource r SET r.state=&#x27;DELETING&#x27;,r.revision=r.revision+1 	 WHERE r.id=? AND r.state=&#x27;READY&#x27; AND r.revision=? AND r.revision&lt;18446744073709551615 	 AND  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>u</code> | <code>Upload</code> | 在此声明中的实际使用：<code>u.State</code>；<code>u.Revision</code>；<code>u.ID</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>n == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RETAINED_RESOURCE_STATE_CHANGED&quot;)</code>；<code>err</code>；<code>errors.New(&quot;RETAINED_RESOURCE_DELETING_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>u.State != &quot;READY&quot; &#124;&#124; u.Revision == ^uint64(0)</code>。条件成立时执行 <code>return false, errors.New(&quot;RETAINED_RESOURCE_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE resource r SET r.state=&#x27;DELETING&#x27;,r.revision=r.revision+1 	 WHERE r.id=? AND r.state=&#x27;READY&#x27; AND r.revision=? AND r.revision&lt;18446744073709551615 	 AND  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n &gt; 1</code>。条件成立时执行 <code>return false, errors.New(&quot;RETAINED_RESOURCE_DELETING_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return n == 1, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 40 bytes=39257:40571 -->
````go
// 只供已取得Node墓碑并清除正文的孤立留存治理使用。与普通GC相同，最终
// 许可在资源锁下重查引用/保护；区别仅是此明确删除不等上传一小时宽限。
func MarkRetainedReadyDeleting(ctx context.Context, tx *sql.Tx, u Upload) (bool, error) {
	if u.State != "READY" || u.Revision == ^uint64(0) {
		return false, errors.New("RETAINED_RESOURCE_STATE_CHANGED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE resource r SET r.state='DELETING',r.revision=r.revision+1
	 WHERE r.id=? AND r.state='READY' AND r.revision=? AND r.revision<18446744073709551615
	 AND EXISTS(SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id AND q.state='COMMITTED' AND q.space_id=r.space_id AND q.bytes=r.byte_size)
	 AND NOT EXISTS(SELECT 1 FROM resource_reference x WHERE x.resource_id=r.id
	  AND NOT(x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'))
	 AND NOT EXISTS(SELECT 1 FROM resource_protection p WHERE p.state<>'RELEASED' AND JSON_CONTAINS(p.resource_ids,CAST(? AS JSON)))`,
		u.ID, u.Revision, strconv.FormatUint(u.ID, 10))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n > 1 {
		return false, errors.New("RETAINED_RESOURCE_DELETING_FAILED")
	}
	return n == 1, nil
}

````

## 第 41 步：RetainedResourceProtection

插入位置：第 40 步代码之后，紧接追加；完成稿约第 868 行。

本步声明：`func RetainedResourceProtection(ctx context.Context, tx *sql.Tx, u Upload, document uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT 	 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND owner_ref=? AND purpose=&#x27;UPLOAD&#x27;) …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>u</code> | <code>Upload</code> | 在此声明中的实际使用：<code>u.ID</code>；<code>u.Operation</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT 	 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND owner_ref=? AND purpose=&#x27;UPLOAD&#x27;) …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>shared</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;RETAINED_RESOURCE_PROTECTION_PENDING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT 	 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND owner_ref=? AND purpose=&#x27;UPLOAD&#x27;) …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT 	 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND owner_ref=? AND purpose=&#x27;UPLOAD&#x27;) …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>own &#124;&#124; unknown</code>。条件成立时执行 <code>return false, errors.New(&quot;RETAINED_RESOURCE_PROTECTION_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return shared, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=40571:41638 -->
````go
func RetainedResourceProtection(ctx context.Context, tx *sql.Tx, u Upload, document uint64) (bool, error) {
	var own, shared, unknown bool
	err := tx.QueryRowContext(ctx, `SELECT
	 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind='OPERATION' AND owner_ref=? AND purpose='UPLOAD')),
	 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id<>? AND NOT(owner_kind='OPERATION' AND owner_ref=? AND purpose='UPLOAD'))
	 OR EXISTS(SELECT 1 FROM resource_protection WHERE document_id<>? AND state IN ('PROTECTED','CONFIRMED') AND JSON_CONTAINS(resource_ids,CAST(? AS JSON))),
	 EXISTS(SELECT 1 FROM resource_protection WHERE state='UNKNOWN' AND JSON_CONTAINS(resource_ids,CAST(? AS JSON)))`,
		u.ID, document, u.Operation, u.ID, document, u.Operation, document, strconv.FormatUint(u.ID, 10), strconv.FormatUint(u.ID, 10)).Scan(&own, &shared, &unknown)
	if err != nil {
		return false, err
	}
	if own || unknown {
		return false, errors.New("RETAINED_RESOURCE_PROTECTION_PENDING")
	}
	return shared, nil
}

````

## 第 42 步：MarkRetainedUploadDeleting

插入位置：第 41 步代码之后，紧接追加；完成稿约第 885 行。

孤立文档已永久停止输入且持文件排它锁，未完成上传也可立即回收；
RESERVED才可转换，UNKNOWN预留不能在删除IO后才发现无法结算。

本步声明：`func MarkRetainedUploadDeleting(ctx context.Context, tx *sql.Tx, u Upload) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE resource r SET r.state=&#x27;DELETING&#x27;,r.revision=r.revision+1 	 WHERE r.id=? AND r.state=&#x27;UPLOADING&#x27; AND r.revision=? AND r.revision&lt;18446744073709551615 	  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>u</code> | <code>Upload</code> | 在此声明中的实际使用：<code>u.ID</code>；<code>u.Revision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;RETAINED_UPLOAD_DELETING_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE resource r SET r.state=&#x27;DELETING&#x27;,r.revision=r.revision+1 	 WHERE r.id=? AND r.state=&#x27;UPLOADING&#x27; AND r.revision=? AND r.revision&lt;18446744073709551615 	  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_UPLOAD_DELETING_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 42 bytes=41638:42846 -->
````go
// 孤立文档已永久停止输入且持文件排它锁，未完成上传也可立即回收；
// RESERVED才可转换，UNKNOWN预留不能在删除IO后才发现无法结算。
func MarkRetainedUploadDeleting(ctx context.Context, tx *sql.Tx, u Upload) error {
	result, err := tx.ExecContext(ctx, `UPDATE resource r SET r.state='DELETING',r.revision=r.revision+1
	 WHERE r.id=? AND r.state='UPLOADING' AND r.revision=? AND r.revision<18446744073709551615
	 AND EXISTS(SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id AND q.state='RESERVED' AND q.space_id=r.space_id AND q.bytes=r.byte_size)
	 AND NOT EXISTS(SELECT 1 FROM resource_reference x WHERE x.resource_id=r.id
	  AND NOT(x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'))
	 AND NOT EXISTS(SELECT 1 FROM resource_protection p WHERE p.state<>'RELEASED' AND JSON_CONTAINS(p.resource_ids,CAST(? AS JSON)))`,
		u.ID, u.Revision, strconv.FormatUint(u.ID, 10))
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("RETAINED_UPLOAD_DELETING_FAILED")
	}
	return nil
}

// temporary.go：保留原实现，按业务合并文件。

````

## 第 43 步：temporaryBudget

插入位置：第 42 步代码之后，紧接追加；完成稿约第 906 行。

本步声明：`type temporaryBudget struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>limit</code> | <code>uint64</code> | 全局上传临时容量上限（字节）。 |
| <code>reserved</code> | <code>uint64</code> | 当前上传临时预留量。 |
| <code>revision</code> | <code>uint64</code> | 临时容量账户修订号，随预留/释放推进。 |

<!-- write-step: 43 bytes=42846:42913 -->
````go
type temporaryBudget struct {
	limit, reserved, revision uint64
}

````

## 第 44 步：ReserveTemporary

插入位置：第 43 步代码之后，紧接追加；完成稿约第 910 行。

ReserveTemporary 在空间配额与资源锁之后占用全局上传暂存预算。
仅新建 UPLOADING 的事务调用一次；操作重试沿用资源状态，不能再次占用。
limit 只用于首次初始化，后续配置变更不能隐式覆盖已持久化的预算。

本步声明：`func ReserveTemporary(ctx context.Context, tx *sql.Tx, limit, size uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO temporary_budget(kind,limit_bytes,reserved_bytes) VALUES(&#x27;RESOURCE_UPLOAD&#x27;,?,0)`, limit)</code>；<code>lockTemporary(ctx, tx)</code>；<code>writeTemporary(ctx, tx, b, b.reserved+size)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>lockTemporary(ctx, tx)</code>；<code>writeTemporary(ctx, tx, b, b.reserved+size)</code> |
| <code>limit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>limit == 0</code>；<code>tx.ExecContext(ctx, `INSERT INTO temporary_budget(kind,limit_bytes,reserved_bytes) VALUES(&#x27;RESOURCE_UPLOAD&#x27;,?,0)`, limit)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>size &gt; b.limit-b.reserved</code>；<code>b.reserved+size</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;TEMPORARY_BUDGET_CONFIG_INVALID&quot;)</code>；<code>errors.New(&quot;TEMPORARY_BUDGET_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TEMPORARY_BUDGET_EXCEEDED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>limit == 0</code>。条件成立时执行 <code>return domain.Violation(domain.InvalidRequest, &quot;TEMPORARY_BUDGET_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO temporary_budget(kind,limit_bytes,reserved_bytes) VALUES(&#x27;RESOURCE_UPLOAD&#x27;,?,0)`, limit)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>!errors.As(err, &amp;duplicate) &#124;&#124; duplicate.Number != 1062</code>。条件成立时执行 <code>return errors.New(&quot;TEMPORARY_BUDGET_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>size &gt; b.limit-b.reserved</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;TEMPORARY_BUDGET_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return writeTemporary(ctx, tx, b, b.reserved+size)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 44 bytes=42913:43927 -->
````go
// ReserveTemporary 在空间配额与资源锁之后占用全局上传暂存预算。
// 仅新建 UPLOADING 的事务调用一次；操作重试沿用资源状态，不能再次占用。
// limit 只用于首次初始化，后续配置变更不能隐式覆盖已持久化的预算。
func ReserveTemporary(ctx context.Context, tx *sql.Tx, limit, size uint64) error {
	if limit == 0 {
		return domain.Violation(domain.InvalidRequest, "TEMPORARY_BUDGET_CONFIG_INVALID")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO temporary_budget(kind,limit_bytes,reserved_bytes) VALUES('RESOURCE_UPLOAD',?,0)`, limit)
	if err != nil {
		var duplicate *mysql.MySQLError
		if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
			return errors.New("TEMPORARY_BUDGET_UNAVAILABLE")
		}
	}
	b, err := lockTemporary(ctx, tx)
	if err != nil {
		return err
	}
	if size > b.limit-b.reserved {
		return domain.Violation(domain.StateConflict, "TEMPORARY_BUDGET_EXCEEDED")
	}
	return writeTemporary(ctx, tx, b, b.reserved+size)
}

````

## 第 45 步：ReleaseTemporary

插入位置：第 44 步代码之后，紧接追加；完成稿约第 934 行。

ReleaseTemporary 与 READY 或 DELETED 状态转换处于同一事务。
调用方先锁定资源并确认尚未完成转换，因此重试不会重复释放其他上传的预算。
删除路径必须先完成文件清理，再释放预算，避免磁盘空间尚未回收就接纳新上传。

本步声明：`func ReleaseTemporary(ctx context.Context, tx *sql.Tx, size uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>lockTemporary(ctx, tx)</code>；<code>writeTemporary(ctx, tx, b, b.reserved-size)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>lockTemporary(ctx, tx)</code>；<code>writeTemporary(ctx, tx, b, b.reserved-size)</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>size &gt; b.reserved</code>；<code>b.reserved-size</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;TEMPORARY_BUDGET_INCONSISTENT&quot;)</code>；<code>writeTemporary(ctx, tx, b, b.reserved-size)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>size &gt; b.reserved</code>。条件成立时执行 <code>return errors.New(&quot;TEMPORARY_BUDGET_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return writeTemporary(ctx, tx, b, b.reserved-size)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 45 bytes=43927:44508 -->
````go
// ReleaseTemporary 与 READY 或 DELETED 状态转换处于同一事务。
// 调用方先锁定资源并确认尚未完成转换，因此重试不会重复释放其他上传的预算。
// 删除路径必须先完成文件清理，再释放预算，避免磁盘空间尚未回收就接纳新上传。
func ReleaseTemporary(ctx context.Context, tx *sql.Tx, size uint64) error {
	b, err := lockTemporary(ctx, tx)
	if err != nil {
		return err
	}
	if size > b.reserved {
		return errors.New("TEMPORARY_BUDGET_INCONSISTENT")
	}
	return writeTemporary(ctx, tx, b, b.reserved-size)
}

````

## 第 46 步：lockTemporary

插入位置：第 45 步代码之后，紧接追加；完成稿约第 948 行。

本步声明：`func lockTemporary(ctx context.Context, tx *sql.Tx) (temporaryBudget, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT limit_bytes,reserved_bytes,revision FROM temporary_budget WHERE kind=&#x27;RESOURCE_UPLOAD&#x27; FOR UPDATE`)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>temporaryBudget</code> | 实际返回表达式：<code>b</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;TEMPORARY_BUDGET_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;TEMPORARY_BUDGET_INCONSISTENT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return b, errors.New(&quot;TEMPORARY_BUDGET_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT limit_bytes,reserved_bytes,revision FROM temporary_budget WHERE kind=&#x27;RESOURCE_UPLOAD&#x27; FOR UPDATE`).Scan(&amp;b.limit, &amp;b.reserved, &amp;b.revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT limit_bytes,reserved_bytes,revision FROM temporary_budget WHERE kind=&#x27;RESOURCE_UPLOAD&#x27; FOR UPDATE`)</code>。
- 检查 <code>b.limit == 0 &#124;&#124; b.revision == 0 &#124;&#124; b.reserved &gt; b.limit</code>。条件成立时执行 <code>return b, errors.New(&quot;TEMPORARY_BUDGET_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return b, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 46 bytes=44508:45005 -->
````go
func lockTemporary(ctx context.Context, tx *sql.Tx) (temporaryBudget, error) {
	var b temporaryBudget
	if err := tx.QueryRowContext(ctx, `SELECT limit_bytes,reserved_bytes,revision FROM temporary_budget WHERE kind='RESOURCE_UPLOAD' FOR UPDATE`).Scan(&b.limit, &b.reserved, &b.revision); err != nil {
		return b, errors.New("TEMPORARY_BUDGET_UNAVAILABLE")
	}
	if b.limit == 0 || b.revision == 0 || b.reserved > b.limit {
		return b, errors.New("TEMPORARY_BUDGET_INCONSISTENT")
	}
	return b, nil
}

````

## 第 47 步：writeTemporary

插入位置：第 46 步代码之后，紧接追加；完成稿约第 959 行。

本步声明：`func writeTemporary(ctx context.Context, tx *sql.Tx, b temporaryBudget, reserved uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE temporary_budget SET reserved_bytes=?,revision=revision+1 WHERE kind=&#x27;RESOURCE_UPLOAD&#x27; AND revision=? AND revision&lt;18446744073709551615`, reserved, b.revision)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>b</code> | <code>temporaryBudget</code> | 在此声明中的实际使用：<code>b.revision</code> |
| <code>reserved</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE temporary_budget SET reserved_bytes=?,revision=revision+1 WHERE kind=&#x27;RESOURCE_UPLOAD&#x27; AND revision=? AND revision&lt;18446744073709551615`, reserved, b.revision)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;TEMPORARY_BUDGET_REVISION_EXHAUSTED&quot;)</code>；<code>errors.New(&quot;TEMPORARY_BUDGET_WRITE_FAILED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;TEMPORARY_BUDGET_REVISION_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>b.revision == ^uint64(0)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;TEMPORARY_BUDGET_REVISION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE temporary_budget SET reserved_bytes=?,revision=revision+1 WHERE kind=&#x27;RESOURCE_UPLOAD&#x27; AND revision=? AND revision&lt;18446744073709551615`, reserved, b.revision)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;TEMPORARY_BUDGET_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;TEMPORARY_BUDGET_REVISION_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 47 bytes=45005:45735 -->
````go
func writeTemporary(ctx context.Context, tx *sql.Tx, b temporaryBudget, reserved uint64) error {
	if b.revision == ^uint64(0) {
		return domain.Violation(domain.StateConflict, "TEMPORARY_BUDGET_REVISION_EXHAUSTED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE temporary_budget SET reserved_bytes=?,revision=revision+1 WHERE kind='RESOURCE_UPLOAD' AND revision=? AND revision<18446744073709551615`, reserved, b.revision)
	if err != nil {
		return errors.New("TEMPORARY_BUDGET_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "TEMPORARY_BUDGET_REVISION_CHANGED")
	}
	return nil
}

// transfer_allocation.go：保留原实现，按业务合并文件。

````

## 第 48 步：ResourceAllowedForDocument

插入位置：第 47 步代码之后，紧接追加；完成稿约第 975 行。

精确资格绑定已接受的 Doc、目标空间与原转让，不允许只凭资源编号跨空间复用。

本步声明：`func ResourceAllowedForDocument(ctx context.Context, tx *sql.Tx, resource, document, space uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource r WHERE r.id=? AND  ((r.space_id=? AND (EXISTS(SELECT 1 FROM resource_space_allocation z WHERE z.resource_id=r.id AND z.space …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource r WHERE r.id=? AND  ((r.space_id=? AND (EXISTS(SELECT 1 FROM resource_space_allocation z WHERE z.resource_id=r.id AND z.space …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource r WHERE r.id=? AND  ((r.space_id=? AND (EXISTS(SELECT 1 FROM resource_space_allocation z WHERE z.resource_id=r.id AND z.space …</code>（长表达式，按本步完整代码继续） |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource r WHERE r.id=? AND  ((r.space_id=? AND (EXISTS(SELECT 1 FROM resource_space_allocation z WHERE z.resource_id=r.id AND z.space …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>n == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource r WHERE r.id=? AND  ((r.space_id=? AND (EXISTS(SELECT 1 FROM resource_space_allocation z WHERE z.resource_id=r.id AND z.space …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource r WHERE r.id=? AND  ((r.space_id=? AND (EXISTS(SELECT 1 FROM resource_space_allocation z WHERE z.resource_id=r.id AND z.space …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return n == 1, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 48 bytes=45735:46902 -->
````go
// 精确资格绑定已接受的 Doc、目标空间与原转让，不允许只凭资源编号跨空间复用。
func ResourceAllowedForDocument(ctx context.Context, tx *sql.Tx, resource, document, space uint64) (bool, error) {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource r WHERE r.id=? AND
 ((r.space_id=? AND (EXISTS(SELECT 1 FROM resource_space_allocation z WHERE z.resource_id=r.id AND z.space_id=r.space_id AND z.state='ACTIVE')
 OR (NOT EXISTS(SELECT 1 FROM resource_space_allocation z WHERE z.resource_id=r.id AND z.space_id=r.space_id)
 AND EXISTS(SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id AND q.state='COMMITTED' AND q.space_id=r.space_id AND q.bytes=r.byte_size))))
 OR EXISTS(SELECT 1 FROM document_resource_allocation a
 JOIN personal_transfer t ON t.id=a.transfer_id AND t.state='ACCEPTED'
 JOIN resource_space_allocation s ON s.resource_id=a.resource_id AND s.space_id=a.space_id AND s.state='ACTIVE'
 WHERE a.resource_id=r.id AND a.document_id=? AND a.space_id=? AND a.state='ACTIVE'
 AND t.document_id=a.document_id))`, resource, space, document, space).Scan(&n)
	return n == 1, err
}

````

## 第 49 步：LockTransferQuotas

插入位置：第 48 步代码之后，紧接追加；完成稿约第 990 行。

所有转让按照空间编号持锁，然后才锁资源。计划预留无 TTL：UNKNOWN 不归还预算。

本步声明：`func LockTransferQuotas(ctx context.Context, tx *sql.Tx, source, target, limit uint64) (Quota, Quota, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockQuota(ctx, tx, id, limit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockQuota(ctx, tx, id, limit)</code> |
| <code>source</code> | <code>uint64</code> | 在此声明中的实际使用：<code>ids := &#91;&#93;uint64{source, target}</code>；<code>return values&#91;source&#93;, values&#91;target&#93;, nil</code> |
| <code>target</code> | <code>uint64</code> | 在此声明中的实际使用：<code>ids := &#91;&#93;uint64{source, target}</code>；<code>return values&#91;source&#93;, values&#91;target&#93;, nil</code> |
| <code>limit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockQuota(ctx, tx, id, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Quota</code> | 实际返回表达式：<code>Quota{}</code>；<code>values&#91;source&#93;</code> |
| <code>(匿名/未命名)</code> | <code>Quota</code> | 实际返回表达式：<code>Quota{}</code>；<code>values&#91;target&#93;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>ok</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Quota{}, Quota{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return values&#91;source&#93;, values&#91;target&#93;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 49 bytes=46902:47489 -->
````go
// 所有转让按照空间编号持锁，然后才锁资源。计划预留无 TTL：UNKNOWN 不归还预算。
func LockTransferQuotas(ctx context.Context, tx *sql.Tx, source, target, limit uint64) (Quota, Quota, error) {
	ids := []uint64{source, target}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	values := map[uint64]Quota{}
	for _, id := range ids {
		if _, ok := values[id]; ok {
			continue
		}
		q, err := LockQuota(ctx, tx, id, limit)
		if err != nil {
			return Quota{}, Quota{}, err
		}
		values[id] = q
	}
	return values[source], values[target], nil
}

````

## 第 50 步：ReserveTransferCapacity

插入位置：第 49 步代码之后，紧接追加；完成稿约第 1008 行。

本步声明：`func ReserveTransferCapacity(ctx context.Context, tx *sql.Tx, q Quota, bytes, expected uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>writeQuota(ctx, tx, q, q.Used, q.Reserved+bytes)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>writeQuota(ctx, tx, q, q.Used, q.Reserved+bytes)</code> |
| <code>q</code> | <code>Quota</code> | 在此声明中的实际使用：<code>q.Revision</code>；<code>q.Used</code>；<code>q.Limit</code>；<code>q.Reserved</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>bytes</code> | <code>uint64</code> | 在此声明中的实际使用：<code>bytes &gt; q.Limit-q.Used-q.Reserved</code>；<code>q.Reserved+bytes</code> |
| <code>expected</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.Revision != expected</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;QUOTA_REVISION_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;QUOTA_EXCEEDED&quot;)</code>；<code>errors.New(&quot;QUOTA_REVISION_EXHAUSTED&quot;)</code>；<code>writeQuota(ctx, tx, q, q.Used, q.Reserved+bytes)</code> |

**跟着本段代码走一遍：**

- 检查 <code>q.Revision != expected</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_REVISION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>q.Used &gt; q.Limit &#124;&#124; q.Reserved &gt; q.Limit-q.Used &#124;&#124; bytes &gt; q.Limit-q.Used-q.Reserved</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;QUOTA_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>q.Revision == ^uint64(0)</code>。条件成立时执行 <code>return errors.New(&quot;QUOTA_REVISION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return writeQuota(ctx, tx, q, q.Used, q.Reserved+bytes)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 50 bytes=47489:48000 -->
````go
func ReserveTransferCapacity(ctx context.Context, tx *sql.Tx, q Quota, bytes, expected uint64) error {
	if q.Revision != expected {
		return domain.Violation(domain.StateConflict, "QUOTA_REVISION_CHANGED")
	}
	if q.Used > q.Limit || q.Reserved > q.Limit-q.Used || bytes > q.Limit-q.Used-q.Reserved {
		return domain.Violation(domain.StateConflict, "QUOTA_EXCEEDED")
	}
	if q.Revision == ^uint64(0) {
		return errors.New("QUOTA_REVISION_EXHAUSTED")
	}
	return writeQuota(ctx, tx, q, q.Used, q.Reserved+bytes)
}

````

## 第 51 步：CommitTransferCapacity

插入位置：第 50 步代码之后，紧接追加；完成稿约第 1021 行。

本步声明：`func CommitTransferCapacity(ctx context.Context, tx *sql.Tx, source, target Quota, reserved, released, redundant uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>writeQuota(ctx, tx, source, source.Used-released, source.Reserved)</code>；<code>writeQuota(ctx, tx, target, target.Used+reserved-redundant, target.Reserved-reserved)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>writeQuota(ctx, tx, source, source.Used-released, source.Reserved)</code>；<code>writeQuota(ctx, tx, target, target.Used+reserved-redundant, target.Reserved-reserved)</code> |
| <code>source</code> | <code>Quota</code> | 在此声明中的实际使用：<code>source.Used</code>；<code>writeQuota(ctx, tx, source, source.Used-released, source.Reserved)</code>；<code>source.Reserved</code> |
| <code>target</code> | <code>Quota</code> | 在此声明中的实际使用：<code>target.Reserved</code>；<code>target.Limit</code>；<code>target.Used</code>；<code>writeQuota(ctx, tx, target, target.Used+reserved-redundant, target.Reserved-reserved)</code> |
| <code>reserved</code> | <code>uint64</code> | 在此声明中的实际使用：<code>redundant &gt; reserved</code>；<code>reserved &gt; target.Reserved</code>；<code>reserved-redundant</code>；<code>target.Used+reserved</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>released</code> | <code>uint64</code> | 在此声明中的实际使用：<code>released &gt; source.Used</code>；<code>source.Used-released</code> |
| <code>redundant</code> | <code>uint64</code> | 在此声明中的实际使用：<code>redundant &gt; reserved</code>；<code>reserved-redundant</code>；<code>target.Used+reserved-redundant</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;TRANSFER_QUOTA_INCONSISTENT&quot;)</code>；<code>err</code>；<code>writeQuota(ctx, tx, target, target.Used+reserved-redundant, target.Reserved-reserved)</code> |

**跟着本段代码走一遍：**

- 检查 <code>redundant &gt; reserved &#124;&#124; reserved &gt; target.Reserved &#124;&#124; reserved-redundant &gt; target.Limit-target.Used &#124;&#124; released &gt; source.Used</code>。条件成立时执行 <code>return errors.New(&quot;TRANSFER_QUOTA_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return writeQuota(ctx, tx, target, target.Used+reserved-redundant, target.Reserved-reserved)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 51 bytes=48000:48520 -->
````go
func CommitTransferCapacity(ctx context.Context, tx *sql.Tx, source, target Quota, reserved, released, redundant uint64) error {
	if redundant > reserved || reserved > target.Reserved || reserved-redundant > target.Limit-target.Used || released > source.Used {
		return errors.New("TRANSFER_QUOTA_INCONSISTENT")
	}
	if err := writeQuota(ctx, tx, source, source.Used-released, source.Reserved); err != nil {
		return err
	}
	return writeQuota(ctx, tx, target, target.Used+reserved-redundant, target.Reserved-reserved)
}

````

## 第 52 步：ReleaseTransferCapacity

插入位置：第 51 步代码之后，紧接追加；完成稿约第 1031 行。

本步声明：`func ReleaseTransferCapacity(ctx context.Context, tx *sql.Tx, q Quota, reserved uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved)</code> |
| <code>q</code> | <code>Quota</code> | 在此声明中的实际使用：<code>q.Reserved</code>；<code>writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved)</code>；<code>q.Used</code> |
| <code>reserved</code> | <code>uint64</code> | 在此声明中的实际使用：<code>reserved &gt; q.Reserved</code>；<code>q.Reserved-reserved</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;TRANSFER_QUOTA_INCONSISTENT&quot;)</code>；<code>writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved)</code> |

**跟着本段代码走一遍：**

- 检查 <code>reserved &gt; q.Reserved</code>。条件成立时执行 <code>return errors.New(&quot;TRANSFER_QUOTA_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 52 bytes=48520:48761 -->
````go
func ReleaseTransferCapacity(ctx context.Context, tx *sql.Tx, q Quota, reserved uint64) error {
	if reserved > q.Reserved {
		return errors.New("TRANSFER_QUOTA_INCONSISTENT")
	}
	return writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved)
}

````

## 第 53 步：PrepareTransferResources

插入位置：第 52 步代码之后，紧接追加；完成稿约第 1038 行。

本步声明：`func PrepareTransferResources(ctx context.Context, tx *sql.Tx, document, source, target uint64) ([]domain.TransferResource, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT DISTINCT id FROM (  SELECT resource_id id FROM resource_reference WHERE document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;)  UNION SELEC …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT space_id,byte_size,state,upload_operation_id FROM resource WHERE id=? FOR UPDATE`, id)</code>；<code>ResourceAllowedForDocument(ctx, tx, id, document, source)</code>；<code>tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id)    SELECT id,space_id,byte_size,&#x27;ACTIVE&#x27;,upload_operation_id FROM resource …</code>（长表达式，按本步完整代码继续）<br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code>；<code>tx.QueryRowContext</code>；<code>ResourceAllowedForDocument(ctx, tx, id, document, source)</code>；<code>tx.ExecContext</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT DISTINCT id FROM (  SELECT resource_id id FROM resource_reference WHERE document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;)  UNION SELEC …</code>（长表达式，按本步完整代码继续）；<code>ResourceAllowedForDocument(ctx, tx, id, document, source)</code> |
| <code>source</code> | <code>uint64</code> | 在此声明中的实际使用：<code>ResourceAllowedForDocument(ctx, tx, id, document, source)</code> |
| <code>target</code> | <code>uint64</code> | 在此声明中的实际使用：<code>lockedAllocationActive(ctx, tx, id, target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.TransferResource</code> | 使用 <code>domain.TransferResource</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>total</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRANSFER_RESOURCE_NOT_READY&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;TRANSFER_RESOURCE_SCOPE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT DISTINCT id FROM (  SELECT resource_id id FROM resource_reference WHERE document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;)  UNION SELEC …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, 0, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return nil, 0, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,byte_size,state,upload_operation_id FROM resource WHERE id=? FOR UPDATE`, id).Scan(&amp;physical, &amp;size, &amp;state, &amp;operation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,byte_size,state,upload_operation_id FROM resource WHERE id=? FOR UPDATE`, id)</code>。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return nil, 0, domain.Violation(domain.StateConflict, &quot;TRANSFER_RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, 0, err</code>。当前函数在这里结束。
- 检查 <code>!allowed</code>。条件成立时执行 <code>return nil, 0, domain.Violation(domain.PermissionDenied, &quot;TRANSFER_RESOURCE_SCOPE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id)    SELECT id,space_id,byte_size,&#x27;ACTIVE&#x27;,upload_operation_id FROM resource …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 53 bytes=48761:50927 -->
````go
func PrepareTransferResources(ctx context.Context, tx *sql.Tx, document, source, target uint64) ([]domain.TransferResource, uint64, error) {
	// 保护先于 Node ACK 持久化，覆盖尚未转为 DRAFT 引用的已保存更新。
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT id FROM (
 SELECT resource_id id FROM resource_reference WHERE document_id=? AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD')
 UNION SELECT j.id FROM resource_protection p JOIN JSON_TABLE(p.resource_ids,'$[*]' COLUMNS(id BIGINT UNSIGNED PATH '$')) j
 WHERE p.document_id=? AND p.state<>'RELEASED') x ORDER BY id`, document, document)
	if err != nil {
		return nil, 0, err
	}
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	result := make([]domain.TransferResource, 0, len(ids))
	var total uint64
	for _, id := range ids {
		var physical, size uint64
		var state, operation string
		if err = tx.QueryRowContext(ctx, `SELECT space_id,byte_size,state,upload_operation_id FROM resource WHERE id=? FOR UPDATE`, id).Scan(&physical, &size, &state, &operation); err != nil {
			return nil, 0, err
		}
		if state != "READY" {
			return nil, 0, domain.Violation(domain.StateConflict, "TRANSFER_RESOURCE_NOT_READY")
		}
		allowed, err := ResourceAllowedForDocument(ctx, tx, id, document, source)
		if err != nil {
			return nil, 0, err
		}
		if !allowed {
			return nil, 0, domain.Violation(domain.PermissionDenied, "TRANSFER_RESOURCE_SCOPE_CHANGED")
		}
		// 兼容迁移后上传：只有真实 COMMITTED 原上传才能物化原计费事实。
		if _, err = tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id)
   SELECT id,space_id,byte_size,'ACTIVE',upload_operation_id FROM resource r WHERE id=?
   AND EXISTS(SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id AND q.state='COMMITTED' AND q.bytes=r.byte_size AND q.space_id=r.space_id)
   ON DUPLICATE KEY UPDATE resource_id=VALUES(resource_id)`, id); err != nil {
````

## 第 54 步：PrepareTransferResources（接着实现）

插入位置：第 53 步代码之后，紧接追加；完成稿约第 1084 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, 0, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, 0, err</code>。当前函数在这里结束。
- 检查 <code>r.TargetCharge</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, total, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 54 bytes=50927:51322 -->
````go
			return nil, 0, err
		}
		charged, err := lockedAllocationActive(ctx, tx, id, target)
		if err != nil {
			return nil, 0, err
		}
		r := domain.TransferResource{ID: id, Size: size, TargetCharge: !charged}
		result = append(result, r)
		if r.TargetCharge {
			total, err = domain.TransferCapacity(total, size)
			if err != nil {
				return nil, 0, err
			}
		}
	}
	return result, total, nil
}

````

## 第 55 步：ApplyTransferResources

插入位置：第 54 步代码之后，紧接追加；完成稿约第 1102 行。

本步声明：`func ApplyTransferResources(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) (uint64, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT byte_size,state FROM resource WHERE id=? FOR UPDATE`, r.ID)</code>；<code>lockedAllocationActive(ctx, tx, r.ID, p.TargetSpace)</code>；<code>tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id) VALUES(?,?,?,&#x27;ACTIVE&#x27;,?)      ON DUPLICATE KEY UPDATE bytes=VALUES(bytes), …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `INSERT INTO document_resource_allocation(resource_id,document_id,space_id,transfer_id,state) VALUES(?,?,?,?,&#x27;ACTIVE&#x27;)    ON DUPLICATE KEY UPDATE transfer_id=VA …</code>（长表达式，按本步完整代码继续）<br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>lockedAllocationActive(ctx, tx, r.ID, p.TargetSpace)</code>；<code>tx.ExecContext</code>；<code>TransferResourceConsumerDocuments(ctx, tx, &#91;&#93;domain.TransferResource{r}, true)</code> |
| <code>p</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>在此声明中的实际使用：<code>p.Resources</code>；<code>p.TargetSpace</code>；<code>p.Operation</code>；<code>p.Document</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>released</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>redundant</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRANSFER_RESOURCE_CHANGED&quot;)</code>；<code>errors.New(&quot;TRANSFER_ALLOCATION_INCONSISTENT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT byte_size,state FROM resource WHERE id=? FOR UPDATE`, r.ID).Scan(&amp;size, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT byte_size,state FROM resource WHERE id=? FOR UPDATE`, r.ID)</code>。
- 检查 <code>size != r.Size &#124;&#124; state != &quot;READY&quot;</code>。条件成立时执行 <code>return 0, 0, domain.Violation(domain.StateConflict, &quot;TRANSFER_RESOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>r.TargetCharge</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 检查 <code>active</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id) VALUES(?,?,?,&#x27;ACTIVE&#x27;,?)      ON DUPLICATE KEY UPDATE bytes=VALUES(bytes), …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO document_resource_allocation(resource_id,document_id,space_id,transfer_id,state) VALUES(?,?,?,?,&#x27;ACTIVE&#x27;)    ON DUPLICATE KEY UPDATE transfer_id=VA …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE document_resource_allocation SET state=&#x27;RELEASED&#x27; WHERE resource_id=? AND document_id=? AND space_id=?`, r.ID, p.Document, p.SourceSpace)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 检查 <code>id == p.Document</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。

<!-- write-step: 55 bytes=51322:53402 -->
````go
func ApplyTransferResources(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) (uint64, uint64, error) {
	var released, redundant uint64
	for _, r := range p.Resources {
		var size uint64
		var state string
		if err := tx.QueryRowContext(ctx, `SELECT byte_size,state FROM resource WHERE id=? FOR UPDATE`, r.ID).Scan(&size, &state); err != nil {
			return 0, 0, err
		}
		if size != r.Size || state != "READY" {
			return 0, 0, domain.Violation(domain.StateConflict, "TRANSFER_RESOURCE_CHANGED")
		}
		if r.TargetCharge {
			// 目标的另一份计划可能先提交同资源 allocation。容量预留仍真实存在，
			// 多收的部分在本事务归还，不因并发共享增加第二笔 used。
			active, err := lockedAllocationActive(ctx, tx, r.ID, p.TargetSpace)
			if err != nil {
				return 0, 0, err
			}
			if active {
				var err error
				redundant, err = domain.TransferCapacity(redundant, r.Size)
				if err != nil {
					return 0, 0, err
				}
			} else {
				if _, err := tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id) VALUES(?,?,?,'ACTIVE',?)
     ON DUPLICATE KEY UPDATE bytes=VALUES(bytes),state='ACTIVE',operation_id=VALUES(operation_id)`, r.ID, p.TargetSpace, r.Size, p.Operation); err != nil {
					return 0, 0, err
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO document_resource_allocation(resource_id,document_id,space_id,transfer_id,state) VALUES(?,?,?,?,'ACTIVE')
   ON DUPLICATE KEY UPDATE transfer_id=VALUES(transfer_id),state='ACTIVE'`, r.ID, p.Document, p.TargetSpace, p.Transfer); err != nil {
			return 0, 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE document_resource_allocation SET state='RELEASED' WHERE resource_id=? AND document_id=? AND space_id=?`, r.ID, p.Document, p.SourceSpace); err != nil {
			return 0, 0, err
		}
		consumers, err := TransferResourceConsumerDocuments(ctx, tx, []domain.TransferResource{r}, true)
		if err != nil {
			return 0, 0, err
		}
		others := false
		for _, id := range consumers {
			if id == p.Document {
````

## 第 56 步：ApplyTransferResources（接着实现）

插入位置：第 55 步代码之后，紧接追加；完成稿约第 1147 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=? FOR UPDATE`, id).Scan(&amp;space)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=? FOR UPDATE`, id)</code>。
- 检查 <code>space == p.SourceSpace</code>。
- 检查 <code>!others</code>。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT bytes FROM resource_space_allocation WHERE resource_id=? AND space_id=? AND state=&#x27;ACTIVE&#x27; FOR UPDATE`, r.ID, p.SourceSpace).Scan(&amp;bytes)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT bytes FROM resource_space_allocation WHERE resource_id=? AND space_id=? AND state=&#x27;ACTIVE&#x27; FOR UPDATE`, r.ID, p.SourceSpace)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 检查 <code>bytes != r.Size</code>。条件成立时执行 <code>return 0, 0, errors.New(&quot;TRANSFER_ALLOCATION_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state=&#x27;RELEASED&#x27; WHERE resource_id=? AND space_id=?`, r.ID, p.SourceSpace)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return released, redundant, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 56 bytes=53402:54487 -->
````go
				continue
			}
			var space uint64
			// 调用方已在配额/资源之前锁定全部消费者 Doc，这里只是当前读确认空间。
			if err = tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=? FOR UPDATE`, id).Scan(&space); err != nil {
				return 0, 0, err
			}
			if space == p.SourceSpace {
				others = true
			}
		}
		if !others {
			var bytes uint64
			err := tx.QueryRowContext(ctx, `SELECT bytes FROM resource_space_allocation WHERE resource_id=? AND space_id=? AND state='ACTIVE' FOR UPDATE`, r.ID, p.SourceSpace).Scan(&bytes)
			if err == sql.ErrNoRows {
				continue
			}
			if err != nil {
				return 0, 0, err
			}
			if bytes != r.Size {
				return 0, 0, errors.New("TRANSFER_ALLOCATION_INCONSISTENT")
			}
			released, err = domain.TransferCapacity(released, bytes)
			if err != nil {
				return 0, 0, err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state='RELEASED' WHERE resource_id=? AND space_id=?`, r.ID, p.SourceSpace); err != nil {
				return 0, 0, err
			}
		}
	}
	return released, redundant, nil
}

````

## 第 57 步：RefreshTransferResourceCharges

插入位置：第 56 步代码之后，紧接追加；完成稿约第 1182 行。

最终行锁下重计目标空间已经存在的分配。接受固定范围不变；共享资源可能由另一个
原迁移先迁入或迁出，所以旧 TargetCharge 不能决定现在的真实预算。

本步声明：`func RefreshTransferResourceCharges(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) (domain.PersonalTransferPlan, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT byte_size,state FROM resource WHERE id=? FOR UPDATE`, r.ID)</code>；<code>lockedAllocationActive(ctx, tx, r.ID, p.TargetSpace)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>lockedAllocationActive(ctx, tx, r.ID, p.TargetSpace)</code> |
| <code>p</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>在此声明中的实际使用：<code>p.Resources</code>；<code>return p, 0, err</code>；<code>return p, 0, domain.Violation(domain.StateConflict, &quot;TRANSFER_RESOURCE_CHANGED&quot;)</code>；<code>p.TargetSpace</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>实际返回表达式：<code>p</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>total</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRANSFER_RESOURCE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return p, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT byte_size,state FROM resource WHERE id=? FOR UPDATE`, r.ID).Scan(&amp;size, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT byte_size,state FROM resource WHERE id=? FOR UPDATE`, r.ID)</code>。
- 检查 <code>size != r.Size &#124;&#124; state != &quot;READY&quot;</code>。条件成立时执行 <code>return p, 0, domain.Violation(domain.StateConflict, &quot;TRANSFER_RESOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return p, 0, err</code>。当前函数在这里结束。
- 检查 <code>!charged</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return p, 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return p, total, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 57 bytes=54487:55542 -->
````go
// 最终行锁下重计目标空间已经存在的分配。接受固定范围不变；共享资源可能由另一个
// 原迁移先迁入或迁出，所以旧 TargetCharge 不能决定现在的真实预算。
func RefreshTransferResourceCharges(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) (domain.PersonalTransferPlan, uint64, error) {
	var total uint64
	for i, r := range p.Resources {
		var size uint64
		var state string
		if err := tx.QueryRowContext(ctx, `SELECT byte_size,state FROM resource WHERE id=? FOR UPDATE`, r.ID).Scan(&size, &state); err != nil {
			return p, 0, err
		}
		if size != r.Size || state != "READY" {
			return p, 0, domain.Violation(domain.StateConflict, "TRANSFER_RESOURCE_CHANGED")
		}
		charged, err := lockedAllocationActive(ctx, tx, r.ID, p.TargetSpace)
		if err != nil {
			return p, 0, err
		}
		p.Resources[i].TargetCharge = !charged
		if !charged {
			var err error
			total, err = domain.TransferCapacity(total, size)
			if err != nil {
				return p, 0, err
			}
		}
	}
	return p, total, nil
}

````

## 第 58 步：lockedAllocationActive

插入位置：第 57 步代码之后，紧接追加；完成稿约第 1211 行。

本步声明：`func lockedAllocationActive(ctx context.Context, tx *sql.Tx, resource, space uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT state FROM resource_space_allocation WHERE resource_id=? AND space_id=? FOR UPDATE`, resource, space)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT state FROM resource_space_allocation WHERE resource_id=? AND space_id=? FOR UPDATE`, resource, space)</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT state FROM resource_space_allocation WHERE resource_id=? AND space_id=? FOR UPDATE`, resource, space)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>state == &quot;ACTIVE&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT state FROM resource_space_allocation WHERE resource_id=? AND space_id=? FOR UPDATE`, resource, space).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT state FROM resource_space_allocation WHERE resource_id=? AND space_id=? FOR UPDATE`, resource, space)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 此处通过 <code>return state == &quot;ACTIVE&quot;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 58 bytes=55542:55940 -->
````go
func lockedAllocationActive(ctx context.Context, tx *sql.Tx, resource, space uint64) (bool, error) {
	var state string
	err := tx.QueryRowContext(ctx, `SELECT state FROM resource_space_allocation WHERE resource_id=? AND space_id=? FOR UPDATE`, resource, space).Scan(&state)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return state == "ACTIVE", nil
}

````

## 第 59 步：ResourceAllocationSpaces

插入位置：第 58 步代码之后，紧接追加；完成稿约第 1223 行。

GC 调用方须先锁所有真实 allocation 的配额，再锁资源并证明无引用/UNKNOWN保护。
MarkDeleted 的 CAS 与本函数同事务；重复删除不能二次返还 used。

本步声明：`func ResourceAllocationSpaces(ctx context.Context, q interface { 	QueryContext(context.Context, string, ...any) (*sql.Rows, error) }, id uint64) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT space_id FROM resource_space_allocation WHERE resource_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY space_id`, id)</code> |
| <code>q</code> | <code>interface { 	QueryContext(context.Context, string, ...any) (*sql.Rows, error) }</code> | 在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT space_id FROM resource_space_allocation WHERE resource_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY space_id`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT space_id FROM resource_space_allocation WHERE resource_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY space_id`, id)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 59 bytes=55940:56673 -->
````go
// GC 调用方须先锁所有真实 allocation 的配额，再锁资源并证明无引用/UNKNOWN保护。
// MarkDeleted 的 CAS 与本函数同事务；重复删除不能二次返还 used。
func ResourceAllocationSpaces(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, id uint64) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, `SELECT space_id FROM resource_space_allocation WHERE resource_id=? AND state='ACTIVE' ORDER BY space_id`, id)
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

## 第 60 步：ReleaseResourceAllocations

插入位置：第 59 步代码之后，紧接追加；完成稿约第 1244 行。

本步声明：`func ReleaseResourceAllocations(ctx context.Context, tx *sql.Tx, id uint64, quotas map[uint64]Quota) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT space_id,bytes FROM resource_space_allocation WHERE resource_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY space_id FOR UPDATE`, id)</code>；<code>writeQuota(ctx, tx, q, q.Used-a.bytes, q.Reserved)</code>；<code>tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state=&#x27;RELEASED&#x27; WHERE resource_id=? AND state=&#x27;ACTIVE&#x27;`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code>；<code>writeQuota(ctx, tx, q, q.Used-a.bytes, q.Reserved)</code>；<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT space_id,bytes FROM resource_space_allocation WHERE resource_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY space_id FOR UPDATE`, id)</code>；<code>tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state=&#x27;RELEASED&#x27; WHERE resource_id=? AND state=&#x27;ACTIVE&#x27;`, id)</code> |
| <code>quotas</code> | <code>map&#91;uint64&#93;Quota</code> | 在此声明中的实际使用：<code>q, ok := quotas&#91;a.space&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;RESOURCE_ALLOCATION_QUOTA_INCONSISTENT&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT space_id,bytes FROM resource_space_allocation WHERE resource_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY space_id FOR UPDATE`, id)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;a.space, &amp;a.bytes)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; a.bytes &gt; q.Used</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_ALLOCATION_QUOTA_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state=&#x27;RELEASED&#x27; WHERE resource_id=? AND state=&#x27;ACTIVE&#x27;`, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 60 bytes=56673:57737 -->
````go
func ReleaseResourceAllocations(ctx context.Context, tx *sql.Tx, id uint64, quotas map[uint64]Quota) error {
	rows, err := tx.QueryContext(ctx, `SELECT space_id,bytes FROM resource_space_allocation WHERE resource_id=? AND state='ACTIVE' ORDER BY space_id FOR UPDATE`, id)
	if err != nil {
		return err
	}
	type allocation struct{ space, bytes uint64 }
	items := []allocation{}
	for rows.Next() {
		var a allocation
		if err = rows.Scan(&a.space, &a.bytes); err != nil {
			rows.Close()
			return err
		}
		items = append(items, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, a := range items {
		q, ok := quotas[a.space]
		if !ok || a.bytes > q.Used {
			return errors.New("RESOURCE_ALLOCATION_QUOTA_INCONSISTENT")
		}
		if err = writeQuota(ctx, tx, q, q.Used-a.bytes, q.Reserved); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state='RELEASED' WHERE resource_id=? AND state='ACTIVE'`, id)
	return err
}

// transfer_consumers.go：保留原实现，按业务合并文件。

````

## 第 61 步：TransferResourceConsumerDocuments

插入位置：第 60 步代码之后，紧接追加；完成稿约第 1279 行。

先发现共享 Doc 集合，调用方按编号在 quota/resource 之前锁这些 Doc。
资源锁后再次当前读核对集合，若发现新消费者则重试，避免 REPEATABLE READ 的
旧空间快照错误释放另一篇文档仍在使用的源计费分配。

本步声明：`func TransferResourceConsumerDocuments(ctx context.Context, q interface { 	QueryContext(context.Context, string, ...any) (*sql.Rows, error) }, items []domain.TransferResource, lock bool) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT document_id FROM resource_reference WHERE resource_id IN (`+marks+`) AND document_id IS NOT NULL  AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;) O …</code>（长表达式，按本步完整代码继续）；<code>q.QueryContext(ctx, `SELECT p.document_id FROM resource_protection p  JOIN JSON_TABLE(p.resource_ids,&#x27;$&#91;*&#93;&#x27; COLUMNS(resource_id BIGINT UNSIGNED PATH &#x27;$&#x27;)) j  WHERE p.state&lt;&gt;&#x27;RELEAS …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>interface { 	QueryContext(context.Context, string, ...any) (*sql.Rows, error) }</code> | 在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>items</code> | <code>&#91;&#93;domain.TransferResource</code> | 使用 <code>domain.TransferResource</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>在此声明中的实际使用：<code>len(items)</code>；<code>resourceIDs(items)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>&#91;&#93;uint64{}</code>；<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(items) == 0</code>。条件成立时执行 <code>return &#91;&#93;uint64{}, nil</code>。当前函数在这里结束。
- 检查 <code>lock</code>。
- 执行 <code>q.QueryContext(ctx, `SELECT document_id FROM resource_reference WHERE resource_id IN (`+marks+`) AND document_id IS NOT NULL  AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;) O …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>lock</code>。
- 执行 <code>q.QueryContext(ctx, `SELECT p.document_id FROM resource_protection p  JOIN JSON_TABLE(p.resource_ids,&#x27;$&#91;*&#93;&#x27; COLUMNS(resource_id BIGINT UNSIGNED PATH &#x27;$&#x27;)) j  WHERE p.state&lt;&gt;&#x27;RELEAS …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 61 bytes=57737:59664 -->
````go
// 先发现共享 Doc 集合，调用方按编号在 quota/resource 之前锁这些 Doc。
// 资源锁后再次当前读核对集合，若发现新消费者则重试，避免 REPEATABLE READ 的
// 旧空间快照错误释放另一篇文档仍在使用的源计费分配。
func TransferResourceConsumerDocuments(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, items []domain.TransferResource, lock bool) ([]uint64, error) {
	if len(items) == 0 {
		return []uint64{}, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(items)), ",")
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	// 直接锁真实引用行，在下面已有的集合中去重，保留当前读而非旧快照。
	rows, err := q.QueryContext(ctx, `SELECT document_id FROM resource_reference WHERE resource_id IN (`+marks+`) AND document_id IS NOT NULL
 AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD') ORDER BY document_id`+suffix, resourceIDs(items)...)
	if err != nil {
		return nil, err
	}
	ids := map[uint64]bool{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// 已确认保护即使尚未成为普通引用，也属于真实消费者；UNKNOWN 同样保留。
	protectionLock := ""
	if lock {
		protectionLock = " FOR UPDATE OF p"
	}
	// JSON_TABLE 是同一保护行的展开视图；只锁 p，否则 MySQL 把函数表加入
	// 锁定子句时触发 ER_DUPLICATE_TABLE_LOCK，原迁移无法落定。
	rows, err = q.QueryContext(ctx, `SELECT p.document_id FROM resource_protection p
 JOIN JSON_TABLE(p.resource_ids,'$[*]' COLUMNS(resource_id BIGINT UNSIGNED PATH '$')) j
 WHERE p.state<>'RELEASED' AND j.resource_id IN (`+marks+`) ORDER BY p.document_id`+protectionLock, resourceIDs(items)...)
	if err != nil {
````

## 第 62 步：TransferResourceConsumerDocuments（接着实现）

插入位置：第 61 步代码之后，紧接追加；完成稿约第 1324 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result&#91;i&#93; &lt; result&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 62 bytes=59664:60081 -->
````go
		return nil, err
	}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := make([]uint64, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

````

## 第 63 步：resourceIDs

插入位置：第 62 步代码之后，紧接追加；完成稿约第 1347 行。

本步声明：`func resourceIDs(items []domain.TransferResource) []any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>items</code> | <code>&#91;&#93;domain.TransferResource</code> | 使用 <code>domain.TransferResource</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>在此声明中的实际使用：<code>len(items)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;any</code> | 实际返回表达式：<code>args</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return args</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 63 bytes=60081:60300 -->
````go
func resourceIDs(items []domain.TransferResource) []any {
	args := make([]any, len(items))
	for i, item := range items {
		args[i] = item.ID
	}
	return args
}

// upload.go：保留原实现，按业务合并文件。

````

## 第 64 步：Upload

插入位置：第 63 步代码之后，紧接追加；完成稿约第 1357 行。

本步声明：`type Upload struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 上传会话编号。 |
| <code>Space</code> | <code>uint64</code> | 此次上传记账的空间。 |
| <code>Document</code> | <code>uint64</code> | 上传资源准备关联的文档。 |
| <code>Uploader</code> | <code>uint64</code> | 提交上传的账号。 |
| <code>Size</code> | <code>uint64</code> | 预期上传字节数。 |
| <code>Revision</code> | <code>uint64</code> | 上传会话的并发修订号。 |
| <code>Operation</code> | <code>string</code> | 原上传业务操作号。 |
| <code>Key</code> | <code>string</code> | 上传对象存储键，恢复沿用同一键。 |
| <code>MediaType</code> | <code>string</code> | 上传声明并经核验的媒体类型。 |
| <code>State</code> | <code>string</code> | 上传会话状态，决定可否完成或清理。 |
| <code>Checksum</code> | <code>&#91;32&#93;byte</code> | 预期完整内容的SHA-256摘要。 |
| <code>Expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。 |

<!-- write-step: 64 bytes=60300:60545 -->
````go
type Upload struct {
	ID, Space, Document, Uploader, Size, Revision uint64
	Operation, Key, MediaType, State              string
	Checksum                                      [32]byte
	Expires                                       time.Time
}

````

## 第 65 步：Queryer

插入位置：第 64 步代码之后，紧接追加；完成稿约第 1364 行。

本步声明：`type Queryer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>QueryRowContext</code> | <code>(context.Context, string, ...any) *sql.Row</code> | 依次接收ctx、带占位符的SQL和占位符参数；返回单行句柄，实际查询/无行错误在Scan时处理。接口让连接池或原事务都可注入，但涉及锁时必须沿用原事务。 |

<!-- write-step: 65 bytes=60545:60632 -->
````go
type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

````

## 第 66 步：UploadRow

插入位置：第 65 步代码之后，紧接追加；完成稿约第 1368 行。

UploadRow从持久UPLOAD引用取得原文档，不能用调用者提供的另一个document_id替换。

本步声明：`func UploadRow(ctx context.Context, q Queryer, id uint64, lock bool) (Upload, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Upload</code> | 实际返回表达式：<code>u</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;RESOURCE_UPLOAD_READ_FAILED&quot;)</code>；<code>errors.New(&quot;RESOURCE_UPLOAD_BINDING_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, query, id).Scan(&amp;u.ID, &amp;u.Space, &amp;u.Document, &amp;u.Uploader, &amp;u.Size, &amp;u.Revision, &amp;u.Operation, &amp;u.Key, &amp;u.MediaType, &amp;u.State, &amp;checksum, &amp;u.Expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, query, id)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return u, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return u, errors.New(&quot;RESOURCE_UPLOAD_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(checksum) != 32 &#124;&#124; u.Document == 0 &#124;&#124; u.Revision == 0</code>。条件成立时执行 <code>return u, errors.New(&quot;RESOURCE_UPLOAD_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return u, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 66 bytes=60632:61779 -->
````go
// UploadRow从持久UPLOAD引用取得原文档，不能用调用者提供的另一个document_id替换。
func UploadRow(ctx context.Context, q Queryer, id uint64, lock bool) (Upload, error) {
	var u Upload
	var checksum []byte
	query := `SELECT r.id,r.space_id,x.document_id,r.uploader_id,r.byte_size,r.revision,r.upload_operation_id,r.storage_key,r.media_type,r.state,r.checksum,q.expires_at
		FROM resource r JOIN resource_reference x ON x.resource_id=r.id AND x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'
		JOIN quota_reservation q ON q.operation_id=r.upload_operation_id WHERE r.id=?`
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&u.ID, &u.Space, &u.Document, &u.Uploader, &u.Size, &u.Revision, &u.Operation, &u.Key, &u.MediaType, &u.State, &checksum, &u.Expires)
	if err == sql.ErrNoRows {
		return u, err
	}
	if err != nil {
		return u, errors.New("RESOURCE_UPLOAD_READ_FAILED")
	}
	if len(checksum) != 32 || u.Document == 0 || u.Revision == 0 {
		return u, errors.New("RESOURCE_UPLOAD_BINDING_INVALID")
	}
	copy(u.Checksum[:], checksum)
	return u, nil
}

````

## 第 67 步：InsertUpload

插入位置：第 66 步代码之后，紧接追加；完成稿约第 1392 行。

InsertUpload必须与quota reservation和通用操作回执使用同一事务。

本步声明：`func InsertUpload(ctx context.Context, tx *sql.Tx, u Upload) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO resource(space_id,uploader_id,storage_key,checksum,byte_size,media_type,state,upload_operation_id) VALUES(?,?,?,?,?,?,&#x27;UPLOADING&#x27;,?)`, u.Space, u.U …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;)</code>；<code>tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,&#x27;OPERATION&#x27;,?,&#x27;UPLOAD&#x27;)`, id, u.Document, u.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>u</code> | <code>Upload</code> | 在此声明中的实际使用：<code>u.Space</code>；<code>u.Uploader</code>；<code>u.Key</code>；<code>u.Checksum</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RESOURCE_UPLOAD_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;RESOURCE_UPLOAD_BINDING_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;RESOURCE_UPLOAD_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO resource(space_id,uploader_id,storage_key,checksum,byte_size,media_type,state,upload_operation_id) VALUES(?,?,?,?,?,?,&#x27;UPLOADING&#x27;,?)`, u.Space, u.U …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;).Scan(&amp;id) != nil &#124;&#124; id == 0</code>。条件成立时执行 <code>return 0, errors.New(&quot;RESOURCE_UPLOAD_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;).Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;RESOURCE_UPLOAD_BINDING_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,&#x27;OPERATION&#x27;,?,&#x27;UPLOAD&#x27;)`, id, u.Document, u.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return id, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 67 bytes=61779:62710 -->
````go
// InsertUpload必须与quota reservation和通用操作回执使用同一事务。
func InsertUpload(ctx context.Context, tx *sql.Tx, u Upload) (uint64, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO resource(space_id,uploader_id,storage_key,checksum,byte_size,media_type,state,upload_operation_id) VALUES(?,?,?,?,?,?,'UPLOADING',?)`, u.Space, u.Uploader, u.Key, u.Checksum[:], u.Size, u.MediaType, u.Operation); err != nil {
		return 0, errors.New("RESOURCE_UPLOAD_WRITE_FAILED")
	}
	var id uint64
	if tx.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&id) != nil || id == 0 {
		return 0, errors.New("RESOURCE_UPLOAD_WRITE_FAILED")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,'OPERATION',?,'UPLOAD')`, id, u.Document, u.Operation); err != nil {
		return 0, errors.New("RESOURCE_UPLOAD_BINDING_FAILED")
	}
	return id, nil
}

````

## 第 68 步：MarkReady

插入位置：第 67 步代码之后，紧接追加；完成稿约第 1407 行。

本步声明：`func MarkReady(ctx context.Context, tx *sql.Tx, u Upload) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE resource SET state=&#x27;READY&#x27;,revision=revision+1 WHERE id=? AND state=&#x27;UPLOADING&#x27; AND revision=? AND revision&lt;18446744073709551615`, u.ID, u.Revision)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>u</code> | <code>Upload</code> | 在此声明中的实际使用：<code>u.ID</code>；<code>u.Revision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RESOURCE_READY_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;RESOURCE_UPLOAD_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE resource SET state=&#x27;READY&#x27;,revision=revision+1 WHERE id=? AND state=&#x27;UPLOADING&#x27; AND revision=? AND revision&lt;18446744073709551615`, u.ID, u.Revision)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_READY_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>e != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;RESOURCE_UPLOAD_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 68 bytes=62710:63161 -->
````go
func MarkReady(ctx context.Context, tx *sql.Tx, u Upload) error {
	result, err := tx.ExecContext(ctx, `UPDATE resource SET state='READY',revision=revision+1 WHERE id=? AND state='UPLOADING' AND revision=? AND revision<18446744073709551615`, u.ID, u.Revision)
	if err != nil {
		return errors.New("RESOURCE_READY_WRITE_FAILED")
	}
	if n, e := result.RowsAffected(); e != nil || n != 1 {
		return errors.New("RESOURCE_UPLOAD_CHANGED")
	}
	return nil
}

````

## 第 69 步：QuotaLimit

插入位置：第 68 步代码之后，紧接追加；完成稿约第 1418 行。

本步声明：`func QuotaLimit(ctx context.Context, q Queryer, space uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT limit_bytes FROM quota_account WHERE space_id=?&quot;, space)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT limit_bytes FROM quota_account WHERE space_id=?&quot;, space)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>limit</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT limit_bytes FROM quota_account WHERE space_id=?&quot;, space).Scan(&amp;limit)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT limit_bytes FROM quota_account WHERE space_id=?&quot;, space)</code>。
- 此处通过 <code>return limit, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 69 bytes=63161:63390 -->
````go
func QuotaLimit(ctx context.Context, q Queryer, space uint64) (uint64, error) {
	var limit uint64
	err := q.QueryRowContext(ctx, "SELECT limit_bytes FROM quota_account WHERE space_id=?", space).Scan(&limit)
	return limit, err
}

````

## 第 70 步：LockedReservationState

插入位置：第 69 步代码之后，紧接追加；完成稿约第 1424 行。

本步声明：`func LockedReservationState(ctx context.Context, tx *sql.Tx, operation string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM quota_reservation WHERE operation_id=? FOR UPDATE&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM quota_reservation WHERE operation_id=? FOR UPDATE&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM quota_reservation WHERE operation_id=? FOR UPDATE&quot;, operation).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM quota_reservation WHERE operation_id=? FOR UPDATE&quot;, operation)</code>。
- 此处通过 <code>return state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 70 bytes=63390:63653 -->
````go
func LockedReservationState(ctx context.Context, tx *sql.Tx, operation string) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM quota_reservation WHERE operation_id=? FOR UPDATE", operation).Scan(&state)
	return state, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/resource/resource_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
