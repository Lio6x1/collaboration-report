# 分步编写 `services/document/internal/logic/document/resources_protection_service.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/227-resources-protection-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

为协作UPDATE预保护资源并原子预留正文容量。

服务证书只允许调用接口，实际actor/target/sources仍需匹配Document持久EDIT准入。

写之前先按这条执行路线理解各段的连接：

1. ValidateDraftResourceRequest校验绑定
2. validateProtectionGrant锁核原准入
3. protect按排序资源取锁并检查READY/空间分配
4. ProtectBody同事务处理正文
5. Get按原操作查询回执。

本文件需要守住的边界：

- 正文预留与资源拒绝不能形成半成功
- 跨单元共享资源必须先合并排序，避免反向锁序。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/resources_protection_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/resources_protection_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/resources_protection_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/resources_protection_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:599 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/proto"
	"slices"
	"time"
)

// fixed_resource_locks.go：保留原实现，按业务合并文件。

````

## 第 2 步：lockFixedContentResources

插入位置：第 1 步代码之后，紧接追加；完成稿约第 24 行。

MAIN 与 candidate 可能以不同单元顺序共享资源；先合并排序再锁定，避免跨 Doc
的合法转让资源在不同写入口产生相反的资源锁顺序。

本步声明：`func lockFixedContentResources(ctx context.Context, tx *sql.Tx, document, space uint64, contents []*pb.FixedContent, reason string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockedDocumentResource(ctx, tx, id, document, space)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockedDocumentResource(ctx, tx, id, document, space)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.LockedDocumentResource(ctx, tx, id, document, space)</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.LockedDocumentResource(ctx, tx, id, document, space)</code> |
| <code>contents</code> | <code>&#91;&#93;*pb.FixedContent</code> | 本次操作使用的全部固定正文，汇总资源引用后按稳定顺序锁定，防止候选/基线中的图片并发被清理。 |
| <code>reason</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.Violation(domain.StateConflict, reason)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, reason)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, reason)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=599:1372 -->
````go
// MAIN 与 candidate 可能以不同单元顺序共享资源；先合并排序再锁定，避免跨 Doc
// 的合法转让资源在不同写入口产生相反的资源锁顺序。
func lockFixedContentResources(ctx context.Context, tx *sql.Tx, document, space uint64, contents []*pb.FixedContent, reason string) error {
	var ids []uint64
	for _, content := range contents {
		ids = append(ids, content.ResourceIds...)
	}
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		state, err := repository.LockedDocumentResource(ctx, tx, id, document, space)
		if err != nil {
			return err
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, reason)
		}
	}
	return nil
}

// resource_protection.go：保留原实现，按业务合并文件。

````

## 第 3 步：NodeOutcomeReader

插入位置：第 2 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`type NodeOutcomeReader interface {`。

**这一段为什么这样写：** 图片引用保护的恢复依赖真实协作更新是否落盘，不能因HTTP超时直接删对象。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>UpdateOutcome</code> | <code>(context.Context, *pb.DraftResourceRequest) (domain.NodeUpdateProof, error)</code> | ctx及原草稿资源请求查询Node该次更新结果，返回有绑定的持久结果证明；未知时不能释放图片保护。 |

<!-- write-step: 3 bytes=1372:1499 -->
````go
type NodeOutcomeReader interface {
	UpdateOutcome(context.Context, *pb.DraftResourceRequest) (domain.NodeUpdateProof, error)
}
````

## 第 4 步：DraftResources

插入位置：第 3 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`type DraftResources struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>outcomes</code> | <code>NodeOutcomeReader</code> | 使用 <code>NodeOutcomeReader</code> 类型；其定义处的职责：图片引用保护的恢复依赖真实协作更新是否落盘，不能因HTTP超时直接删对象。 |

<!-- write-step: 4 bytes=1499:1577 -->
````go
type DraftResources struct {
	db       *sql.DB
	outcomes NodeOutcomeReader
}

````

## 第 5 步：NewDraftResources

插入位置：第 4 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`func NewDraftResources(db *sql.DB, outcomes NodeOutcomeReader) *DraftResources`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>outcomes</code> | <code>NodeOutcomeReader</code> | 使用 <code>NodeOutcomeReader</code> 类型；其定义处的职责：图片引用保护的恢复依赖真实协作更新是否落盘，不能因HTTP超时直接删对象。<br>在此声明中的实际使用：<code>outcomes: outcomes</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*DraftResources</code> | 实际返回表达式：<code>&amp;DraftResources{db: db, outcomes: outcomes}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;DraftResources{db: db, outcomes: outcomes}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1577:1712 -->
````go
func NewDraftResources(db *sql.DB, outcomes NodeOutcomeReader) *DraftResources {
	return &DraftResources{db: db, outcomes: outcomes}
}
````

## 第 6 步：protectionReceipt

插入位置：第 5 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func protectionReceipt(req *pb.DraftResourceRequest, state string) *pb.DraftResourceReceipt`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>req</code> | <code>*pb.DraftResourceRequest</code> | 在此声明中的实际使用：<code>req.ResourceIds</code>；<code>req.Context</code>；<code>req.Generation</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>pb.DraftResourceReceipt_State(pb.DraftResourceReceipt_State_value&#91;state&#93;)</code>；<code>domain.ProtectionReceiptDigest(req.Context.OperationId, req.Generation, digest, state)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 实际返回表达式：<code>&amp;pb.DraftResourceReceipt{State: value, OperationId: req.Context.OperationId, Generation: req.Generation, ResourceDigest: digest, ReceiptDigest: domain.ProtectionReceiptDigest(req.C …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;pb.DraftResourceReceipt{State: value, OperationId: req.Context.OperationId, Generation: req.Generation, ResourceDigest: digest, ReceiptDigest: domain.ProtectionReceiptDiges …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1712:2184 -->
````go
func protectionReceipt(req *pb.DraftResourceRequest, state string) *pb.DraftResourceReceipt {
	digest := domain.ProtectionDigest(req.ResourceIds)
	value := pb.DraftResourceReceipt_State(pb.DraftResourceReceipt_State_value[state])
	return &pb.DraftResourceReceipt{State: value, OperationId: req.Context.OperationId, Generation: req.Generation, ResourceDigest: digest, ReceiptDigest: domain.ProtectionReceiptDigest(req.Context.OperationId, req.Generation, digest, state)}
}
````

## 第 7 步：loadProtection

插入位置：第 6 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func loadProtection(ctx context.Context, q repository.Queryer, id string) (*pb.DraftResourceRequest, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ResourceProtectionBinding(ctx, q, id)</code> |
| <code>q</code> | <code>repository.Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>repository.ResourceProtectionBinding(ctx, q, id)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.ResourceProtectionBinding(ctx, q, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceRequest</code> | 实际返回表达式：<code>nil</code>；<code>req</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RejectProtection(domain.ProtectionMissing, &quot;PROTECTION_NOT_FOUND&quot;)</code>；<code>domain.RejectProtection(domain.ProtectionUnknown, &quot;PROTECTION_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionMissing, &quot;PROTECTION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;saved) != nil</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionUnknown, &quot;PROTECTION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; proto.Unmarshal(wire, req) != nil</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionUnknown, &quot;PROTECTION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return req, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2184:2922 -->
````go
func loadProtection(ctx context.Context, q repository.Queryer, id string) (*pb.DraftResourceRequest, error) {
	raw, err := repository.ResourceProtectionBinding(ctx, q, id)
	if err != nil {
		return nil, domain.RejectProtection(domain.ProtectionMissing, "PROTECTION_NOT_FOUND")
	}
	var saved struct {
		Wire string `json:"wire"`
	}
	if json.Unmarshal(raw, &saved) != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_BINDING_UNKNOWN")
	}
	wire, err := base64.RawStdEncoding.DecodeString(saved.Wire)
	req := new(pb.DraftResourceRequest)
	if err != nil || proto.Unmarshal(wire, req) != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_BINDING_UNKNOWN")
	}
	return req, nil
}

````

## 第 8 步：validateProtectionGrant

插入位置：第 7 步代码之后，紧接追加；完成稿约第 81 行。

服务证书只许可调用；实际actor/target/全部来源必须匹配Doc持久的EDIT准入。

本步声明：`func validateProtectionGrant(ctx context.Context, tx *sql.Tx, req *pb.DraftResourceRequest) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AdmissionSources(ctx, tx, req.Context.DocumentAdmissionRef)</code>；<code>LockSources(ctx, tx, sources)</code>；<code>repository.LockedProtectionAdmission(ctx, tx, req.Context.DocumentAdmissionRef)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.AdmissionSources(ctx, tx, req.Context.DocumentAdmissionRef)</code>；<code>LockSources(ctx, tx, sources)</code>；<code>repository.LockedProtectionAdmission(ctx, tx, req.Context.DocumentAdmissionRef)</code> |
| <code>req</code> | <code>*pb.DraftResourceRequest</code> | 在此声明中的实际使用：<code>req.Context</code>；<code>req.GrantId</code>；<code>req.ClientSessionId</code>；<code>req.RoomId</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RejectProtection(domain.ProtectionForbidden, &quot;SOURCES_REQUIRED&quot;)</code>；<code>err</code>；<code>domain.RejectProtection(domain.ProtectionForbidden, &quot;SOURCE_CLOSED&quot;)</code>；<code>domain.RejectProtection(domain.ProtectionForbidden, &quot;GRANT_CLOSED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, domain.ErrSourcesRequired)</code>。条件成立时执行 <code>return domain.RejectProtection(domain.ProtectionForbidden, &quot;SOURCES_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.RejectProtection(domain.ProtectionForbidden, &quot;SOURCE_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; current.State != &quot;OPEN&quot; &#124;&#124; !current.Expires.After(time.Now())</code>。条件成立时执行 <code>return domain.RejectProtection(domain.ProtectionForbidden, &quot;GRANT_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(current.TargetJSON, &amp;binding) != nil &#124;&#124; binding.GrantID != req.GrantId &#124;&#124; binding.ClientSessionID != req.ClientSessionId &#124;&#124; binding.RoomID != req.RoomId &#124;&#124; binding.T …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.RejectProtection(domain.ProtectionForbidden, &quot;RESOURCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(sources) != len(req.Context.Sources)</code>。条件成立时执行 <code>return domain.RejectProtection(domain.ProtectionForbidden, &quot;SOURCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(source, req.Context.Sources&#91;i&#93;)</code>。条件成立时执行 <code>return domain.RejectProtection(domain.ProtectionForbidden, &quot;SOURCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2922:4608 -->
````go
// 服务证书只许可调用；实际actor/target/全部来源必须匹配Doc持久的EDIT准入。
func validateProtectionGrant(ctx context.Context, tx *sql.Tx, req *pb.DraftResourceRequest) error {
	sources, err := repository.AdmissionSources(ctx, tx, req.Context.DocumentAdmissionRef)
	if errors.Is(err, domain.ErrSourcesRequired) {
		return domain.RejectProtection(domain.ProtectionForbidden, "SOURCES_REQUIRED")
	}
	if err != nil {
		return err
	}
	if err = LockSources(ctx, tx, sources); err != nil {
		return domain.RejectProtection(domain.ProtectionForbidden, "SOURCE_CLOSED")
	}
	current, err := repository.LockedProtectionAdmission(ctx, tx, req.Context.DocumentAdmissionRef)
	if err != nil || current.State != "OPEN" || !current.Expires.After(time.Now()) {
		return domain.RejectProtection(domain.ProtectionForbidden, "GRANT_CLOSED")
	}
	var binding EditBinding
	if json.Unmarshal(current.TargetJSON, &binding) != nil || binding.GrantID != req.GrantId || binding.ClientSessionID != req.ClientSessionId || binding.RoomID != req.RoomId || binding.Target == nil || binding.Target.WorkspaceEpoch != req.WorkspaceEpoch || current.Actor != req.Context.ActorId || !proto.Equal(binding.Target, req.Context.Target) || req.Context.IdentityAdmissionRef != "" {
		return domain.RejectProtection(domain.ProtectionForbidden, "RESOURCE_BINDING_INVALID")
	}
	if len(sources) != len(req.Context.Sources) {
		return domain.RejectProtection(domain.ProtectionForbidden, "SOURCE_BINDING_INVALID")
	}
	for i, source := range sources {
		if !proto.Equal(source, req.Context.Sources[i]) {
			return domain.RejectProtection(domain.ProtectionForbidden, "SOURCE_BINDING_INVALID")
		}
	}
	return nil
}
````

## 第 9 步：DraftResources.Protect

插入位置：第 8 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`func (s *DraftResources) Protect(ctx context.Context, req *pb.DraftResourceRequest) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.protect(ctx, req, nil)</code> |
| <code>req</code> | <code>*pb.DraftResourceRequest</code> | 在此声明中的实际使用：<code>s.protect(ctx, req, nil)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 由 <code>s.protect(ctx, req, nil)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.protect(ctx, req, nil)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.protect(ctx, req, nil)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4608:4764 -->
````go
func (s *DraftResources) Protect(ctx context.Context, req *pb.DraftResourceRequest) (*pb.DraftResourceReceipt, error) {
	return s.protect(ctx, req, nil)
}

````

## 第 10 步：DraftResources.ProtectBody

插入位置：第 9 步代码之后，紧接追加；完成稿约第 115 行。

正文与资源使用同一原号、同一准入、同一事务，不留下正文已预留而资源拒绝的半结果。

本步声明：`func (s *DraftResources) ProtectBody(ctx context.Context, req *pb.DraftResourceRequest, body domain.DraftBodyMeasure) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.protect(ctx, req, &amp;body)</code> |
| <code>req</code> | <code>*pb.DraftResourceRequest</code> | 在此声明中的实际使用：<code>s.protect(ctx, req, &amp;body)</code> |
| <code>body</code> | <code>domain.DraftBodyMeasure</code> | 使用 <code>domain.DraftBodyMeasure</code> 类型；其定义处的职责：表达草稿正文计量、资源保护绑定及Node提交证明。<br>在此声明中的实际使用：<code>&amp;body</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 由 <code>s.protect(ctx, req, &amp;body)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.protect(ctx, req, &amp;body)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.protect(ctx, req, &amp;body)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4764:5080 -->
````go
// 正文与资源使用同一原号、同一准入、同一事务，不留下正文已预留而资源拒绝的半结果。
func (s *DraftResources) ProtectBody(ctx context.Context, req *pb.DraftResourceRequest, body domain.DraftBodyMeasure) (*pb.DraftResourceReceipt, error) {
	return s.protect(ctx, req, &body)
}

````

## 第 11 步：DraftResources.protect

插入位置：第 10 步代码之后，紧接追加；完成稿约第 120 行。

本步声明：`func (s *DraftResources) protect(ctx context.Context, req *pb.DraftResourceRequest, body *domain.DraftBodyMeasure) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>validateProtectionGrant(ctx, tx, req)</code>；<code>repository.LockedResourceDocument(ctx, tx, req.Context.Target.DocumentId)</code>；<code>repository.LockDocumentCapacity(ctx, tx, req.Context.Target.DocumentId)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |
| <code>req</code> | <code>*pb.DraftResourceRequest</code> | 在此声明中的实际使用：<code>ValidateDraftResourceRequest(req)</code>；<code>proto.MarshalOptions{Deterministic: true}.Marshal(req)</code>；<code>validateProtectionGrant(ctx, tx, req)</code>；<code>req.Context</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>body</code> | <code>*domain.DraftBodyMeasure</code> | 使用 <code>domain.DraftBodyMeasure</code> 类型；其定义处的职责：表达草稿正文计量、资源保护绑定及Node提交证明。<br>在此声明中的实际使用：<code>body != nil</code>；<code>resources.ReserveDraftBody(ctx, tx, q, req.Context.Target.DocumentId, req.RoomId, req.WorkspaceEpoch, req.Context.OperationId, *body, exists)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 实际返回表达式：<code>nil</code>；<code>protectionReceipt(req, prior.State)</code>；<code>protectionReceipt(req, &quot;PROTECTED&quot;)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.RejectProtection(domain.ProtectionForbidden, &quot;DOCUMENT_NOT_CURRENT&quot;)</code>；<code>domain.RejectProtection(domain.ProtectionConflict, &quot;PROTECTION_ID_CONFLICT&quot;)</code>；<code>domain.RejectProtection(domain.ProtectionPrecondition, &quot;PROTECTION_RELEASED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionForbidden, &quot;DOCUMENT_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>body != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>exists</code>。
- 检查 <code>!bytes.Equal(prior.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionConflict, &quot;PROTECTION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>prior.State == &quot;RELEASED&quot;</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionPrecondition, &quot;PROTECTION_RELEASED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>body != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=5080:6708 -->
````go
func (s *DraftResources) protect(ctx context.Context, req *pb.DraftResourceRequest, body *domain.DraftBodyMeasure) (*pb.DraftResourceReceipt, error) {
	if err := ValidateDraftResourceRequest(req); err != nil {
		return nil, err
	}
	wire, _ := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	fingerprint := sha256.Sum256(wire)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = validateProtectionGrant(ctx, tx, req); err != nil {
		return nil, err
	}
	space, lifecycle, err := repository.LockedResourceDocument(ctx, tx, req.Context.Target.DocumentId)
	if err != nil || lifecycle != "ACTIVE" {
		return nil, domain.RejectProtection(domain.ProtectionForbidden, "DOCUMENT_NOT_CURRENT")
	}
	// 配额总是位于资源锁之前；不以事务外余额接受正文。
	if body != nil {
		if err = repository.LockDocumentCapacity(ctx, tx, req.Context.Target.DocumentId); err != nil {
			return nil, err
		}
	}
	prior, err := repository.LockedResourceProtection(ctx, tx, req.Context.OperationId)
	exists := err == nil
	if exists {
		if !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
			return nil, domain.RejectProtection(domain.ProtectionConflict, "PROTECTION_ID_CONFLICT")
		}
		if prior.State == "RELEASED" {
			return nil, domain.RejectProtection(domain.ProtectionPrecondition, "PROTECTION_RELEASED")
		}
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	if body != nil {
		limit, err := resources.QuotaLimit(ctx, tx, space)
		if err != nil {
			return nil, err
		}
		q, err := resources.LockQuota(ctx, tx, space, limit)
		if err != nil {
			return nil, err
		}
````

## 第 12 步：DraftResources.protect（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 165 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>exists</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return protectionReceipt(req, prior.State), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; state != &quot;READY&quot;</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionPrecondition, &quot;RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!allowed</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionPrecondition, &quot;RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionUnknown, &quot;PROTECTION_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return protectionReceipt(req, &quot;PROTECTED&quot;), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6708:8751 -->
````go
		if err = resources.ReserveDraftBody(ctx, tx, q, req.Context.Target.DocumentId, req.RoomId, req.WorkspaceEpoch, req.Context.OperationId, *body, exists); err != nil {
			return nil, err
		}
	}
	if exists {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return protectionReceipt(req, prior.State), nil
	}
	// READY核验与pin插入持有同一资源行锁；删除器也取得此锁，避免核验后资源被删除。
	for _, id := range req.ResourceIds {
		_, state, err := repository.LockedResourceSpace(ctx, tx, id)
		if err != nil || state != "READY" {
			return nil, domain.RejectProtection(domain.ProtectionPrecondition, "RESOURCE_NOT_READY")
		}
		allowed, err := resources.ResourceAllowedForDocument(ctx, tx, id, req.Context.Target.DocumentId, space)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, domain.RejectProtection(domain.ProtectionPrecondition, "RESOURCE_NOT_READY")
		}
	}
	ids, _ := json.Marshal(req.ResourceIds)
	sources, _ := json.Marshal(SourcesWire(req.Context.Sources))
	record := domain.ProtectionRecord{Operation: req.Context.OperationId, Document: req.Context.Target.DocumentId, Room: req.RoomId, Grant: req.GrantId, Epoch: req.WorkspaceEpoch, Generation: req.Generation, Fingerprint: fingerprint[:], ResourceJSON: string(ids), SourceJSON: string(sources)}
	if err = repository.InsertResourceProtection(ctx, tx, record); err != nil {
		return nil, err
	}
	saved, _ := json.Marshal(map[string]string{"wire": base64.RawStdEncoding.EncodeToString(wire)})
	if err = repository.InsertResourceProtectionBinding(ctx, tx, req.Context.OperationId, saved); err != nil {
		return nil, err
	}
	for _, id := range req.ResourceIds {
		if err = repository.InsertOperationResourceRef(ctx, tx, id, req.Context.Target.DocumentId, req.Context.OperationId, req.Generation); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_COMMIT_UNKNOWN")
	}
	return protectionReceipt(req, "PROTECTED"), nil
}
````

## 第 13 步：DraftResources.Get

插入位置：第 12 步代码之后，紧接追加；完成稿约第 209 行。

本步声明：`func (s *DraftResources) Get(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.queryProtection(ctx, in)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>validateProtectionGrant(ctx, tx, req)</code>；<code>repository.LockedResourceProtectionState(ctx, tx, req.Context.OperationId)</code> |
| <code>in</code> | <code>*pb.DraftResourceQuery</code> | 在此声明中的实际使用：<code>s.queryProtection(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 实际返回表达式：<code>nil</code>；<code>protectionReceipt(req, state)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.RejectProtection(domain.ProtectionUnknown, &quot;PROTECTION_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionUnknown, &quot;PROTECTION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return protectionReceipt(req, state), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=8751:9417 -->
````go
func (s *DraftResources) Get(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceReceipt, error) {
	req, err := s.queryProtection(ctx, in)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = validateProtectionGrant(ctx, tx, req); err != nil {
		return nil, err
	}
	state, err := repository.LockedResourceProtectionState(ctx, tx, req.Context.OperationId)
	if err != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_UNKNOWN")
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return protectionReceipt(req, state), nil
}
````

## 第 14 步：DraftResources.queryProtection

插入位置：第 13 步代码之后，紧接追加；完成稿约第 231 行。

本步声明：`func (s *DraftResources) queryProtection(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceRequest, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>loadProtection(ctx, s.db, in.Context.OperationId)</code> |
| <code>in</code> | <code>*pb.DraftResourceQuery</code> | 在此声明中的实际使用：<code>in == nil</code>；<code>in.Context</code>；<code>in.Generation</code>；<code>in.ResourceDigest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceRequest</code> | 实际返回表达式：<code>nil</code>；<code>req</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RejectProtection(domain.ProtectionInvalid, &quot;PROTECTION_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.RejectProtection(domain.ProtectionConflict, &quot;PROTECTION_BINDING_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>in == nil &#124;&#124; in.Context == nil</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionInvalid, &quot;PROTECTION_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(req.Context, in.Context) &#124;&#124; req.Generation != in.Generation &#124;&#124; !bytes.Equal(domain.ProtectionDigest(req.ResourceIds), in.ResourceDigest)</code>。条件成立时执行 <code>return nil, domain.RejectProtection(domain.ProtectionConflict, &quot;PROTECTION_BINDING_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return req, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=9417:10128 -->
````go
func (s *DraftResources) queryProtection(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceRequest, error) {
	if in == nil || in.Context == nil {
		return nil, domain.RejectProtection(domain.ProtectionInvalid, "PROTECTION_QUERY_INVALID")
	}
	req, err := loadProtection(ctx, s.db, in.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(req.Context, in.Context) || req.Generation != in.Generation || !bytes.Equal(domain.ProtectionDigest(req.ResourceIds), in.ResourceDigest) {
		return nil, domain.RejectProtection(domain.ProtectionConflict, "PROTECTION_BINDING_CONFLICT")
	}
	return req, nil
}

// resource_protection_input.go：保留原实现，按业务合并文件。

````

## 第 15 步：ValidateDraftResourceRequest

插入位置：第 14 步代码之后，紧接追加；完成稿约第 247 行。

RPC 和正文侧账 HTTP 共享请求校验，传输扩展不能绕过原 UPDATE 身份绑定。

本步声明：`func ValidateDraftResourceRequest(req *pb.DraftResourceRequest) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>req</code> | <code>*pb.DraftResourceRequest</code> | 在此声明中的实际使用：<code>req == nil</code>；<code>req.Context</code>；<code>req.Generation</code>；<code>req.UpdateHash</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RejectProtection(domain.ProtectionInvalid, &quot;PROTECTION_INVALID&quot;)</code>；<code>domain.RejectProtection(domain.ProtectionInvalid, &quot;PROTECTION_ID_INVALID&quot;)</code>；<code>domain.RejectProtection(domain.ProtectionInvalid, &quot;RESOURCE_SET_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>req == nil &#124;&#124; req.Context == nil &#124;&#124; req.Context.Target == nil &#124;&#124; !domain.ValidOperationID(req.Context.OperationId) &#124;&#124; req.Generation != 1 &#124;&#124; len(req.UpdateHash) != 32 &#124;&#124; !bytes.Equ …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.RejectProtection(domain.ProtectionInvalid, &quot;PROTECTION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>req.Context.OperationId != hex.EncodeToString(expected&#91;:&#93;)</code>。条件成立时执行 <code>return domain.RejectProtection(domain.ProtectionInvalid, &quot;PROTECTION_ID_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>id == 0 &#124;&#124; (i &gt; 0 &amp;&amp; req.ResourceIds&#91;i-1&#93; &gt;= id)</code>。条件成立时执行 <code>return domain.RejectProtection(domain.ProtectionInvalid, &quot;RESOURCE_SET_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=10128:11235 -->
````go
// RPC 和正文侧账 HTTP 共享请求校验，传输扩展不能绕过原 UPDATE 身份绑定。
func ValidateDraftResourceRequest(req *pb.DraftResourceRequest) error {
	if req == nil || req.Context == nil || req.Context.Target == nil || !domain.ValidOperationID(req.Context.OperationId) || req.Generation != 1 || len(req.UpdateHash) != 32 || !bytes.Equal(req.Context.RequestFingerprint, req.UpdateHash) || !domain.ValidOperationID(req.ClientUpdateId) || len(req.ResourceIds) > 10000 {
		return domain.RejectProtection(domain.ProtectionInvalid, "PROTECTION_INVALID")
	}
	expected := sha256.Sum256([]byte(domain.TextID(req.RoomId) + "\n" + domain.TextID(req.WorkspaceEpoch) + "\n" + req.ClientSessionId + "\n" + req.ClientUpdateId + "\n" + req.GrantId))
	if req.Context.OperationId != hex.EncodeToString(expected[:]) {
		return domain.RejectProtection(domain.ProtectionInvalid, "PROTECTION_ID_INVALID")
	}
	for i, id := range req.ResourceIds {
		if id == 0 || (i > 0 && req.ResourceIds[i-1] >= id) {
			return domain.RejectProtection(domain.ProtectionInvalid, "RESOURCE_SET_INVALID")
		}
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/resources_protection_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
