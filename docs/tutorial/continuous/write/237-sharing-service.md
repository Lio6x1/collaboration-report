# 分步编写 `services/document/internal/logic/document/sharing_service.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/237-sharing-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现个人文档邀请、申请、审批、状态分页与安全归档。

邀请只授予有限协作能力，持有链接不等于已经获得访问权；接受申请才写入具体授权。

写之前先按这条执行路线理解各段的连接：

1. personalManagement确认个人文档管理资格
2. validateInvitationGrants限定EDIT或REVIEW/COMMENT
3. CreateInvitation加密保存令牌并按当前资格展示
4. Apply固定申请
5. Decide锁定PENDING申请后接受或拒绝。
6. Applications先授权、取倒序候选、由Account原证明解析姓名，登记输出前复核原修订。
7. Archive在同一文档锁下拒绝活跃记录和旧修订，仅隐藏管理记录，已有授权与申请人历史保持。

本文件需要守住的边界：

- 不允许邀请授予MANAGE
- 取消、过期或版本改变需拒绝新申请
- 原号重放敏感链接仍要重验管理资格。
- 游标绑定状态筛选，不能把有效邀请游标用于失效列表。
- 姓名读取不能在Doc事务中远程等待，也不能直接读Account数据库。
- 关闭只接受当前OPEN邀请；失效记录应归档，不能重复推进关闭修订。
- 归档入口对不存在和无资格记录统一403及拒绝码，数据库故障仍返回服务失败，不能冒充记录不存在。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/sharing_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/sharing_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/sharing_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/sharing_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:509 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// applications.go：保留原实现，按业务合并文件。

````

