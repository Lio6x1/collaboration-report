# 分步编写 `services/document/internal/logic/document/retained_governance_resources.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/231-retained-governance-resources.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

用真实文件锁和IO清理留存资源并结算额度。

删除引用不是物理删除，跨Doc消费者需要保留对象。

写之前先按这条执行路线理解各段的连接：

1. CleanRetainedResource核文档/空间，取文件锁，再锁quota/resource
2. 无消费者才转DELETING、删除、结算。

本文件需要守住的边界：

- UNKNOWN保护继续等待
- 上传预留可结算后才删IO
- 共享资源不能随孤立文档误删。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/retained_governance_resources.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/retained_governance_resources.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/retained_governance_resources.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/retained_governance_resources.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:175 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/resource"
	"context"
	"errors"
)

````

## 第 2 步：Resources.CleanRetainedResource

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

复用资源服务的真实文件排它锁、删除IO和配额结算，不通过删引用就宣布
物理成功。跨Doc引用或已确认保护仍保留；UNKNOWN不能冒充合法共享保留。

本步声明：`func (s *Resources) CleanRetainedResource(ctx context.Context, id, space, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.UploadRow(ctx, s.db, id, false)</code>；<code>s.files.Acquire(ctx, u.Key)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockResourceAllocationQuotas(ctx, tx, u.ID, u.Space, s.limits.SpaceBytes)</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.UploadRow(ctx, s.db, id, false)</code>；<code>repository.UploadRow(ctx, tx, id, true)</code>；<code>repository.ResourceTransferEvidence(ctx, tx, id, document, space)</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>u.Space != space</code>；<code>repository.ResourceTransferEvidence(ctx, tx, id, document, space)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ResourceTransferEvidence(ctx, tx, id, document, space)</code>；<code>repository.RetainedResourceProtection(ctx, tx, u, document)</code>；<code>u.Document != document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_RESOURCE_BINDING_CHANGED&quot;)</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_SHARED_RESOURCE_NOT_READY&quot;)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>release()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>u.Space != space</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!known</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_RESOURCE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>u.State == &quot;DELETED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=175:1548 -->
````go
// 复用资源服务的真实文件排它锁、删除IO和配额结算，不通过删引用就宣布
// 物理成功。跨Doc引用或已确认保护仍保留；UNKNOWN不能冒充合法共享保留。
func (s *Resources) CleanRetainedResource(ctx context.Context, id, space, document uint64) error {
	u, err := repository.UploadRow(ctx, s.db, id, false)
	if err != nil {
		return err
	}
	release, err := s.files.Acquire(ctx, u.Key)
	if err != nil {
		return err
	}
	defer release()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	quotas, err := repository.LockResourceAllocationQuotas(ctx, tx, u.ID, u.Space, s.limits.SpaceBytes)
	if err != nil {
		return err
	}
	u, err = repository.UploadRow(ctx, tx, id, true)
	if err != nil {
		return err
	}
	if u.Space != space {
		known, err := repository.ResourceTransferEvidence(ctx, tx, id, document, space)
		if err != nil {
			return err
		}
		if !known {
			return domain.Violation(domain.StateConflict, "RETAINED_RESOURCE_BINDING_CHANGED")
		}
	}
	if u.State == "DELETED" {
		return nil
	}
	if err = repository.RecordCommittedAllocation(ctx, tx, u.ID); err != nil {
		return err
	}
	if err = repository.ValidateResourceAllocationQuotas(ctx, tx, u.ID, quotas); err != nil {
		return err
	}
	shared, err := repository.RetainedResourceProtection(ctx, tx, u, document)
	if err != nil {
````

## 第 3 步：Resources.CleanRetainedResource（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 55 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>shared</code>。
- 检查 <code>u.State != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_SHARED_RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!committed</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>reservation != &quot;RESERVED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_RESOURCE_RESERVATION_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>committed &amp;&amp; u.State == &quot;READY&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!changed</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_RESOURCE_CLEANUP_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>committed &amp;&amp; u.State != &quot;DELETING&quot;</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_RESOURCE_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!committed &amp;&amp; (u.Document != document &#124;&#124; (u.State != &quot;UPLOADING&quot; &amp;&amp; u.State != &quot;DELETING&quot;))</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_UPLOAD_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!committed &amp;&amp; u.State == &quot;UPLOADING&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_RESOURCE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>committed</code>。条件成立时执行 <code>return s.cleanupReady(ctx, u)</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=1548:2973 -->
````go
		return err
	}
	if shared {
		if u.State != "READY" {
			return domain.Violation(domain.StateConflict, "RETAINED_SHARED_RESOURCE_NOT_READY")
		}
		return nil
	}
	committed, err := repository.ReadyCleanupCommitted(ctx, tx, u)
	if err != nil {
		return err
	}
	if !committed {
		reservation, err := repository.LockedReservationState(ctx, tx, u.Operation)
		if err != nil {
			return err
		}
		if reservation != "RESERVED" {
			return errors.New("RETAINED_RESOURCE_RESERVATION_PENDING")
		}
	}
	if committed && u.State == "READY" {
		changed, err := repository.MarkRetainedReadyDeleting(ctx, tx, u)
		if err != nil {
			return err
		}
		if !changed {
			return domain.Violation(domain.StateConflict, "RETAINED_RESOURCE_CLEANUP_PENDING")
		}
	} else if committed && u.State != "DELETING" {
		return errors.New("RETAINED_RESOURCE_STATE_CHANGED")
	} else if !committed && (u.Document != document || (u.State != "UPLOADING" && u.State != "DELETING")) {
		return errors.New("RETAINED_UPLOAD_BINDING_CHANGED")
	}
	if !committed && u.State == "UPLOADING" {
		if err = repository.MarkRetainedUploadDeleting(ctx, tx, u); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return errors.New("RETAINED_RESOURCE_COMMIT_UNKNOWN")
	}
	if committed {
		// 状态已持久DELETING；cleanupReady原号恢复实际删除及used扣减，
		// 不再走普通READY候选的一小时宽限，不重复释放temporary预算。
````

## 第 4 步：Resources.CleanRetainedResource（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 100 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return s.cleanupReady(ctx, u)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return s.cleanupUpload(ctx, u)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2973:3042 -->
````go
		return s.cleanupReady(ctx, u)
	}
	return s.cleanupUpload(ctx, u)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/retained_governance_resources.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
