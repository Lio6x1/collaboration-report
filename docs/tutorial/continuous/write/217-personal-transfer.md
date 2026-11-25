# 分步编写 `services/document/internal/logic/document/personal_transfer.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/217-personal-transfer.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供个人文档邀约、参与者列表、取消拒绝和接受登记。

个人财产转交需接收方接受并承担目标额度，不能直接改负责人。

写之前先按这条执行路线理解各段的连接：

1. Offer核个人所有者
2. Detail/List限参与者
3. Decide核revision和期限
4. Accept锁父级及目标KB、固定资源与预算后登记关闭。

本文件需要守住的边界：

- 禁止转给自己
- 目标必须接收者个人空间
- 接受202不代表真实迁移已完成。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/personal_transfer.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/personal_transfer.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/personal_transfer.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/personal_transfer.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:404 -->
````go
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

````

## 第 2 步：PersonalTransferStorage

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

Node 只读计量须由真实旧来源关闭事实固定，不能由用户声称保存/关闭成功。

本步声明：`type PersonalTransferStorage struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 本次个人空间转移原操作号。 |
| <code>Document</code> | <code>uint64</code> | 测量的目标文档编号。 |
| <code>Bytes</code> | <code>uint64</code> | 对应固定存储范围的字节数。 |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 测量结果及绑定的摘要，准备与提交时核对同一测量。 |

<!-- write-step: 2 bytes=404:622 -->
````go
// Node 只读计量须由真实旧来源关闭事实固定，不能由用户声称保存/关闭成功。
type PersonalTransferStorage struct {
	Operation       string
	Document, Bytes uint64
	Digest          []byte
}
````

## 第 3 步：PersonalTransferStorageReader

插入位置：第 2 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`type PersonalTransferStorageReader interface {`。

**这一段为什么这样写：** 跨空间迁移前从协作存储核对字节数及绑定，避免只移动文档元数据却漏计正文容量。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>MeasurePersonalTransfer</code> | <code>(context.Context, domain.PersonalTransferPlan) (PersonalTransferStorage, error)</code> | ctx及固定个人转移计划输入，返回绑定原号/文档的字节数和摘要；容量必须基于真实存储测量。 |

<!-- write-step: 3 bytes=622:776 -->
````go
type PersonalTransferStorageReader interface {
	MeasurePersonalTransfer(context.Context, domain.PersonalTransferPlan) (PersonalTransferStorage, error)
}

````

## 第 4 步：PersonalTransfers

插入位置：第 3 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`type PersonalTransfers struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。 |
| <code>fences</code> | <code>*Fences</code> | 使用 <code>Fences</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。 |
| <code>storage</code> | <code>PersonalTransferStorageReader</code> | 使用 <code>PersonalTransferStorageReader</code> 类型；其定义处的职责：跨空间迁移前从协作存储核对字节数及绑定，避免只移动文档元数据却漏计正文容量。 |
| <code>quotaLimit</code> | <code>uint64</code> | 目标空间容量账户初始化/检查使用的字节上限；转移不能绕过容量记账。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 4 bytes=776:997 -->
````go
type PersonalTransfers struct {
	db         *sql.DB
	identity   *Identity
	access     *Access
	pagination *Pagination
	fences     *Fences
	storage    PersonalTransferStorageReader
	quotaLimit uint64
	instance   string
}

````

## 第 5 步：NewPersonalTransfers

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func NewPersonalTransfers(db *sql.DB, identity *Identity, access *Access, pagination *Pagination, fences *Fences, storage PersonalTransferStorageReader, quotaLimit uint64, instance string) *PersonalTransfers`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>pagination: pagination</code> |
| <code>fences</code> | <code>*Fences</code> | 使用 <code>Fences</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。<br>在此声明中的实际使用：<code>fences: fences</code> |
| <code>storage</code> | <code>PersonalTransferStorageReader</code> | 使用 <code>PersonalTransferStorageReader</code> 类型；其定义处的职责：跨空间迁移前从协作存储核对字节数及绑定，避免只移动文档元数据却漏计正文容量。<br>在此声明中的实际使用：<code>storage: storage</code> |
| <code>quotaLimit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>quotaLimit: quotaLimit</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance: instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*PersonalTransfers</code> | 实际返回表达式：<code>&amp;PersonalTransfers{db: db, identity: identity, access: access, pagination: pagination, fences: fences, storage: storage, quotaLimit: quotaLimit, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;PersonalTransfers{db: db, identity: identity, access: access, pagination: pagination, fences: fences, storage: storage, quotaLimit: quotaLimit, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=997:1383 -->
````go
func NewPersonalTransfers(db *sql.DB, identity *Identity, access *Access, pagination *Pagination, fences *Fences, storage PersonalTransferStorageReader, quotaLimit uint64, instance string) *PersonalTransfers {
	return &PersonalTransfers{db: db, identity: identity, access: access, pagination: pagination, fences: fences, storage: storage, quotaLimit: quotaLimit, instance: instance}
}

````

## 第 6 步：PersonalTransfers.Ready

插入位置：第 5 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`func (s *PersonalTransfers) Ready() bool`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>s != nil &amp;&amp; s.db != nil &amp;&amp; s.identity != nil &amp;&amp; s.access != nil &amp;&amp; s.pagination != nil &amp;&amp; s.fences != nil &amp;&amp; s.storage != nil &amp;&amp; s.quotaLimit &gt; 0 &amp;&amp; s.instance != &quot;&quot;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return s != nil &amp;&amp; s.db != nil &amp;&amp; s.identity != nil &amp;&amp; s.access != nil &amp;&amp; s.pagination != nil &amp;&amp; s.fences != nil &amp;&amp; s.storage != nil &amp;&amp; s.quotaLimit &gt; 0 &amp;&amp; s.instance != &quot;&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1383:1603 -->
````go
func (s *PersonalTransfers) Ready() bool {
	return s != nil && s.db != nil && s.identity != nil && s.access != nil && s.pagination != nil && s.fences != nil && s.storage != nil && s.quotaLimit > 0 && s.instance != ""
}

````

## 第 7 步：PersonalTransfers.Offer

插入位置：第 6 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`func (s *PersonalTransfers) Offer(ctx context.Context, actor Actor, id uint64, input dto.OwnerChange) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;PERSONAL_TRANSFER_OFFER&quot;)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &quot;PERSONAL_TRANSFER_OFFER&quot;, id)</code>；<code>s.identity.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, input.OperationId, &amp;pb.Target{DocumentId …</code>（长表达式，按本步完整代码继续）；<code>s.identity.Revalidate(ctx, proof)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, err</code>；<code>actor.ID</code>；<code>return actor, nil, domain.Violation(domain.InvalidRequest, &quot;TRANSFER_TO_SELF&quot;)</code>；<code>s.access.Authorize(ctx, actor, id, &quot;PERSONAL_TRANSFER_OFFER&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;PERSONAL_TRANSFER_OFFER&quot;)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &quot;PERSONAL_TRANSFER_OFFER&quot;, id)</code>；<code>DocumentId: id</code>；<code>repository.TransferInterlock(ctx, tx, id)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.OwnerChange</code> | 在此声明中的实际使用：<code>input.TargetUserId</code>；<code>input.OperationId</code>；<code>input.ExpectedRevision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.InvalidRequest, &quot;TRANSFER_TO_SELF&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;PERSONAL_DOCUMENT_OWNER_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>target == actor.ID</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.InvalidRequest, &quot;TRANSFER_TO_SELF&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>space.Kind != &quot;PERSONAL&quot; &#124;&#124; space.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; access.Document.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; access.Document.ResponsibleUserId == nil &#124;&#124; *access.Document.ResponsibleUserId != domain.TextID(actor.ID)</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.PermissionDenied, &quot;PERSONAL_DOCUMENT_OWNER_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; current.ControlRevision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;CONTROL_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=1603:3696 -->
````go
func (s *PersonalTransfers) Offer(ctx context.Context, actor Actor, id uint64, input dto.OwnerChange) (Actor, MutationBody, error) {
	target, err := domain.CanonicalID(input.TargetUserId)
	if err != nil {
		return actor, nil, err
	}
	if target == actor.ID {
		return actor, nil, domain.Violation(domain.InvalidRequest, "TRANSFER_TO_SELF")
	}
	access, err := s.access.Authorize(ctx, actor, id, "PERSONAL_TRANSFER_OFFER")
	if err != nil {
		return actor, nil, err
	}
	spaceID, _ := domain.CanonicalID(access.Document.SpaceId)
	space, proof, err := s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_PERSONAL_SPACE_OWNER, "PERSONAL_TRANSFER_OFFER", id)
	if err != nil {
		return actor, nil, err
	}
	if space.Kind != "PERSONAL" || space.Lifecycle != "ACTIVE" || access.Document.Lifecycle != "ACTIVE" || access.Document.ResponsibleUserId == nil || *access.Document.ResponsibleUserId != domain.TextID(actor.ID) {
		return actor, nil, domain.Violation(domain.PermissionDenied, "PERSONAL_DOCUMENT_OWNER_REQUIRED")
	}
	receiver, err := s.identity.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, input.OperationId, &pb.Target{DocumentId: id, Purpose: "PERSONAL_TRANSFER_OFFER"})
	if err != nil {
		return actor, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return actor, nil, err
	}
	if err = s.identity.Revalidate(ctx, receiver); err != nil {
		return actor, nil, err
	}
	actor.Sources, err = MergeSources(append(access.Proofs, proof, receiver)...)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = append(actor.Sources, documentSources(access)...)
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.Lifecycle != "ACTIVE" || current.ControlRevision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "CONTROL_REVISION_MISMATCH")
		}
		space, err := repository.SpaceRow(ctx, tx, spaceID, true)
		if err != nil {
````

## 第 8 步：PersonalTransfers.Offer（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 92 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>space.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;SPACE_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, offer), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3696:4578 -->
````go
			return nil, err
		}
		if space.Lifecycle != "ACTIVE" {
			return nil, domain.Violation(domain.StateConflict, "SPACE_NOT_ACTIVE")
		}
		if err = repository.TransferInterlock(ctx, tx, id); err != nil {
			return nil, err
		}
		revision, _ := domain.CanonicalRevision(input.ExpectedRevision)
		offer, err := repository.InsertPersonalTransfer(ctx, tx, domain.PersonalTransfer{Document: id, Title: current.Title, SourceUser: actor.ID, TargetUser: target, SourceSpace: spaceID, ExpectedControl: revision, OfferOperation: input.OperationId})
		if err != nil {
			return nil, err
		}
		// 申请由受本人签名的 incoming 列表读取；成功 receipt 的 entity 是 offer。
		// 既有负责人通知只能在完成迁移后发送，不能提前宣称接收方已经成为 owner。
		return OperationResult(input.OperationId, "SUCCEEDED", offer), nil
	}
	return actor, run, nil
}