## 第 2 步：Sharing.Apply

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func (s *Sharing) Apply(ctx context.Context, actor Actor, hash []byte, input dto.DocumentApplicationCreate) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.InvitationByHash(ctx, s.db, hash)</code>；<code>repository.DocumentRow(ctx, s.db, document, false)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, err</code>；<code>actor.Sources</code>；<code>actor.Proof</code>；<code>actor.ID</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>repository.InvitationByHash(ctx, s.db, hash)</code> |
| <code>input</code> | <code>dto.DocumentApplicationCreate</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.Reason</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.AclRevision != snapshot.AclRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;DOCUMENT_ACCESS_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>invite.State != &quot;OPEN&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;INVITATION_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err == nil</code>。条件成立时执行 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, existing), nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=509:2255 -->
````go
func (s *Sharing) Apply(ctx context.Context, actor Actor, hash []byte, input dto.DocumentApplicationCreate) (Actor, MutationBody, error) {
	invitation, document, err := repository.InvitationByHash(ctx, s.db, hash)
	if err != nil {
		return actor, nil, err
	}
	snapshot, err := repository.DocumentRow(ctx, s.db, document, false)
	if err != nil {
		return actor, nil, err
	}
	epoch, _ := domain.CanonicalID(snapshot.AclRevision)
	actor.Sources = append(append([]*pb.SourceEpoch{}, actor.Proof.Sources...), &pb.SourceEpoch{ScopeKey: "doc-acl:" + domain.TextID(document), Epoch: epoch})
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := repository.DocumentRow(ctx, tx, document, true)
		if err != nil {
			return nil, err
		}
		if current.AclRevision != snapshot.AclRevision {
			return nil, domain.Violation(domain.StateConflict, "DOCUMENT_ACCESS_CHANGED")
		}
		invite, _, err := invitationSnapshot(ctx, tx, invitation, true)
		if err != nil {
			return nil, err
		}
		if invite.State != "OPEN" {
			return nil, domain.Violation(domain.StateConflict, "INVITATION_NOT_CURRENT")
		}
		// 同一文档锁串行不同operation的申请，避免同一个申请者生成多个待审批实体。
		existing, err := repository.PendingApplication(ctx, tx, document, actor.ID)
		if err == nil {
			return OperationResult(input.OperationId, "SUCCEEDED", existing), nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
		raw, _ := json.Marshal(invite.RequestedGrants)
		id, err := repository.InsertApplication(ctx, tx, document, invitation, actor.ID, string(raw), input.Reason)
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}
````

## 第 3 步：Sharing.Decide

插入位置：第 2 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`func (s *Sharing) Decide(ctx context.Context, actor Actor, id uint64, input dto.ApplicationReview) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ApplicationRow(ctx, s.db, id, false)</code>；<code>s.personalManagement(ctx, actor, document, &quot;DECIDE_DOCUMENT_APPLICATION&quot;)</code>；<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, &quot;DECIDE_DOCUMENT_APPLICATION&quot;, document)</code>；<code>s.identity.Revalidate(ctx, proof)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, err</code>；<code>s.personalManagement(ctx, actor, document, &quot;DECIDE_DOCUMENT_APPLICATION&quot;)</code>；<code>actor.Sources</code>；<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, &quot;DECIDE_DOCUMENT_APPLICATION&quot;, document)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ApplicationRow(ctx, s.db, id, false)</code>；<code>repository.ApplicationRow(ctx, tx, id, true)</code>；<code>repository.DecideApplication(ctx, tx, id, actor.ID, state)</code>；<code>OperationResult(input.OperationId, resultState, id)</code> |
| <code>input</code> | <code>dto.ApplicationReview</code> | 在此声明中的实际使用：<code>input.Decision</code>；<code>input.ExpectedRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>input.Decision == &quot;ACCEPT&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.State != &quot;PENDING&quot; &#124;&#124; current.Revision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;APPLICATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.Decision == &quot;ACCEPT&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=2255:4032 -->
````go
func (s *Sharing) Decide(ctx context.Context, actor Actor, id uint64, input dto.ApplicationReview) (Actor, MutationBody, error) {
	application, err := repository.ApplicationRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	document, _ := domain.CanonicalID(application.DocumentId)
	access, err := s.personalManagement(ctx, actor, document, "DECIDE_DOCUMENT_APPLICATION")
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	if input.Decision == "ACCEPT" {
		target, _ := domain.CanonicalID(application.ApplicantId)
		proof, err := s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, "DECIDE_DOCUMENT_APPLICATION", document)
		if err != nil {
			return actor, nil, err
		}
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return actor, nil, err
		}
		sources, err := MergeSources(append(access.Proofs, proof)...)
		if err != nil {
			return actor, nil, err
		}
		baseSources, _ := MergeSources(access.Proofs...)
		actor.Sources = append(sources, access.Sources[len(baseSources):]...)
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		d, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		current, err := repository.ApplicationRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if current.State != "PENDING" || current.Revision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "APPLICATION_CHANGED")
		}
		state := "REJECTED"
		if input.Decision == "ACCEPT" {
			if err = validateInvitationGrants(ctx, tx, document, current.RequestedGrants); err != nil {
				return nil, err
			}
			for _, grant := range current.RequestedGrants {
````

## 第 4 步：Sharing.Decide（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 109 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>state == &quot;ACCEPTED&quot;</code>。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, resultState, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=4032:4945 -->
````go
				if err = repository.ApproveInvitationGrant(ctx, tx, document, current.ApplicantId, grant); err != nil {
					return nil, err
				}
			}
			state = "ACCEPTED"
			if err = repository.AdvanceDocumentACL(ctx, tx, document); err != nil {
				return nil, err
			}
			epoch, _ := domain.CanonicalID(d.AclRevision)
			if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "doc-acl:" + domain.TextID(document), Epoch: epoch, Entity: document, Action: "ACL"}); err != nil {
				return nil, err
			}
		}
		// 提交时有效性已持久固定；审批不重新检查邀请当前期限或OPEN状态。
		if err = repository.DecideApplication(ctx, tx, id, actor.ID, state); err != nil {
			return nil, err
		}
		resultState := "SUCCEEDED"
		if state == "ACCEPTED" {
			resultState = "PREPARING"
		}
		return OperationResult(input.OperationId, resultState, id), nil
	}
	return actor, run, nil
}
````

## 第 5 步：Sharing.Applications

插入位置：第 4 步代码之后，紧接追加；完成稿约第 134 行。

本步声明：`func (s *Sharing) Applications(ctx context.Context, actor Actor, document uint64, limitText, cursor string, filters ...string) ([]*pb.SourceEpoch, ReadBody, error)`。

**这一段为什么这样写：** 管理者按文档查询，本人按actor查询；姓名证明合入输出来源，最后复核修订防止审批/归档期间交付旧状态。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.personalManagement(ctx, actor, document, &quot;LIST_DOCUMENT_APPLICATIONS&quot;)</code>；<code>repository.ApplicationIDs(ctx, s.db, key, document != 0, last, limit+1, filter)</code>；<code>repository.ApplicationRow(ctx, s.db, id, false)</code>；<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, &quot;DOCUMENT_APPLICATION_NAMES&quot;, doc)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Proof</code>；<code>actor.ID</code>；<code>s.personalManagement(ctx, actor, document, &quot;LIST_DOCUMENT_APPLICATIONS&quot;)</code>；<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, &quot;DOCUMENT_APPLICATION_NAMES&quot;, doc)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>document != 0</code>；<code>s.personalManagement(ctx, actor, document, &quot;LIST_DOCUMENT_APPLICATIONS&quot;)</code>；<code>domain.TextID(document)</code>；<code>key = document</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |
| <code>cursor</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |
| <code>filters</code> | <code>...string</code> | 可选兼容参数；未传或空时ALL；明确PENDING、PROCESSED、ALL之外拒绝。<br>在此声明中的实际使用：<code>sharingFilter(filters, &quot;PENDING&quot;, &quot;PROCESSED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>document != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last == &quot;&quot;</code>。
- 检查 <code>document != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>len(ids) &gt; limit</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=4945:6464 -->
````go
func (s *Sharing) Applications(ctx context.Context, actor Actor, document uint64, limitText, cursor string, filters ...string) ([]*pb.SourceEpoch, ReadBody, error) {
	filter, err := sharingFilter(filters, "PENDING", "PROCESSED")
	if err != nil {
		return nil, nil, err
	}
	var access DocumentAccess
	sources := actor.Proof.Sources
	binding := "doc-applications:own:" + domain.TextID(actor.ID)
	if document != 0 {
		access, err = s.personalManagement(ctx, actor, document, "LIST_DOCUMENT_APPLICATIONS")
		if err != nil {
			return nil, nil, err
		}
		sources = access.Sources
		binding = "doc-applications:" + domain.TextID(actor.ID) + ":" + domain.TextID(document) + ":" + access.Document.AclRevision
	}
	binding += ":newest:" + filter
	limit, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	key := actor.ID
	if document != 0 {
		key = document
	}
	ids, err := repository.ApplicationIDs(ctx, s.db, key, document != 0, last, limit+1, filter)
	if err != nil {
		return nil, nil, err
	}
	page := dto.DocumentApplicationPage{Items: []dto.DocumentApplication{}}
	if len(ids) > limit {
		page.HasMore = true
		ids = ids[:limit]
		page.NextCursor = s.pagination.Next(binding, domain.TextID(ids[limit-1]))
	}
	for _, id := range ids {
		application, err := repository.ApplicationRow(ctx, s.db, id, false)
		if err != nil {
			return nil, nil, err
		}
		name := "已停用账号"
		target, _ := domain.CanonicalID(application.ApplicantId)
````

## 第 6 步：Sharing.Applications（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 179 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 检查 <code>!errors.As(err, &amp;violation) &#124;&#124; violation.Kind != domain.PermissionDenied</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>person.UserId != application.ApplicantId &#124;&#124; person.DisplayName == &quot;&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>document != 0</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Revision != application.Revision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;APPLICATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=6464:8263 -->
````go
		doc, _ := domain.CanonicalID(application.DocumentId)
		proof, err := s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, "DOCUMENT_APPLICATION_NAMES", doc)
		if err != nil {
			var violation domain.RuleViolation
			if !errors.As(err, &violation) || violation.Kind != domain.PermissionDenied {
				return nil, nil, err
			}
		} else {
			person, err := s.directory.Collaborator(ctx, &pb.IdentityRevalidation{AdmissionId: proof.AdmissionId, OperationId: proof.OperationId, Purpose: proof.Purpose, Target: proof.Target, IdentityTarget: proof.IdentityTarget})
			if err != nil {
				return nil, nil, err
			}
			if person.UserId != application.ApplicantId || person.DisplayName == "" {
				return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
			}
			name = person.DisplayName
			sources, err = MergeSources(&pb.IdentityAdmission{Sources: sources}, proof)
			if err != nil {
				return nil, nil, err
			}
		}
		application.ApplicantDisplayName = &name
		page.Items = append(page.Items, application)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if document != 0 {
			if _, err := ValidateAccess(ctx, tx, access); err != nil {
				return nil, err
			}
		}
		// 姓名解析在事务外完成，输出登记前复核原记录，避免审批/归档后返回旧状态。
		for _, application := range page.Items {
			id, _ := domain.CanonicalID(application.Id)
			current, err := repository.ApplicationRow(ctx, tx, id, false)
			if err != nil {
				return nil, err
			}
			if current.Revision != application.Revision {
				return nil, domain.Violation(domain.StateConflict, "APPLICATION_CHANGED")
			}
		}
		return page, nil
	}
	return sources, run, nil
}
````

## 第 7 步：Sharing.OperationProgress

插入位置：第 6 步代码之后，紧接追加；完成稿约第 224 行。

本步声明：`func (s *Sharing) OperationProgress(actor Actor, operation string) ReadBody`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.ActorOperationResult(ctx, tx, operation, actor.ID)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		raw, err := repository.ActorOperationResult(ctx, tx, operation, actor.ID) 		if err != nil { 			return nil, err 		}  …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		raw, err := repository.ActorOperationResult(ctx, tx, operation, actor.ID) 		if err != nil { 			return nil, e …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>json.Unmarshal(raw, &amp;result) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;OPERATION_RESULT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=8263:8753 -->
````go
func (s *Sharing) OperationProgress(actor Actor, operation string) ReadBody {
	return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		raw, err := repository.ActorOperationResult(ctx, tx, operation, actor.ID)
		if err != nil {
			return nil, err
		}
		var result map[string]any
		if json.Unmarshal(raw, &result) != nil {
			return nil, errors.New("OPERATION_RESULT_UNAVAILABLE")
		}
		return result, nil
	}
}

// invitations.go：保留原实现，按业务合并文件。

````

## 第 8 步：Sharing.CreateInvitation

插入位置：第 7 步代码之后，紧接追加；完成稿约第 240 行。

本步声明：`func (s *Sharing) CreateInvitation(ctx context.Context, actor Actor, id uint64, hours int64, input dto.DocumentInvitationCreate) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.personalManagement(ctx, actor, id, &quot;CREATE_DOCUMENT_INVITATION&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.personalManagement(ctx, actor, id, &quot;CREATE_DOCUMENT_INVITATION&quot;)</code>；<code>return actor, nil, nil, err</code>；<code>actor.Sources</code>；<code>actor.ID</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.personalManagement(ctx, actor, id, &quot;CREATE_DOCUMENT_INVITATION&quot;)</code>；<code>validateInvitationGrants(ctx, tx, id, input.RequestedGrants)</code>；<code>domain.TextID(id)</code>；<code>repository.InsertInvitation(ctx, tx, id, actor.ID, hash&#91;:&#93;, encrypted, string(grants), time.Now().UTC().Add(time.Duration(hours)*time.Hour))</code> |
| <code>hours</code> | <code>int64</code> | 在此声明中的实际使用：<code>time.Duration(hours)</code> |
| <code>input</code> | <code>dto.DocumentInvitationCreate</code> | 在此声明中的实际使用：<code>input.ExpectedAclRevision</code>；<code>input.RequestedGrants</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>d.AclRevision != input.ExpectedAclRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;ACL_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, invite), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>json.Unmarshal(raw, &amp;ref) != nil</code>。条件成立时执行 <code>return nil, nil, errors.New(&quot;INVITATION_RESULT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=8753:10624 -->
````go
func (s *Sharing) CreateInvitation(ctx context.Context, actor Actor, id uint64, hours int64, input dto.DocumentInvitationCreate) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.personalManagement(ctx, actor, id, "CREATE_DOCUMENT_INVITATION")
	if err != nil {
		return actor, nil, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		d, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if d.AclRevision != input.ExpectedAclRevision {
			return nil, domain.Violation(domain.StateConflict, "ACL_REVISION_MISMATCH")
		}
		if err = validateInvitationGrants(ctx, tx, id, input.RequestedGrants); err != nil {
			return nil, err
		}
		raw := make([]byte, 32)
		if _, err = rand.Read(raw); err != nil {
			return nil, err
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		hash := sha256.Sum256(raw)
		encrypted, err := s.tokens.Seal(token, domain.TextID(id))
		if err != nil {
			return nil, err
		}
		grants, _ := json.Marshal(input.RequestedGrants)
		invite, err := repository.InsertInvitation(ctx, tx, id, actor.ID, hash[:], encrypted, string(grants), time.Now().UTC().Add(time.Duration(hours)*time.Hour))
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", invite), nil
	}
	// 创建与原号重放都重新核验敏感输出资格；receipt只保存实体引用，令牌由绑定密文恢复。
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, nil, err
		}
		raw, _ := json.Marshal(value["result"])
		var ref struct {
			Entity string `json:"entity_id"`
		}
		if json.Unmarshal(raw, &ref) != nil {
			return nil, nil, errors.New("INVITATION_RESULT_UNAVAILABLE")
		}
````

## 第 9 步：Sharing.CreateInvitation（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 286 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; invite.DocumentId != domain.TextID(id)</code>。条件成立时执行 <code>return nil, nil, errors.New(&quot;INVITATION_RESULT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return dto.DocumentInvitationCreated{Invitation: invite, ShareUrl: s.origin + &quot;/join-document?invitation_token=&quot; + token, OperationId: input.OperationId}, access.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=10624:11241 -->
````go
		inviteID, err := domain.CanonicalID(ref.Entity)
		if err != nil {
			return nil, nil, err
		}
		invite, encrypted, err := invitationSnapshot(ctx, tx, inviteID, true)
		if err != nil || invite.DocumentId != domain.TextID(id) {
			return nil, nil, errors.New("INVITATION_RESULT_UNAVAILABLE")
		}
		token, err := s.tokens.Open(encrypted, domain.TextID(id))
		if err != nil {
			return nil, nil, err
		}
		return dto.DocumentInvitationCreated{Invitation: invite, ShareUrl: s.origin + "/join-document?invitation_token=" + token, OperationId: input.OperationId}, access.Sources, nil
	}
	return actor, run, present, nil
}
````

## 第 10 步：Sharing.CancelInvitation

插入位置：第 9 步代码之后，紧接追加；完成稿约第 302 行。

本步声明：`func (s *Sharing) CancelInvitation(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction) (Actor, MutationBody, error)`。

**这一段为什么这样写：** 锁定原邀请后同时校验修订号与OPEN状态；原号回放走原回执，新号不能重新关闭已失效链接。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>invitationSnapshot(ctx, s.db, id, false)</code>；<code>s.personalManagement(ctx, actor, document, &quot;CANCEL_DOCUMENT_INVITATION&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, err</code>；<code>s.personalManagement(ctx, actor, document, &quot;CANCEL_DOCUMENT_INVITATION&quot;)</code>；<code>actor.Sources</code>；<code>return actor, run, nil</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>invitationSnapshot(ctx, s.db, id, false)</code>；<code>invitationSnapshot(ctx, tx, id, true)</code>；<code>repository.CloseInvitation(ctx, tx, id)</code>；<code>OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id)</code> |
| <code>input</code> | <code>dto.VersionedAction</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Revision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;INVITATION_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.State != &quot;OPEN&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;INVITATION_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=11241:12398 -->
````go
func (s *Sharing) CancelInvitation(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction) (Actor, MutationBody, error) {
	invite, _, err := invitationSnapshot(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	document, _ := domain.CanonicalID(invite.DocumentId)
	access, err := s.personalManagement(ctx, actor, document, "CANCEL_DOCUMENT_INVITATION")
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		current, _, err := invitationSnapshot(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if current.Revision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "INVITATION_REVISION_MISMATCH")
		}
		if current.State != "OPEN" {
			return nil, domain.Violation(domain.StateConflict, "INVITATION_NOT_CURRENT")
		}
		if err = repository.CloseInvitation(ctx, tx, id); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}
````

## 第 11 步：Sharing.Invitations

插入位置：第 10 步代码之后，紧接追加；完成稿约第 334 行。

本步声明：`func (s *Sharing) Invitations(ctx context.Context, actor Actor, id uint64, limitText, cursor string, filters ...string) ([]*pb.SourceEpoch, ReadBody, error)`。

**这一段为什么这样写：** 先在同一事务中重验个人文档管理资格，再按服务端状态过滤取降序页。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.personalManagement(ctx, actor, id, &quot;LIST_DOCUMENT_INVITATIONS&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.personalManagement(ctx, actor, id, &quot;LIST_DOCUMENT_INVITATIONS&quot;)</code>；<code>actor.ID</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.personalManagement(ctx, actor, id, &quot;LIST_DOCUMENT_INVITATIONS&quot;)</code>；<code>domain.TextID(id)</code>；<code>repository.InvitationIDs(ctx, tx, id, last, limit+1, filter)</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |
| <code>cursor</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |
| <code>filters</code> | <code>...string</code> | ACTIVE、INACTIVE或ALL；绑定到同一个签名游标中。<br>在此声明中的实际使用：<code>sharingFilter(filters, &quot;ACTIVE&quot;, &quot;INACTIVE&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last == &quot;&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>len(page.Items) &gt; limit</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=12398:13875 -->
````go
func (s *Sharing) Invitations(ctx context.Context, actor Actor, id uint64, limitText, cursor string, filters ...string) ([]*pb.SourceEpoch, ReadBody, error) {
	filter, err := sharingFilter(filters, "ACTIVE", "INACTIVE")
	if err != nil {
		return nil, nil, err
	}
	access, err := s.personalManagement(ctx, actor, id, "LIST_DOCUMENT_INVITATIONS")
	if err != nil {
		return nil, nil, err
	}
	binding := "doc-invitations:" + domain.TextID(actor.ID) + ":" + domain.TextID(id) + ":" + access.Document.AclRevision + ":newest:" + filter
	limit, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		ids, err := repository.InvitationIDs(ctx, tx, id, last, limit+1, filter)
		if err != nil {
			return nil, err
		}
		page := dto.DocumentInvitationPage{Items: []dto.DocumentInvitation{}}
		for _, key := range ids {
			value, _, err := invitationSnapshot(ctx, tx, key, false)
			if err != nil {
				return nil, err
			}
			page.Items = append(page.Items, value)
		}
		if len(page.Items) > limit {
			page.Items = page.Items[:limit]
			page.HasMore = true
			page.NextCursor = s.pagination.Next(binding, page.Items[limit-1].Id)
		}
		return page, nil
	}
	return access.Sources, run, nil
}

// sharing.go：保留原实现，按业务合并文件。

````

## 第 12 步：InvitationCipher

插入位置：第 11 步代码之后，紧接追加；完成稿约第 379 行。

本步声明：`type InvitationCipher interface {`。

**这一段为什么这样写：** 服务器需再次展示邀请链接时保存密文；查询匹配另用摘要，不在数据库公开原文令牌。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Seal</code> | <code>(string, string) (&#91;&#93;byte, error)</code> | 原文令牌与附加绑定字符串（文档）输入，返回加密字节；绑定参与认证，不能跨文档换用。 |
| <code>Open</code> | <code>(&#91;&#93;byte, string) (string, error)</code> | 密文字节与同一文档绑定输入，返回原文令牌或验证错误。 |

<!-- write-step: 12 bytes=13875:13987 -->
````go
type InvitationCipher interface {
	Seal(string, string) ([]byte, error)
	Open([]byte, string) (string, error)
}
````

## 第 13 步：Sharing

插入位置：第 12 步代码之后，紧接追加；完成稿约第 383 行。

本步声明：`type Sharing struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。 |
| <code>tokens</code> | <code>InvitationCipher</code> | 使用 <code>InvitationCipher</code> 类型；其定义处的职责：服务器需再次展示邀请链接时保存密文；查询匹配另用摘要，不在数据库公开原文令牌。 |
| <code>origin</code> | <code>string</code> | 用户站点的固定源地址，用于构建同源编辑/资源/邀请入口；不能信任任意客户端Host。 |
| <code>directory</code> | <code>CollaboratorDirectoryReader</code> | 消费者定义的人员解析接口；通过Account原文档/目标/用途证明读取申请人昵称。 |

<!-- write-step: 13 bytes=13987:14186 -->
````go
type Sharing struct {
	db         *sql.DB
	identity   *Identity
	access     *Access
	pagination *Pagination
	tokens     InvitationCipher
	origin     string
	directory  CollaboratorDirectoryReader
}

````

## 第 14 步：NewSharing

插入位置：第 13 步代码之后，紧接追加；完成稿约第 393 行。

本步声明：`func NewSharing(db *sql.DB, identity *Identity, access *Access, pagination *Pagination, tokens InvitationCipher, origin string, directory CollaboratorDirectoryReader) *Sharing`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>pagination: pagination</code> |
| <code>tokens</code> | <code>InvitationCipher</code> | 使用 <code>InvitationCipher</code> 类型；其定义处的职责：服务器需再次展示邀请链接时保存密文；查询匹配另用摘要，不在数据库公开原文令牌。<br>在此声明中的实际使用：<code>tokens: tokens</code> |
| <code>origin</code> | <code>string</code> | 在此声明中的实际使用：<code>origin: origin</code> |
| <code>directory</code> | <code>CollaboratorDirectoryReader</code> | 使用 <code>CollaboratorDirectoryReader</code> 类型；其定义处的职责：协作者列表通过账号服务解析名称/身份，保留稳定账号编号做内部关联。<br>在此声明中的实际使用：<code>directory: directory</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Sharing</code> | 实际返回表达式：<code>&amp;Sharing{db: db, identity: identity, access: access, pagination: pagination, tokens: tokens, origin: origin, directory: directory}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Sharing{db: db, identity: identity, access: access, pagination: pagination, tokens: tokens, origin: origin, directory: directory}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=14186:14506 -->
````go
func NewSharing(db *sql.DB, identity *Identity, access *Access, pagination *Pagination, tokens InvitationCipher, origin string, directory CollaboratorDirectoryReader) *Sharing {
	return &Sharing{db: db, identity: identity, access: access, pagination: pagination, tokens: tokens, origin: origin, directory: directory}
}

````

## 第 15 步：sharingFilter

插入位置：第 14 步代码之后，紧接追加；完成稿约第 397 行。

本步声明：`func sharingFilter(filters []string, first, second string) (string, error)`。

**这一段为什么这样写：** 把可选状态归一为固定白名单，非法输入返回400，不能拼入SQL。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>filters</code> | <code>&#91;&#93;string</code> | 兼容旧调用的可选状态数组，当前HTTP只传一个值。<br>在此声明中的实际使用：<code>len(filters)</code>；<code>filters&#91;0&#93; != &quot;&quot;</code>；<code>value = filters&#91;0&#93;</code> |
| <code>first</code> | <code>string</code> | 该列表允许的第一个业务筛选。<br>在此声明中的实际使用：<code>value != first</code> |
| <code>second</code> | <code>string</code> | 该列表允许的第二个业务筛选。<br>在此声明中的实际使用：<code>value != second</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;SHARING_FILTER_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(filters) &gt; 0 &amp;&amp; filters&#91;0&#93; != &quot;&quot;</code>。
- 检查 <code>value != &quot;ALL&quot; &amp;&amp; value != first &amp;&amp; value != second</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.InvalidRequest, &quot;SHARING_FILTER_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=14506:14828 -->
````go
func sharingFilter(filters []string, first, second string) (string, error) {
	value := "ALL"
	if len(filters) > 0 && filters[0] != "" {
		value = filters[0]
	}
	if value != "ALL" && value != first && value != second {
		return "", domain.Violation(domain.InvalidRequest, "SHARING_FILTER_INVALID")
	}
	return value, nil
}

````

## 第 16 步：Sharing.Archive

插入位置：第 15 步代码之后，紧接追加；完成稿约第 408 行。

本步声明：`func (s *Sharing) Archive(ctx context.Context, actor Actor, id uint64, kind string, input dto.VersionedAction) (Actor, MutationBody, error)`。

**这一段为什么这样写：** 管理删除是归档：原修订号、文档管理权限和非活跃状态在同一事务核验；不存在与无资格统一拒绝码，防止记录存在性探测。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>invitationSnapshot(ctx, s.db, id, false)</code>；<code>repository.ApplicationRow(ctx, s.db, id, false)</code>；<code>s.personalManagement(ctx, actor, doc, &quot;ARCHIVE_DOCUMENT_SHARING&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, domain.Violation(domain.InvalidRequest, &quot;SHARING_RECORD_INVALID&quot;)</code>；<code>return actor, nil, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>；<code>return actor, nil, lookupError</code>；<code>s.personalManagement(ctx, actor, doc, &quot;ARCHIVE_DOCUMENT_SHARING&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>invitationSnapshot(ctx, s.db, id, false)</code>；<code>repository.ApplicationRow(ctx, s.db, id, false)</code>；<code>invitationSnapshot(ctx, tx, id, true)</code>；<code>repository.ApplicationRow(ctx, tx, id, true)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>kind</code> | <code>string</code> | handler固定传INVITATION或APPLICATION，决定锁定哪种记录。<br>在此声明中的实际使用：<code>kind != &quot;INVITATION&quot;</code>；<code>kind != &quot;APPLICATION&quot;</code>；<code>kind == &quot;INVITATION&quot;</code>；<code>repository.ArchiveSharingRecord(ctx, tx, kind, id, doc, actor.ID)</code> |
| <code>input</code> | <code>dto.VersionedAction</code> | VersionedAction包含原operation_id与expected_revision；原号重试复用固定结果。<br>在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;SHARING_RECORD_INVALID&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>；<code>lookupError</code>；<code>err</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>kind != &quot;INVITATION&quot; &amp;&amp; kind != &quot;APPLICATION&quot;</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.InvalidRequest, &quot;SHARING_RECORD_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>kind == &quot;INVITATION&quot;</code>。
- 检查 <code>lookupError != nil</code>。
- 检查 <code>errors.As(lookupError, &amp;violation) &amp;&amp; violation.Kind == domain.EntityNotFound</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, nil, lookupError</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.EntityNotFound)</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>kind == &quot;INVITATION&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 16 bytes=14828:16638 -->
````go
func (s *Sharing) Archive(ctx context.Context, actor Actor, id uint64, kind string, input dto.VersionedAction) (Actor, MutationBody, error) {
	if kind != "INVITATION" && kind != "APPLICATION" {
		return actor, nil, domain.Violation(domain.InvalidRequest, "SHARING_RECORD_INVALID")
	}
	var document string
	var lookupError error
	if kind == "INVITATION" {
		value, _, err := invitationSnapshot(ctx, s.db, id, false)
		lookupError = err
		document = value.DocumentId
	} else {
		value, err := repository.ApplicationRow(ctx, s.db, id, false)
		lookupError = err
		document = value.DocumentId
	}
	if lookupError != nil {
		var violation domain.RuleViolation
		if errors.As(lookupError, &violation) && violation.Kind == domain.EntityNotFound {
			// 不存在与无管理资格同样拒绝，避免裸记录编号成为存在性探测接口。
			return actor, nil, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
		}
		return actor, nil, lookupError
	}
	doc, _ := domain.CanonicalID(document)
	access, err := s.personalManagement(ctx, actor, doc, "ARCHIVE_DOCUMENT_SHARING")
	if err != nil {
		var violation domain.RuleViolation
		if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound) {
			return actor, nil, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
		}
		return actor, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		var revision, state string
		if kind == "INVITATION" {
			value, _, err := invitationSnapshot(ctx, tx, id, true)
			if err != nil {
				return nil, err
			}
			revision, state = value.Revision, value.State
		} else {
````

## 第 17 步：Sharing.Archive（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 453 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>revision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;SHARING_RECORD_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>state == &quot;OPEN&quot; &#124;&#124; state == &quot;PENDING&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;SHARING_RECORD_ACTIVE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=16638:17261 -->
````go
			value, err := repository.ApplicationRow(ctx, tx, id, true)
			if err != nil {
				return nil, err
			}
			revision, state = value.Revision, value.State
		}
		if revision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "SHARING_RECORD_CHANGED")
		}
		if state == "OPEN" || state == "PENDING" {
			return nil, domain.Violation(domain.StateConflict, "SHARING_RECORD_ACTIVE")
		}
		if err := repository.ArchiveSharingRecord(ctx, tx, kind, id, doc, actor.ID); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}
````

## 第 18 步：Sharing.personalManagement

插入位置：第 17 步代码之后，紧接追加；完成稿约第 472 行。

本步声明：`func (s *Sharing) personalManagement(ctx context.Context, actor Actor, id uint64, purpose string) (DocumentAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code>；<code>repository.SpaceRow(ctx, s.db, spaceID, false)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>access</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;PERSONAL_DOCUMENT_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return access, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>space.Kind != &quot;PERSONAL&quot;</code>。条件成立时执行 <code>return access, domain.Violation(domain.PermissionDenied, &quot;PERSONAL_DOCUMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return access, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=17261:17921 -->
````go
func (s *Sharing) personalManagement(ctx context.Context, actor Actor, id uint64, purpose string) (DocumentAccess, error) {
	access, err := s.access.Authorize(ctx, actor, id, purpose)
	if err != nil {
		return access, err
	}
	if !access.Permissions.Manage {
		return access, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
	}
	spaceID, _ := domain.CanonicalID(access.Document.SpaceId)
	space, err := repository.SpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return access, err
	}
	if space.Kind != "PERSONAL" {
		return access, domain.Violation(domain.PermissionDenied, "PERSONAL_DOCUMENT_REQUIRED")
	}
	return access, nil
}

````

## 第 19 步：validateInvitationGrants

插入位置：第 18 步代码之后，紧接追加；完成稿约第 491 行。

邀请只能授予当前有效单元的EDIT或当前文档的REVIEW/COMMENT，不能扩成管理权限。

本步声明：`func validateInvitationGrants(ctx context.Context, tx *sql.Tx, document uint64, grants []dto.InvitationGrant) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.EditGrantScope(ctx, tx, grant.ScopeKind, id, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.EditGrantScope(ctx, tx, grant.ScopeKind, id, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.EditGrantScope(ctx, tx, grant.ScopeKind, id, document)</code>；<code>id != document</code> |
| <code>grants</code> | <code>&#91;&#93;dto.InvitationGrant</code> | 在此声明中的实际使用：<code>len(grants)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;INVITATION_GRANTS_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.InvalidRequest, &quot;EDIT_SCOPE_INVALID&quot;)</code>；<code>domain.Violation(domain.InvalidRequest, &quot;GRANT_SCOPE_NOT_IN_DOCUMENT&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>len(grants) &lt; 1 &#124;&#124; len(grants) &gt; 100</code>。条件成立时执行 <code>return domain.Violation(domain.InvalidRequest, &quot;INVITATION_GRANTS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>grant.Capability == &quot;EDIT&quot;</code>。
- 检查 <code>grant.ScopeKind != &quot;UNIT&quot; &amp;&amp; grant.ScopeKind != &quot;CHAPTER&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.InvalidRequest, &quot;EDIT_SCOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>repository.EditGrantScope(ctx, tx, grant.ScopeKind, id, document) != nil</code>。条件成立时执行 <code>return domain.Violation(domain.InvalidRequest, &quot;GRANT_SCOPE_NOT_IN_DOCUMENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>(grant.Capability != &quot;REVIEW&quot; &amp;&amp; grant.Capability != &quot;COMMENT&quot;) &#124;&#124; grant.ScopeKind != &quot;DOCUMENT&quot; &#124;&#124; id != document</code>。条件成立时执行 <code>return domain.Violation(domain.InvalidRequest, &quot;INVITATION_CAPABILITY_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=17921:18966 -->
````go
// 邀请只能授予当前有效单元的EDIT或当前文档的REVIEW/COMMENT，不能扩成管理权限。
func validateInvitationGrants(ctx context.Context, tx *sql.Tx, document uint64, grants []dto.InvitationGrant) error {
	if len(grants) < 1 || len(grants) > 100 {
		return domain.Violation(domain.InvalidRequest, "INVITATION_GRANTS_INVALID")
	}
	for _, grant := range grants {
		id, err := domain.CanonicalID(grant.ScopeId)
		if err != nil {
			return err
		}
		if grant.Capability == "EDIT" {
			if grant.ScopeKind != "UNIT" && grant.ScopeKind != "CHAPTER" {
				return domain.Violation(domain.InvalidRequest, "EDIT_SCOPE_INVALID")
			}
			if repository.EditGrantScope(ctx, tx, grant.ScopeKind, id, document) != nil {
				return domain.Violation(domain.InvalidRequest, "GRANT_SCOPE_NOT_IN_DOCUMENT")
			}
		} else if (grant.Capability != "REVIEW" && grant.Capability != "COMMENT") || grant.ScopeKind != "DOCUMENT" || id != document {
			return domain.Violation(domain.InvalidRequest, "INVITATION_CAPABILITY_INVALID")
		}
	}
	return nil
}
````

## 第 20 步：invitationSnapshot

插入位置：第 19 步代码之后，紧接追加；完成稿约第 514 行。

本步声明：`func invitationSnapshot(ctx context.Context, q repository.Queryer, id uint64, lock bool) (dto.DocumentInvitation, []byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.InvitationRow(ctx, q, id, lock)</code> |
| <code>q</code> | <code>repository.Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>repository.InvitationRow(ctx, q, id, lock)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.InvitationRow(ctx, q, id, lock)</code> |
| <code>lock</code> | <code>bool</code> | 在此声明中的实际使用：<code>repository.InvitationRow(ctx, q, id, lock)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.DocumentInvitation</code> | 实际返回表达式：<code>fact.Invitation</code>；<code>v</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>fact.Ciphertext</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fact.Invitation, nil, err</code>。当前函数在这里结束。
- 检查 <code>v.State == &quot;OPEN&quot; &amp;&amp; !fact.ExpiresAt.After(time.Now())</code>。
- 此处通过 <code>return v, fact.Ciphertext, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=18966:19412 -->
````go
func invitationSnapshot(ctx context.Context, q repository.Queryer, id uint64, lock bool) (dto.DocumentInvitation, []byte, error) {
	fact, err := repository.InvitationRow(ctx, q, id, lock)
	if err != nil {
		return fact.Invitation, nil, err
	}
	v := fact.Invitation
	v.ExpiresAt = fact.ExpiresAt.UTC().Format(time.RFC3339Nano)
	if v.State == "OPEN" && !fact.ExpiresAt.After(time.Now()) {
		v.State = "EXPIRED"
	}
	return v, fact.Ciphertext, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/sharing_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
