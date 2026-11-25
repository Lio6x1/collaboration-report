# 分步编写 `services/document/internal/repository/document/approval_actions.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/145-approval-actions.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

从完整数据库事实计算合并批准提示。

浏览器只加载一页历史会漏掉历次提交者或未解决意见，不能以页面状态决定资格。

写之前先按这条执行路线理解各段的连接：

1. MergeApprovalFacts查询请求下提交历史与意见事实，交给领域CanOfferMerge计算按钮提示。

本文件需要守住的边界：

- 提示不替代最终合并事务复核
- 分页历史的空列表不代表没有阻塞意见。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/approval_actions.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/approval_actions.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/approval_actions.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/approval_actions.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:99 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
)

````

## 第 2 步：MergeApprovalFacts

插入位置：第 1 步代码之后，紧接追加；完成稿约第 8 行。

同一输出事务读取全部权威事实。不能为GET调用会插入guard的LockedReviewGuard，
也不能使用分页轮次或分页意见推断“从未提交”和“没有其他人的要求”。

本步声明：`func MergeApprovalFacts(ctx context.Context, q Queryer, request, actor uint64) (domain.MergeApprovalFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT 		EXISTS(SELECT 1 FROM request_submitter s WHERE s.request_id=r.id AND s.user_id=?), 		(SELECT COUNT(*) FROM change_request c WHERE c.request_id=r.id …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT 		EXISTS(SELECT 1 FROM request_submitter s WHERE s.request_id=r.id AND s.user_id=?), 		(SELECT COUNT(*) FROM change_request c WHERE c.request_id=r.id …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT 		EXISTS(SELECT 1 FROM request_submitter s WHERE s.request_id=r.id AND s.user_id=?), 		(SELECT COUNT(*) FROM change_request c WHERE c.request_id=r.id …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.MergeApprovalFacts</code> | 使用 <code>domain.MergeApprovalFacts</code> 类型；其定义处的职责：提示依据完整请求事实，与浏览器加载了多少页历史无关；实际写入仍由合并用例再次核验。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT 		EXISTS(SELECT 1 FROM request_submitter s WHERE s.request_id=r.id AND s.user_id=?), 		(SELECT COUNT(*) FROM change_request c WHERE c.request_id=r.id …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=99:1387 -->
````go
// 同一输出事务读取全部权威事实。不能为GET调用会插入guard的LockedReviewGuard，
// 也不能使用分页轮次或分页意见推断“从未提交”和“没有其他人的要求”。
func MergeApprovalFacts(ctx context.Context, q Queryer, request, actor uint64) (domain.MergeApprovalFacts, error) {
	var value domain.MergeApprovalFacts
	err := q.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM request_submitter s WHERE s.request_id=r.id AND s.user_id=?),
		(SELECT COUNT(*) FROM change_request c WHERE c.request_id=r.id AND c.status='OPEN' AND c.author_id=?),
		(SELECT COUNT(*) FROM change_request c WHERE c.request_id=r.id AND c.status='OPEN' AND c.author_id<>?),
		EXISTS(SELECT 1 FROM unit_review_guard g
			JOIN collaboration_unit u ON u.id=g.unit_id
			JOIN branch b ON b.id=r.branch_id AND b.unit_id=u.id AND b.document_id=r.document_id
			WHERE g.unit_id=r.unit_id AND g.ongoing_request_id=r.id
				AND g.submission_operation_id IS NULL AND r.active_merge_operation_id IS NULL
				AND u.retired_structure_id IS NULL AND u.active_branch_id=b.id AND b.lifecycle='ACTIVE')
		FROM review_request r WHERE r.id=?`, actor, actor, actor, request).
		Scan(&value.EverSubmitted, &value.OwnOpen, &value.ForeignOpen, &value.PositionReady)
	return value, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/approval_actions.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