````

## 第 9 步：documentSources

插入位置：第 8 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func documentSources(access DocumentAccess) []*pb.SourceEpoch`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Document</code>；<code>access.KBRevision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>result</code> |

**跟着本段代码走一遍：**

- 检查 <code>access.Document.KnowledgeBaseId != nil</code>。
- 此处通过 <code>return result</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4578:4978 -->
````go
func documentSources(access DocumentAccess) []*pb.SourceEpoch {
	acl, _ := domain.CanonicalID(access.Document.AclRevision)
	result := []*pb.SourceEpoch{{ScopeKey: "doc-acl:" + access.Document.Id, Epoch: acl}}
	if access.Document.KnowledgeBaseId != nil {
		result = append(result, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *access.Document.KnowledgeBaseId, Epoch: access.KBRevision})
	}
	return result
}

````

## 第 10 步：transferMetadata

插入位置：第 9 步代码之后，紧接追加；完成稿约第 121 行。

本步声明：`func transferMetadata(p domain.PersonalTransfer) dto.PersonalTransferMetadata`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>p</code> | <code>domain.PersonalTransfer</code> | 使用 <code>domain.PersonalTransfer</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>在此声明中的实际使用：<code>p.State</code>；<code>p.Expires</code>；<code>p.Now</code>；<code>p.ID</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.PersonalTransferMetadata</code> | 实际返回表达式：<code>result</code> |

**跟着本段代码走一遍：**

- 检查 <code>state == &quot;PENDING&quot; &amp;&amp; !p.Expires.After(p.Now)</code>。
- 检查 <code>p.AcceptOperation != &quot;&quot;</code>。
- 此处通过 <code>return result</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4978:5822 -->
````go
func transferMetadata(p domain.PersonalTransfer) dto.PersonalTransferMetadata {
	state := p.State
	if state == "PENDING" && !p.Expires.After(p.Now) {
		state = "EXPIRED"
	}
	result := dto.PersonalTransferMetadata{Id: domain.TextID(p.ID), DocumentId: domain.TextID(p.Document), DocumentTitle: p.Title, SourceUserId: domain.TextID(p.SourceUser), TargetUserId: domain.TextID(p.TargetUser), SourceSpaceId: domain.TextID(p.SourceSpace), ExpectedControlRevision: domain.TextID(p.ExpectedControl), Revision: domain.TextID(p.Revision), State: state, ExpiresAt: p.Expires.UTC().Format(time.RFC3339Nano)}
	if p.AcceptOperation != "" {
		result.AcceptOperationId = &p.AcceptOperation
		space, kb := domain.TextID(p.Plan.TargetSpace), domain.TextID(p.Plan.TargetKB)
		result.TargetSpaceId = &space
		result.TargetKnowledgeBaseId = &kb
	}
	return result
}

````

## 第 11 步：transferParticipant

插入位置：第 10 步代码之后，紧接追加；完成稿约第 136 行。

本步声明：`func transferParticipant(p domain.PersonalTransfer, actor uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>p</code> | <code>domain.PersonalTransfer</code> | 使用 <code>domain.PersonalTransfer</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>在此声明中的实际使用：<code>p.SourceUser</code>；<code>p.TargetUser</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>actor != p.SourceUser</code>；<code>actor != p.TargetUser</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;TRANSFER_PARTICIPANT_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>actor != p.SourceUser &amp;&amp; actor != p.TargetUser</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;TRANSFER_PARTICIPANT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5822:6051 -->
````go
func transferParticipant(p domain.PersonalTransfer, actor uint64) error {
	if actor != p.SourceUser && actor != p.TargetUser {
		return domain.Violation(domain.PermissionDenied, "TRANSFER_PARTICIPANT_REQUIRED")
	}
	return nil
}

````

## 第 12 步：PersonalTransfers.Detail

插入位置：第 11 步代码之后，紧接追加；完成稿约第 143 行。

本步声明：`func (s *PersonalTransfers) Detail(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PersonalTransferRow(ctx, s.db, id, false)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.PersonalTransferRow(ctx, s.db, id, false)</code>；<code>repository.PersonalTransferRow(ctx, tx, id, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>actor.Proof.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return transferMetadata(p), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor.Proof.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6051:6694 -->
````go
func (s *PersonalTransfers) Detail(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	p, err := repository.PersonalTransferRow(ctx, s.db, id, false)
	if err != nil {
		return nil, nil, err
	}
	if err = transferParticipant(p, actor.ID); err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		p, err := repository.PersonalTransferRow(ctx, tx, id, false)
		if err != nil {
			return nil, err
		}
		if err = transferParticipant(p, actor.ID); err != nil {
			return nil, err
		}
		return transferMetadata(p), nil
	}
	return actor.Proof.Sources, run, nil
}

````

## 第 13 步：PersonalTransfers.List

插入位置：第 12 步代码之后，紧接追加；完成稿约第 164 行。

本步声明：`func (s *PersonalTransfers) List(ctx context.Context, actor Actor, direction, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>direction</code> | <code>string</code> | 在此声明中的实际使用：<code>direction != &quot;incoming&quot;</code>；<code>direction != &quot;outgoing&quot;</code>；<code>&quot;personal-transfers:&quot; + domain.TextID(actor.ID) + &quot;:&quot; + hex.EncodeToString(actor.Proof.SessionRef) + &quot;:&quot; + direction</code>；<code>repository.PersonalTransferIDs(ctx, tx, actor.ID, direction, last, limit+1)</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |
| <code>cursor</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>actor.Proof.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;TRANSFER_DIRECTION_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>direction != &quot;incoming&quot; &amp;&amp; direction != &quot;outgoing&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.InvalidRequest, &quot;TRANSFER_DIRECTION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>lastText != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>more</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>more</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor.Proof.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6694:8140 -->
````go
func (s *PersonalTransfers) List(ctx context.Context, actor Actor, direction, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error) {
	if direction != "incoming" && direction != "outgoing" {
		return nil, nil, domain.Violation(domain.InvalidRequest, "TRANSFER_DIRECTION_INVALID")
	}
	binding := "personal-transfers:" + domain.TextID(actor.ID) + ":" + hex.EncodeToString(actor.Proof.SessionRef) + ":" + direction
	limit, lastText, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	var last uint64
	if lastText != "" {
		last, err = domain.CanonicalID(lastText)
		if err != nil {
			return nil, nil, err
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		ids, err := repository.PersonalTransferIDs(ctx, tx, actor.ID, direction, last, limit+1)
		if err != nil {
			return nil, err
		}
		page := dto.PersonalTransferPage{Items: []dto.PersonalTransferMetadata{}}
		more := len(ids) > limit
		if more {
			ids = ids[:limit]
		}
		for _, id := range ids {
			p, err := repository.PersonalTransferRow(ctx, tx, id, false)
			if err != nil {
				return nil, err
			}
			if err = transferParticipant(p, actor.ID); err != nil {
				return nil, err
			}
			page.Items = append(page.Items, transferMetadata(p))
		}
		if more {
			page.NextCursor = s.pagination.Next(binding, domain.TextID(ids[len(ids)-1]))
		}
		return page, nil
	}
	return actor.Proof.Sources, run, nil
}

````

## 第 14 步：PersonalTransfers.Decide

插入位置：第 13 步代码之后，紧接追加；完成稿约第 208 行。

本步声明：`func (s *PersonalTransfers) Decide(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction, action string) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, err</code>；<code>return actor, nil, domain.Violation(domain.InvalidRequest, &quot;TRANSFER_ACTION_INVALID&quot;)</code>；<code>actor.Sources</code>；<code>actor.Proof</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.PersonalTransferRow(ctx, tx, id, true)</code>；<code>repository.ClosePersonalTransfer(ctx, tx, id, revision, state)</code>；<code>OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id)</code> |
| <code>input</code> | <code>dto.VersionedAction</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.OperationId</code> |
| <code>action</code> | <code>string</code> | 在此声明中的实际使用：<code>action != &quot;CANCEL&quot;</code>；<code>action != &quot;REJECT&quot;</code>；<code>domain.ValidatePersonalTransferAction(p, actor.ID, revision, action)</code>；<code>action == &quot;REJECT&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.InvalidRequest, &quot;TRANSFER_ACTION_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>action != &quot;CANCEL&quot; &amp;&amp; action != &quot;REJECT&quot;</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.InvalidRequest, &quot;TRANSFER_ACTION_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>action == &quot;REJECT&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=8140:9154 -->
````go
func (s *PersonalTransfers) Decide(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction, action string) (Actor, MutationBody, error) {
	revision, err := domain.CanonicalRevision(input.ExpectedRevision)
	if err != nil {
		return actor, nil, err
	}
	if action != "CANCEL" && action != "REJECT" {
		return actor, nil, domain.Violation(domain.InvalidRequest, "TRANSFER_ACTION_INVALID")
	}
	actor.Sources = actor.Proof.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		p, err := repository.PersonalTransferRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidatePersonalTransferAction(p, actor.ID, revision, action); err != nil {
			return nil, err
		}
		state := "CANCELLED"
		if action == "REJECT" {
			state = "REJECTED"
		}
		if err = repository.ClosePersonalTransfer(ctx, tx, id, revision, state); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}

````

## 第 15 步：PersonalTransfers.Accept

插入位置：第 14 步代码之后，紧接追加；完成稿约第 237 行。

本步声明：`func (s *PersonalTransfers) Accept(ctx context.Context, actor Actor, id uint64, input dto.PersonalTransferAccept) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PersonalTransferRow(ctx, s.db, id, false)</code>；<code>repository.DocumentRow(ctx, s.db, offer.Document, false)</code>；<code>repository.CreationKnowledgeBase(ctx, s.db, kb, false)</code>；<code>s.access.SpaceIdentity(ctx, actor, targetSpace, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &quot;PERSONAL_TRANSFER_ACCEPT&quot;, offer.Document)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PERSONAL_TRANSFER_NOT_READY&quot;)</code>；<code>return actor, nil, nil, err</code>；<code>actor.ID</code>；<code>s.access.SpaceIdentity(ctx, actor, targetSpace, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &quot;PERSONAL_TRANSFER_ACCEPT&quot;, offer.Document)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.PersonalTransferRow(ctx, s.db, id, false)</code>；<code>repository.PersonalTransferRow(ctx, tx, id, true)</code> |
| <code>input</code> | <code>dto.PersonalTransferAccept</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.TargetKnowledgeBaseId</code>；<code>input.ExpectedQuotaRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PERSONAL_TRANSFER_NOT_READY&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRANSFER_TARGET_PERSONAL_SPACE_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!s.Ready()</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PERSONAL_TRANSFER_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>targetFacts.Kind != &quot;PERSONAL&quot; &#124;&#124; targetSpace == offer.SourceSpace &#124;&#124; kbState != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;TRANSFER_TARGET_PERSONAL_SPACE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=9154:11403 -->
````go
func (s *PersonalTransfers) Accept(ctx context.Context, actor Actor, id uint64, input dto.PersonalTransferAccept) (Actor, MutationBody, MutationPresentation, error) {
	if !s.Ready() {
		return actor, nil, nil, domain.Violation(domain.TemporarilyUnavailable, "PERSONAL_TRANSFER_NOT_READY")
	}
	offer, err := repository.PersonalTransferRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, nil, err
	}
	revision, err := domain.CanonicalRevision(input.ExpectedRevision)
	if err != nil {
		return actor, nil, nil, err
	}
	if err = domain.ValidatePersonalTransferAction(offer, actor.ID, revision, "ACCEPT"); err != nil {
		return actor, nil, nil, err
	}
	kb, err := domain.CanonicalID(input.TargetKnowledgeBaseId)
	if err != nil {
		return actor, nil, nil, err
	}
	expectedQuota, err := domain.CanonicalRevision(input.ExpectedQuotaRevision)
	if err != nil {
		return actor, nil, nil, err
	}
	current, err := repository.DocumentRow(ctx, s.db, offer.Document, false)
	if err != nil {
		return actor, nil, nil, err
	}
	targetKB, err := repository.CreationKnowledgeBase(ctx, s.db, kb, false)
	if err != nil {
		return actor, nil, nil, err
	}
	targetSpace, targetACL, kbState := targetKB.Space, targetKB.Revision, targetKB.Lifecycle
	targetFacts, _, err := s.access.SpaceIdentity(ctx, actor, targetSpace, pb.IdentityCapability_PERSONAL_SPACE_OWNER, "PERSONAL_TRANSFER_ACCEPT", offer.Document)
	if err != nil {
		return actor, nil, nil, err
	}
	if targetFacts.Kind != "PERSONAL" || targetSpace == offer.SourceSpace || kbState != "ACTIVE" {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "TRANSFER_TARGET_PERSONAL_SPACE_REQUIRED")
	}
	targetProof, err := s.identity.AcquireBound(ctx, actor, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, input.OperationId, &pb.Target{DocumentId: offer.Document, Purpose: "PERSONAL_TRANSFER_ACCEPT"})
	if err != nil {
		return actor, nil, nil, err
	}
	sourceProof, err := s.identity.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: offer.SourceUser}, input.OperationId, &pb.Target{DocumentId: offer.Document, Purpose: "PERSONAL_TRANSFER_ACCEPT"})
	if err != nil {
````

## 第 16 步：PersonalTransfers.Accept（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 282 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return actor, nil, nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>current.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>doc.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; doc.ControlRevision != domain.TextID(p.ExpectedControl) &#124;&#124; doc.SpaceId != domain.TextID(p.SourceSpace) &#124;&#124; doc.ResponsibleUserId == nil &#124;&#124; *doc.Responsi …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;TRANSFER_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 16 bytes=11403:13436 -->
````go
		return actor, nil, nil, err
	}
	if err = s.identity.Revalidate(ctx, targetProof); err != nil {
		return actor, nil, nil, err
	}
	if err = s.identity.Revalidate(ctx, sourceProof); err != nil {
		return actor, nil, nil, err
	}
	sources, err := MergeSources(actor.Proof, targetProof, sourceProof)
	if err != nil {
		return actor, nil, nil, err
	}
	acl, _ := domain.CanonicalRevision(current.AclRevision)
	publication, _ := domain.CanonicalRevision(current.PublicationEpoch)
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + current.Id, Epoch: acl}, &pb.SourceEpoch{ScopeKey: "publication:" + current.Id, Epoch: publication}, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(kb), Epoch: targetACL})
	if current.KnowledgeBaseId != nil {
		sourceKB, err := domain.CanonicalID(*current.KnowledgeBaseId)
		if err != nil {
			return actor, nil, nil, err
		}
		epoch, err := repository.KnowledgeBaseACL(ctx, s.db, sourceKB)
		if err != nil {
			return actor, nil, nil, err
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *current.KnowledgeBaseId, Epoch: epoch})
	}
	actor.Sources = sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		doc, err := repository.DocumentRow(ctx, tx, offer.Document, true)
		if err != nil {
			return nil, err
		}
		p, err := repository.PersonalTransferRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidatePersonalTransferAction(p, actor.ID, revision, "ACCEPT"); err != nil {
			return nil, err
		}
		if doc.Lifecycle != "ACTIVE" || doc.ControlRevision != domain.TextID(p.ExpectedControl) || doc.SpaceId != domain.TextID(p.SourceSpace) || doc.ResponsibleUserId == nil || *doc.ResponsibleUserId != domain.TextID(p.SourceUser) || doc.AclRevision != current.AclRevision || doc.PublicationEpoch != current.PublicationEpoch {
			return nil, domain.Violation(domain.StateConflict, "TRANSFER_DOCUMENT_CHANGED")
		}
		if err = repository.TransferInterlock(ctx, tx, p.Document); err != nil {
			return nil, err
		}
