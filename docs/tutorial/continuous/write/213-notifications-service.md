# 分步编写 `services/document/internal/logic/document/notifications_service.go`

先按 [D09 · 固定批注、提及与可靠通知](../lessons/document/09-comments-notifications.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/213-notifications-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

处理审核/离组收件快照、通知列表过滤、固定轮次跳转与已读。

投递候选固定保证恢复不混入新人；展示时再次鉴权避免通知保留失权内容。

写之前先按这条执行路线理解各段的连接：

1. prepareReviewRecipients冻结完整候选并分页推进
2. deliverDeparture复核当前KB管理员
3. List验证原目标
4. ReviewRoundTarget返回原轮次
5. MarkRead幂等标记。

本文件需要守住的边界：

- 公开target_id缺轮次时不能用当前轮次补齐
- 已关闭KB只终结任务不制造无入口通知。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/notifications_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/notifications_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/notifications_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/notifications_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:673 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	comments "collaboration/document/internal/repository/comment"
	documents "collaboration/document/internal/repository/document"
	notices "collaboration/document/internal/repository/notification"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

// notification_delivery_access.go：保留原实现，按业务合并文件。

````

## 第 2 步：notificationDeliveryAccess

插入位置：第 1 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`type notificationDeliveryAccess struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>doc</code> | <code>dto.Document</code> | 投递前核验的文档元数据。 |
| <code>space</code> | <code>dto.Space</code> | 文档所属空间事实。 |
| <code>proof</code> | <code>platform.NotificationRecipientsResponse</code> | 使用 <code>platform.NotificationRecipientsResponse</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。 |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 这次通知实际交付依赖的全部权限来源。 |
| <code>kbRevision</code> | <code>uint64</code> | 投递核验时的知识库ACL修订。 |

<!-- write-step: 2 bytes=673:865 -->
````go
type notificationDeliveryAccess struct {
	doc        dto.Document
	space      dto.Space
	proof      platform.NotificationRecipientsResponse
	sources    []*pb.SourceEpoch
	kbRevision uint64
}

````

## 第 3 步：NotificationDelivery.deliveryAccess

插入位置：第 2 步代码之后，紧接追加；完成稿约第 33 行。

跨服务查询统一放在SQL锁外；每种通知只负责自己的业务绑定和收件范围。
返回的来源必须在最终业务事务持锁，短期身份结果不能直接当成投递许可。

本步声明：`func (s *NotificationDelivery) deliveryAccess(ctx context.Context, document uint64, ids []uint64) (notificationDeliveryAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>documents.DocumentRow(ctx, s.db, document, false)</code>；<code>documents.SpaceRow(ctx, s.db, spaceID, false)</code>；<code>s.recipients.Resolve(ctx, organization, ids)</code>；<code>s.comments.access.repository.KnowledgeBase(ctx, kb, spaceID)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>documents.DocumentRow(ctx, s.db, document, false)</code> |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(ids)</code>；<code>s.recipients.Resolve(ctx, organization, ids)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>notificationDeliveryAccess</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;KNOWLEDGE_BASE_ACCESS_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>value.space.OrganizationId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>len(ids) &gt; 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>value.doc.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return value, domain.Violation(domain.StateConflict, &quot;KNOWLEDGE_BASE_ACCESS_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=865:2551 -->
````go
// 跨服务查询统一放在SQL锁外；每种通知只负责自己的业务绑定和收件范围。
// 返回的来源必须在最终业务事务持锁，短期身份结果不能直接当成投递许可。
func (s *NotificationDelivery) deliveryAccess(ctx context.Context, document uint64, ids []uint64) (notificationDeliveryAccess, error) {
	var value notificationDeliveryAccess
	var err error
	value.doc, err = documents.DocumentRow(ctx, s.db, document, false)
	if err != nil {
		return value, err
	}
	spaceID, err := domain.CanonicalID(value.doc.SpaceId)
	if err != nil {
		return value, err
	}
	value.space, err = documents.SpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return value, err
	}
	var organization uint64
	if value.space.OrganizationId != nil {
		organization, err = domain.CanonicalID(*value.space.OrganizationId)
		if err != nil {
			return value, err
		}
	}
	value.proof = platform.NotificationRecipientsResponse{ExpiresUnixMS: time.Now().Add(5 * time.Second).UnixMilli()}
	if len(ids) > 0 {
		value.proof, err = s.recipients.Resolve(ctx, organization, ids)
		if err != nil {
			return value, err
		}
	}
	acl, err := domain.CanonicalID(value.doc.AclRevision)
	if err != nil {
		return value, err
	}
	value.sources = []*pb.SourceEpoch{{ScopeKey: "doc-acl:" + value.doc.Id, Epoch: acl}}
	if value.doc.KnowledgeBaseId != nil {
		kb, _ := domain.CanonicalID(*value.doc.KnowledgeBaseId)
		var state string
		state, value.kbRevision, err = s.comments.access.repository.KnowledgeBase(ctx, kb, spaceID)
		if err != nil {
			return value, err
		}
		if state != "ACTIVE" {
			return value, domain.Violation(domain.StateConflict, "KNOWLEDGE_BASE_ACCESS_CHANGED")
		}
````

## 第 4 步：NotificationDelivery.deliveryAccess（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 79 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>recipient.Eligible</code>。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2551:3035 -->
````go
		value.sources = append(value.sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *value.doc.KnowledgeBaseId, Epoch: value.kbRevision})
	}
	for _, recipient := range value.proof.Recipients {
		if recipient.Eligible {
			for _, source := range recipient.Sources {
				value.sources = append(value.sources, &pb.SourceEpoch{ScopeKey: source.ScopeKey, Epoch: source.Epoch})
			}
		}
	}
	return value, nil
}

// notification_departure_delivery.go：保留原实现，按业务合并文件。

````

## 第 5 步：NotificationDelivery.deliverDeparture

插入位置：第 4 步代码之后，紧接追加；完成稿约第 93 行。

收件集合来自负责人清空时的真实知识库管理员快照，投递时再核验当前身份和KB角色。
不把组织ADMIN、负责人或一般COMMENT成员扩充为知识库管理员。

本步声明：`func (s *NotificationDelivery) deliverDeparture(ctx context.Context, task notices.Task) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>notices.DepartureBinding(ctx, s.db, task)</code>；<code>s.deliveryAccess(ctx, event.Document, ids)</code>；<code>s.finishClosedDeparture(ctx, task, event)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>notices.Task</code> | 使用 <code>notices.Task</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>在此声明中的实际使用：<code>notices.DepartureBinding(ctx, s.db, task)</code>；<code>s.finishClosedDeparture(ctx, task, event)</code>；<code>task.ID</code>；<code>notificationTaskUnchanged(locked, task)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>finishErr</code>；<code>nil</code>；<code>errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_EXPIRED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return event.Recipients&#91;i&#93; &gt; event.DeliveryAfter</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>end &gt; len(event.Recipients)</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>finishErr != nil</code>。条件成立时执行 <code>return finishErr</code>。当前函数在这里结束。
- 检查 <code>finished</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>locked.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=3035:4554 -->
````go
// 收件集合来自负责人清空时的真实知识库管理员快照，投递时再核验当前身份和KB角色。
// 不把组织ADMIN、负责人或一般COMMENT成员扩充为知识库管理员。
func (s *NotificationDelivery) deliverDeparture(ctx context.Context, task notices.Task) error {
	event, err := notices.DepartureBinding(ctx, s.db, task)
	if err != nil {
		return err
	}
	start := sort.Search(len(event.Recipients), func(i int) bool { return event.Recipients[i] > event.DeliveryAfter })
	end := start + 100
	if end > len(event.Recipients) {
		end = len(event.Recipients)
	}
	ids := event.Recipients[start:end]
	resolved, err := s.deliveryAccess(ctx, event.Document, ids)
	if err != nil {
		// 关闭中的库仍等待排空；只有持锁确认CLOSED，才可把失去目标的提醒结束。
		// 资格查询超时、缺行或其他未知错误不能被当成“无需投递”。
		finished, finishErr := s.finishClosedDeparture(ctx, task, event)
		if finishErr != nil {
			return finishErr
		}
		if finished {
			return nil
		}
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, resolved.sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, DocumentAccess{Document: resolved.doc, KBRevision: resolved.kbRevision})
	if err != nil {
		return err
	}
	locked, err := notices.LockedTask(ctx, tx, task.ID)
	if err != nil {
		return err
	}
	if locked.State == "SUCCEEDED" {
		return nil
	}
````

## 第 6 步：NotificationDelivery.deliverDeparture（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 138 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>resolved.proof.ExpiresUnixMS &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>eligibleContext</code>。
- 检查 <code>!recipient.Eligible &#124;&#124; recipient.UserID == event.Departed</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>role != &quot;ADMIN&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>resolved.proof.ExpiresUnixMS &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>end &lt; len(event.Recipients)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=4554:6212 -->
````go
	if err = notificationTaskUnchanged(locked, task); err != nil {
		return err
	}
	if event, err = notices.DepartureBinding(ctx, tx, locked); err != nil {
		return err
	}
	if resolved.proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	eligibleContext := current.Lifecycle == "ACTIVE" && current.KnowledgeBaseId != nil && *current.KnowledgeBaseId == domain.TextID(event.KnowledgeBase) && resolved.space.OrganizationId != nil && *resolved.space.OrganizationId == domain.TextID(event.Organization)
	if eligibleContext {
		for _, recipient := range resolved.proof.Recipients {
			if !recipient.Eligible || recipient.UserID == event.Departed {
				continue
			}
			role, err := documents.KnowledgeBaseRole(ctx, tx, event.KnowledgeBase, recipient.UserID)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if role != "ADMIN" {
				continue
			}
			target := notices.Target{Document: current.Id, KnowledgeBase: domain.TextID(event.KnowledgeBase)}
			if err = notices.Insert(ctx, tx, recipient.UserID, event.Event, "RESPONSIBLE_DEPARTURE", target); err != nil {
				return err
			}
		}
	}
	if resolved.proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	// 通知与批次位置在同一事务提交；未知提交仅重新读取原任务，不改收件集合。
	if end < len(event.Recipients) {
		event.DeliveryAfter = ids[len(ids)-1]
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if err = notices.AdvanceTask(ctx, tx, task, raw); err != nil {
			return err
		}
	} else {
````

## 第 7 步：NotificationDelivery.deliverDeparture（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 183 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!done</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_TASK_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 7 bytes=6212:6390 -->
````go
		done, err := notices.FinishTask(ctx, tx, task)
		if err != nil {
			return err
		}
		if !done {
			return errors.New("NOTIFICATION_TASK_CHANGED")
		}
	}
	return tx.Commit()
}

````

## 第 8 步：NotificationDelivery.finishClosedDeparture

插入位置：第 7 步代码之后，紧接追加；完成稿约第 194 行。

已关闭知识库没有可进入的管理目标。验证原回执后终结任务，不制造通知，
不删除回执或资源；原知识库锁阻止生命周期事实在判断和提交之间改变。

本步声明：`func (s *NotificationDelivery) finishClosedDeparture(ctx context.Context, task notices.Task, event domain.DepartureNotification) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>documents.LockedKnowledgeBase(ctx, tx, domain.TextID(event.KnowledgeBase))</code>；<code>notices.LockedTask(ctx, tx, task.ID)</code>；<code>notices.DepartureBinding(ctx, tx, locked)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>notices.Task</code> | 使用 <code>notices.Task</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>在此声明中的实际使用：<code>task.ID</code>；<code>notificationTaskUnchanged(locked, task)</code>；<code>notices.FinishTask(ctx, tx, task)</code> |
| <code>event</code> | <code>domain.DepartureNotification</code> | 使用 <code>domain.DepartureNotification</code> 类型；其定义处的职责：DepartureNotification 固定离组时的文档范围和真实知识库管理员；后续补任负责人、 成员重新入组或者门禁代次推进，都不能改写这次历史事件的收件集合。<br>在此声明中的实际使用：<code>event.KnowledgeBase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>errors.New(&quot;NOTIFICATION_TASK_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil &#124;&#124; state != &quot;CLOSED&quot;</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>locked.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return true, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>!done</code>。条件成立时执行 <code>return false, errors.New(&quot;NOTIFICATION_TASK_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=6390:7642 -->
````go
// 已关闭知识库没有可进入的管理目标。验证原回执后终结任务，不制造通知，
// 不删除回执或资源；原知识库锁阻止生命周期事实在判断和提交之间改变。
func (s *NotificationDelivery) finishClosedDeparture(ctx context.Context, task notices.Task, event domain.DepartureNotification) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	state, _, err := documents.LockedKnowledgeBase(ctx, tx, domain.TextID(event.KnowledgeBase))
	if err != nil || state != "CLOSED" {
		return false, err
	}
	locked, err := notices.LockedTask(ctx, tx, task.ID)
	if err != nil {
		return false, err
	}
	if locked.State == "SUCCEEDED" {
		return true, nil
	}
	if err = notificationTaskUnchanged(locked, task); err != nil {
		return false, err
	}
	if _, err = notices.DepartureBinding(ctx, tx, locked); err != nil {
		return false, err
	}
	done, err := notices.FinishTask(ctx, tx, task)
	if err != nil {
		return false, err
	}
	if !done {
		return false, errors.New("NOTIFICATION_TASK_CHANGED")
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// notification_review_delivery.go：保留原实现，按业务合并文件。

````

## 第 9 步：notificationTaskUnchanged

插入位置：第 8 步代码之后，紧接追加；完成稿约第 234 行。

本步声明：`func notificationTaskUnchanged(current, original notices.Task) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>current</code> | <code>notices.Task</code> | 使用 <code>notices.Task</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>在此声明中的实际使用：<code>current.Kind</code>；<code>current.Operation</code>；<code>current.Generation</code>；<code>current.State</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>original</code> | <code>notices.Task</code> | 使用 <code>notices.Task</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>在此声明中的实际使用：<code>original.Kind</code>；<code>original.Operation</code>；<code>original.Generation</code>；<code>original.State</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;NOTIFICATION_TASK_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>current.Kind != original.Kind &#124;&#124; current.Operation != original.Operation &#124;&#124; current.Generation != original.Generation &#124;&#124; current.State != original.State &#124;&#124; !bytes.Equal(current.Payload, original.Payload)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;NOTIFICATION_TASK_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=7642:8018 -->
````go
func notificationTaskUnchanged(current, original notices.Task) error {
	if current.Kind != original.Kind || current.Operation != original.Operation || current.Generation != original.Generation || current.State != original.State || !bytes.Equal(current.Payload, original.Payload) {
		return domain.Violation(domain.StateConflict, "NOTIFICATION_TASK_CHANGED")
	}
	return nil
}

````

## 第 10 步：orderedRecipients

插入位置：第 9 步代码之后，紧接追加；完成稿约第 241 行。

本步声明：`func orderedRecipients(groups ...[]uint64) []uint64`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>groups</code> | <code>...&#91;&#93;uint64</code> | 来自作者、审核参与者、提及等来源的接收者列表；合并去重并确定稳定顺序，避免重复投递。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>ids</code> |

**跟着本段代码走一遍：**

- 检查 <code>id != 0</code>。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return ids</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=8018:8376 -->
````go
func orderedRecipients(groups ...[]uint64) []uint64 {
	seen := map[uint64]bool{}
	for _, group := range groups {
		for _, id := range group {
			if id != 0 {
				seen[id] = true
			}
		}
	}
	ids := make([]uint64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

````

## 第 11 步：NotificationDelivery.prepareReviewRecipients

插入位置：第 10 步代码之后，紧接追加；完成稿约第 258 行。

首次恢复时冻结完整候选集，原业务JSON字段仍保留，既有恢复绑定检查继续生效。
Account目录只供候选枚举，后续每批均取得新身份事实并检查当前Doc资格。

本步声明：`func (s *NotificationDelivery) prepareReviewRecipients(ctx context.Context, task notices.Task, event notices.ReviewEvent) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>documents.DocumentRow(ctx, s.db, event.Document, false)</code>；<code>documents.SpaceRow(ctx, s.db, spaceID, false)</code>；<code>s.recipients.Administrators(ctx, organization, after)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>notices.Task</code> | 使用 <code>notices.Task</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>在此声明中的实际使用：<code>task.ID</code>；<code>notificationTaskUnchanged(current, task)</code>；<code>notices.ReviewBinding(ctx, tx, task)</code>；<code>commitReviewProgress(ctx, tx, task, event.Payload)</code> |
| <code>event</code> | <code>notices.ReviewEvent</code> | 使用 <code>notices.ReviewEvent</code> 类型；其定义处的职责：ReviewEvent来自成功业务事实；任务JSON只用于核对绑定，不能凭任务自己声称成功。<br>在此声明中的实际使用：<code>event.Document</code>；<code>event, err = notices.ReviewBinding(ctx, tx, task)</code>；<code>notices.ReviewCandidates(ctx, tx, event)</code>；<code>event.Actor</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;NOTIFICATION_DIRECTORY_CURSOR_INVALID&quot;)</code>；<code>nil</code>；<code>commitReviewProgress(ctx, tx, task, event.Payload)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>space.OrganizationId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>page.NextAfter == 0</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>page.NextAfter &lt;= after</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_DIRECTORY_CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=8376:9696 -->
````go
// 首次恢复时冻结完整候选集，原业务JSON字段仍保留，既有恢复绑定检查继续生效。
// Account目录只供候选枚举，后续每批均取得新身份事实并检查当前Doc资格。
func (s *NotificationDelivery) prepareReviewRecipients(ctx context.Context, task notices.Task, event notices.ReviewEvent) error {
	doc, err := documents.DocumentRow(ctx, s.db, event.Document, false)
	if err != nil {
		return err
	}
	spaceID, err := domain.CanonicalID(doc.SpaceId)
	if err != nil {
		return err
	}
	space, err := documents.SpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return err
	}
	admins := []uint64{}
	if space.OrganizationId != nil {
		organization, err := domain.CanonicalID(*space.OrganizationId)
		if err != nil {
			return err
		}
		var after uint64
		for {
			page, err := s.recipients.Administrators(ctx, organization, after)
			if err != nil {
				return err
			}
			admins = append(admins, page.UserIDs...)
			if page.NextAfter == 0 {
				break
			}
			if page.NextAfter <= after {
				return errors.New("NOTIFICATION_DIRECTORY_CURSOR_INVALID")
			}
			after = page.NextAfter
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = documents.DocumentRow(ctx, tx, event.Document, true); err != nil {
		return err
	}
````

## 第 12 步：NotificationDelivery.prepareReviewRecipients（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 303 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>id != event.Actor</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return commitReviewProgress(ctx, tx, task, event.Payload)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=9696:10615 -->
````go
	current, err := notices.LockedTask(ctx, tx, task.ID)
	if err != nil {
		return err
	}
	if current.State == "SUCCEEDED" {
		return nil
	}
	if err = notificationTaskUnchanged(current, task); err != nil {
		return err
	}
	if event, err = notices.ReviewBinding(ctx, tx, task); err != nil {
		return err
	}
	ids, participants, err := notices.ReviewCandidates(ctx, tx, event)
	if err != nil {
		return err
	}
	all := orderedRecipients(ids, admins, participants)
	filtered := all[:0]
	for _, id := range all {
		if id != event.Actor {
			filtered = append(filtered, id)
		}
	}
	raw, err := json.Marshal(filtered)
	if err != nil {
		return err
	}
	event.Payload["delivery_recipients"] = string(raw)
	raw, err = json.Marshal(participants)
	if err != nil {
		return err
	}
	event.Payload["delivery_participants"] = string(raw)
	event.Payload["delivery_after"] = "0"
	return commitReviewProgress(ctx, tx, task, event.Payload)
}

````

## 第 13 步：decodeReviewRecipients

插入位置：第 12 步代码之后，紧接追加；完成稿约第 341 行。

本步声明：`func decodeReviewRecipients(event notices.ReviewEvent) ([]uint64, map[uint64]bool, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>event</code> | <code>notices.ReviewEvent</code> | 使用 <code>notices.ReviewEvent</code> 类型；其定义处的职责：ReviewEvent来自成功业务事实；任务JSON只用于核对绑定，不能凭任务自己声称成功。<br>在此声明中的实际使用：<code>event.Payload</code>；<code>event.Actor</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>map&#91;uint64&#93;bool</code> | 实际返回表达式：<code>nil</code>；<code>participantsSet</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>after</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;NOTIFICATION_RECIPIENT_SNAPSHOT_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;NOTIFICATION_RECIPIENT_CURSOR_INVALID&quot;)</code>；<code>errors.New(&quot;NOTIFICATION_PARTICIPANT_SNAPSHOT_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(&#91;&#93;byte(event.Payload&#91;&quot;delivery_recipients&quot;&#93;), &amp;ids) != nil &#124;&#124; ids == nil &#124;&#124; json.Unmarshal(&#91;&#93;byte(event.Payload&#91;&quot;delivery_participants&quot;&#93;), &amp;participants) != nil &#124;&#124; participants == nil</code>。条件成立时执行 <code>return nil, nil, 0, errors.New(&quot;NOTIFICATION_RECIPIENT_SNAPSHOT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>text != &quot;0&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, 0, err</code>。当前函数在这里结束。
- 检查 <code>id &lt;= previous &#124;&#124; id == event.Actor</code>。条件成立时执行 <code>return nil, nil, 0, errors.New(&quot;NOTIFICATION_RECIPIENT_SNAPSHOT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>after != 0 &amp;&amp; !seen&#91;after&#93;</code>。条件成立时执行 <code>return nil, nil, 0, errors.New(&quot;NOTIFICATION_RECIPIENT_CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!seen&#91;id&#93; &#124;&#124; participantsSet&#91;id&#93;</code>。条件成立时执行 <code>return nil, nil, 0, errors.New(&quot;NOTIFICATION_PARTICIPANT_SNAPSHOT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return ids, participantsSet, after, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=10615:11833 -->
````go
func decodeReviewRecipients(event notices.ReviewEvent) ([]uint64, map[uint64]bool, uint64, error) {
	var ids, participants []uint64
	if json.Unmarshal([]byte(event.Payload["delivery_recipients"]), &ids) != nil || ids == nil || json.Unmarshal([]byte(event.Payload["delivery_participants"]), &participants) != nil || participants == nil {
		return nil, nil, 0, errors.New("NOTIFICATION_RECIPIENT_SNAPSHOT_INVALID")
	}
	var after uint64
	if text := event.Payload["delivery_after"]; text != "0" {
		var err error
		after, err = domain.CanonicalID(text)
		if err != nil {
			return nil, nil, 0, err
		}
	}
	seen := map[uint64]bool{}
	var previous uint64
	for _, id := range ids {
		if id <= previous || id == event.Actor {
			return nil, nil, 0, errors.New("NOTIFICATION_RECIPIENT_SNAPSHOT_INVALID")
		}
		seen[id] = true
		previous = id
	}
	if after != 0 && !seen[after] {
		return nil, nil, 0, errors.New("NOTIFICATION_RECIPIENT_CURSOR_INVALID")
	}
	participantsSet := map[uint64]bool{}
	for _, id := range participants {
		if !seen[id] || participantsSet[id] {
			return nil, nil, 0, errors.New("NOTIFICATION_PARTICIPANT_SNAPSHOT_INVALID")
		}
		participantsSet[id] = true
	}
	return ids, participantsSet, after, nil
}

````

## 第 14 步：commitReviewProgress

插入位置：第 13 步代码之后，紧接追加；完成稿约第 376 行。

本步声明：`func commitReviewProgress(ctx context.Context, tx *sql.Tx, task notices.Task, payload map[string]string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>notices.AdvanceTask(ctx, tx, task, raw)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>notices.AdvanceTask(ctx, tx, task, raw)</code>；<code>tx.Commit</code> |
| <code>task</code> | <code>notices.Task</code> | 使用 <code>notices.Task</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>在此声明中的实际使用：<code>notices.AdvanceTask(ctx, tx, task, raw)</code> |
| <code>payload</code> | <code>map&#91;string&#93;string</code> | 在此声明中的实际使用：<code>json.Marshal(payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 14 bytes=11833:12117 -->
````go
func commitReviewProgress(ctx context.Context, tx *sql.Tx, task notices.Task, payload map[string]string) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err = notices.AdvanceTask(ctx, tx, task, raw); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 15 步：NotificationDelivery.deliverReview

插入位置：第 14 步代码之后，紧接追加；完成稿约第 387 行。

本步声明：`func (s *NotificationDelivery) deliverReview(ctx context.Context, task notices.Task) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>notices.ReviewBinding(ctx, s.db, task)</code>；<code>s.prepareReviewRecipients(ctx, task, event)</code>；<code>s.deliveryAccess(ctx, event.Document, ids)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>notices.Task</code> | 使用 <code>notices.Task</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>在此声明中的实际使用：<code>notices.ReviewBinding(ctx, s.db, task)</code>；<code>s.prepareReviewRecipients(ctx, task, event)</code>；<code>task.ID</code>；<code>notificationTaskUnchanged(locked, task)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>s.prepareReviewRecipients(ctx, task, event)</code>；<code>nil</code>；<code>errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_EXPIRED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!ready</code>。条件成立时执行 <code>return s.prepareReviewRecipients(ctx, task, event)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return all&#91;i&#93; &gt; after</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>end &gt; len(all)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>locked.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=12117:13366 -->
````go
func (s *NotificationDelivery) deliverReview(ctx context.Context, task notices.Task) error {
	event, err := notices.ReviewBinding(ctx, s.db, task)
	if err != nil {
		return err
	}
	if _, ready := event.Payload["delivery_recipients"]; !ready {
		return s.prepareReviewRecipients(ctx, task, event)
	}
	all, participants, after, err := decodeReviewRecipients(event)
	if err != nil {
		return err
	}
	start := sort.Search(len(all), func(i int) bool { return all[i] > after })
	end := start + 100
	if end > len(all) {
		end = len(all)
	}
	ids := all[start:end]
	resolved, err := s.deliveryAccess(ctx, event.Document, ids)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, resolved.sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, DocumentAccess{Document: resolved.doc, KBRevision: resolved.kbRevision})
	if err != nil {
		return err
	}
	locked, err := notices.LockedTask(ctx, tx, task.ID)
	if err != nil {
		return err
	}
	if locked.State == "SUCCEEDED" {
		return nil
	}
	if err = notificationTaskUnchanged(locked, task); err != nil {
		return err
	}
	if event, err = notices.ReviewBinding(ctx, tx, task); err != nil {
````

## 第 16 步：NotificationDelivery.deliverReview（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 432 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>resolved.proof.ExpiresUnixMS &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.Lifecycle == &quot;ACTIVE&quot;</code>。
- 检查 <code>!recipient.Eligible</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Read &#124;&#124; (!access.Permissions.Review &amp;&amp; !access.Permissions.Manage &amp;&amp; !participants&#91;recipient.UserID&#93;)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。
- 检查 <code>notificationHidden(err)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>resolved.proof.ExpiresUnixMS &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>end &lt; len(all)</code>。
- 此处通过 <code>return commitReviewProgress(ctx, tx, task, event.Payload)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 16 bytes=13366:15090 -->
````go
		return err
	}
	pointer, err := documents.ReviewPointerRow(ctx, tx, event.Request)
	if err != nil {
		return err
	}
	if resolved.proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	if current.Lifecycle == "ACTIVE" {
		for _, recipient := range resolved.proof.Recipients {
			if !recipient.Eligible {
				continue
			}
			facts, err := notices.Facts(ctx, tx, current, resolved.space, recipient.UserID, recipient.OrgOwner || recipient.Role == "ADMIN")
			if err != nil {
				return err
			}
			access := DocumentAccess{Document: current, Permissions: domain.ResolvePermissions(facts), KBRevision: resolved.kbRevision}
			// 提交只提醒当前审核/管理者；动作及合并额外提醒原参与者，但参与不授予读取权。
			if !access.Permissions.Read || (!access.Permissions.Review && !access.Permissions.Manage && !participants[recipient.UserID]) {
				continue
			}
			if err = s.comments.reviews.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
				if notificationHidden(err) {
					continue
				}
				return err
			}
			target := notices.Target{Document: current.Id, Request: domain.TextID(event.Request), Round: domain.TextID(event.Round)}
			if err = notices.Insert(ctx, tx, recipient.UserID, task.Operation, event.Kind, target); err != nil {
				return err
			}
		}
	}
	if resolved.proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	if end < len(all) {
		event.Payload["delivery_after"] = domain.TextID(ids[len(ids)-1])
		return commitReviewProgress(ctx, tx, task, event.Payload)
	}
	done, err := notices.FinishTask(ctx, tx, task)
	if err != nil {
		return err
	}
````

## 第 17 步：NotificationDelivery.deliverReview（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 478 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!done</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_TASK_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 17 bytes=15090:15256 -->
````go
	if !done {
		return errors.New("NOTIFICATION_TASK_CHANGED")
	}
	return tx.Commit()
}

// notification_review_target.go：保留原实现，按业务合并文件。

````

## 第 18 步：Notifications.LookupReviewTarget

插入位置：第 17 步代码之后，紧接追加；完成稿约第 486 行。

先按真实收件人读取原目标，供handler绑定文档身份准入；公开target_id不含轮次，不能拿当前轮次补齐。

本步声明：`func (s *Notifications) LookupReviewTarget(ctx context.Context, id, recipient uint64) (notices.Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>notices.Row(ctx, s.db, id, recipient, false)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>notices.Row(ctx, s.db, id, recipient, false)</code> |
| <code>recipient</code> | <code>uint64</code> | 在此声明中的实际使用：<code>notices.Row(ctx, s.db, id, recipient, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>notices.Entry</code> | 使用 <code>notices.Entry</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>实际返回表达式：<code>n</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_NOT_FOUND&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_REVIEW_TARGET_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return n, domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return n, err</code>。当前函数在这里结束。
- 此处通过 <code>return n, domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_REVIEW_TARGET_NOT_FOUND&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>n.Target.Thread != &quot;&quot;</code>。条件成立时执行 <code>return n, domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_REVIEW_TARGET_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return n, domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_REVIEW_TARGET_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return n, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=15256:16303 -->
````go
// 先按真实收件人读取原目标，供handler绑定文档身份准入；公开target_id不含轮次，不能拿当前轮次补齐。
func (s *Notifications) LookupReviewTarget(ctx context.Context, id, recipient uint64) (notices.Entry, error) {
	n, err := notices.Row(ctx, s.db, id, recipient, false)
	if errors.Is(err, sql.ErrNoRows) {
		return n, domain.Violation(domain.EntityNotFound, "NOTIFICATION_NOT_FOUND")
	}
	if err != nil {
		return n, err
	}
	switch n.Kind {
	case "REVIEW_SUBMITTED", "REVIEW_RESUBMITTED", "REVIEW_ACTION", "REVIEW_MERGED":
	default:
		return n, domain.Violation(domain.EntityNotFound, "NOTIFICATION_REVIEW_TARGET_NOT_FOUND")
	}
	if n.Target.Thread != "" {
		return n, domain.Violation(domain.EntityNotFound, "NOTIFICATION_REVIEW_TARGET_NOT_FOUND")
	}
	for _, value := range []string{n.Target.Document, n.Target.Request, n.Target.Round} {
		if _, err := domain.CanonicalID(value); err != nil {
			return n, domain.Violation(domain.EntityNotFound, "NOTIFICATION_REVIEW_TARGET_NOT_FOUND")
		}
	}
	return n, nil
}

````

## 第 19 步：Notifications.ReviewRoundTarget

插入位置：第 18 步代码之后，紧接追加；完成稿约第 511 行。

本步声明：`func (s *Notifications) ReviewRoundTarget(ctx context.Context, actor Actor, n notices.Entry) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;NOTIFICATION_REVIEW_ROUND&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.access.Authorize(ctx, actor, document, &quot;NOTIFICATION_REVIEW_ROUND&quot;)</code>；<code>s.validateTarget(ctx, tx, actor, access, n)</code> |
| <code>n</code> | <code>notices.Entry</code> | 使用 <code>notices.Entry</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>在此声明中的实际使用：<code>n.Recipient</code>；<code>n.Target</code>；<code>s.validateTarget(ctx, tx, actor, access, n)</code>；<code>n.ID</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_NOT_FOUND&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>n.Recipient != actor.ID</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Target != n.Target &#124;&#124; current.Kind != n.Kind &#124;&#124; current.Event != n.Event</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;NOTIFICATION_TARGET_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_REVIEW_TARGET_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return fixed, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=16303:18010 -->
````go
func (s *Notifications) ReviewRoundTarget(ctx context.Context, actor Actor, n notices.Entry) ([]*pb.SourceEpoch, ReadBody, error) {
	if n.Recipient != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "NOTIFICATION_NOT_FOUND")
	}
	document, err := domain.CanonicalID(n.Target.Document)
	if err != nil {
		return nil, nil, err
	}
	request, err := domain.CanonicalID(n.Target.Request)
	if err != nil {
		return nil, nil, err
	}
	round, err := domain.CanonicalID(n.Target.Round)
	if err != nil {
		return nil, nil, err
	}
	access, err := s.access.Authorize(ctx, actor, document, "NOTIFICATION_REVIEW_ROUND")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 最终输出事务先锁定并复核文档/审核资格，再重读原通知；通知本身不替代当前读取资格。
		if err := s.validateTarget(ctx, tx, actor, access, n); err != nil {
			return nil, err
		}
		current, err := notices.Row(ctx, tx, n.ID, actor.ID, false)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.Violation(domain.EntityNotFound, "NOTIFICATION_NOT_FOUND")
		}
		if err != nil {
			return nil, err
		}
		if current.Target != n.Target || current.Kind != n.Kind || current.Event != n.Event {
			return nil, domain.Violation(domain.StateConflict, "NOTIFICATION_TARGET_CHANGED")
		}
		fixed, err := documents.ReviewRoundRow(ctx, tx, request, round, n.Target.Document)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.Violation(domain.EntityNotFound, "NOTIFICATION_REVIEW_TARGET_NOT_FOUND")
		}
		return fixed, err
	}
	return access.Sources, run, nil
}

// notifications.go：保留原实现，按业务合并文件。

````

## 第 20 步：Notifications

插入位置：第 19 步代码之后，紧接追加；完成稿约第 557 行。

本步声明：`type Notifications struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>comments</code> | <code>*Comments</code> | 使用 <code>Comments</code> 类型；其定义处的职责：执行固定上下文批注的创建、回复、编辑、删除、状态与分页读取。 |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。 |
| <code>recipients</code> | <code>NotificationRecipientResolver</code> | 使用 <code>NotificationRecipientResolver</code> 类型；其定义处的职责：Doc不跨库读账号成员关系，通过Account返回权威接收人事实再核验内容可见性。 |

<!-- write-step: 20 bytes=18010:18169 -->
````go
type Notifications struct {
	db         *sql.DB
	access     *Access
	comments   *Comments
	pagination *Pagination
	recipients NotificationRecipientResolver
}

````

## 第 21 步：NewNotifications

插入位置：第 20 步代码之后，紧接追加；完成稿约第 565 行。

本步声明：`func NewNotifications(db *sql.DB, access *Access, comments *Comments, pagination *Pagination, recipients NotificationRecipientResolver) *Notifications`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>comments</code> | <code>*Comments</code> | 使用 <code>Comments</code> 类型；其定义处的职责：执行固定上下文批注的创建、回复、编辑、删除、状态与分页读取。<br>在此声明中的实际使用：<code>comments: comments</code> |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>pagination: pagination</code> |
| <code>recipients</code> | <code>NotificationRecipientResolver</code> | 使用 <code>NotificationRecipientResolver</code> 类型；其定义处的职责：Doc不跨库读账号成员关系，通过Account返回权威接收人事实再核验内容可见性。<br>在此声明中的实际使用：<code>recipients: recipients</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Notifications</code> | 实际返回表达式：<code>&amp;Notifications{db: db, access: access, comments: comments, pagination: pagination, recipients: recipients}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Notifications{db: db, access: access, comments: comments, pagination: pagination, recipients: recipients}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=18169:18440 -->
````go
func NewNotifications(db *sql.DB, access *Access, comments *Comments, pagination *Pagination, recipients NotificationRecipientResolver) *Notifications {
	return &Notifications{db: db, access: access, comments: comments, pagination: pagination, recipients: recipients}
}

````

## 第 22 步：notificationHidden

插入位置：第 21 步代码之后，紧接追加；完成稿约第 569 行。

本步声明：`func notificationHidden(err error) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.Is(err, sql.ErrNoRows)</code>；<code>errors.As(err, &amp;rule)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>errors.Is(err, sql.ErrNoRows) &#124;&#124; (errors.As(err, &amp;rule) &amp;&amp; (rule.Kind == domain.PermissionDenied &#124;&#124; rule.Kind == domain.EntityNotFound))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.Is(err, sql.ErrNoRows) &#124;&#124; (errors.As(err, &amp;rule) &amp;&amp; (rule.Kind == domain.PermissionDenied &#124;&#124; rule.Kind == domain.EntityNotFound))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=18440:18661 -->
````go
func notificationHidden(err error) bool {
	var rule domain.RuleViolation
	return errors.Is(err, sql.ErrNoRows) || (errors.As(err, &rule) && (rule.Kind == domain.PermissionDenied || rule.Kind == domain.EntityNotFound))
}

````

## 第 23 步：notificationValue

插入位置：第 22 步代码之后，紧接追加；完成稿约第 574 行。

通知显示通用事件名；文档标题和正文仅在跳转目标的授权接口中读取。

本步声明：`func notificationValue(n notices.Entry) (dto.Notification, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>n</code> | <code>notices.Entry</code> | 使用 <code>notices.Entry</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>在此声明中的实际使用：<code>n.ID</code>；<code>n.Event</code>；<code>n.Kind</code>；<code>n.Target</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Notification</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;NOTIFICATION_EVENT_INVALID&quot;)</code>；<code>errors.New(&quot;NOTIFICATION_KIND_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!strings.HasPrefix(n.Event, &quot;account-owner:&quot;)</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_EVENT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, errors.New(&quot;NOTIFICATION_KIND_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>n.ReadAt != nil</code>。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=18661:20500 -->
````go
// 通知显示通用事件名；文档标题和正文仅在跳转目标的授权接口中读取。
func notificationValue(n notices.Entry) (dto.Notification, error) {
	value := dto.Notification{Id: domain.TextID(n.ID), EventId: n.Event, Kind: n.Kind}
	target := n.Target.Document
	switch n.Kind {
	case "ORG_OWNER_TRANSFER":
		value.Title, target = "组织所有者已转交给你", n.Target.Organization
		// 内部来源命名空间使用冒号，公开event_id沿冻结OperationID字符集呈现。
		// 只转换传输表示，原始事件及唯一键不变，旧已读记录仍对应同一通知。
		if !strings.HasPrefix(n.Event, "account-owner:") {
			return value, errors.New("NOTIFICATION_EVENT_INVALID")
		}
		value.EventId = "account-owner-" + strings.TrimPrefix(n.Event, "account-owner:")
	case "RESPONSIBLE_TRANSFER":
		value.Title = "你已成为文档负责人"
	case "RESPONSIBLE_DEPARTURE":
		value.Title = "文档负责人已离开组织，请安排接任人"
	case "COMMENT_MENTION":
		value.Title, target = "有人在讨论中提到了你", n.Target.Thread
	case "COMMENT_REPLY":
		value.Title, target = "你参与的讨论有新回复", n.Target.Thread
	case "REVIEW_SUBMITTED":
		value.Title, target = "有新的审核提交", n.Target.Request
	case "REVIEW_RESUBMITTED":
		value.Title, target = "审核内容已重新提交", n.Target.Request
	case "REVIEW_ACTION":
		value.Title, target = "审核状态有更新", n.Target.Request
	case "REVIEW_MERGED":
		value.Title, target = "审核内容已合并", n.Target.Request
	default:
		return value, errors.New("NOTIFICATION_KIND_INVALID")
	}
	if _, err := domain.CanonicalID(target); err != nil {
		return value, err
	}
	value.TargetId = &target
	if n.ReadAt != nil {
		read := n.ReadAt.UTC().Format(time.RFC3339Nano)
		value.ReadAt = &read
	}
	return value, nil
}

````

## 第 24 步：Notifications.validateTarget

插入位置：第 23 步代码之后，紧接追加；完成稿约第 617 行。

本步声明：`func (s *Notifications) validateTarget(ctx context.Context, tx *sql.Tx, actor Actor, a DocumentAccess, n notices.Entry) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, a)</code>；<code>documents.KnowledgeBaseRole(ctx, tx, kb, actor.ID)</code>；<code>comments.ThreadRow(ctx, tx, id, false)</code>；<code>s.comments.validateContext(ctx, tx, actor, a, t, false)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, a)</code>；<code>documents.KnowledgeBaseRole(ctx, tx, kb, actor.ID)</code>；<code>comments.ThreadRow(ctx, tx, id, false)</code>；<code>s.comments.validateContext(ctx, tx, actor, a, t, false)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.comments.validateContext(ctx, tx, actor, a, t, false)</code> |
| <code>a</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, a)</code>；<code>s.comments.validateContext(ctx, tx, actor, a, t, false)</code>；<code>s.comments.reviews.validateReviewReadAccess(ctx, tx, a, pointer)</code> |
| <code>n</code> | <code>notices.Entry</code> | 使用 <code>notices.Entry</code> 类型；其定义处的职责：保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。<br>在此声明中的实际使用：<code>n.Kind</code>；<code>n.Target</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_TARGET_NOT_AVAILABLE&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;KNOWLEDGE_BASE_ADMIN_REQUIRED&quot;)</code>；<code>errors.New(&quot;NOTIFICATION_TARGET_INVALID&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>doc.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_TARGET_NOT_AVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>n.Kind == &quot;RESPONSIBLE_DEPARTURE&quot;</code>。
- 检查 <code>doc.KnowledgeBaseId == nil &#124;&#124; *doc.KnowledgeBaseId != n.Target.KnowledgeBase</code>。条件成立时执行 <code>return domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_TARGET_NOT_AVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows) &#124;&#124; (err == nil &amp;&amp; role != &quot;ADMIN&quot;)</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;KNOWLEDGE_BASE_ADMIN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>n.Target.Thread != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>domain.TextID(t.Document) != doc.Id</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_TARGET_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.comments.validateContext(ctx, tx, actor, a, t, false)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>n.Target.Request != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 24 bytes=20500:21953 -->
````go
func (s *Notifications) validateTarget(ctx context.Context, tx *sql.Tx, actor Actor, a DocumentAccess, n notices.Entry) error {
	doc, err := ValidateAccess(ctx, tx, a)
	if err != nil {
		return err
	}
	if doc.Lifecycle != "ACTIVE" {
		return domain.Violation(domain.EntityNotFound, "NOTIFICATION_TARGET_NOT_AVAILABLE")
	}
	if n.Kind == "RESPONSIBLE_DEPARTURE" {
		if doc.KnowledgeBaseId == nil || *doc.KnowledgeBaseId != n.Target.KnowledgeBase {
			return domain.Violation(domain.EntityNotFound, "NOTIFICATION_TARGET_NOT_AVAILABLE")
		}
		kb, err := domain.CanonicalID(n.Target.KnowledgeBase)
		if err != nil {
			return err
		}
		role, err := documents.KnowledgeBaseRole(ctx, tx, kb, actor.ID)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && role != "ADMIN") {
			return domain.Violation(domain.PermissionDenied, "KNOWLEDGE_BASE_ADMIN_REQUIRED")
		}
		if err != nil {
			return err
		}
	}
	if n.Target.Thread != "" {
		id, err := domain.CanonicalID(n.Target.Thread)
		if err != nil {
			return err
		}
		t, err := comments.ThreadRow(ctx, tx, id, false)
		if err != nil {
			return err
		}
		if domain.TextID(t.Document) != doc.Id {
			return errors.New("NOTIFICATION_TARGET_INVALID")
		}
		return s.comments.validateContext(ctx, tx, actor, a, t, false)
	}
	if n.Target.Request != "" {
		id, err := domain.CanonicalID(n.Target.Request)
		if err != nil {
			return err
		}
		pointer, err := documents.ReviewPointerRow(ctx, tx, id)
		if err != nil {
````

## 第 25 步：Notifications.validateTarget（接着实现）

插入位置：第 24 步代码之后，紧接追加；完成稿约第 662 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>domain.TextID(pointer.Document) != doc.Id</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_TARGET_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.comments.reviews.validateReviewReadAccess(ctx, tx, a, pointer)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=21953:22168 -->
````go
			return err
		}
		if domain.TextID(pointer.Document) != doc.Id {
			return errors.New("NOTIFICATION_TARGET_INVALID")
		}
		return s.comments.reviews.validateReviewReadAccess(ctx, tx, a, pointer)
	}
	return nil
}

````

## 第 26 步：Notifications.List

插入位置：第 25 步代码之后，紧接追加；完成稿约第 672 行。

本步声明：`func (s *Notifications) List(ctx context.Context, actor Actor, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>notices.Candidates(ctx, s.db, actor.ID, after, limit+1)</code>；<code>s.recipients.Resolve(ctx, id, &#91;&#93;uint64{actor.ID})</code>；<code>s.access.Authorize(ctx, actor, id, &quot;NOTIFICATIONS&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code>；<code>s.access.Authorize(ctx, actor, id, &quot;NOTIFICATIONS&quot;)</code>；<code>s.validateTarget(ctx, tx, actor, accesses&#91;id&#93;, n)</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |
| <code>cursor</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;NOTIFICATION_ACCESS_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>n.Target.Organization != &quot;&quot;</code>。
- 检查 <code>exists</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>proof.Recipients&#91;0&#93;.Eligible</code>。

<!-- write-step: 26 bytes=22168:23759 -->
````go
func (s *Notifications) List(ctx context.Context, actor Actor, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error) {
	binding := "notifications:" + domain.TextID(actor.ID) + ":" + hex.EncodeToString(actor.Proof.SessionRef)
	limit, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	var after uint64
	if last != "" {
		after, err = domain.CanonicalID(last)
		if err != nil {
			return nil, nil, err
		}
	}
	// 有界扫描允许隐藏项形成空页，但必须保留续页位置，不能泄露隐藏总数。
	candidates, err := notices.Candidates(ctx, s.db, actor.ID, after, limit+1)
	if err != nil {
		return nil, nil, err
	}
	accesses := map[uint64]DocumentAccess{}
	organizations := map[string]platform.NotificationRecipientsResponse{}
	hidden := map[uint64]bool{}
	sources, err := MergeSources(actor.Proof)
	if err != nil {
		return nil, nil, err
	}
	epochs := map[string]uint64{}
	for _, source := range sources {
		epochs[source.ScopeKey] = source.Epoch
	}
	for _, n := range candidates {
		if n.Target.Organization != "" {
			if _, exists := organizations[n.Target.Organization]; exists {
				continue
			}
			id, err := domain.CanonicalID(n.Target.Organization)
			if err != nil {
				return nil, nil, err
			}
			proof, err := s.recipients.Resolve(ctx, id, []uint64{actor.ID})
			if err != nil {
				return nil, nil, err
			}
			organizations[n.Target.Organization] = proof
			// 当前组织成员身份决定旧事件是否可见，通知本身不授予访问组织的权利。
			if proof.Recipients[0].Eligible {
````

## 第 27 步：Notifications.List（接着实现）

插入位置：第 26 步代码之后，紧接追加；完成稿约第 717 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>exists &amp;&amp; old != source.Epoch</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;NOTIFICATION_ACCESS_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>exists &#124;&#124; hidden&#91;id&#93;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>notificationHidden(err)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>exists</code>。
- 检查 <code>old != source.Epoch</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;NOTIFICATION_ACCESS_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 27 bytes=23759:25178 -->
````go
				for _, source := range proof.Recipients[0].Sources {
					if old, exists := epochs[source.ScopeKey]; exists && old != source.Epoch {
						return nil, nil, domain.Violation(domain.StateConflict, "NOTIFICATION_ACCESS_CHANGED")
					}
					if _, exists := epochs[source.ScopeKey]; !exists {
						sources = append(sources, &pb.SourceEpoch{ScopeKey: source.ScopeKey, Epoch: source.Epoch})
						epochs[source.ScopeKey] = source.Epoch
					}
				}
			}
			continue
		}
		id, err := domain.CanonicalID(n.Target.Document)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := accesses[id]; exists || hidden[id] {
			continue
		}
		a, err := s.access.Authorize(ctx, actor, id, "NOTIFICATIONS")
		if notificationHidden(err) {
			hidden[id] = true
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		accesses[id] = a
		for _, source := range a.Sources {
			if old, exists := epochs[source.ScopeKey]; exists {
				if old != source.Epoch {
					return nil, nil, domain.Violation(domain.StateConflict, "NOTIFICATION_ACCESS_CHANGED")
				}
			} else {
				sources = append(sources, source)
				epochs[source.ScopeKey] = source.Epoch
			}
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 多文档结果按主键锁定，不能随通知事件的先后形成相反锁序。
		ids := make([]uint64, 0, len(accesses))
		for id := range accesses {
			ids = append(ids, id)
		}
````

## 第 28 步：Notifications.List（接着实现）

插入位置：第 27 步代码之后，紧接追加；完成稿约第 762 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>index == limit</code>。
- 在本段的匿名函数内：检查 <code>n.Target.Organization != &quot;&quot;</code>。
- 在本段的匿名函数内：检查 <code>!proof.Recipients&#91;0&#93;.Eligible</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 在本段的匿名函数内：检查 <code>proof.ExpiresUnixMS &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;NOTIFICATION_ACCESS_EXPIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>hidden&#91;id&#93;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：检查 <code>notificationHidden(err)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 在本段的匿名函数内：此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Target != n.Target &#124;&#124; current.Kind != n.Kind &#124;&#124; current.Event != n.Event</code>。条件成立时执行 <code>return nil, errors.New(&quot;NOTIFICATION_TARGET_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 28 bytes=25178:26599 -->
````go
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			if _, err := ValidateAccess(ctx, tx, accesses[id]); err != nil {
				return nil, err
			}
		}
		page := dto.NotificationPage{Items: []dto.Notification{}}
		scanned := after
		for index, n := range candidates {
			if index == limit {
				page.HasMore = true
				break
			}
			scanned = n.ID
			if n.Target.Organization != "" {
				proof := organizations[n.Target.Organization]
				if !proof.Recipients[0].Eligible {
					continue
				}
				// 这些来源已由统一Read事务加锁并登记输出租约；短期权威证明不可跨过期点复用。
				if proof.ExpiresUnixMS <= time.Now().UnixMilli() {
					return nil, domain.Violation(domain.TemporarilyUnavailable, "NOTIFICATION_ACCESS_EXPIRED")
				}
			} else {
				id, _ := domain.CanonicalID(n.Target.Document)
				if hidden[id] {
					continue
				}
				if err := s.validateTarget(ctx, tx, actor, accesses[id], n); err != nil {
					if notificationHidden(err) {
						continue
					}
					return nil, err
				}
			}
			current, err := notices.Row(ctx, tx, n.ID, actor.ID, false)
			if err != nil {
				return nil, err
			}
			if current.Target != n.Target || current.Kind != n.Kind || current.Event != n.Event {
				return nil, errors.New("NOTIFICATION_TARGET_CHANGED")
			}
			value, err := notificationValue(current)
			if err != nil {
				return nil, err
			}
````

## 第 29 步：Notifications.List（接着实现）

插入位置：第 28 步代码之后，紧接追加；完成稿约第 808 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=26599:26792 -->
````go
			page.Items = append(page.Items, value)
		}
		if page.HasMore {
			page.NextCursor = s.pagination.Next(binding, domain.TextID(scanned))
		}
		return page, nil
	}
	return sources, run, nil
}

````

## 第 30 步：Notifications.MarkRead

插入位置：第 29 步代码之后，紧接追加；完成稿约第 818 行。

本步声明：`func (s *Notifications) MarkRead(actor Actor, id uint64, operation string) (Actor, MutationBody, MutationPresentation)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Sources</code>；<code>actor.Proof</code>；<code>actor.ID</code>；<code>return actor, run, present</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>notices.Row(ctx, tx, id, actor.ID, true)</code>；<code>notices.MarkRead(ctx, tx, id, actor.ID)</code>；<code>OperationResult(operation, &quot;SUCCEEDED&quot;, id)</code>；<code>notices.Row(ctx, tx, id, actor.ID, false)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>OperationResult(operation, &quot;SUCCEEDED&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>present</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;NOTIFICATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(operation, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result, actor.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=26792:27785 -->
````go
func (s *Notifications) MarkRead(actor Actor, id uint64, operation string) (Actor, MutationBody, MutationPresentation) {
	actor.Sources, _ = MergeSources(actor.Proof)
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		// 冻结契约的已读能力只要求收件人本人；失去目标访问权仍可清理自己的提醒。
		if _, err := notices.Row(ctx, tx, id, actor.ID, true); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, domain.Violation(domain.EntityNotFound, "NOTIFICATION_NOT_FOUND")
			}
			return nil, err
		}
		if err := notices.MarkRead(ctx, tx, id, actor.ID); err != nil {
			return nil, err
		}
		return OperationResult(operation, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, result map[string]any) (any, []*pb.SourceEpoch, error) {
		if _, err := notices.Row(ctx, tx, id, actor.ID, false); err != nil {
			return nil, nil, err
		}
		return result, actor.Sources, nil
	}
	return actor, run, present
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/notifications_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/09-comments-notifications.md)。