````

## 第 17 步：PersonalTransfers.Accept（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 327 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>p.ExpectedControl == ^uint64(0) &#124;&#124; acl == ^uint64(0) &#124;&#124; publication == ^uint64(0)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;TRANSFER_REVISION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 17 bytes=13436:15749 -->
````go
		if p.ExpectedControl == ^uint64(0) || acl == ^uint64(0) || publication == ^uint64(0) {
			return nil, domain.Violation(domain.StateConflict, "TRANSFER_REVISION_EXHAUSTED")
		}
		if err = lockTransferParents(ctx, tx, p.SourceSpace, targetSpace, kb, actor.ID); err != nil {
			return nil, err
		}
		_, target, err := resources.LockTransferQuotas(ctx, tx, p.SourceSpace, targetSpace, s.quotaLimit)
		if err != nil {
			return nil, err
		}
		fixed, amount, err := resources.PrepareTransferResources(ctx, tx, p.Document, p.SourceSpace, targetSpace)
		if err != nil {
			return nil, err
		}
		body, paid, err := repository.TransferBodyCapacity(ctx, tx, p.Document)
		if err != nil {
			return nil, err
		}
		amount, err = domain.TransferCapacity(amount, body)
		if err != nil {
			return nil, err
		}
		if err = resources.ReserveTransferCapacity(ctx, tx, target, amount, expectedQuota); err != nil {
			return nil, err
		}
		sourceIdentity, _ := proto.Marshal(sourceProof)
		receiverIdentity, _ := proto.Marshal(targetProof)
		plan := domain.PersonalTransferPlan{Operation: input.OperationId, Transfer: p.ID, Document: p.Document, SourceUser: p.SourceUser, TargetUser: p.TargetUser, SourceSpace: p.SourceSpace, TargetSpace: targetSpace, TargetKB: kb, ExpectedControl: p.ExpectedControl, ClosingControl: p.ExpectedControl + 1, ACL: acl, Publication: publication, BodyBytes: body, SourceBodyBytes: paid, ReservedBytes: amount, Resources: fixed, SourceIdentity: sourceIdentity, ReceiverIdentity: receiverIdentity, ReceiverSession: actor.Proof.SessionRef}
		for _, v := range []struct {
			scope string
			epoch uint64
		}{{"doc-acl:" + doc.Id, acl}, {"publication:" + doc.Id, publication}} {
			plan.Scopes = append(plan.Scopes, domain.PurgeScope{Key: v.scope, Epoch: v.epoch, Operation: domain.PersonalTransferFence(input.OperationId, v.scope)})
		}
		if err = repository.BeginPersonalTransfer(ctx, tx, plan); err != nil {
			return nil, err
		}
		for _, scope := range plan.Scopes {
			if err = QueueClosure(ctx, tx, scope.Operation, actor.ID, domain.Closure{Scope: scope.Key, Epoch: scope.Epoch, Entity: p.Document, Action: "PERSONAL_TRANSFER"}); err != nil {
				return nil, err
			}
		}
		if err = queueVisibilityIndexEvents(ctx, tx, input.OperationId, []uint64{p.Document}, 0); err != nil {
			return nil, err
		}
````

## 第 18 步：PersonalTransfers.Accept（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 372 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return purgeProgress(input.OperationId, &quot;PREPARING&quot;, p.Document), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return value, actor.Proof.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=15749:16106 -->
````go
		return purgeProgress(input.OperationId, "PREPARING", p.Document), nil
	}
	// 旧 Doc 来源已经被本事务关闭，最小原号输出只绑定真实 receiver session。
	present := func(_ context.Context, _ *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		return value, actor.Proof.Sources, nil
	}
	return actor, run, present, nil
}

````

## 第 19 步：lockTransferParents

插入位置：第 18 步代码之后，紧接追加；完成稿约第 381 行。

本步声明：`func lockTransferParents(ctx context.Context, tx *sql.Tx, source, target, kb, user uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockedCreationKnowledgeBase(ctx, tx, kb)</code>；<code>repository.SpaceRow(ctx, tx, id, true)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockedCreationKnowledgeBase(ctx, tx, kb)</code>；<code>repository.SpaceRow(ctx, tx, id, true)</code> |
| <code>source</code> | <code>uint64</code> | 在此声明中的实际使用：<code>ids := &#91;&#93;uint64{source, target}</code> |
| <code>target</code> | <code>uint64</code> | 在此声明中的实际使用：<code>facts.Space != target</code>；<code>ids := &#91;&#93;uint64{source, target}</code>；<code>id == target</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.LockedCreationKnowledgeBase(ctx, tx, kb)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRANSFER_TARGET_KB_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;TRANSFER_SPACE_CHANGED&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;PERSONAL_SPACE_NOT_OWNED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>facts.Space != target &#124;&#124; facts.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;TRANSFER_TARGET_KB_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>s.Kind != &quot;PERSONAL&quot; &#124;&#124; s.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;TRANSFER_SPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>id == target &amp;&amp; (s.OwnerUserId == nil &#124;&#124; *s.OwnerUserId != domain.TextID(user))</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;PERSONAL_SPACE_NOT_OWNED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=16106:16979 -->
````go
func lockTransferParents(ctx context.Context, tx *sql.Tx, source, target, kb, user uint64) error {
	facts, err := repository.LockedCreationKnowledgeBase(ctx, tx, kb)
	if err != nil {
		return err
	}
	if facts.Space != target || facts.Lifecycle != "ACTIVE" {
		return domain.Violation(domain.StateConflict, "TRANSFER_TARGET_KB_CHANGED")
	}
	ids := []uint64{source, target}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		s, err := repository.SpaceRow(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if s.Kind != "PERSONAL" || s.Lifecycle != "ACTIVE" {
			return domain.Violation(domain.StateConflict, "TRANSFER_SPACE_CHANGED")
		}
		if id == target && (s.OwnerUserId == nil || *s.OwnerUserId != domain.TextID(user)) {
			return domain.Violation(domain.PermissionDenied, "PERSONAL_SPACE_NOT_OWNED")
		}
	}
	return nil
}

````

## 第 20 步：ErrPersonalTransferIdle

插入位置：第 19 步代码之后，紧接追加；完成稿约第 406 行。

本步声明：`var ErrPersonalTransferIdle = errors.New("PERSONAL_TRANSFER_IDLE")`。

<!-- write-step: 20 bytes=16979:17046 -->
````go
var ErrPersonalTransferIdle = errors.New("PERSONAL_TRANSFER_IDLE")
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/personal_transfer.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
